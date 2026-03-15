/** Evaluation harness utilities. Exported for testing. */

import * as fs from "fs/promises";
import * as path from "path";

export type EvalDomain = "memory" | "skill" | "context";

export interface EvalCase {
  id?: string;
  domain: EvalDomain;
  query: string;
  expectedIds?: string[];
  expectedUris?: string[];
  expected?: string[];
  topK?: number;
}

export interface EvalDataset {
  cases: EvalCase[];
}

export interface PerCaseResult {
  id: string;
  domain: EvalDomain;
  query: string;
  topK: number;
  expected: string[];
  retrieved: string[];
  hitCount: number;
  recallAtK: number;
  firstRelevantRank: number | null;
  reciprocalRank: number;
  latencyMs: number;
}

export interface ParseArgsResult {
  datasetPath: string;
  topK: number;
  verbose: boolean;
}

export function parseArgs(argv: string[]): ParseArgsResult {
  const args = new Map<string, string>();
  for (let i = 0; i < argv.length; i += 1) {
    const token = argv[i];
    if (!token.startsWith("--")) continue;
    const key = token.slice(2);
    const value = argv[i + 1] && !argv[i + 1].startsWith("--") ? argv[i + 1] : "true";
    args.set(key, value);
  }
  return {
    datasetPath: args.get("dataset") || "eval/dataset.json",
    topK: Number.parseInt(args.get("topK") || "5", 10),
    verbose: args.get("verbose") === "true",
  };
}

export async function loadDataset(datasetPath: string): Promise<EvalDataset> {
  const absolute = path.isAbsolute(datasetPath)
    ? datasetPath
    : path.resolve(process.cwd(), datasetPath);
  const raw = await fs.readFile(absolute, "utf8");
  const parsed = JSON.parse(raw);
  if (Array.isArray(parsed)) {
    return { cases: parsed as EvalCase[] };
  }
  if (!parsed || !Array.isArray(parsed.cases)) {
    throw new Error("Dataset must be an array or an object with a `cases` array.");
  }
  return parsed as EvalDataset;
}

export function summarize(results: PerCaseResult[]) {
  const totals = {
    cases: results.length,
    avgRecallAtK: 0,
    mrr: 0,
    avgLatencyMs: 0,
  };
  if (results.length === 0) return totals;

  totals.avgRecallAtK =
    results.reduce((sum, result) => sum + result.recallAtK, 0) / results.length;
  totals.mrr = results.reduce((sum, result) => sum + result.reciprocalRank, 0) / results.length;
  totals.avgLatencyMs =
    results.reduce((sum, result) => sum + result.latencyMs, 0) / results.length;
  return totals;
}

export interface RecallMetrics {
  hitCount: number;
  firstRelevantRank: number | null;
  reciprocalRank: number;
  recallAtK: number;
}

export function computeRecallMetrics(
  retrieved: string[],
  expected: string[]
): RecallMetrics {
  const expectedSet = new Set(expected);
  const hitIndexes = retrieved
    .map((id, idx) => (expectedSet.has(id) ? idx + 1 : -1))
    .filter((rank) => rank > 0);
  const hitCount = hitIndexes.length;
  const firstRelevantRank = hitIndexes.length > 0 ? Math.min(...hitIndexes) : null;
  const reciprocalRank = firstRelevantRank ? 1 / firstRelevantRank : 0;
  const recallAtK = hitCount / expected.length;
  return { hitCount, firstRelevantRank, reciprocalRank, recallAtK };
}
