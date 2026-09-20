# Contributor guide

`metactl` is a Go and Cobra command-line client for Meta Business Graph API surfaces.

- Run `make verify` before every commit. It includes formatting, static analysis, security checks, tests, docs drift, the API manifest gates, coverage, black-box E2E, the Definition-of-Done checks, and the review rubric.
- `specs/enumeration.json` is the endpoint inventory. `api-manifest.json` is the public command contract. Update both before changing API command coverage.
- All HTTP calls go through `internal/api.Client`. Tests use loopback fake servers; never use live credentials or Meta hosts.
- Secrets belong in the keyring or encrypted fallback. Configuration contains only non-secret account metadata.
- Resource commands are registered through the shared operation builder and must carry a read, write, or destructive annotation.
- Comments explain constraints and rationale. Prefer clear names over narration.
- Distribution stops at local commits unless the owner explicitly expands it.

