import {
  CallExpression,
  FunctionDeclaration,
  MethodDeclaration,
  Node,
  Project,
  SourceFile,
  SyntaxKind,
} from "ts-morph";
import type {
  LanguageDescriber,
  FileGraphResult,
  LogicSymbol,
  LogicEdge,
  LogicSymbolKind,
  ComplexityBucket,
} from "./languageDescriber";

interface RawLogicGraph {
  symbols: LogicSymbol[];
  edges: LogicEdge[];
  imports: string[];
}

interface CallableSymbolRef {
  symbolId: string;
  className: string | null;
  node: FunctionDeclaration | MethodDeclaration;
}

const KIND_SORT_ORDER: Record<LogicSymbolKind, number> = {
  cls: 0,
  fn: 1,
  mtd: 2,
  var: 3,
  iface: 4,
  enum: 5,
  type: 6,
};

export class TypeScriptDescriber implements LanguageDescriber {
  readonly languageId = "typescript";
  readonly extensions: ReadonlySet<string> = new Set([".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"]);

  extractFileGraph(filePath: string, sourceText: string): FileGraphResult {
    const project = new Project({
      compilerOptions: { allowJs: true },
      useInMemoryFileSystem: true,
    });
    const sourceFile = project.createSourceFile(filePath, sourceText);
    const rawGraph = this.extractRawLogicGraph(sourceFile);

    return {
      symbols: rawGraph.symbols,
      edges: rawGraph.edges,
      imports: rawGraph.imports,
      symbolDescriptions: new Map(),
      fileSummary: rawGraph.symbols.length === 0
        ? "Empty or declaration-free source file."
        : `Source file with ${rawGraph.symbols.length} symbols and ${rawGraph.edges.length} edges.`,
    };
  }

