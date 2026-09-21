#!/usr/bin/env bash
set -euo pipefail
mkdir -p completions
go run ./cmd/meta completion bash > completions/meta.bash
go run ./cmd/meta completion zsh > completions/meta.zsh
go run ./cmd/meta completion fish > completions/meta.fish

