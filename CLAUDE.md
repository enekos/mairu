# contextfs

A dynamic context and memory storage system for coding agents. Provides native hybrid retrieval (kNN + BM25 + function scoring) backed by Elasticsearch with Google Gemini embeddings. Exposes two interfaces: CLI and REST API (dashboard).

## Tech Stack

- **Runtime:** Bun 1+, TypeScript (ES2022, CommonJS)
- **Database:** Elasticsearch 8.17+ (Docker)
- **Search:** Native hybrid — kNN dense vector + BM25 full-text + function scoring (importance, recency decay)
- **Embeddings:** Google Gemini (`gemini-embedding-001`, 3072 dims)
- **Frontend:** Svelte 5 + Vite
- **Testing:** Vitest
- **Linting:** oxlint

## Setup

```bash
docker compose up -d    # start Elasticsearch
bun install
bun --cwd dashboard install
cp .env.example .env    # fill in ELASTIC_URL, GEMINI_API_KEY
bun run setup           # create ES indices (destructive — drops and recreates)
```

## Commands

| Command | Description |
|---|---|
| `docker compose up -d` | Start Elasticsearch container |
| `docker compose down` | Stop Elasticsearch container |
| `bun run link` | Build and install `context-cli` globally via `bun link` |
| `bun run build` | Compile TypeScript → `dist/` |
| `bun run typecheck` | Type-check without emit |
| `bun run lint` | Run oxlint on `src/` |
| `bun run test` | Run Vitest tests once |
| `bun run test:watch` | Vitest in watch mode |
| `bun run clean` | Remove `dist/` |
| `bun run setup` | Init/reset Elasticsearch indices |
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

Elasticsearch handles all scoring natively in a single query:

1. **kNN** — dense vector cosine similarity on Gemini embeddings
2. **BM25** — full-text search with English stemming/stopwords via custom analyzer
3. **Function scoring** — exponential recency decay + field-value importance boost
4. ES merges kNN and text results, summing scores for documents found by both retrievers

Weights (vector, keyword, recency, importance) map directly to ES `boost` parameters — defined in `scorer.ts`.

### Elasticsearch Indices

| Index | Key Fields |
|---|---|
| `contextfs_skills` | name (text), description (text), embedding (dense_vector), project (keyword) |
| `contextfs_memories` | content (text), category/owner (keyword), importance (integer), embedding (dense_vector) |
| `contextfs_context_nodes` | name/abstract/overview/content (text), uri/parent_uri (keyword), ancestors (keyword[]), embedding (dense_vector) |

All text fields use a custom `content_analyzer` (English stemming + stopword removal + optional synonyms). Key text fields also have an `ngram` sub-field for partial/substring matching.

### Search Features

| Feature | Description | Controlled by |
|---|---|---|
| **kNN vector search** | Dense cosine similarity on Gemini embeddings | `weights.vector` |
| **BM25 full-text** | Stemmed English text matching with IDF weighting | `weights.keyword`, `ES_BM25_K1`, `ES_BM25_B` |
| **Fuzzy matching** | Typo tolerance (Levenshtein distance) | `--fuzziness` / `ES_DEFAULT_FUZZINESS` |
| **Phrase boost** | Bonus for exact phrase ordering | `--phraseBoost` / `ES_PHRASE_BOOST` |
| **Ngram partial match** | Substring matching (e.g., "auth" finds "authentication") | Always active on name/content fields |
| **Synonyms** | Custom synonym expansion (e.g., "k8s" → "kubernetes") | `ES_SYNONYMS` env var |
| **Importance boost** | Field-value factor on importance (1-10) | `weights.importance` |
| **Recency decay** | Exponential decay on created_at | `weights.recency`, `ES_RECENCY_SCALE`, `ES_RECENCY_DECAY` |
| **Min score cutoff** | Hard threshold to drop low-confidence results | `--minScore` |
| **Highlights** | Returns `<mark>`-tagged snippets showing matched terms | `--highlight` |
| **Field boosts** | Per-search field weight overrides | `fieldBoosts` option (API only) |

### Key Modules

| File | Role |
|---|---|
| `src/elasticDB.ts` | DB layer: CRUD, hybrid search (kNN + BM25 + function_score), tree queries |
| `src/contextManager.ts` | High-level API used by CLI |
| `src/embedder.ts` | Gemini embedding calls |
| `src/scorer.ts` | Hybrid weight definitions (mapped to ES boosts) |
| `src/llmRouter.ts` | LLM-powered deduplication (CREATE / UPDATE / SKIP) |
| `src/ingestor.ts` | Free-form text → structured context nodes |
| `src/vibeEngine.ts` | LLM-driven free-text query planning and mutation planning |
| `src/cli.ts` | CLI entry point |
| `src/dashboardApi.ts` | REST API for dashboard |
| `src/evaluate.ts` | Evaluation harness entry point |
| `src/daemon.ts` | File watcher daemon: parallel processing, persistent cache, NL content assembly |
| `src/languageDescriber.ts` | Pluggable interface for language-specific AST extraction + shared types/utilities |
| `src/typescriptDescriber.ts` | TypeScript/JS implementation of LanguageDescriber (ts-morph based) |
| `src/nlDescriber.ts` | AST-to-English engine: converts function bodies to numbered NL descriptions |
| `src/nlEnricher.ts` | Post-enrichment pass: injects cross-function context into NL descriptions |

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

