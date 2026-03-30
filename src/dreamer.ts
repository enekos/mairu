import { config } from "./core/config";
import { createContextManager } from "./storage/client";
import { planVibeMutation, executeMutationOp } from "./llm/vibeEngine";
import { AgentMemory } from "./core/types";

const cm = createContextManager();

/**
 * Background Dreaming Daemon (Continual Learning).
 * 
 * Runs asynchronously, finding high-activity sessions/peers and compressing their
 * raw observational/message data into high-importance profiles and patterns.
 */
async function dreamLoop() {
  console.log("[dreamer] Starting dream cycle...");
  try {
    // 1) Collect raw memories that can be synthesized.
    const recentMemories = await cm.listMemories({
      category: "message",
      memoryState: "raw",
    }, 100);

    if (!recentMemories || recentMemories.length === 0) {
      console.log("[dreamer] No new memories to dream about. Sleeping...");
      return scheduleNext();
    }

    // 2) Group by project + peer/session so synthesis stays localized.
    const groups = new Map<string, { memories: AgentMemory[]; lines: string[] }>();
    for (const m of recentMemories) {
      const project = m.project || "default";
      const actorKey = m.peer_id || m.session_id || "global";
      const key = `${project}::${actorKey}`;
      if (!groups.has(key)) groups.set(key, { memories: [], lines: [] });
      
      const group = groups.get(key)!;
      group.memories.push(m);
      group.lines.push(`[${m.owner}]: ${m.content}`);
    }

    // 3) Synthesize each sufficiently large group.
    for (const [key, group] of groups.entries()) {
      if (group.lines.length < 5) continue;

      const [project, actorKey] = key.split("::");
      const projectName = project === "default" ? undefined : project;

      const prompt = `Synthesize these recent observations into long-term profile facts or patterns about the user/agent. Focus on evolving preferences, constraints, or identity traits. IMPORTANT: Only extract meaningful long-term facts. Do NOT summarize casual chatter.`;
      
      console.log(`[dreamer] Dreaming for peer/session ${actorKey} (project=${projectName || "none"})...`);
      
      try {
        // Use vibe mutation to plan the synthesis. We give it the recent messages.
        // vibeMutation will AUTOMATICALLY fetch the existing context for this peer/session
        // and decide whether to update existing profile facts or create new ones.
        const plan = await planVibeMutation(cm, prompt + "\n\n" + group.lines.join("\n"), undefined, 15);
        
        let operationsExecuted = 0;
        const maxOpsPerGroup = 5;
        const allowedCategories = new Set(["profile", "patterns", "preferences", "constraint", "decision"]);
        
        if (plan.operations.length > 0) {
          console.log(`[dreamer] Found ${plan.operations.length} insights for ${key}`);
          for (const op of plan.operations.slice(0, maxOpsPerGroup)) {
            // Only synthesize new memories in the daemon.
            if (op.op !== "create_memory") continue;
            if (!op.data || typeof op.data.content !== "string" || !op.data.content.trim()) continue;

            // Add peer_id/session_id + project to the new memory if applicable.
            if (op.op === "create_memory" && op.data) {
              op.data.project = projectName;
              if (actorKey !== "global") {
                if (group.memories[0].peer_id === actorKey) {
                   op.data.peer_id = actorKey;
                } else if (group.memories[0].session_id === actorKey) {
                   op.data.session_id = actorKey;
                }
              }

              // Ensure curated category + state.
              if (!allowedCategories.has(op.data.category)) {
                op.data.category = "profile";
              }
              op.data.memory_state = "curated";
              op.data.source_memory_ids = group.memories.map((m) => m.id);
              op.data.confidence = typeof op.data.confidence === "number" ? op.data.confidence : 0.7;
              op.data.quality_score = typeof op.data.quality_score === "number" ? op.data.quality_score : 0.75;
            }

            // Novelty guard: skip near-duplicate curated memory.
            const noveltyProbe = await cm.searchMemories(op.data.content, {
              project: projectName,
              topK: 1,
              retrievalMode: "surface",
              memoryState: "curated",
            });
            if (noveltyProbe[0] && noveltyProbe[0]._score > 0.9) {
              continue;
            }
            await executeMutationOp(cm, op);
            operationsExecuted++;
          }
        }
        
        // 4) Archive source memories to avoid re-processing.
        if (operationsExecuted > 0 || plan.operations.length === 0) {
           console.log(`[dreamer] Marking ${group.memories.length} messages as processed for ${key}`);
           for (const m of group.memories) {
               await cm.updateMemory(m.id, {
                 category: "observation",
                 memory_state: "archived",
                 metadata: {
                   ...(m.metadata || {}),
                   archived_by: "dreamer",
                   archived_at: new Date().toISOString(),
                 },
               });
           }
        }

      } catch (err) {
        console.error(`[dreamer] Failed to process group ${key}:`, err);
      }
    }

  } catch (err) {
    console.error("[dreamer] Error during dream cycle:", err);
  }

  scheduleNext();
}

function scheduleNext() {
  const interval = 2 * 60 * 1000; // 2 minutes
  console.log(`[dreamer] Cycle complete. Next dream in ${interval/1000}s`);
  setTimeout(dreamLoop, interval);
}

// Start the daemon if run directly
if (require.main === module) {
  dreamLoop();
}

export { dreamLoop };
