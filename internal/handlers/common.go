package handlers

import (
	"github.com/buharamanya/gophermart/internal/service/auth"
	"github.com/buharamanya/gophermart/internal/service/order"
)

const (
	readReqErrStr     = "failed to read request body"
	ContentTypeHeader = "Content-Type"
	JSONContentType   = "application/json"
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
