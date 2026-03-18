# contextfs -- dynamic context builder

Centralized context and memory storage for coding agents with:

- vector embeddings on Turso/LibSQL
- hybrid retrieval (vector + keyword + optional recency/importance)
- a CLI for shell automation
- a simple dashboard
- a retrieval evaluation harness (`Recall@K`, `MRR`, latency)

## Features

- `memories`: user and agent facts with category, owner, importance
- `skills`: reusable capability descriptions
- `context nodes`: hierarchical context tree with recursive path/subtree queries
- configurable embedding model and dimension
- retrieval quality benchmarking via JSON datasets

## Requirements

- Bun 1+
- Turso database URL and auth token
- Gemini API key (unless you intentionally use zero-vector fallback for local testing)

## Quickstart

```bash
bun install
bun --cwd dashboard install
cp .env.example .env
```

Update `.env`:

```env
TURSO_URL=libsql://your-db-url.turso.io
TURSO_AUTH_TOKEN=your_turso_auth_token
GEMINI_API_KEY=your_gemini_api_key
EMBEDDING_MODEL=gemini-embedding-001
EMBEDDING_DIM=768
ALLOW_ZERO_EMBEDDINGS=false
```

Initialize schema (destructive reset):

```bash
bun run setup
```

Build:

```bash
bun run build
```

## Embedding Configuration

Embedding behavior is configured end-to-end:

- `EMBEDDING_MODEL`: model name passed to Gemini embeddings API.
- `EMBEDDING_DIM`: expected embedding dimension.
- `ALLOW_ZERO_EMBEDDINGS`: when `true`, permits zero-vector fallback if no `GEMINI_API_KEY` is set.

The runtime validates model/dimension consistency and vector size to prevent drift.

## CLI Usage

Run TypeScript source directly:

```bash
bun src/cli.ts --help
```

Or after build:

```bash
bun dist/cli.js --help
```

Examples:

```bash
# memories
bun src/cli.ts memory add "User prefers strict TypeScript and hexagonal architecture" --category preferences --owner user --importance 8
bun src/cli.ts memory search "coding preferences" --topK 5 --owner user --category preferences --minImportance 5

# skills
bun src/cli.ts skill add "Postgres Expert" "Optimizes large SQL queries and indexes"
bun src/cli.ts skill search "postgres optimization" --topK 5

# context nodes
bun src/cli.ts node add "contextfs://project/backend" "Backend" "Core backend architecture"
bun src/cli.ts node search "auth architecture" --topK 5
```

## Retrieval Evaluation Harness

1. Create a dataset:

```bash
cp eval/dataset.example.json eval/dataset.json
```

2. Replace expected IDs/URIs in `eval/dataset.json` with real DB entities.

3. Run benchmark:

```bash
bun run eval:retrieval -- --dataset eval/dataset.json --topK 5 --verbose true
```

Outputs include:

- `avgRecallAtK`
- `mrr`
- `avgLatencyMs`
- optional `perCase` details with hit ranks and retrieved IDs

## Dashboard

Start API:

```bash
bun run dashboard:api
```

Start Svelte UI:

```bash
bun run dashboard:dev
```

Open [http://localhost:4173](http://localhost:4173).

## Package Scripts

- `bun run clean`: remove build output
- `bun run build`: compile TypeScript
- `bun run typecheck`: type-check without emit
- `bun run test`: run unit tests
- `bun run test:watch`: run tests in watch mode
- `bun run setup`: reset and initialize Turso schema (destructive)
- `bun run eval:retrieval -- --dataset ...`: run retrieval benchmark

## Contributing

See `CONTRIBUTING.md`.

## License

ISC (`LICENSE`).
