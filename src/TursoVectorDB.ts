import { createClient, Client } from "@libsql/client";
import {
  AgentSkill,
  AgentMemory,
  AgentContextNode,
  MemorySearchOptions,
  SkillSearchOptions,
  ContextSearchOptions,
  HybridSearchWeights,
} from "./types";
import { assertEmbeddingDimension, getEmbeddingConfig } from "./embeddingConfig";

const EMBEDDING_DIM = getEmbeddingConfig().dimension;
const SKILLS_TABLE = "agent_skills";
const MEMORIES_TABLE = "agent_memories";
const CONTEXT_TABLE = "agent_context_nodes";
const DEFAULT_SKILL_WEIGHTS: HybridSearchWeights = { vector: 0.8, keyword: 0.2, recency: 0 };
const DEFAULT_MEMORY_WEIGHTS: HybridSearchWeights = {
  vector: 0.65,
  keyword: 0.15,
  importance: 0.15,
  recency: 0.05,
};
const DEFAULT_CONTEXT_WEIGHTS: HybridSearchWeights = { vector: 0.8, keyword: 0.2, recency: 0 };

export class TursoVectorDB {
  private client: Client;

  constructor(url: string, authToken?: string) {
    this.client = createClient({
      url,
      authToken,
    });
  }

  // Initialize the database tables with F32_BLOB for vectors
  async initTables() {
    await this.client.executeMultiple(`
      CREATE TABLE IF NOT EXISTS ${SKILLS_TABLE} (
        id TEXT PRIMARY KEY,
        name TEXT NOT NULL,
        description TEXT NOT NULL,
        embedding F32_BLOB(${EMBEDDING_DIM}),
        metadata TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
      );

      CREATE INDEX IF NOT EXISTS idx_skills_vec ON ${SKILLS_TABLE}(libsql_vector_idx(embedding));

      CREATE TABLE IF NOT EXISTS ${MEMORIES_TABLE} (
        id TEXT PRIMARY KEY,
        content TEXT NOT NULL,
        category TEXT NOT NULL,
        owner TEXT NOT NULL,
        importance INTEGER DEFAULT 1,
        embedding F32_BLOB(${EMBEDDING_DIM}),
        metadata TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
      );

      CREATE INDEX IF NOT EXISTS idx_memories_vec ON ${MEMORIES_TABLE}(libsql_vector_idx(embedding));

      CREATE TABLE IF NOT EXISTS ${CONTEXT_TABLE} (
        uri TEXT PRIMARY KEY,
        parent_uri TEXT,
        name TEXT NOT NULL,
        abstract TEXT NOT NULL,
        overview TEXT,
        content TEXT,
        embedding F32_BLOB(${EMBEDDING_DIM}),
        metadata TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        FOREIGN KEY (parent_uri) REFERENCES ${CONTEXT_TABLE}(uri) ON DELETE CASCADE
      );

      CREATE INDEX IF NOT EXISTS idx_context_nodes_vec ON ${CONTEXT_TABLE}(libsql_vector_idx(embedding));
    `);
  }

  async resetTables() {
    await this.client.executeMultiple(`
      DROP INDEX IF EXISTS idx_skills_vec_v3;
      DROP INDEX IF EXISTS idx_memories_vec_v3;
      DROP INDEX IF EXISTS idx_context_nodes_vec_v3;
      DROP INDEX IF EXISTS idx_skills_vec;
      DROP INDEX IF EXISTS idx_memories_vec;
      DROP INDEX IF EXISTS idx_context_nodes_vec;

      DROP TABLE IF EXISTS agent_context_nodes_v4;
      DROP TABLE IF EXISTS agent_memories_v4;
      DROP TABLE IF EXISTS agent_skills_v4;

      DROP TABLE IF EXISTS ${CONTEXT_TABLE};
      DROP TABLE IF EXISTS ${MEMORIES_TABLE};
      DROP TABLE IF EXISTS ${SKILLS_TABLE};
    `);
  }

  // Helper to convert array of numbers into standard JSON for passing to libsql vector functions
  private floatArrayToVector(arr: number[]): string {
    assertEmbeddingDimension(arr, "TursoVectorDB.floatArrayToVector");
    return `[${arr.join(",")}]`;
  }

  private parseMetadata(raw: unknown): Record<string, any> | null {
    if (!raw) return null;
    try {
      return JSON.parse(raw as string);
    } catch {
      return null;
    }
  }

