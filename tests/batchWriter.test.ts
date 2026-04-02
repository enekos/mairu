import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("../src/storage/embedder", () => ({
  Embedder: {
    getEmbeddings: vi.fn().mockImplementation((texts: string[]) =>
      Promise.resolve(texts.map(() => Array(3072).fill(0)))
    ),
  },
}));

const mockBulkIndex = vi.fn().mockResolvedValue({ successful: 2, failed: 0, errors: [] });

vi.mock("meilisearch", () => ({
  Meilisearch: vi.fn().mockImplementation(() => ({
    getStats: vi.fn().mockResolvedValue({ indexes: {} }),
    createIndex: vi.fn().mockResolvedValue({ taskUid: 0 }),
    tasks: { waitForTask: vi.fn().mockResolvedValue({ status: "succeeded" }) },
    deleteIndex: vi.fn().mockResolvedValue({ taskUid: 0 }),
    index: vi.fn().mockReturnValue({
      search: vi.fn().mockResolvedValue({ hits: [], estimatedTotalHits: 0 }),
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

vi.mock("../src/core/config", async (importOriginal) => {
  const original = await importOriginal<typeof import("../src/core/config")>();
  return {
    ...original,
    config: {
      ...original.config,
      meili: { url: "http://localhost:7700", apiKey: "", synonyms: [], recencyScale: "30d", recencyDecay: 0.5 },
      embedding: { model: "test", dimension: 3072, allowZeroEmbeddings: true },
      geminiApiKey: "test",
      budget: { memoryPerProject: 0, skillPerProject: 0, nodePerProject: 0 },
    },
    assertEmbeddingDimension: vi.fn(),
  };
});

import { BatchWriter } from "../src/storage/batchWriter";
import { MeilisearchDB } from "../src/storage/meilisearchDB";
import { Embedder } from "../src/storage/embedder";

describe("BatchWriter", () => {
  let db: MeilisearchDB;
  let writer: BatchWriter;

  beforeEach(() => {
    vi.clearAllMocks();
    db = new MeilisearchDB("http://localhost:7700");
    db.bulkIndex = mockBulkIndex;
    writer = new BatchWriter(db, { batchSize: 3, flushIntervalMs: 50000 });
  });

  it("enqueue does not immediately write", () => {
    writer.enqueue({
      type: "memory",
      data: {
        id: "mem_1", project: "p", content: "test",
        category: "observation", owner: "agent", importance: 5,
        metadata: {}, ai_intent: null, ai_topics: null, ai_quality_score: null,
        created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
      },
    });
    expect(mockBulkIndex).not.toHaveBeenCalled();
  });

  it("flush writes all queued ops", async () => {
    writer.enqueue({
      type: "memory",
      data: {
        id: "mem_1", project: "p", content: "hello",
        category: "observation", owner: "agent", importance: 5,
        metadata: {}, ai_intent: null, ai_topics: null, ai_quality_score: null,
        created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
      },
    });
    writer.enqueue({
      type: "skill",
      data: {
        id: "skill_1", project: "p", name: "code", description: "write code",
        metadata: {}, ai_intent: null, ai_topics: null, ai_quality_score: null,
        created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
      },
    });

    const result = await writer.flush();
    expect(mockBulkIndex).toHaveBeenCalledTimes(1);
    expect(result.successful).toBe(2);
  });
});
