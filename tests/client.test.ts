import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Mock dotenv
vi.mock("dotenv", () => {
  return {
    config: vi.fn(),
  };
});

// Mock ContextManager
vi.mock("../src/ContextManager", () => {
  return {
    ContextManager: vi.fn().mockImplementation((url, token) => {
      return { url, token };
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

  it("throws an error if TURSO_URL is missing", async () => {
    delete process.env.TURSO_URL;
    const { createContextManager } = await import("../src/client");
    expect(() => createContextManager()).toThrow("Please set TURSO_URL in your .env file or environment.");
  });

  it("returns a new ContextManager with url and token", async () => {
    process.env.TURSO_URL = "http://example.com";
    process.env.TURSO_AUTH_TOKEN = "secret123";

    const { createContextManager } = await import("../src/client");
    const manager = createContextManager() as any;

    expect(manager.url).toBe("http://example.com");
    expect(manager.token).toBe("secret123");
  });

  it("returns a new ContextManager even if TURSO_AUTH_TOKEN is missing", async () => {
    process.env.TURSO_URL = "http://example.com";
    delete process.env.TURSO_AUTH_TOKEN;

    const { createContextManager } = await import("../src/client");
    const manager = createContextManager() as any;

    expect(manager.url).toBe("http://example.com");
    expect(manager.token).toBeUndefined();
  });
});
