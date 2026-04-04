// @ts-ignore
import { Meilisearch, Index } from "meilisearch";
import {
  AgentSkill,
  AgentMemory,
  AgentContextNode,
  MemorySearchOptions,
  MemoryCategory,
  SkillSearchOptions,
  ContextSearchOptions,
} from "../core/types";
import { assertEmbeddingDimension, config } from "../core/config";
import {
  DEFAULT_MEMORY_WEIGHTS,
  DEFAULT_SKILL_WEIGHTS,
  DEFAULT_CONTEXT_WEIGHTS,
  normalizeWeights,
} from "./scorer";

const EMBEDDING_DIM = config.embedding.dimension;
const CANDIDATE_MULTIPLIER = config.candidateMultiplier;
const AI_QUALITY_FUNCTION_WEIGHT = 2;

export const SKILLS_INDEX = "contextfs_skills";
export const MEMORIES_INDEX = "contextfs_memories";
export const CONTEXT_INDEX = "contextfs_context_nodes";
export const SYMBOLS_INDEX = "contextfs_symbols";

/** Parse a duration string like "30d", "7d", "90d" to milliseconds. */
function parseDurationMs(duration: string): number {
  const match = duration.match(/^(\d+)([dhms])$/);
  if (!match) return 30 * 24 * 60 * 60 * 1000; // default 30d
  const value = parseInt(match[1], 10);
  const unit = match[2];
  switch (unit) {
    case "d": return value * 24 * 60 * 60 * 1000;
    case "h": return value * 60 * 60 * 1000;
    case "m": return value * 60 * 1000;
    case "s": return value * 1000;
    default: return 30 * 24 * 60 * 60 * 1000;
  }
}

export class MeilisearchDB {
  private client: Meilisearch;
  private initialized = false;

  constructor(url: string, apiKey?: string) {
    this.client = new Meilisearch({ host: url, apiKey: apiKey || undefined });
  }

  private async ensureInitialized() {
    if (this.initialized) return;
    try {
      await this.initIndices();
      this.initialized = true;
    } catch (e: any) {
      if (e?.code === "ECONNREFUSED" || e?.type === "MeiliSearchCommunicationError") {
        console.error("❌ Meilisearch connection failed. Is Docker running? (Run: docker compose up -d)");
        process.exit(1);
      }
      throw e;
    }
  }

  async initIndices() {
    // Create indexes (no-op if they already exist)
    const indexes = [
      { uid: SKILLS_INDEX, primaryKey: "id" },
      { uid: MEMORIES_INDEX, primaryKey: "id" },
      { uid: CONTEXT_INDEX, primaryKey: "id" },
      { uid: SYMBOLS_INDEX, primaryKey: "id" },
    ];

    for (const idx of indexes) {
      const task = await this.client.createIndex(idx.uid, { primaryKey: idx.primaryKey });
      await this.client.tasks.waitForTask(task.taskUid);
    }

    // Configure skills index
    const skillsIndex = this.client.index(SKILLS_INDEX);
    await this.waitForSettings(skillsIndex, {
      searchableAttributes: ["name", "description"],
      filterableAttributes: ["project", "ai_intent", "ai_topics", "created_at", "updated_at"],
      sortableAttributes: ["updated_at", "created_at"],
    });

    // Configure memories index
    const memoriesIndex = this.client.index(MEMORIES_INDEX);
    await this.waitForSettings(memoriesIndex, {
      searchableAttributes: ["content"],
      filterableAttributes: ["project", "category", "owner", "importance", "ai_intent", "ai_topics", "created_at", "updated_at"],
      sortableAttributes: ["updated_at", "created_at", "importance"],
    });

    // Configure context nodes index
    const contextIndex = this.client.index(CONTEXT_INDEX);
    await this.waitForSettings(contextIndex, {
      searchableAttributes: ["name", "abstract", "overview", "content"],
      filterableAttributes: ["project", "uri", "parent_uri", "ancestors", "is_deleted", "ai_intent", "ai_topics", "created_at", "updated_at"],
      sortableAttributes: ["updated_at", "created_at"],
    });

    // Configure symbols index
    const symbolsIndex = this.client.index(SYMBOLS_INDEX);
    await this.waitForSettings(symbolsIndex, {
      searchableAttributes: ["name", "kind", "file_path"],
      filterableAttributes: ["file_path", "kind"],
    });

    // Configure embedders on all indexes
    for (const uid of [SKILLS_INDEX, MEMORIES_INDEX, CONTEXT_INDEX]) {
      const index = this.client.index(uid);
      const task = await index.updateEmbedders({
        default: { source: "userProvided", dimensions: EMBEDDING_DIM },
      } as any);
      await this.client.tasks.waitForTask(task.taskUid);
    }

    // Push synonyms
    const synonymGroups = config.meili.synonyms;
    if (synonymGroups.length > 0) {
      const synonymMap: Record<string, string[]> = {};
      for (const group of synonymGroups) {
        const words = group.split(",").map((w) => w.trim()).filter(Boolean);
        for (const word of words) {
          synonymMap[word] = words.filter((w) => w !== word);
        }
      }
      for (const uid of [SKILLS_INDEX, MEMORIES_INDEX, CONTEXT_INDEX]) {
        const task = await this.client.index(uid).updateSynonyms(synonymMap);
        await this.client.tasks.waitForTask(task.taskUid);
      }
    }
  }

