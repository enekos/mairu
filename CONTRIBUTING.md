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

4. Optional local hooks:

```bash
make install-hooks
```

## Useful Commands

- `make setup` - initialize/reset Meilisearch indexes (destructive).
- `make dashboard` - run Go context server API and Svelte dev server.
- `bun run --cwd mairu/ui dev` - run unified Svelte dashboard (`mairu/ui`).
- `make eval-retrieval` - run retrieval benchmark.
- `./mairu/bin/mairu memory search "query" -P my-project` - search project memories.
- `./mairu/bin/mairu vibe query "how does auth work?" -P my-project` - natural-language retrieval.
- `./mairu/bin/mairu vibe mutation "remember X" -P my-project -y` - store/update context from plain English.

## Contribution Guidelines

- Keep changes focused and incremental.
- Update docs and examples when behavior changes.
- Do not commit secrets (`.env`, tokens, credentials).
- If changing adaptive retrieval behavior, run eval in both baseline and adaptive modes.

## Pull Requests

Please include:

- What changed and why.
- How you tested the change.
- Any migration notes for users.
