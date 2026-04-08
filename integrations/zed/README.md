# Mairu Zed Extension

This extension integrates the Mairu Agent into Zed as a Model Context Protocol (MCP) server.

## Installation

Since this extension is in development, you can install it as a local dev extension in Zed:

1. Open Zed.
2. Run the `zed: extensions` command from the command palette.
3. Click the "Install Dev Extension" button in the Extensions view.
4. Select the `integrations/zed` folder.

## Configuration

By default, the extension assumes `mairu` is available in your system's PATH.
If it is installed somewhere else, you can configure the path in Zed's settings (`settings.json`):

```json
{
  "context_servers": {
    "mairu": {
      "path": "/path/to/mairu/bin/mairu"
    }
  }
}
```

## Usage

Once installed, Zed will automatically start `mairu mcp` when it needs context. It exposes tools like `search_memories`, `store_memory`, `search_nodes`, `vibe_query`, and `vibe_mutation` which Zed can use to retrieve context from the `mairu` database.

Make sure you have Mairu configured properly and that you pass the correct `-P, --project` identifiers in prompts if necessary (although the Mairu agent MCP server handles extracting them via the agent prompts).
