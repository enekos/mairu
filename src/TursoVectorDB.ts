import { createClient, Client } from "@libsql/client";
import {
  AgentSkill,
  AgentMemory,
  AgentContextNode,
  MemorySearchOptions,
  SkillSearchOptions,
  ContextSearchOptions,
} from "./types";
import { assertEmbeddingDimension, getEmbeddingConfig } from "./embeddingConfig";

const EMBEDDING_DIM = getEmbeddingConfig().dimension;
const SKILLS_TABLE = "agent_skills";
const MEMORIES_TABLE = "agent_memories";
const CONTEXT_TABLE = "agent_context_nodes";

// Multiplier for how many vector candidates to fetch before application-side re-ranking
const CANDIDATE_MULTIPLIER = Number(process.env.CANDIDATE_MULTIPLIER || "4");

export class TursoVectorDB {
  private client: Client;

  constructor(url: string, authToken?: string) {
    this.client = createClient({ url, authToken });
  }

  async initTables() {
    await this.client.executeMultiple(`
      CREATE TABLE IF NOT EXISTS ${SKILLS_TABLE} (
        id TEXT PRIMARY KEY,
        project TEXT,
        name TEXT NOT NULL,
        description TEXT NOT NULL,
        embedding F32_BLOB(${EMBEDDING_DIM}),
        metadata TEXT,
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL
      );
      CREATE INDEX IF NOT EXISTS idx_skills_vec ON ${SKILLS_TABLE}(libsql_vector_idx(embedding));

      CREATE TABLE IF NOT EXISTS ${MEMORIES_TABLE} (
        id TEXT PRIMARY KEY,
        project TEXT,
        content TEXT NOT NULL,
        category TEXT NOT NULL,
        owner TEXT NOT NULL,
        importance INTEGER DEFAULT 1,
        embedding F32_BLOB(${EMBEDDING_DIM}),
        metadata TEXT,
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL
      );
      CREATE INDEX IF NOT EXISTS idx_memories_vec ON ${MEMORIES_TABLE}(libsql_vector_idx(embedding));

      CREATE TABLE IF NOT EXISTS ${CONTEXT_TABLE} (
        uri TEXT PRIMARY KEY,
        project TEXT,
        parent_uri TEXT,
        name TEXT NOT NULL,
        abstract TEXT NOT NULL,
        overview TEXT,
        content TEXT,
        embedding F32_BLOB(${EMBEDDING_DIM}),
        metadata TEXT,
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL,
        FOREIGN KEY (parent_uri) REFERENCES ${CONTEXT_TABLE}(uri) ON DELETE CASCADE
      );
      CREATE INDEX IF NOT EXISTS idx_context_nodes_vec ON ${CONTEXT_TABLE}(libsql_vector_idx(embedding));
    `);
  }

  async resetTables() {
    await this.client.executeMultiple(`
      DROP INDEX IF EXISTS idx_skills_vec;
      DROP INDEX IF EXISTS idx_memories_vec;
      DROP INDEX IF EXISTS idx_context_nodes_vec;
      DROP TABLE IF EXISTS ${CONTEXT_TABLE};
      DROP TABLE IF EXISTS ${MEMORIES_TABLE};
      DROP TABLE IF EXISTS ${SKILLS_TABLE};
    `);
  }

  private vec(arr: number[]): string {
    assertEmbeddingDimension(arr, "TursoVectorDB.vec");
    return `[${arr.join(",")}]`;
  }

  private parseMeta(raw: unknown): Record<string, any> | null {
    if (!raw) return null;
    try {
      return JSON.parse(raw as string);
    } catch {
      return null;
    }
  }

  private now(): string {
    return new Date().toISOString();
  }

  // ---------------------------------------------------------------------------
  // Skills
  // ---------------------------------------------------------------------------

  async addSkill(skill: AgentSkill, embedding: number[]) {
    const ts = this.now();
    await this.client.execute({
      sql: `INSERT INTO ${SKILLS_TABLE} (id, project, name, description, embedding, metadata, created_at, updated_at)
            VALUES (?, ?, ?, ?, vector(?), ?, ?, ?)`,
      args: [
        skill.id,
        skill.project || null,
        skill.name,
        skill.description,
        this.vec(embedding),
        skill.metadata ? JSON.stringify(skill.metadata) : null,
        skill.created_at || ts,
        skill.updated_at || ts,
      ],
    });
  }

