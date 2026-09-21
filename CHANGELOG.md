# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- **Breaking:** the command is now `meta` (previously `metactl`), and every environment variable
  uses the `META_` prefix (previously `METACTL_`). The installer's release pin is
  `META_CLI_VERSION`. The repository, Go module, packages, and release archives stay `meta-cli`.
- Configuration now lives in the `meta` directory under the user configuration directory
  (previously `metactl`); move an existing `config.yaml` to keep its accounts.
- The OS keyring service is now `meta`; run `meta auth login` again after upgrading to store
  credentials under the new service name.

### Added

- Unified Instagram, Facebook Pages, and WhatsApp Business command surface.
- Resumable uploads, scheduled posts, composite publishing, cursor pagination, and rate-limit handling.
- Multi-account configuration, keyring-backed credentials, deterministic output, and redacted dry runs.
- MCP tools and generated host safety policies based on command annotations.
- Generated command documentation, release packaging, and local fake-server acceptance tests.

[Unreleased]: https://github.com/jjuanrivvera/meta-cli/commits/main
