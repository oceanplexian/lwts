package board

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/oceanplexian/lwts/server/internal/db"
	"github.com/oceanplexian/lwts/server/internal/embed"
	"github.com/oceanplexian/lwts/server/internal/repo"
)

type SearchHandler struct {
	ds            db.Datasource
	embed         *embed.Service
	pgvectorReady bool
}

func NewSearchHandler(ds db.Datasource) *SearchHandler {
	return &SearchHandler{ds: ds}
}

// SetEmbed wires the optional semantic search service. pgvectorReady reflects
// whether EnsureSchema reported the column+index were provisioned at startup.
// Both can be nil/false: the handler falls back to LIKE search transparently.
func (h *SearchHandler) SetEmbed(svc *embed.Service, pgvectorReady bool) {
	h.embed = svc
	h.pgvectorReady = pgvectorReady
}

func (h *SearchHandler) RegisterRoutes(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/search", authMW(http.HandlerFunc(h.Search)))
}

// semanticAvailable reports whether the runtime conditions for semantic search
// are met: a service is wired, pgvector schema is ready, and the workspace
// setting search_mode == "semantic".
func (h *SearchHandler) semanticAvailable(r *http.Request) bool {
	if h.embed == nil || !h.pgvectorReady {
		return false
	}
	return readSearchMode(r, h.ds) == "semantic"
}

func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	boardID := r.URL.Query().Get("board_id")
	assigneeID := r.URL.Query().Get("assignee_id")
	assigneeName := strings.TrimSpace(r.URL.Query().Get("assignee"))
	columnID := r.URL.Query().Get("column_id")
	tag := r.URL.Query().Get("tag")
	priority := r.URL.Query().Get("priority")
	limitStr := r.URL.Query().Get("limit")

	// At least one filter required
	if q == "" && assigneeID == "" && assigneeName == "" && columnID == "" && tag == "" && priority == "" && boardID == "" {
		writeErr(w, http.StatusBadRequest, "at least one filter required (q, assignee_id, assignee, column_id, tag, priority, board_id)")
		return
	}

	// If assignee is a name, resolve to user IDs
	var assigneeIDs []string
	if assigneeName != "" {
		users := repo.NewUserRepository(h.ds)
		allUsers, err := users.List(r.Context())
		if err == nil {
			nameLower := strings.ToLower(assigneeName)
			for _, u := range allUsers {
				if strings.Contains(strings.ToLower(u.Name), nameLower) ||
					strings.Contains(strings.ToLower(u.Email), nameLower) {
					assigneeIDs = append(assigneeIDs, u.ID)
				}
			}
		}
		if len(assigneeIDs) == 0 {
			writeJSON(w, http.StatusOK, []repo.Card{})
			return
		}
	}
	if assigneeID != "" {
		assigneeIDs = append(assigneeIDs, assigneeID)
	}

	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
			limit = l
		}
	}

	// Semantic dispatch: if the query string is non-empty AND the workspace is
	// configured for semantic search AND pgvector is ready, run the cascade
	// engine. Filters still apply. Falls through to LIKE on any error.
	if q != "" && h.semanticAvailable(r) {
		opts := embed.SearchOptions{
			BoardID:     boardID,
			AssigneeIDs: assigneeIDs,
			ColumnID:    columnID,
			Tag:         tag,
			Priority:    priority,
			Limit:       limit,
		}
		ids, err := h.embed.SearchCascade(r.Context(), q, opts)
		if err == nil {
			cards, err := h.fetchCardsByIDs(r, ids)
			if err == nil {
				writeJSON(w, http.StatusOK, cards)
				return
			}
		}
		// fall through to LIKE on failure
	}

	h.likeSearch(w, r, q, boardID, assigneeIDs, columnID, tag, priority, limit)
}

