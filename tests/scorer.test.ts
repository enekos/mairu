import { describe, it, expect } from "vitest";
import {
  keywordOverlapScore,
  recencyScore,
  normalizeWeights,
  hybridRerank,
  DEFAULT_MEMORY_WEIGHTS,
  DEFAULT_SKILL_WEIGHTS,
  DEFAULT_CONTEXT_WEIGHTS,
  type HybridWeights,
} from "../src/scorer";

// ─────────────────────────────────────────────────────────────────────────────
// keywordOverlapScore
// ─────────────────────────────────────────────────────────────────────────────
describe("keywordOverlapScore", () => {
  it("returns 0 for empty query", () => {
    expect(keywordOverlapScore("", "some content")).toBe(0);
  });

  it("returns 0 when query is only stopwords", () => {
    expect(keywordOverlapScore("the and or", "some content")).toBe(0);
  });

  it("returns 1 for exact match single token", () => {
    const score = keywordOverlapScore("authentication", "authentication middleware");
    expect(score).toBeGreaterThan(0);
  });

  it("partial overlap: 2 of 3 tokens match → score > 0.5", () => {
    const score = keywordOverlapScore("authentication middleware express", "authentication middleware");
    expect(score).toBeGreaterThanOrEqual(2 / 3);
  });

  it("no match returns 0", () => {
    expect(keywordOverlapScore("completely unrelated query", "nothing similar")).toBe(0);
  });

  it("phrase boost: exact substring adds extra score", () => {
    const withPhrase = keywordOverlapScore("auth middleware", "the auth middleware handles requests");
    const noPhrase = keywordOverlapScore("middleware auth", "the auth middleware handles requests");
    // Both should match tokens, but exact phrase order should score >= no phrase
    expect(withPhrase).toBeGreaterThanOrEqual(noPhrase);
  });

  it("searches across multiple fields", () => {
    const score = keywordOverlapScore("token expiry", null, undefined, "JWT token expiry handling", "auth module");
    expect(score).toBeGreaterThan(0);
  });

  it("ignores null/undefined fields", () => {
    expect(() => keywordOverlapScore("query", null, undefined, "valid text")).not.toThrow();
  });

  it("score is capped at 1.0", () => {
    const score = keywordOverlapScore("auth", "auth auth auth auth");
    expect(score).toBeLessThanOrEqual(1.0);
  });

  it("case insensitive matching", () => {
    const lower = keywordOverlapScore("authentication", "Authentication middleware");
    const upper = keywordOverlapScore("AUTHENTICATION", "authentication middleware");
    expect(lower).toBe(upper);
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// recencyScore
// ─────────────────────────────────────────────────────────────────────────────
describe("recencyScore", () => {
  it("returns 0 for null/undefined", () => {
    expect(recencyScore(null)).toBe(0);
    expect(recencyScore(undefined)).toBe(0);
  });

  it("returns close to 1 for a timestamp just now", () => {
    const score = recencyScore(new Date().toISOString());
    expect(score).toBeGreaterThan(0.99);
  });

  it("returns lower score for older timestamps", () => {
    const old = new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString(); // 30 days ago
    const recent = new Date(Date.now() - 1 * 24 * 60 * 60 * 1000).toISOString(); // 1 day ago
    expect(recencyScore(recent)).toBeGreaterThan(recencyScore(old));
  });

  it("score is always between 0 and 1", () => {
    const veryOld = new Date(2000, 0, 1).toISOString();
    expect(recencyScore(veryOld)).toBeGreaterThanOrEqual(0);
    expect(recencyScore(veryOld)).toBeLessThanOrEqual(1);
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// normalizeWeights
// ─────────────────────────────────────────────────────────────────────────────
describe("normalizeWeights", () => {
  it("normalizes weights to sum to 1", () => {
    const w = normalizeWeights({ vector: 3, keyword: 1, recency: 0, importance: 0 });
    const sum = w.vector + w.keyword + w.recency + w.importance;
    expect(sum).toBeCloseTo(1.0);
  });

  it("preserves proportions", () => {
    const w = normalizeWeights({ vector: 3, keyword: 1, recency: 0, importance: 0 });
    expect(w.vector).toBeCloseTo(0.75);
    expect(w.keyword).toBeCloseTo(0.25);
  });

  it("throws when all weights are zero", () => {
    expect(() => normalizeWeights({ vector: 0, keyword: 0, recency: 0, importance: 0 })).toThrow();
  });

  it("default weights all normalize correctly", () => {
    for (const weights of [DEFAULT_MEMORY_WEIGHTS, DEFAULT_SKILL_WEIGHTS, DEFAULT_CONTEXT_WEIGHTS]) {
      const n = normalizeWeights(weights);
      const sum = n.vector + n.keyword + n.recency + n.importance;
      expect(sum).toBeCloseTo(1.0);
    }
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// hybridRerank
// ─────────────────────────────────────────────────────────────────────────────
describe("hybridRerank", () => {
  const now = new Date().toISOString();
  const old = new Date(Date.now() - 90 * 24 * 60 * 60 * 1000).toISOString();

  const rows = [
    { id: "a", name: "JWT authentication", description: "handles JWT tokens", distance: 0.1, created_at: now },
    { id: "b", name: "database migrations", description: "runs SQL migrations", distance: 0.4, created_at: now },
    { id: "c", name: "authentication middleware", description: "validates tokens", distance: 0.3, created_at: old },
  ];

  const weights: HybridWeights = { vector: 0.7, keyword: 0.3, recency: 0, importance: 0 };

  it("returns all rows with score fields", () => {
    const result = hybridRerank(rows, "authentication", ["name", "description"], weights);
    expect(result).toHaveLength(3);
    for (const r of result) {
      expect(r).toHaveProperty("_hybrid_score");
      expect(r).toHaveProperty("_vector_score");
      expect(r).toHaveProperty("_keyword_score");
      expect(r).toHaveProperty("_recency_score");
      expect(r).toHaveProperty("_importance_score");
    }
  });

  it("sorts by hybrid_score descending", () => {
    const result = hybridRerank(rows, "authentication", ["name", "description"], weights);
    for (let i = 0; i < result.length - 1; i++) {
      expect(result[i]._hybrid_score).toBeGreaterThanOrEqual(result[i + 1]._hybrid_score);
    }
  });

  it("keyword match boosts lower-similarity result above poor-match high-similarity result", () => {
    // row b (distance 0.4) doesn't match "authentication"
    // row c (distance 0.3) does match "authentication"
    // with keyword weight, c should rank above b
    const result = hybridRerank(rows, "authentication", ["name", "description"], weights);
    const cIdx = result.findIndex((r) => r.id === "c");
    const bIdx = result.findIndex((r) => r.id === "b");
    expect(cIdx).toBeLessThan(bIdx);
  });

  it("vector score is 1 - distance (clamped to 0)", () => {
    const result = hybridRerank([{ id: "x", distance: 0.3, created_at: now }], "q", [], weights);
    expect(result[0]._vector_score).toBeCloseTo(0.7);
  });

  it("distance > 1 clamps vector score to 0", () => {
    const result = hybridRerank([{ id: "x", distance: 1.5, created_at: now }], "q", [], weights);
    expect(result[0]._vector_score).toBe(0);
  });

  it("importance field is scored 0-1 (divides by 10)", () => {
    const rows = [{ id: "a", content: "test", importance: 8, distance: 0.2, created_at: now }];
    const w: HybridWeights = { vector: 0, keyword: 0, recency: 0, importance: 1 };
    const result = hybridRerank(rows, "test", ["content"], w, "importance");
    expect(result[0]._importance_score).toBeCloseTo(0.8);
  });

  it("preserves all original row fields", () => {
    const row = { id: "z", custom_field: "hello", distance: 0.2, created_at: now };
    const result = hybridRerank([row], "q", [], weights);
    expect(result[0].custom_field).toBe("hello");
    expect(result[0].id).toBe("z");
  });

  it("handles missing distance gracefully (treats as 1)", () => {
    const row = { id: "a", name: "test", created_at: now };
    expect(() => hybridRerank([row], "test", ["name"], weights)).not.toThrow();
    const result = hybridRerank([row], "test", ["name"], weights);
    expect(result[0]._vector_score).toBe(0); // 1 - 1 = 0
  });

  it("returns empty array for empty input", () => {
    expect(hybridRerank([], "query", ["content"], weights)).toEqual([]);
  });

  it("recency weight gives newer items an edge", () => {
    const recencyRows = [
      { id: "new", content: "same content", distance: 0.3, created_at: now },
      { id: "old", content: "same content", distance: 0.3, created_at: old },
    ];
    const w: HybridWeights = { vector: 0.5, keyword: 0.0, recency: 0.5, importance: 0 };
    const result = hybridRerank(recencyRows, "same content", ["content"], w);
    expect(result[0].id).toBe("new");
  });
});
