import { ContextManager } from "./contextManager";
import { config } from "../core/config";

export function createContextManager(): ContextManager {
  const node = config.elasticUrl;

  if (!node) {
    throw new Error("Please set ELASTIC_URL in your .env file or environment.");
  }

  const auth = config.elasticUsername && config.elasticPassword
    ? { username: config.elasticUsername, password: config.elasticPassword }
    : undefined;

  return new ContextManager(node, auth);
}
