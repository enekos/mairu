import type { LanguageDescriber, FileGraphResult } from "./languageDescriber";

export class TypeScriptDescriber implements LanguageDescriber {
  readonly languageId = "typescript";
  readonly extensions: ReadonlySet<string> = new Set([".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"]);

  extractFileGraph(_filePath: string, _sourceText: string): FileGraphResult {
    return {
      symbols: [],
      edges: [],
      imports: [],
      symbolDescriptions: new Map(),
      fileSummary: "",
    };
  }
}
