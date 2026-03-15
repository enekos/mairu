# Contributing

Thanks for your interest in improving this project.

## Development Setup

1. Install dependencies:

```bash
npm install
npm --prefix dashboard install
```

2. Create a local environment file:

```bash
cp .env.example .env
```

3. Build and typecheck:

```bash
npm run build
npm run typecheck
```

## Useful Commands

- `npm run setup` - reset and initialize database schema (destructive).
- `npm run dashboard:api` - run API for dashboard.
- `npm run dashboard:dev` - run Svelte dashboard.
- `npm run eval:retrieval -- --dataset eval/dataset.json --topK 5 --verbose true` - run retrieval benchmark.

## Contribution Guidelines

- Keep changes focused and incremental.
- Update docs and examples when behavior changes.
- Preserve backward compatibility where practical.
- Do not commit secrets (`.env`, tokens, credentials).

## Pull Requests

Please include:

- What changed and why.
- How you tested the change.
- Any migration notes for users.
