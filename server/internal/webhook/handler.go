package webhook

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/oceanplexian/lwts/server/internal/db"
)

// Handler serves webhook CRUD and delivery endpoints.
type Handler struct {
	store      *Store
	dispatcher *Dispatcher
}

func NewHandler(store *Store, dispatcher *Dispatcher) *Handler {
	return &Handler{store: store, dispatcher: dispatcher}
}

// RegisterRoutes mounts webhook routes onto the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/boards/{boardID}/webhooks", h.Create)
	mux.HandleFunc("GET /api/v1/boards/{boardID}/webhooks", h.List)
	mux.HandleFunc("GET /api/v1/boards/{boardID}/webhooks/{webhookID}", h.Get)
	mux.HandleFunc("PATCH /api/v1/boards/{boardID}/webhooks/{webhookID}", h.Update)
	mux.HandleFunc("DELETE /api/v1/boards/{boardID}/webhooks/{webhookID}", h.Delete)
	mux.HandleFunc("POST /api/v1/boards/{boardID}/webhooks/{webhookID}/test", h.Test)
	mux.HandleFunc("GET /api/v1/boards/{boardID}/webhooks/{webhookID}/deliveries", h.Deliveries)
}

type createRequest struct {
	URL       string `json:"url"`
	EventType string `json:"event_type"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	boardID := r.PathValue("boardID")

	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.URL == "" || req.EventType == "" {
		writeError(w, http.StatusBadRequest, "url and event_type are required")
		return
	}

	// Validate URL has scheme
	u, err := url.Parse(req.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		writeError(w, http.StatusBadRequest, "url must have http or https scheme")
		return
	}

	wh, err := h.store.CreateWebhook(r.Context(), boardID, req.URL, req.EventType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create webhook")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(wh)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	boardID := r.PathValue("boardID")

	webhooks, err := h.store.ListWebhooks(r.Context(), boardID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list webhooks")
		return
	}
	if webhooks == nil {
		webhooks = []Webhook{}
	}

	// Mask secrets
	for i := range webhooks {
		webhooks[i].Secret = maskSecret(webhooks[i].Secret)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webhooks)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	webhookID := r.PathValue("webhookID")

	wh, err := h.store.GetWebhook(r.Context(), webhookID)
	if err == db.ErrNoRows {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get webhook")
		return
	}

	wh.Secret = maskSecret(wh.Secret)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(wh)
}

type updateRequest struct {
	URL       *string `json:"url"`
	EventType *string `json:"event_type"`
	Enabled   *bool   `json:"enabled"`
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	webhookID := r.PathValue("webhookID")

	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	wh, err := h.store.UpdateWebhook(r.Context(), webhookID, WebhookUpdate(req))
	if err == db.ErrNoRows {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update webhook")
		return
	}

	wh.Secret = maskSecret(wh.Secret)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(wh)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	webhookID := r.PathValue("webhookID")

	err := h.store.DeleteWebhook(r.Context(), webhookID)
	if err == db.ErrNoRows {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete webhook")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Test(w http.ResponseWriter, r *http.Request) {
	webhookID := r.PathValue("webhookID")

	wh, err := h.store.GetWebhook(r.Context(), webhookID)
	if err == db.ErrNoRows {
		writeError(w, http.StatusNotFound, "webhook not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get webhook")
		return
	}

	// Emit a test event directly
	h.dispatcher.Emit(wh.BoardID, EventWebhookTest, map[string]string{
		"webhook_id": wh.ID,
		"message":    "This is a test webhook delivery",
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "test event queued"})
}

func (h *Handler) Deliveries(w http.ResponseWriter, r *http.Request) {
	webhookID := r.PathValue("webhookID")

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	deliveries, err := h.store.ListDeliveries(r.Context(), webhookID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list deliveries")
		return
	}
	if deliveries == nil {
		deliveries = []Delivery{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(deliveries)
}

func maskSecret(s string) string {
	if len(s) <= 8 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + strings.Repeat("*", len(s)-8) + s[len(s)-4:]
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