  private async waitForSettings(index: Index, settings: {
    searchableAttributes?: string[];
    filterableAttributes?: string[];
    sortableAttributes?: string[];
  }) {
    if (settings.searchableAttributes) {
      const task = await index.updateSearchableAttributes(settings.searchableAttributes);
      await this.client.tasks.waitForTask(task.taskUid);
    }
    if (settings.filterableAttributes) {
      const task = await index.updateFilterableAttributes(settings.filterableAttributes);
      await this.client.tasks.waitForTask(task.taskUid);
    }
    if (settings.sortableAttributes) {
      const task = await index.updateSortableAttributes(settings.sortableAttributes);
      await this.client.tasks.waitForTask(task.taskUid);
    }
  }

  async resetIndices() {
    for (const uid of [SKILLS_INDEX, MEMORIES_INDEX, CONTEXT_INDEX]) {
      try {
        const task = await this.client.deleteIndex(uid);
        await this.client.tasks.waitForTask(task.taskUid);
      } catch (e: any) {
        if ((e?.code !== "index_not_found" && e?.cause?.code !== "index_not_found")) throw e;
      }
    }
    this.initialized = false;
  }

  /** Cluster/instance stats for the dashboard */
  async getClusterStats() {
    await this.ensureInitialized();
    const stats = await this.client.getStats();

    const indices: Record<string, any> = {};
    for (const uid of [SKILLS_INDEX, MEMORIES_INDEX, CONTEXT_INDEX]) {
      const s = stats.indexes[uid];
      indices[uid] = {
        docs: s?.numberOfDocuments ?? 0,
        deletedDocs: 0,
        sizeBytes: 0,
      };
    }

    return {
      clusterName: "meilisearch",
      status: "green",
      numberOfNodes: 1,
      activeShards: 0,
      relocatingShards: 0,
      unassignedShards: 0,
      indices,
    };
  }

  async countByProject(index: string, project: string): Promise<number> {
    await this.ensureInitialized();
    const idx = this.client.index(index);
    const res = await idx.search("", {
      filter: `project = "${this.escapeFilterValue(project)}"`,
      limit: 0,
    });
    return res.estimatedTotalHits ?? 0;
  }

  async bulkIndex(ops: Array<{ index: string; id: string; body: object }>): Promise<{
    successful: number;
    failed: number;
    errors: Array<{ id: string; error: string }>;
  }> {
    if (ops.length === 0) return { successful: 0, failed: 0, errors: [] };

    // Group by index
    const byIndex = new Map<string, Array<Record<string, any>>>();
    for (const op of ops) {
      const docs = byIndex.get(op.index) ?? [];
      docs.push({ ...op.body });
      byIndex.set(op.index, docs);
    }

    const errors: Array<{ id: string; error: string }> = [];
    let successful = 0;

    for (const [indexUid, docs] of byIndex) {
      const index = this.client.index(indexUid);
      const task = await index.addDocuments(docs);
      const result = await this.client.tasks.waitForTask(task.taskUid);
      if (result.status === "succeeded") {
        successful += docs.length;
      } else {
        for (const doc of docs) {
          errors.push({ id: doc.id || doc.uri || "unknown", error: result.error?.message || "Unknown error" });
        }
      }
    }

    return { successful, failed: errors.length, errors };
  }

  // ---------------------------------------------------------------------------
  // Skills
  // ---------------------------------------------------------------------------

  async addSkill(skill: AgentSkill, embedding: number[]) {
    await this.ensureInitialized();
    assertEmbeddingDimension(embedding, "MeilisearchDB.addSkill");
    const ts = new Date().toISOString();
    const index = this.client.index(SKILLS_INDEX);
    const doc = {
      id: skill.id,
      project: skill.project || null,
      name: skill.name,
      description: skill.description,
      ai_intent: skill.ai_intent ?? null,
      ai_topics: skill.ai_topics ?? null,
      ai_quality_score: skill.ai_quality_score ?? null,
      _vectors: { default: embedding },
      metadata: skill.metadata || null,
      created_at: skill.created_at || ts,
      updated_at: skill.updated_at || ts,
    };
    const task = await index.addDocuments([doc]);
    await this.client.tasks.waitForTask(task.taskUid);
  }

  async updateSkill(
    id: string,
    updates: {
      name?: string;
      description?: string;
      ai_intent?: AgentSkill["ai_intent"];
      ai_topics?: AgentSkill["ai_topics"];
      ai_quality_score?: AgentSkill["ai_quality_score"];
      metadata?: Record<string, any>;
    },
    embedding?: number[]
  ) {
    await this.ensureInitialized();
    const doc: Record<string, any> = { id, updated_at: new Date().toISOString() };
    if (updates.name !== undefined) doc.name = updates.name;
    if (updates.description !== undefined) doc.description = updates.description;
    if (updates.ai_intent !== undefined) doc.ai_intent = updates.ai_intent;
    if (updates.ai_topics !== undefined) doc.ai_topics = updates.ai_topics;
    if (updates.ai_quality_score !== undefined) doc.ai_quality_score = updates.ai_quality_score;
    if (updates.metadata !== undefined) doc.metadata = updates.metadata;
    if (embedding) {
      assertEmbeddingDimension(embedding, "MeilisearchDB.updateSkill");
      doc._vectors = { default: embedding };
    }
    const index = this.client.index(SKILLS_INDEX);
    const task = await index.updateDocuments([doc]);
    await this.client.tasks.waitForTask(task.taskUid);
  }

