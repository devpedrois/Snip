package main

import (
	"context"
	"database/sql"
	"encoding/json"
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

	"github.com/devpedrois/Snip/internal/config"
	mysqlrepo "github.com/devpedrois/Snip/internal/repository/mysql"
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

	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Error("failed to load config", "err", err)
		return err
	}

	db, err := mysqlrepo.NewMySQLDB(context.Background(), cfg.DSN(), cfg.MySQLMaxOpenConns, cfg.MySQLMaxIdleConns)
	if err != nil {
		logger.Error("failed to connect to mysql", "err", err)
		return err
	}
	defer db.Close()

	if err := runMigrations(db, logger); err != nil {
		logger.Error("failed to run migrations", "err", err)
		return err
	}

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)

	r.Get("/health", healthHandler)

	srv := &http.Server{
		Addr:         net.JoinHostPort("", cfg.AppPort),
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("server starting", "port", cfg.AppPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case <-quit:
		logger.Info("shutdown signal received")
	case err := <-serverErr:
		return fmt.Errorf("server: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	logger.Info("server stopped")
	return nil
}

func runMigrations(db *sql.DB, logger *slog.Logger) error {
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

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
		slog.Error("health: encode response", "err", err)
	}
}
