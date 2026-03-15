import { TursoVectorDB } from "./TursoVectorDB";
import * as dotenv from "dotenv";
import { getEmbeddingConfig } from "./embeddingConfig";

dotenv.config();

const url = process.env.TURSO_URL;
const authToken = process.env.TURSO_AUTH_TOKEN;

async function run() {
  if (!url) {
    console.error("Missing TURSO_URL");
    return;
  }
  const db = new TursoVectorDB(url, authToken);
  const embeddingConfig = getEmbeddingConfig();

  console.log("Adding a test memory...");
  // Fake embedding for example purposes (dimension must match configured EMBEDDING_DIM)
  const embedding = Array(embeddingConfig.dimension).fill(0.1);

  await db.addMemory(
    {
      id: "mem_001",
      content: "User prefers concise answers.",
      category: "observation",
      owner: "agent",
      importance: 5,
      metadata: { source: "conversation_42" },
      created_at: new Date().toISOString(),
    },
    embedding
  );

  console.log("Searching for similar memories...");
  // Find memories similar to the given embedding
  const results = await db.searchMemories("User preference lookup", embedding, { topK: 3 });

  console.log("Results:", results);
}

run().catch(console.error);
