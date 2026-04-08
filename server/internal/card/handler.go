package card

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/oceanplexian/lwts/server/internal/auth"
	"github.com/oceanplexian/lwts/server/internal/discord"
	"github.com/oceanplexian/lwts/server/internal/repo"
	"github.com/oceanplexian/lwts/server/internal/sse"
)

type Handler struct {
	cards    *repo.CardRepository
	boards   *repo.BoardRepository
	comments *repo.CommentRepository
	hub      *sse.Hub
	discord  *discord.Notifier
}

func NewHandler(cards *repo.CardRepository, boards *repo.BoardRepository, comments *repo.CommentRepository, hub *sse.Hub) *Handler {
	return &Handler{cards: cards, boards: boards, comments: comments, hub: hub}
}

func (h *Handler) SetDiscord(d *discord.Notifier) { h.discord = d }

func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMW func(http.Handler) http.Handler, memberMW func(http.Handler) http.Handler) {
	mux.Handle("POST /api/v1/boards/{boardId}/cards/bulk-move", memberMW(http.HandlerFunc(h.BulkMove)))
	mux.Handle("POST /api/v1/boards/{boardId}/cards", memberMW(http.HandlerFunc(h.Create)))
	mux.Handle("GET /api/v1/boards/{boardId}/cards", authMW(http.HandlerFunc(h.ListByBoard)))
	mux.Handle("GET /api/v1/cards/{id}", authMW(http.HandlerFunc(h.Get)))
	mux.Handle("PUT /api/v1/cards/{id}", memberMW(http.HandlerFunc(h.Update)))
	mux.Handle("POST /api/v1/cards/{id}/move", memberMW(http.HandlerFunc(h.Move)))
	mux.Handle("DELETE /api/v1/cards/{id}", memberMW(http.HandlerFunc(h.Delete)))
}

