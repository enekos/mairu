import { randomUUID } from "crypto";
import { TursoVectorDB } from "./TursoVectorDB";
import { Embedder } from "./embedder";
import {
  ContextSearchOptions,
  MemoryCategory,
  MemoryOwner,
  MemorySearchOptions,
  SkillSearchOptions,
  SkippedWrite,
  UpdatedWrite,
} from "./types";
import {
  hybridRerank,
  DEFAULT_MEMORY_WEIGHTS,
  DEFAULT_SKILL_WEIGHTS,
  DEFAULT_CONTEXT_WEIGHTS,
  HybridWeights,
} from "./scorer";
import { decideMemoryAction, decideContextAction, RouterCandidate } from "./llmRouter";

export class ContextManager {
  private db: TursoVectorDB;

  constructor(url: string, authToken?: string) {
    this.db = new TursoVectorDB(url, authToken);
  }

  // ---------------------------------------------------------------------------
  // Skills
  // ---------------------------------------------------------------------------

  async addSkill(name: string, description: string, metadata: Record<string, any> = {}) {
    const embedding = await Embedder.getEmbedding(`${name}: ${description}`);
    const skill = {
      id: `skill_${randomUUID().replace(/-/g, "")}`,
      name,
      description,
      metadata,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };
    await this.db.addSkill(skill, embedding);
    return skill;
  }

  async updateSkill(id: string, updates: { name?: string; description?: string; metadata?: Record<string, any> }) {
    let embedding: number[] | undefined;
    if (updates.name !== undefined || updates.description !== undefined) {
      // Re-embed with updated name/description (need current values if partial)
      const current = await this.db.listSkills(1000).then((rows) => rows.find((r: any) => r.id === id));
      const name = updates.name ?? (current as any)?.name ?? "";
      const description = updates.description ?? (current as any)?.description ?? "";
      embedding = await Embedder.getEmbedding(`${name}: ${description}`);
    }
    await this.db.updateSkill(id, updates, embedding);
    return { id, updated: true };
  }

  async searchSkills(query: string, topKOrOptions?: number | SkillSearchOptions, threshold?: number) {
    const opts = this.normalizeOptions<SkillSearchOptions>(topKOrOptions, threshold);
    const topK = opts.topK ?? 10;
    const embedding = await Embedder.getEmbedding(query);
    const candidates = await this.db.searchSkills(embedding, opts);

    const weights: HybridWeights = {
      vector: opts.weights?.vector ?? DEFAULT_SKILL_WEIGHTS.vector,
      keyword: opts.weights?.keyword ?? DEFAULT_SKILL_WEIGHTS.keyword,
      recency: opts.weights?.recency ?? DEFAULT_SKILL_WEIGHTS.recency,
      importance: 0,
    };

    const ranked = hybridRerank(candidates, query, ["name", "description"], weights);
    const results = opts.threshold !== undefined
      ? ranked.filter((r) => r.distance <= opts.threshold!)
      : ranked;

    return results.slice(0, topK);
  }

  async listSkills(limit = 100) {
    return this.db.listSkills(limit);
  }

  async deleteSkill(id: string) {
    return this.db.deleteSkill(id);
  }

  // ---------------------------------------------------------------------------
  // Memories
  // ---------------------------------------------------------------------------

  /**
   * Smart add: embeds content, searches for similar existing memories,
   * and uses the LLM router to decide create/update/skip.
   */
  async addMemory(
    content: string,
    category: MemoryCategory,
    owner: MemoryOwner,
    importance = 1,
    metadata: Record<string, any> = {},
    useRouter = true
  ): Promise<any | SkippedWrite | UpdatedWrite> {
    const embedding = await Embedder.getEmbedding(content);

    if (useRouter) {
      // Fetch top similar candidates for the LLM to consider
      const candidates = await this.db.searchMemories(embedding, { topK: 5 });
      const ranked = hybridRerank(candidates, content, ["content"], DEFAULT_MEMORY_WEIGHTS);

      const routerCandidates: RouterCandidate[] = ranked.slice(0, 5).map((r) => ({
        id: r.id as string,
        content: r.content as string,
        score: r._hybrid_score,
      }));

      const decision = await decideMemoryAction(content, routerCandidates);

      if (decision.action === "skip") {
        return { skipped: true, reason: decision.reason, existingId: routerCandidates[0]?.id ?? "" };
      }

      if (decision.action === "update") {
        const mergedEmbedding = await Embedder.getEmbedding(decision.mergedContent);
        await this.db.updateMemory(
          decision.targetId,
          { content: decision.mergedContent, importance: Math.max(importance, 1) },
          mergedEmbedding
        );
        return { updated: true, id: decision.targetId };
      }
    }

    // Create
    const memory = {
      id: `mem_${randomUUID().replace(/-/g, "")}`,
      content,
      category,
      owner,
      importance,
      metadata,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };
    await this.db.addMemory(memory, embedding);
    return memory;
  }

  async updateMemory(
    id: string,
    updates: { content?: string; importance?: number; metadata?: Record<string, any> }
  ) {
    let embedding: number[] | undefined;
    if (updates.content !== undefined) {
      embedding = await Embedder.getEmbedding(updates.content);
    }
    await this.db.updateMemory(id, updates, embedding);
    return { id, updated: true };
  }

