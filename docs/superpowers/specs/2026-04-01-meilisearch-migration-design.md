# Meilisearch Migration Design

**Date:** 2026-04-01
**Goal:** Replace Elasticsearch with Meilisearch as the sole storage/search backend to reduce resource footprint (~2GB RAM down to ~50-100MB).

## Scope

Full replacement. Remove `@elastic/elasticsearch` dependency, `ElasticDB` class, and all ES-specific configuration. Introduce `MeilisearchDB` with the same public API surface.

## Architecture

### Files Changed

| File | Action |
|---|---|
| `src/storage/meilisearchDB.ts` | **New** — replaces `elasticDB.ts`, same method signatures |
| `src/storage/elasticDB.ts` | **Delete** |
| `src/storage/contextManager.ts` | Update imports: `ElasticDB` -> `MeilisearchDB` |
| `src/storage/batchWriter.ts` | Update imports: `ElasticDB` -> `MeilisearchDB` |
| `src/storage/scorer.ts` | Keep as-is (weights used in app-side re-ranking) |
| `src/core/config.ts` | Replace ES config with Meilisearch config |
| `src/core/types.ts` | Rename `ElasticSearchTuning` -> `SearchTuning` |
| `src/scripts/setup.ts` | Rewrite for Meilisearch index creation |
| `src/index.ts` | Update re-exports |
| `src/eval/evaluate.ts` | Update imports |
| `src/eval/evalSeeder.ts` | Update imports |
| `docker-compose.yml` | Swap ES container for Meilisearch |
| `package.json` | Remove `@elastic/elasticsearch`, add `meilisearch` |
| `.env.example` | Replace ES env vars with Meilisearch ones |
| `tests/batchWriter.test.ts` | Update imports and constructor |
| `tests/budget.test.ts` | Update imports and constructor |
| `tools/clear-tables.ts` | Update imports |
| `tools/drop-tables.ts` | Update imports |
| `CLAUDE.md` | Update documentation references |

### Indexes

Same three indexes, same names:

| Index | Primary Key |
|---|---|
| `contextfs_skills` | `id` |
| `contextfs_memories` | `id` |
| `contextfs_context_nodes` | `uri` |

### Index Configuration (per index at setup time)

- **Searchable attributes** — text fields that Meilisearch indexes for full-text search
- **Filterable attributes** — keyword/numeric fields used in filter expressions (project, category, owner, importance, is_deleted, ancestors, parent_uri, created_at, updated_at)
- **Sortable attributes** — fields used for sorting (updated_at, created_at, importance)
- **Synonyms** — pushed via Meilisearch synonyms API from `SYNONYMS` env var
- **Embedders** — configured as "userProvided" with dimension from `EMBEDDING_DIM` config

#### Skills Index

- Searchable: `name`, `description`
- Filterable: `project`, `ai_intent`, `ai_topics`, `created_at`, `updated_at`
- Sortable: `updated_at`, `created_at`

#### Memories Index

- Searchable: `content`
- Filterable: `project`, `category`, `owner`, `importance`, `ai_intent`, `ai_topics`, `created_at`, `updated_at`
- Sortable: `updated_at`, `created_at`, `importance`

#### Context Nodes Index

- Searchable: `name`, `abstract`, `overview`, `content`
- Filterable: `project`, `uri`, `parent_uri`, `ancestors`, `is_deleted`, `ai_intent`, `ai_topics`, `created_at`, `updated_at`
- Sortable: `updated_at`, `created_at`

## Search & Scoring

### Hybrid Search (Native)

Meilisearch v1.3+ supports hybrid search combining semantic (vector) and keyword results in a single query.

```typescript
const semanticRatio = normalizedWeights.vector / (normalizedWeights.vector + normalizedWeights.keyword);

const results = await index.search(queryText, {
  hybrid: {
    semanticRatio,
    embedder: "default",
  },
  vector: queryEmbedding,
  filter: buildFilterExpression(options),
  limit: topK * CANDIDATE_MULTIPLIER,
});
```

### App-Side Re-Ranking

Meilisearch returns results with a `_rankingScore` (0-1). We combine this with app-side boosts:

```typescript
function rerank(hits, weights, options) {
  const now = Date.now();
  return hits
    .map(hit => {
      let score = hit._rankingScore;

      // Recency decay (exponential, same math as ES exp function)
      if (weights.recency > 0) {
        const ageMs = now - new Date(hit.created_at).getTime();
        const scaleMs = parseDuration(options.recencyScale || "30d");
        const decay = options.recencyDecay || 0.5;
        const recencyScore = Math.pow(decay, ageMs / scaleMs);
        score += recencyScore * weights.recency;
      }

      // Importance boost
      if (weights.importance > 0 && hit.importance) {
        score += (hit.importance / 10) * weights.importance;
      }

      // AI quality boost
      if (hit.ai_quality_score) {
        score += (hit.ai_quality_score / 10) * AI_QUALITY_FUNCTION_WEIGHT * 0.1;
      }

      return { ...hit, _score: score };
    })
    .sort((a, b) => b._score - a._score)
    .slice(0, topK);
}
```

