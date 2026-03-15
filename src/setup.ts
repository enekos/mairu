import * as dotenv from "dotenv";
import { TursoVectorDB } from "./TursoVectorDB";

dotenv.config({ path: require("path").resolve(__dirname, "..", ".env") });

const url = process.env.TURSO_URL;
const authToken = process.env.TURSO_AUTH_TOKEN;

if (!url) {
  console.error("Please set TURSO_URL and TURSO_AUTH_TOKEN in your .env file");
  process.exit(1);
}

// setup.ts directly uses TursoVectorDB (not ContextManager) to init tables before embedder is available
const db = new TursoVectorDB(url, authToken);

async function main() {
  console.log("Resetting and initializing Turso Vector Database for Agent Context...");
  try {
    await db.resetTables();
    await db.initTables();
    console.log("Successfully reset and initialized tables!");
  } catch (err) {
    console.error("Failed to reset/initialize tables:", err);
  }
}

main();
