package handlers

import (
	"github.com/buharamanya/gophermart/internal/service/auth"
	"github.com/buharamanya/gophermart/internal/service/order"
)

type Handler struct {
	authService  *auth.Service
	orderService *order.Service
}

func NewHandler(
	auth *auth.Service,
	order *order.Service,
) *Handler {
	return &Handler{
		authService:  auth,
		orderService: order,
	}
}
