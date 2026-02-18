package api

import (
	"encoding/json"
	"net/http"

	"github.com/Priya8975/webhook-delivery-system/internal/domain"
	"github.com/Priya8975/webhook-delivery-system/internal/engine"
	"github.com/Priya8975/webhook-delivery-system/internal/store"
	"github.com/go-chi/chi/v5"
)

type SubscriberHandler struct {
	store          *store.PostgresStore
	circuitBreaker *engine.CircuitBreaker
}

func NewSubscriberHandler(s *store.PostgresStore, cb *engine.CircuitBreaker) *SubscriberHandler {
	return &SubscriberHandler{store: s, circuitBreaker: cb}
}

func (h *SubscriberHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateSubscriberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.EndpointURL == "" {
		respondError(w, http.StatusBadRequest, "endpoint_url is required")
		return
	}
	if len(req.EventTypes) == 0 {
		respondError(w, http.StatusBadRequest, "at least one event_type is required")
		return
	}

	sub, err := h.store.CreateSubscriber(r.Context(), req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create subscriber")
		return
	}

	respondJSON(w, http.StatusCreated, domain.CreateSubscriberResponse{
		ID:        sub.ID,
		Name:      sub.Name,
		SecretKey: sub.SecretKey,
	})
}

func (h *SubscriberHandler) List(w http.ResponseWriter, r *http.Request) {
	subscribers, err := h.store.ListSubscribers(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list subscribers")
		return
	}

	respondJSON(w, http.StatusOK, subscribers)
}

func (h *SubscriberHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	sub, err := h.store.GetSubscriber(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get subscriber")
		return
	}
	if sub == nil {
		respondError(w, http.StatusNotFound, "subscriber not found")
		return
	}

	// Get subscriptions for this subscriber
	subscriptions, err := h.store.GetSubscriberSubscriptions(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get subscriptions")
		return
	}

	type subscriberDetail struct {
		domain.Subscriber
		Subscriptions []domain.Subscription `json:"subscriptions"`
	}

	respondJSON(w, http.StatusOK, subscriberDetail{
		Subscriber:    *sub,
		Subscriptions: subscriptions,
	})
}

func (h *SubscriberHandler) Health(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	sub, err := h.store.GetSubscriber(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get subscriber")
		return
	}
	if sub == nil {
		respondError(w, http.StatusNotFound, "subscriber not found")
		return
	}

	cbState := h.circuitBreaker.GetState(r.Context(), id)

	type healthResponse struct {
		SubscriberID   string                       `json:"subscriber_id"`
		Name           string                       `json:"name"`
		EndpointURL    string                       `json:"endpoint_url"`
		IsActive       bool                         `json:"is_active"`
		CircuitBreaker engine.CircuitBreakerState    `json:"circuit_breaker"`
	}

	respondJSON(w, http.StatusOK, healthResponse{
		SubscriberID:   sub.ID,
		Name:           sub.Name,
		EndpointURL:    sub.EndpointURL,
		IsActive:       sub.IsActive,
		CircuitBreaker: cbState,
	})
}

func (h *SubscriberHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req domain.UpdateSubscriberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	sub, err := h.store.UpdateSubscriber(r.Context(), id, req)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update subscriber")
		return
	}
	if sub == nil {
		respondError(w, http.StatusNotFound, "subscriber not found")
		return
	}

	respondJSON(w, http.StatusOK, sub)
}
