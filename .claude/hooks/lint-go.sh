#!/bin/bash
# PostToolUse hook: run golangci-lint on changed Go files
INPUT=$(cat)
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // empty')

# Only lint .go files
if [[ "$FILE_PATH" != *.go ]]; then
  exit 0
fi

# Get the package directory
PKG_DIR=$(dirname "$FILE_PATH")

# Run golangci-lint on the package
OUTPUT=$(golangci-lint run "$PKG_DIR/..." 2>&1)
EXIT_CODE=$?

if [ $EXIT_CODE -ne 0 ]; then
  echo "golangci-lint found issues:" >&2
  echo "$OUTPUT" >&2
  exit 2
fi

exit 0
