import { Project, SourceFile } from "ts-morph";
import { ContextManager } from "./contextManager";
import * as path from "path";
import * as chokidar from "chokidar";
import * as fs from "fs";

type ContentFormat = "compact" | "full";

export interface DaemonOptions {
  maxFileSizeBytes?: number;
  processingDebounceMs?: number;
  contentFormat?: ContentFormat;
}

const DEFAULT_MAX_FILE_SIZE_BYTES = 512 * 1024; // 512 KB
const DEFAULT_PROCESSING_DEBOUNCE_MS = 200;
const DEFAULT_CONTENT_FORMAT: ContentFormat = "compact";
const SUPPORTED_EXTENSIONS = new Set([".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"]);
const IGNORED_PATH_SEGMENTS = new Set(["node_modules", "dist", "build"]);

interface SourceSummary {
  classNames: string[];
  classMethods: Map<string, string[]>;
  functionSignatures: string[];
  variableNames: string[];
  interfaceNames: string[];
  enumNames: string[];
  typeAliasNames: string[];
  abstractText: string;
  overviewText: string;
}

export class CodebaseDaemon {
  private manager: ContextManager;
  private project: string;
  private watchDir: string;
  private tsProject: Project;
  private watcher: chokidar.FSWatcher | null = null;
  private isProcessing = false;
  private pendingFiles: Set<string> = new Set();
  private processTimer: NodeJS.Timeout | null = null;
  private readonly maxFileSizeBytes: number;
  private readonly processingDebounceMs: number;
  private readonly contentFormat: ContentFormat;
  private readonly fileFingerprints: Map<string, string> = new Map();

  constructor(manager: ContextManager, project: string, watchDir: string, options: DaemonOptions = {}) {
    this.manager = manager;
    this.project = project;
    this.watchDir = path.resolve(watchDir);
    this.maxFileSizeBytes = options.maxFileSizeBytes ?? DEFAULT_MAX_FILE_SIZE_BYTES;
    this.processingDebounceMs = options.processingDebounceMs ?? DEFAULT_PROCESSING_DEBOUNCE_MS;
    this.contentFormat = options.contentFormat ?? DEFAULT_CONTENT_FORMAT;
    this.tsProject = new Project({
      compilerOptions: {
        allowJs: true,
      },
    });
  }

