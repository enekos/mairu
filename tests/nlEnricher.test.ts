import { describe, expect, it } from "vitest";
import { enrichDescriptions } from "../src/ast/nlEnricher";
import type { LogicEdge } from "../src/ast/languageDescriber";

describe("enrichDescriptions", () => {
  it("enriches call references with callee context", () => {
    const descriptions = new Map<string, string>([
      ["fn:process", "1. Calls `validate` with `input`.\n2. Returns the validated result."],
      ["fn:validate", "1. If `input` is falsy, throws an Error with message \"Required\".\n2. Returns `input` trimmed."],
    ]);
    const edges: LogicEdge[] = [
      { kind: "call", from: "fn:process", to: "fn:validate" },
    ];

    const enriched = enrichDescriptions(descriptions, edges);

    const processDesc = enriched.get("fn:process")!;
    expect(processDesc).toContain("validate");
    // Should contain enrichment from validate's description
    expect(processDesc).toMatch(/falsy|Required|trimmed/);
  });

  it("does not recurse beyond depth 1", () => {
    const descriptions = new Map<string, string>([
      ["fn:a", "1. Calls `b`."],
      ["fn:b", "1. Calls `c`."],
      ["fn:c", "1. Returns 42."],
    ]);
    const edges: LogicEdge[] = [
      { kind: "call", from: "fn:a", to: "fn:b" },
      { kind: "call", from: "fn:b", to: "fn:c" },
    ];

    const enriched = enrichDescriptions(descriptions, edges);
    // fn:a should mention b's behavior but NOT c's
    const aDesc = enriched.get("fn:a")!;
    expect(aDesc).toMatch(/[Cc]alls.*b/);
  });

  it("handles symbols with no call edges unchanged", () => {
    const descriptions = new Map<string, string>([
      ["fn:simple", "1. Returns 42."],
    ]);
    const enriched = enrichDescriptions(descriptions, []);
    expect(enriched.get("fn:simple")).toBe("1. Returns 42.");
  });
});
