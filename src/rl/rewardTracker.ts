import * as fs from "fs";
import * as path from "path";
import { config } from "../core/config";

export type RetrievalOutcomeAction = "retrieve" | "accepted" | "ignored" | "feedback" | "fallback_deep";

export interface RetrievalOutcomeEvent {
  project?: string;
  query?: string;
  action: RetrievalOutcomeAction;
  armId?: string;
  reward?: number;
  selectedRank?: number;
  topScore?: number;
  timestamp: string;
  metadata?: Record<string, any>;
}

function getEventPath(): string {
  return path.resolve(process.cwd(), config.rl.eventLogPath);
}

export function computeReward(event: Omit<RetrievalOutcomeEvent, "timestamp">): number {
  if (typeof event.reward === "number") return event.reward;
  if (event.action === "accepted") {
    const rankBoost = event.selectedRank && event.selectedRank > 0 ? 1 / event.selectedRank : 0;
    return 1 + rankBoost;
  }
  if (event.action === "ignored") return -0.2;
  if (event.action === "fallback_deep") return -0.4;
  if (event.action === "feedback") return 0;
  return 0;
}

export function logOutcome(event: Omit<RetrievalOutcomeEvent, "timestamp">): RetrievalOutcomeEvent {
  const fullEvent: RetrievalOutcomeEvent = {
    ...event,
    reward: computeReward(event),
    timestamp: new Date().toISOString(),
  };
  const line = `${JSON.stringify(fullEvent)}\n`;
  fs.appendFileSync(getEventPath(), line, "utf8");
  return fullEvent;
}