  async searchSkills(
    queryEmbedding: number[],
    queryText: string,
    options: SkillSearchOptions = {}
  ): Promise<(AgentSkill & { _score: number; _highlight?: Record<string, string[]> })[]> {
    await this.ensureInitialized();
    assertEmbeddingDimension(queryEmbedding, "MeilisearchDB.searchSkills");
    const topK = options.topK ?? 10;
    const ow = options.weights ?? DEFAULT_SKILL_WEIGHTS;
    const w = normalizeWeights({
      vector: ow.vector, keyword: ow.keyword,
      recency: ow.recency ?? 0, importance: 0,
    });

    const filters = this.buildSkillFilters(options);
    const vectorKeywordSum = w.vector + w.keyword;
    const semanticRatio = vectorKeywordSum > 0 ? w.vector / vectorKeywordSum : 0.5;
    const fetchLimit = topK * CANDIDATE_MULTIPLIER;

    const searchParams: any = {
      vector: queryEmbedding,
      hybrid: { semanticRatio, embedder: "default" },
      filter: filters.length > 0 ? filters.join(" AND ") : undefined,
      limit: fetchLimit,
      showRankingScore: true,
    };

    if (options.highlight) {
      searchParams.attributesToHighlight = ["name", "description"];
      searchParams.highlightPreTag = "<mark>";
      searchParams.highlightPostTag = "</mark>";
    }

    if (options.minScore != null) {
      searchParams.rankingScoreThreshold = options.minScore;
    }

    const index = this.client.index(SKILLS_INDEX);
    const res = await index.search(queryText, searchParams);

    return this.rerankAndMap<AgentSkill>(res.hits, w, topK, options);
  }

  async searchSkillsByVector(
    queryEmbedding: number[],
    options: { topK?: number; project?: string } = {}
  ): Promise<(AgentSkill & { _score: number })[]> {
    await this.ensureInitialized();
    assertEmbeddingDimension(queryEmbedding, "MeilisearchDB.searchSkillsByVector");
    const topK = options.topK ?? 10;
    const filters: string[] = [];
    if (options.project) filters.push(`project = "${this.escapeFilterValue(options.project)}"`);

    const index = this.client.index(SKILLS_INDEX);
    const res = await index.search("", {
      vector: queryEmbedding,
      hybrid: { semanticRatio: 1.0, embedder: "default" },
      filter: filters.length > 0 ? filters.join(" AND ") : undefined,
      limit: topK,
      showRankingScore: true,
    } as any);

    return res.hits.map((hit: any) => ({
      ...this.stripVectors(hit),
      _score: hit._rankingScore ?? 0,
    }));
  }

  async listSkills(options?: SkillSearchOptions, limit = 100, offset = 0): Promise<AgentSkill[]> {
    await this.ensureInitialized();
    const filters: string[] = [];
    if (options?.project) filters.push(`project = "${this.escapeFilterValue(options.project)}"`);

    const index = this.client.index(SKILLS_INDEX);
    const res = await index.getDocuments({
      filter: filters.length > 0 ? filters.join(" AND ") : undefined,
      limit,
      offset,
      fields: ["id", "project", "name", "description", "ai_intent", "ai_topics", "ai_quality_score", "metadata", "created_at", "updated_at"],
    });
    return res.results.map((doc: any) => this.stripVectors(doc));
  }

  async getSkill(id: string): Promise<AgentSkill | null> {
    await this.ensureInitialized();
    try {
      const index = this.client.index(SKILLS_INDEX);
      const doc = await index.getDocument(id, {
        fields: ["id", "project", "name", "description", "ai_intent", "ai_topics", "ai_quality_score", "metadata", "created_at", "updated_at"],
      });
      return doc as AgentSkill;
    } catch (e: any) {
      if ((e?.code === "document_not_found" || e?.cause?.code === "document_not_found")) return null;
      throw e;
    }
  }

  async deleteSkill(id: string) {
    await this.ensureInitialized();
    try {
      const index = this.client.index(SKILLS_INDEX);
      const task = await index.deleteDocument(id);
      await this.client.tasks.waitForTask(task.taskUid);
    } catch (e: any) {
      if ((e?.code !== "document_not_found" && e?.cause?.code !== "document_not_found")) throw e;
    }
  }

  private buildSkillFilters(options: SkillSearchOptions): string[] {
    const filters: string[] = [];
    if (options.project) filters.push(`project = "${this.escapeFilterValue(options.project)}"`);
    if (options.maxAgeDays) {
      const cutoff = new Date(Date.now() - options.maxAgeDays * 24 * 60 * 60 * 1000).toISOString();
      filters.push(`created_at >= "${cutoff}"`);
    }
    return filters;
  }

  // ---------------------------------------------------------------------------
  // Memories
  // ---------------------------------------------------------------------------