  async searchMemories(query: string, topKOrOptions?: number | MemorySearchOptions, threshold?: number) {
    const opts = this.normalizeOptions<MemorySearchOptions>(topKOrOptions, threshold);
    const topK = opts.topK ?? 10;
    const embedding = await Embedder.getEmbedding(query);
    const candidates = await this.db.searchMemories(embedding, opts);

    const weights: HybridWeights = {
      vector: opts.weights?.vector ?? DEFAULT_MEMORY_WEIGHTS.vector,
      keyword: opts.weights?.keyword ?? DEFAULT_MEMORY_WEIGHTS.keyword,
      recency: opts.weights?.recency ?? DEFAULT_MEMORY_WEIGHTS.recency,
      importance: opts.weights?.importance ?? DEFAULT_MEMORY_WEIGHTS.importance,
    };

    const ranked = hybridRerank(candidates, query, ["content"], weights, "importance");
    const results = opts.threshold !== undefined
      ? ranked.filter((r) => r.distance <= opts.threshold!)
      : ranked;

    return results.slice(0, topK);
  }

  async listMemories(limit = 100) {
    return this.db.listMemories(limit);
  }

  async deleteMemory(id: string) {
    return this.db.deleteMemory(id);
  }

  // ---------------------------------------------------------------------------
  // Context Nodes
  // ---------------------------------------------------------------------------

  /**
   * Smart add: uses LLM router to decide create/update/skip for context nodes.
   */
  async addContextNode(
    uri: string,
    name: string,
    abstract: string,
    overview?: string,
    content?: string,
    parentUri: string | null = null,
    metadata: Record<string, any> = {},
    useRouter = true
  ): Promise<any | SkippedWrite | UpdatedWrite> {
    const embedding = await Embedder.getEmbedding(`${name}: ${abstract}`);

    if (useRouter) {
      const candidates = await this.db.searchContextNodes(embedding, { topK: 5 });
      const ranked = hybridRerank(candidates, `${name}: ${abstract}`, ["name", "abstract"], DEFAULT_CONTEXT_WEIGHTS);

      const routerCandidates: RouterCandidate[] = ranked.slice(0, 5).map((r) => ({
        id: r.uri as string,
        content: r.abstract as string,
        score: r._hybrid_score,
      }));

      const decision = await decideContextAction(uri, name, abstract, routerCandidates);

      if (decision.action === "skip") {
        return { skipped: true, reason: decision.reason, existingId: routerCandidates[0]?.id ?? "" };
      }

      if (decision.action === "update") {
        const mergedEmbedding = await Embedder.getEmbedding(`${name}: ${decision.mergedContent}`);
        await this.db.updateContextNode(
          decision.targetId,
          { abstract: decision.mergedContent, overview, content },
          mergedEmbedding
        );
        return { updated: true, id: decision.targetId };
      }
    }

    // Create
    const node = {
      uri,
      parent_uri: parentUri,
      name,
      abstract,
      overview,
      content,
      metadata,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };
    await this.db.addContextNode(node, embedding);
    return node;
  }

  async updateContextNode(
    uri: string,
    updates: { abstract?: string; overview?: string; content?: string; metadata?: Record<string, any> }
  ) {
    let embedding: number[] | undefined;
    if (updates.abstract !== undefined) {
      embedding = await Embedder.getEmbedding(updates.abstract);
    }
    await this.db.updateContextNode(uri, updates, embedding);
    return { uri, updated: true };
  }

  async searchContext(query: string, topKOrOptions?: number | ContextSearchOptions, threshold?: number) {
    const opts = this.normalizeOptions<ContextSearchOptions>(topKOrOptions, threshold);
    const topK = opts.topK ?? 10;
    const embedding = await Embedder.getEmbedding(query);
    const candidates = await this.db.searchContextNodes(embedding, opts);

    const weights: HybridWeights = {
      vector: opts.weights?.vector ?? DEFAULT_CONTEXT_WEIGHTS.vector,
      keyword: opts.weights?.keyword ?? DEFAULT_CONTEXT_WEIGHTS.keyword,
      recency: opts.weights?.recency ?? DEFAULT_CONTEXT_WEIGHTS.recency,
      importance: 0,
    };

    // Search across all text layers for best keyword overlap
    const ranked = hybridRerank(candidates, query, ["name", "abstract", "overview", "content"], weights);
    const results = opts.threshold !== undefined
      ? ranked.filter((r) => r.distance <= opts.threshold!)
      : ranked;

    return results.slice(0, topK);
  }

  async listContextNodes(parentUri?: string, limit = 100) {
    return this.db.listContextNodes(parentUri, limit);
  }

  async deleteContextNode(uri: string) {
    return this.db.deleteContextNode(uri);
  }

  async getContextSubtree(nodeUri: string) {
    return this.db.getContextSubtree(nodeUri);
  }

  async getContextPath(nodeUri: string) {
    return this.db.getContextPath(nodeUri);
  }

  // ---------------------------------------------------------------------------
  // Internal helpers
  // ---------------------------------------------------------------------------

  private normalizeOptions<T extends { topK?: number; threshold?: number }>(
    topKOrOptions?: number | T,
    threshold?: number
  ): T {
    if (typeof topKOrOptions === "number") {
      return { topK: topKOrOptions, threshold } as T;
    }
    return (topKOrOptions ?? {}) as T;
  }
}
