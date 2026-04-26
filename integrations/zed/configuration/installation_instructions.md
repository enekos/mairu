# Mairu for Zed

## Prerequisites

1. Install the `mairu` CLI and ensure it is on `$PATH` (or set `path` in extension settings).
2. Start the Mairu context server in the background — slash commands need its HTTP API:
   ```bash
   mairu context-server &           # listens on :8788 by default
   ```
   Optionally start the dashboard for `/mairu-dashboard`:
   ```bash
   make dashboard                    # Svelte UI on :5173
   ```

## What you get

**Context server (MCP):** Zed's Assistant gets tools `search_memories`,
`store_memory`, `search_nodes`, `vibe_mutation` — all auto-scoped to the
project derived from the open worktree (override via the `default_project`
setting).

**Slash commands** (in the Assistant panel):

| Command | Description |
|---|---|
| `/mairu-search <query>` | Inject top memory hits for `<query>` |
| `/mairu-nodes <query>`  | Inject top hierarchical context-node hits |
| `/mairu-recall <query>` | Both memories + nodes, formatted |
| `/mairu-dashboard [q]`  | Print a clickable dashboard URL |
| `/mairu-doctor`         | Show context-server health + active project |

## Project scoping

The extension passes `MAIRU_DEFAULT_PROJECT` to `mairu mcp` so every tool call
is auto-scoped. Resolution order:

1. `default_project` setting (this extension)
2. `MAIRU_DEFAULT_PROJECT` from the worktree's shell env
3. Basename of the open worktree's root path
