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
	MigrateTo  string  `json:"migrate_to,omitempty"` // target column for cards in removed columns
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

	// Validate columns if provided
	if req.Columns != nil {
		newCols, err := repo.ParseColumns(*req.Columns)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid columns JSON")
			return
		}
		if len(newCols) < 2 {
			writeErr(w, http.StatusBadRequest, "board must have at least 2 columns")
			return
		}
		seen := map[string]bool{}
		for _, c := range newCols {
			if c.ID == "" || c.Label == "" {
				writeErr(w, http.StatusBadRequest, "each column must have a non-empty id and label")
				return
			}
			if seen[c.ID] {
				writeErr(w, http.StatusBadRequest, "duplicate column id: "+c.ID)
				return
			}
			seen[c.ID] = true
		}

		// Auto-assign types: first = start, last = done, rest = active
		for i := range newCols {
			switch {
			case i == 0:
				newCols[i].Type = "start"
			case i == len(newCols)-1:
				newCols[i].Type = "done"
			default:
				if newCols[i].Type == "" || newCols[i].Type == "start" || newCols[i].Type == "done" {
					newCols[i].Type = "active"
				}
			}
		}

		// Check for removed columns that still have cards
		oldCols, _ := repo.ParseColumns(b.Columns)
		newColSet := map[string]bool{}
		for _, c := range newCols {
			newColSet[c.ID] = true
		}

		cards, _ := h.cards.ListByBoard(r.Context(), id)
		cardCounts := map[string]int{}
		for _, c := range cards {
			cardCounts[c.ColumnID]++
		}

		for _, oc := range oldCols {
			if !newColSet[oc.ID] && cardCounts[oc.ID] > 0 {
				if req.MigrateTo == "" {
					writeJSON(w, http.StatusConflict, map[string]any{
						"error":      "column_has_cards",
						"column_id":  oc.ID,
						"card_count": cardCounts[oc.ID],
						"message":    "Column has cards — provide migrate_to or move cards first",
					})
					return
				}
				if !newColSet[req.MigrateTo] {
					writeErr(w, http.StatusBadRequest, "migrate_to column does not exist in new columns")
					return
				}
				// Bulk migrate cards from removed column to target
				if _, err := h.cards.MigrateColumn(r.Context(), id, oc.ID, req.MigrateTo); err != nil {
					writeErr(w, http.StatusInternalServerError, "failed to migrate cards")
					return
				}
			}
		}

		// Re-serialize with enforced types
		colJSON, _ := json.Marshal(newCols)
		s := string(colJSON)
		req.Columns = &s
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
