# Mairu Go Runtime

This folder contains the Go runtime for Mairu:

- `cmd/` CLI entrypoints
- `internal/agent` coding agent core
- `internal/contextsrv` centralized context server
- `ui/` web frontend embedded into the Go binary

## Build

From repository root:

```bash
bun run mairu:build
```

Or directly:

```bash
go build -C mairu -o bin/mairu-agent ./cmd/mairu
```

## Run

```bash
./mairu/bin/mairu-agent tui
./mairu/bin/mairu-agent web -p 8080
./mairu/bin/mairu-agent context-server -p 8788

# Go-native ContextFS commands (use -P for project)
./mairu/bin/mairu-agent memory search "auth token" -P my-project -k 5
./mairu/bin/mairu-agent memory store "we use Postgres for context server" -P my-project -c observation -o agent -i 5
./mairu/bin/mairu-agent node search "authentication architecture" -P my-project -k 5
./mairu/bin/mairu-agent vibe query "how does auth work?" -P my-project -k 5
./mairu/bin/mairu-agent vibe mutation "remember we use gRPC internally" -P my-project -k 5

# File/Knowledge Ingestion
./mairu/bin/mairu-agent ingest README.md -P my-project -y

# Background context processing
./mairu/bin/mairu-agent daemon ./src -P my-project
```

## Notes

- The TypeScript context engine now lives at `mairu/contextfs/`.
- The unified dashboard UI lives at `mairu/ui/`.
- Core ContextFS workflows (`memory`, `skill`, `node`, `vibe`, `vibe-query`, `vibe-mutation`, `ingest`, `daemon`) are native Go commands in `mairu` CLI.
