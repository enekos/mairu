/**
 * LLM-powered text ingestor.
 * Parses free text or markdown into structured context nodes
 * suitable for storage in the contextfs hierarchy.
 */
import { GoogleGenAI } from "@google/genai";
import * as dotenv from "dotenv";
import * as path from "path";
import { extractJsonArray } from "./jsonUtils";
import { config } from "./config";

dotenv.config({ path: path.resolve(__dirname, "..", ".env") });

const LLM_MODEL = config.llmModel;

const ai = config.geminiApiKey
  ? new GoogleGenAI({ apiKey: config.geminiApiKey })
  : null;

export interface ProposedContextNode {
  uri: string;
  name: string;
  abstract: string;
  overview?: string;
  content?: string;
  parent_uri: string | null;
}


const MAX_RETRIES = 3;
const RETRY_DELAY_MS = 1000;

async function generateWithRetry(model: string, contents: string, attempt = 1): Promise<any> {
  if (!ai) throw new Error("GoogleGenAI not initialized");
  try {
    return await ai.models.generateContent({ model, contents });
  } catch (error: unknown) {
    if (attempt < MAX_RETRIES && ((error as {status?: number})?.status === 429 || ((error as {status?: number})?.status ?? 0) >= 500 || (error as {message?: string})?.message?.includes("fetch failed"))) {
      const delay = RETRY_DELAY_MS * Math.pow(2, attempt - 1);
      console.warn(`[ingestor] API error (${error instanceof Error ? error instanceof Error ? error.message : String(error) : String(error)}), retrying in ${delay}ms (attempt ${attempt + 1}/${MAX_RETRIES})`);
      await new Promise((resolve) => setTimeout(resolve, delay));
      return generateWithRetry(model, contents, attempt + 1);
    }
    throw error;
  }
}

const MAX_INPUT_CHARS = 100_000;

/**
 * Parse free text or markdown into an array of proposed context nodes.
 *
 * @param text     The raw text or markdown to parse
 * @param baseUri  Optional URI prefix hint (e.g. "contextfs://project/docs")
 */
export async function parseTextIntoContextNodes(
  text: string,
  baseUri = "contextfs://ingested"
): Promise<ProposedContextNode[]> {
  if (!ai) {
    throw new Error("GEMINI_API_KEY is not set — cannot parse text into context nodes.");
  }

  if (text.length > MAX_INPUT_CHARS) {
    throw new Error(
      `Input text is too large (${text.length.toLocaleString()} chars). ` +
      `Maximum is ${MAX_INPUT_CHARS.toLocaleString()} chars (~25k tokens).`
    );
  }

  const prompt = `You are an expert at decomposing technical documents into a hierarchical knowledge graph.

Given the following text, extract a set of context nodes that capture its key concepts, sections, and facts.
Each node represents one coherent topic or section.

URI conventions:
- Root nodes use the base URI: ${baseUri}
- Child nodes extend the parent URI with a slug, e.g.: ${baseUri}/authentication
- Use only lowercase letters, numbers, and hyphens in URI slugs
- Nested topics should use nested URIs, e.g.: ${baseUri}/authentication/oauth

For each node produce:
- uri: unique identifier (string)
- name: short human-readable name (3-8 words)
- abstract: dense single-paragraph summary suitable for search/embedding (~50-120 words)
- overview: optional richer explanation (~200-500 words), include if there is enough detail in the source
- content: optional full verbatim or near-verbatim excerpt from the source text, include for leaf nodes with specific facts/code/details
- parent_uri: URI of the parent node, or null for root nodes

Guidelines:
- Prefer a shallow hierarchy (2-3 levels max) unless the content is clearly deeply nested
- Each abstract must be self-contained and searchable
- Do not invent information not present in the source text
- Aim for 3-15 nodes depending on document length and complexity
- Ensure every non-root node's parent_uri matches another node's uri in the output

INPUT TEXT:
---
${text}
---

Respond with ONLY a JSON array of node objects. No markdown fences, no explanation.
Example shape: [{"uri":"...","name":"...","abstract":"...","overview":"...","content":"...","parent_uri":null}, ...]`;

  const response = await generateWithRetry(LLM_MODEL, prompt);

  const raw = response.text?.trim() || "";
  const parsed = extractJsonArray(raw);

  if (!parsed || !Array.isArray(parsed)) {
    throw new Error(`LLM returned unparseable output:\n${raw.slice(0, 500)}`);
  }

  // Validate and normalise each node
  const nodes: ProposedContextNode[] = parsed
    .filter((n): n is Record<string, unknown> => 
      typeof n === "object" && n !== null && 
      typeof (n as Record<string, unknown>).uri === "string" && 
      typeof (n as Record<string, unknown>).name === "string" && 
      typeof (n as Record<string, unknown>).abstract === "string"
    )
    .map((n) => ({
      uri: n.uri as string,
      name: n.name as string,
      abstract: n.abstract as string,
      overview: typeof n.overview === "string" && n.overview.trim() ? n.overview : undefined,
      content: typeof n.content === "string" && n.content.trim() ? n.content : undefined,
      parent_uri: typeof n.parent_uri === "string" ? n.parent_uri : null,
    }));

  if (nodes.length === 0) {
    throw new Error("LLM returned no valid context nodes.");
  }

  return nodes;
}
