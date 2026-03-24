import { describe, expect, it } from "vitest";
import { TypeScriptDescriber } from "../src/typescriptDescriber";

describe("TypeScriptDescriber", () => {
  const describer = new TypeScriptDescriber();

  it("extracts symbols and edges from TypeScript source", () => {
    const source = [
      "import { slugify } from './slug';",
      "const INTERNAL_SEED = 42;",
      "",
      "function normalize(name: string) {",
      "  return slugify(`${name}-${INTERNAL_SEED}`);",
      "}",
      "",
      "export function greet(name: string) {",
      "  return normalize(name);",
      "}",
      "",
      "export class UserService {",
      "  public run(input: string) {",
      "    this.bump();",
      "    return greet(input);",
      "  }",
      "  private bump() {",
      "    return normalize('x');",
      "  }",
      "}",
    ].join("\n");

    const result = describer.extractFileGraph("/tmp/test/feature.ts", source);

    const symbolIds = result.symbols.map(s => s.id);
    expect(symbolIds).toContain("fn:greet");
    expect(symbolIds).toContain("fn:normalize");
    expect(symbolIds).toContain("cls:UserService");
    expect(symbolIds).toContain("mtd:UserService.run");
    expect(symbolIds).toContain("mtd:UserService.bump");
    expect(symbolIds).toContain("var:INTERNAL_SEED");

    const edgeKeys = result.edges.map(e => `${e.kind}:${e.from}->${e.to}`);
    expect(edgeKeys).toContain("call:fn:greet->fn:normalize");
    expect(edgeKeys).toContain("call:mtd:UserService.run->mtd:UserService.bump");
    expect(edgeKeys).toContain("call:mtd:UserService.run->fn:greet");
    expect(edgeKeys).toContain("import:file->module:./slug");

    expect(result.imports).toContain("./slug");
  });

  it("extracts symbols from empty file", () => {
    const result = describer.extractFileGraph("/tmp/test/empty.ts", "/* empty */");
    expect(result.symbols).toHaveLength(0);
    expect(result.edges).toHaveLength(0);
    expect(typeof result.fileSummary).toBe("string");
  });
});
