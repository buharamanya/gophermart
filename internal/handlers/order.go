package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/buharamanya/gophermart/internal/logger"
	"github.com/buharamanya/gophermart/internal/repository"
	"github.com/buharamanya/gophermart/internal/service/order"
	"go.uber.org/zap"
)

func (h *Handler) AddOrder() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			logger.Log.Error(readReqErrStr, zap.Error(err))
			return
		}

		var userID int

		if userID, err = getUserID(r.Context()); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			logger.Log.Error("failed to get userID from context", zap.Error(err))
			return
		}

		err = h.orderService.UploadOrder(r.Context(), userID, string(body))
		if err != nil {
			if errors.Is(err, order.ErrInvalidOrderNumber) {
				w.WriteHeader(http.StatusUnprocessableEntity)
				return
			}

			if errors.Is(err, repository.ErrOrderNotOwned) {
				w.WriteHeader(http.StatusConflict)
				return
			}

			if errors.Is(err, repository.ErrOrderExists) {
				w.WriteHeader(http.StatusOK)
				return
			}

			w.WriteHeader(http.StatusInternalServerError)
			logger.Log.Error("failed to add order", zap.Error(err))
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}

func (h *Handler) GetOrders() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		var userID, err = getUserID(r.Context())

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			logger.Log.Error("failed to get userID from context", zap.Error(err))
			return
		}

		orders, err := h.orderService.GetOrders(r.Context(), userID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			logger.Log.Error("failed to get orders", zap.Error(err))
			return
		}

		if len(orders) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.Header().Set(ContentTypeHeader, JSONContentType)
		w.WriteHeader(http.StatusOK)

		enc := json.NewEncoder(w)
		if err := enc.Encode(orders); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			logger.Log.Error("failed encoding orders responce", zap.Error(err))
			return
		}
	}
}
