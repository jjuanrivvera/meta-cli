#!/usr/bin/env bash
# dod-check.sh — deterministic Definition-of-Done checks.
# One concrete check per atomic criterion. Copied into a generated CLI under scripts/.
# Usage: ./scripts/dod-check.sh <binary-name>
set -uo pipefail
BIN="${1:-meta}"
fail=0

ok()   { printf "  ✓ %s\n" "$1"; }
bad()  { printf "  ✗ %s\n" "$1"; fail=1; }
# have <description> <test-command...>
have() { if eval "${*:2}" >/dev/null 2>&1; then ok "$1"; else bad "$1"; fi; }

echo "Definition-of-Done checks for '$BIN':"

# Agent surface
have "mcp server command present"        "rg -lq 'ophis|mcp' commands/mcp.go"
have "agent guard command present"       "test -f commands/agent.go"
have "guard PreToolUse hook generator"   "rg -lq 'PreToolUse' commands/agent_hosts.go"
have "guard hook execution battery"      "test -f commands/agent_hook_test.go"
have "guard hook path-prefix hardening"  "rg -Fq '([^[:space:]]*/)?' commands/agent_hosts.go"
guard_flattening_present() { rg -Fq "tr '\\\\n{}:,'" commands/agent_hosts.go; }
have "guard no-jq JSON flattening"       "guard_flattening_present"

# MCP tool annotations. Deliberately a RUNTIME check: a grep for
# "annotate" passes on a CLI that tags every command under its own key names and
# still exports annotations:null, because ophis only reads the singular MCP hint
# keys (readOnlyHint, …). Hosts running a read-only session drop a server with no
# read-only tool, so only the exported list proves it. Skips if jq is absent.
mcp_annotations_present() {
  command -v jq >/dev/null || return 0
  local d mp rc=1
  d=$(mktemp -d .meta-mcp-check.XXXXXX) || return 1
  mp=$(go list -f '{{if eq .Name "main"}}{{.ImportPath}}{{end}}' ./... 2>/dev/null | head -1)
  if [[ -n "$mp" ]] && go build -o "$d/bin" "$mp" >/dev/null 2>&1 &&
     (cd "$d" && ./bin mcp tools >/dev/null 2>&1) &&
     jq -e '[(if type=="array" then . else .tools end)[]
            | select(.annotations.readOnlyHint == true)] | length > 0' \
        "$d/mcp-tools.json" >/dev/null 2>&1; then
    rc=0
  fi
  rm -rf "$d"
  return "$rc"
}
have "MCP tools carry readOnlyHint"      "mcp_annotations_present"

# Output formats (atomic — one per format)
for f in json yaml csv table; do
  have "output format: $f"               "rg -liq '\"$f\"|format$f|$f *format' internal/output"
done

# Resilience & safety
have "--dry-run prints equivalent curl"  "rg -lq 'dry-run' . && rg -lq 'curl' internal/api"
have "Ctrl-C: signal.NotifyContext"      "rg -lq 'signal.NotifyContext' cmd"
have "no stray context.Background()"     "! rg -lq --glob '!*_test.go' 'context.Background()' commands internal/api"
have "secrets in OS keyring"             "rg -q 'zalando/go-keyring' go.mod"
# Interactive secret input must be hidden. fmt.Scan/Scanln/Scanf echo the secret to the
# terminal and stall on long pastes — read via promptSecret (term.ReadPassword) instead.
have "no plaintext stdin reads (fmt.Scan*)"  "! rg -lq --glob '!*_test.go' 'fmt\.Scan(ln|f)?\(' ."
have "idempotent-only retry"             "rg -lq 'idempotent|MethodGet|MethodPut|MethodDelete' internal/api"

# Meta commands (atomic — one per command)
for c in auth config init doctor completion alias api version; do
  have "meta command: $c"                "test -f commands/$c.go || rg -lq '\"$c\"' commands"
done

# Distribution & CI
have "GoReleaser config present"         "test -f .goreleaser.yaml || test -f .goreleaser.yml"
# If goreleaser is installed it MUST pass; if absent, skip (don't fake a pass with '|| true').
have "goreleaser check clean"            "! command -v goreleaser >/dev/null || goreleaser check"
have "install.sh present"                "test -f install.sh"
# If shellcheck is installed the installer MUST pass; if absent, skip.
have "install.sh shellcheck clean"       "! command -v shellcheck >/dev/null || ! test -f install.sh || shellcheck install.sh"
have "CI workflow present"               "test -f .github/workflows/ci.yml"
have "release workflow present"          "test -f .github/workflows/release.yml"

# Hygiene
for doc in README.md LICENSE CHANGELOG.md SECURITY.md AGENTS.md; do
  have "doc: $doc"                       "test -f $doc"
done
have "no committed token"                "! rg -lq '(api[_-]?key|token)\s*[:=]\s*[A-Za-z0-9_-]{16,}' --glob '!*.sh' --glob '!scripts/**' ."

# Behavioral acceptance added on top of the template. These execute the guarantees that source
# searches cannot prove and are regression tests whose old implementations fail.
have "revision safety regression suite"  "go test -count=1 ./commands ./internal/api ./internal/output"
have "strict fake Graph E2E suite"        "go test -count=1 -tags=e2e ./e2e"
have "auth package coverage at least 80%" "go test -count=1 -cover ./internal/auth | rg -q 'coverage: (8[0-9]|9[0-9]|100)(\\.[0-9]+)?%'"
have "config package coverage at least 80%" "go test -count=1 -cover ./internal/config | rg -q 'coverage: (8[0-9]|9[0-9]|100)(\\.[0-9]+)?%'"
have "spec rejects unknown verbs"         "! bin/$BIN __surface resolve pages posts definitely-not-a-verb"
have "manifest equals command tree"       "scripts/spec-completeness.sh api-manifest.json 90"
have "update command"                     "bin/$BIN update --help"
have "live-smoke runbook"                 "test -f docs/live-smoke.md"
have "generated command docs"             "test -f docs/commands/meta.md"

if [[ $fail -ne 0 ]]; then
  echo "✗ Definition-of-Done incomplete"; exit 1
fi
echo "✓ Definition-of-Done satisfied"
