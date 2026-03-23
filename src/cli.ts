#!/usr/bin/env node
import { Command } from "commander";
import { createContextManager } from "./client";
import { AgentContextNode, SkippedWrite, UpdatedWrite } from "./types";
import { executeVibeQuery, planVibeMutation, executeMutationOp, VibeMutationOp } from "./vibeEngine";
import * as fs from "fs";
import * as readline from "readline";
import { CodebaseDaemon } from "./daemon";

const cm = createContextManager();
const program = new Command();

function parseFuzziness(val?: string): "auto" | 0 | 1 | 2 | undefined {
  if (val === undefined) return undefined;
  if (val === "auto") return "auto";
  const n = parseInt(val);
  if (n === 0 || n === 1 || n === 2) return n;
  return "auto";
}

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
  .option("-P, --project <project>", "Project namespace")
  .option("-c, --category <cat>", "Category", "observation")
  .option("-o, --owner <owner>", "Owner: user | agent | system", "agent")
  .option("-i, --importance <n>", "Importance 1-10", "5")
  .action(async (content, opts) => {
    try {
      const result = await cm.addMemory(content, opts.category, opts.owner, parseInt(opts.importance), opts.project, {}, true);
      console.log(JSON.stringify(result, null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

memCmd
  .command("add <content>")
  .description("Force-create a new memory (skips LLM dedup check)")
  .option("-P, --project <project>", "Project namespace")
  .option("-c, --category <cat>", "Category", "observation")
  .option("-o, --owner <owner>", "Owner: user | agent | system", "agent")
  .option("-i, --importance <n>", "Importance 1-10", "1")
  .action(async (content, opts) => {
    try {
      const result = await cm.addMemory(content, opts.category, opts.owner, parseInt(opts.importance), opts.project, {}, false);
      console.log(JSON.stringify(result, null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

memCmd
  .command("search <query>")
  .description("Search memories with hybrid kNN + BM25 scoring")
  .option("-P, --project <project>", "Filter by project")
  .option("-k, --topK <n>", "Results to return", "10")
  .option("-t, --threshold <n>", "Max cosine distance (0-2)")
  .option("--owner <owner>", "Filter by owner")
  .option("--category <cat>", "Filter by category")
  .option("--minImportance <n>", "Min importance score")
  .option("--maxAgeDays <n>", "Max age in days")
  .option("--fuzziness <f>", "Typo tolerance: auto, 0, 1, 2")
  .option("--phraseBoost <n>", "Boost for exact phrase matches (0=off)")
  .option("--minScore <n>", "Hard minimum score cutoff")
  .option("--highlight", "Show highlighted match snippets")
  .action(async (query, opts) => {
    try {
      const results = await cm.searchMemories(query, {
        topK: parseInt(opts.topK),
        threshold: opts.threshold !== undefined ? parseFloat(opts.threshold) : undefined,
        project: opts.project,
        owner: opts.owner,
        category: opts.category,
        minImportance: opts.minImportance !== undefined ? parseInt(opts.minImportance) : undefined,
        maxAgeDays: opts.maxAgeDays !== undefined ? parseInt(opts.maxAgeDays) : undefined,
        fuzziness: parseFuzziness(opts.fuzziness),
        phraseBoost: opts.phraseBoost !== undefined ? parseFloat(opts.phraseBoost) : undefined,
        minScore: opts.minScore !== undefined ? parseFloat(opts.minScore) : undefined,
        highlight: opts.highlight ?? false,
      });
      console.log(JSON.stringify(results, null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

memCmd
  .command("update <id>")
  .description("Update a memory's content or importance")
  .option("-P, --project <project>", "New project assignment")
  .option("--content <text>", "New content")
  .option("-i, --importance <n>", "New importance 1-10")
  .action(async (id, opts) => {
    try {
      const result = await cm.updateMemory(id, {
        content: opts.content,
        project: opts.project,
        importance: opts.importance !== undefined ? parseInt(opts.importance) : undefined,
      });
      console.log(JSON.stringify(result, null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

memCmd
  .command("list")
  .description("List all memories (most recently updated first)")
  .option("-P, --project <project>", "Filter by project")
  .option("-l, --limit <n>", "Max results", "100")
  .action(async (opts) => {
    try {
      console.log(JSON.stringify(await cm.listMemories({ project: opts.project }, parseInt(opts.limit)), null, 2));
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
  .option("-P, --project <project>", "Project namespace")
  .action(async (name, description, opts) => {
    try {
      console.log(JSON.stringify(await cm.addSkill(name, description, opts.project), null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

skillCmd
  .command("search <query>")
  .description("Search skills with hybrid kNN + BM25 scoring")
  .option("-P, --project <project>", "Filter by project")
  .option("-k, --topK <n>", "Results to return", "10")
  .option("-t, --threshold <n>", "Max cosine distance (0-2)")
  .option("--maxAgeDays <n>", "Max age in days")
  .option("--fuzziness <f>", "Typo tolerance: auto, 0, 1, 2")
  .option("--phraseBoost <n>", "Boost for exact phrase matches (0=off)")
  .option("--minScore <n>", "Hard minimum score cutoff")
  .option("--highlight", "Show highlighted match snippets")
  .action(async (query, opts) => {
    try {
      const results = await cm.searchSkills(query, {
        topK: parseInt(opts.topK),
        threshold: opts.threshold !== undefined ? parseFloat(opts.threshold) : undefined,
        maxAgeDays: opts.maxAgeDays !== undefined ? parseInt(opts.maxAgeDays) : undefined,
        fuzziness: parseFuzziness(opts.fuzziness),
        phraseBoost: opts.phraseBoost !== undefined ? parseFloat(opts.phraseBoost) : undefined,
        minScore: opts.minScore !== undefined ? parseFloat(opts.minScore) : undefined,
        highlight: opts.highlight ?? false,
      });
      console.log(JSON.stringify(results, null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

skillCmd
  .command("list")
  .description("List all skills")
  .option("-P, --project <project>", "Filter by project")
  .option("-l, --limit <n>", "Max results", "100")
  .action(async (opts) => {
    try {
      console.log(JSON.stringify(await cm.listSkills({ project: opts.project }, parseInt(opts.limit)), null, 2));
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
  .option("-P, --project <project>", "Project namespace")
  .option("-o, --overview <text>", "L1 overview content")
  .option("-c, --content <text>", "L2 detailed content")
  .option("-p, --parent <uri>", "Parent node URI")
  .action(async (uri, name, abstract, opts) => {
    try {
      const result = await cm.addContextNode(uri, name, abstract, opts.overview, opts.content, opts.parent || null, opts.project, {}, true);
      console.log(JSON.stringify(result, null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

nodeCmd
  .command("add <uri> <name> <abstract>")
  .description("Force-create a context node (skips LLM dedup check)")
  .option("-P, --project <project>", "Project namespace")
  .option("-o, --overview <text>", "L1 overview content")
  .option("-c, --content <text>", "L2 detailed content")
  .option("-p, --parent <uri>", "Parent node URI")
  .action(async (uri, name, abstract, opts) => {
    try {
      const result = await cm.addContextNode(uri, name, abstract, opts.overview, opts.content, opts.parent || null, opts.project, {}, false);
      console.log(JSON.stringify(result, null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

nodeCmd
  .command("search <query>")
  .description("Search context nodes (searches name, abstract, overview, content)")
  .option("-P, --project <project>", "Filter by project")
  .option("-k, --topK <n>", "Results to return", "10")
  .option("-t, --threshold <n>", "Max cosine distance (0-2)")
  .option("--parentUri <uri>", "Filter by parent URI")
  .option("--maxAgeDays <n>", "Max age in days")
  .option("--fuzziness <f>", "Typo tolerance: auto, 0, 1, 2")
  .option("--phraseBoost <n>", "Boost for exact phrase matches (0=off)")
  .option("--minScore <n>", "Hard minimum score cutoff")
  .option("--highlight", "Show highlighted match snippets")
  .action(async (query, opts) => {
    try {
      const results = await cm.searchContext(query, {
        topK: parseInt(opts.topK),
        threshold: opts.threshold !== undefined ? parseFloat(opts.threshold) : undefined,
        parentUri: opts.parentUri,
        maxAgeDays: opts.maxAgeDays !== undefined ? parseInt(opts.maxAgeDays) : undefined,
        fuzziness: parseFuzziness(opts.fuzziness),
        phraseBoost: opts.phraseBoost !== undefined ? parseFloat(opts.phraseBoost) : undefined,
        minScore: opts.minScore !== undefined ? parseFloat(opts.minScore) : undefined,
        highlight: opts.highlight ?? false,
      });
      console.log(JSON.stringify(results, null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

nodeCmd
  .command("update <uri>")
  .description("Update a context node")
  .option("-P, --project <project>", "New project assignment")
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
  .option("-P, --project <project>", "Filter by project")
  .option("-p, --parent <uri>", "Filter by parent URI")
  .option("-l, --limit <n>", "Max results", "100")
  .action(async (opts) => {
    try {
      console.log(JSON.stringify(await cm.listContextNodes(opts.parent, { project: opts.project }, parseInt(opts.limit)), null, 2));
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
  .command("restore <uri>")
  .description("Restore a soft-deleted context node (cascades to descendants)")
  .action(async (uri) => {
    try {
      await cm.restoreContextNode(uri);
      console.log(`Restored context node: ${uri}`);
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
  .option("-P, --project <project>", "Filter by project")
  .action(async (uri, opts) => {
    try {
      console.log(JSON.stringify(await cm.listContextNodes(uri, { project: opts.project }), null, 2));
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

nodeCmd
  .command("read <uri>")
  .description("Read full content of a context node")
  .action(async (uri) => {
    try {
      const subtree = await cm.getContextSubtree(uri);
      const node = subtree.find((r: { uri: unknown }) => r.uri === uri);
      console.log(node ? JSON.stringify(node, null, 2) : "Not found.");
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

// ─────────────────────────────────────────────────────────────────────────────
// Ingest (file or free text → LLM parse → review → persist)
// ─────────────────────────────────────────────────────────────────────────────

function prompt(rl: readline.Interface, question: string): Promise<string> {
  return new Promise((resolve) => rl.question(question, resolve));
}

program
  .command("ingest [file]")
  .description("Parse an MD file or free text via LLM into context nodes, review, then persist")
  .option("-P, --project <project>", "Project namespace for ingested nodes")
  .option("--text <text>", "Free text to ingest (alternative to file argument)")
  .option("--base-uri <uri>", "Base URI namespace for generated nodes", "contextfs://ingested")
  .option("-y, --yes", "Skip interactive review and persist all proposed nodes")
  .option("--no-router", "Skip LLM dedup router when persisting nodes")
  .action(async (file, opts) => {
    try {
      // ── 1. Read input ──────────────────────────────────────────────────────
      let text: string;
      if (file) {
        try {
          text = fs.readFileSync(file, "utf-8");
        } catch (err: unknown) {
          console.error((err as NodeJS.ErrnoException).code === "ENOENT" ? `File not found: ${file}` : `Cannot read file: ${err instanceof Error ? err.message : String(err)}`);
          process.exit(1);
        }
        console.log(`\nRead ${text!.length} characters from ${file}`);
      } else if (opts.text) {
        text = opts.text;
      } else {
        console.error("Provide a file path or --text <text>");
        process.exit(1);
        return; // unreachable, but narrows `text` for TypeScript
      }

      // ── 2. LLM parse ──────────────────────────────────────────────────────
      console.log("\nParsing into context nodes via LLM...");
      const proposed = await cm.parseIngestText(text!, opts.baseUri);
      console.log(`\nProposed ${proposed.length} context node(s):\n`);

      for (const [i, n] of proposed.entries()) {
        console.log(`─── Node ${i + 1}/${proposed.length} ──────────────────────────────────`);
        console.log(`  URI:      ${n.uri}`);
        console.log(`  Name:     ${n.name}`);
        console.log(`  Parent:   ${n.parent_uri ?? "(root)"}`);
        console.log(`  Abstract: ${n.abstract}`);
        if (n.overview) console.log(`  Overview: ${n.overview.length > 120 ? n.overview.slice(0, 120) + "…" : n.overview}`);
        if (n.content)  console.log(`  Content:  ${n.content.length > 80 ? n.content.slice(0, 80) + "…" : n.content}`);
        console.log();
      }

      // ── 3. Review step ────────────────────────────────────────────────────
      let approved: typeof proposed;

      if (opts.yes) {
        approved = proposed;
        console.log("--yes flag set: accepting all nodes.");
      } else {
        approved = [];
        const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
        console.log("Review each proposed node. Keys: [y] accept  [n] skip  [a] accept all  [q] quit\n");

        for (const [i, n] of proposed.entries()) {
          const answer = await prompt(
            rl,
            `[${i + 1}/${proposed.length}] "${n.name}" (${n.uri}) — accept? [y/n/a/q] `
          );

          const key = answer.trim().toLowerCase();
          if (key === "a") {
            approved.push(...proposed.slice(i));
            break;
          } else if (key === "y" || key === "") {
            approved.push(n);
          } else if (key === "q") {
            console.log("Aborted.");
            rl.close();
            process.exit(0);
          }
          // 'n' → skip
        }

        rl.close();
      }

      if (approved.length === 0) {
        console.log("\nNo nodes approved. Nothing persisted.");
        process.exit(0);
      }

      // ── 4. Persist (parallel) ─────────────────────────────────────────────
      const useRouter = opts.router as boolean;
      console.log(`\nPersisting ${approved.length} node(s) (router: ${useRouter})...\n`);

      const results = await Promise.all(
        approved.map((n) =>
          cm.addContextNode(n.uri, n.name, n.abstract, n.overview, n.content, n.parent_uri, opts.project, {}, useRouter)
            .then((result) => ({ n, result }))
        )
      );

      for (const { n, result } of results) {
        if ("skipped" in result) {
          console.log(`  SKIP   ${n.uri} — ${(result as SkippedWrite).reason}`);
        } else if ("updated" in result) {
          console.log(`  UPDATE ${(result as UpdatedWrite).id}`);
        } else {
          console.log(`  CREATE ${(result as AgentContextNode).uri}`);
        }
      }

      console.log("\nDone.");
    } catch (e) {
      console.error("Error:", e);
      process.exit(1);
    }
  });

// ─────────────────────────────────────────────────────────────────────────────
// Vibe Query & Mutation (LLM-driven free-text interface)
// ─────────────────────────────────────────────────────────────────────────────

program
  .command("vibe-query <prompt>")
  .description("Search across all stores using free-text (LLM plans and executes searches)")
  .option("-P, --project <project>", "Project namespace")
  .option("-k, --topK <n>", "Results per search query", "5")
  .action(async (userPrompt, opts) => {
    try {
      console.log("\nPlanning search queries...\n");
      const result = await executeVibeQuery(cm, userPrompt, opts.project, parseInt(opts.topK));

      console.log(`Strategy: ${result.reasoning}\n`);

      for (const group of result.results) {
        console.log(`━━━ ${group.store.toUpperCase()} ━━━  query: "${group.query}"`);
        if (group.items.length === 0) {
          console.log("  (no results)\n");
          continue;
        }
        for (const item of group.items) {
          const id = item.id || item.uri;
          const score = item._score !== undefined
            ? ` (score: ${item._score.toFixed(3)})`
            : "";
          switch (group.store) {
            case "memory":
              console.log(`  [${id}]${score}`);
              console.log(`    ${item.content}`);
              console.log(`    category=${item.category}  owner=${item.owner}  importance=${item.importance}`);
              break;
            case "skill":
              console.log(`  [${id}]${score}`);
              console.log(`    ${item.name}: ${item.description}`);
              break;
            case "node":
              console.log(`  [${item.uri}]${score}`);
              console.log(`    ${item.name}: ${item.abstract}`);
              break;
          }
        }
        console.log();
      }
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

program
  .command("vibe-mutation [prompt]")
  .description("Plan and apply mutations from free-text (LLM plans changes, you approve)")
  .option("-f, --file <path>", "Read prompt/content from a file")
  .option("-P, --project <project>", "Project namespace")
  .option("-k, --topK <n>", "Context search depth", "10")
  .option("-y, --yes", "Skip interactive review and apply all operations")
  .action(async (userPrompt, opts) => {
    try {
      let finalPrompt = userPrompt || "";
      if (opts.file) {
        const fileContent = fs.readFileSync(opts.file, "utf8");
        finalPrompt = finalPrompt ? `${finalPrompt}\n\n${fileContent}` : fileContent;
      }
      
      if (!finalPrompt.trim()) {
        console.error("Error: Must provide a prompt argument or use --file <path>");
        process.exit(1);
      }

      console.log("\nAnalyzing existing data and planning mutations...\n");
      const plan = await planVibeMutation(cm, finalPrompt, opts.project, parseInt(opts.topK));

      if (plan.operations.length === 0) {
        console.log("No mutations planned. The LLM determined no changes are needed.");
        process.exit(0);
      }

      console.log(`Reasoning: ${plan.reasoning}\n`);
      console.log(`Proposed ${plan.operations.length} mutation(s):\n`);

      // Display diff-style preview
      for (const [i, op] of plan.operations.entries()) {
        const prefix = op.op.startsWith("create") ? "\x1b[32m+" : op.op.startsWith("delete") ? "\x1b[31m-" : "\x1b[33m~";
        const reset = "\x1b[0m";
        console.log(`${prefix}── Operation ${i + 1}/${plan.operations.length} ──${reset}`);
        console.log(`  ${prefix}${op.op}${reset}  ${op.target ? `target: ${op.target}` : ""}`);
        console.log(`  ${op.description}`);

        if (op.op.startsWith("create") || op.op.startsWith("update")) {
          for (const [key, value] of Object.entries(op.data)) {
            const val = typeof value === "string" && value.length > 120 ? value.slice(0, 120) + "..." : value;
            const symbol = op.op.startsWith("create") ? "+" : "~";
            console.log(`  ${prefix}${symbol} ${key}: ${val}${reset}`);
          }
        }
        console.log();
      }

      // Interactive approval
      let approved: VibeMutationOp[];

      if (opts.yes) {
        approved = plan.operations;
        console.log("--yes flag set: applying all operations.\n");
      } else {
        approved = [];
        const rl = readline.createInterface({ input: process.stdin, output: process.stdout });
        console.log("Review each operation. Keys: [y] accept  [n] skip  [a] accept all  [q] quit\n");

        for (const [i, op] of plan.operations.entries()) {
          const answer = await prompt(rl,
            `[${i + 1}/${plan.operations.length}] ${op.op} — "${op.description}" — accept? [y/n/a/q] `
          );

          const key = answer.trim().toLowerCase();
          if (key === "a") {
            approved.push(...plan.operations.slice(i));
            break;
          } else if (key === "y" || key === "") {
            approved.push(op);
          } else if (key === "q") {
            console.log("Aborted.");
            rl.close();
            process.exit(0);
          }
          // 'n' → skip
        }
        rl.close();
      }

      if (approved.length === 0) {
        console.log("\nNo operations approved. Nothing changed.");
        process.exit(0);
      }

      // Execute approved operations
      console.log(`\nExecuting ${approved.length} operation(s)...\n`);
      for (const op of approved) {
        try {
          const result = await executeMutationOp(cm, op, opts.project);
          console.log(`  \x1b[32m✓\x1b[0m ${result}`);
        } catch (err) {
          console.error(`  \x1b[31m✗\x1b[0m ${op.op} failed: ${err instanceof Error ? err.message : String(err)}`);
        }
      }

      console.log("\nDone.");
    } catch (e) { console.error("Error:", e); process.exit(1); }
  });

// ─────────────────────────────────────────────────────────────────────────────
// Daemon
// ─────────────────────────────────────────────────────────────────────────────

program
  .command("daemon <dir>")
  .description("Start a background daemon watching a directory to extract AST context nodes automatically")
  .requiredOption("-P, --project <project>", "Project namespace")
  .action(async (dir, opts) => {
    try {
      const daemon = new CodebaseDaemon(cm, opts.project, dir);
      await daemon.start();
      
      process.on("SIGINT", async () => {
        console.log("Shutting down daemon...");
        await daemon.stop();
        process.exit(0);
      });
      process.on("SIGTERM", async () => {
        console.log("Shutting down daemon...");
        await daemon.stop();
        process.exit(0);
      });
    } catch (e) {
      console.error("Daemon error:", e);
      process.exit(1);
    }
  });

program.parse(process.argv);
