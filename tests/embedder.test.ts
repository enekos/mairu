import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Mock `dotenv`
vi.mock("dotenv", () => {
  return {
    config: vi.fn(),
  };
});

// Mock `@google/genai`
const mockEmbedContent = vi.fn();
vi.mock("@google/genai", () => {
  return {
    GoogleGenAI: vi.fn().mockImplementation(() => {
      return {
        models: {
          embedContent: mockEmbedContent,
        },
      };
    }),
  };
});

describe("Embedder", () => {
  const originalEnv = process.env;

  beforeEach(() => {
    if (typeof vi.resetModules === "function") {
      vi.resetModules();
    } else {
      for (const key in require.cache) {
        if (key.includes("/src/")) {
          delete require.cache[key];
        }
      }
    }
    vi.clearAllMocks();
    process.env = { ...originalEnv };
    delete process.env.EMBEDDING_MODEL;
    delete process.env.EMBEDDING_DIM;
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  it("returns zero embeddings when GEMINI_API_KEY is not set and allowZeroEmbeddings is true", async () => {
    delete process.env.GEMINI_API_KEY;
    process.env.ALLOW_ZERO_EMBEDDINGS = "true";

    const { Embedder: FreshEmbedder } = await import("../src/embedder");

    const result = await FreshEmbedder.getEmbedding("test text");
    expect(result.length).toBe(768); // fallback dimension from getEmbeddingConfig if not overriden
    expect(result.every((v) => v === 0)).toBe(true);
  });

  it("throws error when GEMINI_API_KEY is not set and allowZeroEmbeddings is false", async () => {
    delete process.env.GEMINI_API_KEY;
    process.env.ALLOW_ZERO_EMBEDDINGS = "false";

    const { Embedder: FreshEmbedder } = await import("../src/embedder");

    await expect(FreshEmbedder.getEmbedding("test text")).rejects.toThrow(
      "GEMINI_API_KEY is not set and ALLOW_ZERO_EMBEDDINGS=false"
    );
  });

  it("calls Gemini API and returns embedding when API key is set", async () => {
    process.env.GEMINI_API_KEY = "fake-key";
    process.env.EMBEDDING_MODEL = "gemini-embedding-001";
    process.env.EMBEDDING_DIM = "768";
    const fakeEmbedding = Array(768).fill(0.1);
    
    mockEmbedContent.mockResolvedValue({
      embeddings: [{ values: fakeEmbedding }],
    });

    const { Embedder: FreshEmbedder } = await import("../src/embedder");

    const result = await FreshEmbedder.getEmbedding("test text");
    expect(mockEmbedContent).toHaveBeenCalledTimes(1);
    expect(mockEmbedContent).toHaveBeenCalledWith({
      model: "gemini-embedding-001",
      contents: "test text",
    });
    expect(result).toEqual(fakeEmbedding);
  });

  it("throws error if API returns empty embeddings", async () => {
    process.env.GEMINI_API_KEY = "fake-key";
    mockEmbedContent.mockResolvedValue({ embeddings: [] });

    const { Embedder: FreshEmbedder } = await import("../src/embedder");

    await expect(FreshEmbedder.getEmbedding("test")).rejects.toThrow(
      "No embedding returned from Gemini API"
    );
  });
  
  it("retries on 429 status and succeeds", async () => {
    process.env.GEMINI_API_KEY = "fake-key";
    process.env.EMBEDDING_DIM = "768";
    
    // Fail first time with 429
    mockEmbedContent.mockRejectedValueOnce({ status: 429 });
    
    const fakeEmbedding = Array(768).fill(0.1);
    // Succeed second time
    mockEmbedContent.mockResolvedValueOnce({
      embeddings: [{ values: fakeEmbedding }],
    });

    const { Embedder: FreshEmbedder } = await import("../src/embedder");

    const result = await FreshEmbedder.getEmbedding("test text");
    
    expect(mockEmbedContent).toHaveBeenCalledTimes(2);
    expect(result).toEqual(fakeEmbedding);
  });
});

