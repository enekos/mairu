# mairu

A dynamic context and memory storage system for coding agents. Provides native hybrid retrieval (vector + full-text + app-side re-ranking) backed by Meilisearch with Google Gemini embeddings. Exposes two interfaces: CLI and REST API (dashboard).

## Tech Stack

- **Runtime:** Go 1.25+
- **Database:** Meilisearch 1.12+ (Docker)
- **Search:** Native hybrid — vector cosine similarity + full-text + app-side re-ranking (importance, recency decay)
- **Embeddings:** Google Gemini (`gemini-embedding-001`, 3072 dims)
- **Frontend:** Svelte 5 + Vite
- **Testing:** Go testing (`go test`)
- **Linting:** Go vet (`go vet`)

## Setup

```bash
docker compose up -d    # start Meilisearch
bun install
bun --cwd mairu/dashboard install
cp .env.example .env    # fill in MEILI_URL, GEMINI_API_KEY
bun run setup           # create Meilisearch indexes (destructive — drops and recreates)
```

## Commands

| Command | Description |
|---|---|
| `docker compose up -d` | Start Meilisearch container |
| `docker compose down` | Stop Meilisearch container |
| `make build` | Compile Go binary to `mairu/bin/` |
| `make test` | Run Go tests |
| `make lint` | Run go vet |
| `make clean` | Remove `mairu/bin/` |
| `bun run setup` | Init/reset Meilisearch indexes |
| `bun run dashboard:api` | Start REST API on port 8787 |
| `bun run dashboard:dev` | Start Svelte dev server on port 5173 |
| `bun run dashboard:build` | Build Svelte UI |

### Evaluation

```bash
bun run eval:retrieval -- --dataset eval/dataset.json --topK 5 --verbose true
bun run eval:retrieval -- --dataset eval/dataset.json --topK 5 --fail-below-mrr 0.8 --fail-below-recall 0.75
```

## Architecture

### Data Types

- **Memories** — facts with category, owner, importance (1–10)
- **Skills** — capability name + description pairs
- **Context Nodes** — hierarchical tree nodes with abstract/overview/content levels, addressed by URI

### Retrieval Pipeline

Meilisearch handles vector + full-text search natively; app-side re-ranking applies recency decay and importance boosting:

1. **Vector search** — dense vector cosine similarity on Gemini embeddings
2. **Full-text** — Meilisearch built-in keyword search
3. **App-side re-ranking** — exponential recency decay + importance score boost
4. Results from both retrievers are merged and re-ranked before returning

Weights (vector, keyword, recency, importance) are defined in `scorer.ts`.

### Meilisearch Indexes

| Index | Key Fields |
|---|---|
| `contextfs_skills` | name (text), description (text), embedding (dense_vector), project (keyword) |
| `contextfs_memories` | content (text), category/owner (keyword), importance (integer), embedding (dense_vector) |
| `contextfs_context_nodes` | name/abstract/overview/content (text), uri/parent_uri (keyword), ancestors (keyword[]), embedding (dense_vector) |

### Search Features

| Feature | Description | Controlled by |
|---|---|---|
| **Vector search** | Dense cosine similarity on Gemini embeddings | `weights.vector` |
| **Full-text** | Meilisearch keyword search | `weights.keyword` |
| **Synonyms** | Custom synonym expansion (e.g., "k8s" → "kubernetes") | `SYNONYMS` env var |
| **Importance boost** | App-side boost on importance (1-10) | `weights.importance` |
| **Recency decay** | Exponential decay on created_at | `weights.recency`, `RECENCY_SCALE`, `RECENCY_DECAY` |
| **Min score cutoff** | Hard threshold to drop low-confidence results | `--minScore` |
| **Highlights** | Returns `<mark>`-tagged snippets showing matched terms | `--highlight` |
| **Field boosts** | Per-search field weight overrides | `fieldBoosts` option (API only) |

### Key Modules

| File | Role |
|---|---|
| `mairu/internal/db/meilisearchDB.go` | DB layer: CRUD, hybrid search (vector + full-text + re-ranking), tree queries |
| `mairu/internal/contextsrv/service.go` | High-level API used by CLI |
| `mairu/internal/llm/embedder.go` | Gemini embedding calls |
| `mairu/internal/contextsrv/scorer.go` | Hybrid weight definitions |
| `mairu/internal/llm/router.go` | LLM-powered deduplication (CREATE / UPDATE / SKIP) |
| `mairu/internal/llm/ingestor.go` | Free-form text → structured context nodes |
| `mairu/internal/contextsrv/vibe.go` | LLM-driven free-text query planning and mutation planning |
| `mairu/cmd/mairu/main.go` | CLI entry point |
| `mairu/internal/web/server.go` | REST API for dashboard |
| `mairu/internal/eval/evaluate.go` | Evaluation harness entry point |
| `mairu/internal/daemon/daemon.go` | File watcher daemon: parallel processing, persistent cache, NL content assembly |
| `mairu/internal/ast/language_describer.go` | Pluggable interface for language-specific AST extraction + shared types/utilities |
| `mairu/internal/ast/typescript_describer.go` | TypeScript/JS implementation of LanguageDescriber (tree-sitter based) |
| `mairu/internal/ast/nl_describer.go` | AST-to-English engine: converts function bodies to numbered NL descriptions |
| `mairu/internal/ast/nl_enricher.go` | Post-enrichment pass: injects cross-function context into NL descriptions |

