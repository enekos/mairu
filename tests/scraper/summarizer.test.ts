import { describe, it, expect, vi, beforeEach } from "vitest";

vi.mock("@google/genai", () => ({
  GoogleGenAI: vi.fn().mockImplementation(() => ({
    models: {
      generateContent: vi.fn().mockResolvedValue({
        text: JSON.stringify({
          abstract: "A concise summary of the page.",
          overview: "This page covers authentication methods including OAuth2 and JWT.",
          ai_intent: "how_to",
          ai_topics: ["authentication", "oauth2", "jwt"],
          ai_quality_score: 8,
        }),
      }),
    },
  })),
}));

describe("summarizePage", () => {
  beforeEach(() => {
    vi.resetModules();
  });

  it("returns structured summary from LLM", async () => {
    const { summarizePage } = await import("../../src/scraper/summarizer");
    const result = await summarizePage(
      "Authentication Guide",
      "# Auth\nThis guide explains OAuth2 and JWT authentication.",
      "https://docs.example.com/auth"
    );
    expect(result.abstract).toBe("A concise summary of the page.");
    expect(result.ai_intent).toBe("how_to");
    expect(result.ai_topics).toContain("authentication");
    expect(result.ai_quality_score).toBe(8);
  });

  it("returns minimal summary for very short content", async () => {
    const { summarizePage } = await import("../../src/scraper/summarizer");
    const result = await summarizePage(
      "Short Page",
      "Just a few words.",
      "https://example.com/short"
    );
    // For short content (< 50 words), still returns a PageSummary
    expect(result.abstract).toBeTruthy();
    expect(typeof result.ai_quality_score).toBe("number");
  });
});
