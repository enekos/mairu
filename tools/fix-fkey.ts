import { createClient } from "@libsql/client";
import * as dotenv from "dotenv";
dotenv.config({ path: require("path").resolve(__dirname, ".env") });

const client = createClient({
  url: process.env.TURSO_URL!,
  authToken: process.env.TURSO_AUTH_TOKEN!,
});

async function run() {
  const result = await client.execute(`SELECT uri FROM agent_context_nodes`);
  console.log(result.rows.map((r) => r.uri));
}
run();
