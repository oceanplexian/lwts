package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/oceanplexian/lwts/server/internal/db"
	"github.com/oceanplexian/lwts/server/internal/repo"
)

// Event types
const (
	EventCardCreated  = "card.created"
	EventCardAssigned = "card.assigned"
	EventCardMoved    = "card.moved"
	EventCardDone     = "card.done"
	EventCardPriority = "card.priority"
	EventCommentAdded = "comment.added"
)

// Colors for embeds
const (
	ColorGreen  = 0x4ade80 // created, done
	ColorBlue   = 0x82B1FF // assigned
	ColorOrange = 0xfb8c00 // priority escalation
	ColorPurple = 0xa78bfa // comment
)

type Event struct {
	Type     string
	Card     repo.Card
	Board    repo.Board
	User     repo.User // user who performed the action
	Comment  *repo.Comment
	OldValue string // e.g. old column, old priority
}

type notifyPrefs struct {
	Assigned bool `json:"assigned"`
	Done     bool `json:"done"`
	Comment  bool `json:"comment"`
	Created  bool `json:"created"`
	Priority bool `json:"priority"`
}

type config struct {
	BotToken  string
	ChannelID string
	Enabled   bool
	Notify    notifyPrefs
	BaseURL   string
}

type Notifier struct {
	ds     db.Datasource
	users  *repo.UserRepository
	events chan Event
	client *http.Client
	logger *slog.Logger
	stopCh chan struct{}
	wg     sync.WaitGroup

	// Dedup: buffer events briefly to consolidate
	mu      sync.Mutex
	pending map[string]*pendingEvent // keyed by cardID
	flushCh chan struct{}
}

type pendingEvent struct {
	events  []Event
	created time.Time
}

func NewNotifier(ds db.Datasource, users *repo.UserRepository, logger *slog.Logger) *Notifier {
	return &Notifier{
		ds:      ds,
		users:   users,
		events:  make(chan Event, 100),
		client:  &http.Client{Timeout: 10 * time.Second},
		logger:  logger,
		stopCh:  make(chan struct{}),
		pending: make(map[string]*pendingEvent),
		flushCh: make(chan struct{}, 1),
	}
}

func (n *Notifier) Emit(e Event) {
	select {
	case n.events <- e:
	default:
		n.logger.Warn("discord event channel full, dropping", "type", e.Type, "card", e.Card.Key)
	}
}

func (n *Notifier) Run() {
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-n.stopCh:
				n.flush()
				return
			case e := <-n.events:
				n.buffer(e)
			case <-ticker.C:
				n.flush()
			}
		}
	}()
}

func (n *Notifier) Stop() {
	close(n.stopCh)
	n.wg.Wait()
}

func (n *Notifier) buffer(e Event) {
	n.mu.Lock()
	defer n.mu.Unlock()
	key := e.Card.ID
	if _, ok := n.pending[key]; !ok {
		n.pending[key] = &pendingEvent{created: time.Now()}
	}
	n.pending[key].events = append(n.pending[key].events, e)
}

func (n *Notifier) flush() {
	n.mu.Lock()
	batch := n.pending
	n.pending = make(map[string]*pendingEvent)
	n.mu.Unlock()

	if len(batch) == 0 {
		return
	}

	cfg := n.loadConfig()
	if !cfg.Enabled || cfg.BotToken == "" || cfg.ChannelID == "" {
		return
	}

	for _, pe := range batch {
		n.processEvents(cfg, pe.events)
	}
}

func (n *Notifier) loadConfig() config {
	var cfg config
	var enabled int
	var notifyJSON string
	err := n.ds.QueryRow(context.Background(),
		"SELECT bot_token, channel_id, enabled, notify FROM discord_integrations LIMIT 1").
		Scan(&cfg.BotToken, &cfg.ChannelID, &enabled, &notifyJSON)
	if err != nil {
		return config{}
	}
	cfg.Enabled = enabled != 0
	cfg.Notify = notifyPrefs{Assigned: true, Done: true, Comment: true, Priority: true}
	if notifyJSON != "" {
		json.Unmarshal([]byte(notifyJSON), &cfg.Notify)
	}

	// Load base URL from general settings
	var generalJSON string
	err = n.ds.QueryRow(context.Background(),
		"SELECT value FROM settings WHERE key = 'general'").Scan(&generalJSON)
	if err == nil {
		var general map[string]any
		if json.Unmarshal([]byte(generalJSON), &general) == nil {
			if v, ok := general["base_url"].(string); ok {
				cfg.BaseURL = strings.TrimRight(v, "/")
			}
		}
	}
	return cfg
}

