<p align="center">
  <img src="docs/img/lwts-icon.png" alt="LWTS Logo" width="64" />
</p>

<h1 align="center">LWTS</h1>

<p align="center">
  <strong>Lightweight Task Server</strong><br>
  A real-time kanban board that ships as a single binary.<br>
  Built with Go and vanilla JavaScript. No Node.js, no webpack, no runtime dependencies.
</p>

<p align="center">
  <a href="https://github.com/oceanplexian/lwts/actions/workflows/test-and-lint.yml"><img src="https://img.shields.io/github/actions/workflow/status/oceanplexian/lwts/test-and-lint.yml?style=flat-square&label=tests" alt="Tests"></a>
  <a href="https://github.com/oceanplexian/lwts/actions/workflows/docker.yml"><img src="https://img.shields.io/github/actions/workflow/status/oceanplexian/lwts/docker.yml?branch=main&style=flat-square&label=docker" alt="Docker Build"></a>
  <img src="https://img.shields.io/badge/go-1.25-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go 1.25">
  <a href="LICENSE"><img src="https://img.shields.io/github/license/oceanplexian/lwts?style=flat-square" alt="License"></a>
</p>

<p align="center">
  <a href="https://ypy2jdydz5po7fyweuvykikjum0xoney.lambda-url.us-west-2.on.aws/">Live Demo</a> · <a href="#quick-start">Quick Start</a> · <a href="#api">API Docs</a> · <a href="https://github.com/oceanplexian/lwts/releases">Releases</a>
</p>

---

<p align="center">
  <img src="docs/img/sc-2.png" alt="Kanban Board" width="32%" />
  <img src="docs/img/sc-1.png" alt="Card Detail" width="32%" />
  <img src="docs/img/sc-3.png" alt="Settings" width="32%" />
</p>

---

## Features

