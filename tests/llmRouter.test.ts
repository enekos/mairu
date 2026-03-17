/**
 * Tests for llmRouter JSON parsing and gating logic.
 * The LLM call itself is not tested here (requires API key + network).
 * We test the pure logic: JSON extraction, SIMILARITY_GATE filtering,
 * and graceful fallback to "create" when no candidates qualify.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// ─────────────────────────────────────────────────────────────────────────────
// Inline the extractJson helper (mirrors the one in llmRouter.ts) so we can
// unit-test it without importing the whole module (which boots dotenv / AI).
// ─────────────────────────────────────────────────────────────────────────────
function extractJson(text: string): Record<string, any> | null {
  const stripped = text.replace(/```(?:json)?\s*/g, "").replace(/```/g, "").trim();
  const match = stripped.match(/\{[\s\S]*\}/);
  if (!match) return null;
  try {
    return JSON.parse(match[0]);
  } catch {
    return null;
  }
}

describe("extractJson", () => {
  it("parses plain JSON", () => {
    expect(extractJson('{"action":"create"}')).toEqual({ action: "create" });
  });

  it("strips markdown code fences (```json ... ```)", () => {
    const text = "```json\n{\"action\":\"create\"}\n```";
    expect(extractJson(text)).toEqual({ action: "create" });
  });

  it("strips bare code fences (``` ... ```)", () => {
    const text = "```\n{\"action\":\"skip\",\"reason\":\"already captured\"}\n```";
    expect(extractJson(text)).toEqual({ action: "skip", reason: "already captured" });
  });

  it("extracts JSON embedded in prose", () => {
    const text = "Sure! Here is my answer: {\"action\":\"update\",\"targetId\":\"abc\",\"mergedContent\":\"merged\"} done.";
    expect(extractJson(text)).toEqual({ action: "update", targetId: "abc", mergedContent: "merged" });
  });

  it("returns null for text with no JSON object", () => {
    expect(extractJson("No JSON here at all.")).toBeNull();
  });

  it("returns null for malformed JSON", () => {
    expect(extractJson("{action: create}")).toBeNull(); // unquoted keys → invalid JSON
  });

  it("handles nested JSON objects", () => {
    const text = '{"action":"update","targetId":"id1","mergedContent":"text with {braces} inside"}';
    const result = extractJson(text);
    expect(result?.action).toBe("update");
    expect(result?.targetId).toBe("id1");
  });

  it("handles multi-line JSON from LLM", () => {
    const text = `
\`\`\`json
{
  "action": "skip",
  "reason": "identical information already stored"
}
\`\`\`
    `;
    const result = extractJson(text);
    expect(result).toEqual({ action: "skip", reason: "identical information already stored" });
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// Gate logic — the router only calls the LLM when at least one candidate
// has a score >= SIMILARITY_GATE (0.75). We test the filtering behavior.
// ─────────────────────────────────────────────────────────────────────────────
describe("router candidate gating", () => {
  const SIMILARITY_GATE = 0.75;

  function topCandidatesAboveGate(
    candidates: Array<{ id: string; content: string; score: number }>
  ) {
    return candidates.filter((c) => c.score >= SIMILARITY_GATE).slice(0, 4);
  }

  it("returns empty when all candidates are below gate", () => {
    const cands = [
      { id: "a", content: "something", score: 0.5 },
      { id: "b", content: "else", score: 0.6 },
    ];
    expect(topCandidatesAboveGate(cands)).toHaveLength(0);
  });

  it("keeps only candidates above gate", () => {
    const cands = [
      { id: "a", content: "high", score: 0.9 },
      { id: "b", content: "low", score: 0.4 },
      { id: "c", content: "just over", score: 0.75 },
    ];
    const result = topCandidatesAboveGate(cands);
    expect(result).toHaveLength(2);
    expect(result.map((r) => r.id)).toContain("a");
    expect(result.map((r) => r.id)).toContain("c");
  });

  it("caps at 4 candidates regardless of how many qualify", () => {
    const cands = Array.from({ length: 10 }, (_, i) => ({
      id: `id${i}`,
      content: "content",
      score: 0.8 + i * 0.01,
    }));
    expect(topCandidatesAboveGate(cands)).toHaveLength(4);
  });

  it("accepts exactly 0.75 as meeting the gate", () => {
    const cands = [{ id: "a", content: "text", score: 0.75 }];
    expect(topCandidatesAboveGate(cands)).toHaveLength(1);
  });
});

// ─────────────────────────────────────────────────────────────────────────────
// Decision shape validation — mirrors the guards in decideMemoryAction
// ─────────────────────────────────────────────────────────────────────────────
describe("router decision validation", () => {
  type RouterAction =
    | { action: "create" }
    | { action: "update"; targetId: string; mergedContent: string }
    | { action: "skip"; reason: string };

  function validateDecision(decision: Record<string, any>): RouterAction {
    if (
      decision.action === "update" &&
      typeof decision.targetId === "string" &&
      typeof decision.mergedContent === "string"
    ) {
      return { action: "update", targetId: decision.targetId, mergedContent: decision.mergedContent };
    }
    if (decision.action === "skip" && typeof decision.reason === "string") {
      return { action: "skip", reason: decision.reason };
    }
    return { action: "create" };
  }

  it("accepts valid create", () => {
    expect(validateDecision({ action: "create" })).toEqual({ action: "create" });
  });

  it("accepts valid update", () => {
    const d = { action: "update", targetId: "mem_abc", mergedContent: "merged text" };
    expect(validateDecision(d)).toEqual({ action: "update", targetId: "mem_abc", mergedContent: "merged text" });
  });

  it("accepts valid skip", () => {
    const d = { action: "skip", reason: "already captured" };
    expect(validateDecision(d)).toEqual({ action: "skip", reason: "already captured" });
  });

  it("falls back to create when update is missing targetId", () => {
    expect(validateDecision({ action: "update", mergedContent: "text" })).toEqual({ action: "create" });
  });

  it("falls back to create when update is missing mergedContent", () => {
    expect(validateDecision({ action: "update", targetId: "id1" })).toEqual({ action: "create" });
  });

  it("falls back to create when skip is missing reason", () => {
    expect(validateDecision({ action: "skip" })).toEqual({ action: "create" });
  });

  it("falls back to create for unknown action", () => {
    expect(validateDecision({ action: "merge", targetId: "x" })).toEqual({ action: "create" });
  });

  it("falls back to create for completely empty object", () => {
    expect(validateDecision({})).toEqual({ action: "create" });
  });
});
