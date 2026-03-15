import { ContextManager } from "../src/ContextManager";
import * as dotenv from "dotenv";
dotenv.config({ path: require("path").resolve(__dirname, ".env") });

const url = process.env.TURSO_URL;
const authToken = process.env.TURSO_AUTH_TOKEN;
if (!url) process.exit(1);

const contextManager = new ContextManager(url, authToken);

async function safeAddNode(
  uri: string,
  name: string,
  abstract: string,
  overview?: string,
  content?: string,
  parent?: string
) {
  try {
    await contextManager.addContextNode(uri, name, abstract, overview, content, parent);
  } catch (e: any) {
    if (!e.message?.includes("UNIQUE")) console.error(e);
  }
}

async function safeAddMemory(content: string, type: string, source: string, importance: number) {
  try {
    await contextManager.addMemory(content, type as any, source as any, importance);
  } catch (e: any) {
    if (!e.message?.includes("UNIQUE")) console.error(e);
  }
}

async function seed() {
  console.log("Seeding Duo architecture and concepts into OpenContextFS Context DB...");

  // --- ROOT ---
  await safeAddNode(
    "contextfs://duo",
    "Duo Architecture",
    "An AI-Native Data Management Platform using dynamic schemas and multi-tenant architecture.",
    "Duo is a multi-tenant, schema-driven database layer where users define schemas at runtime. AI is a first-class column type, and data can be ingested from structured and unstructured sources.",
    undefined
  );

  // --- BACKEND ---
  await safeAddNode(
    "contextfs://duo/backend",
    "Backend Application",
    "Node.js/Hono API. Handles auth, tenant management, schema engine, data CRUD, AI job queue, ingestion.",
    "The backend is strictly layered using hexagonal architecture. Always respect boundaries: Domain, Application, Infrastructure, Routes. Uses PostgreSQL 17 (PostGIS), Redis, and MinIO.",
    undefined,
    "contextfs://duo"
  );

  await safeAddNode(
    "contextfs://duo/backend/workers",
    "Backend Workers",
    "Background processes defined in src/worker/ running on the same codebase but as separate processes.",
    "Workers include: ai-worker (fills AI columns), assertion-worker (evaluates data quality rules), digest-worker (sends notifications), cleanup-worker (removes expired data).",
    undefined,
    "contextfs://duo/backend"
  );

  await safeAddNode(
    "contextfs://duo/backend/layers",
    "Hexagonal Architecture Layers",
    "Domain, Application, Infrastructure, and Routes.",
    "Domain (src/domain/): Types, ports (interfaces), pure logic. No I/O.\nApplication (src/application/): Use cases and orchestration. Injects dependencies. No direct DB/HTTP.\nInfrastructure (src/infrastructure/): Port implementations: repositories, external services.\nRoutes (src/routes/): HTTP entrypoints. Parse input (Zod), call use cases, format responses.",
    undefined,
    "contextfs://duo/backend"
  );

  // --- FRONTENDS ---
  await safeAddNode(
    "contextfs://duo/frontend",
    "Frontend Application",
    "Vue 3 SPA using Composition API, Pinia, and TanStack Query.",
    "Main UI for schema building, data exploration, queries, reports, search, audit, team management, and flows.",
    undefined,
    "contextfs://duo"
  );

  await safeAddNode(
    "contextfs://duo/flows",
    "Flows Application",
    "Standalone Vue 3 SPA for guest-facing data collection.",
    "No login required. Each flow has a slug-based URL where respondents complete multi-step forms mapped to underlying dynamic data models.",
    undefined,
    "contextfs://duo"
  );

  // --- DATABASE & SCHEMA ---
  await safeAddNode(
    "contextfs://duo/database",
    "Database & Dynamic Schema",
    "PostgreSQL 17. System tables are fixed, user tables are dynamic.",
    "System tables: users, tenants, schema_versions, flows, assertions, ai_jobs, search_index. User-defined tables are prefixed with 'dm_' (e.g. dm_orders). Schema changes generate migrations recorded in schema_versions. Code MUST NOT hardcode table names or columns. Always use parameterized queries and sql.identifier().",
    undefined,
    "contextfs://duo"
  );

  // --- MEMORIES ---
  await safeAddMemory(
    "Commit convention: Use conventional commits (feat, fix, chore, docs, refactor, test). Subject must be lowercase, max 100 chars.",
    "patterns",
    "system",
    7
  );

  await safeAddMemory(
    "Testing: Backend unit/integration tests run via 'npm run test'. Inside apps/backend, use 'bun run test:watch' or 'bun run test:e2e'.",
    "patterns",
    "system",
    8
  );

  console.log("Seeding complete!");
}

seed().catch(console.error);