  async updateSkill(id: string, updates: { name?: string; description?: string; metadata?: Record<string, any> }, embedding?: number[]) {
    const sets: string[] = ["updated_at = ?"];
    const args: any[] = [this.now()];

    if (updates.name !== undefined) { sets.push("name = ?"); args.push(updates.name); }
    if (updates.description !== undefined) { sets.push("description = ?"); args.push(updates.description); }
    if (updates.metadata !== undefined) { sets.push("metadata = ?"); args.push(JSON.stringify(updates.metadata)); }
    if (embedding) { sets.push("embedding = vector(?)"); args.push(this.vec(embedding)); }

    args.push(id);
    await this.client.execute({
      sql: `UPDATE ${SKILLS_TABLE} SET ${sets.join(", ")} WHERE id = ?`,
      args,
    });
  }

  /**
   * Fetch broad vector candidates. Application code does the final re-ranking.
   * Returns rows with a `distance` field (cosine distance, lower = more similar).
   */
  async searchSkills(queryEmbedding: number[], options: SkillSearchOptions = {}) {
    const topK = options.topK ?? 10;
    const limit = topK * CANDIDATE_MULTIPLIER;
    const res = await this.client.execute({
      sql: `SELECT id, project, name, description, metadata, created_at, updated_at,
                   vector_distance_cos(embedding, vector(?)) as distance
            FROM ${SKILLS_TABLE}
            WHERE (? IS NULL OR project = ?)
              AND (? IS NULL OR julianday(created_at) >= julianday('now') - ?)
            ORDER BY distance ASC
            LIMIT ?`,
      args: [
        this.vec(queryEmbedding),
        options.project ?? null, options.project ?? null,
        options.maxAgeDays ?? null,
        options.maxAgeDays ?? null,
        limit,
      ],
    });
    return res.rows.map((r) => ({ ...r, metadata: this.parseMeta(r.metadata) }));
  }

  async listSkills(options?: SkillSearchOptions, limit = 100, offset = 0) {
    const res = await this.client.execute({
      sql: `SELECT id, project, name, description, metadata, created_at, updated_at FROM ${SKILLS_TABLE}
            WHERE (? IS NULL OR project = ?)
            ORDER BY updated_at DESC LIMIT ? OFFSET ?`,
      args: [options?.project ?? null, options?.project ?? null, limit, offset],
    });
    return res.rows.map((r) => ({ ...r, metadata: this.parseMeta(r.metadata) }));
  }

  async getSkill(id: string) {
    const res = await this.client.execute({ sql: `SELECT id, project, name, description, metadata, created_at, updated_at FROM ${SKILLS_TABLE} WHERE id = ?`, args: [id] });
    if (res.rows.length === 0) return null;
    return { ...res.rows[0], metadata: this.parseMeta(res.rows[0].metadata) };
  }

  async deleteSkill(id: string) {
    await this.client.execute({ sql: `DELETE FROM ${SKILLS_TABLE} WHERE id = ?`, args: [id] });
  }

  // ---------------------------------------------------------------------------
  // Memories
  // ---------------------------------------------------------------------------

  async addMemory(memory: AgentMemory, embedding: number[]) {
    const ts = this.now();
    await this.client.execute({
      sql: `INSERT INTO ${MEMORIES_TABLE} (id, project, content, category, owner, importance, embedding, metadata, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, vector(?), ?, ?, ?)`,
      args: [
        memory.id,
        memory.project || null,
        memory.content,
        memory.category,
        memory.owner,
        memory.importance,
        this.vec(embedding),
        memory.metadata ? JSON.stringify(memory.metadata) : null,
        memory.created_at || ts,
        memory.updated_at || ts,
      ],
    });
  }

  async updateMemory(
    id: string,
    updates: { content?: string; importance?: number; metadata?: Record<string, any> },
    embedding?: number[]
  ) {
    const sets: string[] = ["updated_at = ?"];
    const args: any[] = [this.now()];

    if (updates.content !== undefined) { sets.push("content = ?"); args.push(updates.content); }
    if (updates.importance !== undefined) { sets.push("importance = ?"); args.push(updates.importance); }
    if (updates.metadata !== undefined) { sets.push("metadata = ?"); args.push(JSON.stringify(updates.metadata)); }
    if (embedding) { sets.push("embedding = vector(?)"); args.push(this.vec(embedding)); }

    args.push(id);
    await this.client.execute({
      sql: `UPDATE ${MEMORIES_TABLE} SET ${sets.join(", ")} WHERE id = ?`,
      args,
    });
  }

