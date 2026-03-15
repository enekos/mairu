import { ContextManager } from "../src/ContextManager";
import * as dotenv from "dotenv";
dotenv.config({ path: require("path").resolve(__dirname, ".env") });

const url = process.env.TURSO_URL;
const authToken = process.env.TURSO_AUTH_TOKEN;
if (!url) process.exit(1);

const contextManager = new ContextManager(url, authToken);

async function seed() {
  console.log("Adding missing parent node first...");
  await contextManager.addContextNode(
    "contextfs://duo/backend/routes",
    "Routes Layer",
    "HTTP entrypoints",
    "HTTP handlers connecting to application use cases",
    undefined,
    "contextfs://duo/backend"
  );

  console.log("Adding more context nodes to OpenContextFS DB...");

  // --- REPORTS ---
  await contextManager.addContextNode(
    "contextfs://duo/backend/routes/reports",
    "Reports API",
    "Handles custom user-built SELECT queries, assertions, and drill-downs.",
    "Provides export (reportResultToCsv), drilldown cells (drilldownReportCell, drilldownPreviewSql), and formula validation via Vega formulas. Fully respects user-tenant access.",
    undefined,
    "contextfs://duo/backend/routes"
  );

  // --- DASHBOARDS ---
  await contextManager.addContextNode(
    "contextfs://duo/backend/routes/dashboards",
    "Dashboards API",
    "CRUD for dashboards and their widgets.",
    "Follows the same patterns as /api/reports. Dashboards are global entities but are strictly run with a tenantId for data context. Stored in Elasticsearch via buildDashboardDocument.",
    undefined,
    "contextfs://duo/backend/routes"
  );

  // --- NOTEBOOKS ---
  await contextManager.addContextNode(
    "contextfs://duo/backend/routes/notebooks",
    "Notebooks API",
    "Interactive scratchpads with reorderable execution cells.",
    "Uses Use Cases exclusively: listNotebooks, createNotebook, addCell, updateCell, executeCell, reorderCells. Tenant required on all operations.",
    undefined,
    "contextfs://duo/backend/routes"
  );

  // --- AGENTS ---
  await contextManager.addContextNode(
    "contextfs://duo/backend/routes/agents",
    "Swarm Agents API",
    "Integration with Langsmith, Langgraph, and autonomous agents.",
    "Routes logic to src/agents/ using buildAgent and buildSwarmAgent. Supports streamSSE for real-time agent output streaming. Tracked heavily via Prometheus metrics (agentExecutionsTotal).",
    undefined,
    "contextfs://duo/backend/routes"
  );

  // --- LOGS ---
  await contextManager.addContextNode(
    "contextfs://duo/backend/routes/logs",
    "Elastic Logs API",
    "Routes query requests to Elasticsearch queryLogs function.",
    "Allows querying logs with filters for level (error, warn, info, debug), service, requestId, tenantId, and userId.",
    undefined,
    "contextfs://duo/backend/routes"
  );

  // --- MEMORY RULE FOR EMBEDDINGS ---
  await contextManager.addMemory(
    "When adding large ENUM columns to dynamic tables, do NOT pass all enum values into the LLM prompt. Use the 'Embeddings Service' (src/services/embeddings.ts) to match the LLM's raw text output to the closest valid enum using cosine similarity.",
    "patterns",
    "system",
    10
  );

  console.log("Finished adding additional context!");
}

seed().catch(console.error);
