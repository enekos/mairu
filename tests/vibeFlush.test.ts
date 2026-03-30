import { describe, it, expect, vi, beforeEach } from "vitest";

const mockGenerateContent = vi.fn();

vi.mock("@google/genai", () => ({
  GoogleGenAI: vi.fn().mockImplementation(() => ({
    models: { generateContent: mockGenerateContent },
  })),
}));

vi.mock("../src/core/config", async (importOriginal) => {
  const original = await importOriginal<typeof import("../src/core/config")>();
  return {
    ...original,
    config: {
      ...original.config,
      geminiApiKey: "test-key",
      llmModel: "test-model",
      embedding: { model: "test", dimension: 3072, allowZeroEmbeddings: true },
    },
  };
});

const mockSearchMemories = vi.fn().mockResolvedValue([]);
const mockSearchSkills = vi.fn().mockResolvedValue([]);
const mockSearchContext = vi.fn().mockResolvedValue([]);

const mockCm = {
  searchMemories: mockSearchMemories,
  searchSkills: mockSearchSkills,
  searchContext: mockSearchContext,
} as any;

import { planFlush, summarizeSearchResults } from "../src/llm/vibeEngine";

describe("planFlush", () => {
  beforeEach(() => vi.clearAllMocks());

  it("returns a VibeMutationPlan from a transcript", async () => {
    mockGenerateContent
      .mockResolvedValueOnce({
        text: JSON.stringify({
          reasoning: "searching context",
          queries: [{ store: "memory", query: "test" }],
        }),
      })
      .mockResolvedValueOnce({
        text: JSON.stringify({
          reasoning: "Found user preference to save",
          operations: [
            {
              op: "create_memory",
              description: "User prefers dark mode",
              data: { content: "User prefers dark mode", category: "preferences", owner: "user", importance: 8 },
            },
          ],
        }),
      });

    const plan = await planFlush(mockCm, "User said they prefer dark mode for all interfaces.", "test-project");
    expect(plan.operations).toHaveLength(1);
    expect(plan.operations[0].op).toBe("create_memory");
    expect(plan.reasoning).toBeTruthy();
  });

  it("returns empty operations when nothing worth saving", async () => {
    mockGenerateContent
      .mockResolvedValueOnce({
        text: JSON.stringify({ reasoning: "searching", queries: [] }),
      })
      .mockResolvedValueOnce({
        text: JSON.stringify({ reasoning: "Nothing durable found", operations: [] }),
      });

    const plan = await planFlush(mockCm, "I ran the tests and they passed.", "test-project");
    expect(plan.operations).toHaveLength(0);
  });
});

describe("summarizeSearchResults", () => {
  beforeEach(() => vi.clearAllMocks());

  it("returns a summary and sources", async () => {
    mockGenerateContent.mockResolvedValueOnce({
      text: JSON.stringify({
        summary: "The auth module uses JWT tokens with RSA signatures.",
        sources: [{ store: "memory", id: "mem_1", snippet: "JWT with RSA" }],
      }),
    });

    const result = await summarizeSearchResults("how does auth work?", [
      { store: "memory", items: [{ id: "mem_1", content: "Auth uses JWT with RSA signatures", _score: 5.2 }] },
    ]);

    expect(result.summary).toContain("JWT");
    expect(result.sources).toHaveLength(1);
  });

  it("handles empty search results", async () => {
    mockGenerateContent.mockResolvedValueOnce({
      text: JSON.stringify({
        summary: "No relevant information found.",
        sources: [],
      }),
    });

    const result = await summarizeSearchResults("unknown topic", []);
    expect(result.summary).toBeTruthy();
    expect(result.sources).toHaveLength(0);
  });
});
