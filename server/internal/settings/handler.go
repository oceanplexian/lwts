package settings

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/oceanplexian/lwts/server/internal/auth"
	"github.com/oceanplexian/lwts/server/internal/db"
	"github.com/oceanplexian/lwts/server/internal/repo"
	"github.com/google/uuid"
)

// SeedFunc creates demo data for a given owner user ID.
type SeedFunc func(ctx context.Context, ownerID string) error

type Handler struct {
	ds       db.Datasource
	users    *repo.UserRepository
	boards   *repo.BoardRepository
	cards    *repo.CardRepository
	comments *repo.CommentRepository
	seedFunc SeedFunc
}

func NewHandler(ds db.Datasource, users *repo.UserRepository, boards *repo.BoardRepository, cards *repo.CardRepository, comments *repo.CommentRepository) *Handler {
	return &Handler{ds: ds, users: users, boards: boards, cards: cards, comments: comments}
}

// SetSeedFunc sets the function used to seed demo data on reset.
func (h *Handler) SetSeedFunc(fn SeedFunc) {
	h.seedFunc = fn
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux, authMW, adminMW func(http.Handler) http.Handler) {
	// Public endpoint — no auth required (login page needs this)
	mux.HandleFunc("GET /api/v1/registration-status", h.RegistrationStatus)
	mux.Handle("GET /api/v1/settings/{category}", authMW(http.HandlerFunc(h.GetSettings)))
	mux.Handle("PUT /api/v1/settings/{category}", authMW(http.HandlerFunc(h.PutSettings)))
	mux.Handle("POST /api/v1/settings/clear-cards", adminMW(http.HandlerFunc(h.ClearAllCards)))
	mux.Handle("POST /api/v1/settings/reset", adminMW(http.HandlerFunc(h.ResetWorkspace)))
	mux.HandleFunc("GET /api/v1/workspace-status", h.WorkspaceStatus)
	mux.Handle("GET /api/v1/export", authMW(http.HandlerFunc(h.Export)))
	mux.Handle("POST /api/v1/import/jira", adminMW(http.HandlerFunc(h.ImportJira)))
	mux.Handle("POST /api/v1/import/trello", adminMW(http.HandlerFunc(h.ImportTrello)))
	mux.Handle("POST /api/v1/invites", adminMW(http.HandlerFunc(h.CreateInvite)))
	mux.Handle("GET /api/v1/keys", authMW(http.HandlerFunc(h.ListKeys)))
	mux.Handle("POST /api/v1/keys", authMW(http.HandlerFunc(h.CreateKey)))
	mux.Handle("GET /api/v1/keys/{id}/reveal", authMW(http.HandlerFunc(h.RevealKey)))
	mux.Handle("DELETE /api/v1/keys/{id}", adminMW(http.HandlerFunc(h.DeleteKey)))
	mux.Handle("POST /api/v1/users", adminMW(http.HandlerFunc(h.CreateUser)))
	mux.Handle("PUT /api/v1/users/{id}", adminMW(http.HandlerFunc(h.UpdateUser)))
	mux.Handle("DELETE /api/v1/users/{id}", adminMW(http.HandlerFunc(h.DeleteUser)))
	mux.Handle("POST /api/v1/bots", adminMW(http.HandlerFunc(h.CreateBot)))
}

// ── Settings KV ──

var defaultSettings = map[string]string{
	"general":    `{"workspace_name":"LWTS","default_assignee_id":"","auto_save":true,"compact_cards":false,"allow_registration":false,"base_url":""}`,
	"appearance": `{"dark_mode":true,"accent_color":"#e50914","card_animations":true,"density":"default","font_size":"medium","show_card_ids":true,"show_avatars":true,"show_priority_icons":true}`,
}

// IsRegistrationAllowed checks the general settings to see if user registration is enabled.
func (h *Handler) IsRegistrationAllowed(r *http.Request) bool {
	var val string
	err := h.ds.QueryRow(r.Context(), "SELECT value FROM settings WHERE key = 'general'").Scan(&val)
	if err != nil {
		val = defaultSettings["general"]
	}
	var settings map[string]any
	if json.Unmarshal([]byte(val), &settings) != nil {
		return false
	}
	allowed, ok := settings["allow_registration"].(bool)
	return ok && allowed
}

