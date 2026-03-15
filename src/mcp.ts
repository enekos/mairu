#!/usr/bin/env node
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { CallToolRequestSchema, ListToolsRequestSchema } from "@modelcontextprotocol/sdk/types.js";
import { createContextManager } from "./client";

const contextManager = createContextManager();

const server = new Server(
  {
    name: "turso-context-db",
    version: "1.0.0",
  },
  {
    capabilities: {
      tools: {},
    },
  }
);

// Define tools available to Claude via MCP
server.setRequestHandler(ListToolsRequestSchema, async () => {
  return {
    tools: [
      {
        name: "add_memory",
        description: "Add a new memory for the agent context.",
        inputSchema: {
          type: "object",
          properties: {
            content: { type: "string", description: "The memory content" },
            category: {
              type: "string",
              enum: ["profile", "preferences", "entities", "events", "cases", "patterns", "observation", "reflection"],
              description: "Memory category",
            },
            owner: { type: "string", enum: ["user", "agent", "system"], description: "Memory owner" },
            importance: { type: "number", description: "1-10 priority scale" },
          },
          required: ["content"],
        },
      },
      {
        name: "search_memories",
        description: "Search memories with hybrid vector + keyword ranking.",
        inputSchema: {
          type: "object",
          properties: {
            query: { type: "string", description: "Semantic search query" },
            topK: { type: "number", description: "Number of results to return" },
            threshold: { type: "number", description: "Max cosine distance filter (lower is better)" },
            owner: { type: "string", enum: ["user", "agent", "system"] },
            category: {
              type: "string",
              enum: ["profile", "preferences", "entities", "events", "cases", "patterns", "observation", "reflection"],
            },
            minImportance: { type: "number" },
            maxAgeDays: { type: "number" },
          },
          required: ["query"],
        },
      },
      {
        name: "add_skill",
        description: "Add a new skill representation.",
        inputSchema: {
          type: "object",
          properties: {
            name: { type: "string" },
            description: { type: "string" },
          },
          required: ["name", "description"],
        },
      },
      {
        name: "search_skills",
        description: "Search skills with hybrid vector + keyword ranking.",
        inputSchema: {
          type: "object",
          properties: {
            query: { type: "string" },
            topK: { type: "number" },
            threshold: { type: "number" },
            maxAgeDays: { type: "number" },
          },
          required: ["query"],
        },
      },
      {
        name: "add_context_node",
        description: "Add a context node to the hierarchical tree.",
        inputSchema: {
          type: "object",
          properties: {
            uri: { type: "string", description: "Unique URI (e.g. contextfs://duo/backend/auth)" },
            name: { type: "string" },
            abstract: { type: "string", description: "Short abstract used for vector search (~100 tokens)" },
            overview: { type: "string", description: "Optional L1 overview (~2k tokens)" },
            content: { type: "string", description: "Optional L2 detailed content" },
            parent_uri: { type: "string", description: "Optional parent node URI" },
          },
          required: ["uri", "name", "abstract"],
        },
      },
      {
        name: "search_context",
        description: "Search hierarchical context nodes with hybrid ranking.",
        inputSchema: {
          type: "object",
          properties: {
            query: { type: "string" },
            topK: { type: "number" },
            threshold: { type: "number" },
            parentUri: { type: "string" },
            maxAgeDays: { type: "number" },
          },
          required: ["query"],
        },
      },
    ],
  };
});

// Handle tool execution requests
server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args } = request.params;

  if (!args) {
    throw new Error("Arguments missing");
  }

  try {
    let result;
    switch (name) {
      case "add_memory":
        result = await contextManager.addMemory(
          args.content as string,
          (args.category as any) ?? "observation",
          (args.owner as any) ?? "agent",
          (args.importance as number) ?? 1
        );
        break;
      case "search_memories":
        result = await contextManager.searchMemories(args.query as string, {
          topK: args.topK as number | undefined,
          threshold: args.threshold as number | undefined,
          owner: args.owner as any,
          category: args.category as any,
          minImportance: args.minImportance as number | undefined,
          maxAgeDays: args.maxAgeDays as number | undefined,
        });
        break;
      case "add_skill":
        result = await contextManager.addSkill(args.name as string, args.description as string);
        break;
      case "search_skills":
        result = await contextManager.searchSkills(args.query as string, {
          topK: args.topK as number | undefined,
          threshold: args.threshold as number | undefined,
          maxAgeDays: args.maxAgeDays as number | undefined,
        });
        break;
      case "add_context_node":
        result = await contextManager.addContextNode(
          args.uri as string,
          args.name as string,
          args.abstract as string,
          args.overview as string | undefined,
          args.content as string | undefined,
          (args.parent_uri as string) || null
        );
        break;
      case "search_context":
        result = await contextManager.searchContext(args.query as string, {
          topK: args.topK as number | undefined,
          threshold: args.threshold as number | undefined,
          parentUri: args.parentUri as string | undefined,
          maxAgeDays: args.maxAgeDays as number | undefined,
        });
        break;
      default:
        throw new Error(`Unknown tool: ${name}`);
    }

    return {
      content: [{ type: "text", text: JSON.stringify(result, null, 2) }],
    };
  } catch (error: any) {
    return {
      content: [{ type: "text", text: `Error: ${error.message}` }],
      isError: true,
    };
  }
});

async function main() {
  const transport = new StdioServerTransport();
  await server.connect(transport);
  console.error("Turso Context DB MCP Server running on stdio");
}

main().catch((error) => {
  console.error("Fatal error in main():", error);
  process.exit(1);
});
