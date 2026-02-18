package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Priya8975/webhook-delivery-system/internal/store"
	"github.com/go-chi/chi/v5"
)

type DeadLetterHandler struct {
	store *store.PostgresStore
}

func NewDeadLetterHandler(s *store.PostgresStore) *DeadLetterHandler {
	return &DeadLetterHandler{store: s}
}

func (h *DeadLetterHandler) List(w http.ResponseWriter, r *http.Request) {
	subscriberID := r.URL.Query().Get("subscriber_id")
	resolvedStr := r.URL.Query().Get("resolved")
	limitStr := r.URL.Query().Get("limit")

	resolved := false
	if resolvedStr == "true" {
		resolved = true
	}

	limit := 50
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}

	letters, err := h.store.ListDeadLetters(r.Context(), subscriberID, resolved, limit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list dead letters")
		return
	}

	respondJSON(w, http.StatusOK, letters)
}

func (h *DeadLetterHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	letter, err := h.store.GetDeadLetter(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to get dead letter")
		return
	}
	if letter == nil {
		respondError(w, http.StatusNotFound, "dead letter not found")
		return
	}

	respondJSON(w, http.StatusOK, letter)
}

type resolveRequest struct {
	ResolvedBy string `json:"resolved_by"`
}

func (h *DeadLetterHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req resolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ResolvedBy == "" {
		req.ResolvedBy = "manual"
	}

	if err := h.store.ResolveDeadLetter(r.Context(), id, req.ResolvedBy); err != nil {
		respondError(w, http.StatusNotFound, "dead letter not found or already resolved")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}
