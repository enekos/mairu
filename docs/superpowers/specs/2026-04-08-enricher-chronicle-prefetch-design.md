# Mairu Ecosystem Expansion: Enricher, Chronicle, Prefetch

**Date:** 2026-04-08
**Status:** Draft
**Goal:** Add deeper code understanding and autonomous context management to mairu via three new packages that build on the existing daemon + AST pipeline.

---

## Motivation

Mairu's current AST pipeline produces NL descriptions of *what* code does at the file level. This is useful but leaves three gaps:

1. **No semantic intent** — descriptions capture structure but not *why* code exists (its history, purpose, design decisions)
2. **No runtime awareness** — purely static analysis, no understanding of execution patterns, error frequency, or hot paths
3. **No anticipation** — context is only retrieved when explicitly searched; the system is reactive, not proactive

This design addresses all three by adding an enrichment pipeline (layering meaning onto existing nodes), a background learner (extracting patterns from project history), and a predictive pre-fetcher (anticipating what context agents will need).

---

## Architecture Overview

Three new packages, one enrichment pipeline:

| Package | Role | Runs as |
|---|---|---|
| `mairu/internal/enricher` | Post-ingestion passes that layer meaning onto context nodes | Inline in daemon pipeline |
| `mairu/internal/chronicle` | Background worker that extracts patterns from git/PR history | Standalone subcommand or daemon goroutine |
| `mairu/internal/prefetch` | Session-aware anticipatory retrieval and context pre-loading | Embedded in MCP server and CLI |

### Data flow

```
File change
    |
    v
[Daemon] -- AST extraction --> raw ContextNode
    |
    v
[Enricher pipeline] -- GitIntent, CallGraph, ChangeVelocity, Runtime --> enriched ContextNode
    |
    v
[Meilisearch] -- write enriched node

[Chronicle] -- background, periodic --> reads git log/PRs --> writes node annotations + memories

[Prefetch] -- per agent session --> observes signals --> predicts + pre-loads context
```

---

## Package 1: `enricher`

### Purpose

A pluggable enrichment pipeline that runs after the daemon's AST analysis. Each enricher adds a layer of meaning to context nodes before they're written to Meilisearch.

### Interface

```go
package enricher

type Enricher interface {
    // Name returns a unique identifier for this enricher.
    Name() string

    // Applicable returns true if this enricher should run on the given node.
    Applicable(node *core.ContextNode) bool

    // Enrich adds metadata/annotations to the node. Returns the modified node.
    Enrich(ctx context.Context, node *core.ContextNode) (*core.ContextNode, error)
}

// Pipeline holds an ordered list of enrichers and runs them sequentially.
type Pipeline struct {
    enrichers []Enricher
    logger    *slog.Logger
}

func NewPipeline(enrichers []Enricher, logger *slog.Logger) *Pipeline

// Run applies all applicable enrichers to the node in order.
func (p *Pipeline) Run(ctx context.Context, node *core.ContextNode) (*core.ContextNode, error)
```

### Integration with daemon

The daemon currently:
1. Detects file change
2. Parses AST via `LanguageDescriber`
3. Assembles `ContextNode`
4. Writes to Meilisearch

The enricher pipeline inserts between steps 3 and 4:

```
3. Assembles ContextNode
3.5. enricher.Pipeline.Run(ctx, node) --> enriched node
4. Writes enriched node to Meilisearch
```

The pipeline is constructed at daemon startup based on TOML config. Enrichers that aren't configured or whose dependencies aren't available are skipped silently with a log warning.

### Built-in enrichers

#### 1. GitIntentEnricher

**Purpose:** Annotates nodes with *why* code exists — extracted from commit messages, PR titles, and blame history.

**Data source:** `git log`, `git blame` (shelled out via `os/exec`).

**Algorithm:**
1. For the file associated with the node, run `git log --format='%H %s' -20 <filepath>` to get recent commit messages
2. Run `git blame --porcelain <filepath>` to map line ranges to commits
3. For function-level nodes, intersect blame line ranges with function line ranges from AST
4. Group commits by function, extract conventional commit prefixes (`feat:`, `fix:`, `refactor:`, `perf:`)
5. **Batch LLM summarization** (async, not in hot path): accumulate function-commit mappings and periodically summarize with an LLM — "summarize the history of this function in 1-2 sentences focusing on intent and evolution"
6. Write result to node's `intent` field

