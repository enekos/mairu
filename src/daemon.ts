import { ContextManager } from "./contextManager";
import * as path from "path";
import * as chokidar from "chokidar";
import * as fs from "fs";
import { createHash } from "crypto";
import { TypeScriptDescriber } from "./typescriptDescriber";
import { enrichDescriptions } from "./nlEnricher";
import type { LogicSymbol, LogicEdge, LogicSymbolKind } from "./languageDescriber";

export interface DaemonOptions {
  maxFileSizeBytes?: number;
  processingDebounceMs?: number;
  concurrency?: number;
}

const DEFAULT_MAX_FILE_SIZE_BYTES = 512 * 1024; // 512 KB
const DEFAULT_PROCESSING_DEBOUNCE_MS = 200;
const DEFAULT_CONCURRENCY = 8;
const SUPPORTED_EXTENSIONS = new Set([".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"]);
const IGNORED_PATH_SEGMENTS = new Set(["node_modules", "dist", "build"]);

interface SourceSummary {
  abstractText: string;   // NL file summary
  overviewText: string;   // compact graph (was previously compactContent)
  nlContent: string;      // full NL descriptions
  logicGraphMetadata: Record<string, unknown>;
}

interface SelectedLogicGraph {
  symbols: LogicSymbol[];
  edges: LogicEdge[];
  totalSymbols: number;
  totalEdges: number;
  truncatedSymbols: number;
  truncatedEdges: number;
  exportedCount: number;
  internalCount: number;
}

const CACHE_FILENAME = ".contextfs-cache.json";
const CACHE_VERSION = 1;

const LOGIC_GRAPH_VERSION = 1;
const MAX_SERIALIZED_SYMBOLS = 80;
const MAX_SERIALIZED_EDGES = 220;
const MAX_EDGES_PER_SOURCE_SYMBOL = 8;
const MAX_CONTENT_CHARS = 16_000;
const MAX_OVERVIEW_EDGE_LINES = 8;

const KIND_SORT_ORDER: Record<LogicSymbolKind, number> = {
  cls: 0,
  fn: 1,
  mtd: 2,
  var: 3,
  iface: 4,
  enum: 5,
  type: 6,
};

export class CodebaseDaemon {
  private manager: ContextManager;
  private project: string;
  private watchDir: string;
  private readonly describer: TypeScriptDescriber;
  private watcher: chokidar.FSWatcher | null = null;
  private isProcessing = false;
  private pendingFiles: Set<string> = new Set();
  private processTimer: NodeJS.Timeout | null = null;
  private readonly maxFileSizeBytes: number;
  private readonly processingDebounceMs: number;
  private readonly concurrency: number;
  private readonly fileFingerprints: Map<string, string> = new Map();
  private readonly fileContentHashes: Map<string, string> = new Map();
  private readonly nodePayloadHashes: Map<string, string> = new Map();

  constructor(manager: ContextManager, project: string, watchDir: string, options: DaemonOptions = {}) {
    this.manager = manager;
    this.project = project;
    this.watchDir = path.resolve(watchDir);
    this.maxFileSizeBytes = options.maxFileSizeBytes ?? DEFAULT_MAX_FILE_SIZE_BYTES;
    this.processingDebounceMs = options.processingDebounceMs ?? DEFAULT_PROCESSING_DEBOUNCE_MS;
    this.concurrency = options.concurrency ?? DEFAULT_CONCURRENCY;
    this.describer = new TypeScriptDescriber();
  }

  public loadCache(): void {
    const cachePath = path.join(this.watchDir, CACHE_FILENAME);
    try {
      const raw = fs.readFileSync(cachePath, "utf8");
      const data = JSON.parse(raw);
      if (data.version !== CACHE_VERSION) {
        console.log("[Daemon] Cache version mismatch, ignoring");
        return;
      }
      for (const [absPath, entry] of Object.entries(data.files as Record<string, any>)) {
        this.fileFingerprints.set(absPath, entry.fingerprint);
        this.fileContentHashes.set(absPath, entry.contentHash);
        this.nodePayloadHashes.set(absPath, entry.payloadHash);
      }
      console.log(`[Daemon] Loaded cache with ${Object.keys(data.files).length} entries`);
    } catch {
      // No cache or corrupt — start fresh
    }
  }

