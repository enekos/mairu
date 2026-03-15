import * as dotenv from "dotenv";
import { ContextManager } from "./ContextManager";

dotenv.config({ path: require("path").resolve(__dirname, "..", ".env") });

export function createContextManager(): ContextManager {
  const url = process.env.TURSO_URL;
  const authToken = process.env.TURSO_AUTH_TOKEN;

  if (!url) {
    throw new Error("Please set TURSO_URL in your .env file or environment.");
  }

  return new ContextManager(url, authToken);
}
