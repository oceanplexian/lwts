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

// enrichedCard is the JSON shape returned by /api/v1/search. It inlines the
// full Card fields (backward-compatible with existing clients) and adds four
// new fields agents use to evaluate matches: score (0..1), match_kind
// (title_boundary | semantic | lexical), snippet (why the card matched), and
// assignee_name for human-readable display.
type enrichedCard struct {
	repo.Card
	AssigneeName string  `json:"assignee_name"`
	Score        float64 `json:"score"`
	MatchKind    string  `json:"match_kind"`
	Snippet      string  `json:"snippet"`
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
	// Agent-friendly opt-in filters. Defaults preserve pre-feature behavior
	// (include done, no score floor) so the web UI search box is unchanged.
	includeDone := r.URL.Query().Get("include_done") != "false" // default true
	minScoreStr := r.URL.Query().Get("min_score")

	if q == "" && assigneeID == "" && assigneeName == "" && columnID == "" && tag == "" && priority == "" && boardID == "" {
		writeErr(w, http.StatusBadRequest, "at least one filter required (q, assignee_id, assignee, column_id, tag, priority, board_id)")
		return
	}

	// Resolve assignee name → IDs
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
			writeSearchJSON(w, "lexical", 0, []enrichedCard{})
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

	minScore := 0.0
	if minScoreStr != "" {
		if s, err := strconv.ParseFloat(minScoreStr, 64); err == nil && s >= 0 && s <= 1 {
			minScore = s
		}
	}

	// Semantic dispatch: if query is non-empty AND workspace is configured
	// for semantic AND pgvector is ready, run the cascade engine.
	if q != "" && h.semanticAvailable(r) {
		opts := embed.SearchOptions{
			BoardID:     boardID,
			AssigneeIDs: assigneeIDs,
			ColumnID:    columnID,
			Tag:         tag,
			Priority:    priority,
			Limit:       limit,
		}
		if h.runSemanticSearch(w, r, q, opts, includeDone, minScore) {
			return
		}
		// fall through to LIKE on any failure
	}

	h.runLikeSearch(w, r, q, boardID, assigneeIDs, columnID, tag, priority, limit, includeDone, minScore)
}

// runSemanticSearch returns true if it produced a response. Returns false on
// any transient failure so the caller can fall back to LIKE.
func (h *SearchHandler) runSemanticSearch(w http.ResponseWriter, r *http.Request, q string, opts embed.SearchOptions, includeDone bool, minScore float64) bool {
	results, err := h.embed.SearchCascade(r.Context(), q, opts)
	if err != nil {
		return false
	}

	// Hydrate cards (full row) in cascade order.
	ids := make([]string, len(results))
	meta := make(map[string]embed.CascadeResult, len(results))
	for i, res := range results {
		ids[i] = res.CardID
		meta[res.CardID] = res
	}
	cards, err := h.hydrateCards(r, ids)
	if err != nil {
		return false
	}

	// Apply post-query filters (done/cleared and min_score). Total is the
	// count before truncation so the client knows whether to refine.
	total := 0
	out := make([]enrichedCard, 0, len(cards))
	for _, c := range cards {
		if !includeDone && isDoneColumn(c.ColumnID) {
			continue
		}
		res, ok := meta[c.ID]
		if !ok {
			continue
		}
		if res.Score < minScore {
			continue
		}
		total++
		if len(out) >= opts.Limit {
			continue
		}
		out = append(out, enrichedCard{
			Card:         c.Card,
			AssigneeName: c.AssigneeName,
			Score:        res.Score,
			MatchKind:    string(res.Kind),
			Snippet:      buildSnippet(res.Kind, q, c.Title, c.Description),
		})
	}
	writeSearchJSON(w, "semantic", total, out)
	return true
}

