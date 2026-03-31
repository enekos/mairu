import { GoogleGenAI } from "@google/genai";
import { config } from "../core/config";
import type { PageSummary } from "./types";

const MAX_INPUT_TOKENS = 8000; // ~6000 words
const SHORT_PAGE_THRESHOLD = 5; // words

const ai = config.geminiApiKey
  ? new GoogleGenAI({ apiKey: config.geminiApiKey })
  : null;

function truncateMarkdown(markdown: string): string {
  // Rough estimate: 1 token ≈ 4 chars
  const maxChars = MAX_INPUT_TOKENS * 4;
  if (markdown.length <= maxChars) return markdown;
  return markdown.slice(0, maxChars) + "\n\n[content truncated]";
}

function buildPrompt(title: string, markdown: string, url: string): string {
  return `You are a technical documentation indexer. Analyze this web page and return a JSON object.

URL: ${url}
Title: ${title}

Content:
${truncateMarkdown(markdown)}

Return ONLY valid JSON (no markdown, no explanation) with these fields:
{
  "abstract": "1-2 sentence summary of what this page covers",
  "overview": "Key topics, structure, and important concepts on this page (up to 400 words)",
  "ai_intent": "one of: fact, decision, how_to, todo, warning — whichever best describes this page",
  "ai_topics": ["array", "of", "topic", "tags"],
  "ai_quality_score": <integer 1-10 rating content quality and relevance>
}`;
}

function fallbackSummary(title: string, markdown: string, url: string): PageSummary {
  const firstLine = markdown.split("\n").find((l) => l.trim().length > 0) ?? title;
  return {
    abstract: `${title} (${url}): ${firstLine.slice(0, 200)}`,
    overview: markdown.slice(0, 500),
    ai_intent: null,
    ai_topics: [],
    ai_quality_score: 5,
  };
}

export async function summarizePage(
  title: string,
  markdown: string,
  url: string
): Promise<PageSummary> {
  const wordCount = markdown.split(/\s+/).filter(Boolean).length;

  if (wordCount < SHORT_PAGE_THRESHOLD) {
    return fallbackSummary(title, markdown, url);
  }

  if (!ai) {
    return fallbackSummary(title, markdown, url);
  }
  const prompt = buildPrompt(title, markdown, url);

  try {
    const response = await ai.models.generateContent({
      model: config.llmModel,
      contents: prompt,
    });

    const text = response.text?.trim() ?? "";
    // Strip markdown code fences if present
    const cleaned = text.replace(/^```(?:json)?\n?/, "").replace(/\n?```$/, "");
    const parsed = JSON.parse(cleaned);

    return {
      abstract: String(parsed.abstract ?? ""),
      overview: String(parsed.overview ?? ""),
      ai_intent: parsed.ai_intent ?? null,
      ai_topics: Array.isArray(parsed.ai_topics) ? parsed.ai_topics : [],
      ai_quality_score: Number(parsed.ai_quality_score ?? 5),
    };
  } catch {
    return fallbackSummary(title, markdown, url);
  }
}
