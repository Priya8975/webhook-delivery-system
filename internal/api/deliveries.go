package api

import (
	"net/http"
	"strconv"

	"github.com/Priya8975/webhook-delivery-system/internal/store"
	"github.com/go-chi/chi/v5"
)

type DeliveryHandler struct {
	store *store.PostgresStore
}

func NewDeliveryHandler(s *store.PostgresStore) *DeliveryHandler {
	return &DeliveryHandler{store: s}
}

func (h *DeliveryHandler) List(w http.ResponseWriter, r *http.Request) {
	eventID := r.URL.Query().Get("event_id")
	subscriberID := r.URL.Query().Get("subscriber_id")
	status := r.URL.Query().Get("status")
	limitStr := r.URL.Query().Get("limit")

	limit := 50
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}

	attempts, err := h.store.ListDeliveryAttempts(r.Context(), eventID, subscriberID, status, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list delivery attempts")
		return
	}

	respondJSON(w, http.StatusOK, attempts)
}

func (h *DeliveryHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	attempt, err := h.store.GetDeliveryAttempt(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get delivery attempt")
		return
	}
	if attempt == nil {
		respondError(w, http.StatusNotFound, "delivery attempt not found")
		return
	}

	respondJSON(w, http.StatusOK, attempt)
}
