// src/scraper/scrapeManager.ts
import * as path from "path";
import { crawl } from "./crawler";
import { extractContent } from "./extractor";
import { summarizePage } from "./summarizer";
import { ScrapeCache } from "./cache";
import { urlToUri, urlToParentUri } from "./uriMapper";
import type { ScrapeOptions, ScrapeResult } from "./types";
import { createContextManager } from "../storage/client";

const CACHE_FILE = path.join(process.cwd(), ".contextfs-scrape-cache.json");

export async function scrapeAndIngest(options: ScrapeOptions): Promise<ScrapeResult> {
  const {
    project,
    splitSections,
    dryRun,
    useRouter,
    ...crawlOptions
  } = options;

  const cache = new ScrapeCache(CACHE_FILE);
  const cm = createContextManager();

  const result: ScrapeResult = {
    pagesTotal: 0,
    pagesStored: 0,
    pagesUpdated: 0,
    pagesSkipped: 0,
    sectionsStored: 0,
    errors: [],
  };

  for await (const page of crawl(crawlOptions)) {
    result.pagesTotal++;
    const label = new URL(page.url).pathname || "/";

    try {
      const extracted = extractContent(page.html, { selector: crawlOptions.selector });

      if (extracted.wordCount === 0) {
        console.log(`  [${result.pagesTotal}] ${label} .............. skipped (no content)`);
        result.pagesSkipped++;
        continue;
      }

      // Check cache
      if (cache.isUnchanged(page.url, extracted.markdown)) {
        console.log(`  [${result.pagesTotal}] ${label} .............. skipped (unchanged)`);
        result.pagesSkipped++;
        continue;
      }

      const summary = await summarizePage(
        extracted.title || page.title,
        extracted.markdown,
        page.url
      );

      const uri = urlToUri(page.url);
      const parentUri = urlToParentUri(page.url);

      const metadata = {
        source_type: "web_scrape",
        source_url: page.url,
        scraped_at: new Date().toISOString(),
        depth: page.depth,
        word_count: extracted.wordCount,
        ...(crawlOptions.selector ? { selector_used: crawlOptions.selector } : {}),
      };

      if (dryRun) {
        console.log(`  [${result.pagesTotal}] ${label}`);
        console.log(`    → ${uri}`);
        console.log(`    abstract: ${summary.abstract.slice(0, 80)}...`);
        // In dry-run mode, count as "would store" — caller should check dryRun flag
        result.pagesStored++;
        continue;
      }

      const stored = await cm.addContextNode(
        uri,
        extracted.title || page.title,
        summary.abstract,
        summary.overview,
        extracted.markdown,
        parentUri,
        project,
        metadata,
        useRouter,
        {
          ai_intent: summary.ai_intent,
          ai_topics: summary.ai_topics,
          ai_quality_score: summary.ai_quality_score,
        }
      );

      // Update cache
      cache.set(page.url, {
        contentHash: cache.contentHash(extracted.markdown),
        scrapedAt: new Date().toISOString(),
        uri,
      });

      if ("budgetExceeded" in stored && stored.budgetExceeded) {
        console.warn(`  [${result.pagesTotal}] ${label} .............. BUDGET EXCEEDED — stopping`);
        result.errors.push({ url: page.url, error: "budget exceeded" });
        break;
      } else if ("skipped" in stored && stored.skipped) {
        console.log(`  [${result.pagesTotal}] ${label} .............. skipped (duplicate)`);
        result.pagesSkipped++;
      } else if ("updated" in stored && stored.updated) {
        console.log(`  [${result.pagesTotal}] ${label} .............. updated`);
        result.pagesUpdated++;
      } else {
        console.log(`  [${result.pagesTotal}] ${label} .............. stored (new)`);
        result.pagesStored++;
      }

      // Store sections as child nodes if requested
      if (splitSections && extracted.sections.length > 0) {
        for (const section of extracted.sections) {
          const sectionUri = urlToUri(page.url, section.heading);
          const sectionStored = await cm.addContextNode(
            sectionUri,
            section.heading,
            `${section.heading}: ${section.content.slice(0, 200)}`,
            undefined,
            section.content,
            uri,
            project,
            { ...metadata, section_heading: section.heading },
            useRouter
          );
          if (
            !("skipped" in sectionStored && sectionStored.skipped) &&
            !("budgetExceeded" in sectionStored && sectionStored.budgetExceeded)
          ) {
            result.sectionsStored++;
          }
        }
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      console.error(`  [${result.pagesTotal}] ${label} .............. ERROR: ${message}`);
      result.errors.push({ url: page.url, error: message });
    }
  }

  if (!dryRun) {
    try {
      cache.save();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      console.error(`[scraper] Failed to save cache: ${message}`);
    }
  }
  return result;
}
