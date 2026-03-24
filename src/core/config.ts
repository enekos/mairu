import * as dotenv from "dotenv";
import * as path from "path";
import { parsePositiveInt, parseBoolean } from "./configParsing";

dotenv.config({ path: path.resolve(__dirname, "..", ".env") });

const DEFAULT_EMBEDDING_MODEL = "gemini-embedding-001";
const DEFAULT_EMBEDDING_DIMENSION = 3072;

const KNOWN_MODEL_DIMENSIONS: Record<string, number> = {
  "gemini-embedding-001": 3072,
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
  get elasticUrl() { return process.env.ELASTIC_URL || "http://localhost:9200"; },
  get elasticUsername() { return process.env.ELASTIC_USERNAME; },
  get elasticPassword() { return process.env.ELASTIC_PASSWORD; },

  get geminiApiKey() { return process.env.GEMINI_API_KEY; },

  get llmModel() { return process.env.LLM_MODEL || "gemini-2.0-flash-lite"; },

  get dashboardApiPort() { return parsePositiveInt(process.env.DASHBOARD_API_PORT) || 8787; },

  get candidateMultiplier() { return parsePositiveInt(process.env.CANDIDATE_MULTIPLIER) || 4; },

  elastic: {
    get bm25K1() { return parseFloat(process.env.ES_BM25_K1 || "1.2"); },
    get bm25B() { return parseFloat(process.env.ES_BM25_B || "0.75"); },
    get recencyScale() { return process.env.ES_RECENCY_SCALE || "30d"; },
    get recencyDecay() { return parseFloat(process.env.ES_RECENCY_DECAY || "0.5"); },
    get defaultFuzziness() { return (process.env.ES_DEFAULT_FUZZINESS || "auto") as "auto" | "0" | "1" | "2"; },
    get defaultPhraseBoost() { return parseFloat(process.env.ES_PHRASE_BOOST || "2.0"); },
    get synonyms(): string[] {
      const raw = process.env.ES_SYNONYMS || "";
      return raw ? raw.split(";").map((s) => s.trim()).filter(Boolean) : [];
    },
  },

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
