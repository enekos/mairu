import { GoogleGenAI } from "@google/genai";
import * as dotenv from "dotenv";
import { assertEmbeddingDimension, getEmbeddingConfig } from "./embeddingConfig";

dotenv.config({ path: require("path").resolve(__dirname, "..", ".env") });

const embeddingConfig = getEmbeddingConfig();

const ai = process.env.GEMINI_API_KEY
  ? new GoogleGenAI({ apiKey: process.env.GEMINI_API_KEY })
  : null;

export class Embedder {
  static async getEmbedding(text: string): Promise<number[]> {
    if (!ai) {
      if (!embeddingConfig.allowZeroEmbeddings) {
        throw new Error(
          "GEMINI_API_KEY is not set and ALLOW_ZERO_EMBEDDINGS=false. Set a key or explicitly enable zero embeddings."
        );
      }
      console.warn("GEMINI_API_KEY not set, using zero vector fallback");
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
    } catch (error) {
      console.error("Failed to generate embedding:", error);
      throw error;
    }
  }
}
