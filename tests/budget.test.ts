import { describe, it, expect, vi, beforeEach } from "vitest";

const mockSearch = vi.fn();

vi.mock("meilisearch", () => ({
  Meilisearch: vi.fn().mockImplementation(() => ({
    getStats: vi.fn().mockResolvedValue({ indexes: {} }),
    createIndex: vi.fn().mockResolvedValue({ taskUid: 0 }),
    tasks: { waitForTask: vi.fn().mockResolvedValue({ status: "succeeded" }) },
    deleteIndex: vi.fn().mockResolvedValue({ taskUid: 0 }),
    index: vi.fn().mockReturnValue({
      search: mockSearch,
      getDocument: vi.fn().mockRejectedValue({ code: "document_not_found" }),
      getDocuments: vi.fn().mockResolvedValue({ results: [] }),
      addDocuments: vi.fn().mockResolvedValue({ taskUid: 0 }),
      updateDocuments: vi.fn().mockResolvedValue({ taskUid: 0 }),
      deleteDocument: vi.fn().mockResolvedValue({ taskUid: 0 }),
      deleteAllDocuments: vi.fn().mockResolvedValue({ taskUid: 0 }),
      updateSearchableAttributes: vi.fn().mockResolvedValue({ taskUid: 0 }),
      updateFilterableAttributes: vi.fn().mockResolvedValue({ taskUid: 0 }),
      updateSortableAttributes: vi.fn().mockResolvedValue({ taskUid: 0 }),
      updateEmbedders: vi.fn().mockResolvedValue({ taskUid: 0 }),
      updateSynonyms: vi.fn().mockResolvedValue({ taskUid: 0 }),
    }),
  })),
}));

vi.mock("../src/storage/embedder", () => ({
  Embedder: {
    getEmbedding: vi.fn().mockResolvedValue(Array(3072).fill(0)),
    getEmbeddings: vi.fn().mockResolvedValue([Array(3072).fill(0)]),
  },
}));

vi.mock("../src/llm/llmRouter", () => ({
  decideMemoryAction: vi.fn().mockResolvedValue({ action: "create" }),
  decideContextAction: vi.fn().mockResolvedValue({ action: "create" }),
}));

vi.mock("../src/core/config", async (importOriginal) => {
  const original = await importOriginal<typeof import("../src/core/config")>();
  return {
    ...original,
    config: {
      ...original.config,
      meili: { url: "http://localhost:7700", apiKey: "", synonyms: [], recencyScale: "30d", recencyDecay: 0.5 },
      embedding: { model: "test", dimension: 3072, allowZeroEmbeddings: true },
      geminiApiKey: "test",
      budget: {
        memoryPerProject: 2,
        skillPerProject: 1,
        nodePerProject: 2,
      },
    },
    assertEmbeddingDimension: vi.fn(),
  };
});

import { MeilisearchDB, MEMORIES_INDEX } from "../src/storage/meilisearchDB";
import { ContextManager } from "../src/storage/contextManager";
import type { BudgetExceeded } from "../src/core/types";

describe("countByProject", () => {
  let db: MeilisearchDB;

  beforeEach(() => {
    vi.clearAllMocks();
    db = new MeilisearchDB("http://localhost:7700");
  });

  it("returns count for a project", async () => {
    mockSearch.mockResolvedValue({ hits: [], estimatedTotalHits: 42 });
    const result = await db.countByProject(MEMORIES_INDEX, "my-project");
    expect(result).toBe(42);
  });

  it("returns 0 when no documents match", async () => {
    mockSearch.mockResolvedValue({ hits: [], estimatedTotalHits: 0 });
    const result = await db.countByProject(MEMORIES_INDEX, "empty-project");
    expect(result).toBe(0);
  });
});

describe("Budget enforcement via ContextManager", () => {
  let cm: ContextManager;

  beforeEach(() => {
    vi.clearAllMocks();
    cm = new ContextManager("http://localhost:7700");
  });

  it("blocks memory creation when budget is full", async () => {
    mockSearch.mockResolvedValue({ hits: [], estimatedTotalHits: 2 });
    const result = await cm.addMemory("new fact", "observation", "agent", 5, "test-project", {}, false);
    expect((result as BudgetExceeded).budgetExceeded).toBe(true);
    expect((result as BudgetExceeded).message).toContain("Budget full");
  });

  it("allows memory creation when under budget", async () => {
    mockSearch.mockResolvedValue({ hits: [], estimatedTotalHits: 1 });
    const result = await cm.addMemory("new fact", "observation", "agent", 5, "test-project", {}, false);
    expect((result as BudgetExceeded).budgetExceeded).toBeUndefined();
  });
});
