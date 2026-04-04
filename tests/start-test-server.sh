#!/bin/bash
# Start LWTS with a disposable SQLite test database on port 8044.
# Used by Claude hooks before running UI tests.
#
# Usage:
#   ./tests/start-test-server.sh        # start (blocks until healthy)
#   ./tests/start-test-server.sh stop    # stop running instance

set -euo pipefail

PORT=8044
DB_PATH="/tmp/lwts-test-ui.db"
PID_FILE="/tmp/lwts-test-ui.pid"
LOG_FILE="/tmp/lwts-test-ui.log"
BINARY="/tmp/lwts-test-ui-bin"
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

export DB_URL="sqlite://${DB_PATH}"
export JWT_SECRET="test-secret-for-ui-tests"
export PORT="$PORT"
export DEV=true
export LOG_LEVEL=warn

stop_server() {
    # Kill by PID file
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if kill -0 "$PID" 2>/dev/null; then
            kill "$PID" 2>/dev/null || true
            sleep 1
            kill -9 "$PID" 2>/dev/null || true
        fi
        rm -f "$PID_FILE"
    fi
    # Also kill anything still holding the port (catches orphaned go run children)
    lsof -ti :"$PORT" 2>/dev/null | xargs kill -9 2>/dev/null || true
}

if [ "${1:-}" = "stop" ]; then
    stop_server
    echo "test server stopped"
    exit 0
fi

# If already running and healthy, exit early
if [ -f "$PID_FILE" ] && kill -0 "$(cat "$PID_FILE")" 2>/dev/null; then
    if curl -sf "http://localhost:${PORT}/healthz" >/dev/null 2>&1; then
        echo "test server already running on port ${PORT}"
        exit 0
    fi
    stop_server
fi

# Wipe old test DB for a clean slate
rm -f "$DB_PATH" "${DB_PATH}-wal" "${DB_PATH}-shm"

cd "$PROJECT_DIR"

# Build binary once — avoids go run's child PID tracking issues
go build -o "$BINARY" ./server/cmd 2>&1

# Migrate + seed
"$BINARY" migrate 2>&1
"$BINARY" seed-test 2>&1

# Start server in background (direct binary, not go run)
"$BINARY" > "$LOG_FILE" 2>&1 &
echo "$!" > "$PID_FILE"

# Wait for healthy (up to 15s)
for i in $(seq 1 30); do
    if curl -sf "http://localhost:${PORT}/healthz" >/dev/null 2>&1; then
        echo "test server ready on http://localhost:${PORT}"
        exit 0
    fi
    sleep 0.5
done

echo "ERROR: test server failed to start within 15s" >&2
echo "Logs:" >&2
tail -20 "$LOG_FILE" >&2
stop_server
exit 1
