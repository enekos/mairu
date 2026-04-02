import { Parser, Language } from "web-tree-sitter";
import type { Tree } from "web-tree-sitter";
import * as path from "path";
import * as fs from "fs";

export type SupportedLanguage = "typescript" | "tsx" | "javascript" | "vue" | "python";

/**
 * Resolve a path inside a node_modules package by walking up from this file.
 * Works regardless of export map restrictions.
 */
function resolvePackagePath(packageName: string, ...subPath: string[]): string {
  let dir = __dirname;
  while (dir !== path.dirname(dir)) {
    const candidate = path.join(dir, "node_modules", packageName, ...subPath);
    if (fs.existsSync(candidate)) return candidate;
    dir = path.dirname(dir);
  }
  throw new Error(`Cannot find ${packageName}/${subPath.join("/")} in node_modules`);
}

const WASM_DIR = resolvePackagePath("@repomix/tree-sitter-wasms", "out");
const TS_WASM = resolvePackagePath("web-tree-sitter", "web-tree-sitter.wasm");

const WASM_FILES: Record<SupportedLanguage, string> = {
  typescript: path.join(WASM_DIR, "tree-sitter-typescript.wasm"),
  tsx: path.join(WASM_DIR, "tree-sitter-tsx.wasm"),
  javascript: path.join(WASM_DIR, "tree-sitter-javascript.wasm"),
  vue: path.join(WASM_DIR, "tree-sitter-vue.wasm"),
  python: path.join(WASM_DIR, "tree-sitter-python.wasm"),
};

let initialized = false;
const languages = new Map<SupportedLanguage, Language>();

/**
 * Singleton parser pool: initializes web-tree-sitter WASM once,
 * caches Language objects, and provides parsers on demand.
 */
export const ParserPool = {
  /** Must be called once before any parsing. Idempotent. */
  async init(): Promise<void> {
    if (initialized) return;
    await Parser.init({
      locateFile: () => TS_WASM,
    });
    initialized = true;
  },

  /** Load and cache a language. Call after init(). */
  async loadLanguage(lang: SupportedLanguage): Promise<Language> {
    const cached = languages.get(lang);
    if (cached) return cached;
    const wasmPath = WASM_FILES[lang];
    const language = await Language.load(wasmPath);
    languages.set(lang, language);
    return language;
  },

  /** Get a ready-to-use parser for the given language. Caller must call tree.delete() when done. */
  async getParser(lang: SupportedLanguage): Promise<InstanceType<typeof Parser>> {
    const language = await this.loadLanguage(lang);
    const parser = new Parser();
    parser.setLanguage(language);
    return parser;
  },

  /** Parse source text and return the tree. Caller must call tree.delete() when done. */
  async parse(lang: SupportedLanguage, sourceText: string): Promise<Tree> {
    const parser = await this.getParser(lang);
    const tree = parser.parse(sourceText);
    parser.delete();
    if (!tree) throw new Error(`Failed to parse ${lang} source`);
    return tree;
  },

  /** Check if the pool has been initialized. */
  isInitialized(): boolean {
    return initialized;
  },

  /** Reset for testing. */
  _reset(): void {
    initialized = false;
    languages.clear();
  },
};
