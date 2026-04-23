package embed

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/oceanplexian/lwts/server/internal/db"
)

// Result is a single semantic match. Score is cosine similarity in [-1, 1];
// for normalized embeddings it's effectively in [0, 1].
type Result struct {
	CardID string
	Score  float64
}

// SearchOptions narrows the candidate set before vector ranking. All fields
// are optional and combined with AND.
type SearchOptions struct {
	BoardID     string
	AssigneeIDs []string
	ColumnID    string
	Tag         string
	Priority    string
	Limit       int     // top-K returned. Default 50, max 200.
	SimFloor    float64 // drop results below this score. Default 0.5.
}

// SearchSemantic embeds the query and returns ranked card IDs by cosine sim.
// Filters are applied as a WHERE clause before ranking. The hnsw index
// continues to do its thing because we still ORDER BY embedding <=> q.
func (s *Service) SearchSemantic(ctx context.Context, query string, opts SearchOptions) ([]Result, error) {
	if s == nil {
		return nil, fmt.Errorf("embed: service not configured")
	}
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}

	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	if opts.Limit > 200 {
		opts.Limit = 200
	}
	if opts.SimFloor == 0 {
		opts.SimFloor = 0.5
	}

	vecs, err := s.client.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if len(vecs) != 1 {
		return nil, fmt.Errorf("embed query: unexpected vector count")
	}
	qvec := Vector(vecs[0])

	// Build WHERE clauses. We always require embedding IS NOT NULL; the index
	// will be used because the ORDER BY operator matches the index op class.
	var conds []string
	var args []any
	conds = append(conds, "c.embedding IS NOT NULL")
	args = append(args, qvec.String())
	argN := 2 // $1 is reserved for the query vector

	if opts.BoardID != "" {
		conds = append(conds, "c.board_id = $"+strconv.Itoa(argN))
		args = append(args, opts.BoardID)
		argN++
	}
	if len(opts.AssigneeIDs) > 0 {
		ph := make([]string, len(opts.AssigneeIDs))
		for i, id := range opts.AssigneeIDs {
			ph[i] = "$" + strconv.Itoa(argN)
			args = append(args, id)
			argN++
		}
		conds = append(conds, "c.assignee_id IN ("+strings.Join(ph, ",")+")")
	}
	if opts.ColumnID != "" {
		conds = append(conds, "c.column_id = $"+strconv.Itoa(argN))
		args = append(args, opts.ColumnID)
		argN++
	}
	if opts.Tag != "" {
		conds = append(conds, "c.tag = $"+strconv.Itoa(argN))
		args = append(args, opts.Tag)
		argN++
	}
	if opts.Priority != "" {
		conds = append(conds, "c.priority = $"+strconv.Itoa(argN))
		args = append(args, opts.Priority)
	}

	q := `SELECT c.id, 1 - (c.embedding <=> $1::vector) AS sim
	      FROM cards c
	      WHERE ` + strings.Join(conds, " AND ") +
		` ORDER BY c.embedding <=> $1::vector
		  LIMIT ` + strconv.Itoa(opts.Limit*2) // overfetch for sim-floor filtering

	rows, err := s.ds.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}
	defer rows.Close()

	results := make([]Result, 0, opts.Limit)
	for rows.Next() {
		var r Result
		if err := rows.Scan(&r.CardID, &r.Score); err != nil {
			return nil, err
		}
		if r.Score < opts.SimFloor {
			continue
		}
		results = append(results, r)
		if len(results) >= opts.Limit {
			break
		}
	}
	return results, rows.Err()
}

// titleWordBoundaryRe builds a Postgres regex matching the query as a
// case-insensitive whole-word substring. Used by the cascade tier.
var nonWordRe = regexp.MustCompile(`\W+`)

