#!/bin/bash
# Build metadata — sourced by Makefile and CI workflows
BINARY="lwts"
VERSION=$(git describe --tags --always 2>/dev/null || echo "dev")
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
DESC="Lightweight Task System — kanban board"
LICENSE="MIT"
SOURCE_URL="https://github.com/oceanplexian/lwts"
VENDOR="oceanplexian"
