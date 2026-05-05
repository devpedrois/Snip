//go:build integration

package integration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/golang-migrate/migrate/v4"
	migmysql "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	goredis "github.com/redis/go-redis/v9"
	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/devpedrois/snip/internal/analytics"
	"github.com/devpedrois/snip/internal/handler"
	"github.com/devpedrois/snip/internal/middleware"
	mysqlrepo "github.com/devpedrois/snip/internal/repository/mysql"
	redisrepo "github.com/devpedrois/snip/internal/repository/redis"
	"github.com/devpedrois/snip/internal/scanner"
	"github.com/devpedrois/snip/internal/service"
)

type testEnv struct {
	server     *httptest.Server
	dispatcher *analytics.Dispatcher
	cleanup    func()
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	ctx := context.Background()

	mysqlCtr, err := tcmysql.Run(ctx, "mysql:8.0.36",
		tcmysql.WithDatabase("snip_test"),
		tcmysql.WithUsername("snip"),
		tcmysql.WithPassword("snip_pass"),
	)
	if err != nil {
		t.Fatalf("start mysql container: %v", err)
	}

	redisCtr, err := tcredis.Run(ctx, "redis:7")
	if err != nil {
		_ = mysqlCtr.Terminate(ctx)
		t.Fatalf("start redis container: %v", err)
	}

	mysqlDSN, err := mysqlCtr.ConnectionString(ctx, "parseTime=true")
	if err != nil {
		t.Fatalf("mysql connection string: %v", err)
	}

	redisAddr, err := redisCtr.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("redis connection string: %v", err)
	}

	if err := runMigrations(mysqlDSN); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	db, err := mysqlrepo.NewMySQLDB(ctx, mysqlDSN, 10, 5)
	if err != nil {
		t.Fatalf("open mysql db: %v", err)
	}

	redisClient := goredis.NewClient(&goredis.Options{Addr: redisAddr[len("redis://"):]})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Fatalf("ping redis: %v", err)
	}

	urlCache := redisrepo.NewRedisURLCache(redisClient)
	urlRepo := mysqlrepo.NewURLRepository(db)
	clickRepo := mysqlrepo.NewClickRepository(db)

	disp := analytics.NewDispatcher(clickRepo, urlRepo, 2, 100)
	dispCtx, dispCancel := context.WithCancel(context.Background())
	disp.Run(dispCtx)

	shortenerSvc := service.NewShortenerService(urlRepo, urlCache, scanner.NewNoopScanner(), "http://localhost", 30, 10*time.Second)
	redirectorSvc := service.NewRedirectorService(urlRepo, urlCache, 30)
	analyticsSvc := service.NewAnalyticsService(urlRepo, clickRepo)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Recovery(true))

	r.Post("/api/shorten", handler.NewShortenHandler(shortenerSvc, "http://localhost").Handle)
	r.Get("/api/analytics/{hash}", handler.NewAnalyticsHandler(analyticsSvc).Handle)
	r.Get("/health", handler.NewHealthHandler(db, redisClient).Handle)
	r.Get("/{hash}", handler.NewRedirectHandler(redirectorSvc, disp).Handle)

	srv := httptest.NewServer(r)

	cleanup := func() {
		srv.Close()
		dispCancel()
		disp.Shutdown(3 * time.Second)
		_ = redisClient.Close()
		db.Close()
		_ = redisCtr.Terminate(context.Background())
		_ = mysqlCtr.Terminate(context.Background())
	}

	return &testEnv{server: srv, dispatcher: disp, cleanup: cleanup}
}

func runMigrations(dsn string) error {
	migrationDSN := dsn + "&multiStatements=true"
	db, err := sql.Open("mysql", migrationDSN)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer db.Close()

	driver, err := migmysql.WithInstance(db, &migmysql.Config{})
	if err != nil {
		return fmt.Errorf("driver: %w", err)
	}

	_, callerFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(callerFile), "..", "..")
	migrationsPath := filepath.Join(projectRoot, "migrations")

	m, err := migrate.NewWithDatabaseInstance(fmt.Sprintf("file://%s", migrationsPath), "mysql", driver)
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("up: %w", err)
	}

	return nil
}
