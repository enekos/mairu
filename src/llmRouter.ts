/**
 * LLM-powered memory/context router.
 * Before writing, searches for similar existing entries and asks the LLM
 * whether to CREATE, UPDATE (merge), or SKIP.
 *
 * Inspired by SimpleMem's online synthesis and A-Mem's agent-based management.
 */
import { GoogleGenAI } from "@google/genai";
import * as dotenv from "dotenv";
import * as path from "path";

dotenv.config({ path: path.resolve(__dirname, "..", ".env") });

const LLM_MODEL = process.env.LLM_MODEL || "gemini-2.0-flash-lite";

const ai = process.env.GEMINI_API_KEY
  ? new GoogleGenAI({ apiKey: process.env.GEMINI_API_KEY })
  : null;

export type RouterAction =
  | { action: "create" }
  | { action: "update"; targetId: string; mergedContent: string }
  | { action: "skip"; reason: string };

export interface RouterCandidate {
  id: string;
  content: string;
  score: number;
}

const SIMILARITY_GATE = 0.75; // only invoke LLM if best candidate score >= this

/** Extract JSON object from LLM text that may contain markdown fences */
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

/**
 * Decide what to do with a new memory entry.
 */
export async function decideMemoryAction(
  newContent: string,
  candidates: RouterCandidate[]
): Promise<RouterAction> {
  if (!ai) return { action: "create" };

  const topCandidates = candidates
    .filter((c) => c.score >= SIMILARITY_GATE)
    .slice(0, 4);

  if (topCandidates.length === 0) return { action: "create" };

  const candidateList = topCandidates
    .map((c) => `ID: ${c.id}\nSIMILARITY: ${c.score.toFixed(3)}\nCONTENT: ${c.content}`)
    .join("\n---\n");

  const prompt = `You are managing an AI agent's memory database. Decide what to do with new incoming information.

NEW INFORMATION:
${newContent}

EXISTING SIMILAR MEMORIES:
${candidateList}

Rules:
- "create": the new information is genuinely new, adds detail not captured by any existing memory
- "update": an existing memory should be enriched/corrected. Provide merged content that combines both into one complete sentence/fact. Use the ID of the single best matching memory as targetId.
- "skip": the new information is already fully captured by an existing memory

Respond with ONLY a JSON object (no markdown fences):
- {"action":"create"}
- {"action":"update","targetId":"<exact id>","mergedContent":"<merged text>"}
- {"action":"skip","reason":"<brief reason>"}`;

  try {
    const response = await ai.models.generateContent({
      model: LLM_MODEL,
      contents: prompt,
    });
    const text = response.text?.trim() || "";
    const decision = extractJson(text);
    if (!decision) return { action: "create" };

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
    if (decision.action === "create") {
      return { action: "create" };
    }
    return { action: "create" };
  } catch (e) {
    console.warn("[llmRouter] decideMemoryAction failed, defaulting to create:", e);
    return { action: "create" };
  }
}

/**
 * Decide what to do with a new context node.
 */
export async function decideContextAction(
  uri: string,
  name: string,
  abstract: string,
  candidates: RouterCandidate[]
): Promise<RouterAction> {
  if (!ai) return { action: "create" };

  const topCandidates = candidates
    .filter((c) => c.score >= SIMILARITY_GATE)
    .slice(0, 4);

  if (topCandidates.length === 0) return { action: "create" };

  const candidateList = topCandidates
    .map((c) => `ID: ${c.id}\nSIMILARITY: ${c.score.toFixed(3)}\nABSTRACT: ${c.content}`)
    .join("\n---\n");

  const prompt = `You are managing a hierarchical context database for a software project. Decide what to do with a new context node.

NEW NODE:
URI: ${uri}
NAME: ${name}
ABSTRACT: ${abstract}

EXISTING SIMILAR NODES:
${candidateList}

Rules:
- "create": the new node covers genuinely new territory
- "update": an existing node should have its abstract enriched. Provide merged abstract as mergedContent. Use the existing node's URI as targetId.
- "skip": the new node is already fully covered by an existing node

Respond with ONLY a JSON object:
- {"action":"create"}
- {"action":"update","targetId":"<exact uri>","mergedContent":"<merged abstract>"}
- {"action":"skip","reason":"<brief reason>"}`;

  try {
    const response = await ai.models.generateContent({
      model: LLM_MODEL,
      contents: prompt,
    });
    const text = response.text?.trim() || "";
    const decision = extractJson(text);
    if (!decision) return { action: "create" };

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
  } catch (e) {
    console.warn("[llmRouter] decideContextAction failed, defaulting to create:", e);
    return { action: "create" };
  }
}
