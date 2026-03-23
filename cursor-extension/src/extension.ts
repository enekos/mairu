import * as vscode from "vscode";
import {
  buildSearchArgs,
  buildVibeMutationPreviewArgs,
  buildVibeQueryArgs,
  parseCliOutput,
  type QueryKind,
} from "./core";
import { runContextCli, CliRunError } from "./cliRunner";
import { resolveDefaultProject } from "./project";
import { ContextFsTreeProvider, type ResultRow } from "./view";

const PROJECT_KEY = "contextfs.project";

function getConfig(): { cliPath: string; topK: number; timeoutMs: number } {
  const cfg = vscode.workspace.getConfiguration("contextfs");
  return {
    cliPath: cfg.get<string>("cliPath", "context-cli"),
    topK: cfg.get<number>("defaultTopK", 5),
    timeoutMs: cfg.get<number>("commandTimeoutMs", 30_000),
  };
}

function compact(value: unknown, max = 140): string {
  const text = typeof value === "string" ? value : JSON.stringify(value);
  if (text.length <= max) return text;
  return `${text.slice(0, max - 1)}...`;
}

function toRows(kind: QueryKind, parsed: ReturnType<typeof parseCliOutput>): ResultRow[] {
  if (parsed.kind === "text") {
    return parsed.value
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter(Boolean)
      .slice(0, 40)
      .map((line) => ({ label: line }));
  }

  const data = parsed.value;
  if (!Array.isArray(data)) {
    if (!data || typeof data !== "object") {
      return [{ label: compact(data) }];
    }
    return Object.entries(data)
      .slice(0, 25)
      .map(([k, v]) => ({ label: k, description: compact(v, 100) }));
  }

  const rows: ResultRow[] = data.slice(0, 25).map((item: Record<string, unknown>) => {
    if (kind === "memory_search") {
      return {
        label: compact(item.content ?? "(empty memory)"),
        description: `score ${compact(item._score ?? "n/a", 16)}`,
      };
    }
    if (kind === "node_search") {
      return {
        label: compact(item.name ?? item.uri ?? "(unnamed node)", 90),
        description: compact(item.uri ?? item.abstract ?? "", 90),
      };
    }
    if (kind === "skill_search") {
      return {
        label: compact(item.name ?? "(unnamed skill)", 90),
        description: compact(item.description ?? "", 90),
      };
    }
    return {
      label: compact(item.name ?? item.id ?? item.uri ?? "result", 90),
      description: compact(item.content ?? item.description ?? "", 90),
    };
  });

  if (data.length > rows.length) {
    rows.push({ label: `... ${data.length - rows.length} more result(s)` });
  }
  return rows;
}

async function askProject(current: string): Promise<string | undefined> {
  const entered = await vscode.window.showInputBox({
    title: "ContextFS Project",
    prompt: "Project passed to context-cli via -P/--project",
    value: current,
    ignoreFocusOut: true,
  });
  const trimmed = entered?.trim();
  return trimmed || undefined;
}

