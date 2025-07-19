package handlers

import (
	"github.com/buharamanya/gophermart/internal/logger"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func (h *Handler) RegisterRoutes(r *chi.Mux) {

	r.Route("/api/user", func(r chi.Router) {

		r.Use(logger.WithRequestLogging)

		r.Group(func(r chi.Router) {
			r.Use(middleware.AllowContentType("application/json"))

			r.Post("/register", h.RegisterUser())
			r.Post("/login", h.LoginUser())
		})

		r.Group(func(r chi.Router) {
			r.Use(h.Authenticate)

			r.Group(func(r chi.Router) {
				r.Use(WithGzipMiddleware)

				r.Get("/orders", h.GetOrders())
				// r.Get("/withdrawals", h.GetWithdrawals())
			})

			// r.Post("/orders", h.AddOrder())

			// r.Route("/balance", func(r chi.Router) {
			// 	r.Get("/", h.GetBalance())

			// 	r.Group(func(r chi.Router) {
			// 		r.Use(middleware.AllowContentType("application/json"))
			// 		r.Post("/withdraw", h.AddWithdraw())
			// 	})
			// })
		})
	})
}
