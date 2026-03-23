import { describe, expect, it } from "vitest";
import {
  buildSearchArgs,
  buildVibeMutationPreviewArgs,
  parseCliOutput,
  type QueryKind,
} from "../src/core";
import { resolveDefaultProject } from "../src/project";

describe("buildSearchArgs", () => {
  it("builds memory search arguments with project and topK", () => {
    const args = buildSearchArgs({
      kind: "memory_search",
      query: "token refresh policy",
      project: "my-workspace",
      topK: 7,
    });

    expect(args).toEqual([
      "memory",
      "search",
      "token refresh policy",
      "-P",
      "my-workspace",
      "-k",
      "7",
    ]);
  });

  it.each([
    ["node_search", "node", "search"],
    ["skill_search", "skill", "search"],
  ] as Array<[QueryKind, string, string]>)("maps %s to the expected command", (kind, first, second) => {
    const args = buildSearchArgs({
      kind,
      query: "auth",
      project: "proj",
      topK: 5,
    });
    expect(args[0]).toBe(first);
    expect(args[1]).toBe(second);
  });
});

describe("parseCliOutput", () => {
  it("returns JSON when stdout is JSON", () => {
    const parsed = parseCliOutput('[{"id":"1","content":"hello"}]');
    expect(parsed.kind).toBe("json");
    if (parsed.kind === "json") {
      expect(Array.isArray(parsed.value)).toBe(true);
    }
  });

  it("falls back to text when stdout is not JSON", () => {
    const parsed = parseCliOutput("Planning search queries...\nDone.");
    expect(parsed).toEqual({
      kind: "text",
      value: "Planning search queries...\nDone.",
    });
  });
});

describe("buildVibeMutationPreviewArgs", () => {
  it("builds non-destructive preview args", () => {
    const args = buildVibeMutationPreviewArgs({
      prompt: "remember we migrated to OAuth",
      project: "proj",
      topK: 9,
    });
    expect(args).toEqual([
      "vibe-mutation",
      "remember we migrated to OAuth",
      "-P",
      "proj",
      "-k",
      "9",
    ]);
  });
});

describe("resolveDefaultProject", () => {
  it("uses workspace leaf name when available", () => {
    const project = resolveDefaultProject("/Users/me/code/contextfs");
    expect(project).toBe("contextfs");
  });

  it("falls back to 'default' when no workspace path is provided", () => {
    const project = resolveDefaultProject(undefined);
    expect(project).toBe("default");
  });
});
