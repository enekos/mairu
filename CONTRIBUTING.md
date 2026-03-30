# Contributing

Thanks for your interest in improving this project.

## Development Setup

1. Install dependencies:

```bash
bun install
bun --cwd dashboard install
```

2. Create a local environment file:

```bash
cp .env.example .env
```

3. Build and typecheck:

```bash
bun run build
bun run typecheck
```

## Useful Commands

- `bun run setup` - reset and initialize database schema (destructive).
- `bun run dashboard:api` - run API for dashboard.
- `bun run dashboard:dev` - run Svelte dashboard.
- `bun run eval:retrieval -- --dataset eval/dataset.json --topK 5 --verbose true` - run retrieval benchmark.
- `context-cli memory search "query" -P my-project --mode surface` - curated-memory-first retrieval.
- `context-cli memory feedback -P my-project --arm <arm> --outcome accepted|ignored --rank 1` - feed reward signals.
- `context-cli memory policy -P my-project` / `context-cli memory policy -P my-project --reset` - inspect/reset adaptive policy state.

## Contribution Guidelines

- Keep changes focused and incremental.
- Update docs and examples when behavior changes.
- Preserve backward compatibility where practical.
- Do not commit secrets (`.env`, tokens, credentials).
- If changing adaptive retrieval behavior, run eval in both baseline and adaptive modes:
  - `bun run eval:retrieval -- --dataset eval/dataset.json --project my-project --adaptive-compare true`

## Pull Requests

Please include:

- What changed and why.
- How you tested the change.
- Any migration notes for users.
