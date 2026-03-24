/**
 * Hybrid scoring weight definitions.
 * Elasticsearch handles BM25, kNN, and function scoring natively.
 * These weights map to ES boost parameters in ElasticDB.
 */

export interface HybridWeights {
  vector: number;
  keyword: number;
  recency: number;
  importance: number;
}

export const DEFAULT_MEMORY_WEIGHTS: HybridWeights = {
  vector: 0.6,
  keyword: 0.2,
  recency: 0.05,
  importance: 0.15,
};

export const DEFAULT_SKILL_WEIGHTS: HybridWeights = {
  vector: 0.7,
  keyword: 0.3,
  recency: 0,
  importance: 0,
};

export const DEFAULT_CONTEXT_WEIGHTS: HybridWeights = {
  vector: 0.65,
  keyword: 0.3,
  recency: 0.05,
  importance: 0,
};

export function normalizeWeights(w: HybridWeights): HybridWeights {
  const total = w.vector + w.keyword + w.recency + w.importance;
  if (total <= 0) throw new Error("Weights must sum to a positive number");
  return {
    vector: w.vector / total,
    keyword: w.keyword / total,
    recency: w.recency / total,
    importance: w.importance / total,
  };
}
