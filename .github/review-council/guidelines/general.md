# General Project Guidelines

These guidelines apply to all code changes in this repository.

## Project Structure
- **mairu/**: Go-based context/memory server and CLI
- **browser-extension/**: Rust/WASM Chrome extension
- **llmeval/**: Go evaluation harness
- **pii-redact/**: Go PII redaction pipeline
- **integrations/**: Editor plugins (nvim, raycast, zed)

## Code Style
- **Go**: Follow standard Go conventions (`gofmt`, `golangci-lint`).
- **Rust**: Follow `cargo fmt` and `clippy` guidelines.
- **TypeScript/Svelte**: Follow the project's ESLint/Prettier config.

## PR Hygiene
- Keep changes focused and incremental.
- Update docs and examples when behavior changes.
- Do not commit secrets (`.env`, tokens, credentials).
- Include tests for new behavior.
- Update `ARCHITECTURE_GUIDE.md` if adding new major components.

## Testing Requirements
- Go code must have unit tests for non-trivial logic.
- Integration tests requiring external services (Meilisearch, LLMs) must be marked.
- Run `make test` before submitting.

## Dependencies
- Vet new Go/Rust/JS dependencies for maintenance status and security.
- Prefer standard library when equivalent.
- Document why a new dependency is necessary in the PR description.
