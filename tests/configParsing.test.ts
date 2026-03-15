import { describe, it, expect } from "vitest";
import { parsePositiveInt, parseBoolean } from "../src/configParsing";

describe("parsePositiveInt", () => {
  it("returns undefined for undefined", () => {
    expect(parsePositiveInt(undefined)).toBeUndefined();
  });

  it("returns undefined for empty string", () => {
    expect(parsePositiveInt("")).toBeUndefined();
  });

  it("parses valid positive integers", () => {
    expect(parsePositiveInt("1")).toBe(1);
    expect(parsePositiveInt("768")).toBe(768);
    expect(parsePositiveInt("42")).toBe(42);
  });

  it("throws for zero", () => {
    expect(() => parsePositiveInt("0")).toThrow("Invalid positive integer: 0");
  });

  it("throws for negative numbers", () => {
    expect(() => parsePositiveInt("-1")).toThrow("Invalid positive integer: -1");
  });

  it("throws for non-numeric strings", () => {
    expect(() => parsePositiveInt("abc")).toThrow("Invalid positive integer: abc");
  });

  it("truncates decimals (parseInt behavior)", () => {
    expect(parsePositiveInt("12.5")).toBe(12);
  });
});

describe("parseBoolean", () => {
  it("returns fallback for undefined", () => {
    expect(parseBoolean(undefined, true)).toBe(true);
    expect(parseBoolean(undefined, false)).toBe(false);
  });

  it("parses truthy values", () => {
    expect(parseBoolean("1", false)).toBe(true);
    expect(parseBoolean("true", false)).toBe(true);
    expect(parseBoolean("yes", false)).toBe(true);
    expect(parseBoolean("on", false)).toBe(true);
  });

  it("parses falsy values", () => {
    expect(parseBoolean("0", true)).toBe(false);
    expect(parseBoolean("false", true)).toBe(false);
    expect(parseBoolean("no", true)).toBe(false);
    expect(parseBoolean("off", true)).toBe(false);
  });

  it("handles case insensitivity", () => {
    expect(parseBoolean("TRUE", false)).toBe(true);
    expect(parseBoolean("FALSE", true)).toBe(false);
  });

  it("handles whitespace", () => {
    expect(parseBoolean("  true  ", false)).toBe(true);
  });

  it("throws for invalid values", () => {
    expect(() => parseBoolean("maybe", true)).toThrow("Invalid boolean value: maybe");
    expect(() => parseBoolean("1.0", true)).toThrow("Invalid boolean value: 1.0");
  });
});