**Output example:**
```
intent: "Token validation — originally built for JWT (feat, commit abc123),
extended for OAuth refresh tokens (feat, commit def456).
Performance-sensitive: 2 of last 5 commits were latency fixes."
```

**LLM usage:** Batch only. On first enrichment, heuristic-only (conventional commit parsing). LLM summarization runs in a background goroutine on a configurable interval (default: every 100 nodes or 5 minutes, whichever comes first). Nodes are updated in-place after summarization completes.

**Config:**
```toml
[enricher.git_intent]
enabled = true
max_commits = 20           # how far back to look per file
llm_summarize = true       # enable LLM batch summarization
summarize_batch_size = 100 # nodes to accumulate before summarizing
```

#### 2. CallGraphEnricher

**Purpose:** Resolves cross-file call edges — "function A in file X calls function B in file Y."

**Data source:** Indexed context nodes in Meilisearch (their `overview` field contains symbol/edge listings from AST extraction).

**Algorithm:**
1. Parse the node's `overview` field to extract outgoing call edges (function calls to imported symbols)
2. For each unresolved call edge, search Meilisearch for the target symbol by name + project filter
3. If found, create a resolved edge: `{from: "file:function", to: "file:function", kind: "calls"}`
4. Write resolved edges to the node's `call_edges` field

**Output example:**
```json
{
  "call_edges": [
    {"from": "auth/validator.ts:validateToken", "to": "auth/jwt.ts:verifySignature", "kind": "calls"},
    {"from": "auth/validator.ts:validateToken", "to": "db/sessions.ts:lookupSession", "kind": "calls"}
  ]
}
```

**LLM usage:** None.

**Config:**
```toml
[enricher.call_graph]
enabled = true
max_depth = 1    # how many hops to resolve (1 = direct calls only)
```

#### 3. ChangeVelocityEnricher

**Purpose:** Tags nodes with churn signals — how frequently code changes, stability periods, and change patterns.

**Data source:** `git log --follow --format='%H %aI' <filepath>`

**Algorithm:**
1. Collect commit timestamps for the file
2. Compute: total commits, commits in last 30/90/180 days, average interval between changes
3. Derive a churn score (0.0 = dormant, 1.0 = changes every day)
4. Tag with semantic labels: "stable" (< 0.1), "moderate" (0.1-0.5), "volatile" (> 0.5)
5. Write to node's `churn_score` field and add label to `intent` if not already present

**Output example:**
```
churn_score: 0.72
intent (appended): "Volatile — 14 changes in last 90 days, most recent 2 days ago."
```

**LLM usage:** None.

**Config:**
```toml
[enricher.change_velocity]
enabled = true
lookback_days = 180
```

#### 4. RuntimeEnricher

**Purpose:** Attaches runtime signals (error rates, latency, call frequency) to context nodes from OpenTelemetry traces or structured logs.

**Data source:** OTLP/gRPC collector endpoint or structured log files (JSON lines format).

**Architecture:**

```
[OTLP Collector / Log files]
    |
    v
[RuntimeIngester] -- parses spans/logs --> raw runtime events
    |
    v
[RuntimeAggregator] -- aggregates per function/file --> RuntimeStats
    |
    v
[RuntimeEnricher] -- matches stats to nodes --> writes runtime_stats field
```

**Components:**

**RuntimeIngester** — two adapters:
- `OTLPIngester`: Connects to an OTLP/gRPC endpoint as a receiver. Consumes trace spans, extracts: service name, operation name (mapped to function), duration, status code, error flag.
- `LogIngester`: Tails structured JSON log files. Expects fields: `level`, `message`, `function` (or `caller`), `timestamp`, `error`. Configurable field mapping.

**RuntimeAggregator** — maintains a sliding window (configurable, default 1 hour) of runtime events per function/file:
- Error rate: errors / total invocations
- Latency: p50, p95, p99
- Call frequency: invocations per minute
- Last error: most recent error message

Aggregation runs in-memory with periodic flush to a lightweight local store (SQLite or file-based) so stats survive restarts.

**RuntimeEnricher** — the `Enricher` interface implementation:
1. `Applicable()`: returns true if runtime stats exist for the node's file/function
2. `Enrich()`: looks up aggregated stats, writes to node's `runtime_stats` field

