#!/usr/bin/env bash
set -euo pipefail
binary="${1:-metactl}"
fail=0
check() { if eval "$2" >/dev/null 2>&1; then printf '  ok %s\n' "$1"; else printf '  missing %s\n' "$1" >&2; fail=1; fi; }
check "binary" "test -x bin/$binary || make build"
check "four output formats" "rg -q 'FormatTable' internal/output && rg -q 'FormatJSON' internal/output && rg -q 'FormatYAML' internal/output && rg -q 'FormatCSV' internal/output"
check "dry-run curl" "rg -q 'curl' internal/api"
check "signal cancellation" "rg -q 'signal.NotifyContext' cmd"
check "keyring" "rg -q 'zalando/go-keyring' go.mod"
check "MCP server" "test -f commands/mcp.go && bin/$binary mcp tools >/dev/null"
check "guard" "test -f commands/agent.go && test -f commands/agent_hosts.go && test -f commands/agent_hook_test.go"
check "guard hook" "rg -q 'PreToolUse' commands/agent_hosts.go"
check "no plain secret scanning" "! rg -q --glob '!**/*_test.go' 'fmt\\.Scan(ln|f)?\\(' ."
for command in auth config init doctor completion alias api version; do check "command $command" "bin/$binary $command --help"; done
for doc in README.md CHANGELOG.md SECURITY.md LICENSE DECISIONS.md AGENTS.md docs/live-smoke.md; do check "$doc" "test -f $doc"; done
check "generated docs" "test -f docs/commands/metactl.md"
check "installer" "test -f install.sh"
check "release config" "goreleaser check"
test "$fail" -eq 0 || exit 1
echo "Definition of Done checks passed"

