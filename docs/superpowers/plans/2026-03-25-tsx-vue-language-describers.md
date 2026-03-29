# TSX & Vue Language Describers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add language describers for `.tsx`/`.jsx` and `.vue` files that extract template-level AST structure (conditional rendering, loops, slots, component composition) as dedicated symbols in the logic graph, with NL descriptions.

**Architecture:** Three new source files — `templateWalker.ts` (shared template-to-symbols engine), `tsxDescriber.ts` (extends TypeScriptDescriber with JSX walk), `vueDescriber.ts` (parses Vue SFCs). Modified files: `languageDescriber.ts` (new types), `daemon.ts` (routing + rendering). The template walker receives an abstract `TemplateNode[]` so both TSX and Vue can share the same symbol/edge/NL generation.

**Tech Stack:** TypeScript, ts-morph (JSX walk), @vue/compiler-sfc + @vue/compiler-dom (Vue parsing), Vitest

**Spec:** `docs/superpowers/specs/2026-03-25-tsx-vue-language-describers-design.md`

---

### Task 1: Extend type system in languageDescriber.ts

**Files:**
- Modify: `src/ast/languageDescriber.ts`
- Test: `tests/languageDescriber.test.ts`

This task adds new symbol kinds (`tpl`, `tpl-slot`, `tpl-branch`, `tpl-loop`), edge kinds (`render`, `slot`), and updates `KIND_SORT_ORDER`. Template symbols populate `LogicSymbol` fields with sensible defaults: `params: []`, `complexity: "low"`, `control: { async: false, branch: false, await: false, throw: false }`, `exported: false`.

- [ ] **Step 1: Write failing test for new symbol kinds in sort order**

In `tests/languageDescriber.test.ts`, add a test that creates template symbols and verifies they sort after script symbols:

```typescript
it("sorts template symbol kinds after script kinds", () => {
  const tplSymbol: LogicSymbol = {
    id: "tpl:App", kind: "tpl", name: "App", exported: false,
    parentId: null, params: [], complexity: "low",
    control: { async: false, branch: false, await: false, throw: false }, line: 1,
  };
  const fnSymbol: LogicSymbol = {
    id: "fn:setup", kind: "fn", name: "setup", exported: true,
    parentId: null, params: [], complexity: "low",
    control: { async: false, branch: false, await: false, throw: false }, line: 1,
  };
  const sorted = sortSymbols([tplSymbol, fnSymbol]);
  expect(sorted[0].kind).toBe("fn");
  expect(sorted[1].kind).toBe("tpl");
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bun run test -- tests/languageDescriber.test.ts`
Expected: TypeScript compilation error — `"tpl"` is not assignable to `LogicSymbolKind`

- [ ] **Step 3: Update type definitions**

In `src/ast/languageDescriber.ts`:

```typescript
export type LogicSymbolKind = "cls" | "fn" | "mtd" | "var" | "iface" | "enum" | "type"
  | "tpl" | "tpl-slot" | "tpl-branch" | "tpl-loop";

export type LogicEdgeKind = "call" | "import" | "read" | "write" | "extends" | "implements"
  | "render" | "slot";
```

Update `KIND_SORT_ORDER`:

```typescript
export const KIND_SORT_ORDER: Record<LogicSymbolKind, number> = {
  cls: 0, fn: 1, mtd: 2, var: 3, iface: 4, enum: 5, type: 6,
  tpl: 7, "tpl-branch": 8, "tpl-loop": 9, "tpl-slot": 10,
};
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bun run test -- tests/languageDescriber.test.ts`
Expected: PASS

- [ ] **Step 5: Run typecheck**

