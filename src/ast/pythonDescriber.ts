import type { Node as SyntaxNode, Tree } from "web-tree-sitter";
import { Parser } from "web-tree-sitter";
import { createHash } from "crypto";
import type {
  LanguageDescriber,
  FileGraphResult,
  LogicSymbol,
  LogicEdge,
  LogicSymbolKind,
  ComplexityBucket,
} from "./languageDescriber";
import { sortSymbols, sortEdges } from "./languageDescriber";
import { describeStatements } from "./nlDescriber";
import { ParserPool, type SupportedLanguage } from "./parserPool";

export interface RawLogicGraph {
  symbols: LogicSymbol[];
  edges: LogicEdge[];
  imports: string[];
  callableNodes: CallableNodeRef[];
}

export interface CallableNodeRef {
  symbolId: string;
  className: string | null;
  node: SyntaxNode;
}

export class PythonDescriber implements LanguageDescriber {
  readonly languageId: string = "python";
  readonly extensions: ReadonlySet<string> = new Set([".py"]);

  extractFileGraph(filePath: string, sourceText: string): FileGraphResult {
    const tree = this.parseSync("python", sourceText);
    const root = tree.rootNode;

    try {
      const rawGraph = this.extractRawLogicGraph(root);

      // Generate per-symbol NL descriptions
      const symbolDescriptions = new Map<string, string>();

      for (const callable of rawGraph.callableNodes) {
        // We reuse describeStatements, but note that it's tuned for JS/TS
        // A dedicated python descriptor might be needed later, but this serves as a fallback.
        symbolDescriptions.set(callable.symbolId, describeStatements(callable.node));
      }

      // Generate class summaries
      for (const child of root.namedChildren) {
        if (child.type === "class_definition") {
          const className = child.childForFieldName("name")?.text ?? `anonymous_class_${child.startPosition.row + 1}`;
          const classId = `cls:${className}`;
          const superClass = this.getExtendsClause(child);
          const extendsStr = superClass.length > 0 ? ` extends ${superClass.join(", ")}` : "";
          const methodNames = this.getClassMethods(child).map((m) => m.childForFieldName("name")?.text ?? "");
          symbolDescriptions.set(
            classId,
            `Class \`${className}\`${extendsStr} with methods: ${methodNames.join(", ")}`,
          );
        }
      }

      // Generate file summary
      let fileSummary: string;
      if (rawGraph.symbols.length === 0) {
        fileSummary = "Empty or declaration-free source file.";
      } else {
        const exportedSymbols = rawGraph.symbols.filter((s) => s.exported);
        const names = exportedSymbols.map((s) => s.name);
        const kindCounts = new Map<string, number>();
        for (const s of exportedSymbols) {
          kindCounts.set(s.kind, (kindCounts.get(s.kind) ?? 0) + 1);
        }
        const kindSummary = Array.from(kindCounts.entries())
          .map(([kind, count]) => `${count} ${kind}${count > 1 ? "s" : ""}`)
          .join(", ");
        fileSummary = `File containing ${exportedSymbols.length} exported symbols (${kindSummary}): ${names.join(", ")}.`;
      }

      return {
        symbols: rawGraph.symbols,
        edges: rawGraph.edges,
        imports: rawGraph.imports,
        symbolDescriptions,
        fileSummary,
      };
    } finally {
      tree.delete();
    }
  }

  protected parseSync(lang: SupportedLanguage, sourceText: string): Tree {
    const parser = this.getParserSync(lang);
    const tree = parser.parse(sourceText);
    if (!tree) throw new Error(`Failed to parse ${lang} source`);
    return tree;
  }

  private parserCache = new Map<SupportedLanguage, InstanceType<typeof Parser>>();

  private getParserSync(lang: SupportedLanguage): InstanceType<typeof Parser> {
    const cached = this.parserCache.get(lang);
    if (cached) return cached;
    throw new Error(
      `Parser for ${lang} not initialized. Call PythonDescriber.initParsers() first.`,
    );
  }

  async initParsers(): Promise<void> {
    await ParserPool.init();
    const parser = await ParserPool.getParser("python");
    this.parserCache.set("python", parser);
  }

  deleteParsers(): void {
    for (const parser of this.parserCache.values()) {
      parser.delete();
    }
    this.parserCache.clear();
  }

