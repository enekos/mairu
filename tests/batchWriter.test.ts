import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("../src/storage/embedder", () => ({
  Embedder: {
    getEmbeddings: vi.fn().mockImplementation((texts: string[]) =>
      Promise.resolve(texts.map(() => Array(3072).fill(0)))
    ),
  },
}));

const mockBulkIndex = vi.fn().mockResolvedValue({ successful: 2, failed: 0, errors: [] });

vi.mock("@elastic/elasticsearch", () => ({
  Client: vi.fn().mockImplementation(() => ({
    count: vi.fn().mockResolvedValue({ count: 0 }),
    bulk: vi.fn(),
    indices: { exists: vi.fn().mockResolvedValue(true), create: vi.fn(), delete: vi.fn(), putMapping: vi.fn() },
    index: vi.fn(),
    get: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
    search: vi.fn(),
    updateByQuery: vi.fn(),
  })),
  HttpConnection: vi.fn(),
}));

vi.mock("../src/core/config", async (importOriginal) => {
  const original = await importOriginal<typeof import("../src/core/config")>();
  return {
    ...original,
    config: {
      ...original.config,
      embedding: { model: "test", dimension: 3072, allowZeroEmbeddings: true },
      geminiApiKey: "test",
      budget: { memoryPerProject: 0, skillPerProject: 0, nodePerProject: 0 },
    },
    assertEmbeddingDimension: vi.fn(),
  };
});

import { BatchWriter } from "../src/storage/batchWriter";
import { ElasticDB } from "../src/storage/elasticDB";
import { Embedder } from "../src/storage/embedder";

describe("BatchWriter", () => {
  let db: ElasticDB;
  let writer: BatchWriter;

  beforeEach(() => {
    vi.clearAllMocks();
    db = new ElasticDB("http://localhost:9200");
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
        id: "skill_1", project: "p", name: "coding", description: "writes code",
        metadata: {}, ai_intent: null, ai_topics: null, ai_quality_score: null,
        created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
      },
    });

    const results = await writer.flush();
    expect(Embedder.getEmbeddings).toHaveBeenCalled();
    expect(mockBulkIndex).toHaveBeenCalledTimes(1);
    expect(results.successful).toBeGreaterThanOrEqual(0);
  });

  it("flush with empty queue returns zeros", async () => {
    const results = await writer.flush();
    expect(results.successful).toBe(0);
    expect(results.failed).toBe(0);
    expect(mockBulkIndex).not.toHaveBeenCalled();
  });

  it("shutdown flushes remaining ops", async () => {
    writer.enqueue({
      type: "memory",
      data: {
        id: "mem_2", project: "p", content: "shutdown test",
        category: "observation", owner: "agent", importance: 5,
        metadata: {}, ai_intent: null, ai_topics: null, ai_quality_score: null,
        created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
      },
    });

    await writer.shutdown();
    expect(mockBulkIndex).toHaveBeenCalledTimes(1);
  });
});
