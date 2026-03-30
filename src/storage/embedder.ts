import { GoogleGenAI } from "@google/genai";
import { assertEmbeddingDimension, config } from "../core/config";

const ai = config.geminiApiKey
  ? new GoogleGenAI({ apiKey: config.geminiApiKey })
  : null;

const MAX_RETRIES = 3;
const RETRY_DELAY_MS = 1000;

export class Embedder {
  static async getEmbedding(text: string, attempt = 1): Promise<number[]> {
    if (!ai) {
      if (!config.embedding.allowZeroEmbeddings) {
        throw new Error(
          "GEMINI_API_KEY is not set and ALLOW_ZERO_EMBEDDINGS=false. Set a key or explicitly enable zero embeddings."
        );
      }
      return Array(config.embedding.dimension).fill(0);
    }

    try {
      const response = await ai.models.embedContent({
        model: config.embedding.model,
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
    } catch (error: unknown) {
      if (attempt < MAX_RETRIES && ((error as {status?: number})?.status === 429 || ((error as {status?: number})?.status ?? 0) >= 500 || (error as {message?: string})?.message?.includes("fetch failed"))) {
        const delay = RETRY_DELAY_MS * Math.pow(2, attempt - 1);
        console.warn(`[Embedder] API error (${error instanceof Error ? error instanceof Error ? error.message : String(error) : String(error)}), retrying in ${delay}ms (attempt ${attempt + 1}/${MAX_RETRIES})`);
        await new Promise((resolve) => setTimeout(resolve, delay));
        return this.getEmbedding(text, attempt + 1);
      }
      console.error("Failed to generate embedding after retries:", error);
      throw error;
    }
  }

  static async getEmbeddings(texts: string[]): Promise<number[][]> {
    if (texts.length === 0) return [];
    return Promise.all(texts.map((text) => this.getEmbedding(text)));
  }
}
