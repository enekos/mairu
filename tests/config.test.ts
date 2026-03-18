import { describe, it, expect } from "vitest";
import { assertEmbeddingDimension, config } from "../src/config";

describe("assertEmbeddingDimension", () => {
  const dimension = config.embedding.dimension;

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

describe("config.embedding", () => {
  it("returns model, dimension, and allowZeroEmbeddings", () => {
    const embedConfig = config.embedding;
    expect(embedConfig).toHaveProperty("model");
    expect(embedConfig).toHaveProperty("dimension");
    expect(embedConfig).toHaveProperty("allowZeroEmbeddings");
    expect(typeof embedConfig.model).toBe("string");
    expect(typeof embedConfig.dimension).toBe("number");
    expect(typeof embedConfig.allowZeroEmbeddings).toBe("boolean");
  });
});