export interface AgentSkill {
  project?: string;
  id: string;
  name: string;
  description: string;
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
  | "architecture";

export type MemoryOwner = "user" | "agent" | "system";

export interface AgentMemory {
  project?: string;
  id: string;
  content: string;
  category: MemoryCategory;
  owner: MemoryOwner;
  importance: number;
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
  metadata?: Record<string, any>;
  created_at: string;
  updated_at?: string;
}

export interface MemorySearchOptions {
  project?: string;
  topK?: number;
  threshold?: number;
  owner?: MemoryOwner;
  category?: MemoryCategory;
  minImportance?: number;
  maxAgeDays?: number;
  weights?: HybridSearchWeights;
}

export interface SkillSearchOptions {
  project?: string;
  topK?: number;
  threshold?: number;
  maxAgeDays?: number;
  weights?: HybridSearchWeights;
}

export interface ContextSearchOptions {
  project?: string;
  topK?: number;
  threshold?: number;
  parentUri?: string;
  maxAgeDays?: number;
  weights?: HybridSearchWeights;
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
