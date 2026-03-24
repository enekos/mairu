import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Mock `dotenv`
vi.mock("dotenv", () => {
  return {
    config: vi.fn(),
  };
});

// Mock `@google/genai`
const mockGenerateContent = vi.fn();
vi.mock("@google/genai", () => {
  return {
    GoogleGenAI: vi.fn().mockImplementation(() => {
      return {
        models: {
          generateContent: mockGenerateContent,
        },
      };
    }),
  };
});

// Mock `console.warn`
vi.spyOn(console, "warn").mockImplementation(() => {});

describe("vibeEngine", () => {
  const originalEnv = process.env;

  beforeEach(() => {
    if (typeof vi.resetModules === "function") {
      vi.resetModules();
    }
    vi.clearAllMocks();
    process.env = { ...originalEnv };
  });

  afterEach(() => {
    process.env = originalEnv;
  });

  // ─────────────────────────────────────────────────────────────────────────
  // planVibeSearch
  // ─────────────────────────────────────────────────────────────────────────

  describe("planVibeSearch", () => {
    it("throws if GEMINI_API_KEY is not set", async () => {
      delete process.env.GEMINI_API_KEY;
      const { planVibeSearch } = await import("../src/llm/vibeEngine");

      await expect(planVibeSearch("test prompt")).rejects.toThrow(
        "GEMINI_API_KEY is not set"
      );
    });

    it("returns parsed search plan on valid LLM response", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      mockGenerateContent.mockResolvedValue({
        text: JSON.stringify({
          reasoning: "Searching memories and nodes for auth info",
          queries: [
            { store: "memory", query: "authentication setup" },
            { store: "node", query: "auth module architecture" },
          ],
        }),
      });

      const { planVibeSearch } = await import("../src/llm/vibeEngine");
      const result = await planVibeSearch("how does auth work?");

      expect(result.reasoning).toBe("Searching memories and nodes for auth info");
      expect(result.queries).toHaveLength(2);
      expect(result.queries[0]).toEqual({ store: "memory", query: "authentication setup" });
      expect(result.queries[1]).toEqual({ store: "node", query: "auth module architecture" });
    });

    it("falls back to direct search on unparseable LLM response", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      mockGenerateContent.mockResolvedValue({ text: "not json at all" });

      const { planVibeSearch } = await import("../src/llm/vibeEngine");
      const result = await planVibeSearch("test prompt");

      expect(result.reasoning).toBe("Falling back to direct search");
      expect(result.queries).toHaveLength(2);
      expect(result.queries[0].store).toBe("memory");
      expect(result.queries[1].store).toBe("node");
    });

    it("filters out invalid store types from LLM response", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      mockGenerateContent.mockResolvedValue({
        text: JSON.stringify({
          reasoning: "test",
          queries: [
            { store: "memory", query: "valid" },
            { store: "invalid_store", query: "should be filtered" },
            { store: "skill", query: "also valid" },
          ],
        }),
      });

      const { planVibeSearch } = await import("../src/llm/vibeEngine");
      const result = await planVibeSearch("test");

      expect(result.queries).toHaveLength(2);
      expect(result.queries[0].store).toBe("memory");
      expect(result.queries[1].store).toBe("skill");
    });

    it("includes project in prompt when provided", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      mockGenerateContent.mockResolvedValue({
        text: JSON.stringify({ reasoning: "ok", queries: [{ store: "memory", query: "test" }] }),
      });

      const { planVibeSearch } = await import("../src/llm/vibeEngine");
      await planVibeSearch("test", "my-project");

      const prompt = mockGenerateContent.mock.calls[0][0].contents;
      expect(prompt).toContain('Project namespace: "my-project"');
    });

    it("retries on 429 status and succeeds", async () => {
      process.env.GEMINI_API_KEY = "fake-key";

      mockGenerateContent.mockRejectedValueOnce({ status: 429 });
      mockGenerateContent.mockResolvedValueOnce({
        text: JSON.stringify({
          reasoning: "retry worked",
          queries: [{ store: "memory", query: "test" }],
        }),
      });

      const { planVibeSearch } = await import("../src/llm/vibeEngine");
      const result = await planVibeSearch("test");

      expect(mockGenerateContent).toHaveBeenCalledTimes(2);
      expect(result.reasoning).toBe("retry worked");
    });
  });

  // ─────────────────────────────────────────────────────────────────────────
  // executeVibeQuery
  // ─────────────────────────────────────────────────────────────────────────

  describe("executeVibeQuery", () => {
    it("runs search queries against the correct stores", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      mockGenerateContent.mockResolvedValue({
        text: JSON.stringify({
          reasoning: "multi-store search",
          queries: [
            { store: "memory", query: "auth patterns" },
            { store: "skill", query: "authentication" },
            { store: "node", query: "auth docs" },
          ],
        }),
      });

      const mockCm = {
        searchMemories: vi.fn().mockResolvedValue([{ id: "mem1", content: "auth memory" }]),
        searchSkills: vi.fn().mockResolvedValue([{ id: "sk1", name: "auth", description: "auth skill" }]),
        searchContext: vi.fn().mockResolvedValue([{ uri: "ctx://auth", name: "Auth", abstract: "auth node" }]),
      };

      const { executeVibeQuery } = await import("../src/llm/vibeEngine");
      const result = await executeVibeQuery(mockCm as any, "how does auth work?", "proj", 3);

      expect(result.reasoning).toBe("multi-store search");
      expect(result.results).toHaveLength(3);

      expect(mockCm.searchMemories).toHaveBeenCalledWith("auth patterns", { topK: 3, project: "proj" });
      expect(mockCm.searchSkills).toHaveBeenCalledWith("authentication", { topK: 3, project: "proj" });
      expect(mockCm.searchContext).toHaveBeenCalledWith("auth docs", { topK: 3, project: "proj" });
    });

    it("handles empty search results gracefully", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      mockGenerateContent.mockResolvedValue({
        text: JSON.stringify({
          reasoning: "search",
          queries: [{ store: "memory", query: "nonexistent" }],
        }),
      });

      const mockCm = {
        searchMemories: vi.fn().mockResolvedValue([]),
      };

      const { executeVibeQuery } = await import("../src/llm/vibeEngine");
      const result = await executeVibeQuery(mockCm as any, "nothing");

      expect(result.results).toHaveLength(1);
      expect(result.results[0].items).toHaveLength(0);
    });
  });

  // ─────────────────────────────────────────────────────────────────────────
  // planVibeMutation
  // ─────────────────────────────────────────────────────────────────────────

  describe("planVibeMutation", () => {
    it("throws if GEMINI_API_KEY is not set", async () => {
      delete process.env.GEMINI_API_KEY;
      const { planVibeMutation } = await import("../src/llm/vibeEngine");

      await expect(planVibeMutation({} as any, "test")).rejects.toThrow(
        "GEMINI_API_KEY is not set"
      );
    });

    it("returns mutation plan with valid operations", async () => {
      process.env.GEMINI_API_KEY = "fake-key";

      // First call: planVibeSearch (search planning)
      mockGenerateContent.mockResolvedValueOnce({
        text: JSON.stringify({
          reasoning: "search",
          queries: [{ store: "memory", query: "test" }],
        }),
      });
      // Second call: mutation planning
      mockGenerateContent.mockResolvedValueOnce({
        text: JSON.stringify({
          reasoning: "Adding a new memory about testing",
          operations: [
            {
              op: "create_memory",
              description: "Store testing preference",
              data: { content: "We use Vitest", category: "observation", owner: "agent", importance: 5 },
            },
          ],
        }),
      });

      const mockCm = {
        searchMemories: vi.fn().mockResolvedValue([]),
      };

      const { planVibeMutation } = await import("../src/llm/vibeEngine");
      const result = await planVibeMutation(mockCm as any, "remember we use vitest");

      expect(result.reasoning).toBe("Adding a new memory about testing");
      expect(result.operations).toHaveLength(1);
      expect(result.operations[0].op).toBe("create_memory");
      expect(result.operations[0].data.content).toBe("We use Vitest");
    });

    it("filters out operations with invalid op types", async () => {
      process.env.GEMINI_API_KEY = "fake-key";

      mockGenerateContent.mockResolvedValueOnce({
        text: JSON.stringify({ reasoning: "s", queries: [{ store: "memory", query: "t" }] }),
      });
      mockGenerateContent.mockResolvedValueOnce({
        text: JSON.stringify({
          reasoning: "plan",
          operations: [
            { op: "create_memory", description: "valid", data: {} },
            { op: "invalid_op", description: "should be filtered", data: {} },
            { op: "delete_skill", description: "also valid", data: {} },
          ],
        }),
      });

      const mockCm = {
        searchMemories: vi.fn().mockResolvedValue([]),
        searchSkills: vi.fn().mockResolvedValue([]),
        searchContext: vi.fn().mockResolvedValue([]),
      };

      const { planVibeMutation } = await import("../src/llm/vibeEngine");
      const result = await planVibeMutation(mockCm as any, "test");

      expect(result.operations).toHaveLength(2);
      expect(result.operations[0].op).toBe("create_memory");
      expect(result.operations[1].op).toBe("delete_skill");
    });

    it("throws on unparseable mutation plan", async () => {
      process.env.GEMINI_API_KEY = "fake-key";

      mockGenerateContent.mockResolvedValueOnce({
        text: JSON.stringify({ reasoning: "s", queries: [{ store: "memory", query: "t" }] }),
      });
      mockGenerateContent.mockResolvedValueOnce({
        text: "totally not json",
      });

      const mockCm = {
        searchMemories: vi.fn().mockResolvedValue([]),
        searchSkills: vi.fn().mockResolvedValue([]),
        searchContext: vi.fn().mockResolvedValue([]),
      };

      const { planVibeMutation } = await import("../src/llm/vibeEngine");
      await expect(planVibeMutation(mockCm as any, "test")).rejects.toThrow(
        /LLM returned unparseable mutation plan/
      );
    });

    it("retries mutation planning with compact payload when first response is unparseable", async () => {
      process.env.GEMINI_API_KEY = "fake-key";

      // Search planning call
      mockGenerateContent.mockResolvedValueOnce({
        text: JSON.stringify({ reasoning: "s", queries: [{ store: "memory", query: "t" }] }),
      });
      // First mutation planning call (bad)
      mockGenerateContent.mockResolvedValueOnce({
        text: "this is not valid json",
      });
      // Second mutation planning call (compact retry succeeds)
      mockGenerateContent.mockResolvedValueOnce({
        text: JSON.stringify({
          reasoning: "compact retry succeeded",
          operations: [],
        }),
      });

      const mockCm = { searchMemories: vi.fn().mockResolvedValue([]) };

      const { planVibeMutation } = await import("../src/llm/vibeEngine");
      const result = await planVibeMutation(mockCm as any, "test");

      expect(result.reasoning).toBe("compact retry succeeded");
      expect(result.operations).toEqual([]);
      expect(mockGenerateContent).toHaveBeenCalledTimes(3);
    });

    it("truncates very large prompts before sending to mutation planner", async () => {
      process.env.GEMINI_API_KEY = "fake-key";

      mockGenerateContent.mockResolvedValueOnce({
        text: JSON.stringify({ reasoning: "s", queries: [{ store: "memory", query: "t" }] }),
      });
      mockGenerateContent.mockResolvedValueOnce({
        text: JSON.stringify({ reasoning: "ok", operations: [] }),
      });

      const mockCm = { searchMemories: vi.fn().mockResolvedValue([]) };
      const veryLargePrompt = `BEGIN\n${"x".repeat(40000)}\nEND`;

      const { planVibeMutation } = await import("../src/llm/vibeEngine");
      await planVibeMutation(mockCm as any, veryLargePrompt);

      const mutationPrompt = mockGenerateContent.mock.calls[1][0].contents as string;
      expect(mutationPrompt).toContain("[truncated");
      expect(mutationPrompt.length).toBeLessThan(30000);
    });

    it("deduplicates existing context entries by id/uri", async () => {
      process.env.GEMINI_API_KEY = "fake-key";

      mockGenerateContent.mockResolvedValueOnce({
        text: JSON.stringify({
          reasoning: "s",
          queries: [
            { store: "memory", query: "auth" },
            { store: "memory", query: "authentication" },
          ],
        }),
      });
      mockGenerateContent.mockResolvedValueOnce({
        text: JSON.stringify({ reasoning: "plan", operations: [] }),
      });

      const mockCm = {
        searchMemories: vi.fn().mockResolvedValue([
          { id: "mem1", content: "auth stuff", _hybrid_score: 0.9 },
        ]),
      };

      const { planVibeMutation } = await import("../src/llm/vibeEngine");
      await planVibeMutation(mockCm as any, "auth");

      // The mutation prompt should only contain mem1 once despite 2 search queries returning it
      const mutationPrompt = mockGenerateContent.mock.calls[1][0].contents;
      const matches = mutationPrompt.match(/"mem1"/g);
      expect(matches).toHaveLength(1);
    });
  });

  // ─────────────────────────────────────────────────────────────────────────
  // executeMutationOp
  // ─────────────────────────────────────────────────────────────────────────

  describe("executeMutationOp", () => {
    it("forwards AI metadata fields on create_memory", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      const mockCm = {
        addMemory: vi.fn().mockResolvedValue({ id: "mem_ai" }),
      };

      const { executeMutationOp } = await import("../src/llm/vibeEngine");
      await executeMutationOp(mockCm as any, {
        op: "create_memory",
        description: "create with ai fields",
        data: {
          content: "Auth uses signed JWTs",
          category: "decision",
          owner: "agent",
          importance: 8,
          ai_intent: "decision",
          ai_topics: ["auth", "security"],
          ai_quality_score: 0.92,
        },
      }, "my-project");

      expect(mockCm.addMemory).toHaveBeenCalledWith(
        "Auth uses signed JWTs",
        "decision",
        "agent",
        8,
        "my-project",
        {},
        false,
        {
          ai_intent: "decision",
          ai_topics: ["auth", "security"],
          ai_quality_score: 0.92,
        }
      );
    });

    it("executes create_memory", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      const mockCm = {
        addMemory: vi.fn().mockResolvedValue({ id: "mem_new123" }),
      };

      const { executeMutationOp } = await import("../src/llm/vibeEngine");
      const result = await executeMutationOp(mockCm as any, {
        op: "create_memory",
        description: "test",
        data: { content: "test content", category: "observation", owner: "agent", importance: 7 },
      }, "my-project");

      expect(result).toContain("Created memory: mem_new123");
      expect(mockCm.addMemory).toHaveBeenCalledWith(
        "test content",
        "observation",
        "agent",
        7,
        "my-project",
        {},
        false,
        {
          ai_intent: undefined,
          ai_topics: undefined,
          ai_quality_score: undefined,
        }
      );
    });

    it("executes update_memory", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      const mockCm = { updateMemory: vi.fn().mockResolvedValue({}) };

      const { executeMutationOp } = await import("../src/llm/vibeEngine");
      const result = await executeMutationOp(mockCm as any, {
        op: "update_memory",
        target: "mem_abc",
        description: "update test",
        data: { content: "updated content" },
      });

      expect(result).toBe("Updated memory: mem_abc");
      expect(mockCm.updateMemory).toHaveBeenCalledWith("mem_abc", { content: "updated content" });
    });

    it("forwards AI metadata fields on update_node", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      const mockCm = { updateContextNode: vi.fn().mockResolvedValue({}) };

      const { executeMutationOp } = await import("../src/llm/vibeEngine");
      const result = await executeMutationOp(mockCm as any, {
        op: "update_node",
        target: "ctx://existing",
        description: "update node ai metadata",
        data: {
          ai_intent: "how_to",
          ai_topics: ["retrieval", "infra"],
          ai_quality_score: 0.75,
        },
      });

      expect(result).toBe("Updated node: ctx://existing");
      expect(mockCm.updateContextNode).toHaveBeenCalledWith("ctx://existing", {
        ai_intent: "how_to",
        ai_topics: ["retrieval", "infra"],
        ai_quality_score: 0.75,
      });
    });

    it("executes delete_memory", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      const mockCm = { deleteMemory: vi.fn().mockResolvedValue(undefined) };

      const { executeMutationOp } = await import("../src/llm/vibeEngine");
      const result = await executeMutationOp(mockCm as any, {
        op: "delete_memory",
        target: "mem_del",
        description: "delete test",
        data: {},
      });

      expect(result).toBe("Deleted memory: mem_del");
    });

    it("executes create_skill", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      const mockCm = {
        addSkill: vi.fn().mockResolvedValue({ id: "skill_new" }),
      };

      const { executeMutationOp } = await import("../src/llm/vibeEngine");
      const result = await executeMutationOp(mockCm as any, {
        op: "create_skill",
        description: "add skill",
        data: { name: "Testing", description: "Run vitest tests" },
      }, "proj");

      expect(result).toContain("Created skill: skill_new");
      expect(mockCm.addSkill).toHaveBeenCalledWith("Testing", "Run vitest tests", "proj", {}, {
        ai_intent: undefined,
        ai_topics: undefined,
        ai_quality_score: undefined,
      });
    });

    it("executes update_skill", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      const mockCm = { updateSkill: vi.fn().mockResolvedValue({}) };

      const { executeMutationOp } = await import("../src/llm/vibeEngine");
      const result = await executeMutationOp(mockCm as any, {
        op: "update_skill",
        target: "skill_1",
        description: "update skill",
        data: { description: "updated desc" },
      });

      expect(result).toBe("Updated skill: skill_1");
    });

    it("executes delete_skill", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      const mockCm = { deleteSkill: vi.fn().mockResolvedValue(undefined) };

      const { executeMutationOp } = await import("../src/llm/vibeEngine");
      const result = await executeMutationOp(mockCm as any, {
        op: "delete_skill",
        target: "skill_del",
        description: "delete skill",
        data: {},
      });

      expect(result).toBe("Deleted skill: skill_del");
    });

    it("executes create_node", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      const mockCm = {
        addContextNode: vi.fn().mockResolvedValue({ uri: "ctx://new" }),
      };

      const { executeMutationOp } = await import("../src/llm/vibeEngine");
      const result = await executeMutationOp(mockCm as any, {
        op: "create_node",
        description: "add node",
        data: { uri: "ctx://new", name: "New Node", abstract: "A new node" },
      }, "proj");

      expect(result).toContain("Created node: ctx://new");
      expect(mockCm.addContextNode).toHaveBeenCalledWith(
        "ctx://new",
        "New Node",
        "A new node",
        undefined,
        undefined,
        null,
        "proj",
        {},
        false,
        {
          ai_intent: undefined,
          ai_topics: undefined,
          ai_quality_score: undefined,
        }
      );
    });

    it("executes update_node", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      const mockCm = { updateContextNode: vi.fn().mockResolvedValue({}) };

      const { executeMutationOp } = await import("../src/llm/vibeEngine");
      const result = await executeMutationOp(mockCm as any, {
        op: "update_node",
        target: "ctx://existing",
        description: "update node",
        data: { abstract: "updated abstract" },
      });

      expect(result).toBe("Updated node: ctx://existing");
    });

    it("executes delete_node", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      const mockCm = { deleteContextNode: vi.fn().mockResolvedValue(undefined) };

      const { executeMutationOp } = await import("../src/llm/vibeEngine");
      const result = await executeMutationOp(mockCm as any, {
        op: "delete_node",
        target: "ctx://del",
        description: "delete node",
        data: {},
      });

      expect(result).toBe("Deleted node: ctx://del");
    });

    it("uses default category/owner/importance for create_memory", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      const mockCm = {
        addMemory: vi.fn().mockResolvedValue({ id: "mem_defaults" }),
      };

      const { executeMutationOp } = await import("../src/llm/vibeEngine");
      await executeMutationOp(mockCm as any, {
        op: "create_memory",
        description: "minimal create",
        data: { content: "just content" },
      });

      // Should use defaults: category=observation, owner=agent, importance=5
      expect(mockCm.addMemory).toHaveBeenCalledWith(
        "just content",
        "observation",
        "agent",
        5,
        undefined,
        {},
        false,
        {
          ai_intent: undefined,
          ai_topics: undefined,
          ai_quality_score: undefined,
        }
      );
    });

    it("returns message for unknown op type", async () => {
      process.env.GEMINI_API_KEY = "fake-key";
      const { executeMutationOp } = await import("../src/llm/vibeEngine");
      const result = await executeMutationOp({} as any, {
        op: "unknown_op" as any,
        description: "test",
        data: {},
      });

      expect(result).toBe("Unknown operation: unknown_op");
    });
  });
});
