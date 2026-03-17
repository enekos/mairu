import * as dotenv from "dotenv";
import * as path from "path";
import { createServer, IncomingMessage, ServerResponse } from "http";
import { URL } from "url";
import { createContextManager } from "./client";

dotenv.config({ path: path.resolve(__dirname, "..", ".env") });

const cm = createContextManager();
const port = Number(process.env.DASHBOARD_API_PORT || 8787);

function sendJson(res: ServerResponse<IncomingMessage>, statusCode: number, body: unknown) {
  res.writeHead(statusCode, {
    "Content-Type": "application/json; charset=utf-8",
    "Access-Control-Allow-Origin": "*",
    "Access-Control-Allow-Methods": "GET, POST, PUT, DELETE, OPTIONS",
    "Access-Control-Allow-Headers": "Content-Type",
  });
  res.end(JSON.stringify(body));
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

    // Dashboard overview
    if (pathname === "/api/dashboard" && req.method === "GET") {
      const [skills, memories, contextNodes] = await Promise.all([
        cm.listSkills(limit),
        cm.listMemories(limit),
        cm.listContextNodes(undefined, limit),
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

      const results: Record<string, any[]> = {};
      if (type === "all" || type === "skills") {
        results.skills = await cm.searchSkills(q, { topK });
      }
      if (type === "all" || type === "memories") {
        results.memories = await cm.searchMemories(q, { topK });
      }
      if (type === "all" || type === "context") {
        results.contextNodes = await cm.searchContext(q, { topK });
      }
      sendJson(res, 200, results);
      return;
    }

    // Skills CRUD
    if (pathname === "/api/skills") {
      if (req.method === "GET") {
        sendJson(res, 200, await cm.listSkills(limit));
        return;
      }
      if (req.method === "POST") {
        const body = await readBody(req);
        const result = await cm.addSkill(body.name, body.description, body.metadata ?? {});
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
        sendJson(res, 200, await cm.listMemories(limit));
        return;
      }
      if (req.method === "POST") {
        const body = await readBody(req);
        const useRouter = body.useRouter !== false;
        const result = await cm.addMemory(
          body.content,
          body.category ?? "observation",
          body.owner ?? "agent",
          body.importance ?? 5,
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
        sendJson(res, 200, await cm.listContextNodes(parentUri, limit));
        return;
      }
      if (req.method === "POST") {
        const body = await readBody(req);
        const useRouter = body.useRouter !== false;
        const result = await cm.addContextNode(
          body.uri,
          body.name,
          body.abstract,
          body.overview,
          body.content,
          body.parent_uri || null,
          body.metadata ?? {},
          useRouter
        );
        sendJson(res, 201, result);
        return;
      }
      if (req.method === "PUT") {
        const body = await readBody(req);
        const result = await cm.updateContextNode(body.uri, { abstract: body.abstract, overview: body.overview, content: body.content, metadata: body.metadata });
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

    sendJson(res, 404, { error: "Not found" });
  } catch (error: any) {
    console.error("[dashboardApi] error:", error?.message);
    sendJson(res, 500, { error: error?.message || "Internal server error" });
  }
}

const server = createServer(handleRequest);
server.listen(port, () => {
  console.log(`contextfs dashboard API v2 listening on http://localhost:${port}`);
});
