# Testing Guidelines

## Go Tests
- Place tests alongside source files: `foo.go` → `foo_test.go`.
- Use table-driven tests for parameterized scenarios.
- Mock external dependencies (Meilisearch, LLM APIs) in unit tests.
- Race detector: run `go test -race` for concurrency-related changes.

## Evaluation
- LLM-driven features must have eval datasets in `llmeval/`.
- Run `./mairu/bin/mairu eval:retrieval` when changing retrieval behavior.
- Target: MRR ≥ 0.8, Recall@5 ≥ 0.75.

## Browser Extension
- Rust/WASM modules should have unit tests with `cargo test`.
- E2E tests use Playwright in `browser-extension/e2e/`.

## Regression Prevention
- If fixing a bug, add a test that would have caught it.
- If adding a feature, add at least one integration-level test path.
