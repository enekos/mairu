export interface AgentSkill {
  id: string;
  name: string;
  description: string;
  metadata?: Record<string, any>;
  created_at: string;
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
  | "reflection";
export type MemoryOwner = "user" | "agent" | "system";

export interface AgentMemory {
  id: string;
  content: string;
  category: MemoryCategory;
  owner: MemoryOwner;
  importance: number;
  metadata?: Record<string, any>;
  created_at: string;
}

// OpenContextFS File System Paradigm Context Node
export interface AgentContextNode {
  uri: string; // Unique resource identifier (e.g., contextfs://resources/backend/api)
  parent_uri: string | null; // Parent directory URI
  name: string; // File or directory name
  abstract: string; // L0 layer: abstract (~100 tokens) - used for vector search
  overview?: string; // L1 layer: overview (~2k tokens) - used for reranking/navigation
  content?: string; // L2 layer: detailed content - loaded on demand
  metadata?: Record<string, any>;
  created_at: string;
}

export interface MemorySearchOptions {
  topK?: number;
  threshold?: number;
  owner?: MemoryOwner;
  category?: MemoryCategory;
  minImportance?: number;
  maxAgeDays?: number;
  weights?: HybridSearchWeights;
}

export interface SkillSearchOptions {
  topK?: number;
  threshold?: number;
  maxAgeDays?: number;
  weights?: HybridSearchWeights;
}

export interface ContextSearchOptions {
  topK?: number;
  threshold?: number;
  parentUri?: string;
  maxAgeDays?: number;
  weights?: HybridSearchWeights;
}