**Function matching:** Maps runtime operation names to AST-extracted function names. Supports:
- Exact match: operation name == function name
- Convention-based: `ServiceName.MethodName` → `file/service.go:MethodName`
- Configurable custom mapping rules in TOML

**Output example:**
```json
{
  "runtime_stats": {
    "error_rate": 0.02,
    "latency_p50_ms": 12,
    "latency_p95_ms": 45,
    "latency_p99_ms": 120,
    "calls_per_min": 340,
    "last_error": "token expired: refresh token not found",
    "last_error_at": "2026-04-08T10:23:00Z",
    "window": "1h"
  }
}
```

**Config:**
```toml
[enricher.runtime]
enabled = false  # opt-in, requires external infrastructure

[enricher.runtime.otlp]
enabled = true
endpoint = "localhost:4317"
service_filter = ["api-gateway", "auth-service"]  # only ingest spans from these services

[enricher.runtime.logs]
enabled = false
path = "/var/log/app/*.json"
field_mapping = { function = "caller", level = "severity" }

[enricher.runtime.aggregation]
window = "1h"
flush_interval = "5m"
persist_path = ".mairu-runtime-stats"
```

### New fields on ContextNode

```go
type ContextNode struct {
    // ... existing fields ...

    // Enrichment fields (from enricher pipeline)
    Intent       string        `json:"intent,omitempty"`
    CallEdges    []CallEdge    `json:"call_edges,omitempty"`
    ChurnScore   float64       `json:"churn_score,omitempty"`
    RuntimeStats *RuntimeStats `json:"runtime_stats,omitempty"`

    // Chronicle fields (from background analyzers)
    CoChangeCluster string `json:"co_change_cluster,omitempty"` // cluster ID from CoChangeAnalyzer
    ActivityStatus  string `json:"activity_status,omitempty"`   // "active", "dormant", "deprecated"
}

type CallEdge struct {
    From string `json:"from"` // "file:function"
    To   string `json:"to"`   // "file:function"
    Kind string `json:"kind"` // "calls", "implements", "imports"
}

type RuntimeStats struct {
    ErrorRate    float64   `json:"error_rate"`
    LatencyP50   int       `json:"latency_p50_ms"`
    LatencyP95   int       `json:"latency_p95_ms"`
    LatencyP99   int       `json:"latency_p99_ms"`
    CallsPerMin  float64   `json:"calls_per_min"`
    LastError    string    `json:"last_error,omitempty"`
    LastErrorAt  time.Time `json:"last_error_at,omitempty"`
    Window       string    `json:"window"`
}
```

### Meilisearch schema changes

No new indexes. New fields are added to the existing `contextfs_context_nodes` index:

- `intent`: text (searchable, boostable)
- `call_edges`: nested object array (filterable by `to` field for reverse lookups)
- `churn_score`: float (sortable, filterable)
- `runtime_stats`: nested object (filterable by `error_rate > X`, sortable by `calls_per_min`)
- `co_change_cluster`: keyword (filterable — used by prefetch locality strategy)
- `activity_status`: keyword (filterable — "active", "dormant", "deprecated")

The `intent` field is particularly valuable — it becomes searchable text, so queries like "why does token validation exist" or "performance-sensitive auth code" will match on enriched intent descriptions.

---

## Package 2: `chronicle`

### Purpose

A background worker that passively observes git history and project activity to build semantic understanding over time. Unlike enrichers (which run per-node at ingestion time), chronicle processes project-wide patterns asynchronously.

### Architecture

```go
package chronicle

type Chronicle struct {
    repo       *git.Repository  // go-git or shelled git
    db         db.DB            // Meilisearch client
    memorySvc  *contextsrv.Service
    logger     *slog.Logger
    checkpoint string           // last processed commit SHA
    config     Config
}

func New(repo *git.Repository, db db.DB, memorySvc *contextsrv.Service, cfg Config) *Chronicle

// Start runs the chronicle loop. Blocks until ctx is cancelled.
func (c *Chronicle) Start(ctx context.Context) error

// RunOnce processes new commits since last checkpoint. Used for testing and one-shot runs.
func (c *Chronicle) RunOnce(ctx context.Context) error
```

### Execution modes

| Mode | Invocation | Use case |
|---|---|---|
| Daemon goroutine | Launched alongside daemon when `chronicle.enabled = true` | Always-on background learning |
| Standalone | `mairu-agent chronicle start` | Dedicated background process |
| One-shot | `mairu-agent chronicle run` | CI integration, manual trigger |

