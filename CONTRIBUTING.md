# Contributing

Thanks for your interest in improving this project.

## Development Setup

1. Install dependencies:

```bash
bun install
bun --cwd mairu/ui install
```

2. Create a local environment file:

```bash
cp .env.example .env
```

3. Build Go and typecheck frontend:

```bash
make build
bun run --cwd mairu/ui check
```

## Useful Commands

- `make setup` - reset and initialize database schema (destructive).
- `make dashboard` - run Go context server API and Svelte dev server.
- `bun run --cwd mairu/ui dev` - run unified Svelte dashboard (`mairu/ui`).
- `make eval-retrieval` - run retrieval benchmark.
- `mairu memory search "query" -P my-project --mode surface` - curated-memory-first retrieval.
- `mairu memory feedback -P my-project --arm <arm> --outcome accepted|ignored --rank 1` - feed reward signals.
- `mairu memory policy -P my-project` / `mairu memory policy -P my-project --reset` - inspect/reset adaptive policy state.

## Contribution Guidelines

- Keep changes focused and incremental.
- Update docs and examples when behavior changes.
- Preserve backward compatibility where practical.
- Do not commit secrets (`.env`, tokens, credentials).
- If changing adaptive retrieval behavior, run eval in both baseline and adaptive modes.

## Pull Requests

Please include:

- What changed and why.
- How you tested the change.
- Any migration notes for users.
