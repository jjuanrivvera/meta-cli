# Security policy

## Supported versions

Security fixes are applied to the latest released version. Until the first release, use the latest
commit on `main`.

## Report a vulnerability

Please use GitHub's private vulnerability reporting for this repository. Do not open a public issue
with exploit details, tokens, account identifiers, or request captures containing credentials.

Include the affected version, platform, reproduction steps, expected impact, and any proposed
mitigation. You should receive an acknowledgement within seven days.

## Credential handling

`metactl` stores access tokens and app secrets in the operating-system keyring. The optional
headless fallback is encrypted with AES-GCM and a password-derived key. Configuration files contain
only non-secret account metadata. Dry runs redact credentials by default, and secret flags are not
exported as MCP tool inputs.
