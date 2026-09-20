#!/usr/bin/env bash
# judge.sh — the ONE non-deterministic part of the gate: an LLM scores the subjective
# Definition-of-Done items a grep can't prove. Copied into a generated CLI under scripts/.
# CI without an agent: set CLIWRIGHT_SKIP_JUDGE=1 to bypass *intentionally* (logs a warning;
# it never silently passes).
set -uo pipefail
THRESHOLD="${CLIWRIGHT_JUDGE_MIN:-3}"

if [[ "${CLIWRIGHT_SKIP_JUDGE:-0}" == "1" ]]; then
  echo "⚠ judge skipped (CLIWRIGHT_SKIP_JUDGE=1) — subjective DoD items NOT verified" >&2
  exit 0
fi

read -r -d '' PROMPT <<EOF || true
You are a STRICT senior reviewer. Inspect this CLI repo: read internal/api/errors.go,
two command files, and the root --help/-h output. Score each 0-5 (FAIL if any < ${THRESHOLD}):
  1. Errors carry actionable hints keyed by status (401/403/404/429/5xx), not "request failed".
  2. Comments explain WHY, not WHAT.
  3. --help text includes runnable examples.
  4. Output/UX reads like a first-party tool (gh-quality).

Also perform a substantive implementation review instead of counting strings. Read
internal/api/client.go, internal/api/ratelimit.go, commands/auth.go, commands/instagram.go,
commands/pages.go, commands/whatsapp.go, commands/mcp.go, commands/upload.go,
commands/revision_test.go, e2e/e2e_test.go, and all four scripts/*check.sh or judge.sh gates.
FAIL if any of these is unsafe or only asserted rather than behaviorally tested:
  - app secrets and Page tokens cannot appear in dry-run, rendered, or MCP output;
  - Page operations use a derived Page token while /me/accounts uses the user token;
  - local uploads stream from disk and resume at Graph-reported offsets;
  - Page video chunks follow server offsets and thumbnails are multipart files;
  - Graph throttle codes/usage headers, Instagram status, partial publication failures,
    WhatsApp required multipart fields, and MCP root confinement match the implementation;
  - the fake Graph E2E server rejects bad auth, fields, body sizes, and operation order;
  - spec gates use exact canonical command resolution and the actual runnable command tree.
Run focused tests or commands if needed. Cite concrete file paths for any concern.
End with exactly one line: "VERDICT: PASS" or "VERDICT: FAIL".
EOF

if command -v claude >/dev/null 2>&1; then
  if ! out=$(timeout 30 claude -p --permission-mode plan --permission-prompts none "$PROMPT" 2>/dev/null); then
    if command -v codex >/dev/null 2>&1; then
      echo "⚠ primary judge unavailable; retrying with the fallback reviewer" >&2
      out=$(timeout 300 codex exec --sandbox read-only "$PROMPT" 2>/dev/null)
    else
      echo "✗ primary judge did not finish and no fallback reviewer is installed" >&2
      exit 1
    fi
  fi
elif command -v codex >/dev/null 2>&1; then out=$(timeout 300 codex exec --sandbox read-only "$PROMPT" 2>/dev/null)
else
  echo "⚠ no agent (claude/codex) found to run the judge — set CLIWRIGHT_SKIP_JUDGE=1 to bypass intentionally" >&2
  exit 1
fi

echo "$out"
FINAL_LINE=$(awk 'NF { line=$0 } END { print line }' <<<"$out")
[[ "$FINAL_LINE" == "VERDICT: PASS" ]] || { echo "✗ judge verdict: FAIL (subjective DoD items)"; exit 1; }
echo "✓ judge verdict: PASS"
