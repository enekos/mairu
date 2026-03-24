/**
 * LLM-powered memory/context router.
 * Before writing, searches for similar existing entries and asks the LLM
 * whether to CREATE, UPDATE (merge), or SKIP.
 *
 * Inspired by SimpleMem's online synthesis and A-Mem's agent-based management.
 */
import { GoogleGenAI } from "@google/genai";
import { extractJsonObject } from "../core/jsonUtils";
import { config } from "../core/config";

const LLM_MODEL = config.llmModel;

const ai = config.geminiApiKey
  ? new GoogleGenAI({ apiKey: config.geminiApiKey })
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

const MAX_RETRIES = 3;
const RETRY_DELAY_MS = 1000;

async function generateWithRetry(model: string, contents: string, attempt = 1): Promise<any> {
  if (!ai) throw new Error("GoogleGenAI not initialized");
  try {
    return await ai.models.generateContent({ model, contents });
  } catch (error: unknown) {
    if (attempt < MAX_RETRIES && ((error as {status?: number})?.status === 429 || ((error as {status?: number})?.status ?? 0) >= 500 || (error as {message?: string})?.message?.includes("fetch failed"))) {
      const delay = RETRY_DELAY_MS * Math.pow(2, attempt - 1);
      console.warn(`[llmRouter] API error (${error instanceof Error ? error instanceof Error ? error.message : String(error) : String(error)}), retrying in ${delay}ms (attempt ${attempt + 1}/${MAX_RETRIES})`);
      await new Promise((resolve) => setTimeout(resolve, delay));
      return generateWithRetry(model, contents, attempt + 1);
    }
    throw error;
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
    const response = await generateWithRetry(LLM_MODEL, prompt);
    const text = response.text?.trim() || "";
    const decision = extractJsonObject(text);
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
    const response = await generateWithRetry(LLM_MODEL, prompt);
    const text = response.text?.trim() || "";
    const decision = extractJsonObject(text);
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
