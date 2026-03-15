import { createClient } from "@libsql/client";
import * as dotenv from "dotenv";
dotenv.config();
const client = createClient({
  url: process.env.TURSO_URL!,
  authToken: process.env.TURSO_AUTH_TOKEN!,
});

async function run() {
  await client.execute("DELETE FROM agent_context_nodes_v2;");
  await client.execute("DELETE FROM agent_memories_v2;");
  await client.execute("DELETE FROM agent_skills_v2;");
  console.log("Tables cleared!");
}
run();