### Processing pipeline

Each cycle:

1. **Checkpoint read** — load last processed commit SHA from `.mairu-chronicle-checkpoint`
2. **Commit scan** — `git log <checkpoint>..HEAD` to get new commits
3. **Analyzers** — run each analyzer on the new commit batch
4. **Output** — write node annotations and memories
5. **Checkpoint write** — persist new HEAD SHA

On first run (no checkpoint), process the full git history in batches of 500 commits to avoid memory pressure.

### Analyzers

#### CoChangeAnalyzer

**Purpose:** Detect files that always change together — reveals hidden coupling.

**Algorithm:**
1. For each commit, record the set of files changed
2. Build a co-occurrence matrix: for each file pair, count how often they appear in the same commit
3. Normalize by total commits for each file (Jaccard similarity)
4. Pairs with Jaccard > 0.3 are flagged as co-change clusters

**Output:**
- Memory: `"files auth/validator.go and middleware/session.go co-change in 72% of commits — likely coupled through session validation interface"`
- Node annotation: `co_change_cluster` field with cluster ID

#### CommitPatternAnalyzer

**Purpose:** Extract semantic tags from commit message patterns per module/file.

**Algorithm:**
1. Parse conventional commit prefixes: `feat:`, `fix:`, `perf:`, `refactor:`, `docs:`, `test:`
2. Aggregate per directory/file: count of each prefix type
3. Derive tags: if > 40% of commits are `fix:` → "bug-prone", if > 30% `perf:` → "performance-sensitive"

**Output:**
- Node annotation: semantic tags appended to `intent` field
- Memory: `"The payments/ module is performance-sensitive: 35% of its commits are perf: prefixes over the last 6 months"`

#### BranchActivityAnalyzer

**Purpose:** Track what areas of the codebase are actively being worked on vs dormant.

**Algorithm:**
1. List active branches, map each to the files it touches (diff against base branch)
2. Tag directories/files as "active development" if touched by 2+ open branches
3. Tag as "dormant" if no commits in 90+ days and no active branches touch them

**Output:**
- Node annotation: `activity_status` field ("active", "dormant", "deprecated")
- Memory: `"The legacy-api/ directory is dormant — no commits in 120 days, no active branches"`

#### EvolutionSummarizer

**Purpose:** Periodically produce LLM-generated summaries of how modules evolve over time.

**Algorithm:**
1. Accumulate commit data per module (directory-level grouping)
2. When a module accumulates 50+ new commits since last summary, trigger LLM summarization
3. Prompt: "Given these commit messages for module X, summarize in 2-3 sentences: what changed, why, and what direction is this module heading?"
4. Write summary as a memory and as the module-level node's `intent` field

**LLM usage:** Batch only. Triggered by commit threshold, not per-commit. Expected: ~1 LLM call per module per week in an active project.

**Config:**
```toml
[chronicle]
enabled = true
interval = "30m"               # how often to scan for new commits
checkpoint_path = ".mairu-chronicle-checkpoint"

[chronicle.co_change]
enabled = true
min_jaccard = 0.3              # minimum co-change similarity to flag
min_commits = 5                # minimum shared commits to consider

[chronicle.commit_patterns]
enabled = true
lookback_days = 180

[chronicle.branch_activity]
enabled = true
dormant_threshold_days = 90

[chronicle.evolution]
enabled = true
summarize_threshold = 50       # commits per module before triggering LLM summary
llm_model = "gemini-2.0-flash" # lightweight model for summarization
```

---

## Package 3: `prefetch`

### Purpose

Session-aware anticipatory retrieval. Predicts what context an agent will need based on early session signals and pre-loads it before being asked.

### Architecture

