import * as dotenv from "dotenv";
import * as fs from "fs/promises";
import * as path from "path";
import { createContextManager } from "../storage/client";
import { ElasticDB } from "../storage/elasticDB";
import { config } from "../core/config";
import { seedFixtures, cleanupFixtures, type FixtureSpec } from "./evalSeeder";
import {
  loadDataset,
  parseArgs,
  summarize,
  computeRecallMetrics,
  computeScoreStats,
  type EvalCase,
  type PerCaseResult,
  type SummaryStats,
} from "./evalUtils";
import {
  DEFAULT_MEMORY_WEIGHTS,
  DEFAULT_SKILL_WEIGHTS,
  DEFAULT_CONTEXT_WEIGHTS,
  normalizeWeights,
  type HybridWeights,
} from "../storage/scorer";
import type { MemorySearchOptions, SkillSearchOptions, ContextSearchOptions } from "../core/types";

dotenv.config({ path: require("path").resolve(__dirname, "../..", ".env") });

interface DomainWeights {
  memory: HybridWeights;
  skill: HybridWeights;
  context: HybridWeights;
}

const ABLATION_CONFIGS: Record<string, DomainWeights> = {
  "vector-only": {
    memory:  { vector: 1, keyword: 0, recency: 0, importance: 0 },
    skill:   { vector: 1, keyword: 0, recency: 0, importance: 0 },
    context: { vector: 1, keyword: 0, recency: 0, importance: 0 },
  },
  "keyword-only": {
    memory:  { vector: 0, keyword: 1, recency: 0, importance: 0 },
    skill:   { vector: 0, keyword: 1, recency: 0, importance: 0 },
    context: { vector: 0, keyword: 1, recency: 0, importance: 0 },
  },
  hybrid: {
    memory:  DEFAULT_MEMORY_WEIGHTS,
    skill:   DEFAULT_SKILL_WEIGHTS,
    context: DEFAULT_CONTEXT_WEIGHTS,
  },
};

async function runCases(
  contextManager: ReturnType<typeof createContextManager>,
  cases: EvalCase[],
  topK: number,
  project: string | null,
  weights?: DomainWeights
): Promise<PerCaseResult[]> {
  const perCase: PerCaseResult[] = [];

  for (let i = 0; i < cases.length; i += 1) {
    const entry = cases[i];
    const expected = entry.expectedIds ?? entry.expectedUris ?? entry.expected ?? [];
    if (!entry.query || expected.length === 0) {
      throw new Error(`Case at index ${i} is missing query or expected IDs.`);
    }
    const effectiveTopK = entry.topK ?? topK;
    const label = entry.id ?? `${entry.domain}-${i + 1}`;

    const memOpts: MemorySearchOptions = {
      topK: effectiveTopK,
      ...(project ? { project } : {}),
      ...(weights ? { weights: weights.memory } : {}),
    };
    const skillOpts: SkillSearchOptions = {
      topK: effectiveTopK,
      ...(project ? { project } : {}),
      ...(weights ? { weights: weights.skill } : {}),
    };
    const ctxOpts: ContextSearchOptions = {
      topK: effectiveTopK,
      ...(project ? { project } : {}),
      ...(weights ? { weights: weights.context } : {}),
    };

    const start = Date.now();
    let rows: Array<Record<string, unknown>>;
    if (entry.domain === "memory") {
      rows = (await contextManager.searchMemories(entry.query, memOpts)) as unknown as Array<Record<string, unknown>>;
    } else if (entry.domain === "skill") {
      rows = (await contextManager.searchSkills(entry.query, skillOpts)) as unknown as Array<Record<string, unknown>>;
    } else {
      rows = (await contextManager.searchContext(entry.query, ctxOpts)) as unknown as Array<Record<string, unknown>>;
    }
    const latencyMs = Date.now() - start;

    const retrieved = rows.map((row) => String(row.id ?? row.uri ?? ""));
    const scores = rows.map((row) => Number(row._score ?? 0));
    const metrics = computeRecallMetrics(retrieved, expected, entry.negativeIds ?? [], entry.relevanceScores ?? {});
    const scoreStats = computeScoreStats(retrieved, scores, expected);

    perCase.push({
      id: label,
      domain: entry.domain,
      query: entry.query,
      description: entry.description,
      topK: effectiveTopK,
      expected,
      retrieved,
      scores,
      scoreStats,
      hitCount: metrics.hitCount,
      recallAtK: metrics.recallAtK,
      precisionAtK: metrics.precisionAtK,
      hitRate: metrics.hitRate,
      averagePrecision: metrics.averagePrecision,
      firstRelevantRank: metrics.firstRelevantRank,
      reciprocalRank: metrics.reciprocalRank,
      ndcg: metrics.ndcg,
      negativeHits: metrics.negativeHits,
      latencyMs,
    });
  }

  return perCase;
}

function buildSummaryBlock(perCase: PerCaseResult[]): {
  global: SummaryStats;
  byDomain: { memory: SummaryStats; skill: SummaryStats; context: SummaryStats };
} {
  return {
    global: summarize(perCase),
    byDomain: {
      memory: summarize(perCase.filter((x) => x.domain === "memory")),
      skill: summarize(perCase.filter((x) => x.domain === "skill")),
      context: summarize(perCase.filter((x) => x.domain === "context")),
    },
  };
}