Run: `bun run typecheck`
Expected: PASS (no type errors from other files — existing code doesn't use exhaustive switches on these unions)

- [ ] **Step 6: Commit**

```bash
git add src/ast/languageDescriber.ts tests/languageDescriber.test.ts
git commit -m "feat: add template symbol and edge kinds to type system"
```

---

### Task 2: Build the shared template walker

**Files:**
- Create: `src/ast/templateWalker.ts`
- Test: `tests/templateWalker.test.ts`

The template walker converts abstract `TemplateNode[]` into `LogicSymbol[]`, `LogicEdge[]`, and `Map<string, string>` (NL descriptions). It creates `tpl` root, `tpl-branch` for conditionals, `tpl-loop` for iterations, `tpl-slot` for slots, and `render`/`slot` edges to child components.

- [ ] **Step 1: Write failing test — basic component rendering**

Create `tests/templateWalker.test.ts`:

```typescript
import { describe, expect, it } from "vitest";
import { walkTemplate, type TemplateNode } from "../src/ast/templateWalker";

describe("walkTemplate", () => {
  it("creates tpl root and render edges for child components", () => {
    const nodes: TemplateNode[] = [
      {
        tag: "div", isComponent: false, directives: [], line: 1,
        children: [
          { tag: "Header", isComponent: true, directives: [], children: [], line: 2 },
          { tag: "Footer", isComponent: true, directives: [], children: [], line: 3 },
        ],
      },
    ];

    const result = walkTemplate("MyPage", nodes);

    const symbolIds = result.symbols.map(s => s.id);
    expect(symbolIds).toContain("tpl:MyPage");

    const edgeKeys = result.edges.map(e => `${e.kind}:${e.from}->${e.to}`);
    expect(edgeKeys).toContain("render:tpl:MyPage->type:Header");
    expect(edgeKeys).toContain("render:tpl:MyPage->type:Footer");
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bun run test -- tests/templateWalker.test.ts`
Expected: FAIL — module not found

- [ ] **Step 3: Write minimal implementation — root + render edges**

Create `src/ast/templateWalker.ts`:

```typescript
import type { LogicSymbol, LogicEdge, LogicSymbolKind } from "./languageDescriber";

export interface TemplateNode {
  tag: string;
  isComponent: boolean;
  directives: TemplateDirective[];
  children: TemplateNode[];
  line: number;
}

export interface TemplateDirective {
  kind: "if" | "else-if" | "else" | "for" | "show" | "is";
  expression: string;
}

export interface TemplateWalkResult {
  symbols: LogicSymbol[];
  edges: LogicEdge[];
  descriptions: Map<string, string>;
}

function makeTemplateSymbol(
  id: string,
  kind: LogicSymbolKind,
  name: string,
  parentId: string | null,
  line: number
): LogicSymbol {
  return {
    id, kind, name, exported: false, parentId, params: [],
    complexity: "low",
    control: { async: false, branch: false, await: false, throw: false },
    line,
  };
}

export function walkTemplate(componentName: string, nodes: TemplateNode[]): TemplateWalkResult {
  const symbols: LogicSymbol[] = [];
  const edges: LogicEdge[] = [];
  const descriptions = new Map<string, string>();
  const edgeSet = new Set<string>();

  const rootId = `tpl:${componentName}`;
  // Find the line of the first node, default to 1
  const rootLine = nodes.length > 0 ? nodes[0].line : 1;
  symbols.push(makeTemplateSymbol(rootId, "tpl", componentName, null, rootLine));

  const addEdge = (edge: LogicEdge) => {
    const key = `${edge.kind}|${edge.from}|${edge.to}`;
    if (!edgeSet.has(key)) {
      edgeSet.add(key);
      edges.push(edge);
    }
  };

  // Track rendered components for root description
  const rootComponents: string[] = [];
  const rootBranches: string[] = [];
  const rootLoops: string[] = [];

  walkNodes(nodes, rootId, componentName);

  // Build root description
  const descParts: string[] = [];
  if (rootComponents.length > 0) {
    descParts.push(`Renders ${rootComponents.map(c => `\`<${c}>\``).join(", ")}`);
  }
  if (rootBranches.length > 0) {
    descParts.push(rootBranches.join(". "));
  }
  if (rootLoops.length > 0) {
    descParts.push(rootLoops.join(". "));
  }
  if (descParts.length > 0) {
    descriptions.set(rootId, descParts.join(". "));
  }

  return { symbols, edges, descriptions };

  function walkNodes(templateNodes: TemplateNode[], parentSymbolId: string, ownerName: string) {
    for (let i = 0; i < templateNodes.length; i++) {
      const node = templateNodes[i];
      processNode(node, parentSymbolId, ownerName, i);
    }
  }

  function processNode(node: TemplateNode, parentSymbolId: string, ownerName: string, _index: number) {
    // Check for structural directives first
    const ifDir = node.directives.find(d => d.kind === "if");
    const elseIfDir = node.directives.find(d => d.kind === "else-if");
    const elseDir = node.directives.find(d => d.kind === "else");
    const forDir = node.directives.find(d => d.kind === "for");
    const showDir = node.directives.find(d => d.kind === "show");

    let currentParent = parentSymbolId;

    // Handle v-for — wraps the node in a loop symbol
    if (forDir) {
      const forExpr = forDir.expression;
      // Parse "item in items" or "(item, index) in items"
      const inMatch = forExpr.match(/(.+?)\s+in\s+(.+)/);
      const binding = inMatch ? inMatch[1].trim() : "item";
      const iterable = inMatch ? inMatch[2].trim() : forExpr;
      const safeName = iterable.replace(/[^a-zA-Z0-9_]/g, "_");
      const loopId = `tpl-loop:${ownerName}.for_${safeName}`;

      if (!symbols.find(s => s.id === loopId)) {
        symbols.push(makeTemplateSymbol(loopId, "tpl-loop", `for ${binding} in ${iterable}`, parentSymbolId, node.line));

        const desc = `Iterates over each \`${binding}\` in \`${iterable}\``;
        const childComponents = collectChildComponents(node);
        if (childComponents.length > 0) {
          descriptions.set(loopId, `${desc}, rendering ${childComponents.map(c => `\`<${c}>\``).join(", ")} for each`);
        } else {
          descriptions.set(loopId, desc);
        }

        if (parentSymbolId === rootId) {
          rootLoops.push(`Iterates over \`${iterable}\` rendering ${collectChildComponents(node).map(c => `\`<${c}>\``).join(", ") || "elements"} for each`);
        }
      }
      currentParent = loopId;
    }

    // Handle v-if
    if (ifDir) {
      const condExpr = ifDir.expression;
      const safeName = condExpr.replace(/[^a-zA-Z0-9_]/g, "_");
      const branchId = `tpl-branch:${ownerName}.if_${safeName}`;

      if (!symbols.find(s => s.id === branchId)) {
        symbols.push(makeTemplateSymbol(branchId, "tpl-branch", `if ${condExpr}`, currentParent, node.line));

        const childComponents = collectChildComponents(node);
        const rendersStr = childComponents.length > 0
          ? ` rendering ${childComponents.map(c => `\`<${c}>\``).join(", ")}`
          : node.isComponent ? ` rendering \`<${node.tag}>\`` : "";
        descriptions.set(branchId, `Conditionally renders${rendersStr} when \`${condExpr}\` is truthy`);

        if (parentSymbolId === rootId) {
          rootBranches.push(`Conditionally renders${rendersStr} when \`${condExpr}\` is truthy`);
        }
      }
      currentParent = branchId;
    }

    // Handle v-else-if
    if (elseIfDir) {
      const condExpr = elseIfDir.expression;
      const safeName = condExpr.replace(/[^a-zA-Z0-9_]/g, "_");
      const branchId = `tpl-branch:${ownerName}.elseif_${safeName}`;

      if (!symbols.find(s => s.id === branchId)) {
        symbols.push(makeTemplateSymbol(branchId, "tpl-branch", `else if ${condExpr}`, currentParent, node.line));

        const childComponents = collectChildComponents(node);
        const rendersStr = childComponents.length > 0
          ? ` rendering ${childComponents.map(c => `\`<${c}>\``).join(", ")}`
          : node.isComponent ? ` rendering \`<${node.tag}>\`` : "";
        descriptions.set(branchId, `Otherwise, if \`${condExpr}\`,${rendersStr}`);
      }
      currentParent = branchId;
    }

    // Handle v-else
    if (elseDir) {
      const branchId = `tpl-branch:${ownerName}.else`;

      if (!symbols.find(s => s.id === branchId)) {
        symbols.push(makeTemplateSymbol(branchId, "tpl-branch", "else", currentParent, node.line));

        const childComponents = collectChildComponents(node);
        const rendersStr = childComponents.length > 0
          ? ` renders ${childComponents.map(c => `\`<${c}>\``).join(", ")}`
          : node.isComponent ? ` renders \`<${node.tag}>\`` : "";
        descriptions.set(branchId, `Otherwise,${rendersStr}`);
      }
      currentParent = branchId;
    }

    // Handle v-show (treated as tpl-branch)
    if (showDir) {
      const condExpr = showDir.expression;
      const safeName = condExpr.replace(/[^a-zA-Z0-9_]/g, "_");
      const branchId = `tpl-branch:${ownerName}.show_${safeName}`;

      if (!symbols.find(s => s.id === branchId)) {
        symbols.push(makeTemplateSymbol(branchId, "tpl-branch", `show ${condExpr}`, currentParent, node.line));
        descriptions.set(branchId, `Toggles visibility when \`${condExpr}\` is truthy`);
      }
      currentParent = branchId;
    }

    // Handle slots
    if (node.tag === "slot") {
      const nameAttr = "default"; // simplified — Vue walker will pass the real name
      const slotId = `tpl-slot:${ownerName}.${nameAttr}`;

      if (!symbols.find(s => s.id === slotId)) {
        symbols.push(makeTemplateSymbol(slotId, "tpl-slot", nameAttr, currentParent, node.line));
        addEdge({ kind: "slot", from: currentParent, to: slotId });
        descriptions.set(slotId, nameAttr === "default"
          ? "Default slot"
          : `Named slot \`${nameAttr}\``);
      }
    }

    // Handle dynamic component (<component :is="...">)
    const isDir = node.directives.find(d => d.kind === "is");
    if (isDir) {
      addEdge({ kind: "render", from: currentParent, to: `type:dynamic(${isDir.expression})` });
    }

    // Component render edge
    if (node.isComponent) {
      addEdge({ kind: "render", from: currentParent, to: `type:${node.tag}` });
      if (currentParent === rootId) {
        rootComponents.push(node.tag);
      }
    }

    // Recurse children
    walkNodes(node.children, currentParent, ownerName);
  }

  function collectChildComponents(node: TemplateNode): string[] {
    const components: string[] = [];
    if (node.isComponent) components.push(node.tag);
    for (const child of node.children) {
      components.push(...collectChildComponents(child));
    }
    return components;
  }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bun run test -- tests/templateWalker.test.ts`
Expected: PASS

- [ ] **Step 5: Write test — v-if / v-else branching**

Add to `tests/templateWalker.test.ts`:

```typescript
it("creates tpl-branch symbols for v-if/v-else", () => {
  const nodes: TemplateNode[] = [
    {
      tag: "div", isComponent: false, directives: [], line: 1,
      children: [
        {
          tag: "Spinner", isComponent: true, line: 2,
          directives: [{ kind: "if", expression: "isLoading" }],
          children: [],
        },
        {
          tag: "Content", isComponent: true, line: 3,
          directives: [{ kind: "else", expression: "" }],
          children: [],
        },
      ],
    },
  ];

  const result = walkTemplate("MyPage", nodes);

  const symbolIds = result.symbols.map(s => s.id);
  expect(symbolIds).toContain("tpl-branch:MyPage.if_isLoading");
  expect(symbolIds).toContain("tpl-branch:MyPage.else");

  const edgeKeys = result.edges.map(e => `${e.kind}:${e.from}->${e.to}`);
  expect(edgeKeys).toContain("render:tpl-branch:MyPage.if_isLoading->type:Spinner");
  expect(edgeKeys).toContain("render:tpl-branch:MyPage.else->type:Content");

  const branchDesc = result.descriptions.get("tpl-branch:MyPage.if_isLoading");
  expect(branchDesc).toMatch(/isLoading/);
});
```

- [ ] **Step 6: Run test to verify it passes**

Run: `bun run test -- tests/templateWalker.test.ts`
Expected: PASS (implementation already handles this)

- [ ] **Step 7: Write test — v-for loop**

Add to `tests/templateWalker.test.ts`:

```typescript
it("creates tpl-loop symbols for v-for", () => {
  const nodes: TemplateNode[] = [
    {
      tag: "ul", isComponent: false, directives: [], line: 1,
      children: [
        {
          tag: "Card", isComponent: true, line: 2,
          directives: [{ kind: "for", expression: "item in items" }],
          children: [],
        },
      ],
    },
  ];

  const result = walkTemplate("List", nodes);

  const symbolIds = result.symbols.map(s => s.id);
  expect(symbolIds).toContain("tpl-loop:List.for_items");

  const edgeKeys = result.edges.map(e => `${e.kind}:${e.from}->${e.to}`);
  expect(edgeKeys).toContain("render:tpl-loop:List.for_items->type:Card");

  const loopDesc = result.descriptions.get("tpl-loop:List.for_items");
  expect(loopDesc).toMatch(/item/);
  expect(loopDesc).toMatch(/items/);
});
```

- [ ] **Step 8: Run test to verify it passes**

Run: `bun run test -- tests/templateWalker.test.ts`
Expected: PASS

- [ ] **Step 9: Write test — slots**

Add to `tests/templateWalker.test.ts`:

```typescript
it("creates tpl-slot symbols for slot elements", () => {
  const nodes: TemplateNode[] = [
    {
      tag: "div", isComponent: false, directives: [], line: 1,
      children: [
        { tag: "slot", isComponent: false, directives: [], children: [], line: 2 },
      ],
    },
  ];

  const result = walkTemplate("Layout", nodes);

  const symbolIds = result.symbols.map(s => s.id);
  expect(symbolIds).toContain("tpl-slot:Layout.default");

  const edgeKeys = result.edges.map(e => `${e.kind}:${e.from}->${e.to}`);
  expect(edgeKeys).toContain("slot:tpl:Layout->tpl-slot:Layout.default");
});
```

- [ ] **Step 10: Run test to verify it passes**

Run: `bun run test -- tests/templateWalker.test.ts`
Expected: PASS

- [ ] **Step 11: Run full test suite**

Run: `bun run test`
Expected: All pass — no regressions

- [ ] **Step 12: Commit**

```bash
git add src/ast/templateWalker.ts tests/templateWalker.test.ts
git commit -m "feat: add shared template walker for template-to-symbols conversion"
```

---

### Task 3: Build the TSX describer

**Files:**
- Create: `src/ast/tsxDescriber.ts`
- Modify: `src/ast/typescriptDescriber.ts` (make `extractRawLogicGraph` protected)
- Test: `tests/tsxDescriber.test.ts`

The TSX describer extends `TypeScriptDescriber`. After the parent extracts script symbols, it walks JSX return statements to find component renders, ternaries, `&&` short-circuits, and `.map()` calls, converting them to `TemplateNode[]` and passing to the shared walker.

- [ ] **Step 1: Write failing test — TSX component with conditional rendering**

Create `tests/tsxDescriber.test.ts`:

```typescript
import { describe, expect, it } from "vitest";
import { TsxDescriber } from "../src/ast/tsxDescriber";

