import { expect, test } from "@playwright/test";

test.beforeEach(async ({ page }) => {
  await page.route("**/api/dashboard**", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        skills: [{ id: "skill_1", name: "Go API", description: "Builds APIs", updated_at: new Date().toISOString() }],
        memories: [{ id: "mem_1", content: "Use mairu as single name", category: "decision", owner: "agent", importance: 8, updated_at: new Date().toISOString() }],
        contextNodes: [{ uri: "contextfs://demo/root", name: "Root", abstract: "Root node", parent_uri: "", updated_at: new Date().toISOString() }],
      }),
    });
  });

  await page.route("**/api/search**", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        skills: [{ id: "skill_1", name: "Go API", description: "Builds APIs", _hybrid_score: 0.9 }],
        memories: [{ id: "mem_1", content: "Use mairu as single name", category: "decision", owner: "agent", importance: 8, _hybrid_score: 0.8 }],
        contextNodes: [{ uri: "contextfs://demo/root", name: "Root", abstract: "Root node", _hybrid_score: 0.7 }],
      }),
    });
  });

  await page.route("**/api/cluster", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        status: "green",
        clusterName: "mairu-local",
        numberOfNodes: 1,
        activeShards: 3,
        relocatingShards: 0,
        unassignedShards: 0,
        indices: {
          contextfs_memories: { docs: 12, sizeBytes: 12000, deletedDocs: 0 },
          contextfs_skills: { docs: 5, sizeBytes: 5000, deletedDocs: 0 },
          contextfs_context_nodes: { docs: 20, sizeBytes: 22000, deletedDocs: 0 },
        },
      }),
    });
  });

  await page.route("**/api/skills", async (route) => {
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ id: "skill_new", created: true }) });
  });

  await page.route("**/api/memories", async (route) => {
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ id: "mem_new", created: true }) });
  });

  await page.route("**/api/context**", async (route) => {
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ uri: "contextfs://demo/new", created: true }) });
  });

  await page.route("**/api/vibe/query", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        reasoning: "Query executed",
        results: [{ store: "memory", query: "hello", items: [{ id: "m1", content: "result" }] }],
      }),
    });
  });

  await page.route("**/api/vibe/mutation/plan", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        reasoning: "Plan generated",
        operations: [{ op: "create_memory", description: "Create memory", data: { content: "x" } }],
      }),
    });
  });

  await page.route("**/api/vibe/mutation/execute", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ results: [{ op: "create_memory", result: "ok" }] }),
    });
  });
});

test("navigates unified dashboard tabs and runs core workflows", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByRole("button", { name: "Agent" })).toBeVisible();
  await page.getByRole("button", { name: "Overview" }).click();
  await expect(page.getByText("Memory Categories")).toBeVisible();

  await page.getByRole("navigation").getByRole("button", { name: /Skills/ }).click();
  await page.getByRole("button", { name: "+ Add Skill" }).click();
  await page.getByPlaceholder("Skill name").fill("E2E Skill");
  await page.getByPlaceholder(/Description/).fill("Skill from e2e");
  await page.getByRole("button", { name: "Save skill" }).click();

  await page.getByRole("navigation").getByRole("button", { name: /Memories/ }).click();
  await page.getByRole("button", { name: "+ Add Memory" }).click();
  await page.getByPlaceholder(/Memory content/).fill("Remember this");
  await page.getByRole("button", { name: "Save memory" }).click();

  await page.getByRole("navigation").getByRole("button", { name: /Context/ }).click();
  await page.getByRole("button", { name: "Graph" }).click();
  await expect(page.getByText("Root")).toBeVisible();

  await page.getByRole("button", { name: "Search Lab" }).click();
  await page.getByPlaceholder("Search query...").fill("mairu");
  await page.getByRole("button", { name: "Search", exact: true }).click();
  await expect(page.getByText("results for")).toBeVisible();

  await page.getByRole("button", { name: "Vibe" }).click();
  await page.getByPlaceholder(/What do you want to find/).fill("find memory");
  await page.getByRole("button", { name: "Search", exact: true }).click();
  await expect(page.getByText("Query executed")).toBeVisible();

  await page.getByRole("button", { name: "Mutation" }).click();
  await page.getByRole("textbox").first().fill("remember migration");
  await page.getByRole("button", { name: "Plan" }).click();
  await page.getByRole("button", { name: /Execute 1 operation/ }).click();
  await expect(page.getByText("Mutations applied")).toBeVisible();

  await page.getByRole("button", { name: "Cluster" }).click();
  await expect(page.getByText("mairu-local")).toBeVisible();
});