  public saveCache(): void {
    const cachePath = path.join(this.watchDir, CACHE_FILENAME);
    const tmpPath = cachePath + ".tmp";
    const files: Record<string, any> = {};
    for (const [absPath, fingerprint] of this.fileFingerprints) {
      files[absPath] = {
        fingerprint,
        contentHash: this.fileContentHashes.get(absPath) ?? "",
        payloadHash: this.nodePayloadHashes.get(absPath) ?? "",
      };
    }
    const data = JSON.stringify({ version: CACHE_VERSION, files }, null, 2);
    fs.writeFileSync(tmpPath, data, "utf8");
    fs.renameSync(tmpPath, cachePath);
  }

  public async start() {
    console.log(`[Daemon] Starting codebase monitor for project '${this.project}' in ${this.watchDir}`);

    // Load persistent cache before initial scan
    this.loadCache();

    // Initial scan
    await this.processAllFiles();

    // Watch for changes
    this.watcher = chokidar.watch(`${this.watchDir}/**/*.{ts,tsx,js,jsx,mjs,cjs}`, {
      ignored: /(^|[\/\\])\..|node_modules|dist|build/, // ignore dotfiles and generated folders
      persistent: true,
      ignoreInitial: true,
      awaitWriteFinish: {
        stabilityThreshold: 150,
        pollInterval: 50,
      },
    });

    this.watcher
      .on("add", (p) => this.queueFile(p))
      .on("change", (p) => this.queueFile(p))
      .on("unlink", (p) => this.handleFileDelete(p));

    console.log(`[Daemon] Watching for changes...`);
  }

  public async stop() {
    if (this.watcher) {
      await this.watcher.close();
      this.watcher = null;
    }
    if (this.processTimer) {
      clearTimeout(this.processTimer);
      this.processTimer = null;
    }
    if (this.isProcessing || this.pendingFiles.size > 0) {
      await this.processPendingFiles();
    }
    console.log(`[Daemon] Stopped watching ${this.watchDir}`);
  }

