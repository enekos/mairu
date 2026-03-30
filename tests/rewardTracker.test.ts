import { describe, it, expect, beforeEach, afterEach } from "vitest";
import * as fs from "fs";
import * as path from "path";
import { computeReward, logOutcome } from "../src/rl/rewardTracker";

describe("rewardTracker", () => {
  const logPath = path.resolve(process.cwd(), ".tmp-rl-events.jsonl");

  beforeEach(() => {
    process.env.RL_EVENT_LOG_PATH = ".tmp-rl-events.jsonl";
    if (fs.existsSync(logPath)) fs.unlinkSync(logPath);
  });

  afterEach(() => {
    delete process.env.RL_EVENT_LOG_PATH;
    if (fs.existsSync(logPath)) fs.unlinkSync(logPath);
  });

  it("computes rank-shaped reward for accepted results", () => {
    expect(computeReward({ action: "accepted", selectedRank: 2 })).toBeCloseTo(1.5);
  });

  it("logs event lines", () => {
    const event = logOutcome({ action: "feedback", project: "demo", armId: "balanced", reward: 0.8 });
    expect(event.timestamp).toBeTypeOf("string");
    const lines = fs.readFileSync(logPath, "utf8").trim().split("\n");
    expect(lines.length).toBe(1);
  });
});
