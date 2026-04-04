package board

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/oceanplexian/lwts/server/internal/db"
	"github.com/oceanplexian/lwts/server/internal/repo"
)

type SearchHandler struct {
	ds db.Datasource
}

func NewSearchHandler(ds db.Datasource) *SearchHandler {
	return &SearchHandler{ds: ds}
}

func (h *SearchHandler) RegisterRoutes(mux *http.ServeMux, authMW func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/search", authMW(http.HandlerFunc(h.Search)))
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
			// No matching user — return empty
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

	// Build query dynamically
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
		c.due_date, c.related_card_ids, c.blocked_card_ids, c.created_at, c.updated_at,
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