  async addMemory(memory: AgentMemory, embedding: number[]) {
    await this.ensureInitialized();
    assertEmbeddingDimension(embedding, "MeilisearchDB.addMemory");
    const ts = new Date().toISOString();
    const index = this.client.index(MEMORIES_INDEX);
    const doc = {
      id: memory.id,
      project: memory.project || null,
      content: memory.content,
      category: memory.category,
      owner: memory.owner,
      importance: memory.importance,
      ai_intent: memory.ai_intent ?? null,
      ai_topics: memory.ai_topics ?? null,
      ai_quality_score: memory.ai_quality_score ?? null,
      _vectors: { default: embedding },
      metadata: memory.metadata || null,
      created_at: memory.created_at || ts,
      updated_at: memory.updated_at || ts,
    };
    const task = await index.addDocuments([doc]);
    await this.client.tasks.waitForTask(task.taskUid);
  }

  async updateMemory(
    id: string,
    updates: {
      content?: string;
      category?: MemoryCategory;
      importance?: number;
      ai_intent?: AgentMemory["ai_intent"];
      ai_topics?: AgentMemory["ai_topics"];
      ai_quality_score?: AgentMemory["ai_quality_score"];
      metadata?: Record<string, any>;
    },
    embedding?: number[]
  ) {
    await this.ensureInitialized();
    const doc: Record<string, any> = { id, updated_at: new Date().toISOString() };
    if (updates.content !== undefined) doc.content = updates.content;
    if (updates.category !== undefined) doc.category = updates.category;
    if (updates.importance !== undefined) doc.importance = updates.importance;
    if (updates.ai_intent !== undefined) doc.ai_intent = updates.ai_intent;
    if (updates.ai_topics !== undefined) doc.ai_topics = updates.ai_topics;
    if (updates.ai_quality_score !== undefined) doc.ai_quality_score = updates.ai_quality_score;
    if (updates.metadata !== undefined) doc.metadata = updates.metadata;
    if (embedding) {
      assertEmbeddingDimension(embedding, "MeilisearchDB.updateMemory");
      doc._vectors = { default: embedding };
    }
    const index = this.client.index(MEMORIES_INDEX);
    const task = await index.updateDocuments([doc]);
    await this.client.tasks.waitForTask(task.taskUid);
  }

  async searchMemories(
    queryEmbedding: number[],
    queryText: string,
    options: MemorySearchOptions = {}
  ): Promise<(AgentMemory & { _score: number; _highlight?: Record<string, string[]> })[]> {
    await this.ensureInitialized();
    assertEmbeddingDimension(queryEmbedding, "MeilisearchDB.searchMemories");
    const topK = options.topK ?? 10;
    const ow = options.weights ?? DEFAULT_MEMORY_WEIGHTS;
    const w = normalizeWeights({
      vector: ow.vector, keyword: ow.keyword,
      recency: ow.recency ?? 0, importance: ow.importance ?? 0,
    });

    const filters = this.buildMemoryFilters(options);
    const vectorKeywordSum = w.vector + w.keyword;
    const semanticRatio = vectorKeywordSum > 0 ? w.vector / vectorKeywordSum : 0.5;
    const fetchLimit = topK * CANDIDATE_MULTIPLIER;

    const searchParams: any = {
      vector: queryEmbedding,
      hybrid: { semanticRatio, embedder: "default" },
      filter: filters.length > 0 ? filters.join(" AND ") : undefined,
      limit: fetchLimit,
      showRankingScore: true,
    };

    if (options.highlight) {
      searchParams.attributesToHighlight = ["content"];
      searchParams.highlightPreTag = "<mark>";
      searchParams.highlightPostTag = "</mark>";
    }

    if (options.minScore != null) {
      searchParams.rankingScoreThreshold = options.minScore;
    }

    const index = this.client.index(MEMORIES_INDEX);
    const res = await index.search(queryText, searchParams);

    return this.rerankAndMap<AgentMemory>(res.hits, w, topK, options);
  }

  async searchMemoriesByVector(
    queryEmbedding: number[],
    options: { topK?: number; project?: string } = {}
  ): Promise<(AgentMemory & { _score: number })[]> {
    await this.ensureInitialized();
    assertEmbeddingDimension(queryEmbedding, "MeilisearchDB.searchMemoriesByVector");
    const topK = options.topK ?? 10;
    const filters: string[] = [];
    if (options.project) filters.push(`project = "${this.escapeFilterValue(options.project)}"`);

    const index = this.client.index(MEMORIES_INDEX);
    const res = await index.search("", {
      vector: queryEmbedding,
      hybrid: { semanticRatio: 1.0, embedder: "default" },
      filter: filters.length > 0 ? filters.join(" AND ") : undefined,
      limit: topK,
      showRankingScore: true,
    } as any);

    return res.hits.map((hit: any) => ({
      ...this.stripVectors(hit),
      _score: hit._rankingScore ?? 0,
    }));
  }

