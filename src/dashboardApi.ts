import { createServer, IncomingMessage, ServerResponse } from "http";
import { URL } from "url";
import { createContextManager } from "./client";
import { config } from "./config";
import { executeVibeQuery, planVibeMutation, executeMutationOp, VibeMutationOp } from "./vibeEngine";

const cm = createContextManager();
const port = config.dashboardApiPort;

function sendJson(res: ServerResponse<IncomingMessage>, statusCode: number, body: unknown) {
  res.writeHead(statusCode, {
    "Content-Type": "application/json; charset=utf-8",
    "Access-Control-Allow-Origin": "*",
    "Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, OPTIONS",
    "Access-Control-Allow-Headers": "Content-Type",
  });
  res.end(JSON.stringify(body));
}


function validateString(val: unknown, name: string): string {
  if (typeof val !== "string" || !val.trim()) {
    throw new Error(`Invalid or missing ${name}`);
  }
  return val.trim();
}

async function readBody(req: IncomingMessage): Promise<any> {
  return new Promise((resolve, reject) => {
    let data = "";
    req.on("data", (chunk) => { data += chunk; });
    req.on("end", () => {
      try { resolve(data ? JSON.parse(data) : {}); }
      catch (err) { reject(err); }
    });
    req.on("error", reject);
  });
}

