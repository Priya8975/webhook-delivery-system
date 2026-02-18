package api

import (
	"net/http"

	"github.com/Priya8975/webhook-delivery-system/internal/engine"
	"github.com/Priya8975/webhook-delivery-system/internal/store"
	ws "github.com/Priya8975/webhook-delivery-system/internal/websocket"
)

type DashboardHandler struct {
	store  *store.PostgresStore
	fanout *engine.FanOutEngine
	cb     *engine.CircuitBreaker
	hub    *ws.Hub
}

func NewDashboardHandler(s *store.PostgresStore, f *engine.FanOutEngine, cb *engine.CircuitBreaker, hub *ws.Hub) *DashboardHandler {
	return &DashboardHandler{store: s, fanout: f, cb: cb, hub: hub}
}

// Metrics returns aggregated system metrics for the dashboard.
func (h *DashboardHandler) Metrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := h.store.GetDeliveryMetrics(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get metrics")
		return
	}

	// Get queue depth from Redis
	queueDepth, err := h.fanout.QueueDepth(r.Context())
	if err != nil {
		queueDepth = 0
	}

	type metricsResponse struct {
		store.DeliveryMetrics
		QueueDepth       int64 `json:"queue_depth"`
		WebSocketClients int   `json:"websocket_clients"`
	}

	respondJSON(w, http.StatusOK, metricsResponse{
		DeliveryMetrics:  *metrics,
		QueueDepth:       queueDepth,
		WebSocketClients: h.hub.ClientCount(),
	})
}

// SubscriberHealth returns health info for all active subscribers including circuit breaker state.
func (h *DashboardHandler) SubscriberHealth(w http.ResponseWriter, r *http.Request) {
	subscribers, err := h.store.ListSubscribers(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list subscribers")
		return
	}

	type subscriberHealth struct {
		ID             string                    `json:"id"`
		Name           string                    `json:"name"`
		EndpointURL    string                    `json:"endpoint_url"`
		IsActive       bool                      `json:"is_active"`
		CircuitBreaker engine.CircuitBreakerState `json:"circuit_breaker"`
	}

	result := make([]subscriberHealth, 0, len(subscribers))
	for _, sub := range subscribers {
		cbState := h.cb.GetState(r.Context(), sub.ID)
		result = append(result, subscriberHealth{
			ID:             sub.ID,
			Name:           sub.Name,
			EndpointURL:    sub.EndpointURL,
			IsActive:       sub.IsActive,
			CircuitBreaker: cbState,
		})
	}

	respondJSON(w, http.StatusOK, result)
}