  protected extractRawLogicGraph(root: SyntaxNode): RawLogicGraph {
    const symbols: LogicSymbol[] = [];
    const edgesMap = new Map<string, LogicEdge>();
    const nameToSymbolIds = new Map<string, string[]>();
    const symbolById = new Map<string, LogicSymbol>();
    const methodByClassAndName = new Map<string, string>();
    const methodIdsByName = new Map<string, string[]>();
    const callableNodes: CallableNodeRef[] = [];
    const moduleVariableByName = new Map<string, string>();

    const pushSymbol = (symbol: LogicSymbol) => {
      symbols.push(symbol);
      symbolById.set(symbol.id, symbol);
      const existing = nameToSymbolIds.get(symbol.name) ?? [];
      existing.push(symbol.id);
      nameToSymbolIds.set(symbol.name, existing);
    };

    const addEdge = (edge: LogicEdge) => {
      const key = `${edge.kind}|${edge.from}|${edge.to}`;
      if (!edgesMap.has(key)) {
        edgesMap.set(key, edge);
      }
    };

    // Python doesn't have an explicit 'export' keyword, so we treat everything at the module level
    // without a leading underscore as exported by convention.
    const isExported = (name: string) => !name.startsWith("_");

    for (const child of root.namedChildren) {
      switch (child.type) {
        case "import_statement":
        case "import_from_statement": {
          this.extractImports(child).forEach(mod => {
            addEdge({ kind: "import", from: "file", to: `module:${mod}` });
          });
          break;
        }

        case "class_definition": {
          const className = child.childForFieldName("name")?.text ?? `anonymous_class_${child.startPosition.row + 1}`;
          const classId = `cls:${className}`;
          const docstring = this.extractDocstring(child);
          const exported = isExported(className);
          pushSymbol(this.makeSymbol(classId, "cls", className, exported, null, [], child, docstring));

          const superClasses = this.getExtendsClause(child);
          for (const superClass of superClasses) {
            addEdge({ kind: "extends", from: classId, to: `type:${superClass}` });
          }

          for (const method of this.getClassMethods(child)) {
            const methodName = method.childForFieldName("name")?.text ?? "";
            const methodId = `mtd:${className}.${methodName}`;
            const params = this.extractParams(method);
            const methodDoc = this.extractDocstring(method);
            const methodExported = isExported(methodName); // could also be based on class export
            pushSymbol(this.makeSymbol(methodId, "mtd", methodName, methodExported, classId, params, method, methodDoc));
            methodByClassAndName.set(`${className}.${methodName}`, methodId);
            const existingMethodIds = methodIdsByName.get(methodName) ?? [];
            existingMethodIds.push(methodId);
            methodIdsByName.set(methodName, existingMethodIds);
            callableNodes.push({ symbolId: methodId, className, node: method });
          }
          break;
        }

        case "function_definition": {
          const fnName = child.childForFieldName("name")?.text ?? `anonymous_fn_${child.startPosition.row + 1}`;
          const fnId = `fn:${fnName}`;
          const params = this.extractParams(child);
          const docstring = this.extractDocstring(child);
          const exported = isExported(fnName);
          pushSymbol(this.makeSymbol(fnId, "fn", fnName, exported, null, params, child, docstring));
          callableNodes.push({ symbolId: fnId, className: null, node: child });
          break;
        }

        case "expression_statement": {
          // Check if it's an assignment like x = 10 or x: int = 10
          for (const expr of child.namedChildren) {
            if (expr.type === "assignment") {
              const left = expr.childForFieldName("left");
              if (left && left.type === "identifier") {
                const varName = left.text;
                const symbolId = `var:${varName}`;
                const exported = isExported(varName);
                // The next expression_statement might be a docstring, but python docstrings for variables are less standardized.
                pushSymbol(this.makeSymbol(symbolId, "var", varName, exported, null, [], expr));
                moduleVariableByName.set(varName, symbolId);
              }
            }
          }
          break;
        }
      }
    }

    for (const callable of callableNodes) {
      this.collectCallEdges(callable, methodByClassAndName, methodIdsByName, nameToSymbolIds, symbolById, addEdge);
      this.collectVariableEdges(callable, moduleVariableByName, addEdge);
    }

    const imports = this.collectImports(root);

    return {
      symbols: sortSymbols(symbols),
      edges: sortEdges(Array.from(edgesMap.values())),
      imports,
      callableNodes,
    };
  }

  private extractImports(node: SyntaxNode): string[] {
    const modules: string[] = [];
    if (node.type === "import_statement") {
      // import os, sys
      node.namedChildren.forEach(child => {
        if (child.type === "dotted_name" || child.type === "aliased_import") {
          const mod = child.childForFieldName("name")?.text ?? child.text;
          if (mod) modules.push(mod);
        }
      });
    } else if (node.type === "import_from_statement") {
      // from math import sqrt
      const modName = node.childForFieldName("module_name")?.text;
      if (modName) modules.push(modName);
    }
    return modules;
  }

