<div align="center">

# meta

[![CI](https://github.com/jjuanrivvera/meta-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/jjuanrivvera/meta-cli/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/jjuanrivvera/meta-cli)](https://github.com/jjuanrivvera/meta-cli/releases/latest)
[![Coverage](https://img.shields.io/badge/coverage-%E2%89%A580%25-brightgreen)](https://github.com/jjuanrivvera/meta-cli/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/jjuanrivvera/meta-cli.svg)](https://pkg.go.dev/github.com/jjuanrivvera/meta-cli)
[![Go version](https://img.shields.io/github/go-mod/go-version/jjuanrivvera/meta-cli)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/jjuanrivvera/meta-cli)

**Publish and manage Instagram, Facebook Pages, and WhatsApp Business from one command line.**

[Documentation](https://jjuanrivvera.github.io/meta-cli/) · [Command reference](https://jjuanrivvera.github.io/meta-cli/commands/meta/)

</div>

## Install

Before the first tagged release, install directly from source with Go 1.25 or newer:

```sh
go install github.com/jjuanrivvera/meta-cli/cmd/meta@latest
```

After the first tagged release, the checksum-verifying installer will support macOS and Linux:

```sh
curl -fsSL https://raw.githubusercontent.com/jjuanrivvera/meta-cli/main/install.sh | sh
```

Windows users will also be able to install from Scoop:

```powershell
scoop bucket add meta-cli https://github.com/jjuanrivvera/scoop-meta-cli
scoop install meta-cli
```

To build a local checkout instead:

```sh
make build
./bin/meta version
```

The installed command is `meta`; an unrelated npm package also installs a command by that name, so
check `command -v meta` if both are present and adjust your `PATH` order.

The installer reads `META_CLI_VERSION` to pin a release; every runtime setting uses the `META_`
prefix described below.

## Quick start

Create an account configuration, then enter a token at the hidden prompt:

```sh
meta config set work \
  --graph-version v26.0 \
  --page-id 123456789 \
  --instagram-id 17841400000000000 \
  --waba-id 100000000000000 \
  --phone-id 100000000000001
meta auth login --account work
meta auth pages --account work --page-id 123456789 --save
meta doctor --account work
```

Credentials go to the operating-system keyring. On a headless system, an encrypted file backend
can be selected explicitly with `META_KEYRING_BACKEND=file` and a password supplied through
`META_KEYRING_PASSWORD`; plaintext credentials are never written to configuration.
The `auth pages --save` step derives and stores the Page access token that every `pages` command
uses. Set `META_APP_SECRET` for one invocation or use `auth login --prompt-app-secret` when
`appsecret_proof` is required; app secrets are never accepted as command-line values.

Explore and publish:

```sh
meta instagram media list --account work --all -o table
meta instagram publish reel --account work --video ./launch.mp4 \
  --cover-url https://cdn.example/cover.jpg --first-comment "Details in bio" --dry-run
SCHEDULED_AT=$(date -u -v+1H +%s 2>/dev/null || date -u -d '+1 hour' +%s)
meta pages posts create --account work --message "Coming soon" \
  --published=false --scheduled-at "$SCHEDULED_AT" --dry-run
meta whatsapp templates list --account work -o json
```

All API operations support deterministic `table`, `json`, `yaml`, `csv`, and `id` output. Use
`--columns`, `--filter`, `--sort`, `--jq`, `--all`, and `--limit` to shape results. A dry run emits
copy-pasteable `curl` commands with credentials redacted.

## Exit codes

- `0`: the requested operation completed successfully.
- `1`: the operation failed without a completed remote side effect.
- `2`: the primary publish completed, but a follow-up step failed. The JSON result retains the
  published object ID and identifies the failed step. Automation must record the ID and must not
  retry the publish, because doing so can create duplicate content.

MCP upload tools confine file reads to `META_MCP_ROOT`; when it is unset, the MCP server's
working directory is the root. Symlinks that resolve outside that root are rejected.

## Development

```sh
make setup-hooks
make verify
make e2e
```

`make e2e` builds the binary and exercises the publishing workflows against a local fake Graph
server. It does not need an account or network credential.

## License

[MIT](LICENSE)
