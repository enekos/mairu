# contextfs

A dynamic context and memory storage system for coding agents. Provides hybrid vector + keyword retrieval backed by Turso/LibSQL with Google Gemini embeddings. Exposes two interfaces: CLI and REST API (dashboard).

## Tech Stack

- **Runtime:** Bun 1+, TypeScript (ES2022, CommonJS)
- **Database:** Turso/LibSQL with vector index support
- **Embeddings:** Google Gemini (`gemini-embedding-001`, 768 dims)
- **Frontend:** Svelte 5 + Vite
- **Testing:** Vitest
- **Linting:** oxlint

## Setup

```bash
bun install
bun --cwd dashboard install
cp .env.example .env   # fill in TURSO_URL, TURSO_AUTH_TOKEN, GEMINI_API_KEY
bun run setup          # initialize DB schema (destructive — drops and recreates tables)
```

## Commands

| Command | Description |
|---|---|
| `bun run link` | Build and install `context-cli` globally via `bun link` |
| `bun run build` | Compile TypeScript → `dist/` |
| `bun run typecheck` | Type-check without emit |
| `bun run lint` | Run oxlint on `src/` |
| `bun run test` | Run Vitest tests once |
| `bun run test:watch` | Vitest in watch mode |
| `bun run clean` | Remove `dist/` |
| `bun run setup` | Init/reset Turso schema |
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

1. Vector search fetches broad candidates (`CANDIDATE_MULTIPLIER=4`)
2. Application-side re-ranking via `scorer.ts`: hybrid score = vector distance + keyword overlap + recency + importance

### Key Modules

| File | Role |
|---|---|
| `src/TursoVectorDB.ts` | DB layer: CRUD, vector indexing, raw search |
| `src/ContextManager.ts` | High-level API used by CLI |
| `src/embedder.ts` | Gemini embedding calls |
| `src/scorer.ts` | Hybrid re-ranking logic |
| `src/llmRouter.ts` | LLM-powered deduplication (CREATE / UPDATE / SKIP) |
| `src/ingestor.ts` | Free-form text → structured context nodes |
| `src/cli.ts` | CLI entry point |
| `src/dashboardApi.ts` | REST API for dashboard |
| `src/evaluate.ts` | Evaluation harness entry point |

### LLM Deduplication

Before writing, `llmRouter` searches for similar entries. If similarity ≥ 0.75, an LLM decides whether to CREATE, UPDATE, or SKIP the new entry.

## Environment Variables

See `.env.example` for the full list. Required:

```
TURSO_URL=
TURSO_AUTH_TOKEN=
GEMINI_API_KEY=
EMBEDDING_MODEL=gemini-embedding-001
EMBEDDING_DIM=768
```

Optional:
```
ALLOW_ZERO_EMBEDDINGS=false   # set true for local testing without Gemini
DASHBOARD_API_PORT=8787
```

# Agent Integration Instructions

To integrate OpenContextFS into Claude or Opencode using the CLI, refer to this section. You must use the terminal (`bash` tool) to invoke `context-cli`.

**IMPORTANT**: Always use the `-P, --project <project>` flag when managing or searching memories/context so that information is correctly isolated by project.

### 1. Saving context/memory
Whenever you learn something new, solve a complex bug, or want to remember a project convention:
```bash
context-cli memory store "In project X, we use Vitest instead of Jest for unit testing." -c observation -o agent -i 5 -P my-project
```

### 2. Searching memory
If you are starting a new session or need to recall constraints or architecture decisions for the current task:
```bash
context-cli memory search "testing framework" -k 5 -P my-project
```

### 3. Managing Context Nodes (Hierarchical Knowledge)
For broader documentation or code architecture, you can store and read nodes:
```bash
context-cli node store "contextfs://my-project/backend/auth" "Auth Module" "Uses JWT with RSA signatures." -P my-project
context-cli node ls "contextfs://my-project/backend" -P my-project
```

Agents should proactively search memories when beginning a task and store important discoveries or user preferences as they work.
