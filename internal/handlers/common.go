package handlers

import (
	"github.com/buharamanya/gophermart/internal/service/auth"
	"github.com/buharamanya/gophermart/internal/service/balance"
	"github.com/buharamanya/gophermart/internal/service/order"
	"github.com/buharamanya/gophermart/internal/service/withdrawal"
)

const (
	readReqErrStr     = "failed to read request body"
	ContentTypeHeader = "Content-Type"
	JSONContentType   = "application/json"
)

type Handler struct {
	authService       *auth.Service
	orderService      *order.Service
	balanceService    *balance.Service
	withdrawalService *withdrawal.Service
}

func NewHandler(
	auth *auth.Service,
	order *order.Service,
	balance *balance.Service,
	withdrawal *withdrawal.Service,
) *Handler {
	return &Handler{
		authService:       auth,
		orderService:      order,
		balanceService:    balance,
		withdrawalService: withdrawal,
	}
}
