import { describe, it, expect } from "vitest";
import * as fs from "fs/promises";
import * as path from "path";
import * as os from "os";
import {
  parseArgs,
  loadDataset,
  summarize,
  computeRecallMetrics,
  type PerCaseResult,
} from "../src/eval/evalUtils";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeResult(overrides: Partial<PerCaseResult> = {}): PerCaseResult {
  return {
    id: "test-1",
    domain: "memory",
    query: "q",
    topK: 5,
    expected: ["a"],
    retrieved: ["a"],
    scores: [1],
    scoreStats: {
      firstRelevantScore: 1,
      firstIrrelevantScore: null,
      relevantIrrelevantGap: null,
    },
    hitCount: 1,
    recallAtK: 1,
    precisionAtK: 1,
    hitRate: true,
    averagePrecision: 1,
    firstRelevantRank: 1,
    reciprocalRank: 1,
    ndcg: 1,
    negativeHits: 0,
    latencyMs: 10,
    ...overrides,
  };
}

// ---------------------------------------------------------------------------
// parseArgs
// ---------------------------------------------------------------------------

describe("parseArgs", () => {
  it("returns defaults for empty argv", () => {
    const result = parseArgs([]);
    expect(result.datasetPath).toBe("eval/dataset.json");
    expect(result.topK).toBe(5);
    expect(result.verbose).toBe(false);
    expect(result.outputPath).toBeNull();
    expect(result.failBelowMrr).toBeNull();
    expect(result.failBelowRecall).toBeNull();
  });

  it("parses --dataset", () => {
    expect(parseArgs(["--dataset", "custom/path.json"]).datasetPath).toBe("custom/path.json");
  });

  it("parses --topK", () => {
    expect(parseArgs(["--topK", "10"]).topK).toBe(10);
  });

  it("parses --verbose true", () => {
    expect(parseArgs(["--verbose", "true"]).verbose).toBe(true);
  });

  it("treats flag without value as true", () => {
    expect(parseArgs(["--verbose"]).verbose).toBe(true);
  });

  it("ignores non-flag arguments", () => {
    expect(parseArgs(["random", "--dataset", "x.json", "extra"]).datasetPath).toBe("x.json");
  });

  it("parses --output", () => {
    expect(parseArgs(["--output", "results/out.json"]).outputPath).toBe("results/out.json");
  });

  it("parses --fail-below-mrr", () => {
    expect(parseArgs(["--fail-below-mrr", "0.8"]).failBelowMrr).toBeCloseTo(0.8);
  });

  it("parses --fail-below-recall", () => {
    expect(parseArgs(["--fail-below-recall", "0.9"]).failBelowRecall).toBeCloseTo(0.9);
  });
});

// ---------------------------------------------------------------------------
// loadDataset
// ---------------------------------------------------------------------------

describe("loadDataset", () => {
  it("loads object with cases array", async () => {
    const tmpFile = path.join(os.tmpdir(), `eval-dataset-${Date.now()}.json`);
    await fs.writeFile(
      tmpFile,
      JSON.stringify({ cases: [{ domain: "memory", query: "q1", expectedIds: ["id1"] }] }),
      "utf8"
    );
    try {
      const dataset = await loadDataset(tmpFile);
      expect(dataset.cases).toHaveLength(1);
      expect(dataset.cases[0].domain).toBe("memory");
      expect(dataset.cases[0].expectedIds).toEqual(["id1"]);
    } finally {
      await fs.unlink(tmpFile).catch(() => {});
    }
  });

  it("accepts array as root (wraps in cases)", async () => {
    const tmpFile = path.join(os.tmpdir(), `eval-dataset-array-${Date.now()}.json`);
    await fs.writeFile(
      tmpFile,
      JSON.stringify([{ domain: "skill", query: "q2", expectedIds: ["id2"] }]),
      "utf8"
    );
    try {
      const dataset = await loadDataset(tmpFile);
      expect(dataset.cases).toHaveLength(1);
      expect(dataset.cases[0].domain).toBe("skill");
    } finally {
      await fs.unlink(tmpFile).catch(() => {});
    }
  });

  it("loads optional new fields without error", async () => {
    const tmpFile = path.join(os.tmpdir(), `eval-dataset-new-${Date.now()}.json`);
    await fs.writeFile(
      tmpFile,
      JSON.stringify({
        cases: [{
          id: "case-1",
          domain: "memory",
          query: "test",
          expectedIds: ["id1"],
          negativeIds: ["bad1"],
          relevanceScores: { id1: 2 },
          description: "test description",
        }],
      }),
      "utf8"
    );
    try {
      const dataset = await loadDataset(tmpFile);
      const c = dataset.cases[0];
      expect(c.negativeIds).toEqual(["bad1"]);
      expect(c.relevanceScores).toEqual({ id1: 2 });
      expect(c.description).toBe("test description");
    } finally {
      await fs.unlink(tmpFile).catch(() => {});
    }
  });

  it("throws when missing cases array", async () => {
    const tmpFile = path.join(os.tmpdir(), `eval-dataset-bad-${Date.now()}.json`);
    await fs.writeFile(tmpFile, JSON.stringify({}), "utf8");
    try {
      await expect(loadDataset(tmpFile)).rejects.toThrow(
        "Dataset must be an array or an object with a `cases` array."
      );
    } finally {
      await fs.unlink(tmpFile).catch(() => {});
    }
  });
});

