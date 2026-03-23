import * as vscode from "vscode";
import type { QueryKind } from "./core";

export interface ResultRow {
  label: string;
  description?: string;
  tooltip?: string;
}

export interface QueryResult {
  kind: QueryKind;
  title: string;
  commandString: string;
  rawOutput: string;
  rows: ResultRow[];
}

type Node =
  | { type: "project" }
  | { type: "action"; id: QueryKind }
  | { type: "resultGroup" }
  | { type: "resultRow"; row: ResultRow };

export class ContextFsTreeItem extends vscode.TreeItem {
  constructor(
    public readonly node: Node,
    label: string,
    collapsibleState: vscode.TreeItemCollapsibleState
  ) {
    super(label, collapsibleState);
  }
}

const QUERY_LABELS: Record<QueryKind, string> = {
  memory_search: "Memory Search",
  node_search: "Node Search",
  skill_search: "Skill Search",
  vibe_query: "Vibe Query",
  vibe_mutation_plan: "Vibe Mutation Plan",
};

const QUERY_COMMANDS: Record<QueryKind, string> = {
  memory_search: "contextfs.runMemorySearch",
  node_search: "contextfs.runNodeSearch",
  skill_search: "contextfs.runSkillSearch",
  vibe_query: "contextfs.runVibeQuery",
  vibe_mutation_plan: "contextfs.runVibeMutationPlan",
};

export class ContextFsTreeProvider implements vscode.TreeDataProvider<ContextFsTreeItem> {
  private readonly onDidChangeTreeDataEmitter = new vscode.EventEmitter<ContextFsTreeItem | undefined>();
  readonly onDidChangeTreeData = this.onDidChangeTreeDataEmitter.event;

  private project: string;
  private latestResult: QueryResult | null = null;
  private lastError: string | null = null;

  constructor(project: string) {
    this.project = project;
  }

  public getProject(): string {
    return this.project;
  }

  public setProject(next: string): void {
    this.project = next;
    this.refresh();
  }

  public setResult(result: QueryResult): void {
    this.latestResult = result;
    this.lastError = null;
    this.refresh();
  }

  public setError(message: string): void {
    this.lastError = message;
    this.refresh();
  }

  public clearError(): void {
    this.lastError = null;
    this.refresh();
  }

  public getLastCommand(): string | null {
    return this.latestResult?.commandString ?? null;
  }

  public refresh(): void {
    this.onDidChangeTreeDataEmitter.fire(undefined);
  }

  public getTreeItem(element: ContextFsTreeItem): vscode.TreeItem {
    return element;
  }

  public getChildren(element?: ContextFsTreeItem): Thenable<ContextFsTreeItem[]> {
    if (!element) {
      const roots: ContextFsTreeItem[] = [];
      const projectNode = new ContextFsTreeItem(
        { type: "project" },
        `Project: ${this.project}`,
        vscode.TreeItemCollapsibleState.None
      );
      projectNode.command = { command: "contextfs.setProject", title: "Set Project" };
      projectNode.iconPath = new vscode.ThemeIcon("folder");
      roots.push(projectNode);

      for (const id of Object.keys(QUERY_LABELS) as QueryKind[]) {
        const item = new ContextFsTreeItem(
          { type: "action", id },
          QUERY_LABELS[id],
          vscode.TreeItemCollapsibleState.None
        );
        item.command = { command: QUERY_COMMANDS[id], title: QUERY_LABELS[id] };
        item.iconPath = new vscode.ThemeIcon("search");
        roots.push(item);
      }

      if (this.latestResult) {
        const group = new ContextFsTreeItem(
          { type: "resultGroup" },
          `Latest: ${this.latestResult.title}`,
          vscode.TreeItemCollapsibleState.Expanded
        );
        group.description = `${this.latestResult.rows.length} rows`;
        group.iconPath = new vscode.ThemeIcon("list-unordered");
        roots.push(group);
      }

      if (this.lastError) {
        const err = new ContextFsTreeItem(
          { type: "resultRow", row: { label: this.lastError } },
          `Error: ${this.lastError}`,
          vscode.TreeItemCollapsibleState.None
        );
        err.iconPath = new vscode.ThemeIcon("error");
        roots.push(err);
      }

      return Promise.resolve(roots);
    }

    if (element.node.type === "resultGroup" && this.latestResult) {
      const rows = this.latestResult.rows.map((row) => {
        const item = new ContextFsTreeItem(
          { type: "resultRow", row },
          row.label,
          vscode.TreeItemCollapsibleState.None
        );
        item.description = row.description;
        item.tooltip = row.tooltip;
        item.iconPath = new vscode.ThemeIcon("symbol-string");
        return item;
      });
      return Promise.resolve(rows);
    }

    return Promise.resolve([]);
  }
}