  async listMemories(options?: MemorySearchOptions, limit = 100, offset = 0): Promise<AgentMemory[]> {
    await this.ensureInitialized();
    const filters = options ? this.buildMemoryFilters(options) : [];

    const index = this.client.index(MEMORIES_INDEX);
    const res = await index.getDocuments({
      filter: filters.length > 0 ? filters.join(" AND ") : undefined,
      limit,
      offset,
      fields: ["id", "project", "content", "category", "owner", "importance", "ai_intent", "ai_topics", "ai_quality_score", "metadata", "created_at", "updated_at"],
    });
    return res.results.map((doc: any) => this.stripVectors(doc));
  }

  async getMemory(id: string): Promise<AgentMemory | null> {
    await this.ensureInitialized();
    try {
      const index = this.client.index(MEMORIES_INDEX);
      const doc = await index.getDocument(id, {
        fields: ["id", "project", "content", "category", "owner", "importance", "ai_intent", "ai_topics", "ai_quality_score", "metadata", "created_at", "updated_at"],
      });
      return doc as AgentMemory;
    } catch (e: any) {
      if ((e?.code === "document_not_found" || e?.cause?.code === "document_not_found")) return null;
      throw e;
    }
  }

  async deleteMemory(id: string) {
    await this.ensureInitialized();
    try {
      const index = this.client.index(MEMORIES_INDEX);
      const task = await index.deleteDocument(id);
      await this.client.tasks.waitForTask(task.taskUid);
    } catch (e: any) {
      if ((e?.code !== "document_not_found" && e?.cause?.code !== "document_not_found")) throw e;
    }
  }

  private buildMemoryFilters(options: MemorySearchOptions): string[] {
    const filters: string[] = [];
    if (options.project) filters.push(`project = "${this.escapeFilterValue(options.project)}"`);
    if (options.owner) filters.push(`owner = "${this.escapeFilterValue(options.owner)}"`);
    if (options.category) filters.push(`category = "${this.escapeFilterValue(options.category)}"`);
    if (options.minImportance) filters.push(`importance >= ${options.minImportance}`);
    if (options.maxAgeDays) {
      const cutoff = new Date(Date.now() - options.maxAgeDays * 24 * 60 * 60 * 1000).toISOString();
      filters.push(`created_at >= "${cutoff}"`);
    }
    return filters;
  }

  // ---------------------------------------------------------------------------
  // Context Nodes
  // ---------------------------------------------------------------------------

  async addContextNode(node: AgentContextNode, embedding: number[]) {
    await this.ensureInitialized();
    assertEmbeddingDimension(embedding, "MeilisearchDB.addContextNode");
    const ts = new Date().toISOString();
    const ancestors = await this.computeAncestors(node.parent_uri);

    const index = this.client.index(CONTEXT_INDEX);
    const doc = {
      id: this.uriToId(node.uri),
      uri: node.uri,
      project: node.project || null,
      parent_uri: node.parent_uri,
      ancestors,
      name: node.name,
      abstract: node.abstract,
      overview: node.overview || null,
      content: node.content || null,
      ai_intent: node.ai_intent ?? null,
      ai_topics: node.ai_topics ?? null,
      ai_quality_score: node.ai_quality_score ?? null,
      _vectors: { default: embedding },
      metadata: node.metadata || null,
      created_at: node.created_at || ts,
      updated_at: node.updated_at || ts,
      is_deleted: false,
      deleted_at: null,
      version_history: [],
    };
    const task = await index.addDocuments([doc]);
    await this.client.tasks.waitForTask(task.taskUid);
  }

  async updateContextNode(
    uri: string,
    updates: {
      name?: string;
      abstract?: string;
      overview?: string;
      content?: string;
      ai_intent?: AgentContextNode["ai_intent"];
      ai_topics?: AgentContextNode["ai_topics"];
      ai_quality_score?: AgentContextNode["ai_quality_score"];
      metadata?: Record<string, any>;
    },
    embedding?: number[]
  ) {
    await this.ensureInitialized();

    // Read-modify-write for version history
    const existingNode = await this.getContextNodeRaw(uri);

    if (existingNode) {
      const historyEntry = {
        updated_at: existingNode.updated_at || existingNode.created_at,
        name: existingNode.name,
        abstract: existingNode.abstract,
        overview: existingNode.overview || null,
        content: existingNode.content || null,
      };

      let versionHistory: any[] = existingNode.version_history || [];
      versionHistory.push(historyEntry);
      if (versionHistory.length > 10) {
        versionHistory = versionHistory.slice(-10);
      }

      const doc: Record<string, any> = {
        id: this.uriToId(uri),
        uri,
        updated_at: new Date().toISOString(),
        version_history: versionHistory,
      };
      if (updates.name !== undefined) doc.name = updates.name;
      if (updates.abstract !== undefined) doc.abstract = updates.abstract;
      if (updates.overview !== undefined) doc.overview = updates.overview;
      if (updates.content !== undefined) doc.content = updates.content;
      if (updates.ai_intent !== undefined) doc.ai_intent = updates.ai_intent;
      if (updates.ai_topics !== undefined) doc.ai_topics = updates.ai_topics;
      if (updates.ai_quality_score !== undefined) doc.ai_quality_score = updates.ai_quality_score;
      if (updates.metadata !== undefined) doc.metadata = updates.metadata;
      if (embedding) {
        assertEmbeddingDimension(embedding, "MeilisearchDB.updateContextNode");
        doc._vectors = { default: embedding };
      }

      const index = this.client.index(CONTEXT_INDEX);
      const task = await index.updateDocuments([doc]);
      await this.client.tasks.waitForTask(task.taskUid);
    } else {
      // Node doesn't exist yet — partial update
      const doc: Record<string, any> = { id: this.uriToId(uri), uri, updated_at: new Date().toISOString() };
      if (updates.name !== undefined) doc.name = updates.name;
      if (updates.abstract !== undefined) doc.abstract = updates.abstract;
      if (updates.overview !== undefined) doc.overview = updates.overview;
      if (updates.content !== undefined) doc.content = updates.content;
      if (updates.ai_intent !== undefined) doc.ai_intent = updates.ai_intent;
      if (updates.ai_topics !== undefined) doc.ai_topics = updates.ai_topics;
      if (updates.ai_quality_score !== undefined) doc.ai_quality_score = updates.ai_quality_score;
      if (updates.metadata !== undefined) doc.metadata = updates.metadata;
      if (embedding) {
        assertEmbeddingDimension(embedding, "MeilisearchDB.updateContextNode");
        doc._vectors = { default: embedding };
      }
      const index = this.client.index(CONTEXT_INDEX);
      const task = await index.updateDocuments([doc]);
      await this.client.tasks.waitForTask(task.taskUid);
    }
  }

