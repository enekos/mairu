#!/usr/bin/env node
import { Command } from "commander";
import { createContextManager } from "./client";

const cm = createContextManager();
const program = new Command();

program
  .name("context-cli")
  .description("contextfs CLI — manage agent memory, skills, and context nodes")
  .version("2.0.0");

// ─────────────────────────────────────────────────────────────────────────────
// Memories
// ─────────────────────────────────────────────────────────────────────────────
const memCmd = program.command("memory").description("Manage memories");

memCmd
  .command("store <content>")
  .description("Intelligently store a memory (LLM decides create/update/skip)")
  .option("-c, --category <cat>", "Category", "observation")
  .option("-o, --owner <owner>", "Owner: user | agent | system", "agent")
  .option("-i, --importance <n>", "Importance 1-10", "5")
  .action(async (content, opts) => {
    try {
      const result = await cm.addMemory(content, opts.category, opts.owner, parseInt(opts.importance), {}, true);
      console.log(JSON.stringify(result, null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

memCmd
  .command("add <content>")
  .description("Force-create a new memory (skips LLM dedup check)")
  .option("-c, --category <cat>", "Category", "observation")
  .option("-o, --owner <owner>", "Owner: user | agent | system", "agent")
  .option("-i, --importance <n>", "Importance 1-10", "1")
  .action(async (content, opts) => {
    try {
      const result = await cm.addMemory(content, opts.category, opts.owner, parseInt(opts.importance), {}, false);
      console.log(JSON.stringify(result, null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

memCmd
  .command("search <query>")
  .description("Search memories with hybrid vector + keyword re-ranking")
  .option("-k, --topK <n>", "Results to return", "10")
  .option("-t, --threshold <n>", "Max cosine distance (0-2)")
  .option("--owner <owner>", "Filter by owner")
  .option("--category <cat>", "Filter by category")
  .option("--minImportance <n>", "Min importance score")
  .option("--maxAgeDays <n>", "Max age in days")
  .action(async (query, opts) => {
    try {
      const results = await cm.searchMemories(query, {
        topK: parseInt(opts.topK),
        threshold: opts.threshold !== undefined ? parseFloat(opts.threshold) : undefined,
        owner: opts.owner,
        category: opts.category,
        minImportance: opts.minImportance !== undefined ? parseInt(opts.minImportance) : undefined,
        maxAgeDays: opts.maxAgeDays !== undefined ? parseInt(opts.maxAgeDays) : undefined,
      });
      console.log(JSON.stringify(results, null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

memCmd
  .command("update <id>")
  .description("Update a memory's content or importance")
  .option("--content <text>", "New content")
  .option("-i, --importance <n>", "New importance 1-10")
  .action(async (id, opts) => {
    try {
      const result = await cm.updateMemory(id, {
        content: opts.content,
        importance: opts.importance !== undefined ? parseInt(opts.importance) : undefined,
      });
      console.log(JSON.stringify(result, null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

memCmd
  .command("list")
  .description("List all memories (most recently updated first)")
  .option("-l, --limit <n>", "Max results", "100")
  .action(async (opts) => {
    try {
      console.log(JSON.stringify(await cm.listMemories(parseInt(opts.limit)), null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

memCmd
  .command("delete <id>")
  .description("Delete a memory by ID")
  .action(async (id) => {
    try {
      await cm.deleteMemory(id);
      console.log(`Deleted memory: ${id}`);
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

// ─────────────────────────────────────────────────────────────────────────────
// Skills
// ─────────────────────────────────────────────────────────────────────────────
const skillCmd = program.command("skill").description("Manage skills");

skillCmd
  .command("add <name> <description>")
  .description("Add a new skill")
  .action(async (name, description) => {
    try {
      console.log(JSON.stringify(await cm.addSkill(name, description), null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

skillCmd
  .command("search <query>")
  .description("Search skills with hybrid vector + keyword re-ranking")
  .option("-k, --topK <n>", "Results to return", "10")
  .option("-t, --threshold <n>", "Max cosine distance (0-2)")
  .option("--maxAgeDays <n>", "Max age in days")
  .action(async (query, opts) => {
    try {
      const results = await cm.searchSkills(query, {
        topK: parseInt(opts.topK),
        threshold: opts.threshold !== undefined ? parseFloat(opts.threshold) : undefined,
        maxAgeDays: opts.maxAgeDays !== undefined ? parseInt(opts.maxAgeDays) : undefined,
      });
      console.log(JSON.stringify(results, null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

skillCmd
  .command("list")
  .description("List all skills")
  .option("-l, --limit <n>", "Max results", "100")
  .action(async (opts) => {
    try {
      console.log(JSON.stringify(await cm.listSkills(parseInt(opts.limit)), null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

skillCmd
  .command("delete <id>")
  .description("Delete a skill by ID")
  .action(async (id) => {
    try {
      await cm.deleteSkill(id);
      console.log(`Deleted skill: ${id}`);
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

// ─────────────────────────────────────────────────────────────────────────────
// Context Nodes
// ─────────────────────────────────────────────────────────────────────────────
const nodeCmd = program.command("node").description("Manage hierarchical context nodes");

nodeCmd
  .command("store <uri> <name> <abstract>")
  .description("Intelligently store a context node (LLM decides create/update/skip)")
  .option("-o, --overview <text>", "L1 overview content")
  .option("-c, --content <text>", "L2 detailed content")
  .option("-p, --parent <uri>", "Parent node URI")
  .action(async (uri, name, abstract, opts) => {
    try {
      const result = await cm.addContextNode(uri, name, abstract, opts.overview, opts.content, opts.parent || null, {}, true);
      console.log(JSON.stringify(result, null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

nodeCmd
  .command("add <uri> <name> <abstract>")
  .description("Force-create a context node (skips LLM dedup check)")
  .option("-o, --overview <text>", "L1 overview content")
  .option("-c, --content <text>", "L2 detailed content")
  .option("-p, --parent <uri>", "Parent node URI")
  .action(async (uri, name, abstract, opts) => {
    try {
      const result = await cm.addContextNode(uri, name, abstract, opts.overview, opts.content, opts.parent || null, {}, false);
      console.log(JSON.stringify(result, null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

nodeCmd
  .command("search <query>")
  .description("Search context nodes (searches name, abstract, overview, content)")
  .option("-k, --topK <n>", "Results to return", "10")
  .option("-t, --threshold <n>", "Max cosine distance (0-2)")
  .option("--parentUri <uri>", "Filter by parent URI")
  .option("--maxAgeDays <n>", "Max age in days")
  .action(async (query, opts) => {
    try {
      const results = await cm.searchContext(query, {
        topK: parseInt(opts.topK),
        threshold: opts.threshold !== undefined ? parseFloat(opts.threshold) : undefined,
        parentUri: opts.parentUri,
        maxAgeDays: opts.maxAgeDays !== undefined ? parseInt(opts.maxAgeDays) : undefined,
      });
      console.log(JSON.stringify(results, null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

nodeCmd
  .command("update <uri>")
  .description("Update a context node")
  .option("--abstract <text>", "New abstract")
  .option("--overview <text>", "New overview")
  .option("--content <text>", "New content")
  .action(async (uri, opts) => {
    try {
      const result = await cm.updateContextNode(uri, {
        abstract: opts.abstract,
        overview: opts.overview,
        content: opts.content,
      });
      console.log(JSON.stringify(result, null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

nodeCmd
  .command("list")
  .description("List context nodes")
  .option("-p, --parent <uri>", "Filter by parent URI")
  .option("-l, --limit <n>", "Max results", "100")
  .action(async (opts) => {
    try {
      console.log(JSON.stringify(await cm.listContextNodes(opts.parent, parseInt(opts.limit)), null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

nodeCmd
  .command("delete <uri>")
  .description("Delete a context node (cascades to descendants)")
  .action(async (uri) => {
    try {
      await cm.deleteContextNode(uri);
      console.log(`Deleted context node: ${uri}`);
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

nodeCmd
  .command("subtree <uri>")
  .description("Get a node and all its descendants")
  .action(async (uri) => {
    try {
      console.log(JSON.stringify(await cm.getContextSubtree(uri), null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

nodeCmd
  .command("path <uri>")
  .description("Get the ancestor chain from a node to the root")
  .action(async (uri) => {
    try {
      console.log(JSON.stringify(await cm.getContextPath(uri), null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

nodeCmd
  .command("ls <uri>")
  .description("List direct children of a context node")
  .action(async (uri) => {
    try {
      console.log(JSON.stringify(await cm.listContextNodes(uri), null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

nodeCmd
  .command("read <uri>")
  .description("Read full content of a context node")
  .action(async (uri) => {
    try {
      const subtree = await cm.getContextSubtree(uri);
      const node = subtree.find((r: any) => r.uri === uri);
      console.log(node ? JSON.stringify(node, null, 2) : "Not found.");
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

program.parse(process.argv);