  async searchMemories(queryEmbedding: number[], options: MemorySearchOptions = {}) {
    const topK = options.topK ?? 10;
    const limit = topK * CANDIDATE_MULTIPLIER;
    const res = await this.client.execute({
      sql: `SELECT id, project, content, category, owner, importance, metadata, created_at, updated_at,
                   vector_distance_cos(embedding, vector(?)) as distance
            FROM ${MEMORIES_TABLE}
            WHERE (? IS NULL OR project = ?)
              AND (? IS NULL OR owner = ?)
              AND (? IS NULL OR category = ?)
              AND (? IS NULL OR importance >= ?)
              AND (? IS NULL OR julianday(created_at) >= julianday('now') - ?)
            ORDER BY distance ASC
            LIMIT ?`,
      args: [
        this.vec(queryEmbedding),
        options.project ?? null, options.project ?? null,
        options.owner ?? null, options.owner ?? null,
        options.category ?? null, options.category ?? null,
        options.minImportance ?? null, options.minImportance ?? null,
        options.maxAgeDays ?? null, options.maxAgeDays ?? null,
        limit,
      ],
    });
    return res.rows.map((r) => ({ ...r, metadata: this.parseMeta(r.metadata) }));
  }

  async listMemories(options?: MemorySearchOptions, limit = 100, offset = 0) {
    const res = await this.client.execute({
      sql: `SELECT id, project, content, category, owner, importance, metadata, created_at, updated_at FROM ${MEMORIES_TABLE}
            WHERE (? IS NULL OR project = ?)
            ORDER BY updated_at DESC LIMIT ?`,
      args: [options?.project ?? null, options?.project ?? null, limit, offset],
    });
    return res.rows.map((r) => ({ ...r, metadata: this.parseMeta(r.metadata) }));
  }

  async getMemory(id: string) {
    const res = await this.client.execute({ sql: `SELECT id, project, content, category, owner, importance, metadata, created_at, updated_at FROM ${MEMORIES_TABLE} WHERE id = ?`, args: [id] });
    if (res.rows.length === 0) return null;
    return { ...res.rows[0], metadata: this.parseMeta(res.rows[0].metadata) };
  }

  async deleteMemory(id: string) {
    await this.client.execute({ sql: `DELETE FROM ${MEMORIES_TABLE} WHERE id = ?`, args: [id] });
  }

  // ---------------------------------------------------------------------------
  // Context Nodes
  // ---------------------------------------------------------------------------

  async addContextNode(node: AgentContextNode, embedding: number[]) {
    const ts = this.now();
    await this.client.execute({
      sql: `INSERT INTO ${CONTEXT_TABLE} (uri, project, parent_uri, name, abstract, overview, content, embedding, metadata, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, vector(?), ?, ?, ?)`,
      args: [
        node.uri,
        node.project || null,
        node.parent_uri,
        node.name,
        node.abstract,
        node.overview || null,
        node.content || null,
        this.vec(embedding),
        node.metadata ? JSON.stringify(node.metadata) : null,
        node.created_at || ts,
        node.updated_at || ts,
      ],
    });
  }

  async updateContextNode(
    uri: string,
    updates: { name?: string; abstract?: string; overview?: string; content?: string; metadata?: Record<string, any> },
    embedding?: number[]
  ) {
    const sets: string[] = ["updated_at = ?"];
    const args: any[] = [this.now()];

    if (updates.name !== undefined) { sets.push("name = ?"); args.push(updates.name); }
    if (updates.abstract !== undefined) { sets.push("abstract = ?"); args.push(updates.abstract); }
    if (updates.overview !== undefined) { sets.push("overview = ?"); args.push(updates.overview); }
    if (updates.content !== undefined) { sets.push("content = ?"); args.push(updates.content); }
    if (updates.metadata !== undefined) { sets.push("metadata = ?"); args.push(JSON.stringify(updates.metadata)); }
    if (embedding) { sets.push("embedding = vector(?)"); args.push(this.vec(embedding)); }

    args.push(uri);
    await this.client.execute({
      sql: `UPDATE ${CONTEXT_TABLE} SET ${sets.join(", ")} WHERE uri = ?`,
      args,
    });
  }

