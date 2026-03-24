import { describe, expect, it } from "vitest";
import type { LanguageDescriber, FileGraphResult } from "../src/languageDescriber";
import { TypeScriptDescriber } from "../src/typescriptDescriber";

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
});