async function run() {
  const {
    datasetPath,
    topK,
    verbose,
    outputPath,
    failBelowMrr,
    failBelowRecall,
    seed,
    cleanup,
    ablation,
    weightsPath,
    failAboveNegativeRate,
    project,
  } = parseArgs(process.argv.slice(2));

  const dataset = await loadDataset(datasetPath);
  const contextManager = createContextManager();

  // Optionally load custom weights
  let customWeights: DomainWeights | undefined;
  if (weightsPath) {
    const weightsAbsolute = path.isAbsolute(weightsPath)
      ? weightsPath
      : path.resolve(process.cwd(), weightsPath);
    const raw = JSON.parse(await fs.readFile(weightsAbsolute, "utf8"));
    customWeights = {
      memory: normalizeWeights({ ...DEFAULT_MEMORY_WEIGHTS, ...raw.memory }),
      skill: normalizeWeights({ ...DEFAULT_SKILL_WEIGHTS, ...raw.skill }),
      context: normalizeWeights({ ...DEFAULT_CONTEXT_WEIGHTS, ...raw.context }),
    };
    console.error(`Loaded custom weights from ${weightsAbsolute}`);
  }

  // Seed fixtures if requested
  const fixtures = dataset.fixtures as FixtureSpec | undefined;
  if (seed && fixtures) {
    const db = new ElasticDB(
      config.elasticUrl,
      config.elasticUsername ? { username: config.elasticUsername!, password: config.elasticPassword! } : undefined
    );
    console.error("Seeding eval fixtures...");
    await seedFixtures(db, fixtures, (msg) => console.error(msg));
    // Small settle delay so ES refreshes are fully visible
    await new Promise((r) => setTimeout(r, 500));
  }

  // Main eval run
  const effectiveWeights = customWeights ?? undefined;
  const perCase = await runCases(contextManager, dataset.cases, topK, project, effectiveWeights);

  const { global, byDomain } = buildSummaryBlock(perCase);
  const failedCases = perCase.filter((c) => !c.hitRate);

  // Ablation: re-run cases with vector-only, keyword-only, and hybrid weight configs
  let ablationResults: Record<string, { global: SummaryStats; byDomain: { memory: SummaryStats; skill: SummaryStats; context: SummaryStats } }> | undefined;
  if (ablation) {
    console.error("Running ablation study (3 weight configs × all cases)...");
    ablationResults = {};
    for (const [configName, weightConfig] of Object.entries(ABLATION_CONFIGS)) {
      console.error(`  Running: ${configName}`);
      const ablationCases = await runCases(contextManager, dataset.cases, topK, project, weightConfig);
      ablationResults[configName] = buildSummaryBlock(ablationCases);
    }
  }

  const output = {
    datasetPath,
    generatedAt: new Date().toISOString(),
    config: {
      defaultTopK: topK,
      caseCount: perCase.length,
      project: project ?? undefined,
      weightsPath: weightsPath ?? undefined,
    },
    global,
    byDomain,
    ...(ablationResults ? { ablation: ablationResults } : {}),
    failedCases:
      failedCases.length > 0
        ? failedCases.map((c) => ({
            id: c.id,
            domain: c.domain,
            query: c.query,
            description: c.description,
            expected: c.expected,
            retrieved: c.retrieved,
          }))
        : undefined,
    perCase: verbose ? perCase : undefined,
  };

  const json = JSON.stringify(output, null, 2);
  console.log(json);

  if (outputPath) {
    const absolute = path.isAbsolute(outputPath) ? outputPath : path.resolve(process.cwd(), outputPath);
    await fs.writeFile(absolute, json, "utf8");
    console.error(`Results written to ${absolute}`);
  }

  // Cleanup fixtures if requested
  if (cleanup && fixtures) {
    const db = new ElasticDB(
      config.elasticUrl,
      config.elasticUsername ? { username: config.elasticUsername!, password: config.elasticPassword! } : undefined
    );
    console.error("Cleaning up eval fixtures...");
    await cleanupFixtures(db, fixtures, (msg) => console.error(msg));
  }

  // CI threshold checks
  let exitCode = 0;

  if (failBelowMrr != null && global.mrr < failBelowMrr) {
    console.error(`FAIL: MRR ${global.mrr.toFixed(4)} is below threshold ${failBelowMrr}`);
    exitCode = 1;
  }
  if (failBelowRecall != null && global.avgRecallAtK < failBelowRecall) {
    console.error(`FAIL: Recall@K ${global.avgRecallAtK.toFixed(4)} is below threshold ${failBelowRecall}`);
    exitCode = 1;
  }
  if (failAboveNegativeRate != null && global.avgNegativeHits > failAboveNegativeRate) {
    console.error(
      `FAIL: Avg negative hits ${global.avgNegativeHits.toFixed(4)} exceeds threshold ${failAboveNegativeRate}`
    );
    exitCode = 1;
  }
  if (failedCases.length > 0) {
    console.error(`\n${failedCases.length} case(s) had zero hits:`);
    for (const c of failedCases) {
      console.error(`  - [${c.domain}] "${c.query}" (expected: ${c.expected.join(", ")})`);
    }
  }

  process.exit(exitCode);
}

run().catch((error) => {
  console.error("Evaluation failed:", error);
  process.exit(1);
});