  private extractRawLogicGraph(sourceFile: SourceFile): RawLogicGraph {
    const symbols: LogicSymbol[] = [];
    const edgesMap = new Map<string, LogicEdge>();
    const nameToSymbolIds = new Map<string, string[]>();
    const symbolById = new Map<string, LogicSymbol>();
    const methodByClassAndName = new Map<string, string>();
    const methodIdsByName = new Map<string, string[]>();
    const callableSymbols: CallableSymbolRef[] = [];
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

    for (const importDecl of sourceFile.getImportDeclarations()) {
      const moduleName = importDecl.getModuleSpecifierValue().trim();
      if (!moduleName) continue;
      addEdge({ kind: "import", from: "file", to: `module:${moduleName}` });
    }

    for (const cls of sourceFile.getClasses()) {
      const className = cls.getName() ?? `anonymous_class_${cls.getStartLineNumber()}`;
      const classId = `cls:${className}`;
      pushSymbol(this.makeSymbol(classId, "cls", className, cls.isExported(), null, [], cls));

      const extendsNode = cls.getExtends();
      if (extendsNode) {
        addEdge({
          kind: "extends",
          from: classId,
          to: `type:${extendsNode.getExpression().getText().trim()}`,
        });
      }
      for (const implemented of cls.getImplements()) {
        addEdge({
          kind: "implements",
          from: classId,
          to: `type:${implemented.getText().trim()}`,
        });
      }

      for (const method of cls.getMethods()) {
        const methodName = method.getName();
        const methodId = `mtd:${className}.${methodName}`;
        pushSymbol(
          this.makeSymbol(
            methodId,
            "mtd",
            methodName,
            cls.isExported(),
            classId,
            method.getParameters().map((param) => param.getName()),
            method
          )
        );
        methodByClassAndName.set(`${className}.${methodName}`, methodId);
        const existingMethodIds = methodIdsByName.get(methodName) ?? [];
        existingMethodIds.push(methodId);
        methodIdsByName.set(methodName, existingMethodIds);
        callableSymbols.push({ symbolId: methodId, className, node: method });
      }
    }

    for (const fn of sourceFile.getFunctions()) {
      const fnName = fn.getName() ?? `anonymous_fn_${fn.getStartLineNumber()}`;
      const fnId = `fn:${fnName}`;
      pushSymbol(
        this.makeSymbol(
          fnId,
          "fn",
          fnName,
          fn.isExported(),
          null,
          fn.getParameters().map((param) => param.getName()),
          fn
        )
      );
      callableSymbols.push({ symbolId: fnId, className: null, node: fn });
    }

    for (const decl of sourceFile.getVariableDeclarations()) {
      const variableStmt = decl.getVariableStatement();
      if (!variableStmt) continue;
      if (variableStmt.getParent() !== sourceFile) continue;

      const variableName = decl.getName();
      const symbolId = `var:${variableName}`;
      pushSymbol(this.makeSymbol(symbolId, "var", variableName, variableStmt.isExported(), null, [], decl));
      moduleVariableByName.set(variableName, symbolId);
    }

    for (const iface of sourceFile.getInterfaces()) {
      const name = iface.getName();
      const symbolId = `iface:${name}`;
      pushSymbol(this.makeSymbol(symbolId, "iface", name, iface.isExported(), null, [], iface));
    }

    for (const enumDecl of sourceFile.getEnums()) {
      const name = enumDecl.getName();
      const symbolId = `enum:${name}`;
      pushSymbol(this.makeSymbol(symbolId, "enum", name, enumDecl.isExported(), null, [], enumDecl));
    }

    for (const typeAlias of sourceFile.getTypeAliases()) {
      const name = typeAlias.getName();
      const symbolId = `type:${name}`;
      pushSymbol(this.makeSymbol(symbolId, "type", name, typeAlias.isExported(), null, [], typeAlias));
    }

    for (const callable of callableSymbols) {
      for (const callExpr of callable.node.getDescendantsOfKind(SyntaxKind.CallExpression)) {
        const targetId = this.resolveCallTarget(
          callExpr,
          callable.className,
          methodByClassAndName,
          methodIdsByName,
          nameToSymbolIds,
          symbolById
        );
        if (!targetId) continue;
        addEdge({ kind: "call", from: callable.symbolId, to: targetId });
      }

      for (const identifier of callable.node.getDescendantsOfKind(SyntaxKind.Identifier)) {
        const identifierName = identifier.getText();
        const variableId = moduleVariableByName.get(identifierName);
        if (!variableId) continue;
        if (this.isDeclarationIdentifier(identifier)) continue;

        const isWrite = this.isWriteIdentifier(identifier);
        addEdge({
          kind: isWrite ? "write" : "read",
          from: callable.symbolId,
          to: variableId,
        });
      }
    }

    const imports = Array.from(
      new Set(
        sourceFile.getImportDeclarations()
          .map((decl) => decl.getModuleSpecifierValue().trim())
          .filter((value) => value.length > 0)
      )
    ).sort((a, b) => a.localeCompare(b));

    return {
      symbols: this.sortSymbols(symbols),
      edges: this.sortEdges(Array.from(edgesMap.values())),
      imports,
    };
  }

  private makeSymbol(
    id: string,
    kind: LogicSymbolKind,
    name: string,
    exported: boolean,
    parentId: string | null,
    params: string[],
    node: Node
  ): LogicSymbol {
    const functionLikeNode = this.asFunctionLikeNode(node);
    const control = {
      async: this.isAsyncFunctionLike(functionLikeNode),
      branch: !!functionLikeNode && this.hasBranching(functionLikeNode),
      await: !!functionLikeNode && functionLikeNode.getDescendantsOfKind(SyntaxKind.AwaitExpression).length > 0,
      throw: !!functionLikeNode && functionLikeNode.getDescendantsOfKind(SyntaxKind.ThrowStatement).length > 0,
    };

    return {
      id,
      kind,
      name,
      exported,
      parentId,
      params: [...params].sort((a, b) => a.localeCompare(b)),
      complexity: this.computeComplexityBucket(functionLikeNode),
      control,
      line: node.getStartLineNumber(),
    };
  }

  private asFunctionLikeNode(node: Node): FunctionDeclaration | MethodDeclaration | null {
    if (Node.isFunctionDeclaration(node)) return node;
    if (Node.isMethodDeclaration(node)) return node;
    return null;
  }

  private isAsyncFunctionLike(node: FunctionDeclaration | MethodDeclaration | null): boolean {
    if (!node) return false;
    return node.isAsync();
  }

