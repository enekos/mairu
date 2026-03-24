import { describe, it, expect } from "vitest";
import {
  normalizeWeights,
  DEFAULT_MEMORY_WEIGHTS,
  DEFAULT_SKILL_WEIGHTS,
  DEFAULT_CONTEXT_WEIGHTS,
  type HybridWeights,
} from "../src/storage/scorer";

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
// Weight presets
// ─────────────────────────────────────────────────────────────────────────────
describe("weight presets", () => {
  it("memory weights sum to 1", () => {
    const { vector, keyword, recency, importance } = DEFAULT_MEMORY_WEIGHTS;
    expect(vector + keyword + recency + importance).toBeCloseTo(1.0);
  });

  it("skill weights sum to 1", () => {
    const { vector, keyword, recency, importance } = DEFAULT_SKILL_WEIGHTS;
    expect(vector + keyword + recency + importance).toBeCloseTo(1.0);
  });

  it("context weights sum to 1", () => {
    const { vector, keyword, recency, importance } = DEFAULT_CONTEXT_WEIGHTS;
    expect(vector + keyword + recency + importance).toBeCloseTo(1.0);
  });

  it("memory weights include importance", () => {
    expect(DEFAULT_MEMORY_WEIGHTS.importance).toBeGreaterThan(0);
  });

  it("skill weights have zero importance and recency", () => {
    expect(DEFAULT_SKILL_WEIGHTS.importance).toBe(0);
    expect(DEFAULT_SKILL_WEIGHTS.recency).toBe(0);
  });

  it("context weights have zero importance", () => {
    expect(DEFAULT_CONTEXT_WEIGHTS.importance).toBe(0);
  });
});