  async searchContextNodes(
    queryEmbedding: number[],
    queryText: string,
    options: ContextSearchOptions = {}
  ): Promise<(AgentContextNode & { _score: number; _highlight?: Record<string, string[]> })[]> {
    await this.ensureInitialized();
    assertEmbeddingDimension(queryEmbedding, "MeilisearchDB.searchContextNodes");
    const topK = options.topK ?? 10;
    const ow = options.weights ?? DEFAULT_CONTEXT_WEIGHTS;
    const w = normalizeWeights({
      vector: ow.vector, keyword: ow.keyword,
      recency: ow.recency ?? 0, importance: 0,
    });

    const filters = this.buildContextFilters(options);
    const vectorKeywordSum = w.vector + w.keyword;
    const semanticRatio = vectorKeywordSum > 0 ? w.vector / vectorKeywordSum : 0.5;
    const fetchLimit = topK * CANDIDATE_MULTIPLIER;

    const searchParams: any = {
      vector: queryEmbedding,
      hybrid: { semanticRatio, embedder: "default" },
      filter: filters.length > 0 ? filters.join(" AND ") : undefined,
      limit: fetchLimit,
      showRankingScore: true,
    };

    if (options.highlight) {
      searchParams.attributesToHighlight = ["name", "abstract", "overview", "content"];
      searchParams.highlightPreTag = "<mark>";
      searchParams.highlightPostTag = "</mark>";
    }

    if (options.minScore != null) {
      searchParams.rankingScoreThreshold = options.minScore;
    }

    const index = this.client.index(CONTEXT_INDEX);
    const res = await index.search(queryText, searchParams);

    return this.rerankAndMap<AgentContextNode>(res.hits, w, topK, options);
  }

  async searchContextNodesByVector(
    queryEmbedding: number[],
    options: { topK?: number; project?: string; includeDeleted?: boolean } = {}
  ): Promise<(AgentContextNode & { _score: number })[]> {
    await this.ensureInitialized();
    assertEmbeddingDimension(queryEmbedding, "MeilisearchDB.searchContextNodesByVector");
    const topK = options.topK ?? 10;
    const filters: string[] = [];
    if (options.project) filters.push(`project = "${this.escapeFilterValue(options.project)}"`);
    if (!options.includeDeleted) filters.push(`is_deleted != true`);

    const index = this.client.index(CONTEXT_INDEX);
    const res = await index.search("", {
      vector: queryEmbedding,
      hybrid: { semanticRatio: 1.0, embedder: "default" },
      filter: filters.length > 0 ? filters.join(" AND ") : undefined,
      limit: topK,
      showRankingScore: true,
    } as any);

    return res.hits.map((hit: any) => ({
      ...this.stripVectors(hit),
      _score: hit._rankingScore ?? 0,
    }));
  }

  async listContextNodes(parentUri?: string, options?: ContextSearchOptions, limit = 100, offset = 0): Promise<AgentContextNode[]> {
    await this.ensureInitialized();
    const filters: string[] = [];
    if (options?.project) filters.push(`project = "${this.escapeFilterValue(options.project)}"`);
    if (parentUri) filters.push(`parent_uri = "${this.escapeFilterValue(parentUri)}"`);
    if (!options?.includeDeleted) filters.push(`is_deleted != true`);

    const index = this.client.index(CONTEXT_INDEX);
    const res = await index.getDocuments({
      filter: filters.length > 0 ? filters.join(" AND ") : undefined,
      limit,
      offset,
    });
    return res.results.map((doc: any) => this.stripVectors(doc));
  }

  async getContextNode(uri: string): Promise<AgentContextNode | null> {
    await this.ensureInitialized();
    try {
      const index = this.client.index(CONTEXT_INDEX);
      const doc = await index.getDocument(this.uriToId(uri));
      const { _vectors, id: _id, ...rest } = doc as any;
      delete rest.ancestors;
      return rest as AgentContextNode;
    } catch (e: any) {
      if ((e?.code === "document_not_found" || e?.cause?.code === "document_not_found")) return null;
      throw e;
    }
  }

