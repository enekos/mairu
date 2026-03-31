export function domainSlug(host: string): string {
  return host
    .toLowerCase()
    .replace(/^www\./, "")
    .replace(/[.:]/g, "-");
}

export function normalizeUrl(url: string): string {
  const parsed = new URL(url);
  parsed.hash = "";
  let result = parsed.origin + parsed.pathname;
  if (result.endsWith("/") && result !== parsed.origin + "/") {
    result = result.slice(0, -1);
  }
  // strip trailing slash even for origin
  return result.replace(/\/$/, "");
}

export function urlToUri(url: string, sectionHeading?: string): string {
  const parsed = new URL(url);
  const slug = domainSlug(parsed.host);
  const pathSegments = parsed.pathname
    .split("/")
    .filter(Boolean)
    .map((s) => encodeURIComponent(s.toLowerCase()));

  const base = ["contextfs://scraped", slug, ...pathSegments].join("/");

  if (sectionHeading) {
    const headingSlug = sectionHeading
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^-|-$/g, "");
    return `${base}/${headingSlug}`;
  }

  return base;
}

export function urlToParentUri(url: string): string | null {
  const uri = urlToUri(url);
  // contextfs://scraped/domain  →  null
  // contextfs://scraped/domain/a  →  contextfs://scraped/domain
  // contextfs://scraped/domain/a/b  →  contextfs://scraped/domain/a
  const parts = uri.split("/");
  // parts: ["contextfs:", "", "scraped", "domain", ...path]
  if (parts.length <= 4) return null; // only domain segment, no parent
  return parts.slice(0, -1).join("/");
}