// ---------------------------------------------------------------------------
// summarize
// ---------------------------------------------------------------------------

describe("summarize", () => {
  it("returns zeros for empty results", () => {
    const result = summarize([]);
    expect(result.cases).toBe(0);
    expect(result.avgRecallAtK).toBe(0);
    expect(result.mrr).toBe(0);
    expect(result.avgLatencyMs).toBe(0);
    expect(result.p50LatencyMs).toBe(0);
    expect(result.p95LatencyMs).toBe(0);
    expect(result.p99LatencyMs).toBe(0);
  });

  it("computes averages correctly for two cases", () => {
    const results: PerCaseResult[] = [
      makeResult({ recallAtK: 1, precisionAtK: 1, averagePrecision: 1, reciprocalRank: 1, ndcg: 1, hitRate: true, latencyMs: 10 }),
      makeResult({ id: "2", recallAtK: 0.5, precisionAtK: 0.5, averagePrecision: 0.5, reciprocalRank: 0.5, ndcg: 0.5, hitRate: true, latencyMs: 30 }),
    ];
    const out = summarize(results);
    expect(out.cases).toBe(2);
    expect(out.avgRecallAtK).toBe(0.75);
    expect(out.avgPrecisionAtK).toBe(0.75);
    expect(out.map).toBe(0.75);
    expect(out.mrr).toBe(0.75);
    expect(out.avgNdcg).toBe(0.75);
    expect(out.avgLatencyMs).toBe(20);
  });

  it("computes hitRate as fraction of cases with at least one hit", () => {
    const results = [
      makeResult({ hitRate: true }),
      makeResult({ id: "2", hitRate: false }),
      makeResult({ id: "3", hitRate: true }),
    ];
    expect(summarize(results).hitRate).toBeCloseTo(2 / 3);
  });

  it("computes stdDevRecallAtK", () => {
    const results = [
      makeResult({ recallAtK: 0 }),
      makeResult({ id: "2", recallAtK: 1 }),
    ];
    const out = summarize(results);
    // mean=0.5, variance=0.25, stddev=0.5
    expect(out.stdDevRecallAtK).toBeCloseTo(0.5);
  });

  it("computes latency percentiles", () => {
    // 20 values: 5, 10, 15, ..., 100 (step 5)
    const latencies = Array.from({ length: 20 }, (_, i) => (i + 1) * 5);
    const results = latencies.map((ms, i) => makeResult({ id: `${i}`, latencyMs: ms }));
    const out = summarize(results);
    // p50: ceil(0.50 * 20) - 1 = 10 - 1 = 9 → sorted[9] = 50
    expect(out.p50LatencyMs).toBe(50);
    // p95: ceil(0.95 * 20) - 1 = 19 - 1 = 18 → sorted[18] = 95
    expect(out.p95LatencyMs).toBe(95);
    // p99: ceil(0.99 * 20) - 1 = 20 - 1 = 19 → sorted[19] = 100
    expect(out.p99LatencyMs).toBe(100);
  });

  it("computes avgNegativeHits", () => {
    const results = [
      makeResult({ negativeHits: 2 }),
      makeResult({ id: "2", negativeHits: 0 }),
    ];
    expect(summarize(results).avgNegativeHits).toBe(1);
  });
});

// ---------------------------------------------------------------------------
// computeRecallMetrics — binary relevance
// ---------------------------------------------------------------------------

describe("computeRecallMetrics — binary", () => {
  it("perfect recall when first result is relevant", () => {
    const m = computeRecallMetrics(["a", "b", "c"], ["a"]);
    expect(m.hitCount).toBe(1);
    expect(m.firstRelevantRank).toBe(1);
    expect(m.reciprocalRank).toBe(1);
    expect(m.recallAtK).toBe(1);
    expect(m.precisionAtK).toBeCloseTo(1 / 3);
    expect(m.hitRate).toBe(true);
    expect(m.averagePrecision).toBe(1);
  });

  it("partial recall when relevant is at rank 2", () => {
    const m = computeRecallMetrics(["x", "a", "b"], ["a"]);
    expect(m.hitCount).toBe(1);
    expect(m.firstRelevantRank).toBe(2);
    expect(m.reciprocalRank).toBe(0.5);
    expect(m.recallAtK).toBe(1);
    expect(m.precisionAtK).toBeCloseTo(1 / 3);
    expect(m.hitRate).toBe(true);
    expect(m.averagePrecision).toBe(0.5);
  });

  it("zero recall when none retrieved", () => {
    const m = computeRecallMetrics(["x", "y"], ["a", "b"]);
    expect(m.hitCount).toBe(0);
    expect(m.firstRelevantRank).toBeNull();
    expect(m.reciprocalRank).toBe(0);
    expect(m.recallAtK).toBe(0);
    expect(m.precisionAtK).toBe(0);
    expect(m.hitRate).toBe(false);
    expect(m.averagePrecision).toBe(0);
  });

  it("multiple expected hits", () => {
    const m = computeRecallMetrics(["a", "b", "c"], ["a", "b"]);
    expect(m.hitCount).toBe(2);
    expect(m.firstRelevantRank).toBe(1);
    expect(m.reciprocalRank).toBe(1);
    expect(m.recallAtK).toBe(1);
  });

  it("partial hit count affects recall", () => {
    const m = computeRecallMetrics(["a", "x"], ["a", "b"]);
    expect(m.hitCount).toBe(1);
    expect(m.recallAtK).toBe(0.5);
  });
});

