import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { z } from "zod";
import { createContextManager } from "../storage/client";
import {
  planVibeMutation,
  executeMutationOp,
} from "../llm/vibeEngine";

export async function runMcpServer(projectOverride?: string) {
  const cm = createContextManager();

  const server = new McpServer({
    name: "contextfs-mcp",
    version: "2.0.0",
  });

  server.tool(
    "search_memories",
    "Search agent memories and conventions",
    {
      query: z.string().describe("The search query"),
      topK: z.number().optional().default(5).describe("Number of results to return"),
      project: z.string().optional().describe("Project namespace"),
    },
    async ({ query, topK, project }) => {
      const proj = project || projectOverride;
      if (!proj) throw new Error("A project namespace is required (provide -P/--project globally or via parameter)");
      const res = await cm.searchMemories(query, { topK, project: proj });
      return {
        content: [{ type: "text", text: JSON.stringify(res, null, 2) }],
      };
    }
  );

  server.tool(
    "store_memory",
    "Store a memory, convention, or fact",
    {
      content: z.string().describe("The memory content"),
      project: z.string().optional().describe("Project namespace"),
    },
    async ({ content, project }) => {
      const proj = project || projectOverride;
      if (!proj) throw new Error("A project namespace is required (provide -P/--project globally or via parameter)");
      const res = await cm.addMemory(content, "observation", "agent", 5, proj, {}, true);
      return {
        content: [{ type: "text", text: JSON.stringify(res, null, 2) }],
      };
    }
  );

  server.tool(
    "search_nodes",
    "Search hierarchical architecture context nodes",
    {
      query: z.string().describe("The search query"),
      topK: z.number().optional().default(5).describe("Number of results to return"),
      project: z.string().optional().describe("Project namespace"),
    },
    async ({ query, topK, project }) => {
      const proj = project || projectOverride;
      if (!proj) throw new Error("A project namespace is required");
      const res = await cm.searchContext(query, { topK, project: proj });
      return {
        content: [{ type: "text", text: JSON.stringify(res, null, 2) }],
      };
    }
  );

  server.tool(
    "list_node_subtree",
    "List all child nodes inside a context node URI",
    {
      uri: z.string().describe("The URI to list (e.g. contextfs://my-project/backend)"),
      project: z.string().optional().describe("Project namespace"),
    },
    async ({ uri, project }) => {
      const proj = project || projectOverride;
      if (!proj) throw new Error("A project namespace is required");
      const res = await cm.listContextNodes(uri, { project: proj });
      return {
        content: [{ type: "text", text: JSON.stringify(res, null, 2) }],
      };
    }
  );

  server.tool(
    "vibe_mutation",
    "Perform a complex context modification via natural language. E.g. 'We decided to use Vitest instead of Jest'. This auto-reads context and plans the changes.",
    {
      instruction: z.string().describe("The mutation instruction"),
      project: z.string().optional().describe("Project namespace"),
    },
    async ({ instruction, project }) => {
      const proj = project || projectOverride;
      if (!proj) throw new Error("A project namespace is required");
      const plan = await planVibeMutation(cm, instruction, proj);
      const res = [];
      for (const op of plan.operations) {
        await executeMutationOp(cm, op, proj);
        res.push(`Executed ${op.op} on ${op.target} ${op.target || ""}`);
      }
      return {
        content: [{ type: "text", text: JSON.stringify({ plan, result: res }, null, 2) }],
      };
    }
  );

  const transport = new StdioServerTransport();
  await server.connect(transport);
  console.error("ContextFS MCP Server running on stdio");
}
