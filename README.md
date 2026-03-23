# contextfs -- dynamic context builder

Centralized context and memory storage for coding agents with:

- native hybrid retrieval (kNN + BM25 + function scoring) backed by Elasticsearch
- Google Gemini embeddings (3072 dims)
- a CLI for shell automation
- a REST API and Svelte dashboard
- a retrieval evaluation harness (`Recall@K`, `MRR`, latency)

## Features

- `memories`: user and agent facts with category, owner, importance
- `skills`: reusable capability descriptions
- `context nodes`: hierarchical context tree with recursive path/subtree queries
- hybrid search: dense vector cosine similarity + BM25 full-text + recency decay + importance boost
- fuzzy matching, exact phrase boost, ngram partial matching, synonym expansion
- configurable embedding model and dimension
- retrieval quality benchmarking via JSON datasets

## Requirements

- Bun 1+
- Docker (for Elasticsearch)
- Gemini API key (unless using zero-vector fallback for local testing)

## Quickstart

```bash
docker compose up -d        # start Elasticsearch
bun install
bun --cwd dashboard install
cp .env.example .env        # fill in GEMINI_API_KEY
bun run setup               # create ES indices (destructive)
```

Minimal `.env`:

```env
ELASTIC_URL=http://localhost:9200
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

## Elasticsearch Fine-Tuning

Index-level settings (require `bun run setup` after changing):

```env
ES_BM25_K1=1.2              # term frequency saturation
ES_BM25_B=0.75              # document length normalization (0 = none, 1 = full)
ES_SYNONYMS=auth,authentication,authn;db,database;k8s,kubernetes
```

Query-level defaults (overridable per-search via CLI flags or API params):

```env
ES_DEFAULT_FUZZINESS=auto   # typo tolerance: auto, 0, 1, 2
ES_PHRASE_BOOST=2.0         # boost for exact phrase matches (0 = disabled)
ES_RECENCY_SCALE=30d        # recency half-life (e.g. 7d, 30d, 90d)
ES_RECENCY_DECAY=0.5        # decay factor at scale distance
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
context-cli memory search "auth setup" -k 5 -P my-project --fuzziness auto --phraseBoost 3 --highlight

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
`contextfs` ships with a background daemon powered by `ts-morph` and `chokidar`. It observes a local codebase directory and automatically extracts class signatures, function definitions, and module exports. This ensures your hierarchical context tree is always synchronized with the real code state without manual ingestion. It skips `node_modules` and build directories automatically.

### Context Versioning & Rollback
To protect against hallucinated updates from AI agents (via `vibe-mutation` or direct node updates), `context nodes` feature soft-deletes and version history.
- Soft Deletes: Calling `context-cli node delete` soft-deletes the node and all its descendants. They will not appear in search results. You can easily recover them via `context-cli node restore`.
- Versioning: Every update operation archives the previous state (`name`, `abstract`, `overview`, `content`) into a `version_history` array (up to 10 versions), directly inside the Elasticsearch document.

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
| `bun run setup` | Init/reset Elasticsearch indices (destructive) |
| `bun run link` | Build and install `context-cli` globally |

## License

ISC (`LICENSE`).