// ---------------------------------------------------------------------------
// computeRecallMetrics — precisionAtK
// ---------------------------------------------------------------------------

describe("computeRecallMetrics — precisionAtK", () => {
  it("is 1 when all retrieved are relevant", () => {
    const m = computeRecallMetrics(["a", "b"], ["a", "b", "c"]);
    expect(m.precisionAtK).toBe(1);
  });

  it("is 0.5 when half retrieved are relevant", () => {
    const m = computeRecallMetrics(["a", "x"], ["a"]);
    expect(m.precisionAtK).toBe(0.5);
  });

  it("is 0 for empty retrieved", () => {
    const m = computeRecallMetrics([], ["a"]);
    expect(m.precisionAtK).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// computeRecallMetrics — averagePrecision (AP)
// ---------------------------------------------------------------------------

describe("computeRecallMetrics — averagePrecision", () => {
  it("AP=1 when all relevant retrieved in order", () => {
    const m = computeRecallMetrics(["a", "b"], ["a", "b"]);
    // rank 1 hit: P@1=1, rank 2 hit: P@2=1 → AP=(1+1)/2=1
    expect(m.averagePrecision).toBe(1);
  });

  it("AP < 1 when relevant items are interspersed with irrelevant", () => {
    // retrieved: [x, a, y, b], expected: [a, b]
    // rank 2 hit: P@2=1/2, rank 4 hit: P@4=2/4=0.5 → AP=(0.5+0.5)/2=0.5
    const m = computeRecallMetrics(["x", "a", "y", "b"], ["a", "b"]);
    expect(m.averagePrecision).toBeCloseTo(0.5);
  });

  it("AP=0 when no relevant items retrieved", () => {
    const m = computeRecallMetrics(["x", "y"], ["a"]);
    expect(m.averagePrecision).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// computeRecallMetrics — NDCG
// ---------------------------------------------------------------------------

describe("computeRecallMetrics — NDCG (binary)", () => {
  it("ndcg=1 for perfect binary ranking", () => {
    const m = computeRecallMetrics(["a", "b"], ["a", "b"]);
    expect(m.ndcg).toBeCloseTo(1);
  });

  it("ndcg < 1 when relevant item is not first", () => {
    // retrieved: [x, a], expected: [a]
    const m = computeRecallMetrics(["x", "a"], ["a"]);
    expect(m.ndcg).toBeGreaterThan(0);
    expect(m.ndcg).toBeLessThan(1);
  });

  it("ndcg=0 when nothing relevant retrieved", () => {
    const m = computeRecallMetrics(["x", "y"], ["a"]);
    expect(m.ndcg).toBe(0);
  });
});

describe("computeRecallMetrics — NDCG (graded relevance)", () => {
  it("perfect graded ranking yields ndcg=1", () => {
    const relevanceScores = { a: 3, b: 2, c: 1 };
    const m = computeRecallMetrics(["a", "b", "c"], [], [], relevanceScores);
    expect(m.ndcg).toBeCloseTo(1);
  });

  it("reversed graded ranking yields ndcg < 1", () => {
    const relevanceScores = { a: 3, b: 2, c: 1 };
    const m = computeRecallMetrics(["c", "b", "a"], [], [], relevanceScores);
    expect(m.ndcg).toBeLessThan(1);
    expect(m.ndcg).toBeGreaterThan(0);
  });

  it("irrelevant items only yields ndcg=0", () => {
    const relevanceScores = { a: 3 };
    const m = computeRecallMetrics(["x", "y"], [], [], relevanceScores);
    expect(m.ndcg).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// computeRecallMetrics — negativeIds
// ---------------------------------------------------------------------------

describe("computeRecallMetrics — negativeIds", () => {
  it("counts negative ID hits in retrieved results", () => {
    const m = computeRecallMetrics(["a", "bad1", "bad2"], ["a"], ["bad1", "bad2"]);
    expect(m.negativeHits).toBe(2);
  });

  it("negativeHits=0 when no negatives retrieved", () => {
    const m = computeRecallMetrics(["a", "b"], ["a"], ["bad1"]);
    expect(m.negativeHits).toBe(0);
  });

  it("negativeHits works independently of expected hits", () => {
    // Nothing expected, but negative was retrieved
    const m = computeRecallMetrics(["bad1", "x"], [], ["bad1"]);
    expect(m.negativeHits).toBe(1);
    expect(m.recallAtK).toBe(0); // no expected, so 0
  });
});
