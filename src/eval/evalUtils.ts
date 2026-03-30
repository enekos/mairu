/** Evaluation harness utilities. Exported for testing. */

import * as fs from "fs/promises";
import * as path from "path";

export type EvalDomain = "memory" | "skill" | "context";

export interface EvalCase {
  id?: string;
  domain: EvalDomain;
  query: string;
  /** Expected IDs/URIs that must appear in retrieved results */
  expectedIds?: string[];
  expectedUris?: string[];
  expected?: string[];
  /** IDs that must NOT appear in top results (anti-recall check) */
  negativeIds?: string[];
  /** Graded relevance scores keyed by ID (1-3), enables NDCG */
  relevanceScores?: Record<string, number>;
  /** Human-readable description of what this case is testing */
  description?: string;
  topK?: number;
}

export interface EvalDataset {
  description?: string;
  fixtures?: {
    memories?: unknown[];
    skills?: unknown[];
    context?: unknown[];
  };
  cases: EvalCase[];
}

export interface ScoreStats {
  min: number;
  max: number;
  mean: number;
  /** ES score of the first relevant document in the ranked list */
  firstRelevantScore: number | null;
  /** ES score of the first irrelevant document in the ranked list */
  firstIrrelevantScore: number | null;
  /** firstRelevantScore - firstIrrelevantScore; positive = relevant ranked higher */
  relevantIrrelevantGap: number | null;
}

export interface PerCaseResult {
  id: string;
  domain: EvalDomain;
  query: string;
  description?: string;
  topK: number;
  expected: string[];
  retrieved: string[];
  scores: number[];
  scoreStats: ScoreStats;
  hitCount: number;
  recallAtK: number;
  precisionAtK: number;
  hitRate: boolean;
  averagePrecision: number;
  firstRelevantRank: number | null;
  reciprocalRank: number;
  ndcg: number;
  /** Count of negative IDs that appeared in retrieved results */
  negativeHits: number;
  latencyMs: number;
}

