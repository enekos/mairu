import { describe, expect, it } from "vitest";
import { describeStatements } from "../src/ast/nlDescriber";
import { Project } from "ts-morph";

function describeFunction(source: string, fnName: string): string {
  const project = new Project({ compilerOptions: { allowJs: true } });
  const sf = project.createSourceFile("/tmp/test.ts", source);
  const fn = sf.getFunction(fnName);
  if (!fn) throw new Error(`Function ${fnName} not found`);
  return describeStatements(fn);
}

describe("nlDescriber", () => {
  it("describes a simple return statement", () => {
    const nl = describeFunction(
      `function greet(name: string) { return "Hello " + name; }`,
      "greet"
    );
    expect(nl).toContain("Returns");
    expect(nl).toContain("name");
  });

  it("describes if/else branches", () => {
    const nl = describeFunction(
      `function check(x: number) {
        if (x > 0) {
          return "positive";
        } else {
          return "non-positive";
        }
      }`,
      "check"
    );
    expect(nl).toContain("x");
    expect(nl).toMatch(/[Ii]f/);
    expect(nl).toMatch(/[Oo]therwise/);
  });

  it("describes for-of loops", () => {
    const nl = describeFunction(
      `function process(items: string[]) {
        for (const item of items) {
          console.log(item);
        }
      }`,
      "process"
    );
    expect(nl).toMatch(/[Ii]terates|[Ll]oops|[Ff]or each/);
    expect(nl).toContain("item");
    expect(nl).toContain("items");
  });

  it("describes try/catch blocks", () => {
    const nl = describeFunction(
      `function safeParse(json: string) {
        try {
          return JSON.parse(json);
        } catch (e) {
          return null;
        }
      }`,
      "safeParse"
    );
    expect(nl).toMatch(/[Aa]ttempts|[Tt]ries/);
    expect(nl).toMatch(/[Ee]rror|fails/);
  });

  it("describes throw statements", () => {
    const nl = describeFunction(
      `function validate(x: any) {
        if (!x) {
          throw new Error("Required");
        }
      }`,
      "validate"
    );
    expect(nl).toMatch(/[Tt]hrows/);
    expect(nl).toContain("Error");
  });

  it("describes variable assignments with function calls", () => {
    const nl = describeFunction(
      `function transform(input: string) {
        const trimmed = input.trim();
        const lower = trimmed.toLowerCase();
        return lower;
      }`,
      "transform"
    );
    expect(nl).toContain("trimmed");
    expect(nl).toContain("trim");
    expect(nl).toContain("lower");
  });

  it("describes await expressions", () => {
    const nl = describeFunction(
      `async function fetchData(url: string) {
        const response = await fetch(url);
        const data = await response.json();
        return data;
      }`,
      "fetchData"
    );
    expect(nl).toMatch(/[Aa]waits/);
    expect(nl).toContain("fetch");
  });

  it("describes switch statements", () => {
    const nl = describeFunction(
      `function classify(status: number) {
        switch (status) {
          case 200: return "ok";
          case 404: return "not found";
          default: return "unknown";
        }
      }`,
      "classify"
    );
    expect(nl).toMatch(/[Ss]witch|[Bb]ased on/);
    expect(nl).toContain("status");
  });

  it("describes while loops", () => {
    const nl = describeFunction(
      `function countdown(n: number) {
        while (n > 0) {
          n--;
        }
      }`,
      "countdown"
    );
    expect(nl).toMatch(/[Ww]hile|[Ll]oops/);
  });

  it("translates common conditions to natural English", () => {
    const nl = describeFunction(
      `function check(x: any, items: string[]) {
        if (x === null) { return "a"; }
        if (!x) { return "b"; }
        if (typeof x === "string") { return "c"; }
        if (items.length > 0) { return "d"; }
        if (x instanceof Error) { return "e"; }
      }`,
      "check"
    );
    expect(nl).toMatch(/`x` is null/);
    expect(nl).toMatch(/`x` is falsy/);
    expect(nl).toMatch(/`x` is a string/);
    expect(nl).toMatch(/`items` is non-empty|`items\.length` is greater than 0/);
    expect(nl).toMatch(/`x` is an instance of `Error`/);
  });

  it("describes nested if inside loop", () => {
    const nl = describeFunction(
      `function filterPositive(nums: number[]) {
        const result: number[] = [];
        for (const n of nums) {
          if (n > 0) {
            result.push(n);
          }
        }
        return result;
      }`,
      "filterPositive"
    );
    expect(nl).toMatch(/[Ii]terates|[Ff]or each/);
    expect(nl).toMatch(/[Ii]f/);
    expect(nl).toContain("result");
  });
});
