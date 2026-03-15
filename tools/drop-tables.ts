import { createClient } from "@libsql/client";
import * as dotenv from "dotenv";
dotenv.config();
const client = createClient({
  url: process.env.TURSO_URL!,
  authToken: process.env.TURSO_AUTH_TOKEN!,
});

async function run() {
  await client.execute("DROP INDEX IF EXISTS idx_skills_vec;");
  await client.execute("DROP INDEX IF EXISTS idx_memories_vec;");
  await client.execute("DROP INDEX IF EXISTS idx_context_nodes_vec;");
  await client.execute("DROP TABLE IF EXISTS agent_context_nodes;");
  await client.execute("DROP TABLE IF EXISTS agent_memories;");
  await client.execute("DROP TABLE IF EXISTS agent_skills;");
  console.log("Tables and indexes dropped!");
}
run();
