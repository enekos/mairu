import * as dotenv from "dotenv";
import { createServer, IncomingMessage, ServerResponse } from "http";
import { URL } from "url";
import { createContextManager } from "./client";

dotenv.config({ path: require("path").resolve(__dirname, "..", ".env") });

const contextManager = createContextManager();
const port = Number(process.env.DASHBOARD_API_PORT || 8787);

function sendJson(res: ServerResponse<IncomingMessage>, statusCode: number, body: unknown) {
  res.writeHead(statusCode, {
    "Content-Type": "application/json; charset=utf-8",
    "Access-Control-Allow-Origin": "*",
    "Access-Control-Allow-Methods": "GET, OPTIONS",
    "Access-Control-Allow-Headers": "Content-Type",
  });
  res.end(JSON.stringify(body));
}

async function handleRequest(req: IncomingMessage, res: ServerResponse<IncomingMessage>) {
  if (req.method === "OPTIONS") {
    sendJson(res, 204, {});
    return;
  }

  if (req.method !== "GET") {
    sendJson(res, 405, { error: "Method not allowed" });
    return;
  }

  const parsed = new URL(req.url || "/", `http://${req.headers.host || "localhost"}`);
  const limit = Number(parsed.searchParams.get("limit") || "100");

  try {
    if (parsed.pathname === "/api/health") {
      sendJson(res, 200, { ok: true });
      return;
    }

    if (parsed.pathname === "/api/skills") {
      const skills = await contextManager.listSkills(limit);
      sendJson(res, 200, skills);
      return;
    }

    if (parsed.pathname === "/api/memories") {
      const memories = await contextManager.listMemories(limit);
      sendJson(res, 200, memories);
      return;
    }

    if (parsed.pathname === "/api/context") {
      const contextNodes = await contextManager.listContextNodes(undefined, limit);
      sendJson(res, 200, contextNodes);
      return;
    }

    if (parsed.pathname === "/api/dashboard") {
      const [skills, memories, contextNodes] = await Promise.all([
        contextManager.listSkills(limit),
        contextManager.listMemories(limit),
        contextManager.listContextNodes(undefined, limit),
      ]);

      sendJson(res, 200, {
        counts: {
          skills: skills.length,
          memories: memories.length,
          contextNodes: contextNodes.length,
        },
        skills,
        memories,
        contextNodes,
      });
      return;
    }

    sendJson(res, 404, { error: "Not found" });
  } catch (error: any) {
    sendJson(res, 500, { error: error?.message || "Internal server error" });
  }
}

const server = createServer(handleRequest);
server.listen(port, () => {
  console.log(`Dashboard API listening on http://localhost:${port}`);
});
