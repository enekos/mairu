# contextfs -- dynamic context builder

Centralized context and memory storage for coding agents with:

- native hybrid retrieval (vector + full-text + app-side re-ranking) backed by Meilisearch
- Google Gemini embeddings (3072 dims)
- a CLI for shell automation
- a REST API and Svelte dashboard
- a retrieval evaluation harness (`Recall@K`, `MRR`, latency)

## Features

- `memories`: user and agent facts with category, owner, importance
- `skills`: reusable capability descriptions
- `context nodes`: hierarchical context tree with recursive path/subtree queries
- hybrid search: dense vector cosine similarity + full-text + recency decay + importance boost
- ngram partial matching, synonym expansion
- configurable embedding model and dimension
- retrieval quality benchmarking via JSON datasets

## Requirements

- Bun 1+
- Docker (for Meilisearch)
- Gemini API key (unless using zero-vector fallback for local testing)

## Quickstart

```bash
docker compose up -d        # start Meilisearch
bun install
bun --cwd dashboard install
cp .env.example .env        # fill in GEMINI_API_KEY
bun run setup               # create Meilisearch indexes (destructive)
```

Minimal `.env`:

```env
MEILI_URL=http://localhost:7700
MEILI_API_KEY=contextfs-dev-key
GEMINI_API_KEY=your_gemini_api_key
EMBEDDING_MODEL=gemini-embedding-001
EMBEDDING_DIM=3072
ALLOW_ZERO_EMBEDDINGS=false
```

Build and link CLI globally:

```bash
bun run build
bun run link
```

## Search Fine-Tuning

Query-level defaults (overridable per-search via API params):

```env
RECENCY_SCALE=30d           # recency half-life (e.g. 7d, 30d, 90d)
RECENCY_DECAY=0.5           # decay factor at scale distance
SYNONYMS=auth,authentication,authn;db,database;k8s,kubernetes
```

## CLI Usage

```bash
context-cli --help
```

Examples:

```bash
# memories
context-cli memory store "User prefers strict TypeScript and hexagonal architecture" -c preferences -o user -i 8 -P my-project
context-cli memory search "coding preferences" -k 5 -P my-project
context-cli memory search "auth setup" -k 5 -P my-project --highlight

# skills
context-cli skill add "Postgres Expert" "Optimizes large SQL queries and indexes" -P my-project
context-cli skill search "postgres optimization" -k 5 -P my-project

# context nodes
context-cli node store "contextfs://project/backend" "Backend" "Core backend architecture" -P my-project
context-cli node search "auth architecture" -k 5 -P my-project
context-cli node restore "contextfs://project/backend" # Restore a soft-deleted node

# optional free-text query planner (fallback when direct searches are insufficient)
context-cli vibe-query "how does authentication work?" -P my-project -k 5

# free-text mutation (LLM plans + interactive approval)
context-cli vibe-mutation "remember we switched to gRPC for internal calls" -P my-project

# start a background AST ingestion daemon on the current directory
context-cli daemon . -P my-project
```

## Features Deep Dive

### Automated AST Codebase Ingestion (Daemon)
`contextfs` ships with a background daemon powered by `ts-morph` and `chokidar`. It observes a local codebase directory and automatically extracts class signatures, function definitions, and module exports, as well as a human readable version of AST in natural language. This ensures your hierarchical context tree is always synchronized with the real code state without manual ingestion. It skips `node_modules` and build directories automatically.

### Context Versioning & Rollback
To protect against hallucinated updates from AI agents (via `vibe-mutation` or direct node updates), `context nodes` feature soft-deletes and version history.
- Soft Deletes: Calling `context-cli node delete` soft-deletes the node and all its descendants. They will not appear in search results. You can easily recover them via `context-cli node restore`.
- Versioning: Every update operation archives the previous state (`name`, `abstract`, `overview`, `content`) into a `version_history` array (up to 10 versions), directly inside the document.

## Retrieval Evaluation Harness

1. Create a dataset:

```bash
cp eval/dataset.example.json eval/dataset.json
```

2. Replace expected IDs/URIs in `eval/dataset.json` with real DB entities.

3. Run benchmark:

```bash
bun run eval:retrieval -- --dataset eval/dataset.json --topK 5 --verbose true
# with pass/fail thresholds:
bun run eval:retrieval -- --dataset eval/dataset.json --topK 5 --fail-below-mrr 0.8 --fail-below-recall 0.75
```

Outputs: `avgRecallAtK`, `mrr`, `avgLatencyMs`, optional `perCase` details.

## Mairu Agent

`contextfs` includes **Mairu**, a blazing fast, graph-powered AI coding agent built in pure Golang. It integrates tightly with the `contextfs` Meilisearch backend for dense, typo-tolerant codebase retrieval.

### Key Capabilities

- **Live AST Graph & Surgical Reads:** Uses the `contextfs` graph to perform surgical reads on specific functions or classes rather than dumping entire files into context, saving massive amounts of tokens.
- **Multi-Agent Dispatch:** The main agent can delegate tasks to sub-agents to research the codebase simultaneously.
- **Terminal Native UI:** Rich Markdown rendering, real-time typing, and session memory powered by Bubbletea.
- **Web UI:** Includes a web dashboard for interactive chats and codebase overview.

### Usage

Mairu requires Go 1.21+.

```bash
cd mairu
go build -o mairu-agent cmd/mairu/main.go

# Index your project
./mairu-agent index

# Start the interactive terminal UI
./mairu-agent tui

# Or start the web UI
./mairu-agent web -p 8080
```

## Dashboard

```bash
bun run dashboard:api   # REST API on port 8787
bun run dashboard:dev   # Svelte dev server on port 5173
```

## Package Scripts

| Command | Description |
|---|---|
| `bun run build` | Compile TypeScript → `dist/` |
| `bun run typecheck` | Type-check without emit |
| `bun run lint` | Run oxlint on `src/` |
| `bun run test` | Run Vitest tests once |
| `bun run test:watch` | Vitest in watch mode |
| `bun run clean` | Remove `dist/` |
| `bun run setup` | Init/reset Meilisearch indexes (destructive) |
| `bun run link` | Build and install `context-cli` globally |
| `bun run eval:ablation` | Compare vector-only/keyword-only/hybrid retrieval |

## License

ISC (`LICENSE`).
