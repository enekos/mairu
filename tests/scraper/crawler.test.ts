import { describe, it, expect } from "vitest";
import { shouldFollowUrl, normalizeLinks, filterLinks } from "../../src/scraper/crawler";

describe("shouldFollowUrl", () => {
  it("allows same-domain URLs", () => {
    expect(shouldFollowUrl("https://docs.example.com/api", "https://docs.example.com")).toBe(true);
  });
  it("rejects different-domain URLs", () => {
    expect(shouldFollowUrl("https://other.com/page", "https://docs.example.com")).toBe(false);
  });
  it("rejects asset URLs", () => {
    expect(shouldFollowUrl("https://docs.example.com/logo.png", "https://docs.example.com")).toBe(false);
    expect(shouldFollowUrl("https://docs.example.com/style.css", "https://docs.example.com")).toBe(false);
    expect(shouldFollowUrl("https://docs.example.com/bundle.js", "https://docs.example.com")).toBe(false);
  });
  it("rejects mailto and tel links", () => {
    expect(shouldFollowUrl("mailto:test@example.com", "https://docs.example.com")).toBe(false);
    expect(shouldFollowUrl("tel:+123456", "https://docs.example.com")).toBe(false);
  });
  it("rejects javascript: links", () => {
    expect(shouldFollowUrl("javascript:void(0)", "https://docs.example.com")).toBe(false);
  });
  it("rejects fragment-only links", () => {
    expect(shouldFollowUrl("#section", "https://docs.example.com")).toBe(false);
  });
  it("respects urlPattern when provided", () => {
    expect(shouldFollowUrl("https://docs.example.com/api/auth", "https://docs.example.com", "/api/.*")).toBe(true);
    expect(shouldFollowUrl("https://docs.example.com/blog/post", "https://docs.example.com", "/api/.*")).toBe(false);
  });
});

describe("normalizeLinks", () => {
  it("resolves relative URLs to absolute", () => {
    const links = normalizeLinks(["/api/auth", "../guide"], "https://docs.example.com/v2/intro");
    expect(links).toContain("https://docs.example.com/api/auth");
  });
  it("strips fragments", () => {
    const links = normalizeLinks(["https://docs.example.com/page#section"], "https://docs.example.com/");
    expect(links).toContain("https://docs.example.com/page");
  });
  it("deduplicates URLs", () => {
    const links = normalizeLinks(
      ["https://docs.example.com/page", "https://docs.example.com/page/"],
      "https://docs.example.com/"
    );
    expect(links.length).toBe(1);
  });
});

describe("filterLinks", () => {
  it("filters to only followable URLs", () => {
    const links = [
      "https://docs.example.com/api",
      "https://other.com/page",
      "https://docs.example.com/logo.png",
    ];
    const result = filterLinks(links, "https://docs.example.com");
    expect(result).toHaveLength(1);
    expect(result[0]).toBe("https://docs.example.com/api");
  });
});
