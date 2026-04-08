# Mairu Web Dashboard

This is the unified Svelte web UI for the Mairu coding agent and ContextFS server. It provides a visual interface for managing your context nodes, memories, skills, and viewing agent chat streams.

## Tech Stack

- **Svelte 5**
- **Vite**
- **Tailwind CSS** (via standard Vite/Svelte integration)

## Development

The dashboard is designed to be run alongside the `mairu context-server` API backend.

1. Ensure dependencies are installed:
```bash
bun install
```

2. Run the full stack from the repository root:
```bash
make dashboard
```

This will concurrently:
- Start the Go context server API on port `8788`.
- Start the Vite development server for the Svelte UI on port `5173`.

Alternatively, to run just the UI dev server:
```bash
bun run dev
```

## Building for Production

To build the static UI assets:
```bash
bun run build
```

*(Note: When fully embedded, the Go backend serves these compiled static assets directly).*