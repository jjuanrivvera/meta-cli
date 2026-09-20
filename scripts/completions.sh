#!/usr/bin/env bash
set -euo pipefail
mkdir -p completions
go run ./cmd/metactl completion bash > completions/metactl.bash
go run ./cmd/metactl completion zsh > completions/metactl.zsh
go run ./cmd/metactl completion fish > completions/metactl.fish

