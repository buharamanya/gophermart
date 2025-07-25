package main

import (
	"context"

	"go.uber.org/zap"

	"github.com/buharamanya/gophermart/internal/app"
	"github.com/buharamanya/gophermart/internal/config"
	"github.com/buharamanya/gophermart/internal/logger"
	"github.com/buharamanya/gophermart/internal/repository"
	"github.com/buharamanya/gophermart/internal/service/auth"
	"github.com/buharamanya/gophermart/internal/service/balance"
	"github.com/buharamanya/gophermart/internal/service/order"
	"github.com/buharamanya/gophermart/internal/service/withdrawal"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger.Initialize("info")

	db, err := app.SetupDatabase(ctx, cfg.DatabaseURI)
	if err != nil {
		logger.Log.Fatal("failed to start database", zap.Error(err))
	}
	defer db.Close()

	if err := app.ApplyMigrations(db); err != nil {
		logger.Log.Fatal("Failed to apply migrations", zap.Error(err))
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

	app.RunServer(cfg.RunAddress, application.Router(), cfg.AccrualSystemAddress, orderService)
}
