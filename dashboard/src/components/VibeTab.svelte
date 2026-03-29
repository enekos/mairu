<script lang="ts">
  import { fmtDate, categoryColors, scoreColor, impColor } from "../lib/utils";

  export let API_BASE: string;

  type Mode = "query" | "mutation";
  let mode: Mode = "query";
  let prompt = "";
  let project = "";
  let topK = 5;
  let loading = false;
  let error = "";

  // Query state
  let queryResult: { reasoning: string; results: Array<{ store: string; query: string; items: Record<string, any>[] }> } | null = null;

  // Mutation state
  let mutationPlan: { reasoning: string; operations: Array<{ op: string; target?: string; description: string; data: Record<string, any> }> } | null = null;
  let selectedOps: boolean[] = [];
  let executing = false;
  let executionResults: Array<{ op: string; result?: string; error?: string }> | null = null;

  // History
  let history: Array<{ mode: Mode; prompt: string; timestamp: Date; reasoning: string }> = [];

  async function runVibeQuery() {
    if (!prompt.trim()) return;
    loading = true; error = ""; queryResult = null; mutationPlan = null; executionResults = null;
    try {
      const res = await fetch(`${API_BASE}/api/vibe/query`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ prompt, project: project || undefined, topK }),
      });
      if (!res.ok) throw new Error(`API ${res.status}: ${(await res.text()).slice(0, 200)}`);
      queryResult = await res.json();
      history = [{ mode: "query", prompt, timestamp: new Date(), reasoning: queryResult?.reasoning || "" }, ...history.slice(0, 19)];
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      loading = false;
    }
  }

  async function runVibeMutation() {
    if (!prompt.trim()) return;
    loading = true; error = ""; queryResult = null; mutationPlan = null; executionResults = null;
    try {
      const res = await fetch(`${API_BASE}/api/vibe/mutation/plan`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ prompt, project: project || undefined, topK }),
      });
      if (!res.ok) throw new Error(`API ${res.status}: ${(await res.text()).slice(0, 200)}`);
      mutationPlan = await res.json();
      selectedOps = (mutationPlan?.operations || []).map(() => true);
      history = [{ mode: "mutation", prompt, timestamp: new Date(), reasoning: mutationPlan?.reasoning || "" }, ...history.slice(0, 19)];
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      loading = false;
    }
  }

  async function executeApproved() {
    if (!mutationPlan) return;
    const approved = mutationPlan.operations.filter((_, i) => selectedOps[i]);
    if (approved.length === 0) return;
    executing = true; error = "";
    try {
      const res = await fetch(`${API_BASE}/api/vibe/mutation/execute`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ operations: approved, project: project || undefined }),
      });
      if (!res.ok) throw new Error(`API ${res.status}`);
      const data = await res.json();
      executionResults = data.results;
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      executing = false;
    }
  }

  function submit() {
    if (mode === "query") runVibeQuery();
    else runVibeMutation();
  }

  function keydown(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); submit(); }
  }

  function toggleAll() {
    const allSelected = selectedOps.every(Boolean);
    selectedOps = selectedOps.map(() => !allSelected);
  }

  function opColor(op: string): string {
    if (op.startsWith("create")) return "#22c55e";
    if (op.startsWith("delete")) return "#ef4444";
    return "#f59e0b";
  }

  function opSymbol(op: string): string {
    if (op.startsWith("create")) return "+";
    if (op.startsWith("delete")) return "-";
    return "~";
  }

  $: totalItems = queryResult ? queryResult.results.reduce((sum, r) => sum + r.items.length, 0) : 0;
  $: approvedCount = selectedOps.filter(Boolean).length;
</script>

