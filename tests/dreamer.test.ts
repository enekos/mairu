import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("dotenv", () => ({ config: vi.fn() }));

const mockGenerateContent = vi.fn();
vi.mock("@google/genai", () => ({
  GoogleGenAI: vi.fn().mockImplementation(() => ({
    models: { generateContent: mockGenerateContent },
  })),
}));

// Mock MeilisearchDB at module level
const mockListMemories = vi.fn();
const mockSearchMemoriesByVector = vi.fn();
const mockUpdateMemory = vi.fn();
const mockDeleteMemory = vi.fn();
const mockGetMemory = vi.fn();
const mockAddMemory = vi.fn();
const mockListContextNodes = vi.fn();
const mockSearchContextNodesByVector = vi.fn();
const mockUpdateContextNode = vi.fn();
const mockDeleteContextNode = vi.fn();
const mockGetContextNode = vi.fn();
const mockAddContextNode = vi.fn();
const mockGetContextSubtree = vi.fn();

vi.mock("../src/storage/meilisearchDB", () => ({
  MeilisearchDB: vi.fn().mockImplementation(() => ({
    listMemories: mockListMemories,
    searchMemoriesByVector: mockSearchMemoriesByVector,
    updateMemory: mockUpdateMemory,
    deleteMemory: mockDeleteMemory,
    getMemory: mockGetMemory,
    addMemory: mockAddMemory,
    listContextNodes: mockListContextNodes,
    searchContextNodesByVector: mockSearchContextNodesByVector,
    updateContextNode: mockUpdateContextNode,
    deleteContextNode: mockDeleteContextNode,
    getContextNode: mockGetContextNode,
    addContextNode: mockAddContextNode,
    getContextSubtree: mockGetContextSubtree,
  })),
}));

const mockGetEmbedding = vi.fn().mockResolvedValue(new Array(3072).fill(0));
vi.mock("../src/storage/embedder", () => ({
  Embedder: { getEmbedding: (...args: any[]) => mockGetEmbedding(...args) },
}));

describe("dreamer types", () => {
  it("accepts derived_pattern as a valid MemoryCategory", async () => {
    // TypeScript compilation is the test — if this compiles, the type exists
    const category: import("../src/core/types").MemoryCategory = "derived_pattern";
    expect(category).toBe("derived_pattern");
  });
});

describe("deductionPass", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    process.env.GEMINI_API_KEY = "fake-key";
  });

  it("merges duplicate memories when LLM says MERGE", async () => {
    const mem1 = {
      id: "mem_1", content: "We use Vitest for testing",
      category: "decision", owner: "agent", importance: 5,
      created_at: "2026-03-01T00:00:00Z", project: "proj",
    };
    const mem2 = {
      id: "mem_2", content: "Testing framework is Vitest",
      category: "decision", owner: "agent", importance: 5,
      created_at: "2026-03-15T00:00:00Z", project: "proj",
    };

    // listMemories returns both memories
    mockListMemories.mockResolvedValueOnce([mem1, mem2]).mockResolvedValueOnce([]);

    // For mem1, vector search returns mem2 as similar
    mockSearchMemoriesByVector.mockResolvedValueOnce([
      { ...mem2, _score: 0.92 },
    ]);
    // For mem2 — already processed, won't be searched

    // LLM decides to merge
    mockGenerateContent.mockResolvedValueOnce({
      text: JSON.stringify({
        decisions: [{ candidateIndex: 0, action: "MERGE", mergedContent: "We use Vitest as our testing framework" }],
      }),
    });

    const { deductionPass } = await import("../src/dreamer");
    await deductionPass("proj");

    // Should update the newer memory with merged content
    expect(mockUpdateMemory).toHaveBeenCalledWith("mem_2", {
      content: "We use Vitest as our testing framework",
    }, expect.any(Array));
    // Should delete the older memory
    expect(mockDeleteMemory).toHaveBeenCalledWith("mem_1");
  });

  it("resolves contradictions by keeping the newer memory", async () => {
    const mem1 = {
      id: "mem_1", content: "We use Jest for testing",
      category: "decision", owner: "agent", importance: 5,
      created_at: "2026-01-01T00:00:00Z", project: "proj",
    };
    const mem2 = {
      id: "mem_2", content: "We migrated from Jest to Vitest",
      category: "decision", owner: "agent", importance: 5,
      created_at: "2026-03-15T00:00:00Z", project: "proj",
    };

    mockListMemories.mockResolvedValueOnce([mem1, mem2]).mockResolvedValueOnce([]);
    mockSearchMemoriesByVector.mockResolvedValueOnce([
      { ...mem2, _score: 0.88 },
    ]);

    mockGenerateContent.mockResolvedValueOnce({
      text: JSON.stringify({
        decisions: [{ candidateIndex: 0, action: "CONTRADICTION" }],
      }),
    });

    const { deductionPass } = await import("../src/dreamer");
    await deductionPass("proj");

    // Should delete the older (source) memory
    expect(mockDeleteMemory).toHaveBeenCalledWith("mem_1");
    // Should NOT delete or modify the newer one
    expect(mockUpdateMemory).not.toHaveBeenCalled();
  });

  it("skips memories with no near-duplicates", async () => {
    const mem1 = {
      id: "mem_1", content: "Auth uses JWT tokens",
      category: "architecture", owner: "agent", importance: 7,
      created_at: "2026-03-01T00:00:00Z", project: "proj",
    };

    mockListMemories.mockResolvedValueOnce([mem1]).mockResolvedValueOnce([]);
    mockSearchMemoriesByVector.mockResolvedValueOnce([]); // no similar memories

    const { deductionPass } = await import("../src/dreamer");
    await deductionPass("proj");

    expect(mockGenerateContent).not.toHaveBeenCalled();
    expect(mockUpdateMemory).not.toHaveBeenCalled();
    expect(mockDeleteMemory).not.toHaveBeenCalled();
  });
});

