package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/buharamanya/gophermart/internal/logger"
	"github.com/buharamanya/gophermart/internal/repository"
	"go.uber.org/zap"
)

type UserCredentials struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (h *Handler) RegisterUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var creds UserCredentials
		if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			logger.Log.Error("Invalid request format", zap.Error(err))
			return
		}

		token, err := h.authService.Register(r.Context(), creds.Login, creds.Password)
		if err != nil {
			if err == repository.ErrUserExists {
				http.Error(w, "Login already exists", http.StatusConflict)
				logger.Log.Error("Login already exists", zap.Error(err))
				return
			}
			http.Error(w, "Failed to create user", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Authorization", "Bearer "+token)
		w.WriteHeader(http.StatusOK)
	}
}

func (h *Handler) LoginUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var creds UserCredentials
		if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
			http.Error(w, "Invalid request format", http.StatusBadRequest)
			return
		}

		token, err := h.authService.Login(r.Context(), creds.Login, creds.Password)
		if err != nil {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Authorization", "Bearer "+token)
		w.WriteHeader(http.StatusOK)
	}
}