// likeSearch is the original substring engine, preserved unchanged so the
// default behavior is identical to before this feature landed.
func (h *SearchHandler) likeSearch(w http.ResponseWriter, r *http.Request,
	q, boardID string, assigneeIDs []string, columnID, tag, priority string, limit int,
) {
	var conditions []string
	var args []any
	argN := 1

	if boardID != "" {
		conditions = append(conditions, "c.board_id = $"+strconv.Itoa(argN))
		args = append(args, boardID)
		argN++
	}
	if q != "" {
		pattern := "%" + q + "%"
		conditions = append(conditions, "(c.title LIKE $"+strconv.Itoa(argN)+" OR c.description LIKE $"+strconv.Itoa(argN+1)+")")
		args = append(args, pattern, pattern)
		argN += 2
	}
	if len(assigneeIDs) > 0 {
		placeholders := make([]string, len(assigneeIDs))
		for i, id := range assigneeIDs {
			placeholders[i] = "$" + strconv.Itoa(argN)
			args = append(args, id)
			argN++
		}
		conditions = append(conditions, "c.assignee_id IN ("+strings.Join(placeholders, ",")+")")
	}
	if columnID != "" {
		conditions = append(conditions, "c.column_id = $"+strconv.Itoa(argN))
		args = append(args, columnID)
		argN++
	}
	if tag != "" {
		conditions = append(conditions, "c.tag = $"+strconv.Itoa(argN))
		args = append(args, tag)
		argN++
	}
	if priority != "" {
		conditions = append(conditions, "c.priority = $"+strconv.Itoa(argN))
		args = append(args, priority)
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}

	query := `SELECT c.id, c.board_id, c.column_id, c.title, c.description, c.tag, c.priority,
		c.assignee_id, c.reporter_id, c.points, c.position, c.key, c.version,
		CAST(c.due_date AS TEXT), c.related_card_ids, c.blocked_card_ids, c.created_at, c.updated_at,
		COALESCE(u.name, '') as assignee_name
		FROM cards c
		LEFT JOIN users u ON c.assignee_id = u.id` +
		where +
		` ORDER BY c.updated_at DESC LIMIT ` + strconv.Itoa(limit)

	rows, err := h.ds.Query(r.Context(), query, args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer rows.Close()

	type cardWithAssignee struct {
		repo.Card
		AssigneeName string `json:"assignee_name"`
	}

	var cards []cardWithAssignee
	for rows.Next() {
		var c cardWithAssignee
		if err := rows.Scan(&c.ID, &c.BoardID, &c.ColumnID, &c.Title, &c.Description, &c.Tag, &c.Priority,
			&c.AssigneeID, &c.ReporterID, &c.Points, &c.Position, &c.Key, &c.Version,
			&c.DueDate, &c.RelatedCardIDs, &c.BlockedCardIDs, &c.CreatedAt, &c.UpdatedAt,
			&c.AssigneeName); err != nil {
			writeErr(w, http.StatusInternalServerError, "internal server error")
			return
		}
		cards = append(cards, c)
	}
	if cards == nil {
		cards = []cardWithAssignee{}
	}
	writeJSON(w, http.StatusOK, cards)
}

// fetchCardsByIDs hydrates a set of card IDs into the same JSON shape used by
// likeSearch. Order is preserved per the input slice (matching the cascade
// ranking, not updated_at).
func (h *SearchHandler) fetchCardsByIDs(r *http.Request, ids []string) ([]any, error) {
	if len(ids) == 0 {
		return []any{}, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "$" + strconv.Itoa(i+1)
		args[i] = id
	}

	q := `SELECT c.id, c.board_id, c.column_id, c.title, c.description, c.tag, c.priority,
		c.assignee_id, c.reporter_id, c.points, c.position, c.key, c.version,
		CAST(c.due_date AS TEXT), c.related_card_ids, c.blocked_card_ids, c.created_at, c.updated_at,
		COALESCE(u.name, '') as assignee_name
		FROM cards c LEFT JOIN users u ON c.assignee_id = u.id
		WHERE c.id IN (` + strings.Join(placeholders, ",") + `)`

	rows, err := h.ds.Query(r.Context(), q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type cardWithAssignee struct {
		repo.Card
		AssigneeName string `json:"assignee_name"`
	}

	byID := make(map[string]cardWithAssignee, len(ids))
	for rows.Next() {
		var c cardWithAssignee
		if err := rows.Scan(&c.ID, &c.BoardID, &c.ColumnID, &c.Title, &c.Description, &c.Tag, &c.Priority,
			&c.AssigneeID, &c.ReporterID, &c.Points, &c.Position, &c.Key, &c.Version,
			&c.DueDate, &c.RelatedCardIDs, &c.BlockedCardIDs, &c.CreatedAt, &c.UpdatedAt,
			&c.AssigneeName); err != nil {
			return nil, err
		}
		byID[c.ID] = c
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Preserve cascade order. Cards that vanished mid-flight are simply skipped.
	out := make([]any, 0, len(ids))
	for _, id := range ids {
		if c, ok := byID[id]; ok {
			out = append(out, c)
		}
	}
	return out, nil
}

// readSearchMode reads the workspace search_mode preference from settings
// without requiring the settings package (avoids an import cycle). Defaults
// to "lexical" on any error.
func readSearchMode(r *http.Request, ds db.Datasource) string {
	var raw string
	if err := ds.QueryRow(r.Context(), "SELECT value FROM settings WHERE key = 'general'").Scan(&raw); err != nil {
		return "lexical"
	}
	var s map[string]any
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return "lexical"
	}
	mode, _ := s["search_mode"].(string)
	if mode == "semantic" {
		return "semantic"
	}
	return "lexical"
}
