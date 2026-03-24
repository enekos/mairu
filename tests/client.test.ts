import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Mock dotenv
vi.mock("dotenv", () => {
  return {
    config: vi.fn(),
  };
});

// Mock ContextManager
vi.mock("../src/storage/contextManager", () => {
  return {
    ContextManager: vi.fn().mockImplementation((node, auth) => {
      return { node, auth };
    }),
  };
});

describe("client", () => {
  const originalEnv = process.env;

  beforeEach(() => {
    if (typeof vi.resetModules === "function") {
      vi.resetModules();
    } else {
      for (const key in require.cache) {
        if (key.includes("/src/")) {
          delete require.cache[key];
        }
      }
    }
    vi.clearAllMocks();
    process.env = { ...originalEnv };
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  it("creates ContextManager with default ELASTIC_URL", async () => {
    process.env.ELASTIC_URL = "http://localhost:9200";
    delete process.env.ELASTIC_USERNAME;
    delete process.env.ELASTIC_PASSWORD;

    const { createContextManager } = await import("../src/storage/client");
    const manager = createContextManager() as any;

    expect(manager.node).toBe("http://localhost:9200");
    expect(manager.auth).toBeUndefined();
  });

  it("passes auth when username and password are set", async () => {
    process.env.ELASTIC_URL = "http://localhost:9200";
    process.env.ELASTIC_USERNAME = "elastic";
    process.env.ELASTIC_PASSWORD = "changeme";

    const { createContextManager } = await import("../src/storage/client");
    const manager = createContextManager() as any;

    expect(manager.node).toBe("http://localhost:9200");
    expect(manager.auth).toEqual({ username: "elastic", password: "changeme" });
  });

  it("works without auth when only ELASTIC_URL is set", async () => {
    process.env.ELASTIC_URL = "http://custom:9200";
    delete process.env.ELASTIC_USERNAME;
    delete process.env.ELASTIC_PASSWORD;

    const { createContextManager } = await import("../src/storage/client");
    const manager = createContextManager() as any;

    expect(manager.node).toBe("http://custom:9200");
    expect(manager.auth).toBeUndefined();
  });
});
