#!/usr/bin/env node
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { CallToolRequestSchema, ListToolsRequestSchema } from "@modelcontextprotocol/sdk/types.js";
import { createContextManager } from "./client";

const cm = createContextManager();

const server = new Server(
  { name: "contextfs", version: "2.0.0" },
  { capabilities: { tools: {} } }
);

server.setRequestHandler(ListToolsRequestSchema, async () => ({
  tools: [
    // -------------------------------------------------------------------------
    // Memories
    // -------------------------------------------------------------------------
    {
      name: "store_memory",
      description:
        "Intelligently store a memory. Uses an LLM to decide whether to create a new memory, " +
        "update/merge with an existing one, or skip if already captured. " +
        "Prefer this over add_memory for most writes — it prevents duplicate/fragmented memories.",
      inputSchema: {
        type: "object",
        properties: {
          content: { type: "string", description: "The memory content (self-contained, no pronouns)" },
          category: {
            type: "string",
            enum: ["profile","preferences","entities","events","cases","patterns","observation","reflection","decision","constraint","architecture"],
            description: "Memory category",
          },
          owner: { type: "string", enum: ["user","agent","system"], description: "Memory owner" },
          importance: { type: "number", description: "Priority 1-10. 7+ for facts you always want to recall." },
        },
        required: ["content"],
      },
    },
    {
      name: "add_memory",
      description:
        "Force-create a new memory without the LLM deduplication check. " +
        "Use this only when you are certain the information is new.",
      inputSchema: {
        type: "object",
        properties: {
          content: { type: "string" },
          category: {
            type: "string",
            enum: ["profile","preferences","entities","events","cases","patterns","observation","reflection","decision","constraint","architecture"],
          },
          owner: { type: "string", enum: ["user","agent","system"] },
          importance: { type: "number", description: "1-10" },
        },
        required: ["content"],
      },
    },
    {
      name: "search_memories",
      description:
        "Search memories with hybrid vector + multi-token keyword re-ranking. " +
        "Returns up to topK results with _hybrid_score so you can judge relevance. " +
        "Results are broadly retrieved and properly ordered — prefer topK 10-20 to get good coverage.",
      inputSchema: {
        type: "object",
        properties: {
          query: { type: "string", description: "Natural language or keyword query" },
          topK: { type: "number", description: "Max results to return (default 10)" },
          threshold: { type: "number", description: "Max cosine distance 0-2 (lower = stricter)" },
          owner: { type: "string", enum: ["user","agent","system"] },
          category: {
            type: "string",
            enum: ["profile","preferences","entities","events","cases","patterns","observation","reflection","decision","constraint","architecture"],
          },
          minImportance: { type: "number" },
          maxAgeDays: { type: "number" },
        },
        required: ["query"],
      },
    },
    {
      name: "update_memory",
      description: "Update an existing memory's content, importance, or metadata. Re-embeds if content changes.",
      inputSchema: {
        type: "object",
        properties: {
          id: { type: "string", description: "Memory ID" },
          content: { type: "string" },
          importance: { type: "number" },
        },
        required: ["id"],
      },
    },

    // -------------------------------------------------------------------------
    // Skills
    // -------------------------------------------------------------------------
    {
      name: "add_skill",
      description: "Add a reusable skill or capability description to the skills store.",
      inputSchema: {
        type: "object",
        properties: {
          name: { type: "string", description: "Short skill name" },
          description: { type: "string", description: "Full description of what the skill does and when to use it" },
        },
        required: ["name", "description"],
      },
    },
    {
      name: "search_skills",
      description:
        "Search skills with hybrid vector + keyword re-ranking. " +
        "Returns skills ordered by relevance with _hybrid_score.",
      inputSchema: {
        type: "object",
        properties: {
          query: { type: "string" },
          topK: { type: "number", description: "Max results (default 10)" },
          threshold: { type: "number" },
          maxAgeDays: { type: "number" },
        },
        required: ["query"],
      },
    },

    // -------------------------------------------------------------------------
    // Context Nodes
    // -------------------------------------------------------------------------
    {
      name: "store_context",
      description:
        "Intelligently store a context node about a project, file, module, or concept. " +
        "Uses an LLM to decide whether to create, merge with an existing node, or skip. " +
        "Use this when indexing project knowledge — it prevents fragmentation.",
      inputSchema: {
        type: "object",
        properties: {
          uri: { type: "string", description: "Unique URI e.g. contextfs://project/backend/auth" },
          name: { type: "string", description: "Human-readable name" },
          abstract: { type: "string", description: "~100-token summary used for search and embedding" },
          overview: { type: "string", description: "~2k-token overview (optional, for richer context)" },
          content: { type: "string", description: "Full detail content (optional, loaded on demand)" },
          parent_uri: { type: "string", description: "Parent node URI for hierarchy" },
        },
        required: ["uri", "name", "abstract"],
      },
    },
    {
      name: "add_context_node",
      description: "Force-create a context node without the LLM deduplication check.",
      inputSchema: {
        type: "object",
        properties: {
          uri: { type: "string" },
          name: { type: "string" },
          abstract: { type: "string", description: "~100-token summary" },
          overview: { type: "string" },
          content: { type: "string" },
          parent_uri: { type: "string" },
        },
        required: ["uri", "name", "abstract"],
      },
    },
    {
      name: "search_context",
      description:
        "Search context nodes across all layers (name, abstract, overview, content) " +
        "with hybrid vector + keyword re-ranking. Returns results ordered by _hybrid_score.",
      inputSchema: {
        type: "object",
        properties: {
          query: { type: "string" },
          topK: { type: "number", description: "Max results (default 10)" },
          threshold: { type: "number" },
          parentUri: { type: "string", description: "Restrict to children of a given URI" },
          maxAgeDays: { type: "number" },
        },
        required: ["query"],
      },
    },
    {
      name: "update_context_node",
      description: "Update a context node's abstract, overview, or content. Re-embeds if abstract changes.",
      inputSchema: {
        type: "object",
        properties: {
          uri: { type: "string" },
          abstract: { type: "string" },
          overview: { type: "string" },
          content: { type: "string" },
        },
        required: ["uri"],
      },
    },
    {
      name: "get_context_subtree",
      description: "Get a context node and all its descendants (useful for exploring a whole subsystem).",
      inputSchema: {
        type: "object",
        properties: {
          uri: { type: "string" },
        },
        required: ["uri"],
      },
    },
    {
      name: "get_context_path",
      description: "Get the ancestor chain from a context node up to the root.",
      inputSchema: {
        type: "object",
        properties: {
          uri: { type: "string" },
        },
        required: ["uri"],
      },
    },
  ],
}));

