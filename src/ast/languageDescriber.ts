export type LogicSymbolKind = "cls" | "fn" | "mtd" | "var" | "iface" | "enum" | "type"
  | "tpl" | "tpl-slot" | "tpl-branch" | "tpl-loop";
export type LogicEdgeKind = "call" | "import" | "read" | "write" | "extends" | "implements"
  | "render" | "slot";
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
  byteStart: number;
  byteEnd: number;
  startLine: number;
  endLine: number;
  contentHash: string;
  docstring?: string;
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

export const KIND_SORT_ORDER: Record<LogicSymbolKind, number> = {
  cls: 0,
  fn: 1,
  mtd: 2,
  var: 3,
  iface: 4,
  enum: 5,
  type: 6,
  tpl: 7,
  "tpl-branch": 8,
  "tpl-loop": 9,
  "tpl-slot": 10,
};

export function compareSymbols(a: LogicSymbol, b: LogicSymbol): number {
  const kindDiff = KIND_SORT_ORDER[a.kind] - KIND_SORT_ORDER[b.kind];
  if (kindDiff !== 0) return kindDiff;
  const nameDiff = a.name.localeCompare(b.name);
  if (nameDiff !== 0) return nameDiff;
  const lineDiff = a.line - b.line;
  if (lineDiff !== 0) return lineDiff;
  return a.id.localeCompare(b.id);
}

export function sortSymbols(symbols: LogicSymbol[]): LogicSymbol[] {
  return [...symbols].sort((a, b) => compareSymbols(a, b));
}

export function sortEdges(edges: LogicEdge[]): LogicEdge[] {
  return [...edges].sort((a, b) => {
    const kindDiff = a.kind.localeCompare(b.kind);
    if (kindDiff !== 0) return kindDiff;
    const fromDiff = a.from.localeCompare(b.from);
    if (fromDiff !== 0) return fromDiff;
    return a.to.localeCompare(b.to);
  });
}
