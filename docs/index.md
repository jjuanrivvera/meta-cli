# metactl

`metactl` is one command-line client for Instagram publishing, Facebook Pages, and WhatsApp
Business. It provides typed resource commands, multi-account configuration, safe dry runs,
resilient uploads, cursor pagination, and machine-readable output.

## Start here

```sh
metactl config set work --graph-version v26.0 --page-id 123456789
metactl auth login --account work
metactl doctor --account work
metactl pages posts list --account work --all
```

Use the [command reference](commands/metactl.md) for every command and flag. The
[workflow guide](workflows.md) covers common publishing tasks.

## Safety defaults

- Credentials are stored in a keyring, never in the YAML configuration.
- Dry-run output redacts access tokens unless `--show-token` is explicitly set.
- Retries are limited to idempotent calls and resumable upload chunks.
- Pagination follows cursor values through the configured host instead of trusting absolute links.
- Destructive commands are distinctly annotated for automation policy generation.

## Local verification

Run `make verify` for the complete deterministic acceptance gate. Run `make e2e` to exercise the
built executable against the local fake Graph server.
