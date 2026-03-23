export type QueryKind =
  | "memory_search"
  | "node_search"
  | "skill_search"
  | "vibe_query"
  | "vibe_mutation_plan";

export interface SearchArgsInput {
  kind: Extract<QueryKind, "memory_search" | "node_search" | "skill_search">;
  query: string;
  project: string;
  topK: number;
}

export interface VibeArgsInput {
  prompt: string;
  project: string;
  topK: number;
}

type ParsedOutput =
  | { kind: "json"; value: unknown }
  | { kind: "text"; value: string };

const SEARCH_PREFIX: Record<SearchArgsInput["kind"], [string, string]> = {
  memory_search: ["memory", "search"],
  node_search: ["node", "search"],
  skill_search: ["skill", "search"],
};

export function buildSearchArgs(input: SearchArgsInput): string[] {
  const prefix = SEARCH_PREFIX[input.kind];
  return [...prefix, input.query, "-P", input.project, "-k", String(input.topK)];
}

export function buildVibeQueryArgs(input: VibeArgsInput): string[] {
  return ["vibe-query", input.prompt, "-P", input.project, "-k", String(input.topK)];
}

export function buildVibeMutationPreviewArgs(input: VibeArgsInput): string[] {
  return ["vibe-mutation", input.prompt, "-P", input.project, "-k", String(input.topK)];
}

export function parseCliOutput(stdout: string): ParsedOutput {
  const trimmed = stdout.trim();
  if (!trimmed) {
    return { kind: "text", value: "" };
  }
  try {
    return { kind: "json", value: JSON.parse(trimmed) };
  } catch {
    return { kind: "text", value: stdout.trimEnd() };
  }
}
