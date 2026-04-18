package embed

import (
	"encoding/json"
	"net/http"
)

// Handler exposes status + backfill endpoints for the embeddings feature.
// Routes are admin-only and registered alongside other settings routes.
type Handler struct {
	svc           *Service
	pgvectorReady bool
	dim           int
}

func NewHandler(svc *Service, pgvectorReady bool, dim int) *Handler {
	return &Handler{svc: svc, pgvectorReady: pgvectorReady, dim: dim}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMW, adminMW func(http.Handler) http.Handler) {
	// Status is auth-only (members can see whether semantic search is on).
	mux.Handle("GET /api/v1/embed/status", authMW(http.HandlerFunc(h.Status)))
	// Backfill mutates state — admin only.
	mux.Handle("POST /api/v1/embed/backfill", adminMW(http.HandlerFunc(h.Backfill)))
}

// StatusResponse is the public-facing health check for the feature.
type StatusResponse struct {
	Configured       bool   `json:"configured"`        // EMBEDDING_API_URL is set
	PgvectorReady    bool   `json:"pgvector_ready"`    // schema provisioned
	Available        bool   `json:"available"`         // both above are true
	Model            string `json:"model"`             // configured embedding model id
	Dim              int    `json:"dim"`               // vector dimension
	CardsWithEmbed   int    `json:"cards_with_embed"`  // count of embedded cards
	CardsTotal       int    `json:"cards_total"`       // total card count
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	resp := StatusResponse{
		Configured:    h.svc.Configured(),
		PgvectorReady: h.pgvectorReady,
		Model:         h.svc.Model(),
		Dim:           h.dim,
	}
	resp.Available = resp.Configured && resp.PgvectorReady
	if h.svc != nil && h.pgvectorReady {
		w_, t, err := h.svc.Counts(r.Context())
		if err == nil {
			resp.CardsWithEmbed = w_
			resp.CardsTotal = t
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// Backfill embeds all cards that don't currently have an embedding. Synchronous
// for now — typical workspaces are small enough that a 30s wait is acceptable
// and the feedback is more useful than fire-and-forget.
func (h *Handler) Backfill(w http.ResponseWriter, r *http.Request) {
	if h.svc == nil || !h.svc.Configured() {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "embedding endpoint not configured (set EMBEDDING_API_URL)",
		})
		return
	}
	if !h.pgvectorReady {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "pgvector not available — install the extension and restart the server",
		})
		return
	}
	embedded, skipped, err := h.svc.Backfill(r.Context(), 32)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"embedded": embedded,
		"skipped":  skipped,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
