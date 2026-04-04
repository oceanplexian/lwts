package settings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/oceanplexian/lwts/server/internal/db"
	"github.com/google/uuid"
)

// ── Discord Integration ──

type notifyPrefs struct {
	Assigned bool `json:"assigned"`
	Done     bool `json:"done"`
	Comment  bool `json:"comment"`
	Created  bool `json:"created"`
	Priority bool `json:"priority"`
}

type discordConfig struct {
	ID        string      `json:"id"`
	BotToken  string      `json:"bot_token"`
	GuildID   string      `json:"guild_id"`
	ChannelID string      `json:"channel_id"`
	Enabled   bool        `json:"enabled"`
	Notify    notifyPrefs `json:"notify"`
}

var defaultNotify = notifyPrefs{Assigned: true, Done: true, Comment: true, Created: false, Priority: true}

func (h *Handler) RegisterDiscordRoutes(mux *http.ServeMux, adminMW func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/integrations/discord", adminMW(http.HandlerFunc(h.GetDiscord)))
	mux.Handle("PUT /api/v1/integrations/discord", adminMW(http.HandlerFunc(h.PutDiscord)))
	mux.Handle("POST /api/v1/integrations/discord/test", adminMW(http.HandlerFunc(h.TestDiscord)))
}

func scanDiscordRow(h *Handler, r *http.Request) (discordConfig, error) {
	var cfg discordConfig
	var enabled int
	var notifyJSON string
	err := h.ds.QueryRow(r.Context(),
		"SELECT id, bot_token, guild_id, channel_id, enabled, notify FROM discord_integrations LIMIT 1").
		Scan(&cfg.ID, &cfg.BotToken, &cfg.GuildID, &cfg.ChannelID, &enabled, &notifyJSON)
	cfg.Enabled = enabled != 0
	cfg.Notify = defaultNotify
	if notifyJSON != "" {
		_ = json.Unmarshal([]byte(notifyJSON), &cfg.Notify)
	}
	return cfg, err
}

func (h *Handler) GetDiscord(w http.ResponseWriter, r *http.Request) {
	cfg, err := scanDiscordRow(h, r)
	if err == db.ErrNoRows {
		writeJSON(w, http.StatusOK, discordConfig{Notify: defaultNotify})
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "get discord config: "+err.Error())
		return
	}
	// Mask the bot token for display
	if len(cfg.BotToken) > 14 {
		cfg.BotToken = cfg.BotToken[:10] + "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022" + cfg.BotToken[len(cfg.BotToken)-4:]
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *Handler) PutDiscord(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BotToken  *string      `json:"bot_token"`
		GuildID   *string      `json:"guild_id"`
		ChannelID *string      `json:"channel_id"`
		Enabled   *bool        `json:"enabled"`
		Notify    *notifyPrefs `json:"notify"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	cfg, err := scanDiscordRow(h, r)
	isNew := err == db.ErrNoRows
	if isNew {
		cfg.ID = uuid.New().String()
		cfg.Notify = defaultNotify
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, "get discord config: "+err.Error())
		return
	}

	if body.BotToken != nil {
		cfg.BotToken = *body.BotToken
	}
	if body.GuildID != nil {
		cfg.GuildID = *body.GuildID
	}
	if body.ChannelID != nil {
		cfg.ChannelID = *body.ChannelID
	}
	if body.Enabled != nil {
		cfg.Enabled = *body.Enabled
	}
	if body.Notify != nil {
		cfg.Notify = *body.Notify
	}

	now := time.Now().UTC()
	enabledInt := 0
	if cfg.Enabled {
		enabledInt = 1
	}
	notifyJSON, _ := json.Marshal(cfg.Notify)

	if isNew {
		_, err = h.ds.Exec(r.Context(),
			`INSERT INTO discord_integrations (id, bot_token, guild_id, channel_id, enabled, notify, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			cfg.ID, cfg.BotToken, cfg.GuildID, cfg.ChannelID, enabledInt, string(notifyJSON), now, now)
	} else {
		_, err = h.ds.Exec(r.Context(),
			`UPDATE discord_integrations SET bot_token=$1, guild_id=$2, channel_id=$3, enabled=$4, notify=$5, updated_at=$6 WHERE id=$7`,
			cfg.BotToken, cfg.GuildID, cfg.ChannelID, enabledInt, string(notifyJSON), now, cfg.ID)
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "save discord config: "+err.Error())
		return
	}

	// Mask token in response
	resp := cfg
	if len(resp.BotToken) > 14 {
		resp.BotToken = resp.BotToken[:10] + "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022" + resp.BotToken[len(resp.BotToken)-4:]
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) TestDiscord(w http.ResponseWriter, r *http.Request) {
	cfg, err := scanDiscordRow(h, r)
	if err == db.ErrNoRows {
		writeErr(w, http.StatusBadRequest, "Discord integration not configured yet. Save your settings first.")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "get discord config: "+err.Error())
		return
	}

	if cfg.BotToken == "" || cfg.ChannelID == "" {
		writeErr(w, http.StatusBadRequest, "Bot token and channel ID are required")
		return
	}

	msgBody, _ := json.Marshal(map[string]string{
		"content": "LWTS Kanban connected successfully! Board notifications will appear here.",
	})

	req, _ := http.NewRequestWithContext(r.Context(), "POST",
		fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", cfg.ChannelID),
		bytes.NewReader(msgBody))
	req.Header.Set("Authorization", "Bot "+cfg.BotToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "Failed to reach Discord API: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		var discordErr struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		}
		_ = json.Unmarshal(body, &discordErr)
		msg := "Discord API error"
		if discordErr.Message != "" {
			msg = discordErr.Message
		}
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			msg = "Bot token is invalid or the bot doesn't have permission to post in this channel"
		}
		if discordErr.Code == 10003 {
			msg = "Channel not found — check the channel ID"
		}
		writeErr(w, http.StatusBadRequest, msg)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "Message sent successfully"})
}
