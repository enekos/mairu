import { MeiliSearch } from "meilisearch";
import { SKILLS_INDEX, MEMORIES_INDEX, CONTEXT_INDEX } from "../src/storage/meilisearchDB";
import * as dotenv from "dotenv";
dotenv.config();

const client = new MeiliSearch({
  host: process.env.MEILI_URL || "http://localhost:7700",
  apiKey: process.env.MEILI_API_KEY || "",
});

async function run() {
  for (const uid of [CONTEXT_INDEX, MEMORIES_INDEX, SKILLS_INDEX]) {
    try {
      const task = await client.index(uid).deleteAllDocuments();
      await client.waitForTask(task.taskUid);
    } catch (e: any) {
      if (e?.code !== "index_not_found") console.error(`Error clearing ${uid}:`, e);
    }
  }
  console.log("Indices cleared!");
}
run();
