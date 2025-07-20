package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/buharamanya/gophermart/internal/logger"
	"github.com/buharamanya/gophermart/internal/repository"
	"github.com/buharamanya/gophermart/internal/util"
	"go.uber.org/zap"
)

// GetBalance возвращает текущий баланс пользователя
func (h *Handler) GetBalance() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := getUserID(r.Context())

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			logger.Log.Error("failed to get userID from context", zap.Error(err))
			return
		}

		balance, err := h.balanceService.GetBalance(r.Context(), userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			logger.Log.Error("failed to get balance", zap.Error(err))
			return
		}

		response := map[string]float64{
			"current":   balance.Current,
			"withdrawn": balance.Withdrawn,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// Withdraw списывает средства с баланса
func (h *Handler) AddWithdraw() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Order string  `json:"order"`
			Sum   float64 `json:"sum"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request format", http.StatusBadRequest)
			return
		}

		userID, err := getUserID(r.Context())

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			logger.Log.Error("failed to get userID from context", zap.Error(err))
			return
		}

		err = h.balanceService.Withdraw(r.Context(), userID, req.Order, req.Sum)
		switch {
		case err == nil:
			w.WriteHeader(http.StatusOK)
		case errors.Is(err, util.ErrInvalidOrderNumber):
			http.Error(w, "invalid order number", http.StatusUnprocessableEntity)
		case errors.Is(err, repository.ErrInsufficientBalance):
			http.Error(w, "insufficient funds", http.StatusPaymentRequired)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
