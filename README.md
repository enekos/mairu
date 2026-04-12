# mairu

![mairu logo](mairu.png)

Unified monorepo for:

- the **Mairu coding agent** (Go, CLI, TUI + web)
- the **central context server** (Go)
- the **unified web UI dashboard** (Svelte)

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for contributor workflow and quality gates.

## Repository Layout

```text
.
├── mairu/
│   ├── cmd/                 # Go entrypoint (mairu binary)
│   ├── internal/            # Go agent + context-server internals
│   ├── scripts/             # Local Meilisearch helper script
│   └── ui/                  # Unified Mairu dashboard UI (Svelte) & Go app frontend
├── docs/                    # Specs and project docs
├── package.json             # UI-only Bun scripts
└── Makefile                 # Go + monorepo dev workflows
```

## Requirements

- Bun 1+ (for dashboard UI)
- Go 1.25+
- Docker (optional if using local Meilisearch fallback)
- Gemini API key (unless `ALLOW_ZERO_EMBEDDINGS=true` for local-only testing)

## Quickstart

The easiest way to set up Mairu locally (without Docker) is using the bootstrap script:

```bash
./bootstrap.sh
```

Then initialize the configuration and set your API key:

```bash
make mairu-build
./mairu/bin/mairu setup
./mairu/bin/mairu init --defaults
```

Start the services:

```bash
make dashboard        # Context server + unified web dashboard
# or
make mairu-web        # Mairu agent web UI
```

## Configuration

Mairu uses a five-tier TOML configuration cascade:
1. Hardcoded defaults
2. User config (`~/.config/mairu/config.toml`)
3. Project config (`.mairu.toml` in your project root or `.git` parent)
4. Environment variables (`MAIRU_` prefix)
5. CLI flags

Manage your config using the CLI:
```bash
./mairu/bin/mairu config list
./mairu/bin/mairu config set api.gemini_api_key "your-key"
./mairu/bin/mairu init            # interactive project setup
./mairu/bin/mairu doctor          # check system health
```

### Sample `config.toml`

Here is an example of what your `~/.config/mairu/config.toml` or `.mairu.toml` might look like:

```toml
[api]
gemini_api_key = "AIzaSyYourKeyHere..."
meili_url = "http://localhost:7700"
meili_api_key = "contextfs-dev-key"

[daemon]
concurrency = 8
max_file_size = "512KB"
debounce = "200ms"
max_content_chars = 16000

[server]
port = 8788
sqlite_dsn = "file:mairu.db?cache=shared&mode=rwc"

[embedding]
model = "gemini-embedding-001"
dimensions = 3072

[output]
format = "table"
color = true
```

## Core Commands

| Command | Description |
|---|---|
| `make mairu-build` | Build Go `mairu` binary |
| `make test-go` | Run Go test suite |
| `make lint-go` | Run Go lint (`golangci-lint` or fallback `go vet`) |
| `make check-go` | Run Go fmt check + lint + tests |
| `make check-go-ci` | Run CI-grade Go checks (fmt + lint + race) |
| `make install-hooks` | Install local pre-commit hook (`make check-go`) |
| `make setup` | Initialize/reset Meilisearch indexes (destructive) |
| `make dashboard` | Run context server + unified Mairu dashboard UI |
| `make mairu-web` | Launch Mairu web UI |
| `bun run dashboard:dev` | Run UI-only dev server |
| `bun run dashboard:build` | Build UI-only frontend bundle |

### Go Dev Tooling

For stricter linting, install `golangci-lint` once:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

If `golangci-lint` is not installed, the tooling falls back to `go vet`.

Optional pre-commit hook:

```bash
make install-hooks
```

## Go CLI Commands (Mairu Agent)

With the latest features, the Go CLI implements the ContextFS API fully via the `mairu` binary.
Output defaults to `table`, but you can use `-o json` or `-o plain` for scripting.

```bash
make mairu-build

# Configuration & Health
./mairu/bin/mairu config list
./mairu/bin/mairu doctor

# Context Server APIs
./mairu/bin/mairu memory search "auth token" -P my-project -k 5
./mairu/bin/mairu node search "authentication architecture" -P my-project -k 5

# Vibe commands (LLM powered mutations and queries)
./mairu/bin/mairu vibe query "how does auth work?" -P my-project
./mairu/bin/mairu vibe mutation "remember we use gRPC internally" -P my-project

# Advanced Tools (Daemon, Ingest, Scraper & History)
./mairu/bin/mairu daemon ./src -P my-project                                        # Scan directory and extract AST to context nodes
./mairu/bin/mairu ingest design.md --base-uri "contextfs://design" -P my-project -y # Parse markdown via LLM and persist
./mairu/bin/mairu scrape web https://example.com -P my-project                       # Scrape one URL into context
./mairu/bin/mairu scrape depth https://example.com -d 2 -P my-project                # Crawl and summarize web content
./mairu/bin/mairu history search "test fail"                                        # Semantically search bash command history

# Full TUI or Web Servers
./mairu/bin/mairu tui
./mairu/bin/mairu web -p 8080
./mairu/bin/mairu context-server -p 8788
```

## Environment Variables (Legacy Support)

Mairu supports older environment variables, but `.mairu.toml` or `~/.config/mairu/config.toml` is preferred.
See `mairu config list` for the complete list of settings.

```env
MEILI_URL=http://localhost:7700
MEILI_API_KEY=contextfs-dev-key
GEMINI_API_KEY=your_gemini_api_key
EMBEDDING_MODEL=gemini-embedding-001
EMBEDDING_DIM=3072
```

## License

ISC (`LICENSE`)