  private resolveWeights(defaults: HybridSearchWeights, weights?: HybridSearchWeights): Required<HybridSearchWeights> {
    const merged = {
      vector: weights?.vector ?? defaults.vector,
      keyword: weights?.keyword ?? defaults.keyword,
      recency: weights?.recency ?? defaults.recency ?? 0,
      importance: weights?.importance ?? defaults.importance ?? 0,
    };
    const total = merged.vector + merged.keyword + merged.recency + merged.importance;
    if (total <= 0) {
      throw new Error("Hybrid search weights must sum to a positive number.");
    }
    return {
      vector: merged.vector / total,
      keyword: merged.keyword / total,
      recency: merged.recency / total,
      importance: merged.importance / total,
    };
  }

  // --- Skills ---

  async addSkill(skill: AgentSkill, embedding: number[]) {
    await this.client.execute({
      sql: `
        INSERT INTO ${SKILLS_TABLE} (id, name, description, embedding, metadata, created_at)
        VALUES (?, ?, ?, vector(?), ?, ?)
      `,
      args: [
        skill.id,
        skill.name,
        skill.description,
        this.floatArrayToVector(embedding),
        skill.metadata ? JSON.stringify(skill.metadata) : null,
        skill.created_at || new Date().toISOString(),
      ],
    });
  }

  async searchSkills(query: string, queryEmbedding: number[], options: SkillSearchOptions = {}) {
    const topK = options.topK ?? 5;
    const weights = this.resolveWeights(DEFAULT_SKILL_WEIGHTS, options.weights);
    const res = await this.client.execute({
      sql: `
        SELECT id, name, description, metadata, created_at,
               vector_distance_cos(embedding, vector(?)) as distance,
               CASE WHEN lower(name || ' ' || description) LIKE '%' || lower(?) || '%' THEN 1.0 ELSE 0.0 END AS keyword_score,
               CASE
                 WHEN created_at IS NULL THEN 0.0
                 ELSE 1.0 / (1.0 + MAX(0.0, julianday('now') - julianday(created_at)))
               END AS recency_score,
               ((1.0 - vector_distance_cos(embedding, vector(?))) * ?
                 + CASE WHEN lower(name || ' ' || description) LIKE '%' || lower(?) || '%' THEN 1.0 ELSE 0.0 END * ?
                 + CASE
                     WHEN created_at IS NULL THEN 0.0
                     ELSE 1.0 / (1.0 + MAX(0.0, julianday('now') - julianday(created_at)))
                   END * ?) AS hybrid_score
        FROM ${SKILLS_TABLE}
        WHERE (? IS NULL OR julianday(created_at) >= julianday('now') - ?)
        ORDER BY hybrid_score DESC
        LIMIT ?
      `,
      args: [
        this.floatArrayToVector(queryEmbedding),
        query,
        this.floatArrayToVector(queryEmbedding),
        weights.vector,
        query,
        weights.keyword,
        weights.recency,
        options.maxAgeDays ?? null,
        options.maxAgeDays ?? null,
        topK,
      ],
    });

    const rows = res.rows.map((row) => ({
      ...row,
      metadata: this.parseMetadata(row.metadata),
    }));
    return options.threshold !== undefined
      ? rows.filter((r: any) => (r.distance as number) <= options.threshold!)
      : rows;
  }

  async listSkills(limit: number = 50) {
    const res = await this.client.execute({
      sql: `SELECT id, name, description, metadata, created_at FROM ${SKILLS_TABLE} ORDER BY created_at DESC LIMIT ?`,
      args: [limit],
    });
    return res.rows.map((row) => ({
      ...row,
      metadata: this.parseMetadata(row.metadata),
    }));
  }

  async deleteSkill(id: string) {
    await this.client.execute({ sql: `DELETE FROM ${SKILLS_TABLE} WHERE id = ?`, args: [id] });
  }

  // --- Memories ---

  async addMemory(memory: AgentMemory, embedding: number[]) {
    await this.client.execute({
      sql: `
        INSERT INTO ${MEMORIES_TABLE} (id, content, category, owner, importance, embedding, metadata, created_at)
        VALUES (?, ?, ?, ?, ?, vector(?), ?, ?)
      `,
      args: [
        memory.id,
        memory.content,
        memory.category,
        memory.owner,
        memory.importance,
        this.floatArrayToVector(embedding),
        memory.metadata ? JSON.stringify(memory.metadata) : null,
        memory.created_at || new Date().toISOString(),
      ],
    });
  }

