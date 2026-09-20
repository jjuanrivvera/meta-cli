<div align="center">

# metactl

[![CI](https://github.com/jjuanrivvera/meta-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/jjuanrivvera/meta-cli/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/jjuanrivvera/meta-cli)](https://github.com/jjuanrivvera/meta-cli/releases/latest)
[![Coverage](https://img.shields.io/badge/coverage-%E2%89%A580%25-brightgreen)](https://github.com/jjuanrivvera/meta-cli/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/jjuanrivvera/meta-cli.svg)](https://pkg.go.dev/github.com/jjuanrivvera/meta-cli)
[![Go version](https://img.shields.io/github/go-mod/go-version/jjuanrivvera/meta-cli)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/jjuanrivvera/meta-cli)
[![Built with cliwright](https://img.shields.io/badge/built_with-cliwright-1f6feb)](https://cliwright.jjuanrivvera.com)

**Publish and manage Instagram, Facebook Pages, and WhatsApp Business from one command line.**

[Documentation](https://jjuanrivvera.github.io/meta-cli/) · [Command reference](https://jjuanrivvera.github.io/meta-cli/commands/metactl/)

</div>

## Install

The checksum-verifying installer supports macOS and Linux:

```sh
curl -fsSL https://raw.githubusercontent.com/jjuanrivvera/meta-cli/main/install.sh | sh
```

Windows users can install from Scoop after the first release:

```powershell
scoop bucket add meta-cli https://github.com/jjuanrivvera/scoop-meta-cli
scoop install meta-cli
```

To build the current checkout instead:

```sh
go install github.com/jjuanrivvera/meta-cli/cmd/metactl@latest
```

## Quick start

Create an account configuration, then enter a token at the hidden prompt:

```sh
metactl config set work \
  --graph-version v26.0 \
  --page-id 123456789 \
  --instagram-id 17841400000000000 \
  --waba-id 100000000000000 \
  --phone-id 100000000000001
metactl auth login --account work
metactl doctor --account work
```

Credentials go to the operating-system keyring. On a headless system, an encrypted file backend
can be selected explicitly with `METACTL_KEYRING_BACKEND=file` and a password supplied through
`METACTL_KEYRING_PASSWORD`; plaintext credentials are never written to configuration.

Explore and publish:

```sh
metactl instagram media list --account work --all -o table
metactl instagram publish reel --account work --video ./launch.mp4 \
  --cover-url https://cdn.example/cover.jpg --first-comment "Details in bio" --dry-run
metactl pages posts create --account work --message "Coming soon" \
  --published=false --scheduled-at 1789900000 --dry-run
metactl whatsapp templates list --account work -o json
```

All API operations support deterministic `table`, `json`, `yaml`, `csv`, and `id` output. Use
`--columns`, `--filter`, `--sort`, `--jq`, `--all`, and `--limit` to shape results. A dry run emits
copy-pasteable `curl` commands with credentials redacted.

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