  async deleteContextNode(uri: string) {
    await this.ensureInitialized();
    const ts = new Date().toISOString();

    // Soft delete descendants
    const descendants = await this.getDescendants(uri);
    if (descendants.length > 0) {
      const updates = descendants.map((d: any) => ({
        id: this.uriToId(d.uri),
        uri: d.uri,
        is_deleted: true,
        deleted_at: ts,
      }));
      const index = this.client.index(CONTEXT_INDEX);
      const task = await index.updateDocuments(updates);
      await this.client.tasks.waitForTask(task.taskUid);
    }

    // Soft delete the node itself
    try {
      const index = this.client.index(CONTEXT_INDEX);
      const task = await index.updateDocuments([{ id: this.uriToId(uri), uri, is_deleted: true, deleted_at: ts }]);
      await this.client.tasks.waitForTask(task.taskUid);
    } catch (e: any) {
      if ((e?.code !== "document_not_found" && e?.cause?.code !== "document_not_found")) throw e;
    }
  }

  async restoreContextNode(uri: string) {
    await this.ensureInitialized();

    // Restore descendants
    const descendants = await this.getDescendants(uri);
    if (descendants.length > 0) {
      const updates = descendants.map((d: any) => ({
        id: this.uriToId(d.uri),
        uri: d.uri,
        is_deleted: false,
        deleted_at: null,
      }));
      const index = this.client.index(CONTEXT_INDEX);
      const task = await index.updateDocuments(updates);
      await this.client.tasks.waitForTask(task.taskUid);
    }

    // Restore the node itself
    try {
      const index = this.client.index(CONTEXT_INDEX);
      const task = await index.updateDocuments([{ id: this.uriToId(uri), uri, is_deleted: false, deleted_at: null }]);
      await this.client.tasks.waitForTask(task.taskUid);
    } catch (e: any) {
      if ((e?.code !== "document_not_found" && e?.cause?.code !== "document_not_found")) throw e;
    }
  }

  async getContextSubtree(nodeUri: string, includeDeleted = false): Promise<(AgentContextNode & { depth: number })[]> {
    await this.ensureInitialized();
    const filters: string[] = [`ancestors = "${this.escapeFilterValue(nodeUri)}"`];
    if (!includeDeleted) filters.push(`is_deleted != true`);

    const index = this.client.index(CONTEXT_INDEX);

    // Fetch root node
    let rootNode: any = null;
    try {
      rootNode = await index.getDocument(this.uriToId(nodeUri));
    } catch (e: any) {
      if ((e?.code === "document_not_found" || e?.cause?.code === "document_not_found")) return [];
      throw e;
    }

    if (!includeDeleted && rootNode.is_deleted) return [];

    // Fetch descendants
    const res = await index.search("", {
      filter: filters.join(" AND "),
      limit: 1000,
    } as any);

    const rootDepth = rootNode.ancestors?.length ?? 0;
    const allNodes = [rootNode, ...res.hits];

    return allNodes
      .map((n: any) => {
        const nodeAncestors: string[] = n.ancestors || [];
        const depth = nodeAncestors.length - rootDepth;
        const { _vectors, _rankingScore, _formatted, _matchesPosition, id: _id, ...rest } = n;
        delete rest.ancestors;
        return { ...rest, depth } as AgentContextNode & { depth: number };
      })
      .sort((a: any, b: any) => a.depth - b.depth);
  }

  async getContextPath(nodeUri: string, includeDeleted = false): Promise<(AgentContextNode & { depth: number })[]> {
    await this.ensureInitialized();
    const node = await this.getContextNodeWithAncestors(nodeUri);
    if (!node) return [];

    const ancestorUris: string[] = (node as any).ancestors || [];
    if (ancestorUris.length === 0) {
      if (!includeDeleted && node.is_deleted) return [];
      const { ancestors: _, _vectors: _v, ...rest } = node as any;
      return [{ ...rest, depth: 0 }];
    }

    // Fetch all ancestor nodes by URI
    const index = this.client.index(CONTEXT_INDEX);
    const allUris = [...ancestorUris, nodeUri];

    const filters: string[] = [
      allUris.map((u) => `uri = "${this.escapeFilterValue(u)}"`).join(" OR "),
    ];
    if (!includeDeleted) filters.push(`is_deleted != true`);

    const res = await index.search("", {
      filter: filters.join(" AND "),
      limit: allUris.length,
    } as any);

    const allNodes = res.hits as any[];
    return allNodes
      .map((n: any) => {
        const nodeAncestors: string[] = n.ancestors || [];
        const depth = nodeAncestors.length;
        const { ancestors: _, _vectors, _rankingScore, _formatted, _matchesPosition, id: _id, ...rest } = n;
        return { ...rest, depth } as AgentContextNode & { depth: number };
      })
      .sort((a: any, b: any) => a.depth - b.depth);
  }

  private async getDescendants(uri: string): Promise<any[]> {
    const index = this.client.index(CONTEXT_INDEX);
    const res = await index.search("", {
      filter: `ancestors = "${this.escapeFilterValue(uri)}"`,
      limit: 1000,
    } as any);
    return res.hits;
  }

