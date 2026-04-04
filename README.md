# LWTS — Lightweight Task System

A kanban board with real-time collaboration, built with Go and vanilla JavaScript.

## Features

- Kanban boards with drag-and-drop
- Real-time updates via Server-Sent Events
- SQLite or PostgreSQL backend
- Discord notifications and webhooks
- Multi-user with role-based access
- Full-text search

## Quick Start

```bash
# Run with SQLite (default)
make run

# Run with PostgreSQL
DB_URL="postgres://user:pass@localhost:5432/lwts?sslmode=disable" make run
```

Default login: `admin@admin.dev` / `admin`

## Docker

```bash
docker pull oceanplexian/lwts:latest
docker run -p 8080:8080 -v lwts-data:/data oceanplexian/lwts:latest
```

Multi-arch images available for `linux/amd64` and `linux/arm64`.

## Development

```bash
make dev          # hot-reload with air
make test         # run unit tests
make test-pg      # run integration tests (requires postgres)
make lint         # golangci-lint
```

## License

MIT
