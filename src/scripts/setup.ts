import { ElasticDB } from "../storage/elasticDB";
import { config } from "../core/config";

const node = config.elasticUrl;

if (!node) {
  console.error("Please set ELASTIC_URL in your .env file");
  process.exit(1);
}

const auth = config.elasticUsername && config.elasticPassword
  ? { username: config.elasticUsername, password: config.elasticPassword }
  : undefined;

const db = new ElasticDB(node, auth);

async function main() {
  console.log("Resetting and initializing Elasticsearch indices for Agent Context...");
  try {
    await db.resetIndices();
    await db.initIndices();
    console.log("Successfully reset and initialized indices!");
  } catch (err) {
    console.error("Failed to reset/initialize indices:", err);
  }
}

main();