func (h *Handler) RegistrationStatus(w http.ResponseWriter, r *http.Request) {
	users, _ := h.users.List(r.Context())
	firstRun := len(users) == 0
	allowed := firstRun || h.IsRegistrationAllowed(r)
	writeJSON(w, http.StatusOK, map[string]any{"allowed": allowed, "first_run": firstRun})
}

func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	def, ok := defaultSettings[category]
	if !ok {
		writeErr(w, http.StatusBadRequest, "unknown settings category")
		return
	}

	// Appearance settings are per-user
	key := category
	if category == "appearance" {
		user := auth.UserFromContext(r.Context())
		if user != nil {
			key = "appearance:" + user.ID
		}
	}

	var val string
	err := h.ds.QueryRow(r.Context(), "SELECT value FROM settings WHERE key = $1", key).Scan(&val)
	if err == db.ErrNoRows {
		val = def
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(val))
}

func (h *Handler) PutSettings(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	if _, ok := defaultSettings[category]; !ok {
		writeErr(w, http.StatusBadRequest, "unknown settings category")
		return
	}

	user := auth.UserFromContext(r.Context())

	// Non-appearance categories require admin role
	if category != "appearance" {
		if auth.RoleLevel(user.Role) < auth.RoleLevel("admin") {
			writeErr(w, http.StatusForbidden, "admin role required to update "+category+" settings")
			return
		}
	}

	var body json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Appearance settings are per-user
	key := category
	if category == "appearance" {
		key = "appearance:" + user.ID
	}

	// Merge with existing
	existing := defaultSettings[category]
	var val string
	err := h.ds.QueryRow(r.Context(), "SELECT value FROM settings WHERE key = $1", key).Scan(&val)
	if err == nil {
		existing = val
	}

	var merged map[string]any
	_ = json.Unmarshal([]byte(existing), &merged)
	var updates map[string]any
	_ = json.Unmarshal(body, &updates)
	for k, v := range updates {
		merged[k] = v
	}

	mergedJSON, _ := json.Marshal(merged)
	now := time.Now().UTC()

	if h.ds.DBType() == "postgres" {
		_, err = h.ds.Exec(r.Context(),
			`INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, $3)
			 ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = $3`,
			key, string(mergedJSON), now)
	} else {
		_, err = h.ds.Exec(r.Context(),
			`INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, $3)
			 ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
			key, string(mergedJSON), now)
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "save settings: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(mergedJSON)
}

// ── Users ──

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.users.List(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if users == nil {
		users = []repo.User{}
	}
	writeJSON(w, http.StatusOK, users)
}

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	currentUser := auth.UserFromContext(r.Context())
	targetID := r.PathValue("id")

	var body struct {
		Role      *string `json:"role"`
		AvatarURL *string `json:"avatar_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if body.Role != nil {
		// Cannot promote above own role
		if auth.RoleLevel(*body.Role) > auth.RoleLevel(currentUser.Role) {
			writeErr(w, http.StatusForbidden, "cannot promote above your own role")
			return
		}
		// Cannot change own role
		if targetID == currentUser.ID {
			writeErr(w, http.StatusForbidden, "cannot change your own role")
			return
		}
	}

	// Validate avatar_url size (data URLs can be large — cap at 100KB)
	if body.AvatarURL != nil && len(*body.AvatarURL) > 100*1024 {
		writeErr(w, http.StatusBadRequest, "avatar_url too large (max 100KB)")
		return
	}

	updated, err := h.users.Update(r.Context(), targetID, repo.UserUpdate{Role: body.Role, AvatarURL: body.AvatarURL})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	currentUser := auth.UserFromContext(r.Context())
	targetID := r.PathValue("id")

	if targetID == currentUser.ID {
		writeErr(w, http.StatusForbidden, "cannot remove yourself")
		return
	}

	target, err := h.users.GetByID(r.Context(), targetID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "user not found")
		return
	}
	if target.Role == "owner" {
		writeErr(w, http.StatusForbidden, "cannot remove the workspace owner")
		return
	}

	if err := h.users.Delete(r.Context(), targetID); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Create User (admin) ──

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	// No registration gate here — this is admin-only (behind adminMW).
	// The allow_registration setting only gates public self-registration.
	var body struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" || body.Email == "" || body.Password == "" {
		writeErr(w, http.StatusBadRequest, "name, email, and password are required")
		return
	}
	if body.Role == "" {
		body.Role = "member"
	}
	if body.Role != "admin" && body.Role != "member" && body.Role != "viewer" {
		writeErr(w, http.StatusBadRequest, "role must be admin, member, or viewer")
		return
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	// Generate avatar color and initials
	colors := []string{"#82B1FF", "#fbc02d", "#4ade80", "#fb8c00", "#e040fb", "#00bcd4"}
	avatarColor := colors[len(body.Name)%len(colors)]
	initials := ""
	words := splitWords(body.Name)
	for _, w := range words {
		if len(w) > 0 {
			initials += string([]rune(w)[0])
		}
	}
	if len(initials) > 2 {
		initials = initials[:2]
	}
	initials = toUpper(initials)

	user, err := h.users.Create(r.Context(), body.Name, body.Email, hash)
	if err != nil {
		writeErr(w, http.StatusConflict, "email already registered")
		return
	}

	// Update role and avatar_color
	_, _ = h.users.Update(r.Context(), user.ID, repo.UserUpdate{
		Role:        &body.Role,
		AvatarColor: &avatarColor,
	})

	user.Role = body.Role
	user.AvatarColor = avatarColor
	user.Initials = initials
	writeJSON(w, http.StatusCreated, user)
}

