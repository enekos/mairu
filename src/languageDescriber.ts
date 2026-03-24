export type LogicSymbolKind = "cls" | "fn" | "mtd" | "var" | "iface" | "enum" | "type";
export type LogicEdgeKind = "call" | "import" | "read" | "write" | "extends" | "implements";
export type ComplexityBucket = "low" | "medium" | "high";

export interface LogicSymbol {
  id: string;
  kind: LogicSymbolKind;
  name: string;
  exported: boolean;
  parentId: string | null;
  params: string[];
  complexity: ComplexityBucket;
  control: {
    async: boolean;
    branch: boolean;
    await: boolean;
    throw: boolean;
  };
  line: number;
}

export interface LogicEdge {
  kind: LogicEdgeKind;
  from: string;
  to: string;
}

export interface FileGraphResult {
  symbols: LogicSymbol[];
  edges: LogicEdge[];
  imports: string[];
  symbolDescriptions: Map<string, string>;
  fileSummary: string;
}

export interface LanguageDescriber {
  readonly languageId: string;
  readonly extensions: ReadonlySet<string>;
  extractFileGraph(filePath: string, sourceText: string): FileGraphResult;
}
