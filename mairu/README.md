# Mairu Go Runtime

This folder contains the Go runtime for Mairu:

- `cmd/` CLI entrypoints
- `internal/agent` coding agent core
- `internal/contextsrv` centralized context server
- `ui/` web frontend embedded into the Go binary

## Build

From repository root:

```bash
make mairu-build
```

Or directly:

```bash
go build -C mairu -o bin/mairu ./cmd/mairu
```

## Run

```bash
./mairu/bin/mairu tui
./mairu/bin/mairu web -p 8080
./mairu/bin/mairu context-server -p 8788

# Go-native ContextFS commands (use -P for project)
./mairu/bin/mairu memory search "auth token" -P my-project -k 5
./mairu/bin/mairu memory store "we use Postgres for context server" -P my-project -c observation -o agent -i 5
./mairu/bin/mairu node search "authentication architecture" -P my-project -k 5
./mairu/bin/mairu vibe query "how does auth work?" -P my-project -k 5
./mairu/bin/mairu vibe mutation "remember we use gRPC internally" -P my-project -k 5

# File/Knowledge Ingestion
./mairu/bin/mairu ingest README.md --base-uri "contextfs://readme" -P my-project -y

# Background context processing
./mairu/bin/mairu daemon ./src -P my-project

# Web Scraping
./mairu/bin/mairu scrape https://example.com --max-depth 2 -P my-project
```

## Notes

- The unified dashboard UI lives at `mairu/ui/`.
- Core ContextFS workflows (`memory`, `skill`, `node`, `vibe`, `vibe-query`, `vibe-mutation`, `ingest`, `daemon`) are native Go commands in `mairu` CLI.
- Go developer tooling is centralized in `mairu/scripts/go-dev.sh` and surfaced via root `make` targets.