async function handleRequest(req: IncomingMessage, res: ServerResponse<IncomingMessage>) {
  if (req.method === "OPTIONS") { sendJson(res, 204, {}); return; }

  const parsed = new URL(req.url || "/", `http://${req.headers.host || "localhost"}`);
  const limit = Number(parsed.searchParams.get("limit") || "200");
  const { pathname } = parsed;

  try {
    // Health
    if (pathname === "/api/health") {
      sendJson(res, 200, { ok: true, version: "2.0.0" });
      return;
    }

    // Cluster stats
    if (pathname === "/api/cluster" && req.method === "GET") {
      const stats = await cm.getClusterStats();
      sendJson(res, 200, stats);
      return;
    }

    // Dashboard overview
    if (pathname === "/api/dashboard" && req.method === "GET") {
      const [skills, memories, contextNodes] = await Promise.all([
        cm.listSkills({}, limit),
        cm.listMemories({}, limit),
        cm.listContextNodes(undefined, {}, limit),
      ]);
      sendJson(res, 200, {
        counts: { skills: skills.length, memories: memories.length, contextNodes: contextNodes.length },
        skills,
        memories,
        contextNodes,
      });
      return;
    }

    // Vector search endpoint (used by dashboard search bar)
    if (pathname === "/api/search" && req.method === "GET") {
      const q = parsed.searchParams.get("q") || "";
      const type = parsed.searchParams.get("type") || "all";
      const topK = Number(parsed.searchParams.get("topK") || "10");

      if (!q) { sendJson(res, 400, { error: "q parameter required" }); return; }

      // ES tuning params from query string
      const tuning: Record<string, any> = {};
      const fz = parsed.searchParams.get("fuzziness");
      if (fz) tuning.fuzziness = fz === "auto" ? "auto" : Number(fz);
      const pb = parsed.searchParams.get("phraseBoost");
      if (pb) tuning.phraseBoost = Number(pb);
      const ms = parsed.searchParams.get("minScore");
      if (ms) tuning.minScore = Number(ms);
      if (parsed.searchParams.get("highlight") === "true") tuning.highlight = true;
      const recencyScale = parsed.searchParams.get("recencyScale");
      if (recencyScale) tuning.recencyScale = recencyScale;
      const recencyDecay = parsed.searchParams.get("recencyDecay");
      if (recencyDecay) tuning.recencyDecay = Number(recencyDecay);

      const wv = parsed.searchParams.get("weightVector");
      const wk = parsed.searchParams.get("weightKeyword");
      const wr = parsed.searchParams.get("weightRecency");
      const wi = parsed.searchParams.get("weightImportance");
      if (wv || wk || wr || wi) {
        tuning.weights = {};
        if (wv) tuning.weights.vector = Number(wv);
        if (wk) tuning.weights.keyword = Number(wk);
        if (wr) tuning.weights.recency = Number(wr);
        if (wi) tuning.weights.importance = Number(wi);
      }

      const opts = { topK, ...tuning };

      const results: Record<string, any[]> = {};
      if (type === "all" || type === "skills") {
        results.skills = await cm.searchSkills(q, opts);
      }
      if (type === "all" || type === "memories") {
        results.memories = await cm.searchMemories(q, opts);
      }
      if (type === "all" || type === "context") {
        results.contextNodes = await cm.searchContext(q, opts);
      }
      sendJson(res, 200, results);
      return;
    }

    // Skills CRUD
    if (pathname === "/api/skills") {
      if (req.method === "GET") {
        sendJson(res, 200, await cm.listSkills({}, limit));
        return;
      }
      if (req.method === "POST") {
        const body = await readBody(req);
        const name = validateString(body.name, "name");
        const description = validateString(body.description, "description");
        const result = await cm.addSkill(name, description, body.project, body.metadata ?? {});
        sendJson(res, 201, result);
        return;
      }
      if (req.method === "PUT") {
        const body = await readBody(req);
        const result = await cm.updateSkill(body.id, { name: body.name, description: body.description, metadata: body.metadata });
        sendJson(res, 200, result);
        return;
      }
      if (req.method === "DELETE") {
        const id = parsed.searchParams.get("id");
        if (id) await cm.deleteSkill(id);
        sendJson(res, 200, { ok: true });
        return;
      }
    }

    // Memories CRUD
    if (pathname === "/api/memories") {
      if (req.method === "GET") {
        sendJson(res, 200, await cm.listMemories({}, limit));
        return;
      }
      if (req.method === "POST") {
        const body = await readBody(req);
        const useRouter = body.useRouter !== false;
        const result = await cm.addMemory(
          validateString(body.content, "content"),
          body.category ?? "observation",
          body.owner ?? "agent",
          body.importance ?? 5,
          body.project,
          body.metadata ?? {},
          useRouter
        );
        sendJson(res, 201, result);
        return;
      }
      if (req.method === "PUT") {
        const body = await readBody(req);
        const result = await cm.updateMemory(body.id, { content: body.content, importance: body.importance, metadata: body.metadata });
        sendJson(res, 200, result);
        return;
      }
      if (req.method === "DELETE") {
        const id = parsed.searchParams.get("id");
        if (id) await cm.deleteMemory(id);
        sendJson(res, 200, { ok: true });
        return;
      }
    }

    // Context nodes CRUD
    if (pathname === "/api/context") {
      if (req.method === "GET") {
        const parentUri = parsed.searchParams.get("parentUri") || undefined;
        sendJson(res, 200, await cm.listContextNodes(parentUri, {}, limit));
        return;
      }
      if (req.method === "POST") {
        const body = await readBody(req);
        const useRouter = body.useRouter !== false;
        const result = await cm.addContextNode(
          validateString(body.uri, "uri"),
          validateString(body.name, "name"),
          validateString(body.abstract, "abstract"),
          body.overview,
          body.content,
          body.parent_uri || null,
          body.project,
          body.metadata ?? {},
          useRouter
        );
        sendJson(res, 201, result);
        return;
      }
      if (req.method === "PUT") {
        const body = await readBody(req);
        const result = await cm.updateContextNode(body.uri, { name: body.name, abstract: body.abstract, overview: body.overview, content: body.content, metadata: body.metadata });
        sendJson(res, 200, result);
        return;
      }
      if (req.method === "DELETE") {
        const uri = parsed.searchParams.get("uri");
        if (uri) await cm.deleteContextNode(uri);
        sendJson(res, 200, { ok: true });
        return;
      }
    }

    // Vibe Query
    if (pathname === "/api/vibe/query" && req.method === "POST") {
      const body = await readBody(req);
      const prompt = validateString(body.prompt, "prompt");
      const result = await executeVibeQuery(cm, prompt, body.project, body.topK ?? 5);
      sendJson(res, 200, result);
      return;
    }

    // Vibe Mutation — plan
    if (pathname === "/api/vibe/mutation/plan" && req.method === "POST") {
      const body = await readBody(req);
      const prompt = validateString(body.prompt, "prompt");
      const plan = await planVibeMutation(cm, prompt, body.project, body.topK ?? 10);
      sendJson(res, 200, plan);
      return;
    }

    // Vibe Mutation — execute approved operations
    if (pathname === "/api/vibe/mutation/execute" && req.method === "POST") {
      const body = await readBody(req);
      if (!Array.isArray(body.operations)) {
        sendJson(res, 400, { error: "operations array required" });
        return;
      }
      const results: Array<{ op: string; result?: string; error?: string }> = [];
      for (const op of body.operations as VibeMutationOp[]) {
        try {
          const result = await executeMutationOp(cm, op, body.project);
          results.push({ op: op.op, result });
        } catch (err) {
          results.push({ op: op.op, error: err instanceof Error ? err.message : String(err) });
        }
      }
      sendJson(res, 200, { results });
      return;
    }

    sendJson(res, 404, { error: "Not found" });
  } catch (error: unknown) {
    console.error("[dashboardApi] error:", error instanceof Error ? error.message : String(error));
    sendJson(res, 500, { error: error instanceof Error ? error.message : String(error) || "Internal server error" });
  }
}

const server = createServer(handleRequest);
server.listen(port, () => {
  console.log(`contextfs dashboard API v2 listening on http://localhost:${port}`);
});
