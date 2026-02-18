package api

import (
	"net/http"

	"github.com/Priya8975/webhook-delivery-system/internal/engine"
	"github.com/Priya8975/webhook-delivery-system/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter creates and configures the HTTP router.
func NewRouter(pgStore *store.PostgresStore, fanout *engine.FanOutEngine) http.Handler {
	r := chi.NewRouter()

	// Middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Heartbeat("/ping"))

	// Handlers
	subHandler := NewSubscriberHandler(pgStore)
	eventHandler := NewEventHandler(pgStore, fanout)

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", HealthHandler())

		r.Route("/subscribers", func(r chi.Router) {
			r.Post("/", subHandler.Create)
			r.Get("/", subHandler.List)
			r.Get("/{id}", subHandler.Get)
			r.Patch("/{id}", subHandler.Update)
		})

		r.Route("/events", func(r chi.Router) {
			r.Post("/", eventHandler.Create)
			r.Get("/", eventHandler.List)
			r.Get("/{id}", eventHandler.Get)
		})
	})

	return r
}