async function askPrompt(title: string, prompt: string): Promise<string | undefined> {
  const entered = await vscode.window.showInputBox({
    title,
    prompt,
    ignoreFocusOut: true,
  });
  const trimmed = entered?.trim();
  return trimmed || undefined;
}

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  const output = vscode.window.createOutputChannel("ContextFS");
  const workspacePath = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
  const saved = context.workspaceState.get<string>(PROJECT_KEY);
  const project = saved ?? resolveDefaultProject(workspacePath);
  const provider = new ContextFsTreeProvider(project);

  context.subscriptions.push(output);
  context.subscriptions.push(vscode.window.registerTreeDataProvider("contextfs.queriesView", provider));

  async function runQuery(
    kind: QueryKind,
    title: string,
    args: string[],
    stdinText?: string
  ): Promise<void> {
    const cfg = getConfig();
    output.appendLine(`[ContextFS] Running: ${cfg.cliPath} ${args.join(" ")}`);
    output.show(true);

    try {
      provider.clearError();
      const result = await runContextCli({
        cliPath: cfg.cliPath,
        args,
        timeoutMs: cfg.timeoutMs,
        stdinText,
      });
      const parsed = parseCliOutput(result.stdout);
      const rows = toRows(kind, parsed);
      provider.setResult({
        kind,
        title,
        commandString: result.commandString,
        rawOutput: result.stdout,
        rows,
      });
      output.appendLine(result.stdout.trim() || "(empty stdout)");
      if (result.stderr.trim()) {
        output.appendLine(`stderr: ${result.stderr.trim()}`);
      }
      vscode.window.showInformationMessage(`ContextFS: ${title} finished (${rows.length} rows).`);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      provider.setError(message);
      output.appendLine(`ERROR: ${message}`);
      if (err instanceof CliRunError && err.stderr) {
        output.appendLine(err.stderr);
      }
      vscode.window.showErrorMessage(
        `ContextFS query failed: ${message}. Check "ContextFS" output channel.`
      );
    }
  }

  context.subscriptions.push(
    vscode.commands.registerCommand("contextfs.refresh", () => provider.refresh())
  );

  context.subscriptions.push(
    vscode.commands.registerCommand("contextfs.setProject", async () => {
      const next = await askProject(provider.getProject());
      if (!next) return;
      provider.setProject(next);
      await context.workspaceState.update(PROJECT_KEY, next);
      vscode.window.showInformationMessage(`ContextFS project set to "${next}".`);
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand("contextfs.runMemorySearch", async () => {
      const query = await askPrompt("Memory Search", "Enter search query");
      if (!query) return;
      const cfg = getConfig();
      const args = buildSearchArgs({
        kind: "memory_search",
        query,
        project: provider.getProject(),
        topK: cfg.topK,
      });
      await runQuery("memory_search", "Memory Search", args);
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand("contextfs.runNodeSearch", async () => {
      const query = await askPrompt("Node Search", "Enter search query");
      if (!query) return;
      const cfg = getConfig();
      const args = buildSearchArgs({
        kind: "node_search",
        query,
        project: provider.getProject(),
        topK: cfg.topK,
      });
      await runQuery("node_search", "Node Search", args);
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand("contextfs.runSkillSearch", async () => {
      const query = await askPrompt("Skill Search", "Enter search query");
      if (!query) return;
      const cfg = getConfig();
      const args = buildSearchArgs({
        kind: "skill_search",
        query,
        project: provider.getProject(),
        topK: cfg.topK,
      });
      await runQuery("skill_search", "Skill Search", args);
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand("contextfs.runVibeQuery", async () => {
      const prompt = await askPrompt("Vibe Query", "Ask a natural-language retrieval question");
      if (!prompt) return;
      const cfg = getConfig();
      const args = buildVibeQueryArgs({
        prompt,
        project: provider.getProject(),
        topK: cfg.topK,
      });
      await runQuery("vibe_query", "Vibe Query", args);
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand("contextfs.runVibeMutationPlan", async () => {
      const prompt = await askPrompt(
        "Vibe Mutation Plan (Preview)",
        "Describe the mutation you want to preview"
      );
      if (!prompt) return;
      const cfg = getConfig();
      const args = buildVibeMutationPreviewArgs({
        prompt,
        project: provider.getProject(),
        topK: cfg.topK,
      });
      // send "q" immediately so no mutation is executed
      await runQuery("vibe_mutation_plan", "Vibe Mutation Plan", args, "q\n");
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand("contextfs.copyLastCommand", async () => {
      const last = provider.getLastCommand();
      if (!last) {
        vscode.window.showWarningMessage("No ContextFS command has run yet.");
        return;
      }
      await vscode.env.clipboard.writeText(last);
      vscode.window.showInformationMessage("Copied last ContextFS command.");
    })
  );
}

export function deactivate(): void {
  // no-op
}
