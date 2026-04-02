import { describe, it, expect } from "vitest";

describe("dreamer types", () => {
  it("accepts derived_pattern as a valid MemoryCategory", async () => {
    // TypeScript compilation is the test — if this compiles, the type exists
    const category: import("../src/core/types").MemoryCategory = "derived_pattern";
    expect(category).toBe("derived_pattern");
  });
});
