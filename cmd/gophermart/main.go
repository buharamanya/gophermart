package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/buharamanya/gophermart/internal/app"
	"github.com/buharamanya/gophermart/internal/config"
	"github.com/buharamanya/gophermart/internal/logger"
	"github.com/buharamanya/gophermart/internal/repository"
	"github.com/buharamanya/gophermart/internal/service/auth"
	"github.com/buharamanya/gophermart/internal/service/order"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Initialize("info")

	db := setupDatabase(ctx, cfg.DatabaseURI)
	defer db.Close()

	if err := applyMigrations(db); err != nil {
		log.Fatalf("Failed to apply migrations: %v", err)
	}

	repo := repository.New(db)

	authService := auth.NewService(repo)
	orderService := order.NewService(repo)
	// balanceService := service.NewBalance(repo)
	// withdrawalService := service.NewWithdrawal(repo)

	application := app.New(
		authService,
		orderService,
		// balanceService,
		// withdrawalService,
	)

	runServer(ctx, cfg.RunAddress, application.Router())
}

func setupDatabase(ctx context.Context, dsn string) *pgxpool.Pool {
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("Unable to parse database config: %v", err)
	}

	poolConfig.MaxConns = 20
	poolConfig.MinConns = 5
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = time.Minute

	db, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatalf("Unable to create connection pool: %v", err)
	}

	if err := db.Ping(ctx); err != nil {
		log.Fatalf("Unable to ping database: %v", err)
	}

	return db
}

func applyMigrations(db *pgxpool.Pool) error {
	// 1. Получаем конфиг из пула и создаем *sql.DB
	config := db.Config()
	sqlDB := stdlib.OpenDB(*config.ConnConfig)
	defer sqlDB.Close()

	// 2. Устанавливаем диалект PostgreSQL
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set dialect: %w", err)
	}

	// 3. Применяем миграции
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}

func runServer(ctx context.Context, addr string, handler http.Handler) {
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	done := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGINT, syscall.SIGTERM)
		<-sigint

		log.Println("Shutting down server...")

		shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
		close(done)
	}()

	log.Printf("Starting server on %s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server error: %v", err)
	}

	<-done
	log.Println("Server stopped gracefully")
}
