package api

import (
	"io/fs"
	"net/http"

	"github.com/Priya8975/webhook-delivery-system/internal/engine"
	"github.com/Priya8975/webhook-delivery-system/internal/store"
	ws "github.com/Priya8975/webhook-delivery-system/internal/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// NewRouter creates and configures the HTTP router.
func NewRouter(pgStore *store.PostgresStore, fanout *engine.FanOutEngine, cb *engine.CircuitBreaker, hub *ws.Hub, dashboardFS fs.FS) http.Handler {
	r := chi.NewRouter()

	// Middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Heartbeat("/ping"))

	// CORS for dashboard
	r.Use(corsMiddleware)

	// Handlers
	subHandler := NewSubscriberHandler(pgStore, cb)
	eventHandler := NewEventHandler(pgStore, fanout)
	deliveryHandler := NewDeliveryHandler(pgStore)
	dlqHandler := NewDeadLetterHandler(pgStore)
	dashHandler := NewDashboardHandler(pgStore, fanout, cb, hub)

	// WebSocket endpoint
	r.Get("/ws", hub.HandleWebSocket)

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", HealthHandler())

		r.Route("/subscribers", func(r chi.Router) {
			r.Post("/", subHandler.Create)
			r.Get("/", subHandler.List)
			r.Get("/{id}", subHandler.Get)
			r.Patch("/{id}", subHandler.Update)
			r.Get("/{id}/health", subHandler.Health)
		})

		r.Route("/events", func(r chi.Router) {
			r.Post("/", eventHandler.Create)
			r.Get("/", eventHandler.List)
			r.Get("/{id}", eventHandler.Get)
		})

		r.Route("/deliveries", func(r chi.Router) {
			r.Get("/", deliveryHandler.List)
			r.Get("/{id}", deliveryHandler.Get)
		})

		r.Route("/dead-letters", func(r chi.Router) {
			r.Get("/", dlqHandler.List)
			r.Get("/{id}", dlqHandler.Get)
			r.Post("/{id}/resolve", dlqHandler.Resolve)
		})

		r.Get("/metrics", dashHandler.Metrics)
		r.Get("/subscribers-health", dashHandler.SubscriberHealth)
	})

	// Serve dashboard static files
	if dashboardFS != nil {
		fileServer := http.FileServer(http.FS(dashboardFS))
		r.Handle("/*", fileServer)
	}

	return r
}

// corsMiddleware adds CORS headers for dashboard development.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
