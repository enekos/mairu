import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

vi.mock("dotenv", () => ({
  config: vi.fn(),
}));

vi.mock("../src/storage/contextManager", () => ({
  ContextManager: vi.fn().mockImplementation((url, apiKey) => {
    return { url, apiKey };
  }),
}));

describe("client", () => {
  const originalEnv = process.env;

  beforeEach(() => {
    if (typeof vi.resetModules === "function") {
      vi.resetModules();
    }
    vi.clearAllMocks();
    process.env = { ...originalEnv };
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  it("creates ContextManager with default MEILI_URL", async () => {
    process.env.MEILI_URL = "http://localhost:7700";
    delete process.env.MEILI_API_KEY;

    const { createContextManager } = await import("../src/storage/client");
    const manager = createContextManager() as any;

    expect(manager.url).toBe("http://localhost:7700");
    expect(manager.apiKey).toBeUndefined();
  });

  it("passes apiKey when set", async () => {
    process.env.MEILI_URL = "http://localhost:7700";
    process.env.MEILI_API_KEY = "my-secret-key";

    const { createContextManager } = await import("../src/storage/client");
    const manager = createContextManager() as any;

    expect(manager.url).toBe("http://localhost:7700");
    expect(manager.apiKey).toBe("my-secret-key");
  });

  it("uses custom MEILI_URL", async () => {
    process.env.MEILI_URL = "http://custom-host:7700";
    delete process.env.MEILI_API_KEY;

    const { createContextManager } = await import("../src/storage/client");
    const manager = createContextManager() as any;

    expect(manager.url).toBe("http://custom-host:7700");
    expect(manager.apiKey).toBeUndefined();
  });
});