<section class="vibe-section">
  <!-- Mode toggle + input -->
  <div class="vibe-header">
    <h2 class="vibe-title">Vibe Engine</h2>
    <p class="vibe-desc">
      {#if mode === "query"}
        Ask anything in plain English. The LLM plans and runs semantic searches across all stores.
      {:else}
        Describe changes in plain English. The LLM plans mutations, you review and approve.
      {/if}
    </p>
  </div>

  <div class="vibe-controls">
    <div class="vibe-mode-toggle">
      <button class:active={mode === "query"} on:click={() => { mode = "query"; mutationPlan = null; executionResults = null; }}>
        Query
      </button>
      <button class:active={mode === "mutation"} on:click={() => { mode = "mutation"; queryResult = null; }}>
        Mutation
      </button>
    </div>

    <div class="vibe-input-row">
      <textarea
        class="vibe-input"
        placeholder={mode === "query" ? "What do you want to find? e.g. 'Show me all auth-related decisions'" : "What do you want to change? e.g. 'Mark all testing memories as importance 8'"}
        bind:value={prompt}
        on:keydown={keydown}
        disabled={loading || executing}
        rows="2"
      ></textarea>

      <div class="vibe-input-actions">
        <div class="vibe-opts">
          <label>
            Project
            <input type="text" class="vibe-opt-input" placeholder="(all)" bind:value={project} />
          </label>
          <label>
            Top K
            <input type="number" class="vibe-opt-input vibe-opt-num" min="1" max="50" bind:value={topK} />
          </label>
        </div>
        <button
          class="btn-primary vibe-submit"
          on:click={submit}
          disabled={loading || executing || !prompt.trim()}
        >
          {#if loading}
            Thinking...
          {:else if mode === "query"}
            Search
          {:else}
            Plan
          {/if}
        </button>
      </div>
    </div>
  </div>

  {#if error}
    <div class="vibe-error">
      <strong>Error:</strong> {error}
      <button on:click={() => error = ""}>x</button>
    </div>
  {/if}

  <!-- Query Results -->
  {#if queryResult}
    <div class="vibe-reasoning">
      <span class="vibe-reasoning-label">Strategy</span>
      {queryResult.reasoning}
    </div>

    <div class="vibe-meta">
      {totalItems} result{totalItems !== 1 ? "s" : ""} across {queryResult.results.length} quer{queryResult.results.length !== 1 ? "ies" : "y"}
    </div>

    {#each queryResult.results as group}
      <div class="vibe-group">
        <div class="vibe-group-header">
          <span class="lab-type-badge lab-type-{group.store}">{group.store}</span>
          <span class="vibe-group-query">"{group.query}"</span>
          <span class="vibe-group-count">{group.items.length} result{group.items.length !== 1 ? "s" : ""}</span>
        </div>

        {#if group.items.length === 0}
          <p class="vibe-no-results">No results</p>
        {:else}
          <div class="vibe-items">
            {#each group.items as item, i}
              <div class="vibe-item">
                <div class="vibe-item-rank">#{i + 1}</div>
                <div class="vibe-item-body">
                  <div class="vibe-item-top">
                    {#if group.store === "memory"}
                      <span class="lab-type-cat" style="background:{categoryColors[item.category] || '#64748b'}">{item.category}</span>
                      <span class="lab-imp-badge {impColor(item.importance)}">{item.importance}</span>
                      {#if item.owner}<span class="vibe-item-owner">{item.owner}</span>{/if}
                    {:else if group.store === "skill"}
                      <strong class="vibe-item-title">{item.name}</strong>
                    {:else}
                      <code class="lab-uri">{item.uri}</code>
                      <strong class="vibe-item-title">{item.name}</strong>
                    {/if}
                    {#if item._hybrid_score !== undefined}
                      <span class="vibe-score" style="color:{scoreColor(item._hybrid_score)}">
                        {(item._hybrid_score * 100).toFixed(1)}%
                      </span>
                    {/if}
                  </div>
                  <div class="vibe-item-content">
                    {#if group.store === "memory"}
                      {item.content}
                    {:else if group.store === "skill"}
                      {item.description}
                    {:else}
                      {item.abstract}
                      {#if item.overview}
                        <div class="vibe-item-overview">{item.overview}</div>
                      {/if}
                    {/if}
                  </div>
                  {#if item.project}
                    <span class="vibe-item-project">{item.project}</span>
                  {/if}
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {/each}
  {/if}

  <!-- Mutation Plan -->
  {#if mutationPlan}
    <div class="vibe-reasoning">
      <span class="vibe-reasoning-label">Plan</span>
      {mutationPlan.reasoning}
    </div>

    {#if mutationPlan.operations.length === 0}
      <p class="vibe-no-results">No mutations needed. The LLM determined no changes are necessary.</p>
    {:else}
      <div class="vibe-mutation-header">
        <span>{mutationPlan.operations.length} operation{mutationPlan.operations.length !== 1 ? "s" : ""} planned</span>
        <button class="vibe-select-all" on:click={toggleAll}>
          {selectedOps.every(Boolean) ? "Deselect all" : "Select all"}
        </button>
      </div>

      <div class="vibe-ops">
        {#each mutationPlan.operations as op, i}
          <div class="vibe-op" class:vibe-op-selected={selectedOps[i]} class:vibe-op-executed={executionResults !== null}>
            {#if !executionResults}
              <label class="vibe-op-check">
                <input type="checkbox" bind:checked={selectedOps[i]} />
              </label>
            {/if}
            <div class="vibe-op-badge" style="color:{opColor(op.op)}">
              {opSymbol(op.op)}
            </div>
            <div class="vibe-op-body">
              <div class="vibe-op-top">
                <span class="vibe-op-type" style="color:{opColor(op.op)}">{op.op}</span>
                {#if op.target}
                  <code class="vibe-op-target">{op.target}</code>
                {/if}
              </div>
              <div class="vibe-op-desc">{op.description}</div>
              {#if Object.keys(op.data).length > 0}
                <div class="vibe-op-data">
                  {#each Object.entries(op.data) as [key, value]}
                    <div class="vibe-op-field" style="color:{opColor(op.op)}">
                      <span class="vibe-op-field-key">{key}:</span>
                      <span class="vibe-op-field-val" style="white-space: pre-wrap; word-break: break-word;">{typeof value === "string" ? value : JSON.stringify(value)}</span>
                    </div>
                  {/each}
                </div>
              {/if}
              {#if executionResults && executionResults[i]}
                <div class="vibe-op-result" class:vibe-op-error={executionResults[i].error}>
                  {#if executionResults[i].error}
                    Failed: {executionResults[i].error}
                  {:else}
                    {executionResults[i].result}
                  {/if}
                </div>
              {/if}
            </div>
          </div>
        {/each}
      </div>

      {#if !executionResults}
        <div class="vibe-execute-bar">
          <button
            class="btn-primary vibe-execute-btn"
            on:click={executeApproved}
            disabled={executing || approvedCount === 0}
          >
            {#if executing}
              Executing...
            {:else}
              Execute {approvedCount} operation{approvedCount !== 1 ? "s" : ""}
            {/if}
          </button>
          <span class="vibe-execute-hint">Review the plan above, then execute approved operations.</span>
        </div>
      {:else}
        <div class="vibe-done-bar">
          Mutations applied. {executionResults.filter(r => !r.error).length}/{executionResults.length} succeeded.
        </div>
      {/if}
    {/if}
  {/if}

  <!-- History sidebar -->
  {#if history.length > 0 && !queryResult && !mutationPlan}
    <div class="vibe-history">
      <h3 class="vibe-history-title">Recent</h3>
      {#each history as h}
        <button class="vibe-history-item" on:click={() => { prompt = h.prompt; mode = h.mode; }}>
          <span class="vibe-history-mode" class:vibe-history-mutation={h.mode === "mutation"}>
            {h.mode === "query" ? "Q" : "M"}
          </span>
          <span class="vibe-history-prompt">{h.prompt}</span>
        </button>
      {/each}
    </div>
  {/if}

  {#if !loading && !queryResult && !mutationPlan && history.length === 0}
    <div class="vibe-empty">
      <div class="vibe-empty-icon">~</div>
      <p>Type a prompt and press <kbd>Enter</kbd> or click <strong>{mode === "query" ? "Search" : "Plan"}</strong>.</p>
      <div class="vibe-examples">
        <p class="vibe-examples-title">Try these:</p>
        <button class="vibe-example" on:click={() => { prompt = "What testing frameworks and patterns are used?"; mode = "query"; }}>
          "What testing frameworks and patterns are used?"
        </button>
        <button class="vibe-example" on:click={() => { prompt = "Show me all architecture decisions"; mode = "query"; }}>
          "Show me all architecture decisions"
        </button>
        <button class="vibe-example" on:click={() => { prompt = "Remember that we now use Bun instead of Node"; mode = "mutation"; }}>
          "Remember that we now use Bun instead of Node"
        </button>
      </div>
    </div>
  {/if}
</section>

<style>
  .vibe-section { display: flex; flex-direction: column; gap: 20px; }

  .vibe-title { font-size: 18px; font-weight: 700; color: #f1f5f9; margin-bottom: 4px; }
  .vibe-desc { font-size: 13px; color: #64748b; }

  .vibe-controls {
    display: flex; flex-direction: column; gap: 12px;
    background: #1e293b; border: 1px solid #334155; border-radius: 12px;
    padding: 16px;
  }

  .vibe-mode-toggle { display: flex; gap: 2px; }
  .vibe-mode-toggle button {
    background: none; border: 1px solid #334155; color: #94a3b8;
    padding: 6px 16px; font-size: 13px; font-weight: 500; cursor: pointer;
    transition: all 0.15s;
  }
  .vibe-mode-toggle button:first-child { border-radius: 7px 0 0 7px; }
  .vibe-mode-toggle button:last-child { border-radius: 0 7px 7px 0; }
  .vibe-mode-toggle button.active {
    background: #312e81; border-color: #4f46e5; color: #a5b4fc;
  }

  .vibe-input-row { display: flex; gap: 12px; align-items: flex-start; }

  .vibe-input {
    flex: 1; background: #0f172a; border: 1px solid #334155; color: #e2e8f0;
    border-radius: 8px; padding: 10px 12px; font-size: 14px; outline: none;
    font-family: inherit; resize: vertical; min-height: 44px;
  }
  .vibe-input:focus { border-color: #6366f1; }
  .vibe-input:disabled { opacity: 0.5; }

  .vibe-input-actions { display: flex; flex-direction: column; gap: 8px; min-width: 160px; }

  .vibe-opts { display: flex; gap: 8px; }
  .vibe-opts label {
    display: flex; flex-direction: column; gap: 2px;
    font-size: 11px; color: #475569;
  }
  .vibe-opt-input {
    background: #0f172a; border: 1px solid #334155; color: #e2e8f0;
    border-radius: 5px; padding: 4px 6px; font-size: 12px; outline: none;
    width: 80px;
  }
  .vibe-opt-num { width: 50px; text-align: center; }

  .vibe-submit { padding: 9px 20px; font-size: 13px; white-space: nowrap; }

  /* Error */
  .vibe-error {
    display: flex; align-items: center; gap: 12px;
    padding: 10px 14px; background: #450a0a; color: #fca5a5;
    border-radius: 8px; font-size: 13px;
  }
  .vibe-error button { margin-left: auto; background: none; border: none; color: #fca5a5; cursor: pointer; }

  /* Reasoning */
  .vibe-reasoning {
    background: #1a1a2e; border: 1px solid #2d2b55;
    border-radius: 8px; padding: 12px 14px;
    font-size: 13px; color: #a5b4fc; line-height: 1.5;
  }
  .vibe-reasoning-label {
    display: inline-block; font-size: 10px; font-weight: 700;
    text-transform: uppercase; letter-spacing: 0.05em;
    color: #6366f1; margin-right: 8px;
    background: #312e81; padding: 2px 7px; border-radius: 4px;
  }

  .vibe-meta {
    font-size: 12px; color: #475569;
    padding: 4px 0; border-bottom: 1px solid #1e293b;
  }

  /* Query result groups */
  .vibe-group { display: flex; flex-direction: column; gap: 8px; }
  .vibe-group-header {
    display: flex; align-items: center; gap: 8px;
    padding: 8px 0 4px;
  }
  .vibe-group-query { font-size: 12px; color: #818cf8; font-style: italic; }
  .vibe-group-count { font-size: 11px; color: #475569; margin-left: auto; }

  .vibe-no-results { color: #475569; font-size: 13px; padding: 12px 0; }

  .vibe-items { display: flex; flex-direction: column; gap: 8px; }

  .vibe-item {
    display: flex; gap: 12px;
    background: #1e293b; border: 1px solid #334155; border-radius: 10px;
    padding: 12px 14px; transition: border-color 0.15s;
  }
  .vibe-item:hover { border-color: #475569; }

  .vibe-item-rank { font-size: 12px; font-weight: 700; color: #475569; min-width: 24px; padding-top: 2px; }
  .vibe-item-body { flex: 1; display: flex; flex-direction: column; gap: 6px; }
  .vibe-item-top { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
  .vibe-item-title { color: #f1f5f9; font-size: 13px; }
  .vibe-item-owner { font-size: 11px; color: #475569; }
  .vibe-score { font-size: 12px; font-weight: 700; margin-left: auto; }
  .vibe-item-content { font-size: 13px; color: #94a3b8; line-height: 1.5; white-space: pre-wrap; word-break: break-word; }
  .vibe-item-overview { margin-top: 4px; font-size: 12px; color: #64748b; white-space: pre-wrap; word-break: break-word; }
  .vibe-item-project {
    display: inline-block; font-size: 10px; color: #64748b;
    background: #0f172a; padding: 2px 6px; border-radius: 4px; width: fit-content;
  }

  /* Mutation plan */
  .vibe-mutation-header {
    display: flex; align-items: center; justify-content: space-between;
    padding: 4px 0;
    font-size: 13px; color: #94a3b8;
  }
  .vibe-select-all {
    background: none; border: 1px solid #334155; color: #94a3b8;
    padding: 4px 10px; border-radius: 6px; cursor: pointer; font-size: 12px;
  }
  .vibe-select-all:hover { background: #334155; }

  .vibe-ops { display: flex; flex-direction: column; gap: 8px; }

  .vibe-op {
    display: flex; gap: 12px; align-items: flex-start;
    background: #1e293b; border: 1px solid #334155; border-radius: 10px;
    padding: 12px 14px; transition: all 0.15s;
  }
  .vibe-op-selected { border-color: #4f46e5; background: #1e2640; }
  .vibe-op-executed { opacity: 0.7; }

  .vibe-op-check { display: flex; align-items: center; padding-top: 2px; cursor: pointer; }
  .vibe-op-check input { accent-color: #6366f1; width: 16px; height: 16px; cursor: pointer; }

  .vibe-op-badge {
    font-size: 18px; font-weight: 700; font-family: monospace;
    min-width: 20px; text-align: center; padding-top: 1px;
  }

  .vibe-op-body { flex: 1; display: flex; flex-direction: column; gap: 6px; }
  .vibe-op-top { display: flex; align-items: center; gap: 8px; }
  .vibe-op-type { font-size: 12px; font-weight: 700; font-family: monospace; }
  .vibe-op-target { font-size: 11px; color: #818cf8; }
  .vibe-op-desc { font-size: 13px; color: #cbd5e1; white-space: pre-wrap; word-break: break-word; }

  .vibe-op-data {
    display: flex; flex-direction: column; gap: 2px;
    background: #0f172a; border-radius: 6px; padding: 8px 10px;
    font-family: monospace; font-size: 12px;
  }
  .vibe-op-field { display: flex; gap: 6px; }
  .vibe-op-field-key { color: #64748b; min-width: 80px; }
  .vibe-op-field-val { color: #94a3b8; word-break: break-word; }

  .vibe-op-result {
    font-size: 12px; color: #86efac;
    background: #0f2a1c; padding: 6px 10px; border-radius: 6px;
    margin-top: 4px;
  }
  .vibe-op-error { color: #fca5a5; background: #2a0f0f; }

  .vibe-execute-bar {
    display: flex; align-items: center; gap: 16px;
    padding: 12px 0;
  }
  .vibe-execute-btn { padding: 10px 24px; font-size: 14px; }
  .vibe-execute-hint { font-size: 12px; color: #475569; }

  .vibe-done-bar {
    padding: 12px 16px; background: #0f2a1c; border: 1px solid #166534;
    border-radius: 8px; color: #86efac; font-size: 13px; font-weight: 500;
  }

  /* History */
  .vibe-history { display: flex; flex-direction: column; gap: 6px; }
  .vibe-history-title {
    font-size: 12px; color: #475569; text-transform: uppercase;
    letter-spacing: 0.05em; margin-bottom: 4px;
  }
  .vibe-history-item {
    display: flex; align-items: center; gap: 10px;
    background: #1e293b; border: 1px solid #334155; border-radius: 8px;
    padding: 8px 12px; cursor: pointer; text-align: left;
    transition: border-color 0.15s; color: inherit;
  }
  .vibe-history-item:hover { border-color: #475569; }
  .vibe-history-mode {
    display: inline-flex; align-items: center; justify-content: center;
    width: 22px; height: 22px; border-radius: 5px;
    font-size: 11px; font-weight: 700;
    background: #312e81; color: #a5b4fc;
  }
  .vibe-history-mutation { background: #3b1c0a; color: #fdba74; }
  .vibe-history-prompt {
    font-size: 13px; color: #94a3b8;
    overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1;
  }

  /* Empty state */
  .vibe-empty {
    display: flex; flex-direction: column; align-items: center;
    gap: 16px; padding: 48px 20px; text-align: center; color: #475569;
  }
  .vibe-empty-icon { font-size: 48px; color: #334155; font-family: monospace; font-weight: 700; }
  .vibe-empty p { font-size: 14px; }
  .vibe-empty kbd {
    background: #1e293b; border: 1px solid #334155; border-radius: 4px;
    padding: 1px 5px; font-size: 11px; color: #94a3b8;
  }

  .vibe-examples {
    display: flex; flex-direction: column; gap: 6px; align-items: center;
    margin-top: 8px;
  }
  .vibe-examples-title { font-size: 12px; color: #64748b; margin-bottom: 4px; }
  .vibe-example {
    background: #1e293b; border: 1px solid #334155; border-radius: 8px;
    padding: 8px 16px; cursor: pointer; color: #818cf8; font-size: 13px;
    font-style: italic; transition: border-color 0.15s;
  }
  .vibe-example:hover { border-color: #6366f1; }
</style>
