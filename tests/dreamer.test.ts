import { describe, it, expect, vi, beforeEach } from "vitest";

// Use doMock which runs inline instead of being hoisted
import { dreamLoop } from "../src/dreamer";
import { planVibeMutation } from "../src/llm/vibeEngine";
import { createContextManager } from "../src/storage/client";

vi.mock("../src/llm/vibeEngine", () => ({
  planVibeMutation: vi.fn(),
  executeMutationOp: vi.fn(),
}));

vi.mock("../src/storage/client", () => {
  return {
    createContextManager: vi.fn(() => {
      // Create a persistent mock object that we can capture and assert on
      if (!(global as any).__mockCm) {
        (global as any).__mockCm = {
          listMemories: vi.fn(),
          searchMemories: vi.fn(),
          updateMemory: vi.fn()
        };
      }
      return (global as any).__mockCm;
    }),
  };
});

describe("Background Dreaming Daemon (Continual Learning)", () => {
  let mockCm: any;
  
  beforeEach(() => {
    vi.clearAllMocks();
    mockCm = (global as any).__mockCm || createContextManager();
    // Prevent the setTimeout from actually running in tests
    vi.spyOn(global, "setTimeout").mockImplementation((() => {}) as any);
  });

  it("should exit cleanly if there are no un-summarized messages", async () => {
    mockCm.listMemories.mockResolvedValueOnce([]);
    
    await dreamLoop();
    
    expect(mockCm.listMemories).toHaveBeenCalledWith({ category: "message", memoryState: "raw" }, 100);
    expect(planVibeMutation).not.toHaveBeenCalled();
  });

  it("should skip peer groups that have less than 5 message observations", async () => {
    mockCm.listMemories.mockResolvedValueOnce([
      { id: "mem1", owner: "user", content: "hello", peer_id: "peerA", session_id: "sess1", category: "message" },
      { id: "mem2", owner: "user", content: "world", peer_id: "peerA", session_id: "sess1", category: "message" }
    ]);

    await dreamLoop();
    
    // It should fetch but skip planning because group length < 5
    expect(planVibeMutation).not.toHaveBeenCalled();
  });

  it("should process peer groups with >= 5 observations, generate mutations, and update old messages", async () => {
    // Generate 5 messages for peerA
    const messages = Array(5).fill(0).map((_, i) => ({
      id: `mem${i}`,
      owner: "user",
      content: `observation ${i}`,
      peer_id: "peerA",
      session_id: "sess1",
      category: "message"
    }));

    mockCm.listMemories.mockResolvedValueOnce(messages);
    mockCm.searchMemories.mockResolvedValue([]);

    // Mock the LLM returning a valid profile extraction
    vi.mocked(planVibeMutation).mockResolvedValueOnce({
      reasoning: "User seems to like testing",
      operations: [
        {
          op: "create_memory",
          target: undefined,
          description: "Noticed a preference",
          data: {
            content: "User likes writing tests",
            category: "profile"
          }
        }
      ]
    } as any);

    await dreamLoop();
    
    // Ensure the vibe engine was called with the combined text
    expect(planVibeMutation).toHaveBeenCalledTimes(1);
    const callArgs = vi.mocked(planVibeMutation).mock.calls[0];
    expect(callArgs[1]).toContain("[user]: observation 0");
    
    // It should have called updateMemory to mark the 5 processed messages
    expect(mockCm.updateMemory).toHaveBeenCalledTimes(5);
    expect(mockCm.updateMemory).toHaveBeenCalledWith(
      "mem0",
      expect.objectContaining({ category: "observation", memory_state: "archived" })
    );
  });
});
