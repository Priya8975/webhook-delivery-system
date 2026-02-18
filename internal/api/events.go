package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Priya8975/webhook-delivery-system/internal/engine"
	"github.com/Priya8975/webhook-delivery-system/internal/store"
	"github.com/go-chi/chi/v5"
)

type EventHandler struct {
	store  *store.PostgresStore
	fanout *engine.FanOutEngine
}

func NewEventHandler(s *store.PostgresStore, f *engine.FanOutEngine) *EventHandler {
	return &EventHandler{store: s, fanout: f}
}

type createEventRequest struct {
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	Source    string          `json:"source,omitempty"`
}

type createEventResponse struct {
	EventID          string `json:"event_id"`
	EventType        string `json:"event_type"`
	DeliveriesQueued int    `json:"deliveries_queued"`
}

func (h *EventHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.EventType == "" {
		respondError(w, http.StatusBadRequest, "event_type is required")
		return
	}
	if len(req.Payload) == 0 {
		respondError(w, http.StatusBadRequest, "payload is required")
		return
	}

	// Validate payload is valid JSON
	if !json.Valid(req.Payload) {
		respondError(w, http.StatusBadRequest, "payload must be valid JSON")
		return
	}

	// Save event to PostgreSQL
	event, err := h.store.CreateEvent(r.Context(), req.EventType, req.Payload, req.Source)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create event")
		return
	}

	// Fan-out: find matching subscribers and queue delivery jobs
	queued, err := h.fanout.FanOut(r.Context(), event)
	if err != nil {
		// Event is saved but fan-out failed — log but don't fail the request
		// The event can be replayed later
		respondJSON(w, http.StatusCreated, createEventResponse{
			EventID:          event.ID,
			EventType:        event.EventType,
			DeliveriesQueued: 0,
		})
		return
	}

	respondJSON(w, http.StatusCreated, createEventResponse{
		EventID:          event.ID,
		EventType:        event.EventType,
		DeliveriesQueued: queued,
	})
}

func (h *EventHandler) List(w http.ResponseWriter, r *http.Request) {
	eventType := r.URL.Query().Get("event_type")
	limitStr := r.URL.Query().Get("limit")

	limit := 50
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}

	events, err := h.store.ListEvents(r.Context(), eventType, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list events")
		return
	}

	respondJSON(w, http.StatusOK, events)
}

func (h *EventHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	event, err := h.store.GetEvent(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get event")
		return
	}
	if event == nil {
		respondError(w, http.StatusNotFound, "event not found")
		return
	}

	respondJSON(w, http.StatusOK, event)
}
