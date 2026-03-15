#!/usr/bin/env node
import { Command } from "commander";
import { createContextManager } from "./client";

const contextManager = createContextManager();

const program = new Command();
program
  .name("context-cli")
  .description(
    "CLI to manage the Ultimate Context DB for local code agents (Turso + Vector Search)"
  )
  .version("1.0.0");

// --- Memories ---
const memoryCmd = program.command("memory").description("Manage memories");

memoryCmd
  .command("add <content>")
  .description("Add a new memory")
  .option(
    "-c, --category <category>",
    "Category: profile, preferences, entities, events, cases, patterns, observation, reflection",
    "observation"
  )
  .option("-o, --owner <owner>", "Owner: user, agent, system", "agent")
  .option("-i, --importance <number>", "Importance score (1-10)", "1")
  .action(async (content, options) => {
    try {
      const memory = await contextManager.addMemory(
        content,
        options.category,
        options.owner,
        parseInt(options.importance)
      );
      console.log("Added memory:", JSON.stringify(memory, null, 2));
    } catch (e) {
      console.error("Error:", e);
    }
  });

memoryCmd
  .command("search <query>")
  .description("Search memories via hybrid vector + keyword ranking")
  .option("-k, --topK <number>", "Number of results to return", "5")
  .option("-t, --threshold <number>", "Max cosine distance to include (0-2, lower = more similar)")
  .option("--owner <owner>", "Filter by owner (user, agent, system)")
  .option("--category <category>", "Filter by category")
  .option("--minImportance <number>", "Filter by minimum importance score")
  .option("--maxAgeDays <number>", "Filter to memories not older than N days")
  .option("--vectorWeight <number>", "Hybrid rank vector score weight")
  .option("--keywordWeight <number>", "Hybrid rank keyword score weight")
  .option("--importanceWeight <number>", "Hybrid rank importance score weight")
  .option("--recencyWeight <number>", "Hybrid rank recency score weight")
  .action(async (query, options) => {
    try {
      const threshold = options.threshold !== undefined ? parseFloat(options.threshold) : undefined;
      const results = await contextManager.searchMemories(query, {
        topK: parseInt(options.topK),
        threshold,
        owner: options.owner,
        category: options.category,
        minImportance:
          options.minImportance !== undefined ? parseInt(options.minImportance) : undefined,
        maxAgeDays: options.maxAgeDays !== undefined ? parseInt(options.maxAgeDays) : undefined,
        weights: {
          vector: options.vectorWeight !== undefined ? parseFloat(options.vectorWeight) : 0.65,
          keyword: options.keywordWeight !== undefined ? parseFloat(options.keywordWeight) : 0.15,
          importance:
            options.importanceWeight !== undefined ? parseFloat(options.importanceWeight) : 0.15,
          recency: options.recencyWeight !== undefined ? parseFloat(options.recencyWeight) : 0.05,
        },
      });
      console.log(JSON.stringify(results, null, 2));
    } catch (e) {
      console.error("Error:", e);
    }
  });

memoryCmd
  .command("list")
  .description("List all memories (most recent first)")
  .option("-l, --limit <number>", "Max number of results", "50")
  .action(async (options) => {
    try {
      const results = await contextManager.listMemories(parseInt(options.limit));
      console.log(JSON.stringify(results, null, 2));
    } catch (e) {
      console.error("Error:", e);
    }
  });

memoryCmd
  .command("delete <id>")
  .description("Delete a memory by ID")
  .action(async (id) => {
    try {
      await contextManager.deleteMemory(id);
      console.log(`Deleted memory: ${id}`);
    } catch (e) {
      console.error("Error:", e);
    }
  });

// --- Skills ---
const skillCmd = program.command("skill").description("Manage skills");

skillCmd
  .command("add <name> <description>")
  .description("Add a new skill")
  .action(async (name, description) => {
    try {
      const skill = await contextManager.addSkill(name, description);
      console.log("Added skill:", JSON.stringify(skill, null, 2));
    } catch (e) {
      console.error("Error:", e);
    }
  });

skillCmd
  .command("search <query>")
  .description("Search skills via hybrid vector + keyword ranking")
  .option("-k, --topK <number>", "Number of results to return", "5")
  .option("-t, --threshold <number>", "Max cosine distance to include (0-2, lower = more similar)")
  .option("--maxAgeDays <number>", "Filter to skills not older than N days")
  .option("--vectorWeight <number>", "Hybrid rank vector score weight")
  .option("--keywordWeight <number>", "Hybrid rank keyword score weight")
  .option("--recencyWeight <number>", "Hybrid rank recency score weight")
  .action(async (query, options) => {
    try {
      const threshold = options.threshold !== undefined ? parseFloat(options.threshold) : undefined;
      const results = await contextManager.searchSkills(query, {
        topK: parseInt(options.topK),
        threshold,
        maxAgeDays: options.maxAgeDays !== undefined ? parseInt(options.maxAgeDays) : undefined,
        weights: {
          vector: options.vectorWeight !== undefined ? parseFloat(options.vectorWeight) : 0.8,
          keyword: options.keywordWeight !== undefined ? parseFloat(options.keywordWeight) : 0.2,
          recency: options.recencyWeight !== undefined ? parseFloat(options.recencyWeight) : 0,
        },
      });
      console.log(JSON.stringify(results, null, 2));
    } catch (e) {
      console.error("Error:", e);
    }
  });

