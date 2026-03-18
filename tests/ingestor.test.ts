import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Mock `dotenv`
vi.mock("dotenv", () => {
  return {
    config: vi.fn(),
  };
});

// Mock `@google/genai`
const mockGenerateContent = vi.fn();
vi.mock("@google/genai", () => {
  return {
    GoogleGenAI: vi.fn().mockImplementation(() => {
      return {
        models: {
          generateContent: mockGenerateContent,
        },
      };
    }),
  };
});

// Mock `console.warn`
const mockWarn = vi.spyOn(console, "warn").mockImplementation(() => {});

describe("ingestor", () => {
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
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  it("throws an error if AI is not initialized", async () => {
    delete process.env.GEMINI_API_KEY;
    const { parseTextIntoContextNodes } = await import("../src/ingestor");
    
    await expect(parseTextIntoContextNodes("some text")).rejects.toThrow(
      "GEMINI_API_KEY is not set — cannot parse text into context nodes."
    );
  });

  it("throws an error if text is too large", async () => {
    process.env.GEMINI_API_KEY = "fake-key";
    const { parseTextIntoContextNodes } = await import("../src/ingestor");
    
    const hugeText = "a".repeat(100_001);
    await expect(parseTextIntoContextNodes(hugeText)).rejects.toThrow(
      /Input text is too large/
    );
  });

  it("returns parsed context nodes on success", async () => {
    process.env.GEMINI_API_KEY = "fake-key";
    mockGenerateContent.mockResolvedValue({
      text: `[
        {
          "uri": "contextfs://test/1",
          "name": "Node 1",
          "abstract": "Abstract 1",
          "parent_uri": null
        },
        {
          "uri": "contextfs://test/1/child",
          "name": "Child Node",
          "abstract": "Child Abstract",
          "overview": "Overview text",
          "content": "Content text",
          "parent_uri": "contextfs://test/1"
        }
      ]`,
    });
    
    const { parseTextIntoContextNodes } = await import("../src/ingestor");
    const result = await parseTextIntoContextNodes("test text", "contextfs://test");
    
    expect(mockGenerateContent).toHaveBeenCalledTimes(1);
    expect(result).toHaveLength(2);
    
    expect(result[0]).toEqual({
      uri: "contextfs://test/1",
      name: "Node 1",
      abstract: "Abstract 1",
      overview: undefined,
      content: undefined,
      parent_uri: null,
    });
    
    expect(result[1]).toEqual({
      uri: "contextfs://test/1/child",
      name: "Child Node",
      abstract: "Child Abstract",
      overview: "Overview text",
      content: "Content text",
      parent_uri: "contextfs://test/1",
    });
  });

  it("throws error if LLM returns non-array JSON", async () => {
    process.env.GEMINI_API_KEY = "fake-key";
    mockGenerateContent.mockResolvedValue({
      text: '{"not": "an array"}',
    });
    
    const { parseTextIntoContextNodes } = await import("../src/ingestor");
    await expect(parseTextIntoContextNodes("test text")).rejects.toThrow(
      /LLM returned unparseable output/
    );
  });

  it("throws error if parsed nodes are empty/invalid", async () => {
    process.env.GEMINI_API_KEY = "fake-key";
    mockGenerateContent.mockResolvedValue({
      text: '[{"invalid": "node missing required fields"}]',
    });
    
    const { parseTextIntoContextNodes } = await import("../src/ingestor");
    await expect(parseTextIntoContextNodes("test text")).rejects.toThrow(
      /LLM returned no valid context nodes/
    );
  });

  it("retries on 429 status and succeeds", async () => {
    process.env.GEMINI_API_KEY = "fake-key";
    
    mockGenerateContent.mockRejectedValueOnce({ status: 429 });
    mockGenerateContent.mockResolvedValueOnce({
      text: `[
        {
          "uri": "contextfs://test/retry",
          "name": "Retry Node",
          "abstract": "It worked on retry",
          "parent_uri": null
        }
      ]`,
    });
    
    const { parseTextIntoContextNodes } = await import("../src/ingestor");
    const result = await parseTextIntoContextNodes("test text");
    
    expect(mockGenerateContent).toHaveBeenCalledTimes(2);
    expect(result).toHaveLength(1);
    expect(result[0].name).toBe("Retry Node");
  });
});