```go
package prefetch

type Prefetcher struct {
    db       db.DB               // reads call_edges and co_change_cluster from stored nodes
    sessions map[string]*Session
    mu       sync.RWMutex
    config   Config
    logger   *slog.Logger
}

type Session struct {
    ID            string
    StartedAt     time.Time
    Signals       []Signal
    PrefetchedIDs map[string]bool  // node IDs already prefetched
    Cache         []core.ContextNode
    TTL           time.Duration
}

type Signal struct {
    Kind      SignalKind // TaskDescription, FileAccess, SearchQuery
    Content   string
    Timestamp time.Time
}

type SignalKind string

const (
    TaskDescription SignalKind = "task_description"
    FileAccess      SignalKind = "file_access"
    SearchQuery     SignalKind = "search_query"
    FileEdit        SignalKind = "file_edit"
)

func New(db db.DB, cfg Config) *Prefetcher

// Observe records a signal for the session and triggers async prediction.
func (p *Prefetcher) Observe(sessionID string, signal Signal)

// Get returns prefetched context for the session, if available.
func (p *Prefetcher) Get(sessionID string) []core.ContextNode

// Close cleans up expired sessions.
func (p *Prefetcher) Close()
```

### Signal collection

| Signal | Source | When |
|---|---|---|
| TaskDescription | First user message or MCP tool call argument | Session start |
| FileAccess | File read via MCP resource or agent file tool | During session |
| SearchQuery | Memory/node search via CLI or MCP tool | During session |
| FileEdit | File write/edit via agent | During session |

**MCP integration:** The MCP server wraps tool handlers to call `prefetcher.Observe()` on relevant tool calls:
- `search` tools → `SearchQuery` signal
- `read` resource → `FileAccess` signal
- Tool call arguments containing file paths → `FileAccess` signal

**CLI integration:** Search commands call `prefetcher.Observe()` before executing.

### Prediction strategies

Strategies are applied in order, each producing candidate node IDs to prefetch. Duplicates are deduplicated.

#### 1. TaskBasedStrategy

**Trigger:** `TaskDescription` signal received.

**Algorithm:**
1. Embed the task description using Gemini embeddings
2. Vector search top-10 related context nodes
3. For each result, expand via `call_edges` (1 hop) to get related functions
4. Return union of direct results + call graph neighbors

**Example:** Task "fix the token refresh bug" → vector search finds `auth/validator.ts` node → call graph expansion adds `auth/jwt.ts`, `db/sessions.ts`

#### 2. LocalityStrategy

**Trigger:** `FileAccess` or `FileEdit` signal received.

**Algorithm:**
1. Find the context node for the accessed file
2. Fetch sibling nodes in the same URI subtree (same parent directory)
3. Look up co-change clusters from chronicle — if file X co-changes with file Y, prefetch Y
4. Expand via `call_edges` (1 hop)

**Example:** Agent reads `auth/validator.ts` → siblings: `auth/jwt.ts`, `auth/types.ts` → co-change cluster adds `middleware/session.ts`

#### 3. HistoryStrategy

**Trigger:** Any signal, evaluated after other strategies.

**Algorithm:**
1. Maintain a lightweight access log: for each session, record which node IDs were actually accessed (searched or read)
2. When a new session's early signals resemble a past session (cosine similarity of signal embeddings > 0.7), prefetch the nodes that were accessed in that past session
3. Access log is persisted to `.mairu-prefetch-history` (JSON lines, capped at 1000 sessions)

**Example:** Past session about "adding a new CLI command" accessed `cmd/` nodes + `config/` nodes + `internal/cmd/` nodes. New session with similar signals gets those pre-loaded.

### Delivery modes

Configured via `prefetch.mode`:

| Mode | Behavior |
|---|---|
| `conservative` | Pre-warm: prefetched nodes are cached in session. When the agent searches, results that are already cached return instantly. No proactive push. |
| `moderate` | Cache + expand: same as conservative, but also expand via call graph and co-change clusters. Broader pre-warming. |
| `proactive` | Cache + expand + push: additionally, send a context summary to the agent via MCP notification. Summary is a short text block: "Based on your task, relevant context includes: [list of node names with 1-line intent descriptions]" |

### Performance constraints

- Prediction runs in a background goroutine — never blocks the agent's first query
- No LLM calls in the prediction hot path — uses embedding similarity + graph traversal only
- Session cache has a configurable max size (default: 50 nodes) to bound memory
- Sessions expire after configurable TTL (default: 2 hours)
- History strategy embedding comparison uses pre-computed embeddings from Meilisearch, no new embedding calls

**Config:**
```toml
[prefetch]
enabled = true
mode = "moderate"          # conservative | moderate | proactive
session_ttl = "2h"
max_cached_nodes = 50

[prefetch.task_strategy]
enabled = true
top_k = 10
expand_hops = 1

[prefetch.locality_strategy]
enabled = true
expand_hops = 1

[prefetch.history_strategy]
enabled = true
similarity_threshold = 0.7
max_history_sessions = 1000
history_path = ".mairu-prefetch-history"
```