- **Kanban boards** — drag-and-drop cards between columns, reorder with real-time sync
- **Real-time collaboration** — Server-Sent Events push updates instantly to all connected clients
- **Full-text search** — find any card across all boards in milliseconds
- **Optional semantic search** — opt-in pgvector + remote embeddings for natural-language queries (see [Semantic Search](#semantic-search))
- **Card detail view** — descriptions, subtasks, attachments, comments, activity history
- **Epic swimlanes** — group cards by epic to see progress across workstreams
- **Multiple views** — board view, list view, and filtered views by assignee, priority, or tag
- **Dark & light themes** — system-aware with manual toggle and customizable accent colors
- **Appearance settings** — density modes, font sizes, card animation preferences, avatar visibility
- **Team management** — invite members, assign roles (owner/member), per-board access
- **Webhooks** — HTTP callbacks on card events with retry, delivery logs, and signature verification
- **Discord notifications** — native Discord integration for card updates
- **JWT authentication** — secure auth with token refresh, registration, and password reset
- **API keys** — generate keys for CI/CD and automation workflows
- **Import / Export** — full data portability
- **Dual database support** — SQLite for single-node, PostgreSQL for production
- **Single binary** — embed the frontend at build time, deploy one file
- **Multi-arch Docker** — `linux/amd64` and `linux/arm64` images
- **AWS Lambda support** — deploy as a serverless function with Lambda Web Adapter

## Quick Start

### Binary

```bash
# Clone and build
git clone https://github.com/oceanplexian/lwts.git
cd lwts
make build

# Run with SQLite (default)
DB_URL="sqlite:///tmp/lwts.db" JWT_SECRET=changeme ./bin/lwts

# Run with PostgreSQL
DB_URL="postgres://user:pass@localhost:5432/lwts?sslmode=disable" JWT_SECRET=changeme ./bin/lwts
```

Open `http://localhost:8080` — default login: `admin@admin.dev` / `admin`

### Docker

```bash
docker pull oceanplexian/lwts:latest

docker run -p 8080:8080 \
  -e DB_URL="sqlite:///data/lwts.db" \
  -e JWT_SECRET=changeme \
  -v lwts-data:/data \
  oceanplexian/lwts:latest
```

### Docker Compose

```yaml
services:
  lwts:
    image: oceanplexian/lwts:latest
    ports:
      - "8080:8080"
    environment:
      DB_URL: sqlite:///data/lwts.db
      JWT_SECRET: changeme
    volumes:
      - lwts-data:/data

volumes:
  lwts-data:
```

## Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `DB_URL` | `postgres://...` | Database connection (`sqlite:///path` or `postgres://...`) |
| `JWT_SECRET` | *(required)* | Secret for signing JWT tokens |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | `text` | Log format: `text` or `json` |
| `CORS_ORIGINS` | `http://localhost:5173` | Comma-separated allowed origins |
| `MAX_UPLOAD_SIZE` | `10485760` | Max upload size in bytes (default 10MB) |
| `SESSION_TTL` | `24h` | JWT token lifetime |
| `DB_MAX_CONNS` | `20` | Max database connections (PostgreSQL) |
| `TLS_CERT` | | Path to TLS certificate |
| `TLS_KEY` | | Path to TLS private key |
| `DEV` | `false` | Enable development mode (disables CORS/body limits, serves from `web/`) |
| `EMBEDDING_API_URL` | | Optional. OpenAI-compatible `/v1/embeddings` endpoint for semantic search |
| `EMBEDDING_API_KEY` | | Optional. Bearer token for the embedding endpoint (e.g. OpenAI key) |
| `EMBEDDING_MODEL` | `BAAI/bge-small-en-v1.5` | Model identifier sent to the embedding endpoint |
| `EMBEDDING_DIM` | `384` | Vector dimension for the chosen model |

## API

RESTful JSON API at `/api/v1/`. Authenticate with `Authorization: Bearer <token>` or an API key.

### Auth

```
POST   /api/v1/auth/login          # Login, returns JWT
POST   /api/v1/auth/register       # Register new user
POST   /api/v1/auth/refresh         # Refresh JWT
```

### Boards

```
GET    /api/v1/boards               # List boards
POST   /api/v1/boards               # Create board
GET    /api/v1/boards/:id           # Get board
PATCH  /api/v1/boards/:id           # Update board
DELETE /api/v1/boards/:id           # Delete board
GET    /api/v1/boards/:id/stream    # SSE event stream
GET    /api/v1/boards/:id/presence  # Active users
```

### Cards

```
GET    /api/v1/boards/:id/cards     # List cards
POST   /api/v1/boards/:id/cards     # Create card
GET    /api/v1/boards/:id/cards/:id # Get card
PATCH  /api/v1/boards/:id/cards/:id # Update card (move, edit, reorder)
DELETE /api/v1/boards/:id/cards/:id # Delete card
```

### Comments

```
GET    /api/v1/boards/:id/cards/:id/comments     # List comments
POST   /api/v1/boards/:id/cards/:id/comments     # Add comment
PATCH  /api/v1/boards/:id/cards/:id/comments/:id # Edit comment
DELETE /api/v1/boards/:id/cards/:id/comments/:id # Delete comment
```

### Webhooks

```
GET    /api/v1/boards/:id/webhooks               # List webhooks
POST   /api/v1/boards/:id/webhooks               # Create webhook
GET    /api/v1/boards/:id/webhooks/:id            # Get webhook
PATCH  /api/v1/boards/:id/webhooks/:id            # Update webhook
DELETE /api/v1/boards/:id/webhooks/:id            # Delete webhook
GET    /api/v1/boards/:id/webhooks/:id/deliveries # Delivery logs
```

### Search, Settings & Keys

```
GET    /api/v1/search?q=term        # Full-text search
GET    /api/v1/settings/general     # Get settings
PUT    /api/v1/settings/general     # Update settings
GET    /api/v1/keys                 # List API keys
POST   /api/v1/keys                 # Create API key
DELETE /api/v1/keys/:id             # Revoke API key
GET    /api/v1/export               # Export all data
POST   /api/v1/settings/reset       # Reset workspace
```

### Example: Create a card

```bash
curl -X POST https://localhost:8080/api/v1/boards/BOARD_ID/cards \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Fix login timeout",
    "priority": "high",
    "tag": "bug",
    "column": "todo",
    "assignee_id": "usr_abc123"
  }'
```

### Example: Listen to real-time events

```bash
curl -N https://localhost:8080/api/v1/boards/BOARD_ID/stream?token=$TOKEN
```

Events: `card_created`, `card_moved`, `card_updated`, `card_deleted`, `comment_added`

## Development

```bash
make dev          # hot-reload with air
make test         # run unit tests
make test-pg      # integration tests (requires PostgreSQL)
make lint         # golangci-lint
make build        # build binary to bin/lwts
```

### Project Structure

```
server/
├── cmd/main.go              # entrypoint, route wiring
├── internal/
│   ├── api/                 # shared API types
│   ├── auth/                # JWT auth, login, registration
│   ├── board/               # board CRUD handlers
│   ├── card/                # card CRUD handlers
│   ├── comment/             # comment handlers
│   ├── config/              # env-based configuration
│   ├── discord/             # Discord notification integration
│   ├── middleware/           # CORS, logging, body limits
│   ├── settings/            # workspace settings, API keys, export
│   ├── sse/                 # Server-Sent Events hub
│   └── webhook/             # webhook dispatch, retry, delivery logs
web/
├── index.html               # SPA shell
├── src/                     # vanilla JS modules
└── styles/                  # CSS (theme, board, detail, settings, responsive)
docs/                        # landing page
tf/                          # Terraform for Lambda deployment
```

## Semantic Search

LWTS ships with a fast, default substring search engine. For workspaces with
hundreds or thousands of cards, an **optional** semantic search engine can be
enabled. It catches paraphrased queries the substring engine misses
("live realtime updates" → finds the card titled "Real-time board updates via
WebSocket") while still pinning exact-string matches at the top via a guarded
cascade.

The feature is **off by default** and requires three things:

1. **PostgreSQL with the [pgvector](https://github.com/pgvector/pgvector) extension**.
   SQLite deployments can't enable semantic search.

   ```sql
   -- run once on your existing postgres database
   CREATE EXTENSION vector;
   ```

2. **A reachable OpenAI-compatible embeddings endpoint** — set `EMBEDDING_API_URL`.
   Any of these work out of the box:

   - [text-embeddings-inference](https://github.com/huggingface/text-embeddings-inference)
     (HuggingFace, recommended for self-hosted; runs CPU-only)
   - [vLLM](https://docs.vllm.ai/) with an embedding model
   - [Ollama](https://ollama.com/) (`/v1/embeddings` with `nomic-embed-text`)
   - OpenAI (`https://api.openai.com`, set `EMBEDDING_API_KEY`)

   Quickstart with TEI:

   ```bash
   docker run -p 8081:80 -v tei-data:/data \
     ghcr.io/huggingface/text-embeddings-inference:cpu-latest \
     --model-id BAAI/bge-small-en-v1.5

   # then point lwts at it
   export EMBEDDING_API_URL=http://localhost:8081
   ```

3. **A user toggle** in **Settings → General → Search → Semantic search**.
   The toggle is disabled and shows a warning until both the extension and
   endpoint are available; the server validates the prerequisites before
   accepting the change.

After enabling, click **Index all cards** in the same panel (or hit
`POST /api/v1/embed/backfill` as an admin) to embed your existing data.
New and updated cards are embedded automatically.

### Resource expectations

The embedding sidecar is the new cost. With `BAAI/bge-small-en-v1.5` on CPU:

- **~250–400 MiB RAM** for the sidecar (text-embeddings-inference)
- **~10 ms** per query (single-doc encode + pgvector HNSW lookup)
- **~120 docs/sec** sustained throughput on Apple Silicon / modern x86
- **~75 MB** pgvector storage per 10k cards (384-dim, HNSW)

The lwts server itself adds **negligible** overhead — it just makes HTTP calls.

### Status and operations

```bash
# Check whether semantic is available and how many cards are indexed.
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/embed/status

# Re-index everything (admin only). Useful after switching models.
curl -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/embed/backfill
```

If you switch to a different embedding model with a different dimension, set
`EMBEDDING_DIM` to match before restarting; the existing column is left in
place but you'll need to drop it manually (`ALTER TABLE cards DROP COLUMN
embedding`) and let the server recreate it.

## Deployment

### Lambda (Serverless)

LWTS can run on AWS Lambda with the [Lambda Web Adapter](https://github.com/awslabs/aws-lambda-web-adapter). Terraform config is included:

```bash
make lambda-build    # build Lambda container
make lambda-push     # push to ECR
cd tf && terraform apply
```

### Production Checklist

- Set a strong `JWT_SECRET`
- Use PostgreSQL for multi-instance deployments
- Put behind a reverse proxy (nginx, Caddy) for TLS termination
- Set `CORS_ORIGINS` to your domain
- Mount a persistent volume for SQLite data

## License

[MIT](LICENSE)
