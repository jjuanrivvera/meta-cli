#!/usr/bin/env bash
set -euo pipefail
score=0
rg -q 'run .* auth login|check permissions|verify .* list|rate limited|server error' internal/api/errors.go && score=$((score + 1))
test "$(rg -l 'Example:' commands --glob '*.go' | awk 'END{print NR}')" -ge 3 && score=$((score + 1))
! rg -n '// (Create|Get|Set|Return|Run|Add) ' cmd commands internal --glob '*.go' >/dev/null && score=$((score + 1))
make build >/dev/null
bin/metactl --help | rg -q 'Instagram|Facebook Pages|WhatsApp' && score=$((score + 1))
bin/metactl mcp tools | jq -e '[.tools[] | select(.annotations.readOnlyHint == true)] | length > 0' >/dev/null && score=$((score + 1))
printf 'review rubric: %d/5\n' "$score"
test "$score" -eq 5
