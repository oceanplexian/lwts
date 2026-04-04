#!/bin/sh
set -e

# Ensure data directory exists and is writable by the lwts user
mkdir -p /data
chown 65532:65532 /data

# Drop to non-root and run
exec su-exec lwts /lwts "$@"
