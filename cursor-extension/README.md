# ContextFS Cursor Extension

Native Cursor extension that adds a **ContextFS** activity bar panel for running `context-cli` queries.

## Included Query Actions

- Memory Search
- Node Search
- Skill Search
- Vibe Query
- Vibe Mutation Plan (preview only, no apply)

## Requirements

- `context-cli` must be available in PATH, or configured with `contextfs.cliPath`.
- ContextFS backend prerequisites (Elasticsearch, env vars) should already be configured.

## Development

```bash
bun install
bun run test
bun run typecheck
bun run build
```

## Run In Cursor/VS Code

1. Open `cursor-extension` as an extension project.
2. Run build once: `bun run build`.
3. Launch extension host (F5 / "Run Extension").
4. Open the **ContextFS** activity bar view.

## Settings

- `contextfs.cliPath` (string): CLI executable path, default `context-cli`
- `contextfs.defaultTopK` (number): default topK, default `5`
- `contextfs.commandTimeoutMs` (number): timeout per command, default `30000`
