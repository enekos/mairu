import { describe, it, expect } from "vitest";
import { urlToUri, urlToParentUri, domainSlug, normalizeUrl } from "../../src/scraper/uriMapper";

describe("domainSlug", () => {
  it("converts dots to hyphens and lowercases", () => {
    expect(domainSlug("docs.Example.com")).toBe("docs-example-com");
  });
  it("strips www prefix", () => {
    expect(domainSlug("www.example.com")).toBe("example-com");
  });
  it("handles port numbers", () => {
    expect(domainSlug("localhost:3000")).toBe("localhost-3000");
  });
});

describe("urlToUri", () => {
  it("maps root URL to domain node", () => {
    expect(urlToUri("https://docs.example.com/")).toBe("contextfs://scraped/docs-example-com");
  });
  it("maps path segments to URI segments", () => {
    expect(urlToUri("https://docs.example.com/api/auth")).toBe(
      "contextfs://scraped/docs-example-com/api/auth"
    );
  });
  it("strips trailing slash from path", () => {
    expect(urlToUri("https://docs.example.com/api/")).toBe(
      "contextfs://scraped/docs-example-com/api"
    );
  });
  it("strips query string and fragment", () => {
    expect(urlToUri("https://example.com/page?v=1#section")).toBe(
      "contextfs://scraped/example-com/page"
    );
  });
  it("section slug from heading text", () => {
    expect(urlToUri("https://example.com/page", "My Section Heading")).toBe(
      "contextfs://scraped/example-com/page/my-section-heading"
    );
  });
});

describe("urlToParentUri", () => {
  it("returns parent URI for deep path", () => {
    expect(urlToParentUri("https://docs.example.com/api/auth")).toBe(
      "contextfs://scraped/docs-example-com/api"
    );
  });
  it("returns null for root URI", () => {
    expect(urlToParentUri("https://docs.example.com/")).toBeNull();
  });
  it("returns domain root as parent for single-segment path", () => {
    expect(urlToParentUri("https://docs.example.com/api")).toBe(
      "contextfs://scraped/docs-example-com"
    );
  });
});

describe("normalizeUrl", () => {
  it("strips fragment", () => {
    expect(normalizeUrl("https://example.com/page#section")).toBe("https://example.com/page");
  });
  it("strips trailing slash", () => {
    expect(normalizeUrl("https://example.com/page/")).toBe("https://example.com/page");
  });
  it("keeps root slash intact", () => {
    expect(normalizeUrl("https://example.com/")).toBe("https://example.com");
  });
});
