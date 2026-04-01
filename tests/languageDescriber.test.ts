import { describe, expect, it } from "vitest";
import type { LanguageDescriber, FileGraphResult, LogicSymbol } from "../src/ast/languageDescriber";
import { sortSymbols } from "../src/ast/languageDescriber";
import { TypeScriptDescriber } from "../src/ast/typescriptDescriber";

describe("LanguageDescriber interface", () => {
  it("TypeScriptDescriber implements LanguageDescriber", () => {
    const describer: LanguageDescriber = new TypeScriptDescriber();
    expect(describer.languageId).toBe("typescript");
    expect(describer.extensions).toContain(".ts");
    expect(describer.extensions).toContain(".tsx");
    expect(describer.extensions).toContain(".js");
    expect(describer.extensions).toContain(".jsx");
    expect(describer.extensions).toContain(".mjs");
    expect(describer.extensions).toContain(".cjs");
    expect(typeof describer.extractFileGraph).toBe("function");
  });

  it("sorts template symbol kinds after script kinds", () => {
    const tplSymbol: LogicSymbol = {
      id: "tpl:App", kind: "tpl", name: "App", exported: false,
      parentId: null, params: [], complexity: "low",
      control: { async: false, branch: false, await: false, throw: false }, line: 1,
      byteStart: 0, byteEnd: 0, contentHash: "",
    };
    const fnSymbol: LogicSymbol = {
      id: "fn:setup", kind: "fn", name: "setup", exported: true,
      parentId: null, params: [], complexity: "low",
      control: { async: false, branch: false, await: false, throw: false }, line: 1,
      byteStart: 0, byteEnd: 0, contentHash: "",
    };
    const sorted = sortSymbols([tplSymbol, fnSymbol]);
    expect(sorted[0].kind).toBe("fn");
    expect(sorted[1].kind).toBe("tpl");
  });
});

describe("LogicSymbol type", () => {
  it("supports docstring, byteStart, byteEnd, and contentHash fields", () => {
    const symbol: LogicSymbol = {
      id: "fn:test",
      kind: "fn",
      name: "test",
      exported: true,
      parentId: null,
      params: [],
      complexity: "low",
      control: { async: false, branch: false, await: false, throw: false },
      line: 1,
      byteStart: 0,
      byteEnd: 42,
      contentHash: "abc123",
    };
    expect(symbol.byteStart).toBe(0);
    expect(symbol.byteEnd).toBe(42);
    expect(symbol.contentHash).toBe("abc123");
    expect(symbol.docstring).toBeUndefined();

    const withDoc: LogicSymbol = { ...symbol, docstring: "Does something useful." };
    expect(withDoc.docstring).toBe("Does something useful.");
  });
});
