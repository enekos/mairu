import * as dotenv from "dotenv";
import { ContextManager } from "./ContextManager";
import { config } from "./config";

export function createContextManager(): ContextManager {
  const url = config.tursoUrl;
  const authToken = config.tursoAuthToken;

  if (!url) {
    throw new Error("Please set TURSO_URL in your .env file or environment.");
  }

  return new ContextManager(url, authToken);
}
