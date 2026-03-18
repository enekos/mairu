/**
 * Application-side hybrid re-ranking with multi-token keyword overlap.
 * Two-phase retrieval: SQL fetches broad vector candidates, this module re-ranks them.
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

/** Stopwords to ignore in token overlap */
const STOPWORDS = new Set([
  "a", "an", "the", "and", "or", "but", "in", "on", "at", "to", "for",
  "of", "with", "by", "from", "is", "are", "was", "were", "be", "been",
  "has", "have", "had", "do", "does", "did", "will", "would", "could",
  "should", "may", "might", "can", "it", "its", "this", "that", "these",
  "those", "i", "you", "he", "she", "we", "they", "not", "no", "so",
]);

function tokenize(text: string): string[] {
  return text
    .toLowerCase()
    .replace(/[^a-z0-9\s_\-\.\/]/g, " ")
    .split(/\s+/)
    .filter((t) => t.length > 1 && !STOPWORDS.has(t));
}

/**
 * Multi-token keyword overlap score.
 * Returns 0-1 based on fraction of query tokens found in the target text.
 * Adds a phrase boost if the full query appears as a substring.
 */
export function keywordOverlapScore(query: string, ...fields: (string | null | undefined)[]): number {
  const queryTokens = tokenize(query);
  if (queryTokens.length === 0) return 0;

  const text = fields.filter(Boolean).join(" ").toLowerCase();

  let matched = 0;
  for (const token of queryTokens) {
    if (text.includes(token)) matched++;
  }
  const tokenScore = matched / queryTokens.length;

  // Phrase boost: reward exact substring matches
  const phraseBoost = text.includes(query.toLowerCase()) ? 0.15 : 0;

  return Math.min(1.0, tokenScore + phraseBoost);
}

export function recencyScore(createdAt: string | null | undefined): number {
  if (!createdAt) return 0;
  const daysOld = (Date.now() - new Date(createdAt).getTime()) / (1000 * 60 * 60 * 24);
  return 1.0 / (1.0 + Math.max(0, daysOld));
}

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

export interface ScoredRow {
  _vector_score: number;
  _keyword_score: number;
  _recency_score: number;
  _importance_score: number;
  _hybrid_score: number;
}

/**
 * Re-rank a set of vector search candidates with full hybrid scoring.
 * @param results   Raw rows from vector search (must have `distance` field)
 * @param query     The search query string
 * @param textFields  Which row fields to check for keyword overlap
 * @param weights   Hybrid scoring weights (will be normalized)
 * @param importanceField  Optional row field containing a 1-10 importance score
 */
export function hybridRerank<T extends { distance?: number; created_at?: string }>(
  results: T[],
  query: string,
  textFields: (keyof T)[],
  weights: HybridWeights,
  importanceField?: keyof T
): (T & ScoredRow)[] {
  const w = normalizeWeights(weights);

  return results
    .map((r) => {
      const distance = typeof r.distance === "number" ? r.distance : 1;
      const vectorScore = Math.max(0, 1.0 - distance);

      const kwScore = keywordOverlapScore(query, ...textFields.map((f) => r[f] as string | undefined));
      const recScore = recencyScore(r.created_at as string | undefined);
      const impScore = importanceField
        ? Math.min(1, Math.max(0, ((r[importanceField] as number) || 0) / 10))
        : 0;

      const hybridScore =
        vectorScore * w.vector +
        kwScore * w.keyword +
        recScore * w.recency +
        impScore * w.importance;

      return {
        ...r,
        _vector_score: vectorScore,
        _keyword_score: kwScore,
        _recency_score: recScore,
        _importance_score: impScore,
        _hybrid_score: hybridScore,
      };
    })
    .sort((a, b) => b._hybrid_score - a._hybrid_score);
}
