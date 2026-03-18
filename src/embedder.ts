import { GoogleGenAI } from "@google/genai";
import * as dotenv from "dotenv";
import { assertEmbeddingDimension, getEmbeddingConfig } from "./embeddingConfig";

dotenv.config({ path: require("path").resolve(__dirname, "..", ".env") });

const embeddingConfig = getEmbeddingConfig();

const ai = process.env.GEMINI_API_KEY
  ? new GoogleGenAI({ apiKey: process.env.GEMINI_API_KEY })
  : null;

const MAX_RETRIES = 3;
const RETRY_DELAY_MS = 1000;

export class Embedder {
  static async getEmbedding(text: string, attempt = 1): Promise<number[]> {
    if (!ai) {
      if (!embeddingConfig.allowZeroEmbeddings) {
        throw new Error(
          "GEMINI_API_KEY is not set and ALLOW_ZERO_EMBEDDINGS=false. Set a key or explicitly enable zero embeddings."
        );
      }
      return Array(embeddingConfig.dimension).fill(0);
    }

    try {
      const response = await ai.models.embedContent({
        model: embeddingConfig.model,
        contents: text,
      });
      if (
        !response.embeddings ||
        response.embeddings.length === 0 ||
        !response.embeddings[0].values
      ) {
        throw new Error("No embedding returned from Gemini API");
      }
      const values = response.embeddings[0].values;
      assertEmbeddingDimension(values, "Embedder.getEmbedding");
      return values;
    } catch (error: any) {
      if (attempt < MAX_RETRIES && (error?.status === 429 || error?.status >= 500 || error?.message?.includes("fetch failed"))) {
        const delay = RETRY_DELAY_MS * Math.pow(2, attempt - 1);
        console.warn(`[Embedder] API error (${error.message}), retrying in ${delay}ms (attempt ${attempt + 1}/${MAX_RETRIES})`);
        await new Promise((resolve) => setTimeout(resolve, delay));
        return this.getEmbedding(text, attempt + 1);
      }
      console.error("Failed to generate embedding after retries:", error);
      throw error;
    }
  }
}