  async searchMemories(query: string, queryEmbedding: number[], options: MemorySearchOptions = {}) {
    const topK = options.topK ?? 5;
    const weights = this.resolveWeights(DEFAULT_MEMORY_WEIGHTS, options.weights);
    const res = await this.client.execute({
      sql: `
        SELECT id, content, category, owner, importance, metadata, created_at,
               vector_distance_cos(embedding, vector(?)) as distance,
               CASE WHEN lower(content) LIKE '%' || lower(?) || '%' THEN 1.0 ELSE 0.0 END AS keyword_score,
               MIN(1.0, MAX(0.0, importance / 10.0)) AS importance_score,
               CASE
                 WHEN created_at IS NULL THEN 0.0
                 ELSE 1.0 / (1.0 + MAX(0.0, julianday('now') - julianday(created_at)))
               END AS recency_score,
               ((1.0 - vector_distance_cos(embedding, vector(?))) * ?
                 + CASE WHEN lower(content) LIKE '%' || lower(?) || '%' THEN 1.0 ELSE 0.0 END * ?
                 + MIN(1.0, MAX(0.0, importance / 10.0)) * ?
                 + CASE
                     WHEN created_at IS NULL THEN 0.0
                     ELSE 1.0 / (1.0 + MAX(0.0, julianday('now') - julianday(created_at)))
                   END * ?) AS hybrid_score
        FROM ${MEMORIES_TABLE}
        WHERE (? IS NULL OR owner = ?)
          AND (? IS NULL OR category = ?)
          AND (? IS NULL OR importance >= ?)
          AND (? IS NULL OR julianday(created_at) >= julianday('now') - ?)
        ORDER BY hybrid_score DESC
        LIMIT ?
      `,
      args: [
        this.floatArrayToVector(queryEmbedding),
        query,
        this.floatArrayToVector(queryEmbedding),
        weights.vector,
        query,
        weights.keyword,
        weights.importance,
        weights.recency,
        options.owner ?? null,
        options.owner ?? null,
        options.category ?? null,
        options.category ?? null,
        options.minImportance ?? null,
        options.minImportance ?? null,
        options.maxAgeDays ?? null,
        options.maxAgeDays ?? null,
        topK,
      ],
    });

    const rows = res.rows.map((row) => ({
      ...row,
      metadata: this.parseMetadata(row.metadata),
    }));
    return options.threshold !== undefined
      ? rows.filter((r: any) => (r.distance as number) <= options.threshold!)
      : rows;
  }

  async listMemories(limit: number = 50) {
    const res = await this.client.execute({
      sql: `SELECT id, content, category, owner, importance, metadata, created_at FROM ${MEMORIES_TABLE} ORDER BY created_at DESC LIMIT ?`,
      args: [limit],
    });
    return res.rows.map((row) => ({
      ...row,
      metadata: this.parseMetadata(row.metadata),
    }));
  }

  async deleteMemory(id: string) {
    await this.client.execute({ sql: `DELETE FROM ${MEMORIES_TABLE} WHERE id = ?`, args: [id] });
  }

  // --- Hierarchical Context ---

  async addContextNode(node: AgentContextNode, embedding: number[]) {
    await this.client.execute({
      sql: `
        INSERT INTO ${CONTEXT_TABLE} (uri, parent_uri, name, abstract, overview, content, embedding, metadata, created_at)
        VALUES (?, ?, ?, ?, ?, ?, vector(?), ?, ?)
      `,
      args: [
        node.uri,
        node.parent_uri,
        node.name,
        node.abstract,
        node.overview || null,
        node.content || null,
        this.floatArrayToVector(embedding),
        node.metadata ? JSON.stringify(node.metadata) : null,
        node.created_at || new Date().toISOString(),
      ],
    });
  }

