import { describe, it, expect } from "vitest";
import { assertEmbeddingDimension, getEmbeddingConfig } from "../src/embeddingConfig";

describe("assertEmbeddingDimension", () => {
  const dimension = getEmbeddingConfig().dimension;

  it("does not throw for correctly sized vector", () => {
    const vector = Array(dimension).fill(0);
    expect(() => assertEmbeddingDimension(vector, "test")).not.toThrow();
  });

  it("throws for undersized vector", () => {
    const vector = Array(dimension - 1).fill(0);
    expect(() => assertEmbeddingDimension(vector, "test-context")).toThrow(
      `Invalid embedding size for test-context. Expected ${dimension}, got ${dimension - 1}.`
    );
  });

  it("throws for oversized vector", () => {
    const vector = Array(dimension + 1).fill(0);
    expect(() => assertEmbeddingDimension(vector, "my-context")).toThrow(
      `Invalid embedding size for my-context. Expected ${dimension}, got ${dimension + 1}.`
    );
  });
});

describe("getEmbeddingConfig", () => {
  it("returns model, dimension, and allowZeroEmbeddings", () => {
    const config = getEmbeddingConfig();
    expect(config).toHaveProperty("model");
    expect(config).toHaveProperty("dimension");
    expect(config).toHaveProperty("allowZeroEmbeddings");
    expect(typeof config.model).toBe("string");
    expect(typeof config.dimension).toBe("number");
    expect(typeof config.allowZeroEmbeddings).toBe("boolean");
  });
});
