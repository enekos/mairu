# TSX & Vue Language Describers

## Summary

Add language describers for `.tsx`/`.jsx` and `.vue` files that extract AST-like template structure alongside script logic. Template directives (`v-if`, `v-for`, etc.) and JSX conditional/iterative patterns produce dedicated symbols in the logic graph, with full NL descriptions.

## Decisions

1. **Both** template symbol tree (overview/graph) and inline NL descriptions (content field)
2. Vue: **`<script setup>` only** (Composition API) — Options API deferred
3. **Structural directives + component composition** (`v-if`, `v-for`, `v-show`, `<slot>`, `<component :is>`) — data binding (`v-model`, `@event`) excluded
4. **Rich template symbol kinds** — `tpl`, `tpl-slot`, `tpl-branch`, `tpl-loop` as dedicated symbols in the graph

## New Symbol & Edge Kinds

### Symbol Kinds

| Kind | Represents | Example |
|------|-----------|---------|
| `tpl` | Template root | One per component/function returning JSX |
| `tpl-slot` | Slot definition | `<slot name="header">` |
| `tpl-branch` | Conditional render block | `v-if="isOpen"`, ternary, `&&` |
| `tpl-loop` | Iterative render block | `v-for="item in items"`, `.map()` |

### Edge Kinds

| Kind | Represents | Example |
|------|-----------|---------|
| `render` | Component renders another component | `tpl:App` -> `type:Modal` |
| `slot` | Component defines/provides a slot | `tpl:Layout` -> `tpl-slot:Layout.header` |

### Symbol Hierarchy Example

```
tpl:MyPage
├── tpl-branch:MyPage.if_isLoading   (v-if="isLoading")
├── tpl-branch:MyPage.else
│   └── tpl-loop:MyPage.for_items    (v-for="item in items")
└── tpl-slot:MyPage.default          (<slot>)
```

## File Architecture

### New Files

| File | Role |
|------|------|
| `src/ast/tsxDescriber.ts` | Extends `TypeScriptDescriber` — after normal TS extraction, walks JSX to extract template symbols/edges/NL |
| `src/ast/vueDescriber.ts` | Vue SFC describer — parses `<template>` and `<script setup>`, delegates script to `TypeScriptDescriber`, template to shared walker |
| `src/ast/templateWalker.ts` | Shared template-to-symbols logic — converts abstract `TemplateNode[]` to symbols/edges/descriptions |

### Modified Files

| File | Change |
|------|--------|
| `src/ast/languageDescriber.ts` | Add new symbol kinds and edge kinds, update `KIND_SORT_ORDER` |
| `src/daemon.ts` | Register new describers, add `.vue` to supported extensions, route by extension, add NL `kindLabel` entries |

## Template Walker Design

### Abstract Input

```typescript
interface TemplateNode {
  tag: string;                     // "div", "Modal", "slot", "template"
  isComponent: boolean;            // true if PascalCase or registered component
  directives: TemplateDirective[];
  children: TemplateNode[];
  line: number;
}

interface TemplateDirective {
  kind: "if" | "else-if" | "else" | "for" | "show" | "is";
  expression: string;              // raw expression text
}
```

### Output

- `LogicSymbol[]` — `tpl`, `tpl-branch`, `tpl-loop`, `tpl-slot` symbols
- `LogicEdge[]` — `render` edges to child components, `slot` edges
- `Map<string, string>` — NL descriptions per template symbol

### Component Detection

A tag is a component if PascalCase (`<Modal>`), contains a dot (`<motion.div>`), or (Vue only) matches an import from `<script setup>`.

## TSX Describer

Extends `TypeScriptDescriber`. Overrides `extractFileGraph()`: calls `super.extractFileGraph()`, then walks JSX in component functions.

### JSX Pattern Detection

| JSX Pattern | Produces |
|---|---|
| `{condition && <X/>}` | `tpl-branch` + `render` edge to X |
| `{condition ? <X/> : <Y/>}` | Two `tpl-branch` symbols (if + else) |
| `{items.map(item => <X/>)}` | `tpl-loop` + `render` edge to X |
| `<Component/>` (direct) | `render` edge from parent `tpl` |
| Nested combinations | Nested symbols with correct `parentId` |

Only functions that return JSX get a `tpl:` root.

## Vue Describer

### Parsing Flow

1. Parse SFC with `@vue/compiler-sfc` (`parse()`)
2. Feed `<script setup>` content to `TypeScriptDescriber.extractFileGraph()`
3. Parse `<template>` with `@vue/compiler-dom` (`parse()`)
4. Convert Vue AST nodes to `TemplateNode[]`
5. Pass to shared `templateWalker`
6. Merge script + template results

### Vue AST Conversion

| Vue AST Node | TemplateNode Mapping |
|---|---|
| `ElementNode` with PascalCase tag or in imports | `isComponent: true` |
| `DirectiveNode` name=`if` | `{ kind: "if", expression }` |
| `DirectiveNode` name=`else-if` | `{ kind: "else-if", expression }` |
| `DirectiveNode` name=`else` | `{ kind: "else", expression: "" }` |
| `DirectiveNode` name=`for` | `{ kind: "for", expression }` |
| `DirectiveNode` name=`show` | `{ kind: "show", expression }` |
| `ElementNode` tag=`slot` | slot |
| `ElementNode` tag=`component` with `:is` | `{ kind: "is", expression }` |

### Imported Component Detection

Scan `<script setup>` imports for PascalCase default/named imports. Build a component registry. Template tags matching a registered import get `isComponent: true`.

## Daemon Integration

- Add `.vue` to `SUPPORTED_EXTENSIONS` and chokidar glob
- Three describers routed by extension:
  - `.vue` -> `VueDescriber`
  - `.tsx`, `.jsx` -> `TsxDescriber`
  - `.ts`, `.js`, `.mjs`, `.cjs` -> `TypeScriptDescriber`
- Update `KIND_SORT_ORDER`: `tpl: 7`, `tpl-branch: 8`, `tpl-loop: 9`, `tpl-slot: 10`
- Add `kindLabel` entries for template kinds in NL content builder

### NL Content Template Section

```
## Template: UserPage
Conditionally renders `<Modal>` when `isOpen` is truthy

### Branch: if isAdmin
Renders `<AdminPanel>`

### Loop: for item in items
Iterates over each `item` in `items`, rendering `<Card>` for each
```

## Dependencies

- `@vue/compiler-sfc` (includes `@vue/compiler-dom`) — single new package
