package app

import (
	"errors"

	"github.com/go-chi/chi/v5"

	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"

	"github.com/buharamanya/gophermart/internal/handlers"
	"github.com/buharamanya/gophermart/internal/logger"
	"github.com/buharamanya/gophermart/internal/service/auth"
	"github.com/buharamanya/gophermart/internal/service/balance"
	"github.com/buharamanya/gophermart/internal/service/order"
	"github.com/buharamanya/gophermart/internal/service/withdrawal"
	"github.com/buharamanya/gophermart/internal/worker"
)

type App struct {
	authService       *auth.Service
	orderService      *order.Service
	balanceService    *balance.Service
	withdrawalService *withdrawal.Service
}

func New(
	auth *auth.Service,
	order *order.Service,
	balance *balance.Service,
	withdrawal *withdrawal.Service,
) *App {
	return &App{
		authService:       auth,
		orderService:      order,
		balanceService:    balance,
		withdrawalService: withdrawal,
	}
}

func (a *App) Router() *chi.Mux {
	r := chi.NewRouter()
	handler := handlers.NewHandler(
		a.authService,
		a.orderService,
		a.balanceService,
		a.withdrawalService,
	)
	handler.RegisterRoutes(r)
	return r
}

func SetupDatabase(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Fatalf("Unable to parse database config: %v", err)
	}

	poolConfig.MaxConns = 20
	poolConfig.MinConns = 5
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = time.Minute

	return pgxpool.NewWithConfig(ctx, poolConfig)
}

func ApplyMigrations(db *pgxpool.Pool) error {
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

func RunServer(addr string, handler http.Handler, accrualSystemAddress string, svc *order.Service) {
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// Создаем контекст для graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Запуск HTTP-сервера
	go func() {
		logger.Log.Info("starting server", zap.String("address", addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Fatal("server error", zap.Error(err))
		}
	}()

	// Запуск воркера для accrual
	var workerCtx context.Context
	var workerCancel context.CancelFunc

	if accrualSystemAddress != "" {
		workerCtx, workerCancel = context.WithCancel(ctx)
		defer workerCancel()

		accrualWorker := worker.NewAccrualWorker(svc, accrualSystemAddress, 10)
		go func() {
			logger.Log.Info("starting accrual worker", zap.String("address", accrualSystemAddress))
			accrualWorker.Start(workerCtx) // Передаем отменяемый контекст
		}()
	} else {
		logger.Log.Warn("accrual system address not provided, worker not started")
	}

	// Ожидаем сигнал завершения
	<-ctx.Done()
	logger.Log.Info("shutting down server...")

	// Завершаем работу сервера с таймаутом
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error("server shutdown error", zap.Error(err))
	}

	logger.Log.Info("server stopped")

}
