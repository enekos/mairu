import { ContextManager } from "../src/ContextManager";
import * as dotenv from "dotenv";
import * as fs from "fs";
import * as path from "path";

dotenv.config({ path: require("path").resolve(__dirname, ".env") });

const url = process.env.TURSO_URL;
const authToken = process.env.TURSO_AUTH_TOKEN;
if (!url) {
  console.error("Please set TURSO_URL in your .env file or environment.");
  process.exit(1);
}

const contextManager = new ContextManager(url, authToken);

async function seed() {
  console.log("Seeding Duo docs into Context DB...");

  const docsDir = path.resolve(__dirname, "../../docs");
  const rootDir = path.resolve(__dirname, "../../");

  try {
    // Add Docs Root
    await contextManager.addContextNode(
      "contextfs://duo/docs",
      "Duo Documentation",
      "Root node for all project documentation",
      "Contains guides, architecture, guidelines, and API specs.",
      undefined,
      "contextfs://duo"
    );
  } catch (e: any) {
    if (!e.message?.includes("UNIQUE")) console.error(e);
  }

  const files = [
    {
      uri: "contextfs://duo/docs/architecture",
      name: "Architecture",
      file: path.join(docsDir, "ARCHITECTURE.md"),
      abstract: "System architecture, layers, data flow",
    },
    {
      uri: "contextfs://duo/docs/agent-guidelines",
      name: "Agent Guidelines",
      file: path.join(docsDir, "AGENT-GUIDELINES.md"),
      abstract: "Rules for AI agents working on Duo codebase",
    },
    {
      uri: "contextfs://duo/docs/soul",
      name: "Soul (Vision)",
      file: path.join(rootDir, "SOUL.md"),
      abstract: "Core vision, principles, and philosophy of Duo",
    },
    {
      uri: "contextfs://duo/docs/rbac",
      name: "RBAC",
      file: path.join(docsDir, "RBAC.md"),
      abstract: "Role-based access control design",
    },
    {
      uri: "contextfs://duo/docs/data-sync",
      name: "Data Sync",
      file: path.join(docsDir, "DATA-SYNC.md"),
      abstract: "Setting up pipelines",
    },
  ];

  for (const f of files) {
    if (fs.existsSync(f.file)) {
      try {
        const content = fs.readFileSync(f.file, "utf8");
        await contextManager.addContextNode(
          f.uri,
          f.name,
          f.abstract,
          `Content of ${f.name}`,
          content, // content
          "contextfs://duo/docs" // parent
        );
        console.log(`Added doc: ${f.name}`);
      } catch (e: any) {
        if (!e.message?.includes("UNIQUE")) console.error(e);
        else console.log(`Doc already exists: ${f.name}`);
      }
    } else {
      console.log(`Warning: File not found ${f.file}`);
    }
  }

  // Also read AGENTS.md
  if (fs.existsSync(path.join(rootDir, "AGENTS.md"))) {
    try {
      const agentsMd = fs.readFileSync(path.join(rootDir, "AGENTS.md"), "utf8");
      await contextManager.addContextNode(
        "contextfs://duo/docs/agents-md",
        "AGENTS.md Map of Territory",
        "Primary entry point for AI agents.",
        "Rules for agents regarding autonomy and exploration.",
        agentsMd,
        "contextfs://duo/docs"
      );
      console.log(`Added doc: AGENTS.md`);
    } catch (e: any) {
      if (!e.message?.includes("UNIQUE")) console.error(e);
      else console.log(`Doc already exists: AGENTS.md`);
    }
  }

  console.log("Docs seeding complete!");
}

seed().catch(console.error);
