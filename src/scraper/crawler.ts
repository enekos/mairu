import { chromium } from "playwright";
import type { CrawlOptions, CrawledPage } from "./types";

const ASSET_EXTENSIONS = /\.(png|jpg|jpeg|gif|svg|webp|ico|css|js|ts|woff|woff2|ttf|eot|pdf|zip|tar|gz|mp4|mp3|wav)$/i;

export function shouldFollowUrl(
  url: string,
  seedOrigin: string,
  urlPattern?: string
): boolean {
  if (!url || url.startsWith("#") || url.startsWith("mailto:") || url.startsWith("tel:") || url.startsWith("javascript:")) {
    return false;
  }
  let parsed: URL;
  try {
    parsed = new URL(url);
  } catch {
    return false;
  }
  if (parsed.origin !== new URL(seedOrigin).origin) return false;
  if (ASSET_EXTENSIONS.test(parsed.pathname)) return false;
  if (urlPattern && !new RegExp(urlPattern).test(parsed.pathname)) return false;
  return true;
}

export function normalizeLinks(hrefs: string[], baseUrl: string): string[] {
  const seen = new Set<string>();
  const results: string[] = [];
  for (const href of hrefs) {
    try {
      const abs = new URL(href, baseUrl);
      abs.hash = "";
      // Strip trailing slash (but keep root)
      const normalized = abs.href.replace(/\/$/, "") || abs.href;
      if (!seen.has(normalized)) {
        seen.add(normalized);
        results.push(normalized);
      }
    } catch {
      // skip unparseable
    }
  }
  return results;
}

export function filterLinks(
  urls: string[],
  seedOrigin: string,
  urlPattern?: string
): string[] {
  return urls.filter((u) => shouldFollowUrl(u, seedOrigin, urlPattern));
}

export async function* crawl(options: CrawlOptions): AsyncGenerator<CrawledPage> {
  const {
    seedUrl,
    maxDepth,
    maxPages,
    concurrency,
    delayMs,
    urlPattern,
    waitUntil,
    selector: _selector,
  } = options;

  const browser = await chromium.launch({ headless: true });
  const visited = new Set<string>();
  const queue: { url: string; depth: number }[] = [{ url: seedUrl, depth: 0 }];
  let pageCount = 0;

  const processUrl = async (url: string, depth: number): Promise<CrawledPage | null> => {
    const page = await browser.newPage();
    try {
      await page.goto(url, { waitUntil, timeout: 15000 });
      const html = await page.content();
      const title = await page.title();
      const hrefs = await page.$$eval("a[href]", (els) =>
        els.map((el) => (el as HTMLAnchorElement).href)
      );
      await page.close();

      const normalized = normalizeLinks(hrefs, url);
      const links = filterLinks(normalized, seedUrl, urlPattern);

      return { url, html, title, links, depth };
    } catch (err) {
      await page.close().catch(() => {});
      console.error(`[crawler] Error fetching ${url}:`, err instanceof Error ? err.message : err);
      return null;
    }
  };

  try {
    while (queue.length > 0 && pageCount < maxPages) {
      const currentLevel = queue.splice(0, queue.length);
      const toProcess = currentLevel.filter(({ url }) => {
        if (visited.has(url)) return false;
        visited.add(url);
        return true;
      });

      // Process this level with concurrency chunks
      const chunks: typeof toProcess[] = [];
      for (let i = 0; i < toProcess.length; i += concurrency) {
        chunks.push(toProcess.slice(i, i + concurrency));
      }

      for (const chunk of chunks) {
        if (pageCount >= maxPages) break;
        const settled = await Promise.allSettled(
          chunk.map(({ url, depth }) => processUrl(url, depth))
        );
        for (const result of settled) {
          if (pageCount >= maxPages) break;
          if (result.status === "fulfilled" && result.value) {
            const crawledPage = result.value;
            pageCount++;
            yield crawledPage;

            // Queue next depth
            if (crawledPage.depth < maxDepth) {
              for (const link of crawledPage.links) {
                if (!visited.has(link) && pageCount < maxPages) {
                  queue.push({ url: link, depth: crawledPage.depth + 1 });
                }
              }
            }
          }
        }
        if (delayMs > 0 && queue.length > 0) {
          await new Promise((r) => setTimeout(r, delayMs));
        }
      }
    }
  } finally {
    await browser.close();
  }
}