  private async computeAncestors(parentUri: string | null): Promise<string[]> {
    if (!parentUri) return [];
    const parent = await this.getContextNodeWithAncestors(parentUri);
    if (!parent) return [parentUri];
    const parentAncestors: string[] = (parent as any).ancestors || [];
    return [...parentAncestors, parentUri];
  }

  private async getContextNodeWithAncestors(uri: string): Promise<(AgentContextNode & { ancestors?: string[] }) | null> {
    try {
      const index = this.client.index(CONTEXT_INDEX);
      const doc = await index.getDocument(this.uriToId(uri));
      const { _vectors, id: _id, ...rest } = doc as any;
      return rest as AgentContextNode & { ancestors?: string[] };
    } catch (e: any) {
      if ((e?.code === "document_not_found" || e?.cause?.code === "document_not_found")) return null;
      throw e;
    }
  }

  /** Internal: get context node with all fields including ancestors and version_history */
  private async getContextNodeRaw(uri: string): Promise<any | null> {
    try {
      const index = this.client.index(CONTEXT_INDEX);
      return await index.getDocument(this.uriToId(uri));
    } catch (e: any) {
      if ((e?.code === "document_not_found" || e?.cause?.code === "document_not_found")) return null;
      throw e;
    }
  }

  private buildContextFilters(options: ContextSearchOptions): string[] {
    const filters: string[] = [];
    if (options.project) filters.push(`project = "${this.escapeFilterValue(options.project)}"`);
    if (options.parentUri) filters.push(`parent_uri = "${this.escapeFilterValue(options.parentUri)}"`);
    if (options.maxAgeDays) {
      const cutoff = new Date(Date.now() - options.maxAgeDays * 24 * 60 * 60 * 1000).toISOString();
      filters.push(`created_at >= "${cutoff}"`);
    }
    if (!options.includeDeleted) filters.push(`is_deleted != true`);
    return filters;
  }

  // ---------------------------------------------------------------------------
  // Symbols (for Mairu Surgical Reads)
  // ---------------------------------------------------------------------------

  async insertSymbols(symbols: any[]) {
    await this.ensureInitialized();
    const batchSize = 1000;
    for (let i = 0; i < symbols.length; i += batchSize) {
      const batch = symbols.slice(i, i + batchSize);
      const task = await this.client.index(SYMBOLS_INDEX).addDocuments(batch, { primaryKey: "id" });
      await this.client.tasks.waitForTask(task.taskUid);
    }
  }

  async clearSymbolsForFile(filePath: string) {
    await this.ensureInitialized();
    const task = await this.client.index(SYMBOLS_INDEX).deleteDocuments({
      filter: [`file_path = "${this.escapeFilterValue(filePath)}"`],
    });
    await this.client.tasks.waitForTask(task.taskUid);
  }

  private rerankAndMap<T>(
    hits: any[],
    weights: { vector: number; keyword: number; recency: number; importance: number },
    topK: number,
    options: { recencyScale?: string; recencyDecay?: number; highlight?: boolean } = {}
  ): (T & { _score: number; _highlight?: Record<string, string[]> })[] {
    const now = Date.now();
    const scaleMs = parseDurationMs(options.recencyScale || config.meili.recencyScale);
    const decay = options.recencyDecay ?? config.meili.recencyDecay;

    const scored = hits.map((hit: any) => {
      let score = hit._rankingScore ?? 0;

      // Recency decay — falls back to config.meili.recencyScale / recencyDecay when not set per-query
      if (weights.recency > 0 && hit.created_at) {
        const ageMs = now - new Date(hit.created_at).getTime();
        const recencyScore = Math.pow(decay, ageMs / scaleMs);
        score += recencyScore * weights.recency;
      }

      // Importance boost
      if (weights.importance > 0 && hit.importance != null) {
        score += (hit.importance / 10) * weights.importance;
      }

      // AI quality boost
      if (hit.ai_quality_score != null && hit.ai_quality_score > 0) {
        score += (hit.ai_quality_score / 10) * AI_QUALITY_FUNCTION_WEIGHT * 0.1;
      }

      const result: any = {
        ...this.stripVectors(hit),
        _score: score,
      };

      // Map highlights from _formatted
      if (options.highlight && hit._formatted) {
        const highlight: Record<string, string[]> = {};
        for (const [key, val] of Object.entries(hit._formatted)) {
          if (typeof val === "string" && val.includes("<mark>")) {
            highlight[key] = [val];
          }
        }
        if (Object.keys(highlight).length > 0) {
          result._highlight = highlight;
        }
      }

      return result;
    });

    return scored
      .sort((a: any, b: any) => b._score - a._score)
      .slice(0, topK);
  }

  private stripVectors(doc: any): any {
    const { _vectors, _rankingScore, _formatted, _matchesPosition, id, ...rest } = doc;
    // Put back id for skills and memories if it was stripped
    if (doc.id && !doc.uri) rest.id = id;
    return rest;
  }

  private escapeFilterValue(value: string): string {
    return value.replace(/"/g, '\\"');
  }

  private uriToId(uri: string): string {
    return Buffer.from(uri).toString('base64url');
  }
}
