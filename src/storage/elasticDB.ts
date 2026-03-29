import { Client, HttpConnection } from "@elastic/elasticsearch";
import {
  AgentSkill,
  AgentMemory,
  AgentContextNode,
  MemorySearchOptions,
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

export const SKILLS_INDEX = "contextfs_skills";
export const MEMORIES_INDEX = "contextfs_memories";
export const CONTEXT_INDEX = "contextfs_context_nodes";
const AI_QUALITY_FUNCTION_WEIGHT = 2;

function buildIndexSettings() {
  const synonyms = config.elastic.synonyms;
  const filters: string[] = ["lowercase", "english_stop", "english_stemmer"];
  const filterDefs: Record<string, any> = {
    english_stop: { type: "stop" as const, stopwords: ["_english_"] },
    english_stemmer: { type: "stemmer" as const, language: "english" as const },
  };

  if (synonyms.length > 0) {
    filters.splice(1, 0, "contextfs_synonyms");
    filterDefs.contextfs_synonyms = {
      type: "synonym" as const,
      synonyms,
    };
  }

  return {
    number_of_shards: 1,
    number_of_replicas: 0,
    analysis: {
      analyzer: {
        content_analyzer: {
          type: "custom" as const,
          tokenizer: "standard",
          filter: filters,
        },
        ngram_analyzer: {
          type: "custom" as const,
          tokenizer: "ngram_tokenizer",
          filter: ["lowercase"],
        },
      },
      tokenizer: {
        ngram_tokenizer: {
          type: "ngram" as const,
          min_gram: 3,
          max_gram: 4,
          token_chars: ["letter", "digit"] as string[],
        },
      },
      filter: filterDefs,
    },
    similarity: {
      contextfs_bm25: {
        type: "BM25",
        k1: config.elastic.bm25K1,
        b: config.elastic.bm25B,
      },
    },
  };
}

/** Build text field mapping with optional ngram sub-field */
function textField(opts?: { keyword?: boolean; ngram?: boolean }) {
  const field: Record<string, any> = {
    type: "text",
    analyzer: "content_analyzer",
    similarity: "contextfs_bm25",
  };
  const fields: Record<string, any> = {};
  if (opts?.keyword) fields.raw = { type: "keyword" };
  if (opts?.ngram) fields.ngram = { type: "text", analyzer: "ngram_analyzer" };
  if (Object.keys(fields).length > 0) field.fields = fields;
  return field;
}

export class ElasticDB {
  private client: Client;
  private initialized = false;

  constructor(node: string, auth?: { username: string; password: string }) {
    this.client = new Client({
      node,
      Connection: HttpConnection,
      ...(auth?.username ? { auth } : {}),
    });
  }

  private async ensureInitialized() {
    if (this.initialized) return;
    try { await this.initIndices(); this.initialized = true; }
    catch (e: any) {
      if (e?.meta?.meta?.connection?.status === "error" || e?.code === "ECONNREFUSED") {
        console.error("❌ Elasticsearch connection failed. Is Docker running? (Run: docker compose up -d)");
        process.exit(1);
      }
      throw e;
    }
  }

  /** Cluster health + per-index stats for the dashboard */
  async getClusterStats() {
    await this.ensureInitialized();
    const [health, indicesStats] = await Promise.all([
      this.client.cluster.health(),
      this.client.indices.stats({
        index: [SKILLS_INDEX, MEMORIES_INDEX, CONTEXT_INDEX],
        metric: ["docs", "store"],
      }),
    ]);

    const indices: Record<string, any> = {};
    for (const idx of [SKILLS_INDEX, MEMORIES_INDEX, CONTEXT_INDEX]) {
      const s = indicesStats.indices?.[idx];
      indices[idx] = {
        docs: s?.primaries?.docs?.count ?? 0,
        deletedDocs: s?.primaries?.docs?.deleted ?? 0,
        sizeBytes: s?.primaries?.store?.size_in_bytes ?? 0,
      };
    }

    return {
      clusterName: health.cluster_name,
      status: health.status,
      numberOfNodes: health.number_of_nodes,
      activeShards: health.active_shards,
      relocatingShards: health.relocating_shards,
      unassignedShards: health.unassigned_shards,
      indices,
    };
  }

  async initIndices() {
    const settings = buildIndexSettings();

    await this.createIndexIfNotExists(SKILLS_INDEX, settings, {
      id: { type: "keyword" },
      project: { type: "keyword" },
      name: textField({ keyword: true, ngram: true }),
      description: textField({ ngram: true }),
      ai_intent: { type: "keyword" },
      ai_topics: { type: "keyword" },
      ai_quality_score: { type: "float" },
      embedding: { type: "dense_vector", dims: EMBEDDING_DIM, index: true, similarity: "cosine", index_options: { type: "hnsw", m: 32, ef_construction: 200 } },
      metadata: { type: "object", dynamic: true },
      created_at: { type: "date" },
      updated_at: { type: "date" },
    });

    await this.createIndexIfNotExists(MEMORIES_INDEX, settings, {
      id: { type: "keyword" },
      project: { type: "keyword" },
      content: textField({ ngram: true }),
      category: { type: "keyword" },
      owner: { type: "keyword" },
      importance: { type: "integer" },
      ai_intent: { type: "keyword" },
      ai_topics: { type: "keyword" },
      ai_quality_score: { type: "float" },
      embedding: { type: "dense_vector", dims: EMBEDDING_DIM, index: true, similarity: "cosine", index_options: { type: "hnsw", m: 32, ef_construction: 200 } },
      metadata: { type: "object", dynamic: true },
      created_at: { type: "date" },
      updated_at: { type: "date" },
    });

    await this.createIndexIfNotExists(CONTEXT_INDEX, settings, {
      uri: { type: "keyword" },
      project: { type: "keyword" },
      parent_uri: { type: "keyword" },
      ancestors: { type: "keyword" },
      name: textField({ keyword: true, ngram: true }),
      abstract: textField({ ngram: true }),
      overview: textField(),
      content: textField(),
      ai_intent: { type: "keyword" },
      ai_topics: { type: "keyword" },
      ai_quality_score: { type: "float" },
      embedding: { type: "dense_vector", dims: EMBEDDING_DIM, index: true, similarity: "cosine", index_options: { type: "hnsw", m: 32, ef_construction: 200 } },
      metadata: { type: "object", dynamic: true },
      created_at: { type: "date" },
      updated_at: { type: "date" },
      is_deleted: { type: "boolean" },
      deleted_at: { type: "date" },
      version_history: { type: "object", dynamic: true, enabled: false }, // Store full objects but do not index their internal fields for search
    });

    // Backward-compatible mapping upgrades for existing indices.
    await Promise.all([
      this.ensureAiFieldMappings(SKILLS_INDEX),
      this.ensureAiFieldMappings(MEMORIES_INDEX),
      this.ensureAiFieldMappings(CONTEXT_INDEX),
    ]);
  }

  async resetIndices() {
    await this.ensureInitialized();
    for (const idx of [SKILLS_INDEX, MEMORIES_INDEX, CONTEXT_INDEX]) {
      const exists = await this.client.indices.exists({ index: idx });
      if (exists) await this.client.indices.delete({ index: idx });
    }
  }

  private async createIndexIfNotExists(index: string, settings: any, properties: Record<string, any>) {
    const exists = await this.client.indices.exists({ index });
    if (!exists) {
      await this.client.indices.create({
        index,
        settings,
        mappings: { properties },
      });
    }
  }

  // ---------------------------------------------------------------------------
  // Skills
  // ---------------------------------------------------------------------------

  async addSkill(skill: AgentSkill, embedding: number[]) {
    await this.ensureInitialized();
    assertEmbeddingDimension(embedding, "ElasticDB.addSkill");
    const ts = new Date().toISOString();
    await this.client.index({
      index: SKILLS_INDEX,
      id: skill.id,
      document: {
        id: skill.id,
        project: skill.project || null,
        name: skill.name,
        description: skill.description,
        ai_intent: skill.ai_intent ?? null,
        ai_topics: skill.ai_topics ?? null,
        ai_quality_score: skill.ai_quality_score ?? null,
        embedding,
        metadata: skill.metadata || null,
        created_at: skill.created_at || ts,
        updated_at: skill.updated_at || ts,
      },
      refresh: true,
    });
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
    const doc: Record<string, any> = { updated_at: new Date().toISOString() };
    if (updates.name !== undefined) doc.name = updates.name;
    if (updates.description !== undefined) doc.description = updates.description;
    if (updates.ai_intent !== undefined) doc.ai_intent = updates.ai_intent;
    if (updates.ai_topics !== undefined) doc.ai_topics = updates.ai_topics;
    if (updates.ai_quality_score !== undefined) doc.ai_quality_score = updates.ai_quality_score;
    if (updates.metadata !== undefined) doc.metadata = updates.metadata;
    if (embedding) {
      assertEmbeddingDimension(embedding, "ElasticDB.updateSkill");
      doc.embedding = embedding;
    }
    await this.client.update({ index: SKILLS_INDEX, id, doc, refresh: true });
  }

  async searchSkills(queryEmbedding: number[], queryText: string, options: SkillSearchOptions = {}): Promise<(AgentSkill & { _score: number; _highlight?: Record<string, string[]> })[]> {
    await this.ensureInitialized();
    assertEmbeddingDimension(queryEmbedding, "ElasticDB.searchSkills");
    const topK = options.topK ?? 10;
    const ow = options.weights ?? DEFAULT_SKILL_WEIGHTS;
    const w = normalizeWeights({
      vector: ow.vector, keyword: ow.keyword,
      recency: ow.recency ?? 0, importance: 0,
    });

    const fuzziness = options.fuzziness ?? config.elastic.defaultFuzziness;
    const phraseBoost = options.phraseBoost ?? config.elastic.defaultPhraseBoost;
    const fb = options.fieldBoosts ?? {};

    const filters: any[] = [];
    if (options.project) filters.push({ term: { project: options.project } });
    if (options.maxAgeDays) filters.push({ range: { created_at: { gte: `now-${options.maxAgeDays}d` } } });

    const nameBoost = fb.name ?? 2;
    const descBoost = fb.description ?? 1;

    const should: any[] = [
      { multi_match: { query: queryText, fields: [`name^${nameBoost}`, `description^${descBoost}`], boost: w.keyword * 10, analyzer: "content_analyzer", fuzziness } },
    ];

    // Ngram sub-field for partial/substring matching
    should.push({ multi_match: { query: queryText, fields: ["name.ngram", "description.ngram"], boost: w.keyword * 2 } });

    // Phrase boost for exact ordering
    if (phraseBoost > 0) {
      should.push({ multi_match: { query: queryText, fields: [`name^${nameBoost}`, `description^${descBoost}`], type: "phrase", boost: phraseBoost } });
    }

    const functions: any[] = [buildAiQualityFunction()];
    if (w.recency > 0) {
      functions.push({
        exp: { created_at: { origin: "now", scale: options.recencyScale || config.elastic.recencyScale, decay: options.recencyDecay || config.elastic.recencyDecay } },
        weight: w.recency * 10,
      });
    }

    const query = {
      function_score: { query: {
        bool: {
          should,
          ...(filters.length ? { filter: filters } : {}),
          minimum_should_match: 0,
        },
      }, functions, score_mode: "sum", boost_mode: "sum" },
    };

    const body: any = {
      index: SKILLS_INDEX,
      size: topK,
      knn: {
        field: "embedding",
        query_vector: queryEmbedding,
        k: topK * CANDIDATE_MULTIPLIER,
        num_candidates: Math.max(100, topK * CANDIDATE_MULTIPLIER * 10),
        boost: w.vector * 10,
        ...(filters.length ? { filter: filters } : {}),
      },
      query,
      _source: { excludes: ["embedding"] },
    };

    if (options.minScore) body.min_score = options.minScore;
    if (options.highlight) body.highlight = buildHighlight(["name", "description"]);

    const res = await this.client.search(body as any);
    return this.mapHits<AgentSkill>(res, options.highlight);
  }

  async searchSkillsByVector(queryEmbedding: number[], options: { topK?: number; project?: string } = {}): Promise<(AgentSkill & { _score: number })[]> {
    await this.ensureInitialized();
    assertEmbeddingDimension(queryEmbedding, "ElasticDB.searchSkillsByVector");
    const topK = options.topK ?? 10;
    const filters: any[] = [];
    if (options.project) filters.push({ term: { project: options.project } });

    const res = await this.client.search({
      index: SKILLS_INDEX,
      size: topK,
      knn: {
        field: "embedding",
        query_vector: queryEmbedding,
        k: topK * CANDIDATE_MULTIPLIER,
        num_candidates: Math.max(100, topK * CANDIDATE_MULTIPLIER * 10),
        ...(filters.length ? { filter: filters } : {}),
      },
      _source: { excludes: ["embedding"] },
    } as any);
    return this.mapHits<AgentSkill>(res);
  }

  async listSkills(options?: SkillSearchOptions, limit = 100, offset = 0): Promise<AgentSkill[]> {
    await this.ensureInitialized();
    const filters: any[] = [];
    if (options?.project) filters.push({ term: { project: options.project } });

    const res = await this.client.search({
      index: SKILLS_INDEX,
      size: limit,
      from: offset,
      query: filters.length ? { bool: { filter: filters } } : { match_all: {} },
      sort: [{ updated_at: { order: "desc" } }],
      _source: { excludes: ["embedding"] },
    });
    return this.mapSources<AgentSkill>(res);
  }

  async getSkill(id: string): Promise<AgentSkill | null> {
    await this.ensureInitialized();
    try {
      const res = await this.client.get({ index: SKILLS_INDEX, id, _source_excludes: ["embedding"] });
      return res._source as AgentSkill;
    } catch (e: any) {
      if (e.meta?.statusCode === 404) return null;
      throw e;
    }
  }

  async deleteSkill(id: string) {
    await this.ensureInitialized();
    await this.client.delete({ index: SKILLS_INDEX, id, refresh: true }).catch((e: any) => {
      if (e.meta?.statusCode !== 404) throw e;
    });
  }

  // ---------------------------------------------------------------------------
  // Memories
  // ---------------------------------------------------------------------------

  async addMemory(memory: AgentMemory, embedding: number[]) {
    await this.ensureInitialized();
    assertEmbeddingDimension(embedding, "ElasticDB.addMemory");
    const ts = new Date().toISOString();
    await this.client.index({
      index: MEMORIES_INDEX,
      id: memory.id,
      document: {
        id: memory.id,
        project: memory.project || null,
        content: memory.content,
        category: memory.category,
        owner: memory.owner,
        importance: memory.importance,
        ai_intent: memory.ai_intent ?? null,
        ai_topics: memory.ai_topics ?? null,
        ai_quality_score: memory.ai_quality_score ?? null,
        embedding,
        metadata: memory.metadata || null,
        created_at: memory.created_at || ts,
        updated_at: memory.updated_at || ts,
      },
      refresh: true,
    });
  }

  async updateMemory(
    id: string,
    updates: {
      content?: string;
      importance?: number;
      ai_intent?: AgentMemory["ai_intent"];
      ai_topics?: AgentMemory["ai_topics"];
      ai_quality_score?: AgentMemory["ai_quality_score"];
      metadata?: Record<string, any>;
    },
    embedding?: number[]
  ) {
    const doc: Record<string, any> = { updated_at: new Date().toISOString() };
    if (updates.content !== undefined) doc.content = updates.content;
    if (updates.importance !== undefined) doc.importance = updates.importance;
    if (updates.ai_intent !== undefined) doc.ai_intent = updates.ai_intent;
    if (updates.ai_topics !== undefined) doc.ai_topics = updates.ai_topics;
    if (updates.ai_quality_score !== undefined) doc.ai_quality_score = updates.ai_quality_score;
    if (updates.metadata !== undefined) doc.metadata = updates.metadata;
    if (embedding) {
      assertEmbeddingDimension(embedding, "ElasticDB.updateMemory");
      doc.embedding = embedding;
    }
    await this.client.update({ index: MEMORIES_INDEX, id, doc, refresh: true });
  }

  async searchMemories(queryEmbedding: number[], queryText: string, options: MemorySearchOptions = {}): Promise<(AgentMemory & { _score: number; _highlight?: Record<string, string[]> })[]> {
    await this.ensureInitialized();
    assertEmbeddingDimension(queryEmbedding, "ElasticDB.searchMemories");
    const topK = options.topK ?? 10;
    const ow = options.weights ?? DEFAULT_MEMORY_WEIGHTS;
    const w = normalizeWeights({
      vector: ow.vector, keyword: ow.keyword,
      recency: ow.recency ?? 0, importance: ow.importance ?? 0,
    });

    const fuzziness = options.fuzziness ?? config.elastic.defaultFuzziness;
    const phraseBoost = options.phraseBoost ?? config.elastic.defaultPhraseBoost;
    const fb = options.fieldBoosts ?? {};

    const filters: any[] = [];
    if (options.project) filters.push({ term: { project: options.project } });
    if (options.owner) filters.push({ term: { owner: options.owner } });
    if (options.category) filters.push({ term: { category: options.category } });
    if (options.minImportance) filters.push({ range: { importance: { gte: options.minImportance } } });
    if (options.maxAgeDays) filters.push({ range: { created_at: { gte: `now-${options.maxAgeDays}d` } } });

    const contentBoost = fb.content ?? 1;

    const should: any[] = [
      { match: { content: { query: queryText, boost: w.keyword * 10 * contentBoost, analyzer: "content_analyzer", fuzziness } } },
    ];

    // Ngram for partial matching
    should.push({ match: { "content.ngram": { query: queryText, boost: w.keyword * 2 } } });

    // Phrase boost
    if (phraseBoost > 0) {
      should.push({ match_phrase: { content: { query: queryText, boost: phraseBoost } } });
    }

    const functions: any[] = [];
    if (w.importance > 0) {
      functions.push({
        script_score: { script: { source: "doc['importance'].size() > 0 ? doc['importance'].value / 10.0 : 0.1" } },
        weight: w.importance * 10,
      });
    }
    functions.push(buildAiQualityFunction());
    if (w.recency > 0) {
      functions.push({
        exp: { created_at: { origin: "now", scale: options.recencyScale || config.elastic.recencyScale, decay: options.recencyDecay || config.elastic.recencyDecay } },
        weight: w.recency * 10,
      });
    }

    const textQuery: any = {
      bool: {
        should,
        ...(filters.length ? { filter: filters } : {}),
        minimum_should_match: 0,
      },
    };

    const query = functions.length > 0
      ? { function_score: { query: textQuery, functions, score_mode: "sum", boost_mode: "sum" } }
      : textQuery;

    const body: any = {
      index: MEMORIES_INDEX,
      size: topK,
      knn: {
        field: "embedding",
        query_vector: queryEmbedding,
        k: topK * CANDIDATE_MULTIPLIER,
        num_candidates: Math.max(100, topK * CANDIDATE_MULTIPLIER * 10),
        boost: w.vector * 10,
        ...(filters.length ? { filter: filters } : {}),
      },
      query,
      _source: { excludes: ["embedding"] },
    };

    if (options.minScore) body.min_score = options.minScore;
    if (options.highlight) body.highlight = buildHighlight(["content"]);

    const res = await this.client.search(body as any);
    return this.mapHits<AgentMemory>(res, options.highlight);
  }

  async searchMemoriesByVector(queryEmbedding: number[], options: { topK?: number; project?: string } = {}): Promise<(AgentMemory & { _score: number })[]> {
    await this.ensureInitialized();
    assertEmbeddingDimension(queryEmbedding, "ElasticDB.searchMemoriesByVector");
    const topK = options.topK ?? 10;
    const filters: any[] = [];
    if (options.project) filters.push({ term: { project: options.project } });

    const res = await this.client.search({
      index: MEMORIES_INDEX,
      size: topK,
      knn: {
        field: "embedding",
        query_vector: queryEmbedding,
        k: topK * CANDIDATE_MULTIPLIER,
        num_candidates: Math.max(100, topK * CANDIDATE_MULTIPLIER * 10),
        ...(filters.length ? { filter: filters } : {}),
      },
      _source: { excludes: ["embedding"] },
    } as any);
    return this.mapHits<AgentMemory>(res);
  }

  async listMemories(options?: MemorySearchOptions, limit = 100, offset = 0): Promise<AgentMemory[]> {
    await this.ensureInitialized();
    const filters: any[] = [];
    if (options?.project) filters.push({ term: { project: options.project } });

    const res = await this.client.search({
      index: MEMORIES_INDEX,
      size: limit,
      from: offset,
      query: filters.length ? { bool: { filter: filters } } : { match_all: {} },
      sort: [{ updated_at: { order: "desc" } }],
      _source: { excludes: ["embedding"] },
    });
    return this.mapSources<AgentMemory>(res);
  }

  async getMemory(id: string): Promise<AgentMemory | null> {
    await this.ensureInitialized();
    try {
      const res = await this.client.get({ index: MEMORIES_INDEX, id, _source_excludes: ["embedding"] });
      return res._source as AgentMemory;
    } catch (e: any) {
      if (e.meta?.statusCode === 404) return null;
      throw e;
    }
  }

  async deleteMemory(id: string) {
    await this.ensureInitialized();
    await this.client.delete({ index: MEMORIES_INDEX, id, refresh: true }).catch((e: any) => {
      if (e.meta?.statusCode !== 404) throw e;
    });
  }

  // ---------------------------------------------------------------------------
  // Context Nodes
  // ---------------------------------------------------------------------------

  async addContextNode(node: AgentContextNode, embedding: number[]) {
    await this.ensureInitialized();
    assertEmbeddingDimension(embedding, "ElasticDB.addContextNode");
    const ts = new Date().toISOString();

    const ancestors = await this.computeAncestors(node.parent_uri);

    await this.client.index({
      index: CONTEXT_INDEX,
      id: node.uri,
      document: {
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
        embedding,
        metadata: node.metadata || null,
        created_at: node.created_at || ts,
        updated_at: node.updated_at || ts,
      },
      refresh: true,
    });
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
    const doc: Record<string, any> = { updated_at: new Date().toISOString() };
    if (updates.name !== undefined) doc.name = updates.name;
    if (updates.abstract !== undefined) doc.abstract = updates.abstract;
    if (updates.overview !== undefined) doc.overview = updates.overview;
    if (updates.content !== undefined) doc.content = updates.content;
    if (updates.ai_intent !== undefined) doc.ai_intent = updates.ai_intent;
    if (updates.ai_topics !== undefined) doc.ai_topics = updates.ai_topics;
    if (updates.ai_quality_score !== undefined) doc.ai_quality_score = updates.ai_quality_score;
    if (updates.metadata !== undefined) doc.metadata = updates.metadata;
    if (embedding) {
      assertEmbeddingDimension(embedding, "ElasticDB.updateContextNode");
      doc.embedding = embedding;
    }
    
    const existingNode = await this.getContextNode(uri);
    
    if (existingNode) {
      const historyEntry = {
        updated_at: existingNode.updated_at || existingNode.created_at,
        name: existingNode.name,
        abstract: existingNode.abstract,
        overview: existingNode.overview || null,
        content: existingNode.content || null,
      };
      
      let source = `
        ctx._source.updated_at = params.doc.updated_at;
        if (params.doc.name != null) ctx._source.name = params.doc.name;
        if (params.doc.abstract != null) ctx._source.abstract = params.doc.abstract;
        if (params.doc.overview != null) ctx._source.overview = params.doc.overview;
        if (params.doc.content != null) ctx._source.content = params.doc.content;
        if (params.doc.containsKey('ai_intent')) ctx._source.ai_intent = params.doc.ai_intent;
        if (params.doc.containsKey('ai_topics')) ctx._source.ai_topics = params.doc.ai_topics;
        if (params.doc.containsKey('ai_quality_score')) ctx._source.ai_quality_score = params.doc.ai_quality_score;
        if (params.doc.metadata != null) ctx._source.metadata = params.doc.metadata;
        if (params.doc.embedding != null) ctx._source.embedding = params.doc.embedding;
        
        if (ctx._source.version_history == null) {
          ctx._source.version_history = [];
        }
        ctx._source.version_history.add(params.historyEntry);
        if (ctx._source.version_history.size() > 10) {
          ctx._source.version_history.remove(0);
        }
      `;
      
      await this.client.update({
        index: CONTEXT_INDEX,
        id: uri,
        script: {
          source,
          params: { doc, historyEntry }
        },
        refresh: true,
      });
    } else {
      await this.client.update({ index: CONTEXT_INDEX, id: uri, doc, refresh: true });
    }
  }

  async searchContextNodes(queryEmbedding: number[], queryText: string, options: ContextSearchOptions = {}): Promise<(AgentContextNode & { _score: number; _highlight?: Record<string, string[]> })[]> {
    await this.ensureInitialized();
    assertEmbeddingDimension(queryEmbedding, "ElasticDB.searchContextNodes");
    const topK = options.topK ?? 10;
    const ow = options.weights ?? DEFAULT_CONTEXT_WEIGHTS;
    const w = normalizeWeights({
      vector: ow.vector, keyword: ow.keyword,
      recency: ow.recency ?? 0, importance: 0,
    });

    const fuzziness = options.fuzziness ?? config.elastic.defaultFuzziness;
    const phraseBoost = options.phraseBoost ?? config.elastic.defaultPhraseBoost;
    const fb = options.fieldBoosts ?? {};

    const filters: any[] = [];
    if (options.project) filters.push({ term: { project: options.project } });
    if (options.parentUri) filters.push({ term: { parent_uri: options.parentUri } });
    if (options.maxAgeDays) filters.push({ range: { created_at: { gte: `now-${options.maxAgeDays}d` } } });
    if (!options.includeDeleted) filters.push({ bool: { must_not: { term: { is_deleted: true } } } });

    const nameBoost = fb.name ?? 3;
    const abstractBoost = fb.abstract ?? 2;
    const overviewBoost = fb.overview ?? 1;
    const contentBoost = fb.content ?? 1;

    const should: any[] = [
      { multi_match: { query: queryText, fields: [`name^${nameBoost}`, `abstract^${abstractBoost}`, `overview^${overviewBoost}`, `content^${contentBoost}`], boost: w.keyword * 10, analyzer: "content_analyzer", fuzziness } },
    ];

    // Ngram partial matching on name and abstract
    should.push({ multi_match: { query: queryText, fields: ["name.ngram", "abstract.ngram"], boost: w.keyword * 2 } });

    // Phrase boost
    if (phraseBoost > 0) {
      should.push({ multi_match: { query: queryText, fields: [`name^${nameBoost}`, `abstract^${abstractBoost}`, `overview^${overviewBoost}`, `content^${contentBoost}`], type: "phrase", boost: phraseBoost } });
    }

    const functions: any[] = [];
    functions.push(buildAiQualityFunction());
    if (w.recency > 0) {
      functions.push({
        exp: { created_at: { origin: "now", scale: options.recencyScale || config.elastic.recencyScale, decay: options.recencyDecay || config.elastic.recencyDecay } },
        weight: w.recency * 10,
      });
    }

    const textQuery: any = {
      bool: {
        should,
        ...(filters.length ? { filter: filters } : {}),
        minimum_should_match: 0,
      },
    };

    const query = functions.length > 0
      ? { function_score: { query: textQuery, functions, score_mode: "sum", boost_mode: "sum" } }
      : textQuery;

    const body: any = {
      index: CONTEXT_INDEX,
      size: topK,
      knn: {
        field: "embedding",
        query_vector: queryEmbedding,
        k: topK * CANDIDATE_MULTIPLIER,
        num_candidates: Math.max(100, topK * CANDIDATE_MULTIPLIER * 10),
        boost: w.vector * 10,
        ...(filters.length ? { filter: filters } : {}),
      },
      query,
      _source: { excludes: ["embedding"] },
    };

    if (options.minScore) body.min_score = options.minScore;
    if (options.highlight) body.highlight = buildHighlight(["name", "abstract", "overview", "content"]);

    const res = await this.client.search(body as any);
    return this.mapHits<AgentContextNode>(res, options.highlight);
  }

  async searchContextNodesByVector(queryEmbedding: number[], options: { topK?: number; project?: string; includeDeleted?: boolean } = {}): Promise<(AgentContextNode & { _score: number })[]> {
    await this.ensureInitialized();
    assertEmbeddingDimension(queryEmbedding, "ElasticDB.searchContextNodesByVector");
    const topK = options.topK ?? 10;
    const filters: any[] = [];
    if (options.project) filters.push({ term: { project: options.project } });
    if (!options.includeDeleted) filters.push({ bool: { must_not: { term: { is_deleted: true } } } });

    const res = await this.client.search({
      index: CONTEXT_INDEX,
      size: topK,
      knn: {
        field: "embedding",
        query_vector: queryEmbedding,
        k: topK * CANDIDATE_MULTIPLIER,
        num_candidates: Math.max(100, topK * CANDIDATE_MULTIPLIER * 10),
        ...(filters.length ? { filter: filters } : {}),
      },
      _source: { excludes: ["embedding"] },
    } as any);
    return this.mapHits<AgentContextNode>(res);
  }

  async listContextNodes(parentUri?: string, options?: ContextSearchOptions, limit = 100, offset = 0): Promise<AgentContextNode[]> {
    await this.ensureInitialized();
    const filters: any[] = [];
    if (options?.project) filters.push({ term: { project: options.project } });
    if (parentUri) filters.push({ term: { parent_uri: parentUri } });
    if (!options?.includeDeleted) filters.push({ bool: { must_not: { term: { is_deleted: true } } } });

    const res = await this.client.search({
      index: CONTEXT_INDEX,
      size: limit,
      from: offset,
      query: filters.length ? { bool: { filter: filters } } : { match_all: {} },
      sort: [{ updated_at: { order: "desc" } }],
      _source: { excludes: ["embedding"] },
    });
    return this.mapSources<AgentContextNode>(res);
  }

  async getContextNode(uri: string): Promise<AgentContextNode | null> {
    await this.ensureInitialized();
    try {
      const res = await this.client.get({ index: CONTEXT_INDEX, id: uri, _source_excludes: ["embedding"] });
      return res._source as AgentContextNode;
    } catch (e: any) {
      if (e.meta?.statusCode === 404) return null;
      throw e;
    }
  }

  async deleteContextNode(uri: string) {
    await this.ensureInitialized();
    const ts = new Date().toISOString();
    
    // Soft delete descendants
    await this.client.updateByQuery({
      index: CONTEXT_INDEX,
      query: { term: { ancestors: uri } },
      script: {
        source: "ctx._source.is_deleted = true; ctx._source.deleted_at = params.ts;",
        params: { ts }
      },
      refresh: true,
    }).catch(() => {});

    // Soft delete the node itself
    await this.client.update({
      index: CONTEXT_INDEX,
      id: uri,
      doc: {
        is_deleted: true,
        deleted_at: ts
      },
      refresh: true,
    }).catch((e: any) => {
      if (e.meta?.statusCode !== 404) throw e;
    });
  }

  async restoreContextNode(uri: string) {
    await this.ensureInitialized();
    // Restore descendants
    await this.client.updateByQuery({
      index: CONTEXT_INDEX,
      query: { term: { ancestors: uri } },
      script: {
        source: "ctx._source.is_deleted = false; ctx._source.deleted_at = null;"
      },
      refresh: true,
    }).catch(() => {});

    // Restore the node itself
    await this.client.update({
      index: CONTEXT_INDEX,
      id: uri,
      doc: {
        is_deleted: false,
        deleted_at: null
      },
      refresh: true,
    }).catch((e: any) => {
      if (e.meta?.statusCode !== 404) throw e;
    });
  }

  async getContextSubtree(nodeUri: string, includeDeleted = false): Promise<(AgentContextNode & { depth: number })[]> {
    await this.ensureInitialized();
    const filters: any[] = [];
    if (!includeDeleted) filters.push({ bool: { must_not: { term: { is_deleted: true } } } });

    const res = await this.client.search({
      index: CONTEXT_INDEX,
      size: 1000,
      query: {
        bool: {
          should: [
            { term: { uri: nodeUri } },
            { term: { ancestors: nodeUri } },
          ],
          minimum_should_match: 1,
          filter: filters,
        },
      },
      _source: { excludes: ["embedding"] },
    });

    const nodes = this.mapSources<AgentContextNode & { ancestors?: string[] }>(res);
    const rootNode = nodes.find((n) => n.uri === nodeUri);
    const rootDepth = rootNode && (rootNode as any).ancestors ? (rootNode as any).ancestors.length : 0;

    return nodes
      .map((n) => {
        const nodeAncestors: string[] = (n as any).ancestors || [];
        const depth = nodeAncestors.length - rootDepth;
        const { ancestors: _, ...rest } = n as any;
        return { ...rest, depth } as AgentContextNode & { depth: number };
      })
      .sort((a, b) => a.depth - b.depth);
  }

  async getContextPath(nodeUri: string, includeDeleted = false): Promise<(AgentContextNode & { depth: number })[]> {
    await this.ensureInitialized();
    const node = await this.getContextNodeWithAncestors(nodeUri);
    if (!node) return [];

    const ancestorUris: string[] = (node as any).ancestors || [];
    if (ancestorUris.length === 0) {
      if (!includeDeleted && node.is_deleted) return [];
      return [{ ...node, depth: 0 }];
    }

    const filters: any[] = [];
    if (!includeDeleted) filters.push({ bool: { must_not: { term: { is_deleted: true } } } });

    const res = await this.client.search({
      index: CONTEXT_INDEX,
      size: ancestorUris.length + 1,
      query: {
        bool: {
          should: [
            { terms: { uri: [...ancestorUris, nodeUri] } },
          ],
          minimum_should_match: 1,
          filter: filters,
        },
      },
      _source: { excludes: ["embedding"] },
    });

    const allNodes = this.mapSources<AgentContextNode & { ancestors?: string[] }>(res);

    return allNodes
      .map((n) => {
        const nodeAncestors: string[] = (n as any).ancestors || [];
        const depth = nodeAncestors.length;
        const { ancestors: _, ...rest } = n as any;
        return { ...rest, depth } as AgentContextNode & { depth: number };
      })
      .sort((a, b) => a.depth - b.depth);
  }

  // ---------------------------------------------------------------------------
  // Helpers
  // ---------------------------------------------------------------------------

  private async computeAncestors(parentUri: string | null): Promise<string[]> {
    if (!parentUri) return [];
    const parent = await this.getContextNodeWithAncestors(parentUri);
    if (!parent) return [parentUri];
    const parentAncestors: string[] = (parent as any).ancestors || [];
    return [...parentAncestors, parentUri];
  }

  private async ensureAiFieldMappings(index: string) {
    await this.client.indices.putMapping({
      index,
      properties: {
        ai_intent: { type: "keyword" },
        ai_topics: { type: "keyword" },
        ai_quality_score: { type: "float" },
      },
    } as any);
  }

  private async getContextNodeWithAncestors(uri: string): Promise<(AgentContextNode & { ancestors?: string[] }) | null> {
    try {
      const res = await this.client.get({ index: CONTEXT_INDEX, id: uri, _source_excludes: ["embedding"] });
      return res._source as AgentContextNode & { ancestors?: string[] };
    } catch (e: any) {
      if (e.meta?.statusCode === 404) return null;
      throw e;
    }
  }

  private mapHits<T>(res: any, includeHighlight?: boolean): (T & { _score: number; _highlight?: Record<string, string[]> })[] {
    return (res.hits?.hits || []).map((hit: any) => ({
      ...hit._source,
      _score: hit._score ?? 0,
      ...(includeHighlight && hit.highlight ? { _highlight: hit.highlight } : {}),
    }));
  }

  private mapSources<T>(res: any): T[] {
    return (res.hits?.hits || []).map((hit: any) => hit._source as T);
  }
}

// ---------------------------------------------------------------------------
// Highlight config builder
// ---------------------------------------------------------------------------

function buildHighlight(fields: string[]): any {
  const highlightFields: Record<string, any> = {};
  for (const f of fields) {
    highlightFields[f] = {
      fragment_size: 150,
      number_of_fragments: 2,
      pre_tags: ["<mark>"],
      post_tags: ["</mark>"],
    };
  }
  return { fields: highlightFields };
}

function buildAiQualityFunction() {
  return {
    script_score: { script: { source: "doc['ai_quality_score'].size() > 0 ? Math.max(doc['ai_quality_score'].value, 0) : 0" } },
    weight: AI_QUALITY_FUNCTION_WEIGHT,
  };
}
