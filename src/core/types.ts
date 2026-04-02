export interface AgentSkill {
  project?: string;
  id: string;
  name: string;
  description: string;
  ai_intent?: "fact" | "decision" | "how_to" | "todo" | "warning" | null;
  ai_topics?: string[] | null;
  ai_quality_score?: number | null;
  metadata?: Record<string, any>;
  created_at: string;
  updated_at?: string;
}

export interface HybridSearchWeights {
  vector: number;
  keyword: number;
  recency?: number;
  importance?: number;
}

/** Search tuning options available on all search methods */
export interface SearchTuning {
  /** Hard minimum score cutoff — results below this are dropped. Default: none */
  minScore?: number;
  /** Return highlighted snippets showing matched terms. Default: false */
  highlight?: boolean;
  /** Custom field boost overrides, e.g. { "name": 5, "content": 1 } */
  fieldBoosts?: Record<string, number>;
  /** Override recency scale per-query (e.g. "30d"). Falls back to config.meili.recencyScale when absent. */
  recencyScale?: string;
  /** Override recency decay factor per-query (e.g. 0.5). Falls back to config.meili.recencyDecay when absent. */
  recencyDecay?: number;
}

export type MemoryCategory =
  | "profile"
  | "preferences"
  | "entities"
  | "events"
  | "cases"
  | "patterns"
  | "observation"
  | "reflection"
  | "decision"
  | "constraint"
  | "architecture"
  | "derived_pattern";

export type MemoryOwner = "user" | "agent" | "system";

export interface AgentMemory {
  project?: string;
  id: string;
  content: string;
  category: MemoryCategory;
  owner: MemoryOwner;
  importance: number;
  ai_intent?: "fact" | "decision" | "how_to" | "todo" | "warning" | null;
  ai_topics?: string[] | null;
  ai_quality_score?: number | null;
  metadata?: Record<string, any>;
  created_at: string;
  updated_at?: string;
}

// OpenContextFS File System Paradigm Context Node
export interface AgentContextNode {
  project?: string;
  uri: string;           // Unique resource identifier (e.g., contextfs://project/backend/auth)
  parent_uri: string | null;
  name: string;
  abstract: string;      // L0: ~100 tokens, used for vector search and embedding
  overview?: string;     // L1: ~2k tokens, for reranking/navigation
  content?: string;      // L2: full detail, loaded on demand
  ai_intent?: "fact" | "decision" | "how_to" | "todo" | "warning" | null;
  ai_topics?: string[] | null;
  ai_quality_score?: number | null;
  metadata?: Record<string, any>;
  created_at: string;
  updated_at?: string;
  is_deleted?: boolean;
  deleted_at?: string;
  version_history?: Array<{
    updated_at: string;
    name: string;
    abstract: string;
    overview?: string;
    content?: string;
  }>;
}

export interface MemorySearchOptions extends SearchTuning {
  project?: string;
  topK?: number;
  threshold?: number;
  owner?: MemoryOwner;
  category?: MemoryCategory;
  minImportance?: number;
  maxAgeDays?: number;
  weights?: HybridSearchWeights;
}

export interface SkillSearchOptions extends SearchTuning {
  project?: string;
  topK?: number;
  threshold?: number;
  maxAgeDays?: number;
  weights?: HybridSearchWeights;
}

export interface ContextSearchOptions extends SearchTuning {
  project?: string;
  topK?: number;
  threshold?: number;
  parentUri?: string;
  maxAgeDays?: number;
  weights?: HybridSearchWeights;
  includeDeleted?: boolean;
}

/** Result type when LLM router decides to skip a write */
export interface SkippedWrite {
  skipped: true;
  reason: string;
  existingId: string;
}

/** Result type when LLM router decides to update an existing entry */
export interface UpdatedWrite {
  updated: true;
  id: string;
}

/** Result type when a write exceeds the per-project budget */
export interface BudgetExceeded {
  budgetExceeded: true;
  current: number;
  limit: number;
  store: "memory" | "skill" | "node";
  message: string;
}