func (n *Notifier) processEvents(cfg config, events []Event) {
	var (
		hasCreated  bool
		hasAssigned bool
		hasDone     bool
		hasPriority bool
		hasComment  bool
		card        repo.Card
		board       repo.Board
		actor       repo.User
		comment     *repo.Comment
		oldColumn   string
		oldPriority string
	)

	for _, e := range events {
		card = e.Card
		board = e.Board
		actor = e.User
		switch e.Type {
		case EventCardCreated:
			hasCreated = true
		case EventCardAssigned:
			hasAssigned = true
		case EventCardDone:
			hasDone = true
			oldColumn = e.OldValue
		case EventCardPriority:
			hasPriority = true
			oldPriority = e.OldValue
		case EventCardMoved:
			oldColumn = e.OldValue
		case EventCommentAdded:
			hasComment = true
			comment = e.Comment
		}
	}

	// Resolve assignee name
	assigneeName := ""
	if card.AssigneeID != nil && *card.AssigneeID != "" {
		if u, err := n.users.GetByID(context.Background(), *card.AssigneeID); err == nil {
			assigneeName = u.Name
		}
	}

	// Build ticket URL
	ticketURL := ""
	if cfg.BaseURL != "" {
		ticketURL = fmt.Sprintf("%s/#card/%s", cfg.BaseURL, card.ID)
	}

	var embeds []embed

	// 1. Card created (consolidates assignment + priority)
	if hasCreated && cfg.Notify.Created {
		e := n.buildCreatedEmbed(card, board, actor, assigneeName, ticketURL)
		embeds = append(embeds, e)
		hasAssigned = false
		hasPriority = false
	}

	// 2. Card assigned (standalone)
	if hasAssigned && cfg.Notify.Assigned {
		e := n.buildAssignedEmbed(card, board, actor, assigneeName, ticketURL)
		embeds = append(embeds, e)
	}

	// 3. Card moved to done
	if hasDone && cfg.Notify.Done {
		e := n.buildDoneEmbed(card, board, actor, assigneeName, oldColumn, ticketURL)
		embeds = append(embeds, e)
		hasPriority = false
	}

	// 4. Priority escalated
	if hasPriority && cfg.Notify.Priority {
		e := n.buildPriorityEmbed(card, board, actor, assigneeName, oldPriority, ticketURL)
		embeds = append(embeds, e)
	}

	// 5. Comment added
	if hasComment && cfg.Notify.Comment && comment != nil {
		e := n.buildCommentEmbed(card, board, actor, comment, ticketURL)
		embeds = append(embeds, e)
	}

	if len(embeds) == 0 {
		return
	}

	n.send(cfg, embeds)
}

// ── Embed Builders ──

func (n *Notifier) buildCreatedEmbed(card repo.Card, board repo.Board, actor repo.User, assignee, url string) embed {
	// Build a compact metadata line
	meta := fmt.Sprintf("**Board:** %s \u2022 **Status:** %s", board.Name, formatColumn(card.ColumnID))
	if assignee != "" {
		meta += fmt.Sprintf(" \u2022 **Assigned:** %s", assignee)
	}
	if card.Priority != "" && card.Priority != "none" {
		meta += fmt.Sprintf("\n**Priority:** %s", formatPriority(card.Priority))
	}

	desc := meta
	if card.Description != "" {
		desc += "\n\n" + truncate(card.Description, 200)
	}

	title := card.Title
	if url != "" {
		title = fmt.Sprintf("[%s](%s)", card.Title, url)
	}

	return embed{
		Author:      &embedAuthor{Name: "\u2728 New ticket \u2022 " + card.Key},
		Description: title + "\n\n" + desc,
		Color:       ColorGreen,
		Footer:      &embedFooter{Text: actor.Name},
		Timestamp:   card.CreatedAt.Format(time.RFC3339),
	}
}

func (n *Notifier) buildAssignedEmbed(card repo.Card, board repo.Board, actor repo.User, assignee, url string) embed {
	title := card.Title
	if url != "" {
		title = fmt.Sprintf("[%s](%s)", card.Title, url)
	}

	desc := title + "\n\n"
	desc += fmt.Sprintf("**Assigned to:** %s\n", assignee)
	desc += fmt.Sprintf("**Board:** %s \u2022 **Status:** %s", board.Name, formatColumn(card.ColumnID))

	return embed{
		Author:      &embedAuthor{Name: "\U0001F464 Assigned \u2022 " + card.Key},
		Description: desc,
		Color:       ColorBlue,
		Footer:      &embedFooter{Text: "by " + actor.Name},
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}
}

