package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/buharamanya/gophermart/internal/util"
)

func (h *Handler) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		userID, err := util.ParseToken(token)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "userID", userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func getUserID(ctx context.Context) (int, error) {
	userID, ok := ctx.Value("userID").(int)
	if !ok {
		return 0, errors.New("userID not found in context")
	}
	return userID, nil
}