export interface ParseArgsResult {
  datasetPath: string;
  topK: number;
  verbose: boolean;
  outputPath: string | null;
  failBelowMrr: number | null;
  failBelowRecall: number | null;
  /** Seed fixtures from dataset before running */
  seed: boolean;
  /** Delete seeded fixtures after running */
  cleanup: boolean;
  /** Run each case with vector-only, keyword-only, and hybrid weights and compare */
  ablation: boolean;
  /** Path to a JSON file with custom HybridWeights per domain: { memory, skill, context } */
  weightsPath: string | null;
  /** Fail if average negative hit rate exceeds this threshold (0.0–1.0) */
  failAboveNegativeRate: number | null;
  /** Scope all searches to this project */
  project: string | null;
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
  const failBelowMrrRaw = args.get("fail-below-mrr");
  const failBelowRecallRaw = args.get("fail-below-recall");
  const failAboveNegativeRateRaw = args.get("fail-above-negative-rate");
  return {
    datasetPath: args.get("dataset") || "eval/dataset.json",
    topK: Number.parseInt(args.get("topK") || "5", 10),
    verbose: args.get("verbose") === "true",
    outputPath: args.get("output") || null,
    failBelowMrr: failBelowMrrRaw != null ? parseFloat(failBelowMrrRaw) : null,
    failBelowRecall: failBelowRecallRaw != null ? parseFloat(failBelowRecallRaw) : null,
    seed: args.get("seed") === "true",
    cleanup: args.get("cleanup") === "true",
    ablation: args.get("ablation") === "true",
    weightsPath: args.get("weights") || null,
    failAboveNegativeRate: failAboveNegativeRateRaw != null ? parseFloat(failAboveNegativeRateRaw) : null,
    project: args.get("project") || null,
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

export interface SummaryStats {
  cases: number;
  avgRecallAtK: number;
  stdDevRecallAtK: number;
  avgPrecisionAtK: number;
  map: number;
  mrr: number;
  hitRate: number;
  avgNdcg: number;
  avgNegativeHits: number;
  /** Mean score gap between first relevant and first irrelevant result (positive = relevant ranked higher) */
  avgRelevantIrrelevantGap: number | null;
  avgLatencyMs: number;
  p50LatencyMs: number;
  p95LatencyMs: number;
  p99LatencyMs: number;
}

function percentile(sorted: number[], p: number): number {
  if (sorted.length === 0) return 0;
  const idx = Math.ceil((p / 100) * sorted.length) - 1;
  return sorted[Math.max(0, Math.min(sorted.length - 1, idx))];
}

function stdDev(values: number[], mean: number): number {
  if (values.length === 0) return 0;
  const variance = values.reduce((sum, v) => sum + (v - mean) ** 2, 0) / values.length;
  return Math.sqrt(variance);
}

export function summarize(results: PerCaseResult[]): SummaryStats {
  const empty: SummaryStats = {
    cases: 0,
    avgRecallAtK: 0,
    stdDevRecallAtK: 0,
    avgPrecisionAtK: 0,
    map: 0,
    mrr: 0,
    hitRate: 0,
    avgNdcg: 0,
    avgNegativeHits: 0,
    avgRelevantIrrelevantGap: null,
    avgLatencyMs: 0,
    p50LatencyMs: 0,
    p95LatencyMs: 0,
    p99LatencyMs: 0,
  };
  if (results.length === 0) return empty;

  const recalls = results.map((r) => r.recallAtK);
  const avgRecallAtK = recalls.reduce((s, v) => s + v, 0) / results.length;

  const sortedLatencies = results.map((r) => r.latencyMs).sort((a, b) => a - b);

  const gapValues = results.map((r) => r.scoreStats.relevantIrrelevantGap).filter((g): g is number => g !== null);
  const avgRelevantIrrelevantGap = gapValues.length > 0
    ? gapValues.reduce((s, v) => s + v, 0) / gapValues.length
    : null;

  return {
    cases: results.length,
    avgRecallAtK,
    stdDevRecallAtK: stdDev(recalls, avgRecallAtK),
    avgPrecisionAtK: results.reduce((s, r) => s + r.precisionAtK, 0) / results.length,
    map: results.reduce((s, r) => s + r.averagePrecision, 0) / results.length,
    mrr: results.reduce((s, r) => s + r.reciprocalRank, 0) / results.length,
    hitRate: results.filter((r) => r.hitRate).length / results.length,
    avgNdcg: results.reduce((s, r) => s + r.ndcg, 0) / results.length,
    avgNegativeHits: results.reduce((s, r) => s + r.negativeHits, 0) / results.length,
    avgRelevantIrrelevantGap,
    avgLatencyMs: sortedLatencies.reduce((s, v) => s + v, 0) / results.length,
    p50LatencyMs: percentile(sortedLatencies, 50),
    p95LatencyMs: percentile(sortedLatencies, 95),
    p99LatencyMs: percentile(sortedLatencies, 99),
  };
}

export function computeScoreStats(
  retrieved: string[],
  scores: number[],
  expected: string[]
): ScoreStats {
  if (scores.length === 0) {
    return { min: 0, max: 0, mean: 0, firstRelevantScore: null, firstIrrelevantScore: null, relevantIrrelevantGap: null };
  }
  const expectedSet = new Set(expected);
  const min = Math.min(...scores);
  const max = Math.max(...scores);
  const mean = scores.reduce((s, v) => s + v, 0) / scores.length;

  let firstRelevantScore: number | null = null;
  let firstIrrelevantScore: number | null = null;
  for (let i = 0; i < retrieved.length; i++) {
    if (expectedSet.has(retrieved[i]) && firstRelevantScore === null) firstRelevantScore = scores[i];
    if (!expectedSet.has(retrieved[i]) && firstIrrelevantScore === null) firstIrrelevantScore = scores[i];
  }

  const relevantIrrelevantGap =
    firstRelevantScore !== null && firstIrrelevantScore !== null
      ? firstRelevantScore - firstIrrelevantScore
      : null;

  return { min, max, mean, firstRelevantScore, firstIrrelevantScore, relevantIrrelevantGap };
}

export interface RecallMetrics {
  hitCount: number;
  firstRelevantRank: number | null;
  reciprocalRank: number;
  recallAtK: number;
  precisionAtK: number;
  hitRate: boolean;
  averagePrecision: number;
  ndcg: number;
  negativeHits: number;
}

/**
 * Compute ideal DCG for graded relevance: sum of rel_i / log2(i+1) for ideal ordering.
 */
function idealDcg(relevanceScores: Record<string, number>, topK: number): number {
  const scores = Object.values(relevanceScores).sort((a, b) => b - a).slice(0, topK);
  return scores.reduce((sum, rel, i) => sum + rel / Math.log2(i + 2), 0);
}

export function computeRecallMetrics(
  retrieved: string[],
  expected: string[],
  negativeIds: string[] = [],
  relevanceScores: Record<string, number> = {}
): RecallMetrics {
  const expectedSet = new Set(expected);
  const negativeSet = new Set(negativeIds);

  // Binary relevance metrics
  const hitIndexes = retrieved
    .map((id, idx) => (expectedSet.has(id) ? idx + 1 : -1))
    .filter((rank) => rank > 0);
  const hitCount = hitIndexes.length;
  const firstRelevantRank = hitIndexes.length > 0 ? Math.min(...hitIndexes) : null;
  const reciprocalRank = firstRelevantRank ? 1 / firstRelevantRank : 0;
  const recallAtK = expected.length > 0 ? hitCount / expected.length : 0;
  const precisionAtK = retrieved.length > 0 ? hitCount / retrieved.length : 0;
  const hitRate = hitCount > 0;

  // Average Precision
  let relevantSeen = 0;
  let apSum = 0;
  for (let i = 0; i < retrieved.length; i++) {
    if (expectedSet.has(retrieved[i])) {
      relevantSeen++;
      apSum += relevantSeen / (i + 1);
    }
  }
  const averagePrecision = expected.length > 0 ? apSum / expected.length : 0;

  // NDCG: use relevanceScores if provided, else binary (0 or 1)
  const hasGradedRelevance = Object.keys(relevanceScores).length > 0;
  let dcg = 0;
  for (let i = 0; i < retrieved.length; i++) {
    const rel = hasGradedRelevance
      ? (relevanceScores[retrieved[i]] ?? 0)
      : (expectedSet.has(retrieved[i]) ? 1 : 0);
    dcg += rel / Math.log2(i + 2);
  }
  const idcg = hasGradedRelevance
    ? idealDcg(relevanceScores, retrieved.length)
    : (expected.length > 0 ? expected.slice(0, retrieved.length).reduce((s, _, i) => s + 1 / Math.log2(i + 2), 0) : 0);
  const ndcg = idcg > 0 ? dcg / idcg : 0;

  // Negative hits: how many negativeIds appear in retrieved
  const negativeHits = retrieved.filter((id) => negativeSet.has(id)).length;

  return {
    hitCount,
    firstRelevantRank,
    reciprocalRank,
    recallAtK,
    precisionAtK,
    hitRate,
    averagePrecision,
    ndcg,
    negativeHits,
  };
}
