#!/usr/bin/env bash
# The subjective acceptance gate needs a local agent; CI must opt out explicitly.
set -uo pipefail

if [[ "${CLIWRIGHT_SKIP_JUDGE:-0}" == "1" ]]; then
  echo "⚠ judge skipped (CLIWRIGHT_SKIP_JUDGE=1) — subjective DoD items NOT verified" >&2
  exit 0
fi

read -r -d '' PROMPT <<'EOF' || true
You are a STRICT senior reviewer. Inspect this CLI repository: read internal/api/errors.go,
two command files, and the root --help/-h output. Score each item from 0-5 and fail if any is below 3:
  1. Errors carry actionable hints keyed by status (401/403/404/429/5xx), not generic failures.
  2. Comments explain why, not what.
  3. --help text includes runnable examples.
  4. Output and UX read like a first-party CLI.
End with exactly one line: "VERDICT: PASS" or "VERDICT: FAIL".
EOF

if command -v claude >/dev/null 2>&1; then
  out=$(claude -p "$PROMPT" 2>/dev/null)
elif command -v codex >/dev/null 2>&1; then
  out=$(codex exec "$PROMPT" 2>/dev/null)
else
  echo "⚠ no agent (claude/codex) found to run the judge — set CLIWRIGHT_SKIP_JUDGE=1 to bypass intentionally" >&2
  exit 1
fi

echo "$out"
rg -q 'VERDICT: PASS' <<<"$out" || {
  echo "✗ judge verdict: FAIL (subjective DoD items)"
  exit 1
}
echo "✓ judge verdict: PASS"
