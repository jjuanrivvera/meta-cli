#!/usr/bin/env bash
set -euo pipefail
threshold="${1:-80}"
test -f coverage.out || go test -count=1 -coverpkg=./... -coverprofile=coverage.out ./... >/dev/null
percent="$(go tool cover -func=coverage.out | awk '/^total:/ {gsub(/%/, "", $3); print $3}')"
awk -v p="$percent" -v t="$threshold" 'BEGIN { if (p+0 < t+0) { printf "coverage %.1f%% < %d%%\n", p, t; exit 1 }; printf "coverage %.1f%% >= %d%%\n", p, t }'