  public async start() {
    console.log(`[Daemon] Starting codebase monitor for project '${this.project}' in ${this.watchDir}`);
    
    // Initial scan
    this.tsProject.addSourceFilesAtPaths(this.getSourceGlobs());
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

  private getSourceGlobs(): string[] {
    return [
      `${this.watchDir}/**/*.{ts,tsx,js,jsx,mjs,cjs}`,
      `!${this.watchDir}/**/node_modules/**`,
      `!${this.watchDir}/**/dist/**`,
      `!${this.watchDir}/**/build/**`,
    ];
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

  private flushPendingSoon() {
    if (this.processTimer) return;
    this.processTimer = setTimeout(() => {
      this.processTimer = null;
      void this.processPendingFiles();
    }, this.processingDebounceMs);
  }

  private clearSourceFileFromProject(filePath: string) {
    const sourceFile = this.tsProject.getSourceFile(filePath);
    if (!sourceFile) return;
    try {
      this.tsProject.removeSourceFile(sourceFile);
    } catch (err) {
      console.warn(`[Daemon] Failed to remove source from cache: ${filePath}`, err);
    }
  }

  private summarizeSourceFile(sourceFile: SourceFile): SourceSummary {
    const exportedDecls = sourceFile.getExportedDeclarations();
    const classNames = new Set<string>();
    const functionNames = new Set<string>();
    const variableNames = new Set<string>();
    const interfaceNames = new Set<string>();
    const enumNames = new Set<string>();
    const typeAliasNames = new Set<string>();

    for (const [name, declarations] of exportedDecls.entries()) {
      for (const declaration of declarations) {
        if (declaration.getKindName() === "ClassDeclaration") {
          classNames.add(name);
          continue;
        }
        if (declaration.getKindName() === "FunctionDeclaration") {
          functionNames.add(name);
          continue;
        }
        if (declaration.getKindName() === "VariableDeclaration") {
          variableNames.add(name);
          continue;
        }
        if (declaration.getKindName() === "InterfaceDeclaration") {
          interfaceNames.add(name);
          continue;
        }
        if (declaration.getKindName() === "EnumDeclaration") {
          enumNames.add(name);
          continue;
        }
        if (declaration.getKindName() === "TypeAliasDeclaration") {
          typeAliasNames.add(name);
          continue;
        }
      }
    }

    const classes = Array.from(classNames).sort();
    const functions = Array.from(functionNames).sort();
    const variables = Array.from(variableNames).sort();
    const interfaces = Array.from(interfaceNames).sort();
    const enums = Array.from(enumNames).sort();
    const typeAliases = Array.from(typeAliasNames).sort();

    const abstractParts: string[] = [];
    const overviewParts: string[] = [];
    const classMethods = new Map<string, string[]>();
    const functionSignatures: string[] = [];

    if (classes.length > 0) {
      abstractParts.push(`Exports ${classes.length} classes: ${classes.join(", ")}`);
      for (const className of classes) {
        const cls = sourceFile.getClass(className);
        overviewParts.push(`Class ${className}:`);
        if (!cls) {
          classMethods.set(className, []);
          continue;
        }
        const methods = cls.getMethods().map((m) => m.getName()).sort();
        classMethods.set(className, methods);
        if (methods.length > 0) {
          overviewParts.push(`  Methods: ${methods.join(", ")}`);
        }
      }
    }

    if (functions.length > 0) {
      abstractParts.push(`Exports ${functions.length} functions: ${functions.join(", ")}`);
      for (const fnName of functions) {
        const fn = sourceFile.getFunction(fnName);
        if (fn) {
          const params = fn.getParameters().map((p) => p.getName()).join(", ");
          const signature = `Function ${fnName}(${params})`;
          functionSignatures.push(signature);
          overviewParts.push(signature);
        } else {
          const signature = `Function ${fnName}(...)`;
          functionSignatures.push(signature);
          overviewParts.push(signature);
        }
      }
    }

    if (variables.length > 0) {
      abstractParts.push(`Exports ${variables.length} variables: ${variables.join(", ")}`);
    }
    if (interfaces.length > 0) {
      abstractParts.push(`Exports ${interfaces.length} interfaces: ${interfaces.join(", ")}`);
    }
    if (enums.length > 0) {
      abstractParts.push(`Exports ${enums.length} enums: ${enums.join(", ")}`);
    }
    if (typeAliases.length > 0) {
      abstractParts.push(`Exports ${typeAliases.length} type aliases: ${typeAliases.join(", ")}`);
    }

    const abstractText = abstractParts.length > 0
      ? abstractParts.join(". ")
      : `File ${sourceFile.getBaseName()} containing source code.`;
    const overviewText = overviewParts.length > 0
      ? overviewParts.join("\n")
      : "No exported classes or functions found.";

    return {
      classNames: classes,
      classMethods,
      functionSignatures,
      variableNames: variables,
      interfaceNames: interfaces,
      enumNames: enums,
      typeAliasNames: typeAliases,
      abstractText,
      overviewText,
    };
  }

  private buildCompactContent(filePath: string, sourceFile: SourceFile, summary: SourceSummary): string {
    const relPath = path.relative(this.watchDir, filePath).replace(/\\/g, "/");
    const importModules = sourceFile.getImportDeclarations()
      .map((decl) => decl.getModuleSpecifierValue())
      .filter(Boolean);
    const lines: string[] = [
      `File: ${relPath}`,
      `Language: ${path.extname(filePath).slice(1) || "unknown"}`,
      "",
      "Imports:",
    ];

    if (importModules.length === 0) {
      lines.push("- (none)");
    } else {
      for (const moduleName of importModules) {
        lines.push(`- ${moduleName}`);
      }
    }

    lines.push("", "Exports:");
    const hasExports = summary.classNames.length > 0
      || summary.functionSignatures.length > 0
      || summary.variableNames.length > 0
      || summary.interfaceNames.length > 0
      || summary.enumNames.length > 0
      || summary.typeAliasNames.length > 0;

    if (!hasExports) {
      lines.push("- (none)");
      return lines.join("\n");
    }

    for (const className of summary.classNames) {
      const methods = summary.classMethods.get(className) ?? [];
      if (methods.length === 0) {
        lines.push(`- class ${className}`);
      } else {
        lines.push(`- class ${className} { methods: ${methods.join(", ")} }`);
      }
    }
    for (const signature of summary.functionSignatures) {
      lines.push(`- ${signature}`);
    }
    for (const variableName of summary.variableNames) {
      lines.push(`- const/let/var ${variableName}`);
    }
    for (const interfaceName of summary.interfaceNames) {
      lines.push(`- interface ${interfaceName}`);
    }
    for (const enumName of summary.enumNames) {
      lines.push(`- enum ${enumName}`);
    }
    for (const typeAliasName of summary.typeAliasNames) {
      lines.push(`- type ${typeAliasName}`);
    }

    return lines.join("\n");
  }

  private queueFile(filePath: string) {
    if (!this.shouldProcessFile(filePath)) return;
    this.pendingFiles.add(path.resolve(filePath));
    this.flushPendingSoon();
  }

  private async processPendingFiles() {
    if (this.isProcessing) return;
    this.isProcessing = true;

    try {
      while (this.pendingFiles.size > 0) {
        const filePath = Array.from(this.pendingFiles)[0];
        this.pendingFiles.delete(filePath);
        await this.processFile(filePath);
      }
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
    const files = this.tsProject.getSourceFiles().filter((f) => this.shouldProcessFile(f.getFilePath()));
    for (const file of files) {
      await this.processFile(file.getFilePath());
    }
    console.log(`[Daemon] Initial scan complete (${files.length} files).`);
  }

  private async handleFileDelete(filePath: string) {
    const absPath = path.resolve(filePath);
    const uri = this.fileToUri(absPath);
    this.pendingFiles.delete(absPath);
    this.fileFingerprints.delete(absPath);
    this.clearSourceFileFromProject(absPath);

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
      this.clearSourceFileFromProject(absPath);
      console.warn(`[Daemon] Skipping large file (${stats.size} bytes): ${absPath}`);
      return;
    }

    const nextFingerprint = this.fingerprintForStats(stats);
    if (this.fileFingerprints.get(absPath) === nextFingerprint) {
      return;
    }
    this.fileFingerprints.set(absPath, nextFingerprint);

    let sourceFile = this.tsProject.getSourceFile(absPath);
    try {
      if (sourceFile) {
        await sourceFile.refreshFromFileSystem();
      } else {
        sourceFile = this.tsProject.addSourceFileAtPathIfExists(absPath);
      }
    } catch (err) {
      console.warn(`[Daemon] Failed to parse file ${absPath}`, err);
      this.clearSourceFileFromProject(absPath);
      return;
    }

    if (!sourceFile) return;

    const uri = this.fileToUri(absPath);
    const parentUri = this.fileToParentUri(absPath);
    const summary = this.summarizeSourceFile(sourceFile);
    const name = path.basename(absPath);
    const content = this.contentFormat === "full"
      ? sourceFile.getFullText()
      : this.buildCompactContent(absPath, sourceFile, summary);

    // We update or add the file as a context node, skipping the router so it overrides directly.
    await this.manager.addContextNode(
      uri,
      name,
      summary.abstractText,
      summary.overviewText,
      content,
      parentUri,
      this.project,
      { type: "file", path: absPath },
      false // don't use router, force update
    );

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
