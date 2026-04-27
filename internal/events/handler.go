package events

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// Ingester is the surface the handler depends on. Defined here (the consumer)
// rather than in repository.go so handler tests can supply a fake without
// importing pgx. This is the standard "accept interfaces, return structs"
// pattern in Go.
type Ingester interface {
	Insert(ctx context.Context, e Event) error
}

type Handler struct {
	repo Ingester
}

func NewHandler(repo Ingester) *Handler {
	return &Handler{repo: repo}
}

type ingestRequest struct {
	UserID     string          `json:"user_id"`
	EventType  string          `json:"event_type"`
	Properties json.RawMessage `json:"properties"`
	CreatedAt  *time.Time      `json:"created_at,omitempty"`
}

func (h *Handler) Ingest(w http.ResponseWriter, r *http.Request) {
	var req ingestRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}
	if err := req.validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ev := Event{
		UserID:     req.UserID,
		EventType:  req.EventType,
		Properties: req.Properties,
	}
	if req.CreatedAt != nil {
		ev.CreatedAt = req.CreatedAt.UTC()
	}

	if err := h.repo.Insert(r.Context(), ev); err != nil {
		slog.Error("insert failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (req ingestRequest) validate() error {
	if req.UserID == "" {
		return errors.New("user_id is required")
	}
	if req.EventType == "" {
		return errors.New("event_type is required")
	}
	if len(req.UserID) > 128 {
		return errors.New("user_id too long")
	}
	if len(req.EventType) > 64 {
		return errors.New("event_type too long")
	}
	return nil
}