  private collectImports(root: SyntaxNode): string[] {
    const modules = new Set<string>();
    for (const child of root.namedChildren) {
      this.extractImports(child).forEach(mod => modules.add(mod));
    }
    return Array.from(modules).sort((a, b) => a.localeCompare(b));
  }

  private getExtendsClause(classNode: SyntaxNode): string[] {
    const args = classNode.childForFieldName("superclasses");
    if (!args) return [];
    return args.namedChildren.map(c => c.text);
  }

  private getClassMethods(classNode: SyntaxNode): SyntaxNode[] {
    const body = classNode.childForFieldName("body");
    if (!body) return [];
    return body.namedChildren.filter((c) => c.type === "function_definition");
  }

  private extractParams(fnNode: SyntaxNode): string[] {
    const params = fnNode.childForFieldName("parameters");
    if (!params) return [];
    return params.namedChildren
      .map((p) => {
        if (p.type === "identifier") return p.text;
        if (p.type === "typed_parameter") return p.childForFieldName("name")?.text ?? p.text;
        if (p.type === "default_parameter" || p.type === "typed_default_parameter") return p.childForFieldName("name")?.text ?? p.text;
        return p.text;
      })
      .filter((n) => n.length > 0 && n !== "self" && n !== "cls"); // skip self/cls by convention
  }

