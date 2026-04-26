# Mairu Zed Extension

Integrates Mairu into Zed as both an MCP context server and a set of slash
commands so you can browse memories, hierarchical context nodes, and the
dashboard directly from the Assistant panel.

## Installation

While this extension is in development, install it as a local dev extension:

1. Open Zed.
2. Run the `zed: extensions` command from the command palette.
3. Click "Install Dev Extension".
4. Select the `integrations/zed` folder.

You also need the Mairu HTTP context server running for slash commands:

```bash
mairu context-server &           # http://localhost:8788
make dashboard                    # optional: Svelte UI on :5173
```

## Features

### MCP tools (Assistant panel)

`search_memories`, `store_memory`, `search_nodes`, `vibe_mutation`. Each call
is auto-scoped to the current project — see "Project scoping" below.

### Slash commands

| Command | What it does |
|---|---|
| `/mairu-search <query>` | Inject top memory hits |
| `/mairu-nodes <query>`  | Inject top hierarchical context-node hits |
| `/mairu-recall <query>` | Memories + nodes together |
| `/mairu-dashboard [q]`  | Print a clickable dashboard link |
| `/mairu-doctor`         | Show context-server health + active project |

## Running the mairu agent inside Zed (ACP)

Beyond MCP, mairu ships an Agent Client Protocol server (`mairu acp`) so you
can run the **mairu agent itself** in Zed's Agent panel — with native access
to memories, bash history, scrape, edit, and analyze tools. ACP agents are
not yet registerable from extensions; wire it up in your Zed `settings.json`:

```json
{
  "agent_servers": {
    "mairu": {
      "command": "mairu",
      "args": ["acp"]
    }
  }
}
```

Restart Zed and pick `mairu` as the agent in the Agent panel. Tool calls,
streaming output, and approval prompts all flow through the ACP protocol.

## Configuration

```json
{
  "context_servers": {
    "mairu": {
      "settings": {
        "path": "/usr/local/bin/mairu",
        "default_project": "myproject",
        "api_url": "http://localhost:8788",
        "auth_token": null
      }
    }
  }
}
```

All settings are optional. `path` selects the binary; `default_project`,
`api_url`, and `auth_token` are forwarded as `MAIRU_DEFAULT_PROJECT`,
`MAIRU_CONTEXT_SERVER_URL`, and `MAIRU_CONTEXT_SERVER_TOKEN` env vars to the
spawned `mairu mcp` process.

### Project scoping

The `project` argument no longer needs to be supplied for every tool call.
Resolution order:

1. Explicit `project` arg in the tool call (or slash command worktree)
2. `default_project` setting (extension settings, MCP only)
3. `MAIRU_DEFAULT_PROJECT` from the worktree shell env
4. Basename of the open worktree (slash commands only)

### Slash-command env overrides

Slash commands run inside Zed's WASM sandbox and read the worktree shell env
rather than extension settings. To override defaults set these in your shell:

| Var | Default |
|---|---|
| `MAIRU_API_URL` / `MAIRU_CONTEXT_SERVER_URL` | `http://localhost:8788` |
| `MAIRU_DASHBOARD_URL` | `http://localhost:5173` |
| `MAIRU_CONTEXT_SERVER_TOKEN` | none |
| `MAIRU_DEFAULT_PROJECT` | worktree basename |
