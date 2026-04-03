import * as dotenv from "dotenv";
import * as path from "path";
import { parsePositiveInt, parseBoolean, parseNonNegativeInt } from "./configParsing";

// 1. Try to load from the contextfs installation directory first
// In both src/core and dist/core, the project root is two levels up
dotenv.config({ path: path.resolve(__dirname, "..", "..", ".env"), quiet: true } as any);

// 2. Fallback: try loading from current working directory (e.g., where 'ctx' is executed)
dotenv.config({ path: path.resolve(process.cwd(), ".env"), quiet: true } as any);

const DEFAULT_EMBEDDING_MODEL = "gemini-embedding-001";
const DEFAULT_EMBEDDING_DIMENSION = 3072;

const KNOWN_MODEL_DIMENSIONS: Record<string, number> = {
  "gemini-embedding-001": 3072,
  "text-embedding-004": 768,
};

function parseDurationMs(value: string | undefined, defaultMs: number): number {
  if (!value) return defaultMs;
  const match = value.match(/^(\d+)(ms|s|m|h|d)$/);
  if (!match) return defaultMs;
  const num = parseInt(match[1], 10);
  const unit = match[2];
  const multipliers: Record<string, number> = { ms: 1, s: 1000, m: 60_000, h: 3_600_000, d: 86_400_000 };
  return num * (multipliers[unit] ?? 1);
}

function getEmbeddingDimension(): number {
  const configuredDimension = parsePositiveInt(process.env.EMBEDDING_DIM);
  const model = process.env.EMBEDDING_MODEL || DEFAULT_EMBEDDING_MODEL;
  const inferredDimension = KNOWN_MODEL_DIMENSIONS[model];
  const dimension = configuredDimension ?? inferredDimension ?? DEFAULT_EMBEDDING_DIMENSION;

  if (configuredDimension && inferredDimension && configuredDimension !== inferredDimension) {
    throw new Error(
      `EMBEDDING_DIM (${configuredDimension}) does not match known dimension for ${model} (${inferredDimension})`
    );
  }

  return dimension;
}

export const config = {
  meili: {
    get url() { return process.env.MEILI_URL || "http://localhost:7700"; },
    get apiKey() { return process.env.MEILI_API_KEY || ""; },
    get synonyms(): string[] {
      const raw = process.env.SYNONYMS || "";
      return raw ? raw.split(";").map((s) => s.trim()).filter(Boolean) : [];
    },
    get recencyScale() { return process.env.RECENCY_SCALE || "30d"; },
    get recencyDecay() { return parseFloat(process.env.RECENCY_DECAY || "0.5"); },
  },

  get geminiApiKey() { return process.env.GEMINI_API_KEY; },

  get llmModel() { return process.env.LLM_MODEL || "gemini-2.0-flash-lite"; },

  get dashboardApiPort() { return parsePositiveInt(process.env.DASHBOARD_API_PORT) || 8787; },

  get candidateMultiplier() { return parsePositiveInt(process.env.CANDIDATE_MULTIPLIER) || 4; },

  embedding: {
    get model() { return process.env.EMBEDDING_MODEL || DEFAULT_EMBEDDING_MODEL; },
    get dimension() { return getEmbeddingDimension(); },
    get allowZeroEmbeddings() { return parseBoolean(process.env.ALLOW_ZERO_EMBEDDINGS, true); },
  },

  budget: {
    get memoryPerProject() { return parseNonNegativeInt(process.env.MEMORY_BUDGET_PER_PROJECT) ?? 500; },
    get skillPerProject() { return parseNonNegativeInt(process.env.SKILL_BUDGET_PER_PROJECT) ?? 100; },
    get nodePerProject() { return parseNonNegativeInt(process.env.NODE_BUDGET_PER_PROJECT) ?? 1000; },
  },

  dream: {
    get threshold() { return parsePositiveInt(process.env.DREAM_THRESHOLD) ?? 25; },
    get cooldownMs() { return parseDurationMs(process.env.DREAM_COOLDOWN, 4 * 3_600_000); },
    get idleTimeoutMs() { return parseDurationMs(process.env.DREAM_IDLE_TIMEOUT, 30 * 60_000); },
    get enabled() { return parseBoolean(process.env.DREAM_ENABLED, true); },
  },
};

export function assertEmbeddingDimension(vector: number[], context: string): void {
  const dim = config.embedding.dimension;
  if (vector.length !== dim) {
    throw new Error(
      `Invalid embedding size for ${context}. Expected ${dim}, got ${vector.length}.`
    );
  }
}