// keyShapeRe matches a ticket-key-shaped query: project key + dash + number,
// e.g. FNAI-16 or kanb-7. Whitespace is trimmed before matching.
var keyShapeRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*-\d+$`)

// LooksLikeKey reports whether `q` is shaped like a card key. Used by the
// cascade and the lexical fallback to know whether to attempt an exact-key
// lookup.
func LooksLikeKey(q string) bool {
	return keyShapeRe.MatchString(strings.TrimSpace(q))
}

// KeyMatch returns the ID of the card whose key matches `q` exactly
// (case-insensitive), honoring board/assignee/column/tag/priority filters.
// Returns an empty string when nothing matches or when q isn't key-shaped.
func KeyMatch(ctx context.Context, ds db.Datasource, q string, opts SearchOptions) (string, error) {
	q = strings.TrimSpace(q)
	if !LooksLikeKey(q) {
		return "", nil
	}

	var conds []string
	var args []any
	conds = append(conds, "UPPER(key) = UPPER($1)")
	args = append(args, q)
	argN := 2

	if opts.BoardID != "" {
		conds = append(conds, "board_id = $"+strconv.Itoa(argN))
		args = append(args, opts.BoardID)
		argN++
	}
	if len(opts.AssigneeIDs) > 0 {
		ph := make([]string, len(opts.AssigneeIDs))
		for i, id := range opts.AssigneeIDs {
			ph[i] = "$" + strconv.Itoa(argN)
			args = append(args, id)
			argN++
		}
		conds = append(conds, "assignee_id IN ("+strings.Join(ph, ",")+")")
	}
	if opts.ColumnID != "" {
		conds = append(conds, "column_id = $"+strconv.Itoa(argN))
		args = append(args, opts.ColumnID)
		argN++
	}
	if opts.Tag != "" {
		conds = append(conds, "tag = $"+strconv.Itoa(argN))
		args = append(args, opts.Tag)
		argN++
	}
	if opts.Priority != "" {
		conds = append(conds, "priority = $"+strconv.Itoa(argN))
		args = append(args, opts.Priority)
	}

	sql := `SELECT id FROM cards WHERE ` + strings.Join(conds, " AND ") + ` LIMIT 1`
	row := ds.QueryRow(ctx, sql, args...)
	var id string
	if err := row.Scan(&id); err != nil {
		if err == db.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("key lookup: %w", err)
	}
	return id, nil
}

// TitleWordBoundaryHits returns card IDs whose title contains the trimmed
// query as a word-boundary case-insensitive match. Used by the guarded cascade
// to detect "user typed a specific term" intent.
//
// Filters from SearchOptions are honored.
func (s *Service) TitleWordBoundaryHits(ctx context.Context, ds db.Datasource, query string, opts SearchOptions, limit int) ([]string, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}

	// Postgres regex: \y matches a word boundary. Escape any regex metacharacters
	// in the query to keep this safe.
	pattern := `\y` + escapeRegex(q) + `\y`

	var conds []string
	var args []any
	conds = append(conds, "title ~* $1")
	args = append(args, pattern)
	argN := 2

	if opts.BoardID != "" {
		conds = append(conds, "board_id = $"+strconv.Itoa(argN))
		args = append(args, opts.BoardID)
		argN++
	}
	if len(opts.AssigneeIDs) > 0 {
		ph := make([]string, len(opts.AssigneeIDs))
		for i, id := range opts.AssigneeIDs {
			ph[i] = "$" + strconv.Itoa(argN)
			args = append(args, id)
			argN++
		}
		conds = append(conds, "assignee_id IN ("+strings.Join(ph, ",")+")")
	}
	if opts.ColumnID != "" {
		conds = append(conds, "column_id = $"+strconv.Itoa(argN))
		args = append(args, opts.ColumnID)
		argN++
	}
	if opts.Tag != "" {
		conds = append(conds, "tag = $"+strconv.Itoa(argN))
		args = append(args, opts.Tag)
		argN++
	}
	if opts.Priority != "" {
		conds = append(conds, "priority = $"+strconv.Itoa(argN))
		args = append(args, opts.Priority)
	}

	sql := `SELECT id FROM cards WHERE ` + strings.Join(conds, " AND ") +
		` ORDER BY length(title) ASC LIMIT ` + strconv.Itoa(limit)

	rows, err := ds.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("title word-boundary query: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// MatchKind tags how a result was surfaced. Callers use this to adjust trust:
// a key match is the user typing an exact ticket id and trumps everything;
// a title-boundary pin is a strong, intentional match; a semantic hit is a
// topical neighbor and may warrant a second look.
type MatchKind string

const (
	MatchKey           MatchKind = "key"
	MatchTitleBoundary MatchKind = "title_boundary"
	MatchSemantic      MatchKind = "semantic"
)

// CascadeResult pairs an ID with its score and the kind of match that produced
// it. For title_boundary pins the score is synthetic (1.0) since there's no
// cosine distance; the pin is more trustworthy than any semantic hit anyway.
type CascadeResult struct {
	CardID string
	Score  float64
	Kind   MatchKind
}

// SearchCascade implements the production strategy chosen during evaluation:
//   - if the query is a ticket key (FNAI-16), pin that exact card first
//   - if the query word-boundary-matches 1..3 card titles, pin those next
//   - fill the rest from semantic search
//   - dedupe; preserves filter constraints
//
// Returns results in rank order with score + match kind attached so callers
// can render snippets and confidence tiers.
func (s *Service) SearchCascade(ctx context.Context, query string, opts SearchOptions) ([]CascadeResult, error) {
	if s == nil {
		return nil, fmt.Errorf("embed: service not configured")
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}

	pinned := []CascadeResult{}
	seen := map[string]struct{}{}

	// Tier 0: exact ticket key. If the user typed something that looks like a
	// card key, surface that card first — they almost certainly want it.
	if keyID, err := KeyMatch(ctx, s.ds, query, opts); err != nil {
		return nil, err
	} else if keyID != "" {
		pinned = append(pinned, CascadeResult{CardID: keyID, Score: 1.0, Kind: MatchKey})
		seen[keyID] = struct{}{}
	}

	hits, err := s.TitleWordBoundaryHits(ctx, s.ds, query, opts, 5)
	if err != nil {
		return nil, err
	}

	// Tier 1: pin small, confident match sets.
	if len(hits) >= 1 && len(hits) <= 3 {
		for _, id := range hits {
			if _, ok := seen[id]; !ok {
				pinned = append(pinned, CascadeResult{CardID: id, Score: 1.0, Kind: MatchTitleBoundary})
				seen[id] = struct{}{}
			}
		}
	}

	// Tier 2: semantic fill.
	semOpts := opts
	semOpts.Limit = limit
	sem, err := s.SearchSemantic(ctx, query, semOpts)
	if err != nil {
		return nil, err
	}

	out := make([]CascadeResult, 0, limit)
	out = append(out, pinned...)
	for _, r := range sem {
		if _, ok := seen[r.CardID]; ok {
			continue
		}
		out = append(out, CascadeResult{CardID: r.CardID, Score: r.Score, Kind: MatchSemantic})
		seen[r.CardID] = struct{}{}
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

// escapeRegex escapes Postgres POSIX regex metacharacters in user input.
func escapeRegex(s string) string {
	return nonWordRe.ReplaceAllStringFunc(s, func(m string) string {
		var b strings.Builder
		for _, r := range m {
			b.WriteByte('\\')
			b.WriteRune(r)
		}
		return b.String()
	})
}
