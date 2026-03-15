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
} from "../src/evalUtils";

describe("parseArgs", () => {
  it("returns defaults for empty argv", () => {
    const result = parseArgs([]);
    expect(result.datasetPath).toBe("eval/dataset.json");
    expect(result.topK).toBe(5);
    expect(result.verbose).toBe(false);
  });

  it("parses --dataset", () => {
    const result = parseArgs(["--dataset", "custom/path.json"]);
    expect(result.datasetPath).toBe("custom/path.json");
  });

  it("parses --topK", () => {
    const result = parseArgs(["--topK", "10"]);
    expect(result.topK).toBe(10);
  });

  it("parses --verbose true", () => {
    const result = parseArgs(["--verbose", "true"]);
    expect(result.verbose).toBe(true);
  });

  it("treats flag without value as true", () => {
    const result = parseArgs(["--verbose"]);
    expect(result.verbose).toBe(true);
  });

  it("ignores non-flag arguments", () => {
    const result = parseArgs(["random", "--dataset", "x.json", "extra"]);
    expect(result.datasetPath).toBe("x.json");
  });
});

describe("loadDataset", () => {
  it("loads object with cases array", async () => {
    const tmpDir = os.tmpdir();
    const tmpFile = path.join(tmpDir, `eval-dataset-${Date.now()}.json`);
    await fs.writeFile(
      tmpFile,
      JSON.stringify({
        cases: [
          { domain: "memory", query: "q1", expectedIds: ["id1"] },
        ],
      }),
      "utf8"
    );
    try {
      const dataset = await loadDataset(tmpFile);
      expect(dataset.cases).toHaveLength(1);
      expect(dataset.cases[0].domain).toBe("memory");
      expect(dataset.cases[0].query).toBe("q1");
      expect(dataset.cases[0].expectedIds).toEqual(["id1"]);
    } finally {
      await fs.unlink(tmpFile).catch(() => {});
    }
  });

  it("accepts array as root (wraps in cases)", async () => {
    const tmpDir = os.tmpdir();
    const tmpFile = path.join(tmpDir, `eval-dataset-array-${Date.now()}.json`);
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

  it("throws when missing cases array", async () => {
    const tmpDir = os.tmpdir();
    const tmpFile = path.join(tmpDir, `eval-dataset-bad-${Date.now()}.json`);
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

describe("summarize", () => {
  it("returns zeros for empty results", () => {
    const result = summarize([]);
    expect(result).toEqual({
      cases: 0,
      avgRecallAtK: 0,
      mrr: 0,
      avgLatencyMs: 0,
    });
  });

  it("computes averages correctly", () => {
    const results: PerCaseResult[] = [
      {
        id: "1",
        domain: "memory",
        query: "q",
        topK: 5,
        expected: ["a"],
        retrieved: ["a"],
        hitCount: 1,
        recallAtK: 1,
        firstRelevantRank: 1,
        reciprocalRank: 1,
        latencyMs: 10,
      },
      {
        id: "2",
        domain: "memory",
        query: "q2",
        topK: 5,
        expected: ["b", "c"],
        retrieved: ["x", "b"],
        hitCount: 1,
        recallAtK: 0.5,
        firstRelevantRank: 2,
        reciprocalRank: 0.5,
        latencyMs: 30,
      },
    ];
    const out = summarize(results);
    expect(out.cases).toBe(2);
    expect(out.avgRecallAtK).toBe(0.75);
    expect(out.mrr).toBe(0.75);
    expect(out.avgLatencyMs).toBe(20);
  });
});

describe("computeRecallMetrics", () => {
  it("perfect recall when first result is relevant", () => {
    const m = computeRecallMetrics(["a", "b", "c"], ["a"]);
    expect(m.hitCount).toBe(1);
    expect(m.firstRelevantRank).toBe(1);
    expect(m.reciprocalRank).toBe(1);
    expect(m.recallAtK).toBe(1);
  });

  it("partial recall when relevant is at rank 2", () => {
    const m = computeRecallMetrics(["x", "a", "b"], ["a"]);
    expect(m.hitCount).toBe(1);
    expect(m.firstRelevantRank).toBe(2);
    expect(m.reciprocalRank).toBe(0.5);
    expect(m.recallAtK).toBe(1);
  });

  it("zero recall when none retrieved", () => {
    const m = computeRecallMetrics(["x", "y"], ["a", "b"]);
    expect(m.hitCount).toBe(0);
    expect(m.firstRelevantRank).toBeNull();
    expect(m.reciprocalRank).toBe(0);
    expect(m.recallAtK).toBe(0);
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
