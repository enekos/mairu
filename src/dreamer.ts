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
    // 1. Find recent un-summarized messages/observations
    // We process these in batches.
    const recentMemories = await cm.listMemories({
      category: "message",
    }, 100);

    if (!recentMemories || recentMemories.length === 0) {
      console.log("[dreamer] No new memories to dream about. Sleeping...");
      return scheduleNext();
    }

    // Group by Peer/Session
    const groups = new Map<string, { memories: AgentMemory[]; lines: string[] }>();
    for (const m of recentMemories) {
      const key = m.peer_id || m.session_id || "global";
      if (!groups.has(key)) groups.set(key, { memories: [], lines: [] });
      
      const group = groups.get(key)!;
      group.memories.push(m);
      group.lines.push(`[${m.owner}]: ${m.content}`);
    }

    // Process each group
    for (const [key, group] of groups.entries()) {
      if (group.lines.length < 5) continue; // Skip groups without enough substance

      const prompt = `Synthesize these recent observations into long-term profile facts or patterns about the user/agent. Focus on evolving preferences, constraints, or identity traits. IMPORTANT: Only extract meaningful long-term facts. Do NOT summarize casual chatter.`;
      
      console.log(`[dreamer] Dreaming for peer/session ${key}...`);
      
      try {
        // Use vibe mutation to plan the synthesis. We give it the recent messages.
        // vibeMutation will AUTOMATICALLY fetch the existing context for this peer/session
        // and decide whether to update existing profile facts or create new ones.
        const plan = await planVibeMutation(cm, prompt + "\n\n" + group.lines.join("\n"), undefined, 15);
        
        let operationsExecuted = 0;
        
        if (plan.operations.length > 0) {
          console.log(`[dreamer] Found ${plan.operations.length} insights for ${key}`);
          for (const op of plan.operations) {
            // Add peer_id/session_id to the new memory if applicable
            if (op.op === "create_memory" && op.data) {
              if (key !== "global") {
                // Determine if this was a peer_id or session_id
                if (group.memories[0].peer_id === key) {
                   op.data.peer_id = key;
                } else if (group.memories[0].session_id === key) {
                   op.data.session_id = key;
                }
              }
              // Ensure category is high-level
              if (!["profile", "pattern", "preferences", "constraint"].includes(op.data.category)) {
                  op.data.category = "profile";
              }
            }
            await executeMutationOp(cm, op);
            operationsExecuted++;
          }
        }
        
        // Mark these messages as processed by updating their category to something else,
        // or deleting them if we want to aggressively compress history. 
        // For safety, let's just change their category so they aren't processed again.
        if (operationsExecuted > 0 || plan.operations.length === 0) {
           console.log(`[dreamer] Marking ${group.memories.length} messages as processed for ${key}`);
           for (const m of group.memories) {
               // We could add a boolean flag, but changing category is easiest for now
               await cm.updateMemory(m.id, { category: "observation" });
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
