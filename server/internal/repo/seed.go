package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/oceanplexian/lwts/server/internal/db"
)

// SeedDemo creates a demo board with sample cards, multiple users, comments,
// and related-ticket links. ownerID must already exist.
func SeedDemo(ctx context.Context, ds db.Datasource, ownerID string) error {
	boards := NewBoardRepository(ds)
	cards := NewCardRepository(ds)
	comments := NewCommentRepository(ds)
	users := NewUserRepository(ds)

	// ── Create additional team members ──
	type seedUser struct {
		Name  string
		Email string
		ID    string // filled after creation
	}
	extraUsers := []seedUser{
		{Name: "Sarah Chen", Email: "sarah@lwts.dev"},
		{Name: "Marcus Rivera", Email: "marcus@lwts.dev"},
		{Name: "Priya Patel", Email: "priya@lwts.dev"},
	}
	userIDs := map[string]string{"owner": ownerID}
	for i, u := range extraUsers {
		created, err := users.Create(ctx, u.Name, u.Email, "$2a$10$placeholder_hash_not_for_login")
		if err != nil {
			return fmt.Errorf("create user %q: %w", u.Name, err)
		}
		extraUsers[i].ID = created.ID
		userIDs[u.Email] = created.ID
	}
	sarahID := extraUsers[0].ID
	marcusID := extraUsers[1].ID
	priyaID := extraUsers[2].ID

	// Set avatar images for all users
	avatarMap := map[string]string{
		ownerID:  "/avatars/admin.png",
		sarahID:  "/avatars/sarah.png",
		marcusID: "/avatars/marcus.png",
		priyaID:  "/avatars/priya.png",
	}
	for uid, url := range avatarMap {
		avatarURL := url
		if _, err := users.Update(ctx, uid, UserUpdate{AvatarURL: &avatarURL}); err != nil {
			return fmt.Errorf("set avatar for user: %w", err)
		}
	}

	board, err := boards.Create(ctx, "kanban", "LWTS", ownerID)
	if err != nil {
		return fmt.Errorf("create board: %w", err)
	}

	type seedComment struct {
		Body   string
		Author string // "owner", or email key
	}
	type seedCard struct {
		Column   string
		Title    string
		Tag      string
		Priority string
		Points   int
		DueDate  string
		Desc     string
		Assignee string // user ID
		Reporter string // user ID
		Comments []seedComment
		Related  []int // indices into seedCards for linking after creation
	}

	seedCards := []seedCard{
		// ════════════════════════════════════════════════════════════
		// BACKLOG — 8 cards (indices 0-7)
		// ════════════════════════════════════════════════════════════

		// 0
		{Column: "backlog", Title: "Add dark/light theme toggle", Tag: "blue", Priority: "low", Points: 3, DueDate: "2026-04-15",
			Assignee: sarahID, Reporter: ownerID,
			Desc: "Users have requested a theme toggle in the header. Should persist preference to localStorage and sync with the system preference on first visit.\n\n## Acceptance criteria\n- Toggle button in user menu (three-way: Auto / Light / Dark)\n- Smooth CSS transition between themes — no flash of unstyled content\n- Persists across sessions via localStorage\n- Respects `prefers-color-scheme` on first visit when set to Auto\n- All color tokens defined in `:root` with `[data-theme=\"light\"]` overrides\n\n## Design notes\nLight palette is in Figma under \"LWTS / Tokens / Light\". The background should be `#f8f9fa`, not pure white — pure white causes eye strain on large monitors.",
			Comments: []seedComment{
				{Body: "Should we support auto/light/dark or just a light/dark toggle?", Author: "owner"},
				{Body: "Three-way toggle. Auto as default, then user can override. Same pattern as GitHub and Linear.", Author: sarahID},
				{Body: "I'll need the design tokens from Figma before starting this. @sarah can you export the light palette as CSS variables?", Author: marcusID},
				{Body: "Exported — see `tokens-light.css` in the #design channel. 47 variables total. I kept the naming consistent with our dark theme vars.", Author: sarahID},
			},
			Related: []int{16}}, // related to card detail modal redesign

		// 1
		{Column: "backlog", Title: "Investigate ES shard rebalancing after disk watermark breach", Tag: "orange", Priority: "medium", Points: 5, DueDate: "2026-04-12",
			Assignee: priyaID, Reporter: ownerID,
			Desc: "Cluster went yellow twice last week when east-1 hit the high disk watermark (90%). Need to investigate thresholds and possibly adjust shard allocation awareness.\n\n## Current state\n- 4 nodes across 2 regions (east-0, east-1, west-0, west-1)\n- Forced awareness: `node.attr.region: east|west`\n- 2 primary shards, 1 replica each\n- Disk usage: east-0 at 84%, east-1 at 91% (over high watermark)\n- Index `et_segments`: ~8M docs, ~415 GB primary\n\n## Investigation plan\n1. Check `_cluster/allocation/explain` for blocked shard moves\n2. Review segment count — may need force merge to reclaim deleted doc space\n3. Consider lowering `cluster.routing.allocation.disk.watermark.high` to 85%\n4. Long-term: add a 5th node or move to larger disks\n\n## Useful commands\n```\ncurl \"http://west.internal.lwts.dev:9200/_cluster/allocation/explain?pretty\"\ncurl \"http://west.internal.lwts.dev:9200/_cat/segments/et_segments?v&h=shard,segment,size,generation\"\n```",
			Comments: []seedComment{
				{Body: "east-1 crossed 90% again this morning. I force-merged the et_segments index — went from 129 segments to 2. Freed about 40GB.", Author: priyaID},
				{Body: "That buys us time but doesn't fix the root cause. We're adding ~15GB/quarter from new transcripts. Need a node expansion plan.", Author: "owner"},
			}},

		// 2
		{Column: "backlog", Title: "Mobile responsive layout for board and list views", Tag: "blue", Priority: "low", Points: 8, DueDate: "2026-04-20",
			Assignee: sarahID, Reporter: marcusID,
			Desc: "The board view doesn't work on tablets or phones. Cards overflow horizontally and column headers are cut off. The list view is slightly better but still needs work.\n\n## Proposed approach\n### Mobile (<768px)\n- Single-column stacked view\n- Column selector as horizontal scrollable pills at the top\n- Cards show in selected column only\n- Swipe left/right to change columns\n\n### Tablet (768px–1024px)\n- Two columns visible at a time with horizontal scroll\n- Slightly narrower cards (min-width: 200px)\n\n### Desktop (>1024px)\n- Current layout unchanged\n\n## Breakpoints\nUse CSS container queries where possible, `@media` as fallback. All breakpoints defined as CSS custom properties in `theme.css`.",
			Comments: []seedComment{
				{Body: "I tested on my iPad — the board is completely unusable. You can only see the first column and there's no way to scroll horizontally.", Author: marcusID},
				{Body: "The list view actually works OK on tablet if we just reduce padding. Mobile is the bigger problem.", Author: sarahID},
			}},

		// 3
		{Column: "backlog", Title: "Set up end-to-end test suite with Playwright", Tag: "orange", Priority: "low", Points: 13, DueDate: "2026-05-01",
			Assignee: marcusID, Reporter: ownerID,
			Desc: "We have zero automated UI tests. Every deploy is a manual smoke test. Set up Playwright with a test database, seed data, and cover the critical user paths.\n\n## Test cases (priority order)\n1. User registration → login → see board\n2. Create card → verify it appears in correct column\n3. Edit card title/description/priority → verify persistence\n4. Drag card between columns → verify column change\n5. Delete card → verify removal\n6. Board switching → verify cards load\n7. Search → verify results\n8. Comment creation → verify comment appears\n9. List view → verify sort works\n10. Settings → theme toggle → verify colors change\n\n## Infrastructure\n- SQLite for test runs (fast spin-up, no external deps)\n- Fresh `SeedDemo` before each test suite\n- Run in CI on every PR via GitHub Actions\n- Screenshot comparison for visual regression\n\n## Open questions\n- Do we test both light and dark themes? (Yes — we've had CSS regressions that only showed in one mode)\n- Headless vs headed in CI? (Headless, but `--headed` flag for local debugging)",
			Comments: []seedComment{
				{Body: "Let's use SQLite for test runs — faster spin-up, no external dependencies.", Author: marcusID},
				{Body: "Agreed. I'll set up a fixture system that runs SeedDemo before each test suite.", Author: "owner"},
				{Body: "Make sure we test with both light and dark themes — we had a CSS regression last month that only showed in light mode. The card borders were invisible.", Author: sarahID},
				{Body: "Can we also add visual regression screenshots? Playwright has built-in support. We compare against golden images and flag any pixel diff > 0.1%.", Author: priyaID},
				{Body: "Good call. I'll add `toMatchSnapshot()` assertions for the board view, list view, and card detail modal.", Author: marcusID},
			}},

		// 4
		{Column: "backlog", Title: "Webhook notifications for card events", Tag: "blue", Priority: "lowest", Points: 5, DueDate: "2026-05-15",
			Assignee: priyaID, Reporter: sarahID,
			Desc: "Allow users to configure webhook URLs that receive POST requests when card events occur. This enables integration with Slack, Discord, PagerDuty, and custom automation.\n\n## Events to support\n- `card.created` — new card added\n- `card.updated` — any field changed (title, description, priority, assignee, etc.)\n- `card.moved` — card moved between columns\n- `card.deleted` — card removed\n- `comment.created` — new comment on a card\n\n## Payload format\n```json\n{\n  \"event\": \"card.moved\",\n  \"timestamp\": \"2026-04-03T12:00:00Z\",\n  \"board\": { \"id\": \"...\", \"name\": \"kanban\" },\n  \"card\": { \"id\": \"...\", \"key\": \"LWTS-15\", \"title\": \"...\" },\n  \"actor\": { \"id\": \"...\", \"name\": \"Admin\" },\n  \"changes\": { \"column_id\": { \"from\": \"todo\", \"to\": \"in-progress\" } }\n}\n```\n\n## Delivery\n- Retry 3x with exponential backoff (1s, 5s, 25s)\n- HMAC-SHA256 signature in `X-LWTS-Signature` header\n- 5-second timeout per attempt\n- Dead letter queue for failed deliveries (visible in admin panel)",
			Comments: []seedComment{
				{Body: "We already have the webhook dispatcher infrastructure from the notification system. We just need to wire up the card events.", Author: priyaID},
				{Body: "Should we add a webhook test button in the settings UI? Sends a sample payload so users can verify their endpoint works.", Author: sarahID},
			}},

		// 5
		{Column: "backlog", Title: "Audit log for admin actions", Tag: "orange", Priority: "low", Points: 8, DueDate: "2026-05-10",
			Assignee: ownerID, Reporter: priyaID,
			Desc: "Track all destructive and sensitive actions in an audit log table. Surface in the admin settings panel with filtering and search.\n\n## Actions to log\n- User created / deleted / role changed\n- Board created / deleted\n- Card deleted (not moves — too noisy)\n- Settings changed (theme, permissions, webhooks)\n- Password reset (admin-initiated)\n- API key generated / revoked\n- Workspace reset\n\n## Schema\n```sql\nCREATE TABLE audit_log (\n  id UUID PRIMARY KEY,\n  actor_id UUID REFERENCES users(id),\n  action TEXT NOT NULL,          -- 'user.deleted', 'board.created', etc.\n  resource_type TEXT NOT NULL,   -- 'user', 'board', 'card', 'settings'\n  resource_id TEXT,\n  metadata JSONB DEFAULT '{}',   -- action-specific details\n  ip_address TEXT,\n  created_at TIMESTAMPTZ DEFAULT NOW()\n);\n```\n\n## Retention\nKeep 90 days by default. Add a cron job to prune older entries.",
			Comments: []seedComment{
				{Body: "Should we log card deletions? They happen frequently and might be noisy.", Author: marcusID},
				{Body: "Yes — card deletion is destructive and not undoable. We should log it. Card moves we can skip.", Author: "owner"},
				{Body: "I'd add the IP address to the log too. Useful for security investigations.", Author: priyaID},
			}},

		// 6
		{Column: "backlog", Title: "Add keyboard shortcuts for power users", Tag: "blue", Priority: "lowest", Points: 3,
			Assignee: sarahID, Reporter: marcusID,
			Desc: "Power users (especially devs) want to navigate the board without touching the mouse. Add a keyboard shortcut system with a discoverable help overlay.\n\n## Shortcuts\n| Key | Action |\n|-----|--------|\n| `c` | Create new card |\n| `n` / `j` | Select next card in column |\n| `p` / `k` | Select previous card in column |\n| `1`–`4` | Move selected card to column 1–4 |\n| `Enter` | Open selected card detail |\n| `Esc` | Close modal / deselect |\n| `/` | Focus search bar |\n| `?` | Show keyboard shortcuts help overlay |\n| `b` | Open board switcher |\n| `l` | Toggle list/board view |\n\n## Implementation\n- Global `keydown` listener on `document`\n- Ignore when focus is in an input/textarea\n- Selected card gets a visible blue outline\n- Shortcuts overlay is a modal triggered by `?`\n- All shortcuts configurable in settings (stretch goal)",
			Comments: []seedComment{
				{Body: "Vim-style j/k for navigation would be a nice touch. Linear does this and it's great.", Author: marcusID},
			}},

		// 7
		{Column: "backlog", Title: "Rate limiting on auth endpoints", Tag: "red", Priority: "medium", Points: 2, DueDate: "2026-04-08",
			Assignee: priyaID, Reporter: ownerID,
			Desc: "Login and registration endpoints have no rate limiting. A brute-force attack could enumerate passwords or create thousands of accounts.\n\n## Proposal\n- **Login**: 5 attempts per minute per IP. On the 6th attempt, return `429 Too Many Requests` with `Retry-After: 60` header.\n- **Registration**: 3 attempts per hour per IP.\n- **Password reset**: 3 attempts per hour per email.\n\n## Implementation\n- In-memory sliding window counter (Go `sync.Map` + cleanup goroutine)\n- Key: `action:ip` or `action:email`\n- No Redis dependency — single-process is fine for now\n- Add middleware that runs before the auth handler\n- Log all rate-limited requests at WARN level\n\n## Security note\nDo NOT reveal whether an email exists in error messages. Always return generic \"invalid credentials\" for failed logins.",
			Comments: []seedComment{
				{Body: "Should we use in-memory or Redis for the rate limit counters?", Author: marcusID},
				{Body: "In-memory is fine — we're single-process. Can move to Redis if we scale horizontally, but that's not on the roadmap.", Author: "owner"},
				{Body: "Don't forget to exempt the health check endpoints from rate limiting. We hit ourselves with our own monitoring once.", Author: priyaID},
			},
			Related: []int{22}}, // related to JWT refresh token rotation

		// ════════════════════════════════════════════════════════════
		// TO DO — 6 cards (indices 8-13)
		// ════════════════════════════════════════════════════════════

		// 8
		{Column: "todo", Title: "Fix SSE heartbeat timeout on slow connections", Tag: "red", Priority: "highest", Points: 5, DueDate: "2026-04-05",
			Assignee: ownerID, Reporter: marcusID,
			Desc: "Clients on high-latency connections (>200ms RTT) drop after 15 seconds. The board \"freezes\" — no real-time updates until manual page refresh. Affecting ~12% of users based on error logs.\n\n## Root cause\nEnvoy's `BackendTrafficPolicy` has a default `idleTimeout` of 15 seconds on backend connections. Our SSE relay sends heartbeat pings every 30 seconds. The connection is killed between heartbeats.\n\n## Fix\n1. Set `idleTimeout: 600s` in the BackendTrafficPolicy for the SSE route (`/api/v1/boards/*/stream`)\n2. Do NOT reduce heartbeat interval — that masks the real issue and wastes bandwidth\n3. Add client-side reconnect with exponential backoff as defense-in-depth\n\n## Reconnect strategy\n```javascript\nlet delay = 1000; // 1s initial\nconst maxDelay = 30000; // 30s max\nfunction reconnect() {\n  setTimeout(() => {\n    connect();\n    delay = Math.min(delay * 2, maxDelay);\n  }, delay);\n}\n// Reset delay on successful connection\nfunction onOpen() { delay = 1000; }\n```\n\n## Validation\n- Test with `tc` to simulate 300ms latency\n- Verify connections survive for 10+ minutes\n- Check no duplicate events after reconnect (server sends last event ID)",
			Comments: []seedComment{
				{Body: "Confirmed — seeing this in west region too. I checked the Envoy access logs and there are thousands of 408 responses on the stream endpoint.", Author: marcusID},
				{Body: "Root cause is Envoy BackendTrafficPolicy default. Need to bump to 600s. I'll push the Gateway API change.", Author: "owner"},
				{Body: "Can we also add a reconnect-with-backoff on the client side? Even with the Envoy fix, network blips happen.", Author: sarahID},
				{Body: "Good idea. I'll add exponential backoff: 1s, 2s, 4s, 8s, max 30s. Reset on successful connection.", Author: "owner"},
				{Body: "Tested the Envoy fix in staging. SSE connections now survive for 10+ minutes without dropping. Client-side reconnect also working — tested by killing the pod mid-stream.", Author: "owner"},
				{Body: "Deployed to prod. Monitoring shows zero 408s on the stream endpoint in the last 3 hours. Marking as ready for done.", Author: marcusID},
			},
			Related: []int{19}}, // related to SSE relay memory leak

		// 9
		{Column: "todo", Title: "Add FRED macro series to the economic dashboard", Tag: "blue", Priority: "medium", Points: 3, DueDate: "2026-04-06",
			Assignee: priyaID, Reporter: ownerID,
			Desc: "Add key FRED economic indicators to the macro overview page. The data should auto-refresh daily via the existing FRED refresh cron job.\n\n## Series to add\n| Series ID | Name | Frequency |\n|-----------|------|-----------|\n| UNRATE | Unemployment Rate | Monthly |\n| CPIAUCSL | Consumer Price Index | Monthly |\n| FEDFUNDS | Federal Funds Rate | Daily |\n| GDP | Gross Domestic Product | Quarterly |\n| T10Y2Y | 10Y-2Y Treasury Spread | Daily |\n| MORTGAGE30US | 30-Year Mortgage Rate | Weekly |\n| UMCSENT | Consumer Sentiment | Monthly |\n\n## Display\n- Sparkline chart for each series (last 2 years)\n- Latest value prominently displayed with direction arrow (↑/↓)\n- Color: green if improving, red if worsening (direction depends on series — lower unemployment is green)\n- Click to expand full historical chart\n\n## API\nUse the existing `POST /api/v1/fred/refresh` endpoint. The stock-server already has FRED integration — we just need to add these series to the config.",
			Comments: []seedComment{
				{Body: "Should we include the yield curve (T10Y2Y)? It's been inverted for a while and that's a recession indicator.", Author: priyaID},
				{Body: "Yes, include it. It's one of the most-watched indicators. Color it red when inverted (negative value).", Author: "owner"},
			}},

		// 10
		{Column: "todo", Title: "Migrate user avatars to S3-compatible object storage", Tag: "orange", Priority: "high", Points: 5, DueDate: "2026-04-07",
			Assignee: marcusID, Reporter: sarahID,
			Desc: "Currently avatar URLs point to external services (Gravatar, GitHub). We need to support custom avatar uploads stored in our own infrastructure.\n\n## Implementation steps\n1. **Upload endpoint**: `POST /api/v1/users/{id}/avatar`\n   - Accept multipart form data\n   - Validate: max 2MB, JPEG/PNG/WebP only\n   - Resize to 256×256 (sharp library or ImageMagick)\n   - Generate unique filename: `{user_id}_{timestamp}.webp`\n\n2. **Storage**: MinIO (S3-compatible, already running in cluster)\n   - Bucket: `lwts-avatars`\n   - Public read access via presigned URLs (24h expiry)\n   - Or serve through our CDN with `Cache-Control: max-age=86400`\n\n3. **Database**: Update `avatar_url` field on user model\n   - Keep supporting external URLs (Gravatar, GitHub)\n   - New uploads get `minio://lwts-avatars/{filename}` stored, resolved to HTTP URL on read\n\n4. **Frontend**: Add upload button to profile settings\n   - Drag-and-drop or click-to-select\n   - Client-side preview before upload\n   - Crop tool (circular mask)\n\n## Migration plan\nExisting users keep their Gravatar/GitHub URLs. No bulk migration needed — users can upload a custom avatar whenever they want.",
			Comments: []seedComment{
				{Body: "MinIO is already running in the cluster at `minio.internal.lwts.dev:9000`. Bucket `lwts-avatars` created and ready.", Author: priyaID},
				{Body: "Should we generate multiple sizes (32, 64, 128, 256) on upload or just serve the 256 and let the browser scale?", Author: marcusID},
				{Body: "Browser scaling is fine — the 256px WebP is only ~15KB. Let's not over-engineer this. We can add srcset later if needed.", Author: "owner"},
				{Body: "I'll add the crop tool to the frontend. Using `cropperjs` — it's 40KB gzipped but worth it for the UX.", Author: sarahID},
			}},

		// 11
		{Column: "todo", Title: "Fix card drag ghost image rendering on Firefox", Tag: "red", Priority: "medium", Points: 2, DueDate: "2026-04-08",
			Assignee: sarahID, Reporter: marcusID,
			Desc: "Firefox renders the drag ghost image with incorrect dimensions — it captures the entire column instead of just the card. This makes drag-and-drop unusable on Firefox because you can't see what you're dragging or where you're dropping.\n\n## Root cause\nFirefox's `setDragImage` has a bug when the source element has CSS `transform` applied (we use `transform` for the hover scale effect). It captures the nearest non-transformed ancestor, which is the column.\n\n## Workaround\n1. On `dragstart`, clone the card element\n2. Strip all transforms from the clone\n3. Position it offscreen (`position: absolute; left: -9999px`)\n4. Append to document body\n5. Use clone as the drag image via `e.dataTransfer.setDragImage(clone, ...)`\n6. Remove the clone on `dragend`\n\n## Browser matrix\n- Chrome 123: ✅ works\n- Safari 17: ✅ works\n- Firefox 124: ❌ broken (this bug)\n- Edge 123: ✅ works (Chromium-based)",
			Comments: []seedComment{
				{Body: "Repro'd on Firefox 124. The ghost image is literally the entire column — 800px tall. Hilarious but unusable.", Author: marcusID},
				{Body: "The clone workaround is ugly but it's the standard fix. Linear and Notion both do this.", Author: sarahID},
			}},

		// 12
		{Column: "todo", Title: "Implement live markdown preview in card descriptions", Tag: "blue", Priority: "low", Points: 3,
			Assignee: sarahID, Reporter: priyaID,
			Desc: "The description editor supports markdown syntax but there's no way to preview the rendered output without saving. Users are writing markdown blind and often get surprised by the formatting.\n\n## Requirements\n- Tab toggle between **Edit** and **Preview** modes\n- Preview renders in real-time as you type (debounced at 150ms)\n- Supported markdown features:\n  - Headers (h1-h3)\n  - Bold, italic, strikethrough\n  - Bullet and numbered lists\n  - Task lists with checkboxes `- [x]`\n  - Code blocks with syntax highlighting (using existing Prism.js)\n  - Links and images\n  - Tables\n  - Blockquotes\n- Editor toolbar with format buttons (bold, italic, code, link)\n- Keyboard shortcuts: `Ctrl+B` bold, `Ctrl+I` italic, `Ctrl+K` link\n\n## Technical approach\nUse the existing `marked` library for rendering. Add `DOMPurify` for XSS prevention on the rendered HTML. The editor is already a `<textarea>` — we just need the preview panel alongside it.",
			Comments: []seedComment{
				{Body: "Can we use CodeMirror instead of a plain textarea? It gives us syntax highlighting in the editor itself, not just the preview.", Author: marcusID},
				{Body: "CodeMirror is 200KB+ gzipped. Let's start with the textarea + preview approach and upgrade later if users ask for it.", Author: sarahID},
				{Body: "Don't forget to sanitize the rendered HTML. We had an XSS vulnerability in the old wiki through markdown injection.", Author: priyaID},
			}},

		// 13
		{Column: "todo", Title: "Database connection pool exhaustion under sustained load", Tag: "red", Priority: "high", Points: 5, DueDate: "2026-04-04",
			Assignee: ownerID, Reporter: priyaID,
			Desc: "Under sustained load (>50 concurrent users), the PostgreSQL connection pool exhausts and requests start failing with `pq: too many clients already` errors. This happened during the all-hands demo last week.\n\n## Current pool settings\n```go\ndb.SetMaxOpenConns(10)\ndb.SetMaxIdleConns(5)\ndb.SetConnMaxLifetime(5 * time.Minute)\n```\n\n## Problem\n10 max connections is far too low. Each SSE stream holds a connection for its lifetime (minutes to hours), so 10 SSE clients = pool exhausted. Regular HTTP requests then queue and timeout.\n\n## Fix (three-phase)\n### Phase 1: Quick fix (this sprint)\n- Bump `MaxOpenConns` to 25\n- Add pool metrics to `/healthz` endpoint (`db.Stats()` exposes `InUse`, `Idle`, `WaitCount`)\n\n### Phase 2: Connection pooler (next sprint)\n- Deploy pgbouncer as a sidecar\n- Transaction-level pooling so SSE connections don't hold backend connections\n- Helm chart is already prepared: `helm/pgbouncer/`\n\n### Phase 3: SSE connection optimization\n- SSE handler should not hold a DB connection\n- Read from an in-memory event bus, not directly from DB\n- The SSE hub already does this for broadcasts — just need to remove the DB ping in the heartbeat",
			Comments: []seedComment{
				{Body: "Hit this in prod yesterday during the all-hands demo. The CEO was showing the board and it just froze. Very embarrassing.", Author: priyaID},
				{Body: "Quick fix deployed: bumped MaxOpenConns to 25. Buys us runway but doesn't fix the SSE connection hogging.", Author: "owner"},
				{Body: "pgbouncer helm chart is ready. Just needs the connection string update in the deployment. I can push this today.", Author: marcusID},
				{Body: "Let's do it. I'll also add the pool metrics to /healthz so we can set up a PagerDuty alert before it happens again.", Author: "owner"},
			},
			Related: []int{19, 8}}, // related to SSE memory leak and SSE heartbeat

		// ════════════════════════════════════════════════════════════
		// IN PROGRESS — 5 cards (indices 14-18)
		// ════════════════════════════════════════════════════════════

		// 14
		{Column: "in-progress", Title: "Transcript parser batch mode for quarterly bulk import", Tag: "green", Priority: "high", Points: 8, DueDate: "2026-04-03",
			Assignee: priyaID, Reporter: ownerID,
			Desc: "Refactoring the transcript parser to process earnings call transcripts in batches of 100 instead of one-by-one. This is a critical path improvement — the quarterly bulk import currently takes 6 hours and needs to run in under 30 minutes.\n\n## Current approach (slow)\n```\nfor each transcript:\n  1. Parse text into segments    (~50ms)\n  2. Call embedding API           (~800ms per segment × 200 segments = 160s)\n  3. Insert into PostgreSQL        (~5ms per segment)\n  4. Index into Elasticsearch      (~10ms per segment)\n```\nTotal: ~3 minutes per transcript × 120 transcripts = 6 hours\n\n## New approach (batched)\n```\n1. Parse all 100 transcripts concurrently    (~2s total)\n2. Batch embed: 100 segments per API call    (~3s per call × 20 calls = 60s)\n3. Bulk insert via COPY                      (~500ms for 20k rows)\n4. Bulk index via ES _bulk API               (~2s for 20k docs)\n```\nTotal: ~65 seconds per batch of 100 × 2 batches = ~2 minutes\n\n## Progress\n- [x] Batch parsing logic (goroutine pool, max 10 concurrent)\n- [x] Batch embedding API call (chunks of 100, retries on 429)\n- [x] Bulk insert via COPY (using pg8000 native protocol)\n- [ ] Bulk Elasticsearch indexing via _bulk API\n- [ ] Error handling for partial failures\n- [ ] Metrics: processed/failed/skipped counts\n- [ ] Logging: structured JSON logs with transcript IDs",
			Comments: []seedComment{
				{Body: "Make sure to use native pg8000 `conn.run()` for the COPY command — the ORM's `executemany` is 10x slower.", Author: "owner"},
				{Body: "Batch embedding cuts API time from 45min to 4min for a full quarter. Huge win. The API supports up to 128 inputs per request.", Author: priyaID},
				{Body: "Found a bug: if one transcript in the batch fails to parse (malformed HTML), the entire batch is dropped. Need to handle partial failures gracefully.", Author: marcusID},
				{Body: "Fixed — now we log the failed transcript and continue with the rest. Failed items get queued to a `failed_transcripts` table for manual review.", Author: priyaID},
				{Body: "ES bulk indexing is done. Using `op_type=create` so existing docs are a no-op (no tombstones). Throughput: 5,000 docs/sec.", Author: priyaID},
			}},

		// 15
		{Column: "in-progress", Title: "Redesign card detail modal with two-column layout", Tag: "blue", Priority: "medium", Points: 5, DueDate: "2026-04-06",
			Assignee: sarahID, Reporter: ownerID,
			Desc: "The card detail modal needs a visual refresh. It's cramped, the description area is tiny, and comments have no threading.\n\n## Current issues\n- Sidebar fields (status, priority, assignee) are squeezed into a narrow column\n- Description textarea is only 3 lines tall — users can't see what they're writing\n- Comments are flat list — no way to reply to a specific comment\n- No activity log showing who changed what\n- Close button is hard to find\n\n## New design\n**Two-column layout**: Left (70%) for content, Right (30%) for metadata.\n\n### Left column\n1. Title (editable inline, larger font)\n2. Description (full-height markdown editor with preview toggle)\n3. Related tickets section\n4. Activity log / Comments (tabbed)\n\n### Right column\n1. Status dropdown\n2. Priority dropdown\n3. Assignee dropdown with avatar\n4. Reporter (read-only)\n5. Points input\n6. Due date picker\n7. Tags\n8. Created / Updated timestamps\n\n### Comments redesign\n- Threaded replies (one level deep)\n- Edit own comments (pencil icon)\n- Delete own comments (trash icon)\n- Markdown support in comments\n- @mentions with autocomplete",
			Comments: []seedComment{
				{Body: "Figma mockups are in the #design channel. I went with the Linear-style layout — clean and spacious.", Author: sarahID},
				{Body: "Should we add an activity log showing who changed what and when? Like 'Marcus changed priority from Medium to High'.", Author: "owner"},
				{Body: "Yes, that's in the Figma. It's a separate tab next to Comments. Each entry shows: actor, action, field, old value, new value, timestamp.", Author: sarahID},
				{Body: "The threaded comments are going to be complex. Can we ship v1 without threading and add it in a follow-up?", Author: marcusID},
				{Body: "Agreed. Let's scope v1 as: new layout + activity log + edit/delete comments. Threading is a separate ticket.", Author: "owner"},
			},
			Related: []int{0, 12}}, // related to theme toggle and markdown preview

		// 16
		{Column: "in-progress", Title: "Implement board-level permissions and access control", Tag: "orange", Priority: "high", Points: 8, DueDate: "2026-04-10",
			Assignee: ownerID, Reporter: priyaID,
			Desc: "Currently all authenticated users can see and edit all boards. This is a blocker for multi-team usage — teams don't want other teams modifying their boards.\n\n## Permission model\n| Role | View board | Edit cards | Manage columns | Delete board | Manage members |\n|------|-----------|------------|----------------|--------------|----------------|\n| Viewer | ✅ | ❌ | ❌ | ❌ | ❌ |\n| Member | ✅ | Own cards | ❌ | ❌ | ❌ |\n| Admin | ✅ | ✅ | ✅ | ❌ | ✅ |\n| Owner | ✅ | ✅ | ✅ | ✅ | ✅ |\n\n## Schema\n```sql\nCREATE TABLE board_members (\n  board_id UUID REFERENCES boards(id) ON DELETE CASCADE,\n  user_id UUID REFERENCES users(id) ON DELETE CASCADE,\n  role TEXT NOT NULL DEFAULT 'member',\n  created_at TIMESTAMPTZ DEFAULT NOW(),\n  PRIMARY KEY (board_id, user_id)\n);\n```\n\n## Implementation\n1. Add `board_members` table + migration\n2. Middleware: `RequireBoardAccess(minRole)` — checks membership before any board/card operation\n3. Board creation automatically adds creator as Owner\n4. UI: \"Members\" section in board settings with invite/remove/role-change\n5. API: `POST /api/v1/boards/{id}/members`, `DELETE /api/v1/boards/{id}/members/{userId}`",
			Comments: []seedComment{
				{Body: "This is the most-requested feature from the beta users. Three teams asked for it in the last week.", Author: priyaID},
				{Body: "The middleware approach is clean. I'll add it as a wrapper that runs after auth but before the handler.", Author: "owner"},
				{Body: "What about the global admin? Should they bypass board permissions?", Author: marcusID},
				{Body: "Yes — users with the 'owner' role at the system level (not board level) should have implicit access to all boards. Same as how GitHub org owners can see all repos.", Author: "owner"},
			}},

		// 17
		{Column: "in-progress", Title: "WebSocket migration for real-time collaboration", Tag: "orange", Priority: "medium", Points: 13, DueDate: "2026-04-15",
			Assignee: marcusID, Reporter: ownerID,
			Desc: "Migrating from SSE to WebSocket for bi-directional real-time communication. SSE is working but has limitations: unidirectional (server→client only), limited to 6 connections per domain in HTTP/1.1, and no binary support.\n\n## Migration plan (4 phases)\n\n### Phase 1: WebSocket endpoint ✅\n- Added `GET /ws/boards/{id}` endpoint\n- Same JSON event format as SSE\n- JWT auth via query parameter (`?token=...`)\n- Gorilla WebSocket library\n\n### Phase 2: Client migration 🔄 (current)\n- Replace `EventSource` with `WebSocket` in `boardstream.js`\n- Add heartbeat ping/pong (30s interval)\n- Reconnect with exponential backoff\n- Detect stale connections via pong timeout\n\n### Phase 3: Bi-directional events\n- Client→server: typing indicators, cursor position\n- Presence: show who's viewing the board, who's editing which card\n- Collaborative editing lock (advisory, not hard lock)\n\n### Phase 4: SSE deprecation\n- Add `Deprecation` header to SSE endpoint\n- Log warning when SSE is used\n- Remove SSE code after 2 release cycles\n\n## Current status\nPhase 2 in progress. WS endpoint handles 200+ concurrent connections with lower memory than SSE.",
			Comments: []seedComment{
				{Body: "WS endpoint is live at `/ws/boards/{id}`. Same event format as SSE — `card_created`, `card_updated`, `card_moved`, `card_deleted`.", Author: marcusID},
				{Body: "Tested with 200 concurrent WS connections — memory usage is actually 30% lower than SSE. The goroutine-per-connection model is more efficient than SSE's chunked transfer encoding.", Author: marcusID},
				{Body: "One gotcha: Envoy needs `upgrade: websocket` in the route config. I added it to the Gateway API HTTPRoute. Without it you get 400 Bad Request.", Author: "owner"},
				{Body: "The typing indicator is going to be fun. We need to debounce it — don't send 'user is typing' on every keystroke, batch to every 2 seconds.", Author: sarahID},
			},
			Related: []int{8, 19}}, // related to SSE heartbeat and SSE memory leak

		// 18
		{Column: "in-progress", Title: "Fix goroutine leak in SSE relay hub", Tag: "red", Priority: "highest", Points: 3, DueDate: "2026-04-03",
			Assignee: ownerID, Reporter: priyaID,
			Desc: "The SSE hub's goroutine map grows unboundedly when clients disconnect without sending a close event. After a few hours under moderate load (~50 users), the goroutine count climbs from ~50 to 12,000+.\n\n## Root cause\nThe hub's main loop processes three channels in a `select`: register, unregister, broadcast. The unregister channel is buffered at 256. Under load:\n1. Broadcast sends a large payload (full board state) to all clients\n2. Slow clients block the write, holding the loop in broadcast processing\n3. Meanwhile, disconnected clients try to unregister\n4. The unregister channel fills up (256 buffer)\n5. New unregistrations are dropped silently (non-blocking send)\n6. Goroutines for disconnected clients are never cleaned up\n\n## Fix\n1. Make unregister channel unbuffered — `make(chan *Client)`\n2. Handle unregister in a dedicated goroutine that drains independently\n3. Add `closeOnce sync.Once` to Client to prevent double-close panics\n4. Add goroutine count to `/healthz` endpoint for monitoring\n5. Set write deadline on SSE writes (5s) — if client can't receive in time, disconnect them\n\n## Validation\n- pprof before fix: 12,453 goroutines after 4 hours\n- pprof after fix: 47 goroutines after 24 hours (soak test)\n- Memory: RSS dropped from 2.1GB to 180MB",
			Comments: []seedComment{
				{Body: "Profiled with pprof — goroutine count climbs to 12k after a few hours. Every single leaked goroutine is stuck in `sse.(*Hub).register` waiting to send on the full unregister channel.", Author: priyaID},
				{Body: "Root cause confirmed: the hub's for-select loop processes register/unregister/broadcast sequentially. If broadcast is slow (large payload to many clients), unregistrations queue up and overflow the 256-deep buffer.", Author: "owner"},
				{Body: "Fix is in — switched to unbuffered channel with a dedicated drain goroutine. Also added 5s write deadline so slow clients get disconnected proactively. Goroutine count stable at 47 after 24h soak test.", Author: "owner"},
				{Body: "Can we add a `/debug/goroutines` endpoint that returns the count? Would help catch this in monitoring before it becomes a problem.", Author: marcusID},
				{Body: "Done. Added goroutine count to `/healthz` output: `{\"status\":\"ok\",\"goroutines\":47,\"db_pool\":{\"in_use\":3,\"idle\":7}}`. Also added a PagerDuty alert that fires when goroutine count exceeds 500.", Author: "owner"},
				{Body: "Deployed to prod 6 hours ago. Grafana shows goroutine count flat at 52. Memory stable at 190MB. This one's fixed.", Author: priyaID},
			},
			Related: []int{8, 13}}, // related to SSE heartbeat and DB pool exhaustion

		// ════════════════════════════════════════════════════════════
		// DONE — 7 cards (indices 19-25)
		// ════════════════════════════════════════════════════════════

		// 19
		{Column: "done", Title: "Deploy polygon-streamer v2 with WebSocket support", Tag: "orange", Priority: "medium", Points: 3, DueDate: "2026-03-29",
			Assignee: priyaID, Reporter: ownerID,
			Desc: "Deployed the v2 rewrite of polygon-streamer. This service consumes real-time stock quotes from Polygon.io and fans them out to connected clients.\n\n## Key changes from v1\n- **Protocol**: Switched from REST polling (5s interval) to Polygon's native WebSocket API\n- **Reconnection**: Automatic with exponential backoff (1s, 2s, 4s, 8s, max 60s)\n- **Latency**: Reduced from 5s (polling interval) to 50ms (WebSocket propagation)\n- **NBBO**: Added National Best Bid/Offer spread calculation inline\n- **Filtering**: Server-side subscription management — clients subscribe to specific symbols\n- **Compression**: Per-message deflate enabled, reducing bandwidth by 70%\n\n## Deployment notes\n- Image: `registry.lwts.dev/polygon-streamer:v2.0.0`\n- Replicas: 2 (one per region)\n- Resource limits: 256Mi memory, 100m CPU\n- Health check: `/healthz` returns connected symbol count\n\n## Rollback plan\nv1 image is tagged `v1.4.2`. Environment variable `POLYGON_USE_REST=true` falls back to polling mode without a redeploy.",
			Comments: []seedComment{
				{Body: "v2 has been running in staging for 2 weeks with zero disconnections. Latency p99 is 47ms. Shipping to prod.", Author: priyaID},
				{Body: "Deployed. Both east and west replicas connected. Quote latency looks great in Grafana.", Author: "owner"},
			}},

		// 20
		{Column: "done", Title: "Fix chat SSE relay memory leak in HeartbeatStream", Tag: "red", Priority: "high", Points: 5, DueDate: "2026-03-27",
			Assignee: ownerID, Reporter: marcusID,
			Desc: "The chat service's SSE relay (`HeartbeatStream`) was leaking goroutines and memory. After a few hours of moderate usage, the chat-server pod's RSS grew from 200MB to 2.1GB, eventually triggering OOMKill.\n\n## Root cause\n`HeartbeatStream` spawns a goroutine per connected client that sends a heartbeat every 30 seconds. When clients disconnect (close browser tab, network drop), the goroutine was NOT cleaned up because:\n1. The `http.ResponseWriter` doesn't return an error on write after client disconnect (it buffers)\n2. The goroutine only checked for context cancellation, but the SSE handler wasn't wiring the request context\n3. Each leaked goroutine held a reference to the response writer, preventing GC\n\n## Fix\n1. Wire `r.Context()` into the heartbeat goroutine's select loop\n2. Add `http.Flusher` interface check — flush after each write to detect broken pipes immediately\n3. Defer `close(done)` channel to signal heartbeat goroutine on handler return\n4. Add read deadline on the underlying connection via `http.Hijacker` (fallback for non-context-aware clients)\n\n## Result\n- Before: RSS grows to 2.1GB over 4 hours, 8,000+ leaked goroutines\n- After: RSS stable at 340MB, goroutine count stable at ~30\n- Zero OOMKills in the 2 weeks since deployment",
			Comments: []seedComment{
				{Body: "Deployed fix. Memory usage back to normal — RSS dropped from 2.1GB to 340MB after the pod restarted. Goroutine count is flat.", Author: "owner"},
				{Body: "I added a unit test that simulates 100 client connect/disconnect cycles and asserts goroutine count returns to baseline. Should prevent regression.", Author: marcusID},
			},
			Related: []int{18}}, // related to SSE relay goroutine leak

		// 21
		{Column: "done", Title: "Implement JWT refresh token rotation with breach detection", Tag: "orange", Priority: "high", Points: 5, DueDate: "2026-03-25",
			Assignee: priyaID, Reporter: ownerID,
			Desc: "Added refresh token rotation to prevent token replay attacks. This is a security hardening measure recommended by the OWASP JWT guidelines.\n\n## How it works\n1. Client sends refresh token to `POST /api/auth/refresh`\n2. Server validates the token, checks it hasn't been used\n3. Server marks the old token as `used_at = NOW()`\n4. Server issues a new access token + refresh token pair\n5. Client stores the new pair, discards the old\n\n## Breach detection\nIf a refresh token that has already been used (`used_at IS NOT NULL`) is presented again, it means either:\n- The token was stolen and the attacker is using it, OR\n- The legitimate user is using a cached copy\n\nIn either case, we revoke ALL tokens for that user (delete from `refresh_tokens` where `user_id = ?`). The user must log in again.\n\n## Schema\n```sql\nALTER TABLE refresh_tokens ADD COLUMN used_at TIMESTAMPTZ;\n```\n\n## Grace period\nTo handle race conditions (concurrent requests from the same client using the same refresh token), we allow a 30-second grace period. If a used token is presented within 30s of `used_at`, we issue new tokens without revoking.\n\n## Token lifetime\n- Access token: 15 minutes\n- Refresh token: 7 days\n- Absolute session limit: 30 days (user must re-authenticate)",
			Comments: []seedComment{
				{Body: "This broke the iOS app initially — it was caching the old refresh token and sending it on the next app launch. Fixed by updating the Keychain storage to always save the latest pair.", Author: sarahID},
				{Body: "Added a 30-second grace period for the old token to handle race conditions. Without this, rapid concurrent API calls (like on page load) would trigger false breach detection.", Author: priyaID},
				{Body: "Good catch on the grace period. I saw this in our error logs — the mobile app fires 4 API calls simultaneously on launch, and they all try to refresh at once.", Author: "owner"},
			},
			Related: []int{7}}, // related to rate limiting

		// 22
		{Column: "done", Title: "Add global search with keyboard shortcut", Tag: "blue", Priority: "medium", Points: 5, DueDate: "2026-03-24",
			Assignee: sarahID, Reporter: ownerID,
			Desc: "Global search in the header bar that searches card titles and descriptions across all columns on the current board.\n\n## Features\n- Instant search-as-you-type (debounced at 200ms)\n- Results dropdown below the search bar showing:\n  - Card key (monospace, dimmed)\n  - Card title (primary text)\n  - Column badge (colored pill)\n- Click result → opens card detail modal\n- Keyboard navigation: ↑/↓ to move, Enter to select, Esc to close\n- Keyboard shortcut: `/` focuses the search bar from anywhere\n\n## Implementation\n- **<100 cards**: Client-side filtering (instant, no API call)\n- **≥100 cards**: Server-side `LIKE` query via `GET /api/v1/search?q=...&board_id=...`\n- Highlight matching text in results (bold the matched substring)\n- No results state: \"No cards found\" with a muted icon\n\n## Performance\n- Client-side: <1ms for 100 cards (simple string includes)\n- Server-side: <50ms with `LIKE` on indexed `title` column\n- Debounce prevents excessive API calls during typing",
			Comments: []seedComment{
				{Body: "The `/` shortcut conflicts with browser's quick-find in Firefox. We should check if an input is focused before handling it.", Author: marcusID},
				{Body: "Good catch. I added a check: `if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return;`", Author: sarahID},
				{Body: "Can we also search by assignee name? Like typing 'sarah' shows all cards assigned to Sarah?", Author: priyaID},
				{Body: "Not in v1 — let's keep it simple. Title and description search only. We can add filters later.", Author: "owner"},
			}},

		// 23
		{Column: "done", Title: "Set up CI/CD pipeline with GitHub Actions", Tag: "orange", Priority: "medium", Points: 5, DueDate: "2026-03-20",
			Assignee: marcusID, Reporter: ownerID,
			Desc: "Set up a complete CI/CD pipeline using GitHub Actions. Every PR should be tested, every merge to main should deploy to staging, and releases should deploy to production.\n\n## Pipeline stages\n\n### CI (runs on every PR)\n1. **Go tests**: `go test ./...` with race detector\n2. **Go lint**: `golangci-lint run` with custom config\n3. **Integration tests**: Spin up PostgreSQL via `services`, run migration + seed + test\n4. **Build check**: `go build ./...` to catch import errors\n5. **Frontend lint**: ESLint on all `.js` files\n\n### CD (runs on merge to main)\n1. Build Docker image (multi-stage, Alpine base)\n2. Push to `registry.lwts.dev/lwts:sha-{commit}`\n3. Deploy to staging via Helm upgrade\n4. Run smoke tests against staging\n\n### Release (runs on version tag)\n1. Build + push with version tag\n2. Deploy to production (both regions)\n3. Create GitHub release with changelog\n\n## Docker image\n- Multi-stage build: Go builder → Alpine runtime\n- Final image size: 28MB\n- Static binary with `CGO_ENABLED=0`\n- Non-root user (`nobody:nobody`)\n- Health check: `HEALTHCHECK CMD wget -q http://localhost:8080/healthz`\n\n## Secrets\nStored in GitHub Actions secrets: `REGISTRY_TOKEN`, `KUBECONFIG_STAGING`, `KUBECONFIG_PROD`",
			Comments: []seedComment{
				{Body: "Pipeline runs in ~3 minutes. Most time is spent on integration tests (PostgreSQL startup + migration takes 15s).", Author: marcusID},
				{Body: "Added a matrix strategy for Go 1.22 and 1.23. Caught a `slices.Collect` usage that doesn't exist in 1.22.", Author: marcusID},
				{Body: "The Docker build uses multi-stage to keep the final image at 28MB. Alpine base with static Go binary. I also added `--no-cache` to the build step since GitHub Actions runners have stale layer caches.", Author: "owner"},
			}},

		// 24
		{Column: "done", Title: "Export workspace data to JSON", Tag: "blue", Priority: "low", Points: 2, DueDate: "2026-03-22",
			Assignee: priyaID, Reporter: sarahID,
			Desc: "Add a data export feature that dumps the entire workspace (boards, cards, comments) to a JSON file. Accessible from the admin settings panel and via the CLI.\n\n## Endpoints\n- `GET /api/v1/export` — returns JSON blob with all data\n- CLI: `lwts backup output.json` — writes to file\n\n## Export format\n```json\n{\n  \"exported_at\": \"2026-03-22T12:00:00Z\",\n  \"version\": \"1.0\",\n  \"users\": [...],\n  \"boards\": [...],\n  \"cards\": [...],\n  \"comments\": [...]\n}\n```\n\n## Security\n- Export requires admin role\n- User passwords are NOT included in the export\n- Export includes user IDs, names, emails, and roles only\n\n## Use cases\n- Backup before major upgrades\n- Migration between instances\n- Data analysis in external tools\n- Compliance / data portability (GDPR right to data portability)",
			Comments: []seedComment{
				{Body: "Should we include user passwords in the export? Useful for migration between instances.", Author: priyaID},
				{Body: "Absolutely not. Password hashes should never leave the database. Users can reset their passwords on the new instance.", Author: "owner"},
				{Body: "The CLI version is nice — `lwts backup ./backup-2026-03-22.json`. I also added `lwts restore` for the import side.", Author: marcusID},
			}},

		// 25
		{Column: "done", Title: "Fix column drag reorder not persisting to database", Tag: "red", Priority: "high", Points: 3, DueDate: "2026-03-26",
			Assignee: sarahID, Reporter: marcusID,
			Desc: "Dragging columns to reorder them worked visually (the DOM updated correctly) but the new order didn't persist — on page reload, columns snapped back to their original order.\n\n## Root cause\nThe column order is stored in the board's `columns` JSON field as an array of `{id, label}` objects. The drag-end handler updated the local state and re-rendered the DOM, but never called the API to persist the change.\n\n## Fix\nOne-liner: added `API.updateBoard(boardId, { columns: JSON.stringify(newOrder) })` to the `onColumnDragEnd` handler.\n\nAlso added:\n- Optimistic update: DOM updates immediately, reverts on API error\n- Toast notification on failure: \"Failed to save column order\"\n- Version check: if board was modified by another user, show conflict toast and reload\n\n## Testing\n- Drag column A to position C → reload → verify order persists\n- Two users: user 1 reorders, user 2 sees change via SSE\n- Network error during save → verify rollback to original order",
			Comments: []seedComment{
				{Body: "One-liner fix: added `API.updateBoard(boardId, { columns: newOrder })` to the drag-end handler. Can't believe we missed this.", Author: sarahID},
				{Body: "Also added optimistic update + rollback on failure, consistent with how card drag-and-drop works. And a toast on error so the user knows something went wrong.", Author: sarahID},
				{Body: "I added an integration test for this: drag column, reload page, assert order. Should prevent regression.", Author: marcusID},
			}},

		// ════════════════════════════════════════════════════════════
		// EPICS (indices 26-28)
		// ════════════════════════════════════════════════════════════

		// 26
		{Column: "in-progress", Title: "UX & Frontend Polish", Tag: "epic", Priority: "high", Points: 34,
			Assignee: sarahID, Reporter: ownerID,
			Desc: "Umbrella epic for all user-facing frontend improvements. Covers theming, responsive design, keyboard navigation, rich text editing, and UI component upgrades.\n\n## Goals\n- Professional, polished UI across all screen sizes\n- Keyboard-first navigation for power users\n- Consistent design language with dark/light theme support\n- Zero visual regressions (enforced by Playwright screenshot tests)\n\n## Key deliverables\n- Dark/light theme toggle with system preference detection\n- Mobile-responsive board and list views\n- Keyboard shortcuts with discoverable help overlay\n- Live markdown preview in card descriptions\n- Redesigned card detail modal with two-column layout\n- Firefox drag-and-drop fix",
			Comments: []seedComment{
				{Body: "Grouping all the frontend work under this epic so we can track velocity. Current estimate: 34 story points across 7 cards.", Author: sarahID},
				{Body: "Let's aim to close this out by end of April. The theme toggle and responsive layout are the highest priority items.", Author: "owner"},
			}},

		// 27
		{Column: "in-progress", Title: "Infrastructure & Reliability", Tag: "epic", Priority: "highest", Points: 43,
			Assignee: ownerID, Reporter: priyaID,
			Desc: "Umbrella epic for backend stability, performance, and operational improvements. Covers database tuning, real-time infrastructure, security hardening, and CI/CD.\n\n## Goals\n- Zero downtime under sustained load (50+ concurrent users)\n- Sub-second response times on all API endpoints (p99)\n- Automated testing and deployment pipeline\n- Security hardening: rate limiting, audit logging, token rotation\n\n## Key deliverables\n- SSE heartbeat fix (Envoy timeout)\n- Connection pool optimization + pgbouncer\n- Goroutine leak fix in SSE relay\n- Rate limiting on auth endpoints\n- JWT refresh token rotation with breach detection\n- End-to-end test suite with Playwright\n- CI/CD pipeline with GitHub Actions\n- Elasticsearch shard rebalancing investigation",
			Comments: []seedComment{
				{Body: "This is our highest-priority epic. The SSE issues and connection pool exhaustion are causing real user pain.", Author: "owner"},
				{Body: "I've been tracking the SSE-related incidents. Three outages in the last two weeks, all traced to either heartbeat timeouts or goroutine leaks.", Author: priyaID},
			}},

		// 28
		{Column: "todo", Title: "Data Pipeline & Analytics", Tag: "epic", Priority: "medium", Points: 16,
			Assignee: priyaID, Reporter: ownerID,
			Desc: "Umbrella epic for the earnings data pipeline, economic indicators, and storage infrastructure. Covers transcript processing, FRED integration, and avatar storage migration.\n\n## Goals\n- Quarterly transcript bulk import in under 5 minutes (currently 6 hours)\n- Real-time economic indicators dashboard with FRED data\n- Self-hosted asset storage (avatars, attachments) via MinIO\n\n## Key deliverables\n- Transcript parser batch mode (100x throughput improvement)\n- FRED macro series integration (7 key indicators)\n- Avatar upload and storage via S3-compatible MinIO\n- Reconcile job for Elasticsearch ↔ PostgreSQL consistency",
			Comments: []seedComment{
				{Body: "The batch parser is the critical path item here. The current 6-hour import window blocks the entire data team during earnings season.", Author: priyaID},
				{Body: "FRED integration is straightforward — the API client already exists. Just need to add the new series and wire up the dashboard.", Author: "owner"},
			}},
	}

	// ── Create all cards and collect IDs for linking ──
	cardIDs := make([]string, len(seedCards))

	for i, sc := range seedCards {
		points := sc.Points
		var dueDate *string
		if sc.DueDate != "" {
			dueDate = &sc.DueDate
		}
		assignee := sc.Assignee
		if assignee == "" {
			assignee = ownerID
		}
		reporter := sc.Reporter
		if reporter == "" {
			reporter = ownerID
		}

		card, err := cards.Create(ctx, board.ID, CardCreate{
			ColumnID:    sc.Column,
			Title:       sc.Title,
			Description: sc.Desc,
			Tag:         sc.Tag,
			Priority:    sc.Priority,
			AssigneeID:  &assignee,
			ReporterID:  &reporter,
			Points:      &points,
			DueDate:     dueDate,
		})
		if err != nil {
			return fmt.Errorf("create card %q: %w", sc.Title, err)
		}
		cardIDs[i] = card.ID

		for _, c := range sc.Comments {
			authorID := ownerID
			if c.Author != "" && c.Author != "owner" {
				authorID = c.Author
			}
			if _, err := comments.Create(ctx, card.ID, authorID, c.Body); err != nil {
				return fmt.Errorf("create comment on %q: %w", card.Key, err)
			}
		}
	}

	// ── Link related tickets ──
	for i, sc := range seedCards {
		if len(sc.Related) == 0 {
			continue
		}
		relatedIDs := make([]string, 0, len(sc.Related))
		for _, idx := range sc.Related {
			if idx >= 0 && idx < len(cardIDs) {
				relatedIDs = append(relatedIDs, cardIDs[idx])
			}
		}
		if len(relatedIDs) == 0 {
			continue
		}
		jsonBytes, _ := json.Marshal(relatedIDs)
		jsonStr := string(jsonBytes)
		if _, err := cards.Update(ctx, cardIDs[i], 1, CardUpdate{
			RelatedCardIDs: &jsonStr,
		}); err != nil {
			return fmt.Errorf("link related cards for card %d: %w", i, err)
		}
	}

	// ── Assign cards to epics ──
	// Epic 26: UX & Frontend Polish
	// Epic 27: Infrastructure & Reliability
	// Epic 28: Data Pipeline & Analytics
	epicAssignments := map[int]int{
		// UX & Frontend (epic 26)
		0: 26, 2: 26, 6: 26, 11: 26, 12: 26, 15: 26,
		// Infrastructure & Reliability (epic 27)
		1: 27, 3: 27, 7: 27, 8: 27, 13: 27, 18: 27, 19: 27, 22: 27, 24: 27, 25: 27,
		// Data Pipeline & Analytics (epic 28)
		9: 28, 10: 28, 14: 28,
	}
	for cardIdx, epicIdx := range epicAssignments {
		if _, err := ds.Exec(ctx,
			"UPDATE cards SET epic_id = $1 WHERE id = $2",
			cardIDs[epicIdx], cardIDs[cardIdx]); err != nil {
			return fmt.Errorf("assign epic for card %d: %w", cardIdx, err)
		}
	}

	return nil
}

// Seed is the legacy entry point used by the `seed` CLI subcommand.
func Seed(ctx context.Context, ds db.Datasource) error {
	users := NewUserRepository(ds)
	existing, err := users.List(ctx)
	if err != nil {
		return fmt.Errorf("check existing users: %w", err)
	}
	if len(existing) == 0 {
		// Keep legacy behavior for fresh DBs where no user exists yet.
		// Callers can create a user first, then run seed again.
		return nil
	}

	boards := NewBoardRepository(ds)
	existingBoards, err := boards.List(ctx)
	if err != nil {
		return fmt.Errorf("check existing boards: %w", err)
	}
	if len(existingBoards) > 0 {
		// Idempotent by default: if workspace data already exists, skip.
		return nil
	}

	ownerID := existing[0].ID
	for _, u := range existing {
		if u.Role == "owner" {
			ownerID = u.ID
			break
		}
	}

	if err := SeedDemo(ctx, ds, ownerID); err != nil {
		return fmt.Errorf("seed demo: %w", err)
	}
	return nil
}