---

## Search integration

The enrichment data should be useful during retrieval, not just stored passively.

### Enhanced search ranking

The existing scorer (`mairu/internal/contextsrv/scorer.go`) gets two new weight dimensions:

| Weight | What it boosts | Default |
|---|---|---|
| `weights.churn` | Nodes with higher churn score rank higher (actively changing code is more likely relevant) | 0.05 |
| `weights.runtime_errors` | Nodes with non-zero error rates get a boost (buggy code is often what you're looking for) | 0.05 |

### Call graph traversal in search

New search option `--expand` that, after retrieving top-K results, expands each via `call_edges` and returns the union. Useful for "show me everything related to this function."

```bash
mairu-agent node search "token validation" -k 5 --expand -P my-project
```

### Intent field search

The `intent` field is full-text searchable. Queries like these now work:

```bash
mairu-agent node search "performance-sensitive authentication" -P my-project
# Matches nodes whose intent mentions performance + auth context

mairu-agent node search "recently refactored" -P my-project
# Matches nodes whose intent mentions refactoring history
```

---

## Testing strategy

### Unit tests

| Package | Key test cases |
|---|---|
| `enricher` | Pipeline runs applicable enrichers in order; skips non-applicable; handles enricher errors gracefully |
| `enricher/gitintent` | Parses conventional commits correctly; maps blame ranges to functions; batch summarization accumulates and flushes |
| `enricher/callgraph` | Resolves cross-file edges; handles missing targets; respects max_depth |
| `enricher/changevelocity` | Computes churn scores correctly; handles files with no history; semantic labels match thresholds |
| `enricher/runtime` | OTLP ingestion parses spans; log ingestion parses JSON lines; aggregator computes percentiles; function matching works |
| `chronicle` | Checkpoint persistence; incremental processing; handles empty git history |
| `chronicle/analyzers` | Co-change Jaccard computation; commit pattern extraction; branch activity detection |
| `prefetch` | Session lifecycle (create, observe, get, expire); strategy produces correct candidates; deduplication works |

### Integration tests

- Daemon + enricher pipeline: file change → AST → enrichment → Meilisearch write → verify enriched fields
- Chronicle: create a test git repo with known history → run chronicle → verify memories and annotations
- Prefetch: simulate agent session signals → verify prefetched nodes match expected set
- Search: verify `intent` field is searchable, `--expand` flag works, new scorer weights affect ranking

### Eval harness

Extend the existing evaluation harness (`mairu/internal/eval/evaluate.go`) with:
- **Enrichment coverage:** what % of nodes have non-empty `intent`, `call_edges`, `runtime_stats`
- **Prefetch hit rate:** in a recorded session, what % of nodes the agent eventually accessed were prefetched
- **Chronicle freshness:** how stale are the oldest annotations vs current git state

---

## Rollout order

Each package ships independently. Recommended order:

| Phase | Package | Why first |
|---|---|---|
| 1 | `enricher` (GitIntent + ChangeVelocity) | Immediate value, no external dependencies, git-only |
| 2 | `enricher` (CallGraph) | Depends on having indexed nodes to resolve cross-file edges |
| 3 | `chronicle` | Depends on enricher fields existing to annotate |
| 4 | `prefetch` | Depends on call graph edges and co-change clusters for expansion strategies |
| 5 | `enricher` (Runtime) | Opt-in, requires external OTLP/log infrastructure |

Each phase is a self-contained PR that can be reviewed and tested independently.

---

## Configuration summary

All new config lives under the existing TOML config system:

```toml
# Enricher pipeline
[enricher]
# Individual enrichers configured under enricher.*
# See each enricher section above for full config

# Chronicle background worker
[chronicle]
enabled = true
interval = "30m"
checkpoint_path = ".mairu-chronicle-checkpoint"
# Individual analyzers configured under chronicle.*
# See chronicle section above for full config

# Prefetch anticipatory retrieval
[prefetch]
enabled = true
mode = "moderate"
session_ttl = "2h"
max_cached_nodes = 50
# Individual strategies configured under prefetch.*
# See prefetch section above for full config
```

Enricher pipeline and chronicle are enabled by default (they only need a git repo). Runtime enricher and prefetch proactive mode are opt-in.