// ── Create Bot (admin) ──

func (h *Handler) CreateBot(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	if body.Role == "" {
		body.Role = "member"
	}
	if body.Role != "admin" && body.Role != "member" && body.Role != "viewer" {
		writeErr(w, http.StatusBadRequest, "role must be admin, member, or viewer")
		return
	}

	// Generate bot email and random password
	email := "bot-" + uuid.New().String()[:8] + "@bots.local"
	pwBytes := make([]byte, 32)
	_, _ = rand.Read(pwBytes)
	password := hex.EncodeToString(pwBytes)

	hash, err := auth.HashPassword(password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user, err := h.users.Create(r.Context(), body.Name, email, hash)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create bot user: "+err.Error())
		return
	}

	// Update role
	_, _ = h.users.Update(r.Context(), user.ID, repo.UserUpdate{Role: &body.Role})
	user.Role = body.Role

	writeJSON(w, http.StatusCreated, user)
}

func splitWords(s string) []string {
	var words []string
	word := ""
	for _, c := range s {
		if c == ' ' || c == '\t' {
			if word != "" {
				words = append(words, word)
				word = ""
			}
		} else {
			word += string(c)
		}
	}
	if word != "" {
		words = append(words, word)
	}
	return words
}

func toUpper(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// ── Invites ──

func (h *Handler) CreateInvite(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	var body struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Role == "" {
		body.Role = "member"
	}

	id := uuid.New().String()
	expires := time.Now().UTC().Add(7 * 24 * time.Hour)
	now := time.Now().UTC()

	_, err := h.ds.Exec(r.Context(),
		`INSERT INTO invites (id, email, role, created_by, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		id, body.Email, body.Role, user.ID, expires, now)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create invite: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"id":         id,
		"invite_url": "/invite/" + id,
		"expires_at": expires.Format(time.RFC3339),
	})
}

// ── API Keys ──

func (h *Handler) ListKeys(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())

	var rows *db.Rows
	var err error
	if auth.RoleLevel(user.Role) >= auth.RoleLevel("admin") {
		rows, err = h.ds.Query(r.Context(),
			`SELECT id, name, key_prefix, permissions, created_at FROM api_keys ORDER BY created_at DESC`)
	} else {
		rows, err = h.ds.Query(r.Context(),
			`SELECT id, name, key_prefix, permissions, created_at FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC`, user.ID)
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer rows.Close()

	type apiKeyResp struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		KeyMasked   string `json:"key_masked"`
		Permissions string `json:"permissions"`
		CreatedAt   string `json:"created_at"`
	}

	var keys []apiKeyResp
	for rows.Next() {
		var k apiKeyResp
		var createdAt time.Time
		if err := rows.Scan(&k.ID, &k.Name, &k.KeyMasked, &k.Permissions, &createdAt); err != nil {
			writeErr(w, http.StatusInternalServerError, "scan: "+err.Error())
			return
		}
		k.CreatedAt = createdAt.Format(time.RFC3339)
		keys = append(keys, k)
	}
	if keys == nil {
		keys = []apiKeyResp{}
	}
	writeJSON(w, http.StatusOK, keys)
}

func (h *Handler) CreateKey(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	var body struct {
		Name        string `json:"name"`
		Permissions string `json:"permissions"`
		UserID      string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Name == "" {
		writeErr(w, http.StatusBadRequest, "name required")
		return
	}
	if body.Permissions == "" {
		body.Permissions = "{}"
	}

	// Determine target user for the key
	targetUserID := user.ID
	if body.UserID != "" && body.UserID != user.ID {
		// Creating a key for another user requires admin role
		if auth.RoleLevel(user.Role) < auth.RoleLevel("admin") {
			writeErr(w, http.StatusForbidden, "admin role required to create keys for other users")
			return
		}
		targetUserID = body.UserID
	}

	// Generate random API key
	rawKey := make([]byte, 32)
	_, _ = rand.Read(rawKey)
	fullKey := "lwts_sk_" + hex.EncodeToString(rawKey)
	prefix := "lwts_sk_" + "••••••••" + fullKey[len(fullKey)-4:]

	hash := sha256.Sum256([]byte(fullKey))
	keyHash := hex.EncodeToString(hash[:])

	id := uuid.New().String()
	now := time.Now().UTC()

	_, err := h.ds.Exec(r.Context(),
		`INSERT INTO api_keys (id, user_id, name, key_hash, key_prefix, key_full, permissions, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		id, targetUserID, body.Name, keyHash, prefix, fullKey, body.Permissions, now)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create key: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"id":         id,
		"key":        fullKey,
		"key_masked":  prefix,
		"name":       body.Name,
		"created_at": now.Format(time.RFC3339),
	})
}

