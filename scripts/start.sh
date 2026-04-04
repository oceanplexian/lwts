#!/bin/bash
# Start LWTS Kanban server with sane defaults
# Run from the project root: ./scripts/start.sh [port]

PORT="${1:-8099}"
DB="${LWTS_DB:-sqlite:///tmp/lwts-e2e.db}"

export JWT_SECRET="${JWT_SECRET:-lwts-dev-secret}"
export DB_URL="$DB"
export PORT="$PORT"
export DEV=true
export LOG_LEVEL=info

# Run migrations
go run ./server/cmd migrate 2>&1

# Seed if fresh DB
go run ./server/cmd seed 2>&1

# Ensure default admin exists with known password (admin@admin.dev / admin)
go run ./server/cmd user-create Admin admin@admin.dev admin owner 2>/dev/null
go run ./server/cmd reset-password admin@admin.dev admin 2>&1

echo "Starting LWTS on http://localhost:${PORT}"
echo "Login: admin@admin.dev / admin"
exec go run ./server/cmd
