package comment

import (
	"encoding/json"
	"net/http"

	"github.com/oceanplexian/lwts/server/internal/auth"
	"github.com/oceanplexian/lwts/server/internal/discord"
	"github.com/oceanplexian/lwts/server/internal/repo"
	"github.com/oceanplexian/lwts/server/internal/sse"
)

type Handler struct {
	comments *repo.CommentRepository
	cards    *repo.CardRepository
	boards   *repo.BoardRepository
	hub      *sse.Hub
	discord  *discord.Notifier
}

func NewHandler(comments *repo.CommentRepository, cards *repo.CardRepository, hub *sse.Hub) *Handler {
	return &Handler{comments: comments, cards: cards, hub: hub}
}

func (h *Handler) SetBoards(b *repo.BoardRepository) { h.boards = b }
func (h *Handler) SetDiscord(d *discord.Notifier)     { h.discord = d }

func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMW func(http.Handler) http.Handler, memberMW func(http.Handler) http.Handler) {
	mux.Handle("POST /api/v1/cards/{cardId}/comments", memberMW(http.HandlerFunc(h.Create)))
	mux.Handle("GET /api/v1/cards/{cardId}/comments", authMW(http.HandlerFunc(h.ListByCard)))
	mux.Handle("PUT /api/v1/comments/{id}", authMW(http.HandlerFunc(h.Update)))
	mux.Handle("DELETE /api/v1/comments/{id}", authMW(http.HandlerFunc(h.Delete)))
}

type createCommentReq struct {
	Body string `json:"body"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	cardID := r.PathValue("cardId")

	var req createCommentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Body == "" {
		writeValidation(w, map[string]string{"body": "required"})
		return
	}

	// Verify card exists and get board ID for SSE
	card, err := h.cards.GetByID(r.Context(), cardID)
	if err == repo.ErrNotFound {
		writeErr(w, http.StatusNotFound, "card not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	cmt, err := h.comments.Create(r.Context(), cardID, user.ID, req.Body)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	broadcast(h.hub, card.BoardID, "comment_added", cmt)

	// Discord notification for new comment
	if h.discord != nil && h.boards != nil {
		board, _ := h.boards.GetByID(r.Context(), card.BoardID)
		u := auth.UserFromContext(r.Context())
		actor := repo.User{}
		if u != nil {
			actor = *u
		}
		h.discord.Emit(discord.Event{
			Type: discord.EventCommentAdded, Card: card, Board: board, User: actor,
			Comment: &cmt,
		})
	}

	writeJSON(w, http.StatusCreated, cmt)
}

func (h *Handler) ListByCard(w http.ResponseWriter, r *http.Request) {
	cardID := r.PathValue("cardId")
	comments, err := h.comments.ListByCard(r.Context(), cardID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if comments == nil {
		comments = []repo.Comment{}
	}
	writeJSON(w, http.StatusOK, comments)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id := r.PathValue("id")

	cmt, err := h.comments.GetByID(r.Context(), id)
	if err == repo.ErrNotFound {
		writeErr(w, http.StatusNotFound, "comment not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if cmt.AuthorID != user.ID && auth.RoleLevel(user.Role) < auth.RoleLevel("admin") {
		writeErr(w, http.StatusForbidden, "can only edit your own comments")
		return
	}

	var req createCommentReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Body == "" {
		writeValidation(w, map[string]string{"body": "required"})
		return
	}

	updated, err := h.comments.Update(r.Context(), id, req.Body)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	card, _ := h.cards.GetByID(r.Context(), cmt.CardID)
	if card.BoardID != "" {
		broadcast(h.hub, card.BoardID, "comment_updated", updated)
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id := r.PathValue("id")

	cmt, err := h.comments.GetByID(r.Context(), id)
	if err == repo.ErrNotFound {
		writeErr(w, http.StatusNotFound, "comment not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Only the author or admin+ can delete
	if cmt.AuthorID != user.ID && auth.RoleLevel(user.Role) < auth.RoleLevel("admin") {
		writeErr(w, http.StatusForbidden, "can only delete your own comments")
		return
	}

	if err := h.comments.Delete(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Broadcast via SSE
	card, _ := h.cards.GetByID(r.Context(), cmt.CardID)
	if card.BoardID != "" {
		broadcast(h.hub, card.BoardID, "comment_deleted", map[string]string{"id": id, "card_id": cmt.CardID})
	}

	w.WriteHeader(http.StatusNoContent)
}

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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeValidation(w http.ResponseWriter, fields map[string]string) {
	writeJSON(w, http.StatusBadRequest, map[string]any{"error": "validation failed", "fields": fields})
}