  private discoverSourceFiles(dir: string): string[] {
    const results: string[] = [];
    let entries: fs.Dirent[];
    try {
      entries = fs.readdirSync(dir, { withFileTypes: true });
    } catch {
      return results;
    }
    for (const entry of entries) {
      const fullPath = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        if (!IGNORED_PATH_SEGMENTS.has(entry.name) && !entry.name.startsWith(".")) {
          results.push(...this.discoverSourceFiles(fullPath));
        }
      } else if (SUPPORTED_EXTENSIONS.has(path.extname(entry.name).toLowerCase())) {
        results.push(fullPath);
      }
    }
    return results;
  }

  private shouldProcessFile(filePath: string): boolean {
    if (this.isIgnoredPath(filePath)) return false;
    const ext = path.extname(filePath).toLowerCase();
    return SUPPORTED_EXTENSIONS.has(ext);
  }

  private isIgnoredPath(filePath: string): boolean {
    const relPath = path.relative(this.watchDir, path.resolve(filePath));
    if (!relPath || relPath === ".") return false;
    if (relPath.startsWith("..")) return true;

    const segments = relPath.replace(/\\/g, "/").split("/");
    return segments.some((segment) => IGNORED_PATH_SEGMENTS.has(segment) || segment.startsWith("."));
  }

  private statFile(filePath: string): fs.Stats | null {
    try {
      return fs.statSync(filePath);
    } catch {
      return null;
    }
  }

  private fingerprintForStats(stats: fs.Stats): string {
    return `${stats.size}:${stats.mtimeMs}`;
  }

  private hashText(value: string): string {
    return createHash("sha1").update(value).digest("hex");
  }

  private flushPendingSoon() {
    if (this.processTimer) return;
    this.processTimer = setTimeout(() => {
      this.processTimer = null;
      void this.processPendingFiles();
    }, this.processingDebounceMs);
  }

  private summarizeSourceFile(filePath: string, sourceText: string): SourceSummary {
    const result = this.describer.extractFileGraph(filePath, sourceText);
    const rawGraph = {
      symbols: result.symbols,
      edges: result.edges,
      imports: result.imports,
    };
    const selectedGraph = this.selectGraphForSerialization(rawGraph);
    const enrichedDescriptions = enrichDescriptions(result.symbolDescriptions, result.edges);
    const abstractText = result.fileSummary;
    const overviewText = this.buildCompactContent(filePath, selectedGraph);
    const nlContent = this.buildNLContent(enrichedDescriptions, selectedGraph);
    const logicGraphMetadata = {
      version: LOGIC_GRAPH_VERSION,
      totals: {
        symbols: selectedGraph.totalSymbols,
        edges: selectedGraph.totalEdges,
        exported_symbols: selectedGraph.exportedCount,
        internal_symbols: selectedGraph.internalCount,
        truncated_symbols: selectedGraph.truncatedSymbols,
        truncated_edges: selectedGraph.truncatedEdges,
      },
      symbols: selectedGraph.symbols,
      edges: selectedGraph.edges,
      imports: rawGraph.imports,
    };
    return { abstractText, overviewText, nlContent, logicGraphMetadata };
  }

  private buildNLContent(descriptions: Map<string, string>, graph: SelectedLogicGraph): string {
    // Group symbols by kind: classes first, then functions, then vars/iface/enum/type
    const kindGroupOrder: LogicSymbolKind[] = ["cls", "fn", "mtd", "var", "iface", "enum", "type"];
    const grouped = new Map<LogicSymbolKind, LogicSymbol[]>();
    for (const symbol of graph.symbols) {
      const existing = grouped.get(symbol.kind) ?? [];
      existing.push(symbol);
      grouped.set(symbol.kind, existing);
    }

    // Rank symbols by score for truncation ordering (least important last)
    const outgoingCounts = new Map<string, number>();
    for (const edge of graph.edges) {
      outgoingCounts.set(edge.from, (outgoingCounts.get(edge.from) ?? 0) + 1);
    }
    const symbolScores = new Map<string, number>();
    for (const symbol of graph.symbols) {
      symbolScores.set(symbol.id, this.symbolScore(symbol, outgoingCounts));
    }

    // Build sections
    type Section = { symbolId: string; score: number; text: string };
    const sections: Section[] = [];

    // Collect class methods by parentId for nesting
    const methodsByClass = new Map<string, LogicSymbol[]>();
    for (const mtd of grouped.get("mtd") ?? []) {
      if (mtd.parentId) {
        const existing = methodsByClass.get(mtd.parentId) ?? [];
        existing.push(mtd);
        methodsByClass.set(mtd.parentId, existing);
      }
    }

    const kindLabel: Record<LogicSymbolKind, string> = {
      cls: "Class",
      fn: "Function",
      mtd: "Method",
      var: "Variable",
      iface: "Interface",
      enum: "Enum",
      type: "Type",
    };

    for (const kind of kindGroupOrder) {
      const symbols = grouped.get(kind) ?? [];
      for (const symbol of symbols) {
        // Skip methods here — they're nested under their class
        if (kind === "mtd") continue;

        const desc = descriptions.get(symbol.id);
        const tags: string[] = [];
        if (symbol.exported) tags.push("exported");
        if (symbol.control.async) tags.push("async");
        const tagStr = tags.length > 0 ? ` (${tags.join(", ")})` : "";

        if (kind === "cls") {
          // Class section with nested methods
          const lines: string[] = [];
          lines.push(`## ${kindLabel[kind]}: ${symbol.name}${tagStr}`);
          if (desc) lines.push(desc);

          const methods = methodsByClass.get(symbol.id) ?? [];
          for (const mtd of methods) {
            const mtdDesc = descriptions.get(mtd.id);
            const mtdTags: string[] = [];
            if (mtd.exported) mtdTags.push("exported");
            if (mtd.control.async) mtdTags.push("async");
            const mtdTagStr = mtdTags.length > 0 ? ` (${mtdTags.join(", ")})` : "";
            const paramStr = mtd.params.length > 0 ? `Parameters: ${mtd.params.join(", ")}` : "";

            lines.push("");
            lines.push(`### Method: ${mtd.name}${mtdTagStr}`);
            if (paramStr) lines.push(paramStr);
            if (mtdDesc) {
              lines.push("");
              lines.push(mtdDesc);
            }
          }

          // Use the max score among the class + its methods
          let maxScore = symbolScores.get(symbol.id) ?? 0;
          for (const mtd of methods) {
            const mtdScore = symbolScores.get(mtd.id) ?? 0;
            if (mtdScore > maxScore) maxScore = mtdScore;
          }

          sections.push({ symbolId: symbol.id, score: maxScore, text: lines.join("\n") });
        } else if (kind === "fn") {
          const lines: string[] = [];
          const paramStr = symbol.params.length > 0 ? `Parameters: ${symbol.params.join(", ")}` : "";
          lines.push(`## ${kindLabel[kind]}: ${symbol.name}${tagStr}`);
          if (paramStr) lines.push(paramStr);
          if (desc) {
            lines.push("");
            lines.push(desc);
          }
          sections.push({ symbolId: symbol.id, score: symbolScores.get(symbol.id) ?? 0, text: lines.join("\n") });
        } else {
          // var, iface, enum, type — one-line mention
          const line = `- ${kindLabel[kind]}: ${symbol.name}${tagStr}${desc ? ` — ${desc}` : ""}`;
          sections.push({ symbolId: symbol.id, score: symbolScores.get(symbol.id) ?? 0, text: line });
        }
      }
    }

    // Assemble and apply truncation
    let totalLength = 0;
    const included: string[] = [];
    let omitted = 0;

    // Sort sections by score descending so we keep the most important
    const sortedSections = [...sections].sort((a, b) => b.score - a.score);

    for (const section of sortedSections) {
      const sectionLength = section.text.length + 2; // +2 for "\n\n" separator
      if (totalLength + sectionLength > MAX_CONTENT_CHARS && included.length > 0) {
        omitted++;
        continue;
      }
      included.push(section.text);
      totalLength += sectionLength;
    }

    let result = included.join("\n\n");
    if (omitted > 0) {
      result += `\n\n...TRUNCATED: ${omitted} symbols omitted`;
    }

    return result;
  }

  private buildCompactContent(filePath: string, graph: SelectedLogicGraph): string {
    const relPath = path.relative(this.watchDir, filePath).replace(/\\/g, "/");
    const lines: string[] = [
      `File: ${relPath}`,
      `Language: ${path.extname(filePath).slice(1) || "unknown"}`,
      `LogicGraph: v${LOGIC_GRAPH_VERSION}`,
      `GraphStats: symbols=${graph.totalSymbols} shown=${graph.symbols.length} edges=${graph.totalEdges} shown=${graph.edges.length} exported=${graph.exportedCount} internal=${graph.internalCount}`,
      "",
      "Symbols:",
    ];

    if (graph.symbols.length === 0) {
      lines.push("- (none)");
    } else {
      for (const symbol of graph.symbols) {
        const params = symbol.params.length > 0 ? `(${symbol.params.join(",")})` : "()";
        const parent = symbol.parentId ? ` parent=${symbol.parentId}` : "";
        const controlTokens = [
          symbol.control.async ? "async" : "",
          symbol.control.branch ? "branch" : "",
          symbol.control.await ? "await" : "",
          symbol.control.throw ? "throw" : "",
        ].filter(Boolean);
        lines.push(
          [
            `- ${symbol.kind} ${symbol.id}`,
            `name=${symbol.name}`,
            `exp=${symbol.exported ? "1" : "0"}`,
            `params=${params}`,
            `cx=${symbol.complexity}`,
            controlTokens.length > 0 ? `ctrl=${controlTokens.join("|")}` : "",
            parent,
          ].filter(Boolean).join(" ")
        );
      }
    }

    lines.push("", "Edges:");
    if (graph.edges.length === 0) {
      lines.push("- (none)");
    } else {
      for (const edge of graph.edges) {
        lines.push(`- ${edge.kind} ${edge.from} -> ${edge.to}`);
      }
    }

    if (graph.truncatedSymbols > 0 || graph.truncatedEdges > 0) {
      lines.push("", `Truncated: symbols=${graph.truncatedSymbols}, edges=${graph.truncatedEdges}`);
    }

    const serialized = lines.join("\n");
    if (serialized.length <= MAX_CONTENT_CHARS) return serialized;
    return `${serialized.slice(0, MAX_CONTENT_CHARS)}\n...TRUNCATED_BY_MAX_CONTENT_CHARS`;
  }

  private selectGraphForSerialization(graph: { symbols: LogicSymbol[]; edges: LogicEdge[]; imports: string[] }): SelectedLogicGraph {
    const outgoingCounts = new Map<string, number>();
    for (const edge of graph.edges) {
      outgoingCounts.set(edge.from, (outgoingCounts.get(edge.from) ?? 0) + 1);
    }

    const rankedSymbols = [...graph.symbols].sort((a, b) => {
      const scoreDiff = this.symbolScore(b, outgoingCounts) - this.symbolScore(a, outgoingCounts);
      if (scoreDiff !== 0) return scoreDiff;
      return this.compareSymbols(a, b);
    });

    const selectedSymbols = rankedSymbols.slice(0, MAX_SERIALIZED_SYMBOLS);
    const selectedSymbolIds = new Set(selectedSymbols.map((symbol) => symbol.id));
    const perSourceEdgeCounts = new Map<string, number>();
    const selectedEdges: LogicEdge[] = [];

    for (const edge of graph.edges) {
      if (edge.kind !== "import") {
        if (!selectedSymbolIds.has(edge.from)) continue;
        if (!selectedSymbolIds.has(edge.to)) continue;
        const sourceCount = perSourceEdgeCounts.get(edge.from) ?? 0;
        if (sourceCount >= MAX_EDGES_PER_SOURCE_SYMBOL) continue;
        perSourceEdgeCounts.set(edge.from, sourceCount + 1);
      }
      selectedEdges.push(edge);
      if (selectedEdges.length >= MAX_SERIALIZED_EDGES) break;
    }

    const exportedCount = graph.symbols.filter((symbol) => symbol.exported).length;
    const totalSymbols = graph.symbols.length;
    const totalEdges = graph.edges.length;
    return {
      symbols: this.sortSymbols(selectedSymbols),
      edges: this.sortEdges(selectedEdges),
      totalSymbols,
      totalEdges,
      truncatedSymbols: Math.max(0, totalSymbols - selectedSymbols.length),
      truncatedEdges: Math.max(0, totalEdges - selectedEdges.length),
      exportedCount,
      internalCount: Math.max(0, totalSymbols - exportedCount),
    };
  }

  private symbolScore(symbol: LogicSymbol, outgoingCounts: Map<string, number>): number {
    const kindBoost = {
      cls: 80,
      fn: 70,
      mtd: 60,
      var: 40,
      iface: 35,
      enum: 35,
      type: 30,
    }[symbol.kind];
    const complexityBoost = symbol.complexity === "high" ? 12 : symbol.complexity === "medium" ? 6 : 0;
    const controlBoost = (symbol.control.branch ? 3 : 0)
      + (symbol.control.await ? 3 : 0)
      + (symbol.control.throw ? 2 : 0)
      + (symbol.control.async ? 2 : 0);
    const exportedBoost = symbol.exported ? 1000 : 0;
    const edgeBoost = (outgoingCounts.get(symbol.id) ?? 0) * 5;
    return exportedBoost + kindBoost + complexityBoost + controlBoost + edgeBoost;
  }

  private sortSymbols(symbols: LogicSymbol[]): LogicSymbol[] {
    return [...symbols].sort((a, b) => this.compareSymbols(a, b));
  }

  private compareSymbols(a: LogicSymbol, b: LogicSymbol): number {
    const kindDiff = KIND_SORT_ORDER[a.kind] - KIND_SORT_ORDER[b.kind];
    if (kindDiff !== 0) return kindDiff;
    const nameDiff = a.name.localeCompare(b.name);
    if (nameDiff !== 0) return nameDiff;
    const lineDiff = a.line - b.line;
    if (lineDiff !== 0) return lineDiff;
    return a.id.localeCompare(b.id);
  }

  private sortEdges(edges: LogicEdge[]): LogicEdge[] {
    return [...edges].sort((a, b) => {
      const kindDiff = a.kind.localeCompare(b.kind);
      if (kindDiff !== 0) return kindDiff;
      const fromDiff = a.from.localeCompare(b.from);
      if (fromDiff !== 0) return fromDiff;
      return a.to.localeCompare(b.to);
    });
  }

  private queueFile(filePath: string) {
    if (!this.shouldProcessFile(filePath)) return;
    this.pendingFiles.add(path.resolve(filePath));
    this.flushPendingSoon();
  }

  private async runWithConcurrency<T>(
    items: T[],
    concurrency: number,
    fn: (item: T) => Promise<void>
  ): Promise<void> {
    let index = 0;
    const workers = Array.from({ length: Math.min(concurrency, items.length) }, async () => {
      while (index < items.length) {
        const currentIndex = index++;
        await fn(items[currentIndex]);
      }
    });
    await Promise.all(workers);
  }

  private async processPendingFiles() {
    if (this.isProcessing) return;
    this.isProcessing = true;

    try {
      const filesToProcess = Array.from(this.pendingFiles);
      this.pendingFiles.clear();
      await this.runWithConcurrency(filesToProcess, this.concurrency, (f) => this.processFile(f));
      this.saveCache();
    } catch (e) {
      console.error("[Daemon] Error processing files:", e);
    } finally {
      this.isProcessing = false;
      if (this.pendingFiles.size > 0) {
        this.flushPendingSoon();
      }
    }
  }

  private async processAllFiles() {
    console.log("[Daemon] Running initial full codebase scan...");
    const files = this.discoverSourceFiles(this.watchDir).filter((f) => this.shouldProcessFile(f));
    await this.runWithConcurrency(files, this.concurrency, (f) => this.processFile(f));
    console.log(`[Daemon] Initial scan complete (${files.length} files).`);
    this.saveCache();
  }

  private async handleFileDelete(filePath: string) {
    const absPath = path.resolve(filePath);
    const uri = this.fileToUri(absPath);
    this.pendingFiles.delete(absPath);
    this.fileFingerprints.delete(absPath);
    this.fileContentHashes.delete(absPath);
    this.nodePayloadHashes.delete(absPath);

    console.log(`[Daemon] File deleted, removing context node: ${uri}`);
    try {
      await this.manager.deleteContextNode(uri);
    } catch (err) {
      console.error(`[Daemon] Failed to remove context node for ${uri}`, err);
    }
  }

  private async processFile(filePath: string) {
    const absPath = path.resolve(filePath);
    if (!this.shouldProcessFile(absPath)) return;
    const stats = this.statFile(absPath);
    if (!stats || !stats.isFile()) return;

    if (stats.size > this.maxFileSizeBytes) {
      console.warn(`[Daemon] Skipping large file (${stats.size} bytes): ${absPath}`);
      return;
    }

    const nextFingerprint = this.fingerprintForStats(stats);
    if (this.fileFingerprints.get(absPath) === nextFingerprint) {
      return;
    }

    let rawContent: string;
    try {
      rawContent = fs.readFileSync(absPath, "utf8");
    } catch (err) {
      console.warn(`[Daemon] Failed to read file ${absPath}`, err);
      return;
    }

    const nextContentHash = this.hashText(rawContent);
    if (this.fileContentHashes.get(absPath) === nextContentHash) {
      // mtime-only/no-op write; skip AST + indexing work.
      this.fileFingerprints.set(absPath, nextFingerprint);
      return;
    }

    const uri = this.fileToUri(absPath);
    const parentUri = this.fileToParentUri(absPath);
    const summary = this.summarizeSourceFile(absPath, rawContent);
    const name = path.basename(absPath);
    const abstractText = summary.abstractText;
    const overviewText = summary.overviewText;
    const content = summary.nlContent;
    const metadata = {
      type: "file",
      path: absPath,
      logic_graph: summary.logicGraphMetadata,
    };
    const nextNodePayloadHash = this.hashText(
      `${abstractText}\n${overviewText}\n${content}\n${JSON.stringify(metadata)}`
    );

    if (this.nodePayloadHashes.get(absPath) === nextNodePayloadHash) {
      // File changed but extracted daemon payload is identical.
      this.fileFingerprints.set(absPath, nextFingerprint);
      this.fileContentHashes.set(absPath, nextContentHash);
      return;
    }

    // Upsert file nodes directly, avoiding router-based dedup.
    await this.manager.upsertFileContextNode(
      uri,
      name,
      abstractText,
      overviewText,
      content,
      parentUri,
      this.project,
      metadata
    );

    this.fileFingerprints.set(absPath, nextFingerprint);
    this.fileContentHashes.set(absPath, nextContentHash);
    this.nodePayloadHashes.set(absPath, nextNodePayloadHash);

    console.log(`[Daemon] Updated AST context for ${name} (${uri})`);
  }

  private fileToUri(filePath: string): string {
    const relPath = path.relative(this.watchDir, filePath);
    // Convert to contextfs scheme
    return `contextfs://${this.project}/${relPath.replace(/\\/g, "/")}`;
  }

  private fileToParentUri(filePath: string): string {
    const relPath = path.relative(this.watchDir, filePath);
    const dir = path.dirname(relPath);
    if (dir === "." || dir === "") return `contextfs://${this.project}`;
    return `contextfs://${this.project}/${dir.replace(/\\/g, "/")}`;
  }
}