describe("TsxDescriber", () => {
  const describer = new TsxDescriber();

  it("extracts script symbols AND template symbols from TSX", () => {
    const source = [
      "import React from 'react';",
      "import { Header } from './Header';",
      "import { Modal } from './Modal';",
      "",
      "export function App({ isOpen }: { isOpen: boolean }) {",
      "  return (",
      "    <div>",
      "      <Header />",
      "      {isOpen && <Modal />}",
      "    </div>",
      "  );",
      "}",
    ].join("\n");

    const result = describer.extractFileGraph("/tmp/test/App.tsx", source);

    // Script symbols still present
    const symbolIds = result.symbols.map(s => s.id);
    expect(symbolIds).toContain("fn:App");

    // Template symbols
    expect(symbolIds).toContain("tpl:App");
    expect(symbolIds).toContain("tpl-branch:App.if_isOpen");

    // Render edges
    const edgeKeys = result.edges.map(e => `${e.kind}:${e.from}->${e.to}`);
    expect(edgeKeys).toContain("render:tpl:App->type:Header");
    expect(edgeKeys).toContain("render:tpl-branch:App.if_isOpen->type:Modal");
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bun run test -- tests/tsxDescriber.test.ts`
Expected: FAIL — module not found

- [ ] **Step 3: Make `extractRawLogicGraph` protected in TypeScriptDescriber**

In `src/ast/typescriptDescriber.ts`, change line 92:

```typescript
// Change: private extractRawLogicGraph
// To:     protected extractRawLogicGraph
protected extractRawLogicGraph(sourceFile: SourceFile): RawLogicGraph {
```

Also change `RawLogicGraph` and `CallableSymbolRef` interfaces from file-private to exported:

```typescript
export interface RawLogicGraph { ... }
export interface CallableSymbolRef { ... }
```

- [ ] **Step 4: Write TsxDescriber implementation**

Create `src/ast/tsxDescriber.ts`. The describer:
1. Calls `super.extractFileGraph()` to get script results
2. Creates a new ts-morph `Project` and `SourceFile` for the same source
3. For each function/arrow function that returns JSX, walks JSX children
4. Converts JSX patterns to `TemplateNode[]`
5. Passes to `walkTemplate()`
6. Merges template symbols/edges/descriptions into the parent result

```typescript
import {
  Node,
  Project,
  SyntaxKind,
} from "ts-morph";
import { TypeScriptDescriber } from "./typescriptDescriber";
import type { FileGraphResult, LogicSymbol, LogicEdge } from "./languageDescriber";
import { walkTemplate, type TemplateNode, type TemplateDirective } from "./templateWalker";

export class TsxDescriber extends TypeScriptDescriber {
  override readonly languageId = "tsx";
  override readonly extensions: ReadonlySet<string> = new Set([".tsx", ".jsx"]);

  override extractFileGraph(filePath: string, sourceText: string): FileGraphResult {
    const baseResult = super.extractFileGraph(filePath, sourceText);

    // Re-parse for JSX walk (ts-morph SourceFile not exposed from parent)
    const project = new Project({
      compilerOptions: { allowJs: true, jsx: 2 /* React */ },
      useInMemoryFileSystem: true,
    });
    const sourceFile = project.createSourceFile(filePath, sourceText);

    const allSymbols: LogicSymbol[] = [...baseResult.symbols];
    const allEdges: LogicEdge[] = [...baseResult.edges];
    const allDescriptions = new Map(baseResult.symbolDescriptions);

    // Find functions/arrow functions that return JSX
    const fnDecls = sourceFile.getFunctions();
    for (const fn of fnDecls) {
      const fnName = fn.getName() ?? `anonymous_fn_${fn.getStartLineNumber()}`;
      if (!this.containsJsx(fn)) continue;
      const templateNodes = this.extractJsxNodes(fn, fnName);
      if (templateNodes.length === 0) continue;
      const walkResult = walkTemplate(fnName, templateNodes);
      allSymbols.push(...walkResult.symbols);
      allEdges.push(...walkResult.edges);
      for (const [k, v] of walkResult.descriptions) allDescriptions.set(k, v);
    }

    // Also check variable declarations with arrow functions (e.g. const App = () => <div/>)
    for (const varDecl of sourceFile.getVariableDeclarations()) {
      const init = varDecl.getInitializer();
      if (!init) continue;
      if (!Node.isArrowFunction(init) && !Node.isFunctionExpression(init)) continue;
      if (!this.containsJsx(init)) continue;
      const varName = varDecl.getName();
      const templateNodes = this.extractJsxNodes(init, varName);
      if (templateNodes.length === 0) continue;
      const walkResult = walkTemplate(varName, templateNodes);
      allSymbols.push(...walkResult.symbols);
      allEdges.push(...walkResult.edges);
      for (const [k, v] of walkResult.descriptions) allDescriptions.set(k, v);
    }

    return {
      symbols: allSymbols,
      edges: allEdges,
      imports: baseResult.imports,
      symbolDescriptions: allDescriptions,
      fileSummary: baseResult.fileSummary,
    };
  }

  private containsJsx(node: Node): boolean {
    return node.getDescendantsOfKind(SyntaxKind.JsxElement).length > 0
      || node.getDescendantsOfKind(SyntaxKind.JsxSelfClosingElement).length > 0
      || node.getDescendantsOfKind(SyntaxKind.JsxFragment).length > 0;
  }

  private extractJsxNodes(node: Node, _componentName: string): TemplateNode[] {
    // Find the top-level JSX return — look at return statements or expression body
    const jsxRoots: Node[] = [];

    // For arrow functions with expression body: () => <div/>
    if (Node.isArrowFunction(node)) {
      const body = node.getBody();
      if (body && this.isJsxNode(body)) {
        jsxRoots.push(body);
      } else if (body && Node.isParenthesizedExpression(body)) {
        const inner = body.getExpression();
        if (this.isJsxNode(inner)) {
          jsxRoots.push(inner);
        }
      }
    }

    // For functions/arrow functions with block body: look at return statements
    if (jsxRoots.length === 0) {
      const returns = node.getDescendantsOfKind(SyntaxKind.ReturnStatement);
      for (const ret of returns) {
        for (const child of ret.getChildren()) {
          if (this.isJsxNode(child)) {
            jsxRoots.push(child);
          } else if (child.getKind() === SyntaxKind.ParenthesizedExpression) {
            const inner = (child as any).getExpression();
            if (inner && this.isJsxNode(inner)) {
              jsxRoots.push(inner);
            }
          }
        }
      }
    }

    const templateNodes: TemplateNode[] = [];
    for (const root of jsxRoots) {
      const converted = this.jsxToTemplateNodes(root);
      templateNodes.push(...converted);
    }
    return templateNodes;
  }

  private isJsxNode(node: Node): boolean {
    const kind = node.getKind();
    return kind === SyntaxKind.JsxElement
      || kind === SyntaxKind.JsxSelfClosingElement
      || kind === SyntaxKind.JsxFragment;
  }

  private jsxToTemplateNodes(node: Node): TemplateNode[] {
    const kind = node.getKind();

    if (kind === SyntaxKind.JsxSelfClosingElement) {
      const tag = (node as any).getTagNameNode().getText();
      return [{
        tag,
        isComponent: this.isComponentTag(tag),
        directives: [],
        children: [],
        line: node.getStartLineNumber(),
      }];
    }

    if (kind === SyntaxKind.JsxElement) {
      const openTag = (node as any).getOpeningElement();
      const tag = openTag.getTagNameNode().getText();
      const children = this.extractJsxChildren(node);
      return [{
        tag,
        isComponent: this.isComponentTag(tag),
        directives: [],
        children,
        line: node.getStartLineNumber(),
      }];
    }

    if (kind === SyntaxKind.JsxFragment) {
      return this.extractJsxChildren(node);
    }

    return [];
  }

  private extractJsxChildren(parent: Node): TemplateNode[] {
    const children: TemplateNode[] = [];

    for (const child of parent.getChildren()) {
      const childKind = child.getKind();

      if (childKind === SyntaxKind.JsxElement || childKind === SyntaxKind.JsxSelfClosingElement) {
        children.push(...this.jsxToTemplateNodes(child));
      } else if (childKind === SyntaxKind.JsxExpression) {
        // Unwrap {expression} containers
        const expr = (child as any).getExpression?.();
        if (expr) {
          children.push(...this.jsxExpressionToTemplateNodes(expr));
        }
      } else if (childKind === SyntaxKind.JsxFragment) {
        children.push(...this.extractJsxChildren(child));
      } else if (childKind === SyntaxKind.SyntaxList) {
        // JsxElement children are wrapped in a SyntaxList
        children.push(...this.extractJsxChildren(child));
      }
    }

    return children;
  }

  private jsxExpressionToTemplateNodes(expr: Node): TemplateNode[] {
    const exprKind = expr.getKind();

    // {condition && <X/>} — short-circuit AND
    if (exprKind === SyntaxKind.BinaryExpression) {
      const binary = expr as any;
      const op = binary.getOperatorToken().getText();
      if (op === "&&") {
        const left = binary.getLeft();
        const right = binary.getRight();
        const condition = left.getText();
        const innerNodes = this.jsxOrExprToTemplateNodes(right);

        // Wrap inner nodes with an if directive
        return [{
          tag: "template",
          isComponent: false,
          directives: [{ kind: "if" as const, expression: condition }],
          children: innerNodes,
          line: expr.getStartLineNumber(),
        }];
      }
    }

    // {condition ? <X/> : <Y/>} — ternary
    if (exprKind === SyntaxKind.ConditionalExpression) {
      const cond = expr as any;
      const condition = cond.getCondition().getText();
      const whenTrue = cond.getWhenTrue();
      const whenFalse = cond.getWhenFalse();

      const trueNodes = this.jsxOrExprToTemplateNodes(whenTrue);
      const falseNodes = this.jsxOrExprToTemplateNodes(whenFalse);

      const result: TemplateNode[] = [];
      result.push({
        tag: "template",
        isComponent: false,
        directives: [{ kind: "if" as const, expression: condition }],
        children: trueNodes,
        line: whenTrue.getStartLineNumber(),
      });
      if (falseNodes.length > 0) {
        result.push({
          tag: "template",
          isComponent: false,
          directives: [{ kind: "else" as const, expression: "" }],
          children: falseNodes,
          line: whenFalse.getStartLineNumber(),
        });
      }
      return result;
    }

    // {items.map(item => <X/>)} — map call
    if (exprKind === SyntaxKind.CallExpression) {
      const call = expr as any;
      const calleeExpr = call.getExpression();
      if (Node.isPropertyAccessExpression(calleeExpr) && calleeExpr.getName() === "map") {
        const iterableText = calleeExpr.getExpression().getText();
        const args = call.getArguments();
        if (args.length > 0) {
          const mapFn = args[0];
          // Extract parameter name
          let binding = "item";
          if (Node.isArrowFunction(mapFn) || Node.isFunctionExpression(mapFn)) {
            const params = mapFn.getParameters();
            if (params.length > 0) {
              binding = params[0].getName();
            }
          }
          // Find JSX inside the map callback
          const innerJsx = this.findJsxInNode(mapFn);
          const innerNodes = innerJsx.flatMap(j => this.jsxToTemplateNodes(j));

          return [{
            tag: "template",
            isComponent: false,
            directives: [{ kind: "for" as const, expression: `${binding} in ${iterableText}` }],
            children: innerNodes,
            line: expr.getStartLineNumber(),
          }];
        }
      }
    }

    // Parenthesized — unwrap
    if (exprKind === SyntaxKind.ParenthesizedExpression) {
      const inner = (expr as any).getExpression();
      return this.jsxExpressionToTemplateNodes(inner);
    }

    // Direct JSX in expression
    if (this.isJsxNode(expr)) {
      return this.jsxToTemplateNodes(expr);
    }

    return [];
  }

  private jsxOrExprToTemplateNodes(node: Node): TemplateNode[] {
    if (this.isJsxNode(node)) {
      return this.jsxToTemplateNodes(node);
    }
    // Might be a parenthesized JSX
    if (node.getKind() === SyntaxKind.ParenthesizedExpression) {
      const inner = (node as any).getExpression();
      if (inner) return this.jsxOrExprToTemplateNodes(inner);
    }
    return this.jsxExpressionToTemplateNodes(node);
  }

  private findJsxInNode(node: Node): Node[] {
    const results: Node[] = [];
    // Check direct children and descendants for JSX
    for (const desc of node.getDescendants()) {
      if (this.isJsxNode(desc)) {
        // Only take top-level JSX (not nested inside other JSX)
        const parent = desc.getParent();
        if (parent && this.isJsxNode(parent)) continue;
        if (parent?.getKind() === SyntaxKind.SyntaxList) {
          const grandparent = parent.getParent();
          if (grandparent && this.isJsxNode(grandparent)) continue;
        }
        results.push(desc);
      }
    }
    return results;
  }

  private isComponentTag(tag: string): boolean {
    // PascalCase: starts with uppercase letter
    if (/^[A-Z]/.test(tag)) return true;
    // Dotted access: motion.div, React.Fragment
    if (tag.includes(".")) return true;
    return false;
  }
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `bun run test -- tests/tsxDescriber.test.ts`
Expected: PASS

- [ ] **Step 6: Write test — ternary rendering**

Add to `tests/tsxDescriber.test.ts`:

```typescript
it("extracts ternary as if/else branches", () => {
  const source = [
    "export function Page({ loading }: { loading: boolean }) {",
    "  return (",
    "    <div>",
    "      {loading ? <Spinner /> : <Content />}",
    "    </div>",
    "  );",
    "}",
  ].join("\n");

  const result = describer.extractFileGraph("/tmp/test/Page.tsx", source);
  const symbolIds = result.symbols.map(s => s.id);

  expect(symbolIds).toContain("tpl-branch:Page.if_loading");
  expect(symbolIds).toContain("tpl-branch:Page.else");

  const edgeKeys = result.edges.map(e => `${e.kind}:${e.from}->${e.to}`);
  expect(edgeKeys).toContain("render:tpl-branch:Page.if_loading->type:Spinner");
  expect(edgeKeys).toContain("render:tpl-branch:Page.else->type:Content");
});
```

- [ ] **Step 7: Run test to verify it passes**

Run: `bun run test -- tests/tsxDescriber.test.ts`
Expected: PASS

- [ ] **Step 8: Write test — .map() iteration**

Add to `tests/tsxDescriber.test.ts`:

```typescript
it("extracts .map() as tpl-loop", () => {
  const source = [
    "export function UserList({ users }: { users: any[] }) {",
    "  return (",
    "    <ul>",
    "      {users.map(u => <UserCard key={u.id} />)}",
    "    </ul>",
    "  );",
    "}",
  ].join("\n");

  const result = describer.extractFileGraph("/tmp/test/UserList.tsx", source);
  const symbolIds = result.symbols.map(s => s.id);

  expect(symbolIds).toContain("tpl-loop:UserList.for_users");

  const edgeKeys = result.edges.map(e => `${e.kind}:${e.from}->${e.to}`);
  expect(edgeKeys).toContain("render:tpl-loop:UserList.for_users->type:UserCard");
});
```

- [ ] **Step 9: Run test to verify it passes**

Run: `bun run test -- tests/tsxDescriber.test.ts`
Expected: PASS

- [ ] **Step 10: Write test — arrow function component**

Add to `tests/tsxDescriber.test.ts`:

```typescript
it("handles arrow function components", () => {
  const source = [
    "export const Card = ({ title }: { title: string }) => (",
    "  <div>",
    "    <Header />",
    "  </div>",
    ");",
  ].join("\n");

  const result = describer.extractFileGraph("/tmp/test/Card.tsx", source);
  const symbolIds = result.symbols.map(s => s.id);

  expect(symbolIds).toContain("tpl:Card");

  const edgeKeys = result.edges.map(e => `${e.kind}:${e.from}->${e.to}`);
  expect(edgeKeys).toContain("render:tpl:Card->type:Header");
});
```

- [ ] **Step 11: Run test to verify it passes**

Run: `bun run test -- tests/tsxDescriber.test.ts`
Expected: PASS

- [ ] **Step 12: Run full test suite (including existing TS describer tests)**

Run: `bun run test`
Expected: All pass — no regressions. TypeScript describer tests unchanged since `extractRawLogicGraph` access modifier change is non-breaking.

- [ ] **Step 13: Commit**

```bash
git add src/ast/tsxDescriber.ts src/ast/typescriptDescriber.ts tests/tsxDescriber.test.ts
git commit -m "feat: add TSX describer with JSX template extraction"
```

---

### Task 4: Build the Vue describer

**Files:**
- Create: `src/ast/vueDescriber.ts`
- Test: `tests/vueDescriber.test.ts`

The Vue describer parses `.vue` SFCs using `@vue/compiler-sfc`. It extracts `<script setup>` content via `TypeScriptDescriber` and `<template>` AST via `@vue/compiler-dom`, converts Vue template nodes to `TemplateNode[]`, and passes to the shared walker.

- [ ] **Step 1: Install @vue/compiler-sfc**

Run: `bun add @vue/compiler-sfc`

- [ ] **Step 2: Write failing test — basic Vue SFC**

Create `tests/vueDescriber.test.ts`:

```typescript
import { describe, expect, it } from "vitest";
import { VueDescriber } from "../src/ast/vueDescriber";

describe("VueDescriber", () => {
  const describer = new VueDescriber();

  it("extracts script and template symbols from Vue SFC", () => {
    const source = [
      "<script setup lang=\"ts\">",
      "import Header from './Header.vue';",
      "import Modal from './Modal.vue';",
      "import { ref } from 'vue';",
      "",
      "const isOpen = ref(false);",
      "",
      "function toggle() {",
      "  isOpen.value = !isOpen.value;",
      "}",
      "</script>",
      "",
      "<template>",
      "  <div>",
      "    <Header />",
      "    <Modal v-if=\"isOpen\" />",
      "  </div>",
      "</template>",
    ].join("\n");

    const result = describer.extractFileGraph("/tmp/test/App.vue", source);

    // Script symbols
    const symbolIds = result.symbols.map(s => s.id);
    expect(symbolIds).toContain("fn:toggle");
    expect(symbolIds).toContain("var:isOpen");

    // Template symbols
    expect(symbolIds).toContain("tpl:App");
    expect(symbolIds).toContain("tpl-branch:App.if_isOpen");

    // Render edges
    const edgeKeys = result.edges.map(e => `${e.kind}:${e.from}->${e.to}`);
    expect(edgeKeys).toContain("render:tpl:App->type:Header");
    expect(edgeKeys).toContain("render:tpl-branch:App.if_isOpen->type:Modal");
  });
});
```

- [ ] **Step 3: Run test to verify it fails**

Run: `bun run test -- tests/vueDescriber.test.ts`
Expected: FAIL — module not found

- [ ] **Step 4: Write VueDescriber implementation**

Create `src/ast/vueDescriber.ts`:

```typescript
import { parse as parseSFC } from "@vue/compiler-sfc";
import { parse as parseTemplate, type RootNode, type TemplateChildNode, type ElementNode, type DirectiveNode } from "@vue/compiler-dom";
import { TypeScriptDescriber } from "./typescriptDescriber";
import type { LanguageDescriber, FileGraphResult, LogicSymbol, LogicEdge } from "./languageDescriber";
import { walkTemplate, type TemplateNode, type TemplateDirective } from "./templateWalker";
import * as path from "path";

export class VueDescriber implements LanguageDescriber {
  readonly languageId = "vue";
  readonly extensions: ReadonlySet<string> = new Set([".vue"]);

  private readonly tsDescriber = new TypeScriptDescriber();

  extractFileGraph(filePath: string, sourceText: string): FileGraphResult {
    const componentName = path.basename(filePath, path.extname(filePath));
    const sfc = parseSFC(sourceText, { filename: filePath });

    // Extract script
    let scriptResult: FileGraphResult | null = null;
    const scriptSetup = sfc.descriptor.scriptSetup;
    const importedComponents = new Set<string>();

    if (scriptSetup) {
      const scriptContent = scriptSetup.content;
      // Give it a .ts extension so ts-morph parses it as TS
      const scriptPath = filePath.replace(/\.vue$/, ".setup.ts");
      scriptResult = this.tsDescriber.extractFileGraph(scriptPath, scriptContent);

      // Scan imports for component names (PascalCase)
      for (const imp of scriptResult.imports) {
        // The import specifier itself won't tell us the name,
        // so we parse the script for import declarations
        const importRegex = /import\s+(\w+)|import\s+\{([^}]+)\}/g;
        let match;
        while ((match = importRegex.exec(scriptContent)) !== null) {
          const defaultImport = match[1];
          const namedImports = match[2];
          if (defaultImport && /^[A-Z]/.test(defaultImport)) {
            importedComponents.add(defaultImport);
          }
          if (namedImports) {
            for (const name of namedImports.split(",")) {
              const trimmed = name.trim().split(/\s+as\s+/).pop()!.trim();
              if (/^[A-Z]/.test(trimmed)) {
                importedComponents.add(trimmed);
              }
            }
          }
        }
      }
    }

    // Extract template
    let templateSymbols: LogicSymbol[] = [];
    let templateEdges: LogicEdge[] = [];
    const templateDescriptions = new Map<string, string>();

    if (sfc.descriptor.template) {
      const templateContent = sfc.descriptor.template.content;
      const templateAst = parseTemplate(templateContent);
      const templateNodes = this.convertVueAst(templateAst.children, importedComponents);
      const walkResult = walkTemplate(componentName, templateNodes);
      templateSymbols = walkResult.symbols;
      templateEdges = walkResult.edges;
      for (const [k, v] of walkResult.descriptions) templateDescriptions.set(k, v);
    }

    // Merge
    const symbols = [...(scriptResult?.symbols ?? []), ...templateSymbols];
    const edges = [...(scriptResult?.edges ?? []), ...templateEdges];
    const descriptions = new Map(scriptResult?.symbolDescriptions ?? []);
    for (const [k, v] of templateDescriptions) descriptions.set(k, v);
    const imports = scriptResult?.imports ?? [];

    // Build file summary
    const exportedSymbols = symbols.filter(s => s.exported);
    const tplRoot = templateSymbols.find(s => s.kind === "tpl");
    let fileSummary: string;
    if (exportedSymbols.length === 0 && !tplRoot) {
      fileSummary = "Empty Vue SFC.";
    } else {
      const parts: string[] = [];
      if (exportedSymbols.length > 0) {
        parts.push(`${exportedSymbols.length} exported symbols`);
      }
      if (tplRoot) {
        const tplDesc = templateDescriptions.get(tplRoot.id);
        if (tplDesc) {
          parts.push(`template: ${tplDesc}`);
        }
      }
      fileSummary = `Vue SFC with ${parts.join(", ")}.`;
    }

    return {
      symbols,
      edges,
      imports,
      symbolDescriptions: descriptions,
      fileSummary,
    };
  }

  private convertVueAst(
    children: TemplateChildNode[],
    importedComponents: Set<string>
  ): TemplateNode[] {
    const result: TemplateNode[] = [];
    for (const child of children) {
      if (child.type === 1 /* NodeTypes.ELEMENT */) {
        result.push(this.convertElement(child as ElementNode, importedComponents));
      }
      // Skip text nodes, comments, interpolations — not structurally relevant
    }
    return result;
  }

  private convertElement(
    el: ElementNode,
    importedComponents: Set<string>
  ): TemplateNode {
    const tag = el.tag;
    const isComponent = /^[A-Z]/.test(tag)
      || tag.includes(".")
      || importedComponents.has(tag)
      || el.tagType === 1; /* COMPONENT */

    const directives: TemplateDirective[] = [];

    for (const prop of el.props) {
      if (prop.type === 7 /* NodeTypes.DIRECTIVE */) {
        const dir = prop as DirectiveNode;
        const dirName = dir.name;
        const expression = dir.exp?.type === 4 /* NodeTypes.SIMPLE_EXPRESSION */
          ? (dir.exp as any).content ?? ""
          : dir.exp?.loc?.source ?? "";

        if (dirName === "if") {
          directives.push({ kind: "if", expression });
        } else if (dirName === "else-if") {
          directives.push({ kind: "else-if", expression });
        } else if (dirName === "else") {
          directives.push({ kind: "else", expression: "" });
        } else if (dirName === "for") {
          directives.push({ kind: "for", expression });
        } else if (dirName === "show") {
          directives.push({ kind: "show", expression });
        } else if (dirName === "bind" && dir.arg?.type === 4 && (dir.arg as any).content === "is") {
          directives.push({ kind: "is", expression });
        }
      }
    }

    // Handle slot — check if it's a named slot via the name attribute
    // The walker handles "slot" tags generically, but we can enhance slot name detection here
    // by modifying the tag or adding it as part of the TemplateNode

    const children = this.convertVueAst(el.children, importedComponents);

    return {
      tag,
      isComponent,
      directives,
      children,
      line: el.loc.start.line,
    };
  }
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `bun run test -- tests/vueDescriber.test.ts`
Expected: PASS

- [ ] **Step 6: Write test — v-for iteration**

Add to `tests/vueDescriber.test.ts`:

```typescript
it("extracts v-for as tpl-loop", () => {
  const source = [
    "<script setup lang=\"ts\">",
    "import Card from './Card.vue';",
    "const items = [1, 2, 3];",
    "</script>",
    "",
    "<template>",
    "  <div>",
    "    <Card v-for=\"item in items\" :key=\"item\" />",
    "  </div>",
    "</template>",
  ].join("\n");

  const result = describer.extractFileGraph("/tmp/test/List.vue", source);
  const symbolIds = result.symbols.map(s => s.id);

  expect(symbolIds).toContain("tpl-loop:List.for_items");

  const edgeKeys = result.edges.map(e => `${e.kind}:${e.from}->${e.to}`);
  expect(edgeKeys).toContain("render:tpl-loop:List.for_items->type:Card");
});
```

- [ ] **Step 7: Run test to verify it passes**

Run: `bun run test -- tests/vueDescriber.test.ts`
Expected: PASS

- [ ] **Step 8: Write test — v-if/v-else-if/v-else chain**

Add to `tests/vueDescriber.test.ts`:

```typescript
it("extracts v-if/v-else-if/v-else chain", () => {
  const source = [
    "<template>",
    "  <div>",
    "    <Spinner v-if=\"loading\" />",
    "    <Error v-else-if=\"error\" />",
    "    <Content v-else />",
    "  </div>",
    "</template>",
  ].join("\n");

  const result = describer.extractFileGraph("/tmp/test/Status.vue", source);
  const symbolIds = result.symbols.map(s => s.id);

  expect(symbolIds).toContain("tpl-branch:Status.if_loading");
  expect(symbolIds).toContain("tpl-branch:Status.elseif_error");
  expect(symbolIds).toContain("tpl-branch:Status.else");
});
```

- [ ] **Step 9: Run test to verify it passes**

Run: `bun run test -- tests/vueDescriber.test.ts`
Expected: PASS

- [ ] **Step 10: Write test — slots**

Add to `tests/vueDescriber.test.ts`:

```typescript
it("extracts slot definitions", () => {
  const source = [
    "<template>",
    "  <div>",
    "    <slot />",
    "  </div>",
    "</template>",
  ].join("\n");

  const result = describer.extractFileGraph("/tmp/test/Layout.vue", source);
  const symbolIds = result.symbols.map(s => s.id);

  expect(symbolIds).toContain("tpl-slot:Layout.default");

  const edgeKeys = result.edges.map(e => `${e.kind}:${e.from}->${e.to}`);
  expect(edgeKeys).toContain("slot:tpl:Layout->tpl-slot:Layout.default");
});
```

- [ ] **Step 11: Run test to verify it passes**

Run: `bun run test -- tests/vueDescriber.test.ts`
Expected: PASS

- [ ] **Step 12: Write test — template-only SFC (no script)**

Add to `tests/vueDescriber.test.ts`:

```typescript
it("handles template-only SFC without script", () => {
  const source = [
    "<template>",
    "  <div>",
    "    <Header />",
    "  </div>",
    "</template>",
  ].join("\n");

  const result = describer.extractFileGraph("/tmp/test/Simple.vue", source);
  const symbolIds = result.symbols.map(s => s.id);

  expect(symbolIds).toContain("tpl:Simple");
  expect(result.edges.some(e => e.kind === "render" && e.to === "type:Header")).toBe(true);
});
```

- [ ] **Step 13: Run full test suite**

Run: `bun run test`
Expected: All pass

- [ ] **Step 14: Commit**

```bash
git add src/ast/vueDescriber.ts tests/vueDescriber.test.ts
git commit -m "feat: add Vue SFC describer with template + script setup extraction"
```

---

### Task 5: Integrate into daemon

**Files:**
- Modify: `src/daemon.ts`
- Test: `tests/daemon.test.ts`

Update the daemon to route files to the correct describer by extension, add `.vue` to supported extensions, and add template symbol kinds to NL rendering.

- [ ] **Step 1: Write failing test — daemon processes .vue files**

Add to `tests/daemon.test.ts`:

```typescript
it("processes .vue files and stores template symbols", async () => {
  const tempDir = makeTempDir();
  const filePath = path.join(tempDir, "App.vue");
  const code = source([
    "<script setup lang=\"ts\">",
    "import Header from './Header.vue';",
    "const show = true;",
    "</script>",
    "",
    "<template>",
    "  <div>",
    "    <Header v-if=\"show\" />",
    "  </div>",
    "</template>",
  ]);
  fs.writeFileSync(filePath, code, "utf8");

  const manager = createManagerStub();
  const daemon = new CodebaseDaemon(manager as any, "test-project", tempDir);
  await daemon.start();
  await daemon.stop();

  expect(manager.upsertFileContextNode).toHaveBeenCalled();
  const call = manager.upsertFileContextNode.mock.calls[0];
  const overview: string = call[3]; // overviewText
  expect(overview).toMatch(/tpl/);
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bun run test -- tests/daemon.test.ts`
Expected: FAIL — daemon doesn't process .vue files

- [ ] **Step 3: Update daemon.ts**

Changes:
1. Import `TsxDescriber` and `VueDescriber`
2. Add `.vue` to `SUPPORTED_EXTENSIONS`
3. Replace single `describer` field with a `Map<string, LanguageDescriber>` keyed by extension
4. Update `summarizeSourceFile` to pick the right describer
5. Add `.vue` to chokidar glob
6. Add template kinds to `kindLabel` and `kindGroupOrder`
7. Add template kind boosts to `symbolScore`

In `src/daemon.ts`:

Update imports:
```typescript
import { TsxDescriber } from "./ast/tsxDescriber";
import { VueDescriber } from "./ast/vueDescriber";
import type { LanguageDescriber } from "./ast/languageDescriber";
```

Update `SUPPORTED_EXTENSIONS`:
```typescript
const SUPPORTED_EXTENSIONS = new Set([".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".vue"]);
```

Replace the `describer` field:
```typescript
private readonly describerMap: Map<string, LanguageDescriber>;
```

In constructor:
```typescript
const tsDescriber = new TypeScriptDescriber();
const tsxDescriber = new TsxDescriber();
const vueDescriber = new VueDescriber();
this.describerMap = new Map([
  [".ts", tsDescriber],
  [".js", tsDescriber],
  [".mjs", tsDescriber],
  [".cjs", tsDescriber],
  [".tsx", tsxDescriber],
  [".jsx", tsxDescriber],
  [".vue", vueDescriber],
]);
```

Update `summarizeSourceFile` to pick describer by extension:
```typescript
const ext = path.extname(filePath).toLowerCase();
const describer = this.describerMap.get(ext);
if (!describer) return { abstractText: "", overviewText: "", nlContent: "", logicGraphMetadata: {} };
const result = describer.extractFileGraph(filePath, sourceText);
```

Update chokidar glob:
```typescript
this.watcher = chokidar.watch(`${this.watchDir}/**/*.{ts,tsx,js,jsx,mjs,cjs,vue}`, {
```

Update `kindGroupOrder` in `buildNLContent`:
```typescript
const kindGroupOrder: LogicSymbolKind[] = [
  "cls", "fn", "mtd", "tpl", "tpl-branch", "tpl-loop", "tpl-slot", "var", "iface", "enum", "type"
];
```

Update `kindLabel`:
```typescript
const kindLabel: Record<LogicSymbolKind, string> = {
  cls: "Class", fn: "Function", mtd: "Method",
  var: "Variable", iface: "Interface", enum: "Enum", type: "Type",
  tpl: "Template", "tpl-branch": "Branch", "tpl-loop": "Loop", "tpl-slot": "Slot",
};
```

Update `symbolScore` kindBoost:
```typescript
const kindBoost: Record<LogicSymbolKind, number> = {
  cls: 80, fn: 70, mtd: 60, var: 40, iface: 35, enum: 35, type: 30,
  tpl: 75, "tpl-branch": 50, "tpl-loop": 50, "tpl-slot": 45,
};
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bun run test -- tests/daemon.test.ts`
Expected: PASS

- [ ] **Step 5: Write test — daemon processes .tsx files with template symbols**

Add to `tests/daemon.test.ts`:

```typescript
it("processes .tsx files with template symbols", async () => {
  const tempDir = makeTempDir();
  const filePath = path.join(tempDir, "Page.tsx");
  const code = source([
    "export function Page({ items }: { items: any[] }) {",
    "  return (",
    "    <div>",
    "      {items.map(i => <Card key={i.id} />)}",
    "    </div>",
    "  );",
    "}",
  ]);
  fs.writeFileSync(filePath, code, "utf8");

  const manager = createManagerStub();
  const daemon = new CodebaseDaemon(manager as any, "test-project", tempDir);
  await daemon.start();
  await daemon.stop();

  expect(manager.upsertFileContextNode).toHaveBeenCalled();
  const call = manager.upsertFileContextNode.mock.calls[0];
  const overview: string = call[3];
  expect(overview).toMatch(/tpl-loop/);
});
```

- [ ] **Step 6: Run test to verify it passes**

Run: `bun run test -- tests/daemon.test.ts`
Expected: PASS

- [ ] **Step 7: Run full test suite + typecheck + lint**

Run: `bun run typecheck && bun run lint && bun run test`
Expected: All pass

- [ ] **Step 8: Commit**

```bash
git add src/daemon.ts tests/daemon.test.ts
git commit -m "feat: integrate TSX and Vue describers into daemon with extension routing"
```

---

### Task 6: Handle named slots in template walker

**Files:**
- Modify: `src/ast/templateWalker.ts`
- Modify: `src/ast/vueDescriber.ts`
- Test: `tests/templateWalker.test.ts`
- Test: `tests/vueDescriber.test.ts`

The current walker treats all `<slot>` elements as "default". Named slots need the slot name passed through.

- [ ] **Step 1: Write failing test — named slot**

Add to `tests/templateWalker.test.ts`:

```typescript
it("creates named slot symbols", () => {
  const nodes: TemplateNode[] = [
    {
      tag: "div", isComponent: false, directives: [], line: 1,
      children: [
        { tag: "slot", isComponent: false, directives: [], children: [], line: 2, slotName: "header" },
        { tag: "slot", isComponent: false, directives: [], children: [], line: 3 },
      ],
    },
  ];

  const result = walkTemplate("Layout", nodes);
  const symbolIds = result.symbols.map(s => s.id);
  expect(symbolIds).toContain("tpl-slot:Layout.header");
  expect(symbolIds).toContain("tpl-slot:Layout.default");
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bun run test -- tests/templateWalker.test.ts`
Expected: FAIL — `slotName` not on type

- [ ] **Step 3: Add `slotName` to TemplateNode and update walker**

In `src/ast/templateWalker.ts`, add optional `slotName` field:

```typescript
export interface TemplateNode {
  tag: string;
  isComponent: boolean;
  directives: TemplateDirective[];
  children: TemplateNode[];
  line: number;
  slotName?: string;  // for <slot name="...">
}
```

Update slot handling in `processNode`:

```typescript
if (node.tag === "slot") {
  const nameAttr = node.slotName ?? "default";
  const slotId = `tpl-slot:${ownerName}.${nameAttr}`;
  // ... rest unchanged, uses nameAttr
```

- [ ] **Step 4: Update VueDescriber to pass slot name**

In `src/ast/vueDescriber.ts`, in `convertElement`, after building directives, detect the slot name:

```typescript
// Detect slot name from static props
let slotName: string | undefined;
if (tag === "slot") {
  for (const prop of el.props) {
    if (prop.type === 6 /* NodeTypes.ATTRIBUTE */ && prop.name === "name" && prop.value) {
      slotName = prop.value.content;
    }
  }
}

return {
  tag, isComponent, directives, children,
  line: el.loc.start.line,
  slotName,
};
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `bun run test -- tests/templateWalker.test.ts tests/vueDescriber.test.ts`
Expected: PASS

- [ ] **Step 6: Add named slot test for Vue**

Add to `tests/vueDescriber.test.ts`:

```typescript
it("extracts named slots", () => {
  const source = [
    "<template>",
    "  <div>",
    "    <slot name=\"header\" />",
    "    <slot />",
    "  </div>",
    "</template>",
  ].join("\n");

  const result = describer.extractFileGraph("/tmp/test/Layout.vue", source);
  const symbolIds = result.symbols.map(s => s.id);

  expect(symbolIds).toContain("tpl-slot:Layout.header");
  expect(symbolIds).toContain("tpl-slot:Layout.default");
});
```

- [ ] **Step 7: Run full test suite**

Run: `bun run test`
Expected: All pass

- [ ] **Step 8: Commit**

```bash
git add src/ast/templateWalker.ts src/ast/vueDescriber.ts tests/templateWalker.test.ts tests/vueDescriber.test.ts
git commit -m "feat: support named slots in template walker and Vue describer"
```

---

### Task 7: Error handling for malformed SFCs and edge cases

**Files:**
- Modify: `src/ast/vueDescriber.ts`
- Modify: `src/ast/tsxDescriber.ts`
- Test: `tests/vueDescriber.test.ts`
- Test: `tests/tsxDescriber.test.ts`

Gracefully handle parse failures — malformed Vue templates, invalid JSX, etc. Fall back to script-only extraction (or empty result) rather than crashing the daemon.

- [ ] **Step 1: Write failing test — malformed Vue template**

Add to `tests/vueDescriber.test.ts`:

```typescript
it("handles malformed template gracefully", () => {
  const source = [
    "<script setup>",
    "const x = 1;",
    "</script>",
    "<template>",
    "  <div>",
    "    <unclosed-tag",
    "</template>",
  ].join("\n");

  // Should not throw — falls back to script-only
  const result = describer.extractFileGraph("/tmp/test/Broken.vue", source);
  expect(result.symbols).toBeDefined();
});
```

- [ ] **Step 2: Run test — may pass or fail depending on @vue/compiler-dom tolerance**

Run: `bun run test -- tests/vueDescriber.test.ts`

- [ ] **Step 3: Add try/catch around template parsing in VueDescriber**

In `src/ast/vueDescriber.ts`, wrap the template extraction in a try/catch:

```typescript
if (sfc.descriptor.template) {
  try {
    const templateContent = sfc.descriptor.template.content;
    const templateAst = parseTemplate(templateContent);
    const templateNodes = this.convertVueAst(templateAst.children, importedComponents);
    const walkResult = walkTemplate(componentName, templateNodes);
    templateSymbols = walkResult.symbols;
    templateEdges = walkResult.edges;
    for (const [k, v] of walkResult.descriptions) templateDescriptions.set(k, v);
  } catch {
    // Template parse failed — continue with script-only extraction
  }
}
```

- [ ] **Step 4: Write test — empty .tsx file produces no template symbols**

Add to `tests/tsxDescriber.test.ts`:

```typescript
it("produces no template symbols for non-JSX functions", () => {
  const source = [
    "export function add(a: number, b: number) {",
    "  return a + b;",
    "}",
  ].join("\n");

  const result = describer.extractFileGraph("/tmp/test/util.tsx", source);
  const tplSymbols = result.symbols.filter(s => s.kind.startsWith("tpl"));
  expect(tplSymbols).toHaveLength(0);
});
```

- [ ] **Step 5: Run full test suite + typecheck**

Run: `bun run typecheck && bun run test`
Expected: All pass

- [ ] **Step 6: Commit**

```bash
git add src/ast/vueDescriber.ts src/ast/tsxDescriber.ts tests/vueDescriber.test.ts tests/tsxDescriber.test.ts
git commit -m "fix: graceful error handling for malformed templates in Vue and TSX describers"
```

---

### Task 8: Update nlEnricher to handle render edges

**Files:**
- Modify: `src/ast/nlEnricher.ts`
- Test: `tests/nlEnricher.test.ts`

The enricher currently only processes `call` edges. Add support for `render` edges so that template descriptions get cross-referenced with component descriptions when both exist in the same file.

- [ ] **Step 1: Read existing nlEnricher tests for patterns**

Check `tests/nlEnricher.test.ts` to understand test patterns.

- [ ] **Step 2: Write failing test — render edge enrichment**

Add to `tests/nlEnricher.test.ts`:

```typescript
it("enriches template descriptions with render edge context", () => {
  const descriptions = new Map([
    ["tpl:App", "Renders `<UserCard>`"],
    ["fn:UserCard", "1. Assigns calling `fetchUser` to `user`\n2. Returns a template string"],
  ]);
  const edges: LogicEdge[] = [
    { kind: "render", from: "tpl:App", to: "fn:UserCard" },
  ];

  const result = enrichDescriptions(descriptions, edges);
  const appDesc = result.get("tpl:App")!;
  expect(appDesc).toMatch(/UserCard/);
  expect(appDesc).toMatch(/fetchUser|template string/i);
});
```

- [ ] **Step 3: Update nlEnricher to handle render edges**

In `src/ast/nlEnricher.ts`, change the edge filter:

```typescript
// Was: if (edge.kind !== "call") continue;
// Now:
if (edge.kind !== "call" && edge.kind !== "render") continue;
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bun run test -- tests/nlEnricher.test.ts`
Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `bun run test`
Expected: All pass

- [ ] **Step 6: Commit**

```bash
git add src/ast/nlEnricher.ts tests/nlEnricher.test.ts
git commit -m "feat: enrich template NL descriptions with render edge cross-references"
```
