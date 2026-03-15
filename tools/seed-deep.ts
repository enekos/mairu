import { ContextManager } from "../src/ContextManager";
import * as dotenv from "dotenv";
import * as path from "path";

dotenv.config({ path: path.resolve(__dirname, ".env") });

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
    console.log(`Added node: ${uri}`);
  } catch (e: any) {
    if (!e.message?.includes("UNIQUE")) console.error(`Error adding node ${uri}:`, e);
  }
}

async function safeAddMemory(content: string, type: string, source: string, importance: number) {
  try {
    await contextManager.addMemory(content, type as any, source as any, importance);
    console.log(`Added memory: ${content.substring(0, 30)}...`);
  } catch (e: any) {
    if (!e.message?.includes("UNIQUE")) console.error(`Error adding memory:`, e);
  }
}

async function seed() {
  console.log("Adding deep architectural context to contextfs...");

  // --- External Services Root ---
  await safeAddNode(
    "contextfs://duo/services",
    "External Services Root",
    "Standalone Python/TS services used by the Duo backend.",
    "Services include LangExtract for document extraction, and various AI scrapers.",
    undefined,
    "contextfs://duo"
  );

  // --- Schema Engine Node ---
  await safeAddNode(
    "contextfs://duo/backend/schema-engine",
    "Dynamic Schema Engine",
    "Handles dynamic table creation, column modifications, and safe SQL generation.",
    "The schema engine maps logical DataModels to physical Postgres tables prefixed with 'dm_'. It strictly validates physical names using isSafePhysicalTableName to prevent SQL injection.",
    undefined,
    "contextfs://duo/backend"
  );

  // --- AI Worker Node ---
  await safeAddNode(
    "contextfs://duo/backend/workers/ai-worker",
    "AI Background Worker",
    "Processes AI column generations asynchronously.",
    "Watches the ai_jobs table. Uses the model configured for the column (e.g., Gemini, OpenAI). Fills in {{column_name}} placeholders in prompts with actual row data.",
    undefined,
    "contextfs://duo/backend/workers"
  );

  // --- Assertion Worker Node ---
  await safeAddNode(
    "contextfs://duo/backend/workers/assertion-worker",
    "Assertion Background Worker",
    "Evaluates data quality rules asynchronously.",
    "Runs assertions defined via Vega expressions against row data to verify data integrity (e.g., row-level constraints).",
    undefined,
    "contextfs://duo/backend/workers"
  );

  // --- Frontend State Node ---
  await safeAddNode(
    "contextfs://duo/frontend/state",
    "Frontend State Management",
    "Pinia stores and TanStack Query.",
    "Uses Pinia for global UI state and TanStack Query (vue-query) for server state, caching, and invalidation. Queries should be strictly typed.",
    undefined,
    "contextfs://duo/frontend"
  );

  // --- LangExtract Node ---
  await safeAddNode(
    "contextfs://duo/services/langextract",
    "LangExtract Service",
    "Python FastAPI service for document OCR and structured extraction.",
    "Exposes /api/cv-extract and /api/langextract. Uses Gemini 2.0 Flash/Pro heavily for document understanding. Requires Google API keys.",
    undefined,
    "contextfs://duo/services"
  );

  // --- Specific Memories ---
  await safeAddMemory(
    "SECURITY: Always use sql.identifier() from slonik or the local query builder when referencing dynamic table or column names to prevent SQL injection.",
    "patterns",
    "system",
    10
  );
  await safeAddMemory(
    "ROUTING: In Hono routes, use `requireAuth` middleware to populate c.get('user') and `optionalTenant` or `requireTenant` middleware for c.get('tenantId').",
    "patterns",
    "system",
    9
  );
  await safeAddMemory(
    "API: Idempotency keys are used in the Public API ingest routes to ensure rows aren't duplicated on retries.",
    "architecture",
    "system",
    8
  );
  await safeAddMemory(
    "TESTING: When writing E2E tests, use Cypress. The Makefile provides 'make e2e' to run them headlessly in Docker.",
    "patterns",
    "system",
    8
  );
  await safeAddMemory(
    "ARCHITECTURE: Keep UI logic inside apps/frontend, public facing forms inside apps/flows, and strictly separate domain logic from infrastructure inside apps/backend.",
    "patterns",
    "system",
    9
  );

  console.log("Deep context seeding complete!");
}

seed().catch(console.error);