### Feature Mapping

| ES Feature | Meilisearch Equivalent |
|---|---|
| kNN dense vector | `hybrid` search with `vector` param |
| BM25 full-text | Native keyword search (no k1/b tuning) |
| `function_score` | App-side re-ranking |
| `fuzziness` | Built-in typo tolerance (configurable per-index) |
| Ngram sub-fields | Built-in prefix search + typo tolerance |
| Phrase boost | Proximity-based ranking (native, not tunable) |
| Custom analyzer (stemming, stopwords) | Built-in language-aware tokenization |
| Synonyms | Synonyms API (pushed at setup) |
| Highlights | `_formatted` fields with crop/highlight config |
| `min_score` cutoff | `rankingScoreThreshold` parameter |

### What We Lose

- BM25 k1/b tuning knobs
- Explicit phrase boost parameter
- Ngram min/max gram control
- Per-query analyzer selection

These are acceptable: Meilisearch's defaults handle the use case well.

## CRUD Operations

### Add / Update Documents

Meilisearch `addDocuments` is an upsert — same ID replaces the document. Used for all create and update operations.

### Version History (Context Nodes)

Update flow becomes read-modify-write in application code:

1. Fetch existing document by URI
2. If exists, snapshot current `{name, abstract, overview, content, updated_at}` into `version_history` array
3. Cap `version_history` at 10 entries (shift oldest)
4. Merge updates, set new `updated_at`
5. Write full document via `addDocuments`

### Soft Delete Cascading

`deleteContextNode(uri)`:
1. Search with filter `ancestors = "uri"` to find all descendants
2. For each descendant + the node itself: set `is_deleted: true`, `deleted_at: timestamp`
3. Batch write via `addDocuments`

`restoreContextNode(uri)`:
1. Same search for descendants
2. Set `is_deleted: false`, `deleted_at: null`
3. Batch write

### Bulk Indexing

`BatchWriter` calls `MeilisearchDB.bulkIndex()` which uses Meilisearch's native `addDocuments` with batch support. Simpler than ES's alternating action/body bulk format.

### Highlights

Meilisearch returns highlighted content in `_formatted` fields when `attributesToHighlight` is set. The highlight tags default to `<em>` but are configurable. We map these to the existing `_highlight` record format for API compatibility.

### Tree Queries

- **Subtree**: `filter: "ancestors = nodeUri"` — direct equivalent of ES `term` query on `ancestors` array
- **Path**: fetch node, read its `ancestors` array, then `filter: "uri IN [ancestor1, ancestor2, ...]"`
- **Ancestors computation**: same app-side logic, no change

## Configuration

### Environment Variables

**Removed:**
- `ELASTIC_URL`, `ELASTIC_USERNAME`, `ELASTIC_PASSWORD`
- `ES_BM25_K1`, `ES_BM25_B`
- `ES_DEFAULT_FUZZINESS`, `ES_PHRASE_BOOST`

**Renamed:**
- `ES_SYNONYMS` -> `SYNONYMS`
- `ES_RECENCY_SCALE` -> `RECENCY_SCALE`
- `ES_RECENCY_DECAY` -> `RECENCY_DECAY`

**Added:**
- `MEILI_URL` (default: `http://localhost:7700`)
- `MEILI_API_KEY` (master key, required for auth)

### config.ts Changes

Replace `config.elastic` block:

```typescript
meili: {
  get url() { return process.env.MEILI_URL || "http://localhost:7700"; },
  get apiKey() { return process.env.MEILI_API_KEY || ""; },
  get synonyms(): string[] { /* same parsing from SYNONYMS env */ },
  get recencyScale() { return process.env.RECENCY_SCALE || "30d"; },
  get recencyDecay() { return parseFloat(process.env.RECENCY_DECAY || "0.5"); },
},
```

### types.ts Changes

Rename `ElasticSearchTuning` to `SearchTuning`. Remove `fuzziness` and `phraseBoost` fields (handled natively by Meilisearch). Keep `minScore`, `highlight`, `fieldBoosts`, `recencyScale`, `recencyDecay`.

### Docker Compose

```yaml
services:
  meilisearch:
    image: getmeili/meilisearch:v1.12
    ports:
      - "7700:7700"
    environment:
      MEILI_MASTER_KEY: "contextfs-dev-key"
      MEILI_ENV: "development"
    volumes:
      - meili_data:/meili_data

volumes:
  meili_data:
```

## Dependencies

**Remove:** `@elastic/elasticsearch`
**Add:** `meilisearch` (official JS client)

## Testing

- Update existing tests (`batchWriter.test.ts`, `budget.test.ts`) to use `MeilisearchDB`
- Tests that hit the DB need Meilisearch running (same pattern as current ES requirement)
- Meilisearch tasks are async — after writes, poll `waitForTask` to ensure indexing completes before assertions

## Setup Script

`bun run setup` will:
1. Create indexes with primary keys
2. Configure searchable/filterable/sortable attributes
3. Configure embedders (userProvided, dimension from config)
4. Push synonyms
5. Optionally delete and recreate indexes (destructive mode, same as current)
