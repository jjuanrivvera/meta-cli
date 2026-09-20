#!/usr/bin/env bash
set -euo pipefail
manifest="${1:-api-manifest.json}"
threshold="${2:-90}"
enumeration="$(jq -r '.enumeration' "$manifest")"
declared="$(jq -r '.api_method_total' "$manifest")"
source="$(jq -r '.api_method_source' "$manifest")"
test -n "$source" && test "$source" != null || { echo "manifest source missing" >&2; exit 1; }
actual="$(jq '.operations | length' "$enumeration")"
test "$actual" -eq "$declared" || { echo "enumeration count $actual differs from manifest total $declared" >&2; exit 1; }
invalid="$(jq '[.operations[] | select(.status != "implemented" and .status != "deferred")] | length' "$enumeration")"
test "$invalid" -eq 0 || { echo "operations need implemented or deferred status" >&2; exit 1; }
covered="$(jq '[.operations[] | select(.status == "implemented")] | length' "$enumeration")"
percent=$((covered * 100 / declared))
printf 'completeness: %d/%d (%d%%)\n' "$covered" "$declared" "$percent"
test "$percent" -ge "$threshold" || { rg -qi 'coverage[ -]?waiver' DECISIONS.md || exit 1; }

