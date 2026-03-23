import { randomUUID } from "crypto";
import { ElasticDB } from "./elasticDB";
import { Embedder } from "./embedder";
import {
  AgentContextNode,
  AgentMemory,
  ContextSearchOptions,
  MemoryCategory,
  MemoryOwner,
  MemorySearchOptions,
  SkillSearchOptions,
  SkippedWrite,
  UpdatedWrite,
} from "./types";
import { decideMemoryAction, decideContextAction, RouterCandidate } from "./llmRouter";
import { parseTextIntoContextNodes, ProposedContextNode } from "./ingestor";

export type { ProposedContextNode };

export class ContextManager {
  private db: ElasticDB;

  constructor(node: string, auth?: { username: string; password: string }) {
    this.db = new ElasticDB(node, auth);
  }

  // ---------------------------------------------------------------------------
  // Skills
  // ---------------------------------------------------------------------------

  async addSkill(name: string, description: string, project?: string, metadata: Record<string, any> = {}) {
    const embedding = await Embedder.getEmbedding(`${name}: ${description}`);
    const skill = {
      id: `skill_${randomUUID().replace(/-/g, "")}`,
      project,
      name,
      description,
      metadata,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };
    await this.db.addSkill(skill, embedding);
    return skill;
  }

  async updateSkill(id: string, updates: { name?: string; description?: string; project?: string; metadata?: Record<string, any> }) {
    let embedding: number[] | undefined;
    const current = await this.db.getSkill(id);

    if (current && (
      (updates.name !== undefined && updates.name !== current.name) ||
      (updates.description !== undefined && updates.description !== current.description)
    )) {
      const name = updates.name ?? current.name ?? "";
      const description = updates.description ?? current.description ?? "";
      embedding = await Embedder.getEmbedding(`${name}: ${description}`);
    }
    await this.db.updateSkill(id, updates, embedding);
    return { id, updated: true };
  }

  async searchSkills(query: string, topKOrOptions?: number | SkillSearchOptions, threshold?: number) {
    const opts = this.normalizeOptions<SkillSearchOptions>(topKOrOptions, threshold);
    const topK = opts.topK ?? 10;
    const embedding = await Embedder.getEmbedding(query);
    return this.db.searchSkills(embedding, query, { ...opts, topK });
  }

  async listSkills(options?: SkillSearchOptions, limit = 100) {
    return this.db.listSkills(options, limit);
  }

  async deleteSkill(id: string) {
    return this.db.deleteSkill(id);
  }

  // ---------------------------------------------------------------------------
  // Memories
  // ---------------------------------------------------------------------------

  async addMemory(
    content: string,
    category: MemoryCategory,
    owner: MemoryOwner,
    importance = 1,
    project?: string,
    metadata: Record<string, any> = {},
    useRouter = true
  ): Promise<AgentMemory | SkippedWrite | UpdatedWrite> {
    const embedding = await Embedder.getEmbedding(content);

    if (useRouter) {
      // Use vector-only search for dedup — scores are pure cosine similarity in [0, 1]
      const candidates = await this.db.searchMemoriesByVector(embedding, { topK: 5, project });

      const routerCandidates: RouterCandidate[] = candidates.slice(0, 5).map((r) => ({
        id: r.id as string,
        content: r.content as string,
        score: r._score,
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
      project,
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
    updates: { content?: string; importance?: number; project?: string; metadata?: Record<string, any> }
  ) {
    let embedding: number[] | undefined;
    const current = await this.db.getMemory(id);

    if (current && updates.content !== undefined && updates.content !== current.content) {
      embedding = await Embedder.getEmbedding(updates.content);
    }
    await this.db.updateMemory(id, updates, embedding);
    return { id, updated: true };
  }

  async searchMemories(query: string, topKOrOptions?: number | MemorySearchOptions, threshold?: number) {
    const opts = this.normalizeOptions<MemorySearchOptions>(topKOrOptions, threshold);
    const topK = opts.topK ?? 10;
    const embedding = await Embedder.getEmbedding(query);
    return this.db.searchMemories(embedding, query, { ...opts, topK });
  }

  async listMemories(options?: MemorySearchOptions, limit = 100) {
    return this.db.listMemories(options, limit);
  }

  async deleteMemory(id: string) {
    return this.db.deleteMemory(id);
  }

  // ---------------------------------------------------------------------------
  // Context Nodes
  // ---------------------------------------------------------------------------

  async addContextNode(
    uri: string,
    name: string,
    abstract: string,
    overview?: string,
    content?: string,
    parentUri: string | null = null,
    project?: string,
    metadata: Record<string, any> = {},
    useRouter = true
  ): Promise<AgentContextNode | SkippedWrite | UpdatedWrite> {
    const embedding = await Embedder.getEmbedding(`${name}: ${abstract}`);

    if (useRouter) {
      const candidates = await this.db.searchContextNodesByVector(embedding, { topK: 5, project });

      const routerCandidates: RouterCandidate[] = candidates.slice(0, 5).map((r) => ({
        id: r.uri as string,
        content: r.abstract as string,
        score: r._score,
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
      project,
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
    updates: { name?: string; abstract?: string; overview?: string; content?: string; project?: string; metadata?: Record<string, any> }
  ) {
    let embedding: number[] | undefined;
    const current = await this.db.getContextNode(uri);

    if (current && (
      (updates.name !== undefined && updates.name !== current.name) ||
      (updates.abstract !== undefined && updates.abstract !== current.abstract)
    )) {
      const name = updates.name ?? current.name ?? "";
      const abstract = updates.abstract ?? current.abstract ?? "";
      embedding = await Embedder.getEmbedding(`${name}: ${abstract}`);
    }
    await this.db.updateContextNode(uri, updates, embedding);
    return { uri, updated: true };
  }

  async searchContext(query: string, topKOrOptions?: number | ContextSearchOptions, threshold?: number) {
    const opts = this.normalizeOptions<ContextSearchOptions>(topKOrOptions, threshold);
    const topK = opts.topK ?? 10;
    const embedding = await Embedder.getEmbedding(query);
    return this.db.searchContextNodes(embedding, query, { ...opts, topK });
  }

  async listContextNodes(parentUri?: string, options?: ContextSearchOptions, limit = 100) {
    return this.db.listContextNodes(parentUri, options, limit);
  }

  async deleteContextNode(uri: string) {
    return this.db.deleteContextNode(uri);
  }

  async restoreContextNode(uri: string) {
    return this.db.restoreContextNode(uri);
  }

  async getContextSubtree(nodeUri: string) {
    return this.db.getContextSubtree(nodeUri);
  }

  async getContextPath(nodeUri: string) {
    return this.db.getContextPath(nodeUri);
  }

  // ---------------------------------------------------------------------------
  // Cluster Stats
  // ---------------------------------------------------------------------------

  async getClusterStats() {
    return this.db.getClusterStats();
  }

  // ---------------------------------------------------------------------------
  // Ingest
  // ---------------------------------------------------------------------------

  async parseIngestText(text: string, baseUri?: string): Promise<ProposedContextNode[]> {
    return parseTextIntoContextNodes(text, baseUri);
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
