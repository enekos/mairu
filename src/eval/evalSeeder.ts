/**
 * Eval fixture seeder — seeds and cleans up test data from eval/dataset.json fixtures.
 * Bypasses the LLM deduplication router and uses explicit IDs for deterministic eval runs.
 */

import { MeilisearchDB } from "../storage/meilisearchDB";
import { Embedder } from "../storage/embedder";
import type { MemoryCategory, MemoryOwner } from "../core/types";

export interface MemoryFixture {
  id: string;
  content: string;
  category: string;
  owner: string;
  importance: number;
  project: string;
  created_at?: string;
}

export interface SkillFixture {
  id: string;
  name: string;
  description: string;
  project: string;
  created_at?: string;
}

export interface ContextFixture {
  uri: string;
  name: string;
  abstract: string;
  overview?: string;
  content?: string;
  parent_uri?: string | null;
  project: string;
}

export interface FixtureSpec {
  memories?: MemoryFixture[];
  skills?: SkillFixture[];
  context?: ContextFixture[];
}

/**
 * Seeds all fixtures into Elasticsearch using explicit IDs.
 * Context nodes are inserted in topological order (parents before children).
 */
export async function seedFixtures(db: MeilisearchDB, fixtures: FixtureSpec, log?: (msg: string) => void): Promise<void> {
  const emit = log ?? (() => {});

  const memories = fixtures.memories ?? [];
  const skills = fixtures.skills ?? [];
  const contextNodes = topologicalSort(fixtures.context ?? []);

  emit(`Seeding ${memories.length} memories...`);
  for (const m of memories) {
    const embedding = await Embedder.getEmbedding(m.content);
    const ts = m.created_at ?? new Date().toISOString();
    await db.addMemory(
      {
        id: m.id,
        content: m.content,
        category: m.category as MemoryCategory,
        owner: m.owner as MemoryOwner,
        importance: m.importance,
        project: m.project,
        metadata: {},
        created_at: ts,
        updated_at: ts,
      },
      embedding
    );
    emit(`  ✓ memory ${m.id}`);
  }

  emit(`Seeding ${skills.length} skills...`);
  for (const s of skills) {
    const embedding = await Embedder.getEmbedding(`${s.name}: ${s.description}`);
    const ts = s.created_at ?? new Date().toISOString();
    await db.addSkill(
      {
        id: s.id,
        name: s.name,
        description: s.description,
        project: s.project,
        metadata: {},
        created_at: ts,
        updated_at: ts,
      },
      embedding
    );
    emit(`  ✓ skill ${s.id}`);
  }

  emit(`Seeding ${contextNodes.length} context nodes...`);
  for (const c of contextNodes) {
    const embedding = await Embedder.getEmbedding(`${c.name}: ${c.abstract}`);
    const ts = new Date().toISOString();
    await db.addContextNode(
      {
        uri: c.uri,
        name: c.name,
        abstract: c.abstract,
        overview: c.overview,
        content: c.content,
        parent_uri: c.parent_uri ?? null,
        project: c.project,
        metadata: {},
        created_at: ts,
        updated_at: ts,
      },
      embedding
    );
    emit(`  ✓ context ${c.uri}`);
  }

  emit("Seeding complete.");
}

/**
 * Deletes all seeded fixtures from Elasticsearch.
 * Context nodes are deleted in reverse topological order (leaves before parents).
 */
export async function cleanupFixtures(db: MeilisearchDB, fixtures: FixtureSpec, log?: (msg: string) => void): Promise<void> {
  const emit = log ?? (() => {});

  for (const m of fixtures.memories ?? []) {
    await db.deleteMemory(m.id);
    emit(`  ✗ deleted memory ${m.id}`);
  }

  for (const s of fixtures.skills ?? []) {
    await db.deleteSkill(s.id);
    emit(`  ✗ deleted skill ${s.id}`);
  }

  const reversed = topologicalSort(fixtures.context ?? []).reverse();
  for (const c of reversed) {
    await db.deleteContextNode(c.uri);
    emit(`  ✗ deleted context ${c.uri}`);
  }

  emit("Cleanup complete.");
}

/** Sort context nodes by URI depth so parents are inserted before children. */
function topologicalSort(nodes: ContextFixture[]): ContextFixture[] {
  return [...nodes].sort((a, b) => {
    const depthA = (a.uri.match(/\//g) ?? []).length;
    const depthB = (b.uri.match(/\//g) ?? []).length;
    return depthA - depthB;
  });
}
