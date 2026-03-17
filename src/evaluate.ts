import * as dotenv from "dotenv";
import * as fs from "fs/promises";
import * as path from "path";
import { createContextManager } from "./client";
import {
  loadDataset,
  parseArgs,
  summarize,
  computeRecallMetrics,
  type PerCaseResult,
} from "./evalUtils";

dotenv.config({ path: require("path").resolve(__dirname, "..", ".env") });

async function run() {
  const { datasetPath, topK, verbose, outputPath, failBelowMrr, failBelowRecall } = parseArgs(
    process.argv.slice(2)
  );
  const dataset = await loadDataset(datasetPath);
  const contextManager = createContextManager();

  const perCase: PerCaseResult[] = [];

  for (let i = 0; i < dataset.cases.length; i += 1) {
    const entry = dataset.cases[i];
    const expected = entry.expectedIds || entry.expectedUris || entry.expected || [];
    if (!entry.query || expected.length === 0) {
      throw new Error(`Case at index ${i} is missing query or expected IDs.`);
    }
    const effectiveTopK = entry.topK ?? topK;
    const label = entry.id || `${entry.domain}-${i + 1}`;

    const start = Date.now();
    let rows: Array<Record<string, unknown>>;
    if (entry.domain === "memory") {
      rows = (await contextManager.searchMemories(entry.query, effectiveTopK)) as Array<
        Record<string, unknown>
      >;
    } else if (entry.domain === "skill") {
      rows = (await contextManager.searchSkills(entry.query, effectiveTopK)) as Array<
        Record<string, unknown>
      >;
    } else {
      rows = (await contextManager.searchContext(entry.query, effectiveTopK)) as Array<
        Record<string, unknown>
      >;
    }
    const latencyMs = Date.now() - start;

    const retrieved = rows.map((row) => String(row.id ?? row.uri ?? ""));
    const metrics = computeRecallMetrics(
      retrieved,
      expected,
      entry.negativeIds ?? [],
      entry.relevanceScores ?? {}
    );

    perCase.push({
      id: label,
      domain: entry.domain,
      query: entry.query,
      description: entry.description,
      topK: effectiveTopK,
      expected,
      retrieved,
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

  const global = summarize(perCase);
  const byDomain = {
    memory: summarize(perCase.filter((x) => x.domain === "memory")),
    skill: summarize(perCase.filter((x) => x.domain === "skill")),
    context: summarize(perCase.filter((x) => x.domain === "context")),
  };

  const failedCases = perCase.filter((c) => !c.hitRate);

  const output = {
    datasetPath,
    generatedAt: new Date().toISOString(),
    config: {
      defaultTopK: topK,
      caseCount: perCase.length,
    },
    global,
    byDomain,
    failedCases: failedCases.length > 0 ? failedCases.map((c) => ({
      id: c.id,
      domain: c.domain,
      query: c.query,
      description: c.description,
      expected: c.expected,
      retrieved: c.retrieved,
    })) : undefined,
    perCase: verbose ? perCase : undefined,
  };

  const json = JSON.stringify(output, null, 2);
  console.log(json);

  if (outputPath) {
    const absolute = path.isAbsolute(outputPath)
      ? outputPath
      : path.resolve(process.cwd(), outputPath);
    await fs.writeFile(absolute, json, "utf8");
    console.error(`Results written to ${absolute}`);
  }

  // CI threshold checks
  let exitCode = 0;
  if (failBelowMrr != null && global.mrr < failBelowMrr) {
    console.error(
      `FAIL: MRR ${global.mrr.toFixed(4)} is below threshold ${failBelowMrr}`
    );
    exitCode = 1;
  }
  if (failBelowRecall != null && global.avgRecallAtK < failBelowRecall) {
    console.error(
      `FAIL: Recall@K ${global.avgRecallAtK.toFixed(4)} is below threshold ${failBelowRecall}`
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
