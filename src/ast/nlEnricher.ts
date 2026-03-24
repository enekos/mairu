import type { LogicEdge } from "./languageDescriber";

/**
 * Extracts the short name from a symbol ID.
 * e.g., "fn:validate" → "validate", "mtd:Service.run" → "run"
 */
function extractShortName(symbolId: string): string {
  // Strip prefix like "fn:", "mtd:", "cls:", etc.
  const withoutPrefix = symbolId.includes(":") ? symbolId.split(":").slice(1).join(":") : symbolId;
  // For dotted names like "Service.run", take the last segment
  const parts = withoutPrefix.split(".");
  return parts[parts.length - 1]!;
}

/**
 * Extracts the first sentence from a description string.
 */
function firstSentence(description: string): string {
  // Grab the first line and trim it
  const firstLine = description.split("\n")[0]!.trim();
  // Strip leading list markers like "1. "
  return firstLine.replace(/^\d+\.\s*/, "").trim();
}

/**
 * Enriches NL descriptions by appending callee context (depth 1 only).
 *
 * For each caller description, finds call edges to callees that also have
 * descriptions, extracts the first sentence of each callee's description,
 * and appends it as a parenthetical after the first mention of the callee
 * name in the caller's description.
 */
export function enrichDescriptions(
  descriptions: Map<string, string>,
  edges: LogicEdge[]
): Map<string, string> {
  // Build caller → Set<calleeSymbolId> from call edges only
  const callMap = new Map<string, Set<string>>();
  for (const edge of edges) {
    if (edge.kind !== "call") continue;
    let callees = callMap.get(edge.from);
    if (!callees) {
      callees = new Set();
      callMap.set(edge.from, callees);
    }
    callees.add(edge.to);
  }

  const result = new Map<string, string>();

  for (const [symbolId, description] of descriptions) {
    const callees = callMap.get(symbolId);
    if (!callees || callees.size === 0) {
      // No call edges — leave description unchanged
      result.set(symbolId, description);
      continue;
    }

    let enriched = description;

    for (const calleeId of callees) {
      const calleeDesc = descriptions.get(calleeId);
      if (!calleeDesc) continue;

      const calleeName = extractShortName(calleeId);
      const summary = firstSentence(calleeDesc);

      // Only append if we haven't already added a parenthetical for this callee
      if (!summary) continue;

      // Build parenthetical: "(where <callee> <summary>)"
      const parenthetical = ` (where \`${calleeName}\` ${summary.charAt(0).toLowerCase()}${summary.slice(1)})`;

      // Find the first mention of the callee name (as a word boundary match)
      // and append the parenthetical right after it
      const nameRegex = new RegExp(`(\`${escapeRegExp(calleeName)}\`|\\b${escapeRegExp(calleeName)}\\b)`, "");
      if (nameRegex.test(enriched)) {
        enriched = enriched.replace(nameRegex, `$1${parenthetical}`);
      } else {
        // Callee name not mentioned — append a note at the end
        enriched = enriched.trimEnd() + `\n   Note: \`${calleeName}\` — ${summary}`;
      }
    }

    result.set(symbolId, enriched);
  }

  return result;
}

function escapeRegExp(str: string): string {
  return str.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
