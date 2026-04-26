# Security Guidelines

## Secrets Management
- NEVER commit API keys, tokens, or credentials to the repository.
- Use environment variables loaded from `.env` (which is in `.gitignore`).
- The `pii-redact/` module must be used when persisting command output that may contain secrets.

## Input Handling
- All user-facing inputs (CLI args, API payloads, file reads) must be validated.
- When executing shell commands, use proper escaping — prefer structured execution over string concatenation.

## Network
- The browser extension native host binds to `127.0.0.1` only.
- All external API calls should have reasonable timeouts.
- Verify TLS certificates in production contexts.

## Data Privacy
- Command history and memories may contain sensitive data.
- The redaction pipeline (5 layers: regex, heuristics, entropy, denylist, damage cap) must run before persisting bash history.
- Logs must not print secrets at `INFO` level or below.
