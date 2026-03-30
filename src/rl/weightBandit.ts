import { normalizeWeights, HybridWeights } from "../storage/scorer";
import { MemoryPolicy, savePolicy } from "./policyStore";

export const MEMORY_WEIGHT_ARMS: Record<string, HybridWeights> = {
  balanced: { vector: 0.6, keyword: 0.2, recency: 0.05, importance: 0.15 },
  lexical: { vector: 0.45, keyword: 0.35, recency: 0.05, importance: 0.15 },
  semantic: { vector: 0.75, keyword: 0.1, recency: 0.05, importance: 0.1 },
  recent: { vector: 0.5, keyword: 0.15, recency: 0.25, importance: 0.1 },
};

export function chooseArm(policy: MemoryPolicy): string {
  for (const [armId, stats] of Object.entries(policy.armStats)) {
    if (stats.count < policy.warmupSamples) return armId;
  }
  if (Math.random() < policy.epsilon) {
    const armIds = Object.keys(policy.armStats);
    return armIds[Math.floor(Math.random() * armIds.length)];
  }
  return getBestArm(policy);
}

export function getBestArm(policy: MemoryPolicy): string {
  let bestArm = policy.currentArmId;
  let bestAvg = Number.NEGATIVE_INFINITY;
  for (const [armId, stats] of Object.entries(policy.armStats)) {
    const avg = stats.count > 0 ? stats.totalReward / stats.count : 0;
    if (avg > bestAvg) {
      bestAvg = avg;
      bestArm = armId;
    }
  }
  return bestArm;
}

export function weightsForArm(armId: string): HybridWeights {
  return normalizeWeights(MEMORY_WEIGHT_ARMS[armId] ?? MEMORY_WEIGHT_ARMS.balanced);
}

export function recordArmReward(policy: MemoryPolicy, armId: string, reward: number): MemoryPolicy {
  const stats = policy.armStats[armId] ?? { count: 0, totalReward: 0 };
  stats.count += 1;
  stats.totalReward += reward;
  policy.armStats[armId] = stats;
  policy.currentArmId = getBestArm(policy);
  return savePolicy(policy);
}
