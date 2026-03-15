import * as dotenv from "dotenv";
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
  const { datasetPath, topK, verbose } = parseArgs(process.argv.slice(2));
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
    const { hitCount, firstRelevantRank, reciprocalRank, recallAtK } = computeRecallMetrics(
      retrieved,
      expected
    );

    perCase.push({
      id: label,
      domain: entry.domain,
      query: entry.query,
      topK: effectiveTopK,
      expected,
      retrieved,
      hitCount,
      recallAtK,
      firstRelevantRank,
      reciprocalRank,
      latencyMs,
    });
  }

  const global = summarize(perCase);
  const byDomain = {
    memory: summarize(perCase.filter((x) => x.domain === "memory")),
    skill: summarize(perCase.filter((x) => x.domain === "skill")),
    context: summarize(perCase.filter((x) => x.domain === "context")),
  };

  const output = {
    datasetPath,
    generatedAt: new Date().toISOString(),
    config: {
      defaultTopK: topK,
      caseCount: perCase.length,
    },
    global,
    byDomain,
    perCase: verbose ? perCase : undefined,
  };

  console.log(JSON.stringify(output, null, 2));
}

run().catch((error) => {
  console.error("Evaluation failed:", error);
  process.exit(1);
});
