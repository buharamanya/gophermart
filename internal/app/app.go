package app

import (
	"github.com/go-chi/chi/v5"

	"github.com/buharamanya/gophermart/internal/handlers"
	"github.com/buharamanya/gophermart/internal/service/auth"
	"github.com/buharamanya/gophermart/internal/service/order"
)

type App struct {
	authService  *auth.Service
	orderService *order.Service
	// balanceService    *service.Balance
	// withdrawalService *service.Withdrawal
}

func New(
	auth *auth.Service,
	order *order.Service,
	// balance *balance.Service,
	// withdrawal *withdrawal.Service,
) *App {
	return &App{
		authService:  auth,
		orderService: order,
		// balanceService:    balance,
		// withdrawalService: withdrawal,
	}
}

func (a *App) Router() *chi.Mux {
	r := chi.NewRouter()
	handler := handlers.NewHandler(
		a.authService,
		a.orderService,
		// a.balanceService,
		// a.withdrawalService,
	)
	handler.RegisterRoutes(r)
	return r
}
