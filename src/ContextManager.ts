import { TursoVectorDB } from "./TursoVectorDB";
import { Embedder } from "./embedder";
import { randomUUID } from "crypto";
import { ContextSearchOptions, MemorySearchOptions, SkillSearchOptions } from "./types";

export class ContextManager {
  private db: TursoVectorDB;
  private normalizeSearchOptions<T extends { topK?: number; threshold?: number }>(
    topKOrOptions?: number | T,
    threshold?: number
  ): T {
    if (typeof topKOrOptions === "number") {
      return { topK: topKOrOptions, threshold } as T;
    }
    return (topKOrOptions ?? {}) as T;
  }


  constructor(url: string, authToken?: string) {
    this.db = new TursoVectorDB(url, authToken);
  }

  async addMemory(
    content: string,
    category: import("./types").MemoryCategory,
    owner: import("./types").MemoryOwner,
    importance: number = 1,
    metadata: Record<string, any> = {}
  ) {
    const embedding = await Embedder.getEmbedding(content);
    const memory = {
      id: `mem_${randomUUID().replace(/-/g, "")}`,
      content,
      category,
      owner,
      importance,
      metadata,
      created_at: new Date().toISOString(),
    };
    await this.db.addMemory(memory, embedding);
    return memory;
  }

  async searchMemories(query: string, topKOrOptions?: number | MemorySearchOptions, threshold?: number) {
    const embedding = await Embedder.getEmbedding(query);
    const options = this.normalizeSearchOptions<MemorySearchOptions>(topKOrOptions, threshold);
    return this.db.searchMemories(query, embedding, options);
  }

  async listMemories(limit: number = 50) {
    return this.db.listMemories(limit);
  }

  async deleteMemory(id: string) {
    return this.db.deleteMemory(id);
  }

  async addSkill(name: string, description: string, metadata: Record<string, any> = {}) {
    const embedding = await Embedder.getEmbedding(`${name}: ${description}`);
    const skill = {
      id: `skill_${randomUUID().replace(/-/g, "")}`,
      name,
      description,
      metadata,
      created_at: new Date().toISOString(),
    };
    await this.db.addSkill(skill, embedding);
    return skill;
  }

  async searchSkills(query: string, topKOrOptions?: number | SkillSearchOptions, threshold?: number) {
    const embedding = await Embedder.getEmbedding(query);
    const options = this.normalizeSearchOptions<SkillSearchOptions>(topKOrOptions, threshold);
    return this.db.searchSkills(query, embedding, options);
  }

  async listSkills(limit: number = 50) {
    return this.db.listSkills(limit);
  }

  async deleteSkill(id: string) {
    return this.db.deleteSkill(id);
  }

  async addContextNode(
    uri: string,
    name: string,
    abstract: string,
    overview?: string,
    content?: string,
    parentUri: string | null = null,
    metadata: Record<string, any> = {}
  ) {
    const embedding = await Embedder.getEmbedding(`${name}: ${abstract}`);
    const node = {
      uri,
      parent_uri: parentUri,
      name,
      abstract,
      overview,
      content,
      metadata,
      created_at: new Date().toISOString(),
    };
    await this.db.addContextNode(node, embedding);
    return node;
  }

  async searchContext(query: string, topKOrOptions?: number | ContextSearchOptions, threshold?: number) {
    const embedding = await Embedder.getEmbedding(query);
    const options = this.normalizeSearchOptions<ContextSearchOptions>(topKOrOptions, threshold);
    return this.db.searchContextNodes(query, embedding, options);
  }

  async listContextNodes(parentUri?: string, limit: number = 50) {
    return this.db.listContextNodes(parentUri, limit);
  }

  async deleteContextNode(uri: string) {
    return this.db.deleteContextNode(uri);
  }

  async getContextSubtree(nodeId: string) {
    return this.db.getContextSubtree(nodeId);
  }

  async getContextPath(nodeId: string) {
    return this.db.getContextPath(nodeId);
  }
}
