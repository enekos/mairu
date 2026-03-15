import { parsePositiveInt, parseBoolean } from "./configParsing";

const DEFAULT_MODEL = "gemini-embedding-001";
const DEFAULT_DIMENSION = 768;

const KNOWN_MODEL_DIMENSIONS: Record<string, number> = {
  "gemini-embedding-001": 768,
  "text-embedding-004": 768,
};

const EMBEDDING_MODEL = process.env.EMBEDDING_MODEL || DEFAULT_MODEL;
const configuredDimension = parsePositiveInt(process.env.EMBEDDING_DIM);
const inferredDimension = KNOWN_MODEL_DIMENSIONS[EMBEDDING_MODEL];
const EMBEDDING_DIMENSION = configuredDimension ?? inferredDimension ?? DEFAULT_DIMENSION;
const ALLOW_ZERO_EMBEDDINGS = parseBoolean(process.env.ALLOW_ZERO_EMBEDDINGS, false);

if (configuredDimension && inferredDimension && configuredDimension !== inferredDimension) {
  throw new Error(
    `EMBEDDING_DIM (${configuredDimension}) does not match known dimension for ${EMBEDDING_MODEL} (${inferredDimension})`
  );
}

export function assertEmbeddingDimension(vector: number[], context: string): void {
  if (vector.length !== EMBEDDING_DIMENSION) {
    throw new Error(
      `Invalid embedding size for ${context}. Expected ${EMBEDDING_DIMENSION}, got ${vector.length}.`
    );
  }
}

export function getEmbeddingConfig() {
  return {
    model: EMBEDDING_MODEL,
    dimension: EMBEDDING_DIMENSION,
    allowZeroEmbeddings: ALLOW_ZERO_EMBEDDINGS,
  };
}