  async searchContextNodes(queryEmbedding: number[], options: ContextSearchOptions = {}) {
    const topK = options.topK ?? 10;
    const limit = topK * CANDIDATE_MULTIPLIER;
    const res = await this.client.execute({
      sql: `SELECT uri, project, parent_uri, name, abstract, overview, content, metadata, created_at, updated_at,
                   vector_distance_cos(embedding, vector(?)) as distance
            FROM ${CONTEXT_TABLE}
            WHERE (? IS NULL OR project = ?)
              AND (? IS NULL OR parent_uri = ?)
              AND (? IS NULL OR julianday(created_at) >= julianday('now') - ?)
            ORDER BY distance ASC
            LIMIT ?`,
      args: [
        this.vec(queryEmbedding),
        options.project ?? null, options.project ?? null,
        options.parentUri ?? null, options.parentUri ?? null,
        options.maxAgeDays ?? null, options.maxAgeDays ?? null,
        limit,
      ],
    });
    return res.rows.map((r) => ({ ...r, metadata: this.parseMeta(r.metadata) }));
  }

  async listContextNodes(parentUri?: string, options?: ContextSearchOptions, limit = 100, offset = 0) {
    const res = parentUri
      ? await this.client.execute({
          sql: `SELECT uri, project, parent_uri, name, abstract, overview, metadata, created_at, updated_at FROM ${CONTEXT_TABLE} WHERE parent_uri = ? AND (? IS NULL OR project = ?) ORDER BY updated_at DESC LIMIT ?`,
          args: [parentUri, options?.project ?? null, options?.project ?? null, limit, offset],
        })
      : await this.client.execute({
          sql: `SELECT uri, project, parent_uri, name, abstract, overview, metadata, created_at, updated_at FROM ${CONTEXT_TABLE} WHERE (? IS NULL OR project = ?) ORDER BY updated_at DESC LIMIT ?`,
          args: [options?.project ?? null, options?.project ?? null, limit, offset],
        });
    return res.rows.map((r) => ({ ...r, metadata: this.parseMeta(r.metadata) }));
  }

  async getContextNode(uri: string) {
    const res = await this.client.execute({ sql: `SELECT uri, project, parent_uri, name, abstract, overview, content, metadata, created_at, updated_at FROM ${CONTEXT_TABLE} WHERE uri = ?`, args: [uri] });
    if (res.rows.length === 0) return null;
    return { ...res.rows[0], metadata: this.parseMeta(res.rows[0].metadata) };
  }

  async deleteContextNode(uri: string) {
    await this.client.execute({ sql: `DELETE FROM ${CONTEXT_TABLE} WHERE uri = ?`, args: [uri] });
  }

  async getContextSubtree(nodeUri: string) {
    const res = await this.client.execute({
      sql: `WITH RECURSIVE subtree AS (
              SELECT uri, project, parent_uri, name, abstract, overview, content, metadata, created_at, updated_at, 0 as depth
              FROM ${CONTEXT_TABLE} WHERE uri = ?
              UNION ALL
              SELECT c.uri, c.project, c.parent_uri, c.name, c.abstract, c.overview, c.content, c.metadata, c.created_at, c.updated_at, s.depth + 1
              FROM ${CONTEXT_TABLE} c JOIN subtree s ON c.parent_uri = s.uri
            )
            SELECT * FROM subtree ORDER BY depth ASC`,
      args: [nodeUri],
    });
    return res.rows.map((r) => ({ ...r, metadata: this.parseMeta(r.metadata) }));
  }

  async getContextPath(nodeUri: string) {
    const res = await this.client.execute({
      sql: `WITH RECURSIVE ancestors AS (
              SELECT uri, project, parent_uri, name, abstract, overview, content, metadata, created_at, updated_at, 0 as depth
              FROM ${CONTEXT_TABLE} WHERE uri = ?
              UNION ALL
              SELECT c.uri, c.project, c.parent_uri, c.name, c.abstract, c.overview, c.content, c.metadata, c.created_at, c.updated_at, a.depth - 1
              FROM ${CONTEXT_TABLE} c JOIN ancestors a ON a.parent_uri = c.uri
            )
            SELECT * FROM ancestors ORDER BY depth ASC`,
      args: [nodeUri],
    });
    return res.rows.map((r) => ({ ...r, metadata: this.parseMeta(r.metadata) }));
  }
}
