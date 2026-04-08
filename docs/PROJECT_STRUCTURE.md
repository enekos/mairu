# Mairu Project Structure

This repository is organized as a single project named **Mairu** with a unified Go runtime.

## Components

- `mairu/` - Go runtime and core product surface
  - `cmd/` - CLI entrypoints (`mairu` binary)
  - `internal/agent/` - coding agent engine
  - `internal/contextsrv/` - centralized context server (HTTP API)
  - `ui/` - unified Svelte web UI for chat + context dashboard features
- `docs/` - project-level docs, specs, and plans

## Typical Flows

### 1) Run Unified Dashboard Stack

```bash
make dashboard
```

This starts:
- `mairu context-server` on port `8788`
- `mairu/ui` dev server on port `5173`

### 2) Run Mairu Agent (Go)

```bash
make mairu-build
./mairu/bin/mairu tui
```

### 3) Run Central Context Server

```bash
./mairu/bin/mairu context-server -p 8788
```

## Data and Runtime Artifacts

Local Meilisearch artifacts are created at the repository root in `.mairu/` (if using local fallback script) or managed by Docker.

Both paths are git-ignored.

## Naming Policy

- Project name: **Mairu**
- Go binary: `mairu`
- Web UI: **Mairu UI**
