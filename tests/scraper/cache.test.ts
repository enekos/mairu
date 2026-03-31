import { describe, it, expect, afterEach } from "vitest";
import { createHash } from "crypto";
import * as fs from "fs";
import * as path from "path";

// We'll test the module functions directly
const tmpCacheFile = path.join("/tmp", `test-scrape-cache-${Date.now()}.json`);

describe("ScrapeCache", () => {
  afterEach(() => {
    if (fs.existsSync(tmpCacheFile)) fs.unlinkSync(tmpCacheFile);
  });

  it("loads empty cache when file does not exist", async () => {
    const { ScrapeCache } = await import("../../src/scraper/cache");
    const cache = new ScrapeCache(tmpCacheFile);
    expect(cache.get("https://example.com")).toBeUndefined();
  });

  it("persists and loads cache entries", async () => {
    const { ScrapeCache } = await import("../../src/scraper/cache");
    const cache = new ScrapeCache(tmpCacheFile);
    cache.set("https://example.com/page", {
      contentHash: "abc123",
      scrapedAt: "2026-01-01T00:00:00Z",
      uri: "contextfs://scraped/example-com/page",
    });
    cache.save();

    const cache2 = new ScrapeCache(tmpCacheFile);
    const entry = cache2.get("https://example.com/page");
    expect(entry?.contentHash).toBe("abc123");
    expect(entry?.uri).toBe("contextfs://scraped/example-com/page");
  });

  it("isUnchanged returns true when content hash matches cached", async () => {
    const { ScrapeCache } = await import("../../src/scraper/cache");
    const cache = new ScrapeCache(tmpCacheFile);
    const content = "hello world";
    const hash = createHash("sha1").update(content).digest("hex");
    cache.set("https://example.com/", {
      contentHash: hash,
      scrapedAt: "2026-01-01T00:00:00Z",
      uri: "contextfs://scraped/example-com",
    });
    expect(cache.isUnchanged("https://example.com/", content)).toBe(true);
    expect(cache.isUnchanged("https://example.com/", "different content")).toBe(false);
  });

  it("isUnchanged returns false for uncached URL", async () => {
    const { ScrapeCache } = await import("../../src/scraper/cache");
    const cache = new ScrapeCache(tmpCacheFile);
    expect(cache.isUnchanged("https://new-url.com/", "any content")).toBe(false);
  });
});
