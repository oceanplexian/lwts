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
	Configured     bool             `json:"configured"`      // EMBEDDING_API_URL is set
	PgvectorReady  bool             `json:"pgvector_ready"`  // schema provisioned
	Available      bool             `json:"available"`       // both above are true
	Model          string           `json:"model"`           // configured embedding model id
	Dim            int              `json:"dim"`             // vector dimension
	CardsWithEmbed int              `json:"cards_with_embed"` // count of embedded cards
	CardsTotal     int              `json:"cards_total"`      // total card count
	Backfill       BackfillProgress `json:"backfill"`         // last/current backfill state
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
		resp.Backfill = h.svc.BackfillProgress()
	}
	writeJSON(w, http.StatusOK, resp)
}

// Backfill kicks off an async backfill and returns immediately. Poll
// /api/v1/embed/status to watch progress. Returns 409 if one is already
// running so the UI can react sensibly.
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
	if err := h.svc.StartBackfill(32); err != nil {
		if err == ErrBackfillRunning {
			writeJSON(w, http.StatusConflict, map[string]string{
				"error": "backfill already running; poll /api/v1/embed/status for progress",
			})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{
		"status": "backfill started; poll /api/v1/embed/status for progress",
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