describe("inductionPass", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    process.env.GEMINI_API_KEY = "fake-key";
  });

  it("creates a derived_pattern from a cluster of 3+ related memories", async () => {
    const memories = [
      { id: "mem_1", content: "Use early returns in auth handlers", category: "decision", owner: "agent", importance: 5, created_at: "2026-03-01T00:00:00Z", project: "proj" },
      { id: "mem_2", content: "Prefer early returns in validation logic", category: "decision", owner: "agent", importance: 6, created_at: "2026-03-05T00:00:00Z", project: "proj" },
      { id: "mem_3", content: "Always use early returns for error handling", category: "decision", owner: "agent", importance: 5, created_at: "2026-03-10T00:00:00Z", project: "proj" },
    ];

    // First call: listMemories returns all 3 (non-pattern memories)
    mockListMemories
      .mockResolvedValueOnce(memories)
      .mockResolvedValueOnce([]);

    // Clustering: mem_1 search returns mem_2 and mem_3 as similar
    mockSearchMemoriesByVector
      .mockResolvedValueOnce([
        { ...memories[1], _score: 0.82 },
        { ...memories[2], _score: 0.80 },
      ])
      // Dedup check for new pattern: no existing pattern matches
      .mockResolvedValueOnce([]);

    // LLM synthesizes a pattern
    mockGenerateContent.mockResolvedValueOnce({
      text: JSON.stringify({
        pattern: "Team consistently prefers early returns for error/validation paths across all handlers",
        confidence: "high",
        evidence: "Observed in auth handlers, validation logic, and error handling",
      }),
    });

    const { inductionPass } = await import("../src/dreamer");
    await inductionPass("proj");

    expect(mockAddMemory).toHaveBeenCalledWith(
      expect.objectContaining({
        content: expect.stringContaining("early returns"),
        category: "derived_pattern",
        owner: "system",
        importance: expect.any(Number),
        project: "proj",
      }),
      expect.any(Array), // embedding
    );
  });

  it("skips clusters smaller than 3 memories", async () => {
    const memories = [
      { id: "mem_1", content: "Use TypeScript strict mode", category: "decision", owner: "agent", importance: 5, created_at: "2026-03-01T00:00:00Z", project: "proj" },
      { id: "mem_2", content: "Enable strict TypeScript", category: "decision", owner: "agent", importance: 5, created_at: "2026-03-05T00:00:00Z", project: "proj" },
    ];

    mockListMemories.mockResolvedValueOnce(memories).mockResolvedValueOnce([]);
    mockSearchMemoriesByVector.mockResolvedValueOnce([
      { ...memories[1], _score: 0.82 },
    ]);

    const { inductionPass } = await import("../src/dreamer");
    await inductionPass("proj");

    expect(mockGenerateContent).not.toHaveBeenCalled();
    expect(mockAddMemory).not.toHaveBeenCalled();
  });

  it("skips if LLM returns null pattern", async () => {
    const memories = [
      { id: "m1", content: "A", category: "observation", owner: "agent", importance: 3, created_at: "2026-03-01T00:00:00Z", project: "proj" },
      { id: "m2", content: "B", category: "observation", owner: "agent", importance: 3, created_at: "2026-03-02T00:00:00Z", project: "proj" },
      { id: "m3", content: "C", category: "observation", owner: "agent", importance: 3, created_at: "2026-03-03T00:00:00Z", project: "proj" },
    ];

    mockListMemories.mockResolvedValueOnce(memories).mockResolvedValueOnce([]);
    mockSearchMemoriesByVector.mockResolvedValueOnce([
      { ...memories[1], _score: 0.80 },
      { ...memories[2], _score: 0.78 },
    ]);

    mockGenerateContent.mockResolvedValueOnce({
      text: JSON.stringify({ pattern: null, confidence: null, evidence: null }),
    });

    const { inductionPass } = await import("../src/dreamer");
    await inductionPass("proj");

    expect(mockAddMemory).not.toHaveBeenCalled();
  });
});
