import { createContextManager } from "../storage/client";
import { AgentContextNode } from "../core/types";

const contextManager = createContextManager();

async function seed() {
  console.log("Seeding the Ultimate Agent Context DB...");

  // 1. ADD SKILLS
  console.log("--- Adding Skills ---");
  await contextManager.addSkill(
    "Dynamic Schema Architecture",
    "Expertise in handling runtime user-defined tables, dynamic SQL generation, migrations, and schema validation. Knows to NEVER hardcode table names and ALWAYS use parameterized queries with sql.identifier()."
  );
  await contextManager.addSkill(
    "Tenant Isolation",
    "Understanding that every database operation MUST be scoped by tenant_id. Strict enforcement of multi-tenant boundaries."
  );
  await contextManager.addSkill(
    "Hexagonal Architecture (Ports and Adapters)",
    "Strict adherence to backend boundaries: domain (pure logic/types), application (use cases), infrastructure (DB/external), and routes (HTTP)."
  );
  await contextManager.addSkill(
    "Batch & Stream Processing",
    "Defaulting to batch processing for writes and streaming/cursor pagination for reads to scale from 100 to millions of rows without OOM errors."
  );

  // 2. ADD MEMORIES (Observations & Rules)
  console.log("--- Adding Memories ---");
  await contextManager.addMemory(
    "Never run `bun build` or `bun run typecheck` directly on the host machine. These commands consume excessive memory and cause OOM crashes. Always run them via Docker (e.g., `make typecheck`).",
    "observation",
    "agent",
    10
  );
  await contextManager.addMemory(
    "Backend workers (AI, Assertions, Digest, Cleanup) run as separate processes from the same codebase, communicating asynchronously or via DB.",
    "observation",
    "agent",
    8
  );
  await contextManager.addMemory(
    "Duo treats AI as a native column type. Background AI workers process jobs to fill these columns asynchronously based on user prompts with {{column_name}} placeholders.",
    "observation",
    "agent",
    9
  );
  await contextManager.addMemory(
    "Full-Text Search in Duo uses GIN-indexed tsvector across text columns. Search index updates are batched for performance.",
    "observation",
    "agent",
    7
  );

  // 3. ADD HIERARCHICAL CONTEXT NODES
  console.log("--- Adding Hierarchical Context ---");
  // Root node
  const duoRoot = await contextManager.addContextNode(
    "contextfs://duo",
    "Duo Project",
    "A multi-tenant, AI-augmented database layer for business data."
  ) as AgentContextNode;

  // High-level architecture
  const backend = await contextManager.addContextNode(
    "contextfs://duo/backend",
    "Backend (Node.js/Hono)",
    "Contains core logic, schema engine, workers, and public API.",
    undefined,
    undefined,
    duoRoot.uri
  ) as AgentContextNode;
  await contextManager.addContextNode(
    "contextfs://duo/frontend",
    "Frontend (Vue 3 SPA)",
    "Main UI for schema building, data exploration.",
    undefined,
    undefined,
    duoRoot.uri
  );
  await contextManager.addContextNode(
    "contextfs://duo/services",
    "External Services",
    "Python/TS services for external processing.",
    undefined,
    undefined,
    duoRoot.uri
  );

  // Deep diving into Backend
  await contextManager.addContextNode(
    "contextfs://duo/backend/domain",
    "Domain Layer",
    "Pure logic, types, and ports (interfaces). No I/O allowed here.",
    undefined,
    undefined,
    backend.uri
  );
  await contextManager.addContextNode(
    "contextfs://duo/backend/application",
    "Application Layer",
    "Use cases and orchestration. Injects dependencies. No direct DB/HTTP.",
    undefined,
    undefined,
    backend.uri
  );
  await contextManager.addContextNode(
    "contextfs://duo/backend/infrastructure",
    "Infrastructure Layer",
    "Implementations, database repositories, external services.",
    undefined,
    undefined,
    backend.uri
  );
  await contextManager.addContextNode(
    "contextfs://duo/backend/routes",
    "Routes Layer",
    "HTTP entrypoints via Hono. Parse input, call use cases, format responses.",
    undefined,
    undefined,
    backend.uri
  );

  console.log("Seeding complete! Opencode and Claude Code now have ultimate knowledge of Duo.");
}

seed().catch(console.error);