  async searchContextNodes(query: string, queryEmbedding: number[], options: ContextSearchOptions = {}) {
    const topK = options.topK ?? 5;
    const weights = this.resolveWeights(DEFAULT_CONTEXT_WEIGHTS, options.weights);
    const res = await this.client.execute({
      sql: `
        SELECT uri, parent_uri, name, abstract, overview, content, metadata, created_at,
               vector_distance_cos(embedding, vector(?)) as distance,
               CASE
                 WHEN lower(name || ' ' || abstract || ' ' || COALESCE(overview, '')) LIKE '%' || lower(?) || '%'
                 THEN 1.0
                 ELSE 0.0
               END AS keyword_score,
               CASE
                 WHEN created_at IS NULL THEN 0.0
                 ELSE 1.0 / (1.0 + MAX(0.0, julianday('now') - julianday(created_at)))
               END AS recency_score,
               ((1.0 - vector_distance_cos(embedding, vector(?))) * ?
                 + CASE
                     WHEN lower(name || ' ' || abstract || ' ' || COALESCE(overview, '')) LIKE '%' || lower(?) || '%'
                     THEN 1.0
                     ELSE 0.0
                   END * ?
                 + CASE
                     WHEN created_at IS NULL THEN 0.0
                     ELSE 1.0 / (1.0 + MAX(0.0, julianday('now') - julianday(created_at)))
                   END * ?) AS hybrid_score
        FROM ${CONTEXT_TABLE}
        WHERE (? IS NULL OR parent_uri = ?)
          AND (? IS NULL OR julianday(created_at) >= julianday('now') - ?)
        ORDER BY hybrid_score DESC
        LIMIT ?
      `,
      args: [
        this.floatArrayToVector(queryEmbedding),
        query,
        this.floatArrayToVector(queryEmbedding),
        weights.vector,
        query,
        weights.keyword,
        weights.recency,
        options.parentUri ?? null,
        options.parentUri ?? null,
        options.maxAgeDays ?? null,
        options.maxAgeDays ?? null,
        topK,
      ],
    });

    const rows = res.rows.map((row) => ({
      ...row,
      metadata: this.parseMetadata(row.metadata),
    }));
    return options.threshold !== undefined
      ? rows.filter((r: any) => (r.distance as number) <= options.threshold!)
      : rows;
  }

  async listContextNodes(parentUri?: string, limit: number = 50) {
    const res = parentUri
      ? await this.client.execute({
          sql: `SELECT uri, parent_uri, name, abstract, overview, metadata, created_at FROM ${CONTEXT_TABLE} WHERE parent_uri = ? ORDER BY created_at DESC LIMIT ?`,
          args: [parentUri, limit],
        })
      : await this.client.execute({
          sql: `SELECT uri, parent_uri, name, abstract, overview, metadata, created_at FROM ${CONTEXT_TABLE} ORDER BY created_at DESC LIMIT ?`,
          args: [limit],
        });
    return res.rows.map((row) => ({
      ...row,
      metadata: this.parseMetadata(row.metadata),
    }));
  }

  async deleteContextNode(uri: string) {
    await this.client.execute({
      sql: `DELETE FROM ${CONTEXT_TABLE} WHERE uri = ?`,
      args: [uri],
    });
  }

  // Get a specific node and all of its deep descendants (Recursive)
  async getContextSubtree(nodeId: string) {
    const res = await this.client.execute({
      sql: `
        WITH RECURSIVE subtree AS (
          -- Base case: the root node of our subtree
          SELECT uri, parent_uri, name, abstract, overview, content, metadata, created_at, 0 as level
          FROM ${CONTEXT_TABLE}
          WHERE uri = ?
          
          UNION ALL
          
          -- Recursive step: find all children of nodes currently in the subtree
          SELECT a.uri, a.parent_uri, a.name, a.abstract, a.overview, a.content, a.metadata, a.created_at, s.level + 1
          FROM ${CONTEXT_TABLE} a
          JOIN subtree s ON a.parent_uri = s.uri
        )
        SELECT * FROM subtree ORDER BY level ASC;
      `,
      args: [nodeId],
    });

    return res.rows.map((row) => ({
      ...row,
      metadata: this.parseMetadata(row.metadata),
    }));
  }

  // Get the path from a specific node up to the root (Recursive)
  async getContextPath(nodeId: string) {
    const res = await this.client.execute({
      sql: `
        WITH RECURSIVE ancestor_path AS (
          -- Base case: the specific node
          SELECT uri, parent_uri, name, abstract, overview, content, metadata, created_at, 0 as level
          FROM ${CONTEXT_TABLE}
          WHERE uri = ?
          
          UNION ALL
          
          -- Recursive step: find the parent
          SELECT a.uri, a.parent_uri, a.name, a.abstract, a.overview, a.content, a.metadata, a.created_at, p.level - 1
          FROM ${CONTEXT_TABLE} a
          JOIN ancestor_path p ON p.parent_uri = a.uri
        )
        SELECT * FROM ancestor_path ORDER BY level ASC;
      `,
      args: [nodeId],
    });

    return res.rows.map((row) => ({
      ...row,
      metadata: this.parseMetadata(row.metadata),
    }));
  }
}
