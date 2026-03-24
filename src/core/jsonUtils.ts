/** Shared JSON extraction utilities for LLM output parsing */

function stripFences(text: string): string {
  return text.replace(/```(?:json)?\s*/g, "").replace(/```/g, "").trim();
}

export function extractJsonObject(text: string): Record<string, any> | null {
  const match = stripFences(text).match(/\{[\s\S]*\}/);
  if (!match) return null;
  try {
    return JSON.parse(match[0]);
  } catch {
    return null;
  }
}

export function extractJsonArray(text: string): unknown[] | null {
  const match = stripFences(text).match(/\[[\s\S]*\]/);
  if (!match) return null;
  try {
    return JSON.parse(match[0]);
  } catch {
    return null;
  }
}
