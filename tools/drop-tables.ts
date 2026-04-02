import { MeilisearchDB } from "../src/storage/meilisearchDB";
import * as dotenv from "dotenv";
dotenv.config();

const db = new MeilisearchDB(
  process.env.MEILI_URL || "http://localhost:7700",
  process.env.MEILI_API_KEY || ""
);

async function run() {
  await db.resetIndices();
  console.log("Indices dropped!");
}
run();
