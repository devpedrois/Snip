package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/golang-migrate/migrate/v4"
	migmysql "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/devpedrois/snip/internal/analytics"
	"github.com/devpedrois/snip/internal/config"
	"github.com/devpedrois/snip/internal/handler"
	"github.com/devpedrois/snip/internal/middleware"
	mysqlrepo "github.com/devpedrois/snip/internal/repository/mysql"
	redisrepo "github.com/devpedrois/snip/internal/repository/redis"
	"github.com/devpedrois/snip/internal/scanner"
	"github.com/devpedrois/snip/internal/service"
)

func main() {
	if err := run(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func run() error {
	var logHandler slog.Handler
	if os.Getenv("APP_ENV") == "production" {
		logHandler = slog.NewJSONHandler(os.Stdout, nil)
	} else {
		logHandler = slog.NewTextHandler(os.Stdout, nil)
	}
	logger := slog.New(logHandler)
	slog.SetDefault(logger)

	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("failed to load config", "err", err)
		return err
	}

	if cfg.AppEnv == "production" && cfg.RedisPassword == "" {
		logger.Error("REDIS_PASSWORD is required in production")
		return fmt.Errorf("config: REDIS_PASSWORD is required in production")
	}

	if cfg.VTEnabled && cfg.VTAPIKey == "" {
		logger.Error("VIRUSTOTAL_API_KEY is required when VT_ENABLED=true")
		return fmt.Errorf("config: VIRUSTOTAL_API_KEY is required when VT_ENABLED=true")
	}

	if err := runMigrations(cfg.MigrationDSN(), logger); err != nil {
		logger.Error("failed to run migrations", "err", err)
		return err
	}

	db, err := mysqlrepo.NewMySQLDB(context.Background(), cfg.DSN(), cfg.MySQLMaxOpenConns, cfg.MySQLMaxIdleConns)
	if err != nil {
		logger.Error("failed to connect to mysql", "err", err)
		return err
	}
	defer db.Close()

	redisClient, err := redisrepo.NewRedisClient(cfg)
	if err != nil {
		logger.Error("failed to connect to redis", "err", err)
		return err
	}
	defer redisClient.Close()

	urlCache := redisrepo.NewRedisURLCache(redisClient)
	urlRepo := mysqlrepo.NewURLRepository(db)
	clickRepo := mysqlrepo.NewClickRepository(db)
	reportRepo := mysqlrepo.NewReportRepository(db)

	dispatcher := analytics.NewDispatcher(clickRepo, urlRepo, cfg.AnalyticsWorkers, cfg.AnalyticsBuffer).
		WithRetention(clickRepo, cfg.AnalyticsRetentionDays)

	var vtScannerDirect scanner.URLScanner
	if cfg.VTEnabled {
		vtScannerDirect = scanner.NewVirusTotalScanner(cfg.VTAPIKey, cfg.VTTimeoutSeconds, cfg.VTMinPositives)
	} else {
		vtScannerDirect = scanner.NewNoopScanner()
	}
	vtCacheTTL := time.Duration(cfg.VTCacheTTLHours) * time.Hour
	cachedScanner := scanner.NewCachedScanner(vtScannerDirect, urlCache, vtCacheTTL)

	rescanInterval := time.Duration(cfg.VTRescanIntervalHours) * time.Hour
	rescanner := scanner.NewRescanner(vtScannerDirect, urlRepo, urlCache, rescanInterval)

	shortenerSvc := service.NewShortenerService(urlRepo, urlCache, cachedScanner, cfg.BaseURL, cfg.URLExpirationDays, cfg.VTTimeoutSeconds)
	shortenHandler := handler.NewShortenHandler(shortenerSvc, cfg.BaseURL)

	redirectorSvc := service.NewRedirectorService(urlRepo, urlCache, cfg.RedisTTLDays)
	redirectHandler := handler.NewRedirectHandlerWithCache(redirectorSvc, dispatcher, urlCache)
	redirectHandler.SetTrustedProxies(cfg.TrustedProxies)

	analyticsSvc := service.NewAnalyticsService(urlRepo, clickRepo)
	analyticsHandler := handler.NewAnalyticsHandler(analyticsSvc)

	reportSvc := service.NewReportService(urlRepo, reportRepo, urlCache, cfg.ReportAutoBlockThreshold)
	reportHandler := handler.NewReportHandler(reportSvc)

	isDev := os.Getenv("APP_ENV") != "production"

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shortenLimiter := middleware.NewRateLimiter(ctx, float64(cfg.RateLimitShorten)/60.0, cfg.RateLimitShorten)
	redirectLimiter := middleware.NewRateLimiter(ctx, float64(cfg.RateLimitRedirect)/60.0, cfg.RateLimitRedirect)
	analyticsLimiter := middleware.NewRateLimiter(ctx, float64(cfg.RateLimitAnalytics)/60.0, cfg.RateLimitAnalytics)
	reportLimiter := middleware.NewRateLimiter(ctx, float64(cfg.RateLimitReport)/60.0, cfg.RateLimitReport)
	r := chi.NewRouter()
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.NewCORS(cfg.AllowedOrigins))
	r.Use(middleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Recovery(isDev))
	r.Use(middleware.Logger(logger))

	healthHandler := handler.NewHealthHandlerWithSecret(db, redisClient, cfg.HealthSecretKey)
	r.Get("/health", healthHandler.Handle)
	r.Get("/health/details", healthHandler.HandleDetails)

	r.With(
		func(next http.Handler) http.Handler {
			return shortenLimiter.MiddlewareWithProxies(cfg.TrustedProxies, next)
		},
		middleware.BodyLimit(1<<20),
		middleware.RequireJSON,
	).Post("/api/shorten", shortenHandler.Handle)

	r.With(
		func(next http.Handler) http.Handler {
			return analyticsLimiter.MiddlewareWithProxies(cfg.TrustedProxies, next)
		},
	).Get("/api/analytics/{hash}", analyticsHandler.Handle)

	r.With(
		func(next http.Handler) http.Handler {
			return reportLimiter.MiddlewareWithProxies(cfg.TrustedProxies, next)
		},
		middleware.BodyLimit(1<<20),
		middleware.RequireJSON,
	).Post("/api/report/{hash}", reportHandler.Handle)

	r.With(
		func(next http.Handler) http.Handler {
			return redirectLimiter.MiddlewareWithProxies(cfg.TrustedProxies, next)
		},
	).Get("/{hash}", redirectHandler.Handle)

	srv := &http.Server{
		Addr:              net.JoinHostPort("", cfg.AppPort),
		Handler:           r,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	dispatcher.Run(ctx)

	go rescanner.Run(ctx)

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("server starting", "port", cfg.AppPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-serverErr:
		return fmt.Errorf("server: %w", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown error", "err", err)
	}

	dispatcher.Shutdown(5 * time.Second)

	logger.Info("server stopped")
	return nil
}

func runMigrations(dsn string, logger *slog.Logger) error {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("migrate: open: %w", err)
	}
	defer db.Close()

	driver, err := migmysql.WithInstance(db, &migmysql.Config{})
	if err != nil {
		return fmt.Errorf("migrate: driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://migrations", "mysql", driver)
	if err != nil {
		return fmt.Errorf("migrate: init: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate: up: %w", err)
	}

	logger.Info("migrations applied")
	return nil
}