skillCmd
  .command("list")
  .description("List all skills (most recent first)")
  .option("-l, --limit <number>", "Max number of results", "50")
  .action(async (options) => {
    try {
      const results = await contextManager.listSkills(parseInt(options.limit));
      console.log(JSON.stringify(results, null, 2));
    } catch (e) {
      console.error("Error:", e);
    }
  });

skillCmd
  .command("delete <id>")
  .description("Delete a skill by ID")
  .action(async (id) => {
    try {
      await contextManager.deleteSkill(id);
      console.log(`Deleted skill: ${id}`);
    } catch (e) {
      console.error("Error:", e);
    }
  });

// --- Context Nodes ---
const contextNodeCmd = program.command("node").description("Manage hierarchical context nodes");

contextNodeCmd
  .command("add <uri> <name> <abstract>")
  .description("Add a context node (OpenContextFS file system paradigm)")
  .option("-o, --overview <content>", "L1 overview content")
  .option("-c, --content <content>", "L2 detailed content")
  .option("-p, --parent <uri>", "Parent node URI")
  .action(async (uri, name, abstract, options) => {
    try {
      const node = await contextManager.addContextNode(
        uri,
        name,
        abstract,
        options.overview,
        options.content,
        options.parent || null
      );
      console.log("Added context node:", JSON.stringify(node, null, 2));
    } catch (e) {
      console.error("Error:", e);
    }
  });

contextNodeCmd
  .command("search <query>")
  .description("Search context nodes via hybrid vector + keyword ranking")
  .option("-k, --topK <number>", "Number of results to return", "5")
  .option("-t, --threshold <number>", "Max cosine distance to include (0-2, lower = more similar)")
  .option("--parentUri <uri>", "Filter by parent URI")
  .option("--maxAgeDays <number>", "Filter to nodes not older than N days")
  .option("--vectorWeight <number>", "Hybrid rank vector score weight")
  .option("--keywordWeight <number>", "Hybrid rank keyword score weight")
  .option("--recencyWeight <number>", "Hybrid rank recency score weight")
  .action(async (query, options) => {
    try {
      const threshold = options.threshold !== undefined ? parseFloat(options.threshold) : undefined;
      const results = await contextManager.searchContext(query, {
        topK: parseInt(options.topK),
        threshold,
        parentUri: options.parentUri,
        maxAgeDays: options.maxAgeDays !== undefined ? parseInt(options.maxAgeDays) : undefined,
        weights: {
          vector: options.vectorWeight !== undefined ? parseFloat(options.vectorWeight) : 0.8,
          keyword: options.keywordWeight !== undefined ? parseFloat(options.keywordWeight) : 0.2,
          recency: options.recencyWeight !== undefined ? parseFloat(options.recencyWeight) : 0,
        },
      });
      console.log(JSON.stringify(results, null, 2));
    } catch (e) {
      console.error("Error:", e);
    }
  });

contextNodeCmd
  .command("list")
  .description("List all context nodes (most recent first)")
  .option("-p, --parent <uri>", "Filter by parent URI")
  .option("-l, --limit <number>", "Max number of results", "50")
  .action(async (options) => {
    try {
      const results = await contextManager.listContextNodes(options.parent, parseInt(options.limit));
      console.log(JSON.stringify(results, null, 2));
    } catch (e) {
      console.error("Error:", e);
    }
  });

contextNodeCmd
  .command("delete <uri>")
  .description("Delete a context node (and its descendants via CASCADE)")
  .action(async (uri) => {
    try {
      await contextManager.deleteContextNode(uri);
      console.log(`Deleted context node: ${uri}`);
    } catch (e) {
      console.error("Error:", e);
    }
  });

contextNodeCmd
  .command("subtree <id>")
  .description("Get a context node and all of its deep descendants")
  .action(async (id) => {
    try {
      const results = await contextManager.getContextSubtree(id);
      console.log(JSON.stringify(results, null, 2));
    } catch (e) {
      console.error("Error:", e);
    }
  });

contextNodeCmd
  .command("path <id>")
  .description("Get the ancestor path from a node up to the root")
  .action(async (id) => {
    try {
      const results = await contextManager.getContextPath(id);
      console.log(JSON.stringify(results, null, 2));
    } catch (e) {
      console.error("Error:", e);
    }
  });

contextNodeCmd
  .command("ls <uri>")
  .description("List direct children of a context node")
  .action(async (uri) => {
    try {
      const results = await contextManager.listContextNodes(uri);
      console.log(JSON.stringify(results, null, 2));
    } catch (e) {
      console.error("Error:", e);
    }
  });

contextNodeCmd
  .command("read <uri>")
  .description("Read the detailed content of a context node")
  .action(async (uri) => {
    try {
      const results = await contextManager.getContextSubtree(uri);
      const node = results.find((r: any) => r.uri === uri);
      if (node) {
        console.log(JSON.stringify(node, null, 2));
      } else {
        console.log("Not found.");
      }
    } catch (e) {
      console.error("Error:", e);
    }
  });

program.parse(process.argv);