func (n *Notifier) buildDoneEmbed(card repo.Card, board repo.Board, actor repo.User, assignee, fromColumn, url string) embed {
	title := card.Title
	if url != "" {
		title = fmt.Sprintf("[%s](%s)", card.Title, url)
	}

	desc := title + "\n\n"
	desc += fmt.Sprintf("**Board:** %s", board.Name)
	if fromColumn != "" {
		desc += fmt.Sprintf(" \u2022 %s \u2192 Done", formatColumn(fromColumn))
	}
	if assignee != "" {
		desc += fmt.Sprintf("\n**Owner:** %s", assignee)
	}

	return embed{
		Author:      &embedAuthor{Name: "\u2705 Completed \u2022 " + card.Key},
		Description: desc,
		Color:       ColorGreen,
		Footer:      &embedFooter{Text: "by " + actor.Name},
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}
}

func (n *Notifier) buildPriorityEmbed(card repo.Card, board repo.Board, actor repo.User, assignee, oldPriority, url string) embed {
	title := card.Title
	if url != "" {
		title = fmt.Sprintf("[%s](%s)", card.Title, url)
	}

	desc := title + "\n\n"
	desc += fmt.Sprintf("%s \u2192 %s\n", formatPriority(oldPriority), formatPriority(card.Priority))
	desc += fmt.Sprintf("**Board:** %s \u2022 **Status:** %s", board.Name, formatColumn(card.ColumnID))
	if assignee != "" {
		desc += fmt.Sprintf("\n**Assigned:** %s", assignee)
	}

	return embed{
		Author:      &embedAuthor{Name: "\u26A0\uFE0F Priority escalated \u2022 " + card.Key},
		Description: desc,
		Color:       ColorOrange,
		Footer:      &embedFooter{Text: "by " + actor.Name},
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}
}

func (n *Notifier) buildCommentEmbed(card repo.Card, board repo.Board, actor repo.User, cmt *repo.Comment, url string) embed {
	cardRef := card.Key
	if url != "" {
		cardRef = fmt.Sprintf("[%s](%s)", card.Key, url)
	}

	body := truncate(cmt.Body, 400)
	desc := fmt.Sprintf("> %s\n\n", strings.ReplaceAll(body, "\n", "\n> "))
	desc += fmt.Sprintf("on %s \u2022 %s", cardRef, card.Title)

	return embed{
		Author:      &embedAuthor{Name: "\U0001F4AC " + actor.Name + " commented"},
		Description: desc,
		Color:       ColorPurple,
		Footer:      &embedFooter{Text: board.Name},
		Timestamp:   cmt.CreatedAt.Format(time.RFC3339),
	}
}

// ── Discord API Types ──

type embed struct {
	Author      *embedAuthor `json:"author,omitempty"`
	Title       string       `json:"title,omitempty"`
	Description string       `json:"description,omitempty"`
	URL         string       `json:"url,omitempty"`
	Color       int          `json:"color,omitempty"`
	Fields      []embedField `json:"fields,omitempty"`
	Footer      *embedFooter `json:"footer,omitempty"`
	Timestamp   string       `json:"timestamp,omitempty"`
}

type embedAuthor struct {
	Name string `json:"name"`
}

type embedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type embedFooter struct {
	Text string `json:"text"`
}

type discordMessage struct {
	Embeds []embed `json:"embeds"`
}

func (n *Notifier) send(cfg config, embeds []embed) {
	msg := discordMessage{Embeds: embeds}
	body, _ := json.Marshal(msg)

	req, _ := http.NewRequest("POST",
		fmt.Sprintf("https://discord.com/api/v10/channels/%s/messages", cfg.ChannelID),
		bytes.NewReader(body))
	req.Header.Set("Authorization", "Bot "+cfg.BotToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		n.logger.Error("discord send failed", "error", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		n.logger.Error("discord API error", "status", resp.StatusCode)
	}
}

// ── Helpers ──

func formatColumn(col string) string {
	col = strings.ReplaceAll(col, "-", " ")
	col = strings.ReplaceAll(col, "_", " ")
	if len(col) > 0 {
		return strings.ToUpper(col[:1]) + col[1:]
	}
	return col
}

func formatPriority(p string) string {
	switch p {
	case "urgent":
		return "\U0001F534 Urgent"
	case "high":
		return "\U0001F7E0 High"
	case "medium":
		return "\U0001F7E1 Medium"
	case "low":
		return "\U0001F7E2 Low"
	case "none", "":
		return "\u26AA None"
	default:
		if len(p) > 0 {
			return strings.ToUpper(p[:1]) + p[1:]
		}
		return p
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\u2026"
}