func (h *Handler) RevealKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var fullKey string
	err := h.ds.QueryRow(r.Context(), "SELECT key_full FROM api_keys WHERE id = $1", id).Scan(&fullKey)
	if err == db.ErrNoRows {
		writeErr(w, http.StatusNotFound, "key not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "reveal key: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"key": fullKey})
}

func (h *Handler) DeleteKey(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	n, err := h.ds.Exec(r.Context(), "DELETE FROM api_keys WHERE id = $1", id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "delete key: "+err.Error())
		return
	}
	if n == 0 {
		writeErr(w, http.StatusNotFound, "key not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── Export ──

func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	users, _ := h.users.List(ctx)
	boards, _ := h.boards.List(ctx)

	type boardExport struct {
		Board    repo.Board     `json:"board"`
		Cards    []repo.Card    `json:"cards"`
		Comments []repo.Comment `json:"comments"`
	}

	var boardExports []boardExport
	for _, b := range boards {
		cards, _ := h.cards.ListByBoard(ctx, b.ID)
		if cards == nil {
			cards = []repo.Card{}
		}
		var allComments []repo.Comment
		for _, c := range cards {
			cmts, _ := h.comments.ListByCard(ctx, c.ID)
			allComments = append(allComments, cmts...)
		}
		if allComments == nil {
			allComments = []repo.Comment{}
		}
		boardExports = append(boardExports, boardExport{Board: b, Cards: cards, Comments: allComments})
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=lwts-export.json")
	json.NewEncoder(w).Encode(map[string]any{
		"users":      users,
		"boards":     boardExports,
		"exported_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// ── Import ──

func (h *Handler) ImportJira(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	ctx := r.Context()

	var body struct {
		BoardName string `json:"board_name"`
		Cards     []struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Priority    string `json:"priority"`
			ColumnID    string `json:"column_id"`
		} `json:"cards"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.BoardName == "" {
		body.BoardName = "Jira Import"
	}

	board, err := h.boards.Create(ctx, body.BoardName, "JIRA", user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create board: "+err.Error())
		return
	}

	count := 0
	for _, c := range body.Cards {
		col := c.ColumnID
		if col == "" {
			col = "backlog"
		}
		pri := c.Priority
		if pri == "" {
			pri = "medium"
		}
		_, err := h.cards.Create(ctx, board.ID, repo.CardCreate{
			ColumnID:    col,
			Title:       c.Title,
			Description: c.Description,
			Priority:    pri,
			ReporterID:  &user.ID,
		})
		if err == nil {
			count++
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"board_id":       board.ID,
		"cards_imported": count,
	})
}

func (h *Handler) ImportTrello(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	ctx := r.Context()

	var body struct {
		Name  string `json:"name"`
		Lists []struct {
			Name  string `json:"name"`
			Cards []struct {
				Name string `json:"name"`
				Desc string `json:"desc"`
			} `json:"cards"`
		} `json:"lists"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid Trello JSON")
		return
	}
	if body.Name == "" {
		body.Name = "Trello Import"
	}

	board, err := h.boards.Create(ctx, body.Name, "TRLO", user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "create board: "+err.Error())
		return
	}

	// Map Trello list names to LWTS columns
	colMap := map[string]string{
		"to do": "todo", "todo": "todo",
		"doing": "in-progress", "in progress": "in-progress",
		"done": "done", "complete": "done",
	}

	count := 0
	for _, list := range body.Lists {
		col := "backlog"
		if mapped, ok := colMap[toLower(list.Name)]; ok {
			col = mapped
		}
		for _, c := range list.Cards {
			_, err := h.cards.Create(ctx, board.ID, repo.CardCreate{
				ColumnID:    col,
				Title:       c.Name,
				Description: c.Desc,
				ReporterID:  &user.ID,
			})
			if err == nil {
				count++
			}
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"board_id":       board.ID,
		"cards_imported": count,
	})
}

// ── Danger Zone ──

func (h *Handler) ClearAllCards(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	boards, _ := h.boards.List(ctx)
	for _, b := range boards {
		cards, _ := h.cards.ListByBoard(ctx, b.ID)
		for _, c := range cards {
			_ = h.cards.Delete(ctx, c.ID)
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "all cards cleared"})
}

func (h *Handler) ResetWorkspace(w http.ResponseWriter, r *http.Request) {
	currentUser := auth.UserFromContext(r.Context())
	ctx := r.Context()

	var body struct {
		Mode string `json:"mode"` // "empty" or "demo"
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		body.Mode = "empty"
	}
	if body.Mode != "demo" {
		body.Mode = "empty"
	}

	// Delete all boards (cascades to cards, comments, webhooks)
	boards, _ := h.boards.List(ctx)
	for _, b := range boards {
		_ = h.boards.Delete(ctx, b.ID)
	}

	// Delete all users except the requesting user
	users, _ := h.users.List(ctx)
	for _, u := range users {
		if u.ID != currentUser.ID {
			_ = h.users.Delete(ctx, u.ID)
		}
	}

	// Clear ancillary data
	_, _ = h.ds.Exec(ctx, "DELETE FROM settings")
	_, _ = h.ds.Exec(ctx, "DELETE FROM api_keys")
	_, _ = h.ds.Exec(ctx, "DELETE FROM invites")
	// Clear refresh tokens for deleted users (keep current user's)
	_, _ = h.ds.Exec(ctx, "DELETE FROM refresh_tokens WHERE user_id != $1", currentUser.ID)

	// Ensure requesting user is owner
	ownerRole := "owner"
	_, _ = h.users.Update(ctx, currentUser.ID, repo.UserUpdate{Role: &ownerRole})

	// Re-seed demo data if requested
	if body.Mode == "demo" && h.seedFunc != nil {
		if err := h.seedFunc(ctx, currentUser.ID); err != nil {
			writeErr(w, http.StatusInternalServerError, "seed demo: "+err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "reset complete", "mode": body.Mode})
}

// WorkspaceStatus returns whether the workspace is initialized and has demo data.
func (h *Handler) WorkspaceStatus(w http.ResponseWriter, r *http.Request) {
	users, _ := h.users.List(r.Context())
	boards, _ := h.boards.List(r.Context())
	initialized := len(users) > 0
	hasDemo := false
	for _, b := range boards {
		if b.ProjectKey == "LWTS" {
			hasDemo = true
			break
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"initialized": initialized,
		"has_demo":    hasDemo,
	})
}

// ── Helpers ──

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// Ensure imports are used
var _ = fmt.Sprintf