// runLikeSearch is the substring-based engine. Preserves legacy behavior
// (including unfiltered results) while still populating the new per-card
// fields so agents calling through the CLI get a consistent shape.
func (h *SearchHandler) runLikeSearch(w http.ResponseWriter, r *http.Request,
	q, boardID string, assigneeIDs []string, columnID, tag, priority string, limit int,
	includeDone bool, minScore float64,
) {
	// Pin an exact ticket-key match first so a query like "FNAI-16" surfaces
	// that card regardless of what the title/description LIKE produces.
	pinnedKeyID := ""
	if q != "" {
		opts := embed.SearchOptions{
			BoardID:     boardID,
			AssigneeIDs: assigneeIDs,
			ColumnID:    columnID,
			Tag:         tag,
			Priority:    priority,
		}
		if id, err := embed.KeyMatch(r.Context(), h.ds, q, opts); err == nil {
			pinnedKeyID = id
		}
	}

	var conditions []string
	var args []any
	argN := 1

	if boardID != "" {
		conditions = append(conditions, "c.board_id = $"+strconv.Itoa(argN))
		args = append(args, boardID)
		argN++
	}
	if q != "" {
		// Match against title, description, and key. Including key lets
		// partial key prefixes like "KANB-" surface tickets even before the
		// user finishes typing the number.
		pattern := "%" + q + "%"
		conditions = append(conditions, "(c.title LIKE $"+strconv.Itoa(argN)+" OR c.description LIKE $"+strconv.Itoa(argN+1)+" OR c.key LIKE $"+strconv.Itoa(argN+2)+")")
		args = append(args, pattern, pattern, pattern)
		argN += 3
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

	// Overfetch to honor the min_score/include_done filters without needing
	// cursor pagination; bound by a reasonable ceiling so we don't scan the
	// entire table for tiny workspaces.
	fetchLimit := limit * 4
	if fetchLimit < 100 {
		fetchLimit = 100
	}
	if fetchLimit > 500 {
		fetchLimit = 500
	}

	query := `SELECT c.id, c.board_id, c.column_id, c.title, c.description, c.tag, c.priority,
		c.assignee_id, c.reporter_id, c.points, c.position, c.key, c.version,
		CAST(c.due_date AS TEXT), c.related_card_ids, c.blocked_card_ids, c.created_at, c.updated_at,
		COALESCE(u.name, '') as assignee_name
		FROM cards c
		LEFT JOIN users u ON c.assignee_id = u.id` +
		where +
		` ORDER BY c.updated_at DESC LIMIT ` + strconv.Itoa(fetchLimit)

	rows, err := h.ds.Query(r.Context(), query, args...)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer rows.Close()

	total := 0
	out := make([]enrichedCard, 0, limit)
	pinnedSeen := false
	for rows.Next() {
		var c cardWithAssignee
		if err := rows.Scan(&c.ID, &c.BoardID, &c.ColumnID, &c.Title, &c.Description, &c.Tag, &c.Priority,
			&c.AssigneeID, &c.ReporterID, &c.Points, &c.Position, &c.Key, &c.Version,
			&c.DueDate, &c.RelatedCardIDs, &c.BlockedCardIDs, &c.CreatedAt, &c.UpdatedAt,
			&c.AssigneeName); err != nil {
			writeErr(w, http.StatusInternalServerError, "internal server error")
			return
		}
		if !includeDone && isDoneColumn(c.ColumnID) {
			continue
		}
		isPinnedKey := pinnedKeyID != "" && c.ID == pinnedKeyID
		score, kind := scoreLexical(q, c.Title, c.Description)
		if isPinnedKey {
			score = 1.0
			kind = string(embed.MatchKey)
		}
		if score < minScore && !isPinnedKey {
			continue
		}
		total++
		card := enrichedCard{
			Card:         c.Card,
			AssigneeName: c.AssigneeName,
			Score:        score,
			MatchKind:    kind,
			Snippet:      buildSnippet(embed.MatchKind(kind), q, c.Title, c.Description),
		}
		if isPinnedKey {
			// Move the pinned card to the front and never let it spill past limit.
			out = append([]enrichedCard{card}, out...)
			if len(out) > limit {
				out = out[:limit]
			}
			pinnedSeen = true
			continue
		}
		if len(out) >= limit {
			continue
		}
		out = append(out, card)
	}
	// If the pinned key wasn't in the LIKE result set (filtered out by limit
	// or because it didn't substring-match the query at all), hydrate it
	// separately so the user still sees their exact match first.
	if pinnedKeyID != "" && !pinnedSeen {
		hydrated, err := h.hydrateCards(r, []string{pinnedKeyID})
		if err == nil && len(hydrated) > 0 {
			c := hydrated[0]
			if includeDone || !isDoneColumn(c.ColumnID) {
				total++
				card := enrichedCard{
					Card:         c.Card,
					AssigneeName: c.AssigneeName,
					Score:        1.0,
					MatchKind:    string(embed.MatchKey),
					Snippet:      buildSnippet(embed.MatchKey, q, c.Title, c.Description),
				}
				out = append([]enrichedCard{card}, out...)
				if len(out) > limit {
					out = out[:limit]
				}
			}
		}
	}
	writeSearchJSON(w, "lexical", total, out)
}

// cardWithAssignee is used internally during row scanning. Unexported;
// enrichedCard is what we return to callers.
type cardWithAssignee struct {
	repo.Card
	AssigneeName string
}

// hydrateCards fetches full card rows for a list of IDs, preserving the input
// order (cascade ranking). Cards that vanished mid-flight are skipped.
func (h *SearchHandler) hydrateCards(r *http.Request, ids []string) ([]cardWithAssignee, error) {
	if len(ids) == 0 {
		return nil, nil
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
	out := make([]cardWithAssignee, 0, len(ids))
	for _, id := range ids {
		if c, ok := byID[id]; ok {
			out = append(out, c)
		}
	}
	return out, nil
}

// isDoneColumn matches the conventional terminal columns. Workspaces can add
// custom columns; we only filter the standard done/cleared pair so as not to
// silently hide work the user cares about.
func isDoneColumn(id string) bool {
	switch strings.ToLower(id) {
	case "done", "cleared", "complete", "completed", "resolved":
		return true
	}
	return false
}

// scoreLexical assigns a synthetic confidence for LIKE matches. Title hit is
// stronger than description-only hit. Used so agents can apply a min_score
// filter consistently across lexical and semantic results.
func scoreLexical(q, title, description string) (float64, string) {
	if q == "" {
		return 0.5, "lexical"
	}
	qLower := strings.ToLower(q)
	if strings.Contains(strings.ToLower(title), qLower) {
		return 0.9, "lexical"
	}
	if strings.Contains(strings.ToLower(description), qLower) {
		return 0.6, "lexical"
	}
	return 0.5, "lexical"
}

// buildSnippet picks an appropriate excerpt to show the caller. Title-boundary
// and key pins don't need a snippet — the title/key is the evidence — but we
// still return a short one for context when the description isn't empty.
func buildSnippet(kind embed.MatchKind, q, title, description string) string {
	body := strings.TrimSpace(description)
	if body == "" {
		body = title
	}
	if kind == embed.MatchTitleBoundary || kind == embed.MatchKey {
		// Title/key already shows the match; include leading description for context.
		return leadingChunk(body)
	}
	return extractSnippet(q, body)
}

// writeSearchJSON writes the legacy array response but also sets the
// aggregate metadata headers the CLI/agents consume.
func writeSearchJSON(w http.ResponseWriter, mode string, totalMatches int, cards []enrichedCard) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Search-Mode", mode)
	w.Header().Set("X-Total-Matches", strconv.Itoa(totalMatches))
	if cards == nil {
		cards = []enrichedCard{}
	}
	_ = json.NewEncoder(w).Encode(cards)
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
