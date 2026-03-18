import { parsePositiveInt, parseBoolean } from "./configParsing";

const DEFAULT_EMBEDDING_MODEL = "gemini-embedding-004";
const DEFAULT_EMBEDDING_DIMENSION = 768;

const KNOWN_MODEL_DIMENSIONS: Record<string, number> = {
  "gemini-embedding-001": 768,
  "text-embedding-004": 768,
};

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
  get tursoUrl() { return process.env.TURSO_URL; },
  get tursoAuthToken() { return process.env.TURSO_AUTH_TOKEN; },

  get geminiApiKey() { return process.env.GEMINI_API_KEY; },

  get llmModel() { return process.env.LLM_MODEL || "gemini-3.1-flash-lite-preview"; },

  get dashboardApiPort() { return parsePositiveInt(process.env.DASHBOARD_API_PORT) || 8787; },

  get candidateMultiplier() { return parsePositiveInt(process.env.CANDIDATE_MULTIPLIER) || 4; },

  embedding: {
    get model() { return process.env.EMBEDDING_MODEL || DEFAULT_EMBEDDING_MODEL; },
    get dimension() { return getEmbeddingDimension(); },
    get allowZeroEmbeddings() { return parseBoolean(process.env.ALLOW_ZERO_EMBEDDINGS, false); },
  }
};

export function assertEmbeddingDimension(vector: number[], context: string): void {
  const dim = config.embedding.dimension;
  if (vector.length !== dim) {
    throw new Error(
      `Invalid embedding size for ${context}. Expected ${dim}, got ${vector.length}.`
    );
  }
}
