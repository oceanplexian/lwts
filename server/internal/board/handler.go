package board

import (
	"encoding/json"
	"net/http"

	"github.com/oceanplexian/lwts/server/internal/auth"
	"github.com/oceanplexian/lwts/server/internal/repo"
	"github.com/oceanplexian/lwts/server/internal/sse"
)

func broadcast(hub *sse.Hub, boardID, eventType string, payload any) {
	if hub == nil {
		return
	}
	data, _ := json.Marshal(payload)
	hub.Broadcast <- &sse.BoardEvent{
		BoardID:   boardID,
		EventType: eventType,
		Data:      data,
	}
}

type Handler struct {
	boards   *repo.BoardRepository
	cards    *repo.CardRepository
	comments *repo.CommentRepository
	hub      *sse.Hub
}

func NewHandler(boards *repo.BoardRepository, cards *repo.CardRepository, comments *repo.CommentRepository, hub *sse.Hub) *Handler {
	return &Handler{boards: boards, cards: cards, comments: comments, hub: hub}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	mux.Handle("POST /api/v1/boards", authMW(http.HandlerFunc(h.Create)))
	mux.Handle("GET /api/v1/boards", authMW(http.HandlerFunc(h.List)))
	mux.Handle("GET /api/v1/boards/{id}", authMW(http.HandlerFunc(h.Get)))
	mux.Handle("PUT /api/v1/boards/{id}", authMW(http.HandlerFunc(h.Update)))
	mux.Handle("DELETE /api/v1/boards/{id}", authMW(http.HandlerFunc(h.Delete)))
}

type createBoardReq struct {
	Name       string `json:"name"`
	ProjectKey string `json:"project_key"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	var req createBoardReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeValidation(w, map[string]string{"name": "required"})
		return
	}
	if req.ProjectKey == "" {
		req.ProjectKey = "LWTS"
	}

	b, err := h.boards.Create(r.Context(), req.Name, req.ProjectKey, user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	boards, err := h.boards.List(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if boards == nil {
		boards = []repo.Board{}
	}
	writeJSON(w, http.StatusOK, boards)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	b, err := h.boards.GetByID(r.Context(), id)
	if err == repo.ErrNotFound {
		writeErr(w, http.StatusNotFound, "board not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Include card counts per column
	cards, _ := h.cards.ListByBoard(r.Context(), id)
	counts := map[string]int{}
	for _, c := range cards {
		counts[c.ColumnID]++
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"board":       b,
		"card_counts": counts,
	})
}

type updateBoardReq struct {
	Name       *string `json:"name"`
	ProjectKey *string `json:"project_key"`
	Columns    *string `json:"columns"`
	Settings   *string `json:"settings"`
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id := r.PathValue("id")

	b, err := h.boards.GetByID(r.Context(), id)
	if err == repo.ErrNotFound {
		writeErr(w, http.StatusNotFound, "board not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if b.OwnerID != user.ID && auth.RoleLevel(user.Role) < auth.RoleLevel("admin") {
		writeErr(w, http.StatusForbidden, "only board owner or admin can update")
		return
	}

	var req updateBoardReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updated, err := h.boards.Update(r.Context(), id, repo.BoardUpdate{
		Name:       req.Name,
		ProjectKey: req.ProjectKey,
		Columns:    req.Columns,
		Settings:   req.Settings,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	broadcast(h.hub, id, "board_updated", updated)
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id := r.PathValue("id")

	b, err := h.boards.GetByID(r.Context(), id)
	if err == repo.ErrNotFound {
		writeErr(w, http.StatusNotFound, "board not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if b.OwnerID != user.ID && auth.RoleLevel(user.Role) < auth.RoleLevel("admin") {
		writeErr(w, http.StatusForbidden, "only board owner or admin can delete")
		return
	}

	if err := h.boards.Delete(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	broadcast(h.hub, id, "board_deleted", map[string]string{"id": id})
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeValidation(w http.ResponseWriter, fields map[string]string) {
	writeJSON(w, http.StatusBadRequest, map[string]any{"error": "validation failed", "fields": fields})
}
