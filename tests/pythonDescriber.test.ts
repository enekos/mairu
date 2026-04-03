import { describe, expect, it, beforeAll, afterAll } from "vitest";
import { PythonDescriber } from "../src/ast/pythonDescriber";

describe("PythonDescriber", () => {
  const describer = new PythonDescriber();

  beforeAll(async () => {
    await describer.initParsers();
  });

  afterAll(() => {
    describer.deleteParsers();
  });

  it("extracts symbols and edges from Python source", () => {
    const source = [
      "from utils import slugify",
      "import os",
      "",
      "INTERNAL_SEED = 42",
      "",
      "def normalize(name: str):",
      "    return slugify(f'{name}-{INTERNAL_SEED}')",
      "",
      "def greet(name: str):",
      "    return normalize(name)",
      "",
      "class UserService(BaseService):",
      "    def run(self, input: str):",
      "        self.bump()",
      "        return greet(input)",
      "    def bump(self):",
      "        return normalize('x')",
    ].join("\n");

    const result = describer.extractFileGraph("/tmp/test/feature.py", source);

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
    expect(edgeKeys).toContain("import:file->module:utils");
    expect(edgeKeys).toContain("import:file->module:os");
    expect(edgeKeys).toContain("extends:cls:UserService->type:BaseService");

    expect(result.imports).toContain("utils");
    expect(result.imports).toContain("os");
  });

  it("generates NL descriptions for each function/method symbol", () => {
    const source = [
      "def greet(name: str):",
      "    trimmed = name.strip()",
      "    if not trimmed:",
      "        raise Exception('Name required')",
      "    return f'Hello {trimmed}'",
    ].join("\n");

    const result = describer.extractFileGraph("/tmp/test/greet.py", source);

    const greetDesc = result.symbolDescriptions.get("fn:greet");
    expect(greetDesc).toBeDefined();
    expect(greetDesc).toContain("trimmed");
  });

  it("generates NL descriptions for class methods", () => {
    const source = [
      "class Calculator:",
      "    def add(self, a: int, b: int):",
      "        return a + b",
      "    def divide(self, a: int, b: int):",
      "        if b == 0:",
      "            raise ValueError('Division by zero')",
      "        return a / b",
    ].join("\n");

    const result = describer.extractFileGraph("/tmp/test/calc.py", source);

    expect(result.symbolDescriptions.get("mtd:Calculator.add")).toBeDefined();
    const divideDesc = result.symbolDescriptions.get("mtd:Calculator.divide");
    expect(divideDesc).toBeDefined();
  });

  it("populates byteStart, byteEnd, and contentHash on symbols", () => {
    const source = [
      "def hello():",
      "    return 'world'",
    ].join("\n");

    const result = describer.extractFileGraph("/tmp/test/offsets.py", source);
    const hello = result.symbols.find(s => s.id === "fn:hello")!;

    expect(hello.byteStart).toBeGreaterThanOrEqual(0);
    expect(hello.byteEnd).toBeGreaterThan(hello.byteStart);
    expect(hello.contentHash).toMatch(/^[0-9a-f]{40}$/);

    const sliced = source.slice(hello.byteStart, hello.byteEnd);
    expect(sliced).toContain("def hello");
    expect(sliced).toContain("return 'world'");
  });

  it("extracts Python docstring from function/class body", () => {
    const source = [
      "def validate(email: str):",
      "    \"\"\"Validates an email address against RFC 5322 rules.\"\"\"",
      "    return '@' in email",
    ].join("\n");

    const result = describer.extractFileGraph("/tmp/test/docstring.py", source);
    const validate = result.symbols.find(s => s.id === "fn:validate")!;

    expect(validate.docstring).toBe("Validates an email address against RFC 5322 rules.");
  });

  it("extracts first sentence from multi-line Python docstring", () => {
    const source = [
      "class Service:",
      "    \"\"\"",
      "    Manages user sessions and authentication state.",
      "    Internal use only.",
      "    \"\"\"",
      "    def clear(self):",
      "        pass",
    ].join("\n");

    const result = describer.extractFileGraph("/tmp/test/multiline.py", source);
    const cls = result.symbols.find(s => s.id === "cls:Service")!;

    expect(cls.docstring).toBe("Manages user sessions and authentication state.");
  });

  it("extracts class and method docstrings correctly", () => {
    const source = [
      "class Service:",
      "    \"\"\"Service class docstring.\"\"\"",
      "    def start(self, port: int):",
      "        \"\"\"Starts the service.\"\"\"",
      "        return self.listen(port)",
      "    def listen(self, port: int):",
      "        pass",
    ].join("\n");

    const result = describer.extractFileGraph("/tmp/test/method-doc.py", source);
    const cls = result.symbols.find(s => s.id === "cls:Service")!;
    const start = result.symbols.find(s => s.id === "mtd:Service.start")!;
    const listen = result.symbols.find(s => s.id === "mtd:Service.listen")!;

    expect(cls.docstring).toBe("Service class docstring.");
    expect(start.docstring).toBe("Starts the service.");
    expect(listen.docstring).toBeUndefined();
  });
});