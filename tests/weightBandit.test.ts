import { describe, it, expect, vi, beforeEach } from "vitest";
vi.mock("../src/rl/policyStore", async (importOriginal) => {
  const original = await importOriginal<typeof import("../src/rl/policyStore")>();
  return {
    ...original,
    savePolicy: (policy: any) => policy,
  };
});
import { chooseArm, MEMORY_WEIGHT_ARMS, recordArmReward, weightsForArm } from "../src/rl/weightBandit";
import { MemoryPolicy } from "../src/rl/policyStore";

function mockPolicy(): MemoryPolicy {
  return {
    project: "proj",
    version: 1,
    updated_at: new Date().toISOString(),
    epsilon: 0,
    warmupSamples: 1,
    currentArmId: "balanced",
    armStats: Object.fromEntries(Object.keys(MEMORY_WEIGHT_ARMS).map((k) => [k, { count: 0, totalReward: 0 }])),
  };
}

describe("weightBandit", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("returns normalized weights for an arm", () => {
    const w = weightsForArm("semantic");
    expect(w.vector + w.keyword + w.recency + w.importance).toBeCloseTo(1);
  });

  it("prioritizes warmup for unseen arms", () => {
    const policy = mockPolicy();
    policy.armStats.balanced.count = 1;
    const arm = chooseArm(policy);
    expect(arm).not.toBe("balanced");
  });

  it("updates stats and selects the best arm", () => {
    const policy = mockPolicy();
    policy.armStats.semantic = { count: 2, totalReward: 0.2 };
    policy.armStats.lexical = { count: 2, totalReward: 2.0 };
    const updated = recordArmReward(policy, "lexical", 1);
    expect(updated.armStats.lexical.count).toBe(3);
    expect(updated.currentArmId).toBe("lexical");
  });
});
