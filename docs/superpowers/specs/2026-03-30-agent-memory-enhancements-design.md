# Agent Memory Enhancements Design

Implements five features inspired by [nousresearch/hermes-agent](https://github.com/nousresearch/hermes-agent) to improve agent memory management: budget enforcement, session flush/nudge, search summarization, content security scanning, and async write batching.

## 1. Memory Budget & Eviction

Per-project configurable limits on stored entries. New writes are rejected when the budget is full, prompting the agent to consolidate or evict.

### Configuration

Three new environment variables in `config.ts`:

| Variable | Default | Description |
|---|---|---|
| `MEMORY_BUDGET_PER_PROJECT` | 500 | Max memories per project (0 = unlimited) |
| `SKILL_BUDGET_PER_PROJECT` | 100 | Max skills per project (0 = unlimited) |
| `NODE_BUDGET_PER_PROJECT` | 1000 | Max context nodes per project (0 = unlimited) |

### New Type

```typescript
// core/types.ts
interface BudgetExceeded {
  budgetExceeded: true;
  current: number;
  limit: number;
  store: "memory" | "skill" | "node";
  message: string;
}
```

### New ES Operation

```typescript
// elasticDB.ts
countByProject(index: string, project: string): Promise<number>
```

Simple ES `count` API with a `term: { project }` filter.

### Enforcement

In `contextManager.ts`, the `addMemory()`, `addSkill()`, and `addContextNode()` methods check the budget **before** embedding. If the LLM router decides "update" or "skip", the budget is not consumed — only "create" triggers the check.

When budget is exceeded, the method returns a `BudgetExceeded` object instead of proceeding.

### CLI Behavior

When budget is exceeded, print:

```
Budget full (500/500 memories). Use 'memory list -P <project>' to review and 'memory delete <id>' to free space.
```

### API Behavior

Return HTTP 409 with `{ error: "budget_exceeded", current, limit, store }`.

---

## 2. Session Flush & Nudge

Two CLI commands that help agents persist observations from conversation context before it's lost to compression.

### `context-cli flush`

Takes a conversation transcript and runs a focused LLM pass to extract and persist durable facts.

```
context-cli flush [prompt] -P <project>
  --text <text>     Inline transcript text
  -f, --file <path> Read transcript from file
  -y, --yes         Auto-approve (default: true for flush)
  -k, --topK <n>    Context search depth (default: 10)
```

**Pipeline:**
1. Calls `planFlush(transcript, project)` in `vibeEngine.ts`
2. The LLM prompt is specialized for extraction: "Extract factual observations, user preferences, architectural decisions, and constraints worth persisting long-term. Ignore transient task details, debugging steps, and ephemeral state."
3. Executes the resulting mutation plan via existing `executeMutationOp()`
4. Respects memory budget — reports what couldn't be saved

### `context-cli nudge`

Lighter version: returns suggested mutations as JSON without executing them.

```
context-cli nudge [prompt] -P <project>
  --text <text>
  -f, --file <path>
  -k, --topK <n>
```

Returns the mutation plan to stdout as JSON. The agent can review and cherry-pick.

### New in vibeEngine.ts

```typescript
planFlush(
  transcript: string,
  project?: string,
  topK?: number
): Promise<VibeMutationPlan>
```

Similar to `planVibeMutation` but with a specialized system prompt focused on extracting durable facts from conversation transcripts. Reuses the same `VibeMutationPlan` type and `executeMutationOp()` path.

**System prompt differences from vibe-mutation:**
- Focuses on long-term facts, not immediate CRUD requests
- Filters out debugging steps, temporary state, in-progress work
- Prioritizes: user preferences, architectural decisions, constraints, environment facts, corrections
- Assigns higher importance (7-10) to user corrections and constraints

---

## 3. Search-Then-Summarize

A retrieval mode that runs hybrid search then passes results through an LLM to produce a synthesized answer.

### CLI Command

```
context-cli summarize <query> -P <project>
  -k, --topK <n>         Results per store (default: 5)
  --stores <list>        Comma-separated: memory,skill,node (default: all)
  --fuzziness <f>        Typo tolerance
  --phraseBoost <n>      Exact phrase boost
```

### API Endpoint

```
POST /api/search/summarize
{
  query: string,
  project?: string,
  topK?: number,
  stores?: ("memory" | "skill" | "node")[]
}
→ {
  summary: string,
  sources: { store: string, id: string, snippet: string }[]
}
```

### New in vibeEngine.ts

```typescript
summarizeSearchResults(
  query: string,
  results: Array<{ store: string, items: any[] }>,
): Promise<{ summary: string, sources: { store: string, id: string, snippet: string }[] }>
```

**Pipeline:**
1. Run hybrid search across specified stores (direct search, no LLM planning)
2. Format results as context, truncated to `MAX_CONTEXT_CHARS` (24k)
3. LLM synthesizes a focused paragraph answering the query
4. Return summary + source references

**Differences from vibe-query:**
- `vibe-query` uses an LLM to plan *which* searches to run
- `summarize` runs a single search per store, then synthesizes results
- Simpler, faster, more predictable: one search round + one LLM call

---

## 4. Content Security Scanning

Pre-write validation that scans content for prompt injection patterns.

### New Module: `src/core/contentSecurity.ts`

```typescript
interface ScanResult {
  safe: boolean;
  warnings: string[];
}

function scanContent(content: string): ScanResult
```

### Scan Rules

| Category | Patterns |
|---|---|
| **Invisible unicode** | Zero-width spaces (`\u200B`), directional overrides (`\u202E`, `\u202D`), variation selectors (`\uFE00-\uFE0F`), zero-width joiners/non-joiners |
| **Prompt injection** | "ignore previous instructions", "you are now", "disregard all", "system prompt", "override your", "forget everything", "new instructions" (case-insensitive) |
| **Exfiltration** | `curl`/`wget`/`fetch(` combined with `$SECRET`, `process.env`, `.env`, `API_KEY`, `TOKEN`, `PASSWORD` |
| **Encoded payloads** | Base64 strings longer than 100 chars (regex: `/[A-Za-z0-9+/=]{100,}/`) |

### Enforcement Points

- `contextManager.addMemory()`, `addSkill()`, `addContextNode()` — before embedding
- `vibeEngine.executeMutationOp()` — before executing creates/updates

### Behavior

**Warning, not blocking.** Returns `ScanResult` with warnings. The caller decides:
- **CLI:** Prints warnings to stderr, proceeds with write
- **API:** Includes `warnings` array in response body
- **MCP:** Includes warnings in tool result text

False positives are expected (e.g., a memory about authentication mentioning "system prompt"). Blocking would be disruptive.

---

## 5. Async Write Batching

Batch embedding calls and ES writes for high-throughput scenarios.

### New Module: `src/storage/batchWriter.ts`

```typescript
interface BatchWriterOptions {
  batchSize?: number;       // default: 10
  flushIntervalMs?: number; // default: 2000
}

type BatchOp =
  | { type: "memory"; data: MemoryInput }
  | { type: "skill"; data: SkillInput }
  | { type: "node"; data: NodeInput };

interface BatchResult {
  op: BatchOp;
  success: boolean;
  id?: string;
  error?: string;
}

class BatchWriter {
  constructor(cm: ContextManager, options?: BatchWriterOptions);
  enqueue(op: BatchOp): void;
  flush(): Promise<BatchResult[]>;
  shutdown(): Promise<void>;
}
```

**Flush strategy:**
1. Collect texts from all pending ops
2. Batch embed via `getEmbeddings()` (parallel Gemini calls)
3. Index via ES `_bulk` API
4. Return per-op results

### New in embedder.ts

```typescript
static async getEmbeddings(texts: string[]): Promise<number[][]>
```

Uses Gemini's `batchEmbedContents` API. Falls back to sequential `getEmbedding()` calls if batch API fails. Chunks into groups of 10 (Gemini batch limit).

### New in elasticDB.ts

```typescript
bulkIndex(ops: Array<{ index: string; id: string; body: object }>): Promise<{
  successful: number;
  failed: number;
  errors: Array<{ id: string; error: string }>;
}>
```

Wraps the ES `_bulk` API with index operations and `refresh: true`.

### Integration Points

- **Daemon:** `processFile()` enqueues into `BatchWriter` instead of calling `upsertFileContextNode()` directly during initial scans. Watches still process individually for low-latency updates.
- **Flush command:** When flush produces multiple mutations, they're batched.
- **Ingest command:** Batch-persists parsed context nodes.

---

## File Changes Summary

### New Files

| File | Lines (est.) | Role |
|---|---|---|
| `src/core/contentSecurity.ts` | ~80 | Prompt injection / exfiltration scanner |
| `src/storage/batchWriter.ts` | ~120 | Batch queue with flush/shutdown |
| `tests/contentSecurity.test.ts` | ~80 | Scanner unit tests |
| `tests/batchWriter.test.ts` | ~100 | Batch writer unit tests |

### Modified Files

| File | Changes |
|---|---|
| `src/core/types.ts` | Add `BudgetExceeded`, `BatchOp`, `BatchResult` types |
| `src/core/config.ts` | Add budget env vars, parse helpers |
| `src/storage/elasticDB.ts` | Add `countByProject()`, `bulkIndex()` |
| `src/storage/contextManager.ts` | Budget checks before writes, security scan calls |
| `src/storage/embedder.ts` | Add `getEmbeddings()` batch method |
| `src/llm/vibeEngine.ts` | Add `planFlush()`, `summarizeSearchResults()` |
| `src/cli.ts` | Add `flush`, `nudge`, `summarize` commands |
| `src/dashboardApi.ts` | Add `POST /api/search/summarize` endpoint |
| `src/daemon.ts` | Integrate `BatchWriter` for initial scans |

---

## Testing Strategy

- **contentSecurity.ts:** Unit tests with known injection patterns + benign content that contains trigger words (false positive verification)
- **Budget checks:** Mock `countByProject()` to return at/over/under budget, verify correct return types
- **planFlush:** Mock LLM responses, verify the specialized prompt is used and output matches `VibeMutationPlan`
- **summarizeSearchResults:** Mock search + LLM, verify summary format
- **batchWriter:** Test enqueue/flush lifecycle, verify batch embedding is called, test shutdown drains queue
- **bulkIndex:** Mock ES bulk API, verify correct request format
- **getEmbeddings:** Mock Gemini batch API, verify chunking at 10

All tests use Vitest with mocked external dependencies (ES client, Gemini API, LLM).