server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args } = request.params;
  if (!args) throw new Error("Arguments missing");

  try {
    let result: any;

    switch (name) {
      case "store_memory":
        result = await cm.addMemory(
          args.content as string,
          (args.category as any) ?? "observation",
          (args.owner as any) ?? "agent",
          (args.importance as number) ?? 5,
          {},
          true  // useRouter = true
        );
        break;

      case "add_memory":
        result = await cm.addMemory(
          args.content as string,
          (args.category as any) ?? "observation",
          (args.owner as any) ?? "agent",
          (args.importance as number) ?? 1,
          {},
          false  // useRouter = false
        );
        break;

      case "search_memories":
        result = await cm.searchMemories(args.query as string, {
          topK: (args.topK as number) ?? 10,
          threshold: args.threshold as number | undefined,
          owner: args.owner as any,
          category: args.category as any,
          minImportance: args.minImportance as number | undefined,
          maxAgeDays: args.maxAgeDays as number | undefined,
        });
        break;

      case "update_memory":
        result = await cm.updateMemory(args.id as string, {
          content: args.content as string | undefined,
          importance: args.importance as number | undefined,
        });
        break;

      case "add_skill":
        result = await cm.addSkill(args.name as string, args.description as string);
        break;

      case "search_skills":
        result = await cm.searchSkills(args.query as string, {
          topK: (args.topK as number) ?? 10,
          threshold: args.threshold as number | undefined,
          maxAgeDays: args.maxAgeDays as number | undefined,
        });
        break;

      case "store_context":
        result = await cm.addContextNode(
          args.uri as string,
          args.name as string,
          args.abstract as string,
          args.overview as string | undefined,
          args.content as string | undefined,
          (args.parent_uri as string) || null,
          {},
          true  // useRouter = true
        );
        break;

      case "add_context_node":
        result = await cm.addContextNode(
          args.uri as string,
          args.name as string,
          args.abstract as string,
          args.overview as string | undefined,
          args.content as string | undefined,
          (args.parent_uri as string) || null,
          {},
          false  // useRouter = false
        );
        break;

      case "search_context":
        result = await cm.searchContext(args.query as string, {
          topK: (args.topK as number) ?? 10,
          threshold: args.threshold as number | undefined,
          parentUri: args.parentUri as string | undefined,
          maxAgeDays: args.maxAgeDays as number | undefined,
        });
        break;

      case "update_context_node":
        result = await cm.updateContextNode(args.uri as string, {
          abstract: args.abstract as string | undefined,
          overview: args.overview as string | undefined,
          content: args.content as string | undefined,
        });
        break;

      case "get_context_subtree":
        result = await cm.getContextSubtree(args.uri as string);
        break;

      case "get_context_path":
        result = await cm.getContextPath(args.uri as string);
        break;

      default:
        throw new Error(`Unknown tool: ${name}`);
    }

    return { content: [{ type: "text", text: JSON.stringify(result, null, 2) }] };
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
  console.error("contextfs MCP server v2 running on stdio");
}

main().catch((error) => {
  console.error("Fatal error:", error);
  process.exit(1);
});
