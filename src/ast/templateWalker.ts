import type { LogicSymbol, LogicEdge, LogicSymbolKind } from "./languageDescriber";

export interface TemplateNode {
  tag: string;
  isComponent: boolean;
  directives: TemplateDirective[];
  children: TemplateNode[];
  line: number;
  slotName?: string;
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

const DEFAULT_CONTROL = { async: false, branch: false, await: false, throw: false };

function makeTemplateSymbol(
  id: string,
  kind: LogicSymbolKind,
  name: string,
  parentId: string | null,
  line: number,
): LogicSymbol {
  return {
    id,
    kind,
    name,
    exported: false,
    parentId,
    params: [],
    complexity: "low",
    control: { ...DEFAULT_CONTROL },
    line,
    byteStart: 0,
    byteEnd: 0,
    contentHash: "",
  };
}

/** Sanitise an expression for use in a symbol ID (strip spaces, limit length). */
function idSafe(expr: string): string {
  return expr.replace(/\s+/g, "_").slice(0, 60);
}

/** Parse a v-for expression like "item in items" or "(item, index) in items". */
function parseForExpression(expr: string): { iterator: string; iterable: string } {
  const match = expr.match(/^\s*(?:\(([^)]+)\)|(\S+))\s+(?:in|of)\s+(.+?)\s*$/);
  if (!match) return { iterator: expr, iterable: expr };
  const iterator = (match[1] ?? match[2]).trim();
  const iterable = match[3].trim();
  return { iterator, iterable };
}

export function walkTemplate(
  componentName: string,
  nodes: TemplateNode[],
): TemplateWalkResult {
  const symbols: LogicSymbol[] = [];
  const edges: LogicEdge[] = [];
  const descriptions = new Map<string, string>();
  const seenIds = new Set<string>();

  const rootId = `tpl:${componentName}`;
  symbols.push(makeTemplateSymbol(rootId, "tpl", componentName, null, 1));
  seenIds.add(rootId);

  const renderedComponents: string[] = [];

  function addSymbol(sym: LogicSymbol): void {
    if (seenIds.has(sym.id)) return;
    seenIds.add(sym.id);
    symbols.push(sym);
  }

  function walk(node: TemplateNode, parentId: string): void {
    let currentParent = parentId;

    // Process directives — order: for first, then conditional, then others
    for (const dir of node.directives) {
      if (dir.kind === "for") {
        const { iterable } = parseForExpression(dir.expression);
        const loopId = `tpl-loop:${componentName}.for_${idSafe(iterable)}`;
        addSymbol(makeTemplateSymbol(loopId, "tpl-loop", `for_${idSafe(iterable)}`, currentParent, node.line));
        descriptions.set(loopId, `Iterates over \`${iterable}\` rendering ${describeNodeContent(node)}`);
        currentParent = loopId;
      }
    }

    for (const dir of node.directives) {
      if (dir.kind === "if") {
        const branchId = `tpl-branch:${componentName}.if_${idSafe(dir.expression)}`;
        addSymbol(makeTemplateSymbol(branchId, "tpl-branch", `if_${idSafe(dir.expression)}`, currentParent, node.line));
        descriptions.set(branchId, `Conditionally renders ${describeNodeContent(node)} when \`${dir.expression}\` is truthy`);
        currentParent = branchId;
      } else if (dir.kind === "else-if") {
        const branchId = `tpl-branch:${componentName}.elseif_${idSafe(dir.expression)}`;
        addSymbol(makeTemplateSymbol(branchId, "tpl-branch", `elseif_${idSafe(dir.expression)}`, currentParent, node.line));
        descriptions.set(branchId, `Conditionally renders ${describeNodeContent(node)} when \`${dir.expression}\` is truthy`);
        currentParent = branchId;
      } else if (dir.kind === "else") {
        const branchId = `tpl-branch:${componentName}.else`;
        addSymbol(makeTemplateSymbol(branchId, "tpl-branch", "else", currentParent, node.line));
        descriptions.set(branchId, `Renders ${describeNodeContent(node)} as fallback (else branch)`);
        currentParent = branchId;
      } else if (dir.kind === "show") {
        const branchId = `tpl-branch:${componentName}.show_${idSafe(dir.expression)}`;
        addSymbol(makeTemplateSymbol(branchId, "tpl-branch", `show_${idSafe(dir.expression)}`, currentParent, node.line));
        descriptions.set(branchId, `Toggles visibility of ${describeNodeContent(node)} when \`${dir.expression}\` is truthy`);
        currentParent = branchId;
      } else if (dir.kind === "is") {
        edges.push({ kind: "render", from: currentParent, to: `type:dynamic(${dir.expression})` });
      }
    }

    // Handle <slot> elements
    if (node.tag === "slot") {
      const slotName = node.slotName ?? "default";
      const slotId = `tpl-slot:${componentName}.${slotName}`;
      addSymbol(makeTemplateSymbol(slotId, "tpl-slot", slotName, currentParent, node.line));
      edges.push({ kind: "slot", from: rootId, to: slotId });
      descriptions.set(slotId, `Exposes a ${slotName === "default" ? "default" : `named "${slotName}"`} slot`);
      return; // slots don't have meaningful children to walk further
    }

    // Handle component references
    if (node.isComponent) {
      edges.push({ kind: "render", from: currentParent, to: `type:${node.tag}` });
      renderedComponents.push(node.tag);
    }

    // Recurse into children
    for (const child of node.children) {
      walk(child, currentParent);
    }
  }

  function describeNodeContent(node: TemplateNode): string {
    if (node.isComponent) return `\`<${node.tag}>\``;
    // Summarise by listing child components
    const childComponents = collectComponents(node);
    if (childComponents.length > 0) {
      return childComponents.map((c) => `\`<${c}>\``).join(", ");
    }
    return `\`<${node.tag}>\``;
  }

  function collectComponents(node: TemplateNode): string[] {
    const comps: string[] = [];
    if (node.isComponent) comps.push(node.tag);
    for (const child of node.children) {
      comps.push(...collectComponents(child));
    }
    return comps;
  }

  for (const node of nodes) {
    walk(node, rootId);
  }

  // Build root description
  const uniqueRendered = [...new Set(renderedComponents)];
  if (uniqueRendered.length > 0) {
    descriptions.set(rootId, `Template renders ${uniqueRendered.map((c) => `\`<${c}>\``).join(", ")}`);
  } else {
    descriptions.set(rootId, `Template for \`${componentName}\``);
  }

  return { symbols, edges, descriptions };
}