type createCardReq struct {
	ColumnID        string  `json:"column_id"`
	ClientRequestID string  `json:"client_request_id"`
	Title           string  `json:"title"`
	Description     string  `json:"description"`
	Tag             string  `json:"tag"`
	Priority        string  `json:"priority"`
	AssigneeID      *string `json:"assignee_id"`
	Points          *int    `json:"points"`
	DueDate         *string `json:"due_date"`
	EpicID          *string `json:"epic_id"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	boardID := r.PathValue("boardId")

	var req createCardReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" {
		writeValidation(w, map[string]string{"title": "required"})
		return
	}
	if req.ColumnID == "" {
		req.ColumnID = "backlog"
	}

	card, err := h.cards.Create(r.Context(), boardID, repo.CardCreate{
		ColumnID:    req.ColumnID,
		Title:       req.Title,
		Description: req.Description,
		Tag:         req.Tag,
		Priority:    req.Priority,
		AssigneeID:  req.AssigneeID,
		ReporterID:  &user.ID,
		Points:      req.Points,
		DueDate:     req.DueDate,
		EpicID:      req.EpicID,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}
	card.ClientRequestID = req.ClientRequestID

	broadcast(h.hub, boardID, "card_created", card, user.ID)

	// Discord notifications
	if h.discord != nil {
		board, _ := h.boards.GetByID(r.Context(), boardID)
		h.discord.Emit(discord.Event{
			Type: discord.EventCardCreated, Card: card, Board: board, User: derefUser(user),
		})
		if card.AssigneeID != nil && *card.AssigneeID != "" && *card.AssigneeID != user.ID {
			h.discord.Emit(discord.Event{
				Type: discord.EventCardAssigned, Card: card, Board: board, User: derefUser(user),
			})
		}
		if card.Priority == "high" || card.Priority == "urgent" {
			h.discord.Emit(discord.Event{
				Type: discord.EventCardPriority, Card: card, Board: board, User: derefUser(user),
				OldValue: "none",
			})
		}
	}

	writeJSON(w, http.StatusCreated, card)
}

func (h *Handler) ListByBoard(w http.ResponseWriter, r *http.Request) {
	boardID := r.PathValue("boardId")
	cards, err := h.cards.ListByBoard(r.Context(), boardID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if cards == nil {
		cards = []repo.Card{}
	}

	// Group by column
	grouped := map[string][]repo.Card{}
	for _, c := range cards {
		grouped[c.ColumnID] = append(grouped[c.ColumnID], c)
	}
	writeJSON(w, http.StatusOK, grouped)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	card, err := h.cards.GetByID(r.Context(), id)
	if err == repo.ErrNotFound {
		writeErr(w, http.StatusNotFound, "card not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	comments, _ := h.comments.ListByCard(r.Context(), id)
	if comments == nil {
		comments = []repo.Comment{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"card":     card,
		"comments": comments,
	})
}

type updateCardReq struct {
	Title          *string  `json:"title"`
	Description    *string  `json:"description"`
	Tag            *string  `json:"tag"`
	Priority       *string  `json:"priority"`
	AssigneeID     **string `json:"assignee_id"`
	Points         **int    `json:"points"`
	DueDate        **string `json:"due_date"`
	RelatedCardIDs *string  `json:"related_card_ids"`
	BlockedCardIDs *string  `json:"blocked_card_ids"`
	EpicID         *string  `json:"epic_id"` // present = set (empty string = clear)
	Version        int      `json:"version"`
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Snapshot before update for discord diff
	oldCard, _ := h.cards.GetByID(r.Context(), id)

	var req updateCardReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	upd := repo.CardUpdate{
		Title:          req.Title,
		Description:    req.Description,
		Tag:            req.Tag,
		Priority:       req.Priority,
		AssigneeID:     req.AssigneeID,
		Points:         req.Points,
		DueDate:        req.DueDate,
		RelatedCardIDs: req.RelatedCardIDs,
		BlockedCardIDs: req.BlockedCardIDs,
	}
	if req.EpicID != nil {
		var epicVal *string
		if *req.EpicID != "" {
			epicVal = req.EpicID
		}
		upd.EpicID = &epicVal
	}
	card, err := h.cards.Update(r.Context(), id, req.Version, upd)
	if err == repo.ErrNotFound {
		writeErr(w, http.StatusNotFound, "card not found")
		return
	}
	if err == repo.ErrConflict {
		// Return current version so client can reconcile
		current, _ := h.cards.GetByID(r.Context(), id)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":   "version conflict",
			"current": current,
		})
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	user := auth.UserFromContext(r.Context())
	broadcast(h.hub, card.BoardID, "card_updated", card, user.ID)

	// Discord notifications for field changes
	if h.discord != nil {
		board, _ := h.boards.GetByID(r.Context(), card.BoardID)

		// Assignee changed
		oldAssignee := ""
		if oldCard.AssigneeID != nil {
			oldAssignee = *oldCard.AssigneeID
		}
		newAssignee := ""
		if card.AssigneeID != nil {
			newAssignee = *card.AssigneeID
		}
		if newAssignee != "" && newAssignee != oldAssignee {
			h.discord.Emit(discord.Event{
				Type: discord.EventCardAssigned, Card: card, Board: board, User: derefUser(user),
			})
		}

		// Priority escalated to high/urgent
		if (card.Priority == "high" || card.Priority == "urgent") && card.Priority != oldCard.Priority {
			h.discord.Emit(discord.Event{
				Type: discord.EventCardPriority, Card: card, Board: board, User: derefUser(user),
				OldValue: oldCard.Priority,
			})
		}
	}

	writeJSON(w, http.StatusOK, card)
}

type moveCardReq struct {
	ColumnID string  `json:"column_id"`
	Position int     `json:"position"`
	Version  int     `json:"version"`
	EpicID   *string `json:"epic_id"` // present = set epic (empty string = clear)
}

func (h *Handler) Move(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req moveCardReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ColumnID == "" {
		writeValidation(w, map[string]string{"column_id": "required"})
		return
	}

	// Get current card to check transition rules
	current, err := h.cards.GetByID(r.Context(), id)
	if err == repo.ErrNotFound {
		writeErr(w, http.StatusNotFound, "card not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Only check rules if the column is actually changing
	if current.ColumnID != req.ColumnID {
		if blockers := h.checkTransitionRules(r.Context(), current, req.ColumnID); len(blockers) > 0 {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"error":    "transition_blocked",
				"message":  fmt.Sprintf("Cannot move to %s", req.ColumnID),
				"blockers": blockers,
			})
			return
		}
	}

	var moveOpts []repo.MoveOption
	if req.EpicID != nil {
		var epicVal *string
		if *req.EpicID != "" {
			epicVal = req.EpicID
		}
		moveOpts = append(moveOpts, repo.MoveOption{EpicID: &epicVal})
	}
	card, err := h.cards.Move(r.Context(), id, req.Version, req.ColumnID, req.Position, moveOpts...)
	if err == repo.ErrNotFound {
		writeErr(w, http.StatusNotFound, "card not found")
		return
	}
	if err == repo.ErrConflict {
		current, _ := h.cards.GetByID(r.Context(), id)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "version conflict", "current": current})
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	user := auth.UserFromContext(r.Context())
	broadcast(h.hub, card.BoardID, "card_moved", card, user.ID)

	// Discord: detect move to done column
	if h.discord != nil && current.ColumnID != req.ColumnID {
		board, _ := h.boards.GetByID(r.Context(), card.BoardID)
		if req.ColumnID == "done" {
			h.discord.Emit(discord.Event{
				Type: discord.EventCardDone, Card: card, Board: board, User: derefUser(user),
				OldValue: current.ColumnID,
			})
		}
	}

	writeJSON(w, http.StatusOK, card)
}

// ── Transition Rules Engine ──

type TransitionRules struct {
	NoBlockedToDone     bool `json:"no_blocked_to_done"`
	RequireCommentDone  bool `json:"require_comment_done"`
	RequireAssigneeProg bool `json:"require_assignee_prog"`
	RequireDescDone     bool `json:"require_desc_done"`
	NoDoneBackward      bool `json:"no_done_backward"`
}

type TransitionBlocker struct {
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

func (h *Handler) getTransitionRules(ctx context.Context, boardID string) TransitionRules {
	board, err := h.boards.GetByID(ctx, boardID)
	if err != nil {
		return TransitionRules{}
	}
	var settings map[string]json.RawMessage
	if err := json.Unmarshal([]byte(board.Settings), &settings); err != nil {
		return TransitionRules{}
	}
	raw, ok := settings["transition_rules"]
	if !ok {
		return TransitionRules{}
	}
	var rules TransitionRules
	_ = json.Unmarshal(raw, &rules)
	return rules
}

func (h *Handler) checkTransitionRules(ctx context.Context, card repo.Card, toColumn string) []TransitionBlocker {
	rules := h.getTransitionRules(ctx, card.BoardID)
	var blockers []TransitionBlocker

	// Rule: blocked cards cannot move to done
	if rules.NoBlockedToDone && toColumn == "done" {
		if card.BlockedCardIDs != "" && card.BlockedCardIDs != "[]" {
			blockers = append(blockers, TransitionBlocker{
				Rule:    "no_blocked_to_done",
				Message: "Card has blocking dependencies that must be resolved first",
			})
		}
	}

	// Rule: require at least one comment before moving to done
	if rules.RequireCommentDone && toColumn == "done" {
		comments, err := h.comments.ListByCard(ctx, card.ID)
		if err == nil && len(comments) == 0 {
			blockers = append(blockers, TransitionBlocker{
				Rule:    "require_comment_done",
				Message: "Add a comment before closing this ticket",
			})
		}
	}

	// Rule: require assignee before moving to in-progress
	if rules.RequireAssigneeProg && toColumn == "in-progress" {
		if card.AssigneeID == nil || *card.AssigneeID == "" {
			blockers = append(blockers, TransitionBlocker{
				Rule:    "require_assignee_prog",
				Message: "Assign someone before starting work",
			})
		}
	}

	// Rule: require description before moving to done
	if rules.RequireDescDone && toColumn == "done" {
		if card.Description == "" {
			blockers = append(blockers, TransitionBlocker{
				Rule:    "require_desc_done",
				Message: "Add a description before closing this ticket",
			})
		}
	}

	// Rule: prevent moving cards backward out of done
	if rules.NoDoneBackward && card.ColumnID == "done" && toColumn != "done" {
		blockers = append(blockers, TransitionBlocker{
			Rule:    "no_done_backward",
			Message: "Completed tickets cannot be reopened",
		})
	}

	return blockers
}

type bulkMoveReq struct {
	CardIDs  []string `json:"card_ids"`
	ColumnID string   `json:"column_id"`
}

func (h *Handler) BulkMove(w http.ResponseWriter, r *http.Request) {
	var req bulkMoveReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.CardIDs) == 0 {
		writeJSON(w, http.StatusOK, []repo.Card{})
		return
	}
	if req.ColumnID == "" {
		writeValidation(w, map[string]string{"column_id": "required"})
		return
	}

	cards, err := h.cards.BulkMove(r.Context(), req.CardIDs, req.ColumnID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "bulk move failed")
		return
	}

	user := auth.UserFromContext(r.Context())
	boardID := r.PathValue("boardId")
	broadcast(h.hub, boardID, "cards_bulk_moved", map[string]any{
		"cards":     cards,
		"column_id": req.ColumnID,
	}, user.ID)

	writeJSON(w, http.StatusOK, cards)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Get card first for board ID (SSE broadcast)
	card, err := h.cards.GetByID(r.Context(), id)
	if err == repo.ErrNotFound {
		writeErr(w, http.StatusNotFound, "card not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := h.cards.Delete(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	user := auth.UserFromContext(r.Context())
	senderID := ""
	if user != nil {
		senderID = user.ID
	}
	broadcast(h.hub, card.BoardID, "card_deleted", map[string]string{"id": id}, senderID)
	w.WriteHeader(http.StatusNoContent)
}

func broadcast(hub *sse.Hub, boardID, eventType string, payload any, _ string) {
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
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeValidation(w http.ResponseWriter, fields map[string]string) {
	writeJSON(w, http.StatusBadRequest, map[string]any{"error": "validation failed", "fields": fields})
}

func derefUser(u *repo.User) repo.User {
	if u == nil {
		return repo.User{}
	}
	return *u
}