**Pluggable interface** — `LanguageDescriber` is designed for future language support. Currently only TypeScript/JS (via ts-morph). To add a new language, implement the interface with `languageId`, `extensions`, and `extractFileGraph()`.

### Hierarchical Context (Tree Queries)

Context nodes store a materialized `ancestors` array. Tree operations:
- **Subtree**: single ES query — `term: { ancestors: nodeUri }` finds all descendants
- **Path**: get node's ancestors array, then `terms: { uri: [...ancestors] }` fetches the full chain

### LLM Deduplication

Before writing, `llmRouter` does a vector-only kNN search. If cosine similarity ≥ 0.75, an LLM decides whether to CREATE, UPDATE, or SKIP the new entry.

## Environment Variables

See `.env.example` for the full list. Required:

```
ELASTIC_URL=http://localhost:9200
GEMINI_API_KEY=
EMBEDDING_MODEL=gemini-embedding-001
EMBEDDING_DIM=3072
```

Optional:
```
ELASTIC_USERNAME=            # for secured clusters
ELASTIC_PASSWORD=
ALLOW_ZERO_EMBEDDINGS=false  # set true for local testing without Gemini
DASHBOARD_API_PORT=8787
```

### Elasticsearch Fine-Tuning

These control index-level settings (**require `bun run setup` after changing**):

```
ES_BM25_K1=1.2              # Term frequency saturation (higher = more weight to repeated terms)
ES_BM25_B=0.75              # Document length normalization (0 = none, 1 = full)
ES_SYNONYMS=auth,authentication,authn;db,database;k8s,kubernetes
```

These control query-level defaults (overridable per-search via CLI flags or API params):

```
ES_DEFAULT_FUZZINESS=auto    # Typo tolerance: auto, 0, 1, 2
ES_PHRASE_BOOST=2.0          # Boost for exact phrase matches (0 = disabled)
ES_RECENCY_SCALE=30d         # Recency half-life (e.g., 7d, 30d, 90d)
ES_RECENCY_DECAY=0.5         # Decay factor at scale distance
```

# Agent Integration Instructions

To integrate OpenContextFS into Claude or Opencode using the CLI, refer to this section. You must use the terminal (`bash` tool) to invoke `context-cli`.

**IMPORTANT**: Always use the `-P, --project <project>` flag when managing or searching memories/context so that information is correctly isolated by project.

### 1. Deterministic Retrieval (Recommended Default)
When starting a new session or debugging an issue, you MUST search memories and context nodes for existing constraints or decisions.
Prefer direct retrieval commands first so the agent can control scope and ranking behavior explicitly:

```bash
context-cli memory search "authentication token validation rules" -k 5 -P my-project
context-cli node search "authentication architecture" -k 5 -P my-project
context-cli node ls "contextfs://my-project/backend/auth" -P my-project
```

### 2. Natural Language Storage (Recommended)
When you successfully complete a complex task, summarize the structural decisions and save them. `vibe-mutation` interprets your instructions and automatically updates/creates memories and nodes.
**Note:** Always pass `-y` to auto-approve mutations.

```bash
context-cli vibe-mutation "remember that we switched from REST to gRPC for internal service calls" -P my-project -y
```

### 3. Natural-Language Retrieval (Optional Fallback)
Use `vibe-query` only when direct memory/node searches are not sufficient (for example, broad or ambiguous questions that need multi-step planning).

```bash
context-cli vibe-query "how does the authentication system work?" -P my-project
```

### 4. Advanced/Precise Operations
Use direct commands when you need exact control over what is stored or retrieved.

**Memory Store:**
```bash
context-cli memory store "In project X, we use Vitest instead of Jest for unit testing." -c observation -o agent -i 5 -P my-project
```

**Memory Search:**
```bash
context-cli memory search "testing framework" -k 5 -P my-project

# With fine-tuning: fuzzy matching + exact phrase boost + highlights
context-cli memory search "authentcation setup" -k 5 -P my-project --fuzziness auto --phraseBoost 3 --highlight
```

**Managing Context Nodes (Hierarchical Knowledge):**
```bash
context-cli node store "contextfs://my-project/backend/auth" "Auth Module" "Uses JWT with RSA signatures." -P my-project
context-cli node ls "contextfs://my-project/backend" -P my-project
```

Agents should proactively search memories and context nodes when beginning a task, and store important discoveries or user preferences as they work.