### AST Ingestion (Daemon)

The daemon watches a directory for TS/JS file changes and produces human-readable natural language descriptions of code via pure AST heuristics (no LLM calls).

**Architecture:** Single-pass AST walker behind a pluggable `LanguageDescriber` interface extracts symbols + edges + NL descriptions. A post-enrichment pass stitches cross-function references.

**Content field layout for file context nodes:**

| Field | Content |
|---|---|
| `abstract` | NL file summary — concise description of exported symbols and file purpose |
| `overview` | Compact graph notation — machine-readable symbol/edge listing for programmatic use |
| `content` | Full NL AST — statement-level English descriptions of every function/method body |

**NL generation** uses AST pattern matching to translate code constructs to English:
- Conditions: `x === null` → "`x` is null", `!x` → "`x` is falsy", `typeof x === "string"` → "`x` is a string"
- Control flow: if/else, for/while loops, try/catch, switch, throw — all described in plain English
- Cross-references: call edges enriched with callee context (e.g., "calls `validate` (which checks if input is falsy)")

**Performance features:**
- **Parallel processing** — configurable concurrency pool (default 8) for initial scan and batch changes
- **Persistent hash cache** — `.contextfs-cache.json` persists fingerprint/content/payload hashes so daemon restarts skip unchanged files
- **Triple-layer dedup** — file stat fingerprint → content SHA1 → payload SHA1 prevents unnecessary re-indexing

**Pluggable interface** — `LanguageDescriber` is designed for future language support. Currently only TypeScript/JS (via tree-sitter). To add a new language, implement the interface with `languageId`, `extensions`, and `extractFileGraph()`.

### Hierarchical Context (Tree Queries)

Context nodes store a materialized `ancestors` array. Tree operations:
- **Subtree**: filter `ancestors = nodeUri` finds all descendants
- **Path**: get node's ancestors array, then fetch the full chain by URI list

### LLM Deduplication

Before writing, `llmRouter` does a vector-only search. If cosine similarity ≥ 0.75, an LLM decides whether to CREATE, UPDATE, or SKIP the new entry.

## Environment Variables

See `.env.example` for the full list. Required:

```
MEILI_URL=http://localhost:7700
GEMINI_API_KEY=
EMBEDDING_MODEL=gemini-embedding-001
EMBEDDING_DIM=3072
```

Optional:
```
MEILI_API_KEY=               # for secured Meilisearch instances
ALLOW_ZERO_EMBEDDINGS=false  # set true for local testing without Gemini
DASHBOARD_API_PORT=8787
```

### Search Fine-Tuning

These control query-level defaults (overridable per-search via API params):

```
SYNONYMS=auth,authentication,authn;db,database;k8s,kubernetes
RECENCY_SCALE=30d            # Recency half-life (e.g., 7d, 30d, 90d)
RECENCY_DECAY=0.5            # Decay factor at scale distance
```

# Agent Integration Instructions

To integrate OpenContextFS into Claude or Opencode using the CLI, refer to this section. You must use the terminal (`bash` tool) to invoke `mairu-context` (or compatibility alias `context-cli`).

**IMPORTANT**: Always use the `-P, --project <project>` flag when managing or searching memories/context so that information is correctly isolated by project.

### 1. Deterministic Retrieval (Recommended Default)
When starting a new session or debugging an issue, you MUST search memories and context nodes for existing constraints or decisions.
Prefer direct retrieval commands first so the agent can control scope and ranking behavior explicitly:

```bash
mairu-context memory search "authentication token validation rules" -k 5 -P my-project
mairu-context node search "authentication architecture" -k 5 -P my-project
mairu-context node ls "contextfs://my-project/backend/auth" -P my-project
```

### 2. Natural Language Storage (Recommended)
When you successfully complete a complex task, summarize the structural decisions and save them. `vibe-mutation` interprets your instructions and automatically updates/creates memories and nodes.
**Note:** Always pass `-y` to auto-approve mutations.

```bash
mairu-context vibe-mutation "remember that we switched from REST to gRPC for internal service calls" -P my-project -y
```

### 3. Natural-Language Retrieval (Optional Fallback)
Use `vibe-query` only when direct memory/node searches are not sufficient (for example, broad or ambiguous questions that need multi-step planning).

```bash
mairu-context vibe-query "how does the authentication system work?" -P my-project
```

### 4. Advanced/Precise Operations
Use direct commands when you need exact control over what is stored or retrieved.

**Memory Store:**
```bash
mairu-context memory store "In project X, we use Vitest instead of Jest for unit testing." -c observation -o agent -i 5 -P my-project
```

**Memory Search:**
```bash
mairu-context memory search "testing framework" -k 5 -P my-project

# With highlights
mairu-context memory search "authentication setup" -k 5 -P my-project --highlight
```

**Managing Context Nodes (Hierarchical Knowledge):**
```bash
mairu-context node store "contextfs://my-project/backend/auth" "Auth Module" "Uses JWT with RSA signatures." -P my-project
mairu-context node ls "contextfs://my-project/backend" -P my-project
```

Agents should proactively search memories and context nodes when beginning a task, and store important discoveries or user preferences as they work.
