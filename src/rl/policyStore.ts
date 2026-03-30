import * as fs from "fs";
import * as path from "path";
import { HybridWeights } from "../storage/scorer";
import { config } from "../core/config";

export interface BanditArmStats {
  count: number;
  totalReward: number;
}

export interface MemoryPolicy {
  project: string;
  version: number;
  updated_at: string;
  epsilon: number;
  warmupSamples: number;
  currentArmId: string;
  armStats: Record<string, BanditArmStats>;
}

type PolicyRecord = Record<string, MemoryPolicy>;

let cache: PolicyRecord | null = null;

function getStorePath(): string {
  return path.resolve(process.cwd(), config.rl.policyStorePath);
}

function readStore(): PolicyRecord {
  if (cache) return cache;
  const p = getStorePath();
  if (!fs.existsSync(p)) {
    cache = {};
    return cache;
  }
  try {
    const raw = fs.readFileSync(p, "utf8");
    cache = raw ? JSON.parse(raw) : {};
  } catch {
    cache = {};
  }
  return cache!;
}

function writeStore(store: PolicyRecord): void {
  const p = getStorePath();
  fs.writeFileSync(p, JSON.stringify(store, null, 2), "utf8");
  cache = store;
}

export function upsertPolicy(project: string, init: () => MemoryPolicy): MemoryPolicy {
  const store = readStore();
  if (!store[project]) {
    store[project] = init();
    writeStore(store);
  }
  return store[project];
}

export function savePolicy(policy: MemoryPolicy): MemoryPolicy {
  const store = readStore();
  policy.version += 1;
  policy.updated_at = new Date().toISOString();
  store[policy.project] = policy;
  writeStore(store);
  return policy;
}

export function getPolicy(project: string): MemoryPolicy | null {
  const store = readStore();
  return store[project] ?? null;
}

export function resetPolicy(project: string): boolean {
  const store = readStore();
  if (!store[project]) return false;
  delete store[project];
  writeStore(store);
  return true;
}

export function listPolicies(): MemoryPolicy[] {
  return Object.values(readStore());
}

export function policyFromPreset(project: string, currentArmId: string, arms: Record<string, HybridWeights>): MemoryPolicy {
  const armStats: Record<string, BanditArmStats> = {};
  for (const id of Object.keys(arms)) {
    armStats[id] = { count: 0, totalReward: 0 };
  }
  return {
    project,
    version: 1,
    updated_at: new Date().toISOString(),
    epsilon: config.rl.epsilon,
    warmupSamples: config.rl.warmupSamples,
    currentArmId,
    armStats,
  };
}
