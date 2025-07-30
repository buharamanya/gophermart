package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/buharamanya/gophermart/internal/logger"
	"go.uber.org/zap"
)

// GetWithdrawals возвращает историю списаний
func (h *Handler) GetWithdrawals() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		userID, err := getUserID(r.Context())
		if err != nil {
			logger.Log.Error("failed to get userID from context", zap.Error(err))
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		withdrawals, err := h.withdrawalService.GetWithdrawals(r.Context(), userID)
		if err != nil {
			logger.Log.Error("failed to get withdrawals", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		if len(withdrawals) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		response := make([]map[string]interface{}, len(withdrawals))
		for i, wd := range withdrawals {
			response[i] = map[string]interface{}{
				"order":        wd.OrderNumber,
				"sum":          wd.Sum,
				"processed_at": wd.ProcessedAt.Format(time.RFC3339),
			}
		}

		w.Header().Set("Content-Type", "application/json")

		// Создаем encoder и устанавливаем параметры
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false) // Отключаем экранирование HTML-символов

		if err := enc.Encode(response); err != nil {
			logger.Log.Error("failed to encode response",
				zap.Error(err),
				zap.Any("withdrawals", withdrawals))
		}
	}
}