  private hasBranching(node: FunctionDeclaration | MethodDeclaration): boolean {
    return node.getDescendantsOfKind(SyntaxKind.IfStatement).length > 0
      || node.getDescendantsOfKind(SyntaxKind.SwitchStatement).length > 0
      || node.getDescendantsOfKind(SyntaxKind.ConditionalExpression).length > 0
      || node.getDescendantsOfKind(SyntaxKind.ForStatement).length > 0
      || node.getDescendantsOfKind(SyntaxKind.ForOfStatement).length > 0
      || node.getDescendantsOfKind(SyntaxKind.ForInStatement).length > 0
      || node.getDescendantsOfKind(SyntaxKind.WhileStatement).length > 0
      || node.getDescendantsOfKind(SyntaxKind.DoStatement).length > 0;
  }

  private computeComplexityBucket(node: FunctionDeclaration | MethodDeclaration | null): ComplexityBucket {
    if (!node) return "low";
    const statements = node.getDescendants().filter((descendant) => Node.isStatement(descendant)).length;
    if (statements >= 18) return "high";
    if (statements >= 6) return "medium";
    return "low";
  }

  private resolveCallTarget(
    callExpr: CallExpression,
    callerClassName: string | null,
    methodByClassAndName: Map<string, string>,
    methodIdsByName: Map<string, string[]>,
    nameToSymbolIds: Map<string, string[]>,
    symbolById: Map<string, LogicSymbol>
  ): string | null {
    const callTarget = callExpr.getExpression();
    if (Node.isIdentifier(callTarget)) {
      const symbolIds = nameToSymbolIds.get(callTarget.getText()) ?? [];
      return this.pickBestCallableSymbolId(symbolIds, symbolById);
    }

    if (Node.isPropertyAccessExpression(callTarget)) {
      const methodName = callTarget.getName();
      const expressionText = callTarget.getExpression().getText();
      if (expressionText === "this" && callerClassName) {
        const ownMethod = methodByClassAndName.get(`${callerClassName}.${methodName}`);
        if (ownMethod) return ownMethod;
      }
      const candidateMethodIds = methodIdsByName.get(methodName) ?? [];
      if (candidateMethodIds.length === 1) return candidateMethodIds[0];
    }

    return null;
  }

  private pickBestCallableSymbolId(symbolIds: string[], symbolById: Map<string, LogicSymbol>): string | null {
    const ranked = symbolIds
      .map((symbolId) => symbolById.get(symbolId))
      .filter((symbol): symbol is LogicSymbol => !!symbol)
      .filter((symbol) => symbol.kind === "fn" || symbol.kind === "mtd")
      .sort((a, b) => {
        const kindWeight = (symbol: LogicSymbol) => (symbol.kind === "fn" ? 0 : 1);
        const weightDiff = kindWeight(a) - kindWeight(b);
        if (weightDiff !== 0) return weightDiff;
        return a.id.localeCompare(b.id);
      });
    return ranked[0]?.id ?? null;
  }

  private isDeclarationIdentifier(identifier: Node): boolean {
    const parent = identifier.getParent();
    if (!parent) return false;
    if (Node.isVariableDeclaration(parent) && parent.getNameNode() === identifier) return true;
    if (Node.isParameterDeclaration(parent) && parent.getNameNode() === identifier) return true;
    if (Node.isFunctionDeclaration(parent) && parent.getNameNode() === identifier) return true;
    if (Node.isMethodDeclaration(parent) && parent.getNameNode() === identifier) return true;
    if (Node.isClassDeclaration(parent) && parent.getNameNode() === identifier) return true;
    if (Node.isInterfaceDeclaration(parent) && parent.getNameNode() === identifier) return true;
    if (Node.isTypeAliasDeclaration(parent) && parent.getNameNode() === identifier) return true;
    if (Node.isEnumDeclaration(parent) && parent.getNameNode() === identifier) return true;
    return false;
  }

  private isWriteIdentifier(identifier: Node): boolean {
    const parent = identifier.getParent();
    if (!parent) return false;

    if (Node.isBinaryExpression(parent) && parent.getLeft() === identifier) {
      const operatorText = parent.getOperatorToken().getText();
      return operatorText.endsWith("=");
    }

    if ((Node.isPrefixUnaryExpression(parent) || Node.isPostfixUnaryExpression(parent)) && parent.getOperand() === identifier) {
      const operatorText = parent.getOperatorToken();
      return operatorText === SyntaxKind.PlusPlusToken || operatorText === SyntaxKind.MinusMinusToken;
    }

    return false;
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
}