  private extractDocstring(node: SyntaxNode): string | undefined {
    const body = node.childForFieldName("body");
    if (!body) return undefined;
    const firstStmt = body.namedChildren[0];
    if (firstStmt && firstStmt.type === "expression_statement") {
      const stringNode = firstStmt.namedChildren[0];
      if (stringNode && stringNode.type === "string") {
        let text = stringNode.text.replace(/^["']{1,3}|["']{1,3}$/g, "");
        // Find the first non-empty line
        const lines = text.split("\n").map(l => l.trim()).filter(l => l.length > 0);
        if (lines.length > 0) {
          text = lines[0];
          return text.length > 200 ? text.slice(0, 200) + "..." : text;
        }
      }
    }
    return undefined;
  }

  private makeSymbol(
    id: string,
    kind: LogicSymbolKind,
    name: string,
    exported: boolean,
    parentId: string | null,
    params: string[],
    node: SyntaxNode,
    docstring?: string,
  ): LogicSymbol {
    const isFnLike = this.isFunctionLikeNode(node);
    const control = {
      async: isFnLike ? this.isAsync(node) : false,
      branch: isFnLike ? this.hasBranching(node) : false,
      await: isFnLike ? this.hasDescendantOfType(node, "await") : false,
      throw: isFnLike ? this.hasDescendantOfType(node, "raise_statement") : false,
    };

    return {
      id,
      kind,
      name,
      exported,
      parentId,
      params: [...params].sort((a, b) => a.localeCompare(b)),
      complexity: this.computeComplexityBucket(isFnLike ? node : null),
      control,
      line: node.startPosition.row + 1,
      byteStart: node.startIndex,
      byteEnd: node.endIndex,
      contentHash: createHash("sha1").update(node.text).digest("hex"),
      ...(docstring ? { docstring } : {}),
    };
  }

  private isFunctionLikeNode(node: SyntaxNode): boolean {
    return node.type === "function_definition";
  }

  private isAsync(node: SyntaxNode): boolean {
    // In python tree-sitter, async functions are usually just 'function_definition'
    // but they might have an 'async' keyword. Wait, no, they are parsed differently?
    // Let's assume there is an 'async' modifier, or check text.
    return node.text.startsWith("async ");
  }

  private hasBranching(node: SyntaxNode): boolean {
    const branchTypes = new Set([
      "if_statement", "for_statement", "while_statement", "match_statement"
    ]);
    return this.hasDescendantOfTypes(node, branchTypes);
  }

  private hasDescendantOfType(node: SyntaxNode, type: string): boolean {
    for (const child of node.namedChildren) {
      if (child.type === type) return true;
      if (this.hasDescendantOfType(child, type)) return true;
    }
    return false;
  }

  private hasDescendantOfTypes(node: SyntaxNode, types: Set<string>): boolean {
    for (const child of node.namedChildren) {
      if (types.has(child.type)) return true;
      if (this.hasDescendantOfTypes(child, types)) return true;
    }
    return false;
  }

  private computeComplexityBucket(node: SyntaxNode | null): ComplexityBucket {
    if (!node) return "low";
    const count = this.countDescendantStatements(node);
    if (count >= 18) return "high";
    if (count >= 6) return "medium";
    return "low";
  }

  private countDescendantStatements(node: SyntaxNode): number {
    let count = 0;
    for (const child of node.namedChildren) {
      if (child.type.endsWith("_statement") || child.type.endsWith("_definition")) {
        count++;
      }
      count += this.countDescendantStatements(child);
    }
    return count;
  }

  private collectCallEdges(
    callable: CallableNodeRef,
    methodByClassAndName: Map<string, string>,
    methodIdsByName: Map<string, string[]>,
    nameToSymbolIds: Map<string, string[]>,
    symbolById: Map<string, LogicSymbol>,
    addEdge: (edge: LogicEdge) => void,
  ): void {
    this.walkForType(callable.node, "call", (callExpr) => {
      const targetId = this.resolveCallTarget(
        callExpr,
        callable.className,
        methodByClassAndName,
        methodIdsByName,
        nameToSymbolIds,
        symbolById,
      );
      if (targetId) {
        addEdge({ kind: "call", from: callable.symbolId, to: targetId });
      }
    });
  }

  private collectVariableEdges(
    callable: CallableNodeRef,
    moduleVariableByName: Map<string, string>,
    addEdge: (edge: LogicEdge) => void,
  ): void {
    this.walkForType(callable.node, "identifier", (identifier) => {
      const name = identifier.text;
      const variableId = moduleVariableByName.get(name);
      if (!variableId) return;
      if (this.isDeclarationIdentifier(identifier)) return;

      const isWrite = this.isWriteIdentifier(identifier);
      addEdge({
        kind: isWrite ? "write" : "read",
        from: callable.symbolId,
        to: variableId,
      });
    });
  }

  private walkForType(node: SyntaxNode, type: string, callback: (node: SyntaxNode) => void): void {
    for (const child of node.namedChildren) {
      if (child.type === type) callback(child);
      this.walkForType(child, type, callback);
    }
  }

  private resolveCallTarget(
    callExpr: SyntaxNode,
    callerClassName: string | null,
    methodByClassAndName: Map<string, string>,
    methodIdsByName: Map<string, string[]>,
    nameToSymbolIds: Map<string, string[]>,
    symbolById: Map<string, LogicSymbol>,
  ): string | null {
    const fn = callExpr.childForFieldName("function");
    if (!fn) return null;

    if (fn.type === "identifier") {
      const symbolIds = nameToSymbolIds.get(fn.text) ?? [];
      return this.pickBestCallableSymbolId(symbolIds, symbolById);
    }

    if (fn.type === "attribute") {
      const property = fn.childForFieldName("attribute");
      const object = fn.childForFieldName("object");
      const methodName = property?.text ?? "";

      if (object?.text === "self" && callerClassName) {
        const ownMethod = methodByClassAndName.get(`${callerClassName}.${methodName}`);
        if (ownMethod) return ownMethod;
      }
      const candidateMethodIds = methodIdsByName.get(methodName) ?? [];
      if (candidateMethodIds.length === 1) return candidateMethodIds[0];
    }

    return null;
  }

  private pickBestCallableSymbolId(
    symbolIds: string[],
    symbolById: Map<string, LogicSymbol>,
  ): string | null {
    const ranked = symbolIds
      .map((id) => symbolById.get(id))
      .filter((s): s is LogicSymbol => !!s)
      .filter((s) => s.kind === "fn" || s.kind === "mtd")
      .sort((a, b) => {
        const kindWeight = (s: LogicSymbol) => (s.kind === "fn" ? 0 : 1);
        const diff = kindWeight(a) - kindWeight(b);
        if (diff !== 0) return diff;
        return a.id.localeCompare(b.id);
      });
    return ranked[0]?.id ?? null;
  }

  private isDeclarationIdentifier(identifier: SyntaxNode): boolean {
    const parent = identifier.parent;
    if (!parent) return false;
    const nameNode = parent.childForFieldName("name");
    if (nameNode && nameNode.id === identifier.id) {
      const declarationTypes = ["function_definition", "class_definition"];
      return declarationTypes.includes(parent.type);
    }
    return false;
  }

  private isWriteIdentifier(identifier: SyntaxNode): boolean {
    const parent = identifier.parent;
    if (!parent) return false;

    if (parent.type === "assignment" || parent.type === "augmented_assignment") {
      const left = parent.childForFieldName("left");
      if (left && left.id === identifier.id) return true;
    }

    return false;
  }
}
