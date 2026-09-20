# Decisions

This file pins choices whose alternatives would otherwise make the command surface drift.

1. **Graph version** → default to `v26.0`, released July 29, 2026, and keep it configurable per account and through `METACTL_GRAPH_VERSION` → Meta versions all Graph endpoints and callers need a stable opt-out when migrations take time.
2. **Playbook version label** → follow the installed playbook content plus the stricter project requirements → the external brief calls it 0.7.0 while the installed metadata says 0.3.0, so behavior is safer than choosing by a mismatched label.
3. **Resource pattern** → service-layer operations declared through one generic command builder → Graph edges, upload phases, field expansions, and publish actions are not uniform CRUD resources.
4. **Enumeration source** → 66 operations transcribed on 2026-09-19 from Meta's v26 documentation for Instagram publishing/comments/insights, Pages posts/video/reels/insights, the partial official WhatsApp specification, and the docs-derived enumeration in the sibling WhatsApp CLI → Pages and Instagram have no complete machine-readable specification.
5. **Composite commands** → list them in the manifest command surface but count only their underlying documented HTTP operations for API completeness → a composite is a convenience workflow, not a new vendor endpoint.
6. **Authentication** → static bearer tokens with optional `appsecret_proof`; token and app secret are separate keyring entries → user, Page, and System User tokens share the same HTTP scheme while the proof must never be written to configuration.
7. **Profiles** → call them accounts with `--account`; retain hidden `--profile` compatibility → one account groups the business identifiers, Graph version, and credential set.
8. **Pagination** → follow `paging.cursors.after`; never follow an absolute `paging.next` URL → rebuilding through the configured base URL prevents an API response from redirecting credentials to another host.
9. **Rate limiting** → start at 10 requests/second, inspect `X-App-Usage`, `X-Business-Use-Case-Usage`, and `X-Page-Usage`, slow near saturation, halve on 429, then restore gradually → Meta exposes usage percentages rather than a simple remaining counter.
10. **Retries** → retry transient network failures, 429, and 5xx only for idempotent methods; upload chunks may retry because offsets make the chunk idempotent → ordinary Graph POST operations must never duplicate a post or message.
11. **File uploads** → allow only paths explicitly supplied as command flags and never accept file indirection inside API data → this avoids turning response or manifest data into an arbitrary local-file read.
12. **Conditional modules** → no event store, offline cache, cross-account sync, or live smoke workflow; no complete refetchable spec and no durable read-only credential → publishing is freshness-sensitive and local deterministic fakes are the required test boundary.
13. **Existing tooling** → keep the unified binary even though Meta publishes SDKs and the sibling repository covers WhatsApp → the requested value is one consistent surface across Instagram, Pages, and WhatsApp.
14. **Distribution** → local commits only; prepare release and package configuration but do not create repositories, push, tag, or publish → the authorized scope is `+commit`.
15. **Packaging add-on** → not applicable for this build → no separate editor plugin package was requested; the binary's MCP surface is the automation integration.
16. **Toolchain** → declare Go 1.25 with the fleet's Go 1.25.12 toolchain → local newer Go may build it, while CI tests the declared fleet floor.
17. **Acceptance rubric** → keep the five subjective checks deterministic and network-independent → the complete gate must be reproducible from a clean checkout without an external service.
