# Agent Context DB (Turso + Hybrid Retrieval)

Centralized context storage for coding agents with:

- vector embeddings on Turso/LibSQL
- hybrid retrieval (vector + keyword + optional recency/importance)
- a CLI for shell automation
- an MCP server for tool-enabled agent clients
- a simple dashboard
- a retrieval evaluation harness (`Recall@K`, `MRR`, latency)

## Features

- `memories`: user and agent facts with category, owner, importance
- `skills`: reusable capability descriptions
- `context nodes`: hierarchical context tree with recursive path/subtree queries
- configurable embedding model and dimension
- retrieval quality benchmarking via JSON datasets

## Requirements

- Node.js 18+
- Turso database URL and auth token
- Gemini API key (unless you intentionally use zero-vector fallback for local testing)

## Quickstart

```bash
npm install
npm --prefix dashboard install
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
npm run setup
```

Build:

```bash
npm run build
```

## Embedding Configuration

Embedding behavior is configured end-to-end:

- `EMBEDDING_MODEL`: model name passed to Gemini embeddings API.
- `EMBEDDING_DIM`: expected embedding dimension.
- `ALLOW_ZERO_EMBEDDINGS`: when `true`, permits zero-vector fallback if no `GEMINI_API_KEY` is set.

The runtime validates model/dimension consistency and vector size to prevent drift.

## CLI Usage

Use TypeScript source directly:

```bash
bun src/cli.ts --help
```

Or after build:

```bash
node dist/cli.js --help
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

## MCP Server

Run:

```bash
node dist/mcp.js
```

Claude Desktop config example:

```json
{
  "mcpServers": {
    "turso-context-db": {
      "command": "node",
      "args": ["/absolute/path/to/contextfs/dist/mcp.js"],
      "env": {
        "TURSO_URL": "libsql://...",
        "TURSO_AUTH_TOKEN": "...",
        "GEMINI_API_KEY": "..."
      }
    }
  }
}
```

## Retrieval Evaluation Harness

1. Create a dataset:

```bash
cp eval/dataset.example.json eval/dataset.json
```

2. Replace expected IDs/URIs in `eval/dataset.json` with real DB entities.

3. Run benchmark:

```bash
npm run eval:retrieval -- --dataset eval/dataset.json --topK 5 --verbose true
```

Outputs include:

- `avgRecallAtK`
- `mrr`
- `avgLatencyMs`
- optional `perCase` details with hit ranks and retrieved IDs

## Dashboard

Start API:

```bash
npm run dashboard:api
```

Start Svelte UI:

```bash
npm run dashboard:dev
```

Open [http://localhost:4173](http://localhost:4173).

## Package Scripts

- `npm run clean`: remove build output
- `npm run build`: compile TypeScript
- `npm run typecheck`: type-check without emit
- `npm run test`: run unit tests
- `npm run test:watch`: run tests in watch mode
- `npm run setup`: reset and initialize Turso schema (destructive)
- `npm run eval:retrieval -- --dataset ...`: run retrieval benchmark

## Contributing

See `CONTRIBUTING.md`.

## License

ISC (`LICENSE`).
