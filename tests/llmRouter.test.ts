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

describe("llmRouter", () => {
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

  describe("decideMemoryAction", () => {
    it("returns 'create' if AI is not initialized", async () => {
      delete process.env.GEMINI_API_KEY;
      const { decideMemoryAction } = await import("../src/llmRouter");
      
      const result = await decideMemoryAction("new content", [{ id: "1", content: "old", score: 0.9 }]);
      expect(result).toEqual({ action: "create" });
    });

    it("returns 'create' if no candidates meet SIMILARITY_GATE", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      const { decideMemoryAction } = await import("../src/llmRouter");
      
      const result = await decideMemoryAction("new content", [{ id: "1", content: "old", score: 0.5 }]);
      expect(result).toEqual({ action: "create" });
      expect(mockGenerateContent).not.toHaveBeenCalled();
    });

    it("returns parsed action on success", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      mockGenerateContent.mockResolvedValue({
        text: '{"action":"update","targetId":"123","mergedContent":"new and old"}',
      });
      const { decideMemoryAction } = await import("../src/llmRouter");
      
      const result = await decideMemoryAction("new content", [{ id: "123", content: "old", score: 0.8 }]);
      expect(result).toEqual({ action: "update", targetId: "123", mergedContent: "new and old" });
      expect(mockGenerateContent).toHaveBeenCalledTimes(1);
    });

    it("returns 'create' on invalid JSON", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      mockGenerateContent.mockResolvedValue({ text: "not json at all" });
      const { decideMemoryAction } = await import("../src/llmRouter");
      
      const result = await decideMemoryAction("new content", [{ id: "1", content: "old", score: 0.8 }]);
      expect(result).toEqual({ action: "create" });
    });

    it("returns 'create' on missing fields for update", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      mockGenerateContent.mockResolvedValue({
        text: '{"action":"update","targetId":"1"}', // missing mergedContent
      });
      const { decideMemoryAction } = await import("../src/llmRouter");
      
      const result = await decideMemoryAction("new content", [{ id: "1", content: "old", score: 0.8 }]);
      expect(result).toEqual({ action: "create" });
    });

    it("returns 'skip' on valid skip JSON", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      mockGenerateContent.mockResolvedValue({
        text: '{"action":"skip","reason":"already there"}',
      });
      const { decideMemoryAction } = await import("../src/llmRouter");
      
      const result = await decideMemoryAction("new content", [{ id: "1", content: "old", score: 0.8 }]);
      expect(result).toEqual({ action: "skip", reason: "already there" });
    });

    it("returns 'create' on API failure without throwing", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      mockGenerateContent.mockRejectedValue(new Error("API Down"));
      const { decideMemoryAction } = await import("../src/llmRouter");
      
      const result = await decideMemoryAction("new content", [{ id: "1", content: "old", score: 0.8 }]);
      expect(result).toEqual({ action: "create" });
      expect(mockWarn).toHaveBeenCalledWith("[llmRouter] decideMemoryAction failed, defaulting to create:", expect.any(Error));
    });
    
    it("retries on 429 status and succeeds", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      
      mockGenerateContent.mockRejectedValueOnce({ status: 429 });
      mockGenerateContent.mockResolvedValueOnce({
        text: '{"action":"skip","reason":"retry worked"}',
      });
      
      const { decideMemoryAction } = await import("../src/llmRouter");
      
      const result = await decideMemoryAction("new content", [{ id: "1", content: "old", score: 0.8 }]);
      
      expect(mockGenerateContent).toHaveBeenCalledTimes(2);
      expect(result).toEqual({ action: "skip", reason: "retry worked" });
    });

    it("returns 'create' when decision action is 'create'", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      mockGenerateContent.mockResolvedValue({
        text: '{"action":"create"}',
      });
      const { decideMemoryAction } = await import("../src/llmRouter");
      
      const result = await decideMemoryAction("new content", [{ id: "1", content: "old", score: 0.8 }]);
      expect(result).toEqual({ action: "create" });
    });
    
    it("returns 'create' on unknown action types", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      mockGenerateContent.mockResolvedValue({
        text: '{"action":"unknown_action"}',
      });
      const { decideMemoryAction } = await import("../src/llmRouter");
      
      const result = await decideMemoryAction("new content", [{ id: "1", content: "old", score: 0.8 }]);
      expect(result).toEqual({ action: "create" });
    });
  });

  describe("decideContextAction", () => {
    it("returns 'create' if AI is not initialized", async () => {
      delete process.env.GEMINI_API_KEY;
      const { decideContextAction } = await import("../src/llmRouter");
      
      const result = await decideContextAction("uri1", "name", "abstract", [{ id: "1", content: "old", score: 0.9 }]);
      expect(result).toEqual({ action: "create" });
    });

    it("returns 'create' if no candidates meet SIMILARITY_GATE", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      const { decideContextAction } = await import("../src/llmRouter");
      
      const result = await decideContextAction("uri1", "name", "abstract", [{ id: "1", content: "old", score: 0.5 }]);
      expect(result).toEqual({ action: "create" });
      expect(mockGenerateContent).not.toHaveBeenCalled();
    });

    it("returns parsed update action on success", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      mockGenerateContent.mockResolvedValue({
        text: '{"action":"update","targetId":"uri1","mergedContent":"merged"}',
      });
      const { decideContextAction } = await import("../src/llmRouter");
      
      const result = await decideContextAction("uri1", "name", "abstract", [{ id: "1", content: "old", score: 0.8 }]);
      expect(result).toEqual({ action: "update", targetId: "uri1", mergedContent: "merged" });
      expect(mockGenerateContent).toHaveBeenCalledTimes(1);
    });

    it("returns parsed skip action on success", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      mockGenerateContent.mockResolvedValue({
        text: '{"action":"skip","reason":"dup"}',
      });
      const { decideContextAction } = await import("../src/llmRouter");
      
      const result = await decideContextAction("uri1", "name", "abstract", [{ id: "1", content: "old", score: 0.8 }]);
      expect(result).toEqual({ action: "skip", reason: "dup" });
    });

    it("returns 'create' on invalid JSON", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      mockGenerateContent.mockResolvedValue({ text: "not json" });
      const { decideContextAction } = await import("../src/llmRouter");
      
      const result = await decideContextAction("uri1", "name", "abstract", [{ id: "1", content: "old", score: 0.8 }]);
      expect(result).toEqual({ action: "create" });
    });

    it("returns 'create' on API failure without throwing", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      mockGenerateContent.mockRejectedValue(new Error("API Down"));
      const { decideContextAction } = await import("../src/llmRouter");
      
      const result = await decideContextAction("uri1", "name", "abstract", [{ id: "1", content: "old", score: 0.8 }]);
      expect(result).toEqual({ action: "create" });
      expect(mockWarn).toHaveBeenCalledWith("[llmRouter] decideContextAction failed, defaulting to create:", expect.any(Error));
    });
  });
});
