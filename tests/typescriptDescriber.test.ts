import { describe, expect, it } from "vitest";
import { TypeScriptDescriber } from "../src/ast/typescriptDescriber";

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

  it("generates NL descriptions for each function/method symbol", () => {
    const source = [
      "export function greet(name: string) {",
      "  const trimmed = name.trim();",
      "  if (!trimmed) {",
      "    throw new Error('Name required');",
      "  }",
      "  return `Hello ${trimmed}`;",
      "}",
    ].join("\n");

    const result = describer.extractFileGraph("/tmp/test/greet.ts", source);

    const greetDesc = result.symbolDescriptions.get("fn:greet");
    expect(greetDesc).toBeDefined();
    expect(greetDesc).toContain("trimmed");
    expect(greetDesc).toMatch(/[Tt]hrows/);
    expect(greetDesc).toMatch(/[Rr]eturns/);
  });

  it("generates NL descriptions for class methods", () => {
    const source = [
      "export class Calculator {",
      "  add(a: number, b: number) {",
      "    return a + b;",
      "  }",
      "  divide(a: number, b: number) {",
      "    if (b === 0) {",
      "      throw new Error('Division by zero');",
      "    }",
      "    return a / b;",
      "  }",
      "}",
    ].join("\n");

    const result = describer.extractFileGraph("/tmp/test/calc.ts", source);

    expect(result.symbolDescriptions.get("mtd:Calculator.add")).toBeDefined();
    const divideDesc = result.symbolDescriptions.get("mtd:Calculator.divide");
    expect(divideDesc).toBeDefined();
    expect(divideDesc).toMatch(/[Dd]ivision|zero/);
  });

  it("generates a file summary", () => {
    const source = [
      "export function greet(name: string) { return 'Hello ' + name; }",
      "export function farewell(name: string) { return 'Bye ' + name; }",
    ].join("\n");

    const result = describer.extractFileGraph("/tmp/test/greetings.ts", source);
    expect(result.fileSummary).toBeTruthy();
    expect(result.fileSummary).toMatch(/greet|farewell/i);
  });

  it("extracts symbols from empty file", () => {
    const result = describer.extractFileGraph("/tmp/test/empty.ts", "/* empty */");
    expect(result.symbols).toHaveLength(0);
    expect(result.edges).toHaveLength(0);
    expect(typeof result.fileSummary).toBe("string");
  });
});
