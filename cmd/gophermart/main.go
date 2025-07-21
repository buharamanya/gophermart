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
	"go.uber.org/zap"

	"github.com/buharamanya/gophermart/internal/app"
	"github.com/buharamanya/gophermart/internal/config"
	"github.com/buharamanya/gophermart/internal/logger"
	"github.com/buharamanya/gophermart/internal/repository"
	"github.com/buharamanya/gophermart/internal/service/auth"
	"github.com/buharamanya/gophermart/internal/service/balance"
	"github.com/buharamanya/gophermart/internal/service/order"
	"github.com/buharamanya/gophermart/internal/service/withdrawal"
	"github.com/buharamanya/gophermart/internal/worker"
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
	balanceService := balance.NewService(repo)
	withdrawalService := withdrawal.NewService(repo)

	application := app.New(
		authService,
		orderService,
		balanceService,
		withdrawalService,
	)

	runServer(cfg.RunAddress, application.Router(), cfg.AccrualSystemAddress, orderService)
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

func runServer(addr string, handler http.Handler, accrualSystemAddress string, svc *order.Service) {
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	go func() {
		logger.Log.Info("starting server", zap.String("address", addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Log.Fatal("server error", zap.Error(err))
		}
	}()

	// Запуск воркера для accrual
	if accrualSystemAddress != "" {
		accrualWorker := worker.NewAccrualWorker(svc, accrualSystemAddress)
		go accrualWorker.Start(context.Background())
		logger.Log.Info("accrual worker started",
			zap.String("address", accrualSystemAddress))
	} else {
		logger.Log.Warn("accrual system address not provided, worker not started")
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Log.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Log.Fatal("server shutdown error", zap.Error(err))
	}

	logger.Log.Info("server stopped")

}
