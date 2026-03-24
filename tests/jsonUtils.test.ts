import { describe, it, expect } from "vitest";
import { extractJsonObject, extractJsonArray } from "../src/core/jsonUtils";

describe("jsonUtils", () => {
  describe("extractJsonObject", () => {
    it("parses plain JSON object", () => {
      expect(extractJsonObject('{"action":"create"}')).toEqual({ action: "create" });
    });

    it("strips markdown code fences (```json ... ```)", () => {
      const text = "```json\n{\"action\":\"create\"}\n```";
      expect(extractJsonObject(text)).toEqual({ action: "create" });
    });

    it("strips bare code fences (``` ... ```)", () => {
      const text = "```\n{\"action\":\"skip\",\"reason\":\"already captured\"}\n```";
      expect(extractJsonObject(text)).toEqual({ action: "skip", reason: "already captured" });
    });

    it("extracts JSON embedded in prose", () => {
      const text = "Sure! Here is my answer: {\"action\":\"update\",\"targetId\":\"abc\",\"mergedContent\":\"merged\"} done.";
      expect(extractJsonObject(text)).toEqual({ action: "update", targetId: "abc", mergedContent: "merged" });
    });

    it("returns null for text with no JSON object", () => {
      expect(extractJsonObject("No JSON here at all.")).toBeNull();
    });

    it("returns null for malformed JSON", () => {
      expect(extractJsonObject("{action: create}")).toBeNull(); // unquoted keys → invalid JSON
    });

    it("handles multi-line JSON", () => {
      const text = `
\`\`\`json
{
  "action": "skip",
  "reason": "identical information already stored"
}
\`\`\`
      `;
      expect(extractJsonObject(text)).toEqual({ action: "skip", reason: "identical information already stored" });
    });
  });

  describe("extractJsonArray", () => {
    it("parses plain JSON array", () => {
      expect(extractJsonArray('["a", "b"]')).toEqual(["a", "b"]);
    });

    it("strips markdown code fences", () => {
      const text = "```json\n[\"a\", \"b\"]\n```";
      expect(extractJsonArray(text)).toEqual(["a", "b"]);
    });

    it("extracts JSON array embedded in prose", () => {
      const text = "Here are the items: [\"item1\", \"item2\"] - thanks!";
      expect(extractJsonArray(text)).toEqual(["item1", "item2"]);
    });

    it("returns null for text with no JSON array", () => {
      expect(extractJsonArray("No array here at all.")).toBeNull();
    });

    it("returns null for malformed JSON array", () => {
      expect(extractJsonArray("['a', 'b']")).toBeNull(); // single quotes → invalid JSON
    });

    it("handles multi-line JSON array", () => {
      const text = `
\`\`\`
[
  "one",
  "two"
]
\`\`\`
      `;
      expect(extractJsonArray(text)).toEqual(["one", "two"]);
    });
  });
});
