import { Embedder } from "./embedder";
import { ElasticDB, MEMORIES_INDEX, SKILLS_INDEX, CONTEXT_INDEX } from "./elasticDB";

export interface BatchWriterOptions {
  batchSize?: number;
  flushIntervalMs?: number;
}

export interface BatchOp {
  type: "memory" | "skill" | "node";
  data: Record<string, any>;
}

export interface BatchResult {
  successful: number;
  failed: number;
  errors: Array<{ id: string; error: string }>;
}

const INDEX_MAP: Record<string, string> = {
  memory: MEMORIES_INDEX,
  skill: SKILLS_INDEX,
  node: CONTEXT_INDEX,
};

function getEmbedText(op: BatchOp): string {
  switch (op.type) {
    case "memory":
      return op.data.content;
    case "skill":
      return `${op.data.name}: ${op.data.description}`;
    case "node":
      return `${op.data.name}: ${op.data.abstract}`;
  }
}

function getId(op: BatchOp): string {
  return op.type === "node" ? op.data.uri : op.data.id;
}

export class BatchWriter {
  private db: ElasticDB;
  private queue: BatchOp[] = [];
  private readonly batchSize: number;
  private flushTimer: ReturnType<typeof setInterval> | null = null;
  private readonly flushIntervalMs: number;

  constructor(db: ElasticDB, options: BatchWriterOptions = {}) {
    this.db = db;
    this.batchSize = options.batchSize ?? 10;
    this.flushIntervalMs = options.flushIntervalMs ?? 2000;
  }

  enqueue(op: BatchOp): void {
    this.queue.push(op);
    if (this.queue.length >= this.batchSize) {
      this.flush().catch((err) =>
        console.error("[BatchWriter] auto-flush error:", err)
      );
    }
  }

  async flush(): Promise<BatchResult> {
    if (this.queue.length === 0) {
      return { successful: 0, failed: 0, errors: [] };
    }

    const batch = this.queue.splice(0);
    const texts = batch.map(getEmbedText);
    const embeddings = await Embedder.getEmbeddings(texts);

    const bulkOps = batch.map((op, i) => ({
      index: INDEX_MAP[op.type],
      id: getId(op),
      body: { ...op.data, embedding: embeddings[i] },
    }));

    return this.db.bulkIndex(bulkOps);
  }

  async shutdown(): Promise<void> {
    if (this.flushTimer) {
      clearInterval(this.flushTimer);
      this.flushTimer = null;
    }
    if (this.queue.length > 0) {
      await this.flush();
    }
  }
}
