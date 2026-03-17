<script lang="ts">
  // @ts-nocheck
  const API_BASE = import.meta.env.VITE_DASHBOARD_API_BASE || "http://localhost:8787";

  type Tab = "overview" | "skills" | "memories" | "context";

  let loading = false;
  let searching = false;
  let error = "";
  let skills: any[] = [];
  let memories: any[] = [];
  let contextNodes: any[] = [];
  let activeTab: Tab = "overview";

  // Search state
  let searchQuery = "";
  let searchMode: "filter" | "vector" = "filter";
  let searchResults: { skills?: any[]; memories?: any[]; contextNodes?: any[] } = {};
  let hasSearchResults = false;
  let searchDebounce: ReturnType<typeof setTimeout>;

  // Displayed rows (either filtered local or vector search results)
  $: displaySkills = hasSearchResults
    ? (searchResults.skills ?? [])
    : searchQuery
    ? skills.filter(s =>
        s.name?.toLowerCase().includes(searchQuery.toLowerCase()) ||
        s.description?.toLowerCase().includes(searchQuery.toLowerCase())
      )
    : skills;

  $: displayMemories = hasSearchResults
    ? (searchResults.memories ?? [])
    : searchQuery
    ? memories.filter(m =>
        m.content?.toLowerCase().includes(searchQuery.toLowerCase()) ||
        m.category?.toLowerCase().includes(searchQuery.toLowerCase())
      )
    : memories;

  $: displayContext = hasSearchResults
    ? (searchResults.contextNodes ?? [])
    : searchQuery
    ? contextNodes.filter(c =>
        c.name?.toLowerCase().includes(searchQuery.toLowerCase()) ||
        c.uri?.toLowerCase().includes(searchQuery.toLowerCase()) ||
        c.abstract?.toLowerCase().includes(searchQuery.toLowerCase())
      )
    : contextNodes;

  // Form states
  let newSkill = { name: "", description: "" };
  let newMemory = { content: "", category: "observation", owner: "agent", importance: 5, useRouter: true };
  let newContext = { uri: "", parent_uri: "", name: "", abstract: "", overview: "", useRouter: true };
  let addingSkill = false;
  let addingMemory = false;
  let addingContext = false;
  let lastWriteResult: any = null;

  // ── Data loading ────────────────────────────────────────────────────────────

  async function load() {
    loading = true; error = "";
    try {
      const res = await fetch(`${API_BASE}/api/dashboard?limit=500`);
      if (!res.ok) throw new Error(`API ${res.status}`);
      const d = await res.json();
      skills = d.skills ?? [];
      memories = d.memories ?? [];
      contextNodes = d.contextNodes ?? [];
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  // ── Search ──────────────────────────────────────────────────────────────────

  function onSearchInput() {
    clearTimeout(searchDebounce);
    if (searchMode === "vector" && searchQuery.length > 2) {
      searchDebounce = setTimeout(runVectorSearch, 450);
    } else {
      hasSearchResults = false;
      searchResults = {};
    }
  }

  async function runVectorSearch() {
    if (!searchQuery.trim()) { hasSearchResults = false; return; }
    searching = true;
    try {
      const type = activeTab === "overview" ? "all"
                 : activeTab === "skills" ? "skills"
                 : activeTab === "memories" ? "memories"
                 : "context";
      const res = await fetch(`${API_BASE}/api/search?q=${encodeURIComponent(searchQuery)}&type=${type}&topK=15`);
      if (!res.ok) throw new Error(`Search API ${res.status}`);
      searchResults = await res.json();
      hasSearchResults = true;
    } catch (e: any) {
      error = e.message;
    } finally {
      searching = false;
    }
  }

  function clearSearch() {
    searchQuery = "";
    hasSearchResults = false;
    searchResults = {};
  }

  // ── Write helpers ───────────────────────────────────────────────────────────

  async function createSkill() {
    addingSkill = true; lastWriteResult = null;
    try {
      const res = await fetch(`${API_BASE}/api/skills`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(newSkill),
      });
      if (!res.ok) throw new Error("Failed to create skill");
      newSkill = { name: "", description: "" };
      await load();
    } catch (e: any) { error = e.message; }
    finally { addingSkill = false; }
  }

  async function createMemory() {
    addingMemory = true; lastWriteResult = null;
    try {
      const res = await fetch(`${API_BASE}/api/memories`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(newMemory),
      });
      if (!res.ok) throw new Error("Failed to create memory");
      const result = await res.json();
      lastWriteResult = result;
      newMemory = { content: "", category: "observation", owner: "agent", importance: 5, useRouter: true };
      await load();
    } catch (e: any) { error = e.message; }
    finally { addingMemory = false; }
  }

  async function createContext() {
    addingContext = true; lastWriteResult = null;
    try {
      const res = await fetch(`${API_BASE}/api/context`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(newContext),
      });
      if (!res.ok) throw new Error("Failed to create context node");
      const result = await res.json();
      lastWriteResult = result;
      newContext = { uri: "", parent_uri: "", name: "", abstract: "", overview: "", useRouter: true };
      await load();
    } catch (e: any) { error = e.message; }
    finally { addingContext = false; }
  }

  async function del(type: string, idParam: string, value: string) {
    if (!confirm(`Delete this ${type}?`)) return;
    loading = true;
    try {
      const res = await fetch(`${API_BASE}/api/${type}?${idParam}=${encodeURIComponent(value)}`, { method: "DELETE" });
      if (!res.ok) throw new Error("Delete failed");
      await load();
    } catch (e: any) { error = e.message; }
    finally { loading = false; }
  }

  // ── Formatting helpers ──────────────────────────────────────────────────────

  function copy(text: string) { navigator.clipboard.writeText(text); }

  function fmt(v: unknown): string {
    if (v === null || v === undefined) return "";
    if (typeof v === "object") return JSON.stringify(v, null, 2);
    return String(v);
  }

  function fmtDate(s: string): string {
    if (!s) return "";
    return new Date(s).toLocaleDateString(undefined, { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" });
  }

  function scoreColor(s: number): string {
    if (s >= 0.8) return "#22c55e";
    if (s >= 0.6) return "#f59e0b";
    if (s >= 0.4) return "#f97316";
    return "#ef4444";
  }

  function impColor(n: number): string {
    if (n >= 8) return "imp-high";
    if (n >= 5) return "imp-med";
    return "imp-low";
  }

  const categoryColors: Record<string, string> = {
    profile: "#6366f1", preferences: "#8b5cf6", entities: "#0ea5e9",
    events: "#14b8a6", cases: "#f59e0b", patterns: "#f97316",
    observation: "#64748b", reflection: "#a855f7", decision: "#ef4444",
    constraint: "#dc2626", architecture: "#2563eb",
  };

  load();
</script>

<main>
  <header>
    <div class="header-brand">
      <span class="logo">⬡</span>
      <span class="brand-name">contextfs</span>
      <span class="brand-version">v2</span>
    </div>

    <nav class="tabs">
      <button class:active={activeTab === "overview"} on:click={() => { activeTab = "overview"; clearSearch(); }}>
        Overview
      </button>
      <button class:active={activeTab === "skills"} on:click={() => { activeTab = "skills"; clearSearch(); }}>
        Skills <span class="count">{skills.length}</span>
      </button>
      <button class:active={activeTab === "memories"} on:click={() => { activeTab = "memories"; clearSearch(); }}>
        Memories <span class="count">{memories.length}</span>
      </button>
      <button class:active={activeTab === "context"} on:click={() => { activeTab = "context"; clearSearch(); }}>
        Context <span class="count">{contextNodes.length}</span>
      </button>
    </nav>

    <div class="header-actions">
      {#if activeTab !== "overview"}
        <div class="search-wrap">
          <select bind:value={searchMode} on:change={clearSearch}>
            <option value="filter">Filter</option>
            <option value="vector">Vector search</option>
          </select>
          <input
            type="text"
            placeholder={searchMode === "vector" ? "Semantic query…" : "Filter…"}
            bind:value={searchQuery}
            on:input={onSearchInput}
          />
          {#if searching}
            <span class="spin">⟳</span>
          {:else if searchQuery}
            <button class="clear-btn" on:click={clearSearch}>✕</button>
          {/if}
        </div>
        {#if hasSearchResults}
          <span class="search-badge">vector</span>
        {/if}
      {/if}
      <button class="btn-refresh" on:click={load} disabled={loading}>
        {loading ? "…" : "↻"}
      </button>
    </div>
  </header>

  {#if error}
    <div class="error-bar">
      <strong>Error:</strong> {error}
      <button on:click={() => error = ""}>✕</button>
    </div>
  {/if}

  {#if lastWriteResult}
    <div class="write-result" class:skipped={lastWriteResult.skipped} class:updated={lastWriteResult.updated}>
      {#if lastWriteResult.skipped}
        ⏭ Skipped — {lastWriteResult.reason} (existing: <code>{lastWriteResult.existingId?.slice(0, 12)}…</code>)
      {:else if lastWriteResult.updated}
        ✎ Merged into existing entry <code>{lastWriteResult.id?.slice(0, 12)}…</code>
      {:else}
        ✓ Created <code>{(lastWriteResult.id || lastWriteResult.uri)?.slice(0, 20)}…</code>
      {/if}
      <button on:click={() => lastWriteResult = null}>✕</button>
    </div>
  {/if}

  <div class="content">

    <!-- OVERVIEW -->
    {#if activeTab === "overview"}
      <section class="overview-grid">
        <article class="stat-card" on:click={() => activeTab = "skills"} role="button" tabindex="0">
          <div class="stat-icon">⚡</div>
          <div class="stat-body">
            <div class="stat-num">{skills.length}</div>
            <div class="stat-label">Skills</div>
          </div>
        </article>
        <article class="stat-card" on:click={() => activeTab = "memories"} role="button" tabindex="0">
          <div class="stat-icon">◈</div>
          <div class="stat-body">
            <div class="stat-num">{memories.length}</div>
            <div class="stat-label">Memories</div>
          </div>
        </article>
        <article class="stat-card" on:click={() => activeTab = "context"} role="button" tabindex="0">
          <div class="stat-icon">⬡</div>
          <div class="stat-body">
            <div class="stat-num">{contextNodes.length}</div>
            <div class="stat-label">Context Nodes</div>
          </div>
        </article>
      </section>

      {#if memories.length > 0}
        <section class="recent-section">
          <h3>Recent memories</h3>
          <ul class="recent-list">
            {#each memories.slice(0, 5) as m}
              <li>
                <span class="cat-dot" style="background:{categoryColors[m.category] || '#64748b'}"></span>
                <span class="recent-content">{m.content}</span>
                <span class="recent-date">{fmtDate(m.updated_at || m.created_at)}</span>
              </li>
            {/each}
          </ul>
        </section>
      {/if}

      {#if contextNodes.length > 0}
        <section class="recent-section">
          <h3>Recent context nodes</h3>
          <ul class="recent-list">
            {#each contextNodes.slice(0, 5) as c}
              <li>
                <span class="cat-dot" style="background:#2563eb"></span>
                <span class="recent-content"><code>{c.uri}</code> — {c.abstract?.slice(0, 80)}{c.abstract?.length > 80 ? "…" : ""}</span>
                <span class="recent-date">{fmtDate(c.updated_at || c.created_at)}</span>
              </li>
            {/each}
          </ul>
        </section>
      {/if}

    <!-- SKILLS -->
    {:else if activeTab === "skills"}
      <section class="add-panel">
        <button class="toggle-add" on:click={() => addingSkill = !addingSkill}>
          {addingSkill ? "▲ Close" : "+ Add Skill"}
        </button>
        {#if addingSkill}
          <form on:submit|preventDefault={createSkill} class="add-form">
            <input type="text" placeholder="Skill name" bind:value={newSkill.name} required />
            <textarea rows="3" placeholder="Description — what this skill does and when to use it" bind:value={newSkill.description} required></textarea>
            <div class="form-footer">
              <button type="submit" class="btn-primary" disabled={loading}>Save skill</button>
            </div>
          </form>
        {/if}
      </section>

      <section class="table-section">
        {#if displaySkills.length === 0}
          <p class="empty">No skills{searchQuery ? " matching your query" : ""}.</p>
        {:else}
          <table>
            <thead>
              <tr>
                <th style="width:40%">Name / Description</th>
                {#if hasSearchResults}<th>Score</th>{/if}
                <th>Updated</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {#each displaySkills as row}
                <tr>
                  <td>
                    <div class="name-cell">
                      <strong>{row.name}</strong>
                      <span class="desc">{row.description}</span>
                      <button class="copy-btn" on:click={() => copy(row.id)} title="Copy ID">⎘ {row.id?.slice(0,8)}</button>
                    </div>
                  </td>
                  {#if hasSearchResults}
                    <td>
                      <span class="score-badge" style="color:{scoreColor(row._hybrid_score ?? 0)}">
                        {((row._hybrid_score ?? 0) * 100).toFixed(0)}%
                      </span>
                      <div class="score-detail">
                        vec {((row._vector_score ?? 0)*100).toFixed(0)}
                        kw {((row._keyword_score ?? 0)*100).toFixed(0)}
                      </div>
                    </td>
                  {/if}
                  <td class="date-cell">{fmtDate(row.updated_at || row.created_at)}</td>
                  <td><button class="btn-del" on:click={() => del("skills", "id", row.id)}>✕</button></td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </section>

    <!-- MEMORIES -->
    {:else if activeTab === "memories"}
      <section class="add-panel">
        <button class="toggle-add" on:click={() => addingMemory = !addingMemory}>
          {addingMemory ? "▲ Close" : "+ Add Memory"}
        </button>
        {#if addingMemory}
          <form on:submit|preventDefault={createMemory} class="add-form">
            <textarea rows="3" placeholder="Memory content — write as a self-contained fact" bind:value={newMemory.content} required></textarea>
            <div class="form-row">
              <label>
                Category
                <select bind:value={newMemory.category}>
                  {#each ["observation","reflection","profile","preferences","entities","events","cases","patterns","decision","constraint","architecture"] as cat}
                    <option value={cat}>{cat}</option>
                  {/each}
                </select>
              </label>
              <label>
                Owner
                <select bind:value={newMemory.owner}>
                  <option value="agent">agent</option>
                  <option value="user">user</option>
                  <option value="system">system</option>
                </select>
              </label>
              <label>
                Importance
                <input type="number" min="1" max="10" bind:value={newMemory.importance} style="width:60px" />
              </label>
            </div>
            <div class="form-footer">
              <label class="router-toggle">
                <input type="checkbox" bind:checked={newMemory.useRouter} />
                Smart dedup (LLM router)
              </label>
              <button type="submit" class="btn-primary" disabled={loading}>Save memory</button>
            </div>
          </form>
        {/if}
      </section>

      <section class="table-section">
        {#if displayMemories.length === 0}
          <p class="empty">No memories{searchQuery ? " matching your query" : ""}.</p>
        {:else}
          <table>
            <thead>
              <tr>
                <th style="width:45%">Content</th>
                <th>Cat / Owner</th>
                <th>Imp</th>
                {#if hasSearchResults}<th>Score</th>{/if}
                <th>Updated</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {#each displayMemories as row}
                <tr>
                  <td>
                    <div class="content-cell">
                      {row.content}
                      <button class="copy-btn" on:click={() => copy(row.id)} title="Copy ID">⎘ {row.id?.slice(0,8)}</button>
                    </div>
                  </td>
                  <td>
                    <span class="cat-badge" style="background:{categoryColors[row.category] || '#64748b'}">{row.category}</span>
                    <div class="owner-text">{row.owner}</div>
                  </td>
                  <td>
                    <span class="imp-badge {impColor(row.importance)}">{row.importance}</span>
                  </td>
                  {#if hasSearchResults}
                    <td>
                      <span class="score-badge" style="color:{scoreColor(row._hybrid_score ?? 0)}">
                        {((row._hybrid_score ?? 0) * 100).toFixed(0)}%
                      </span>
                      <div class="score-detail">
                        vec {((row._vector_score ?? 0)*100).toFixed(0)}
                        kw {((row._keyword_score ?? 0)*100).toFixed(0)}
                        imp {((row._importance_score ?? 0)*100).toFixed(0)}
                      </div>
                    </td>
                  {/if}
                  <td class="date-cell">{fmtDate(row.updated_at || row.created_at)}</td>
                  <td><button class="btn-del" on:click={() => del("memories", "id", row.id)}>✕</button></td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </section>

    <!-- CONTEXT NODES -->
    {:else if activeTab === "context"}
      <section class="add-panel">
        <button class="toggle-add" on:click={() => addingContext = !addingContext}>
          {addingContext ? "▲ Close" : "+ Add Context Node"}
        </button>
        {#if addingContext}
          <form on:submit|preventDefault={createContext} class="add-form">
            <div class="form-row">
              <input type="text" placeholder="URI e.g. contextfs://project/backend/auth" bind:value={newContext.uri} required style="flex:2" />
              <input type="text" placeholder="Parent URI" bind:value={newContext.parent_uri} style="flex:1" />
            </div>
            <input type="text" placeholder="Name" bind:value={newContext.name} required />
            <textarea rows="2" placeholder="Abstract — ~100-token summary used for search" bind:value={newContext.abstract} required></textarea>
            <textarea rows="3" placeholder="Overview (optional, ~2k tokens)" bind:value={newContext.overview}></textarea>
            <div class="form-footer">
              <label class="router-toggle">
                <input type="checkbox" bind:checked={newContext.useRouter} />
                Smart dedup (LLM router)
              </label>
              <button type="submit" class="btn-primary" disabled={loading}>Save node</button>
            </div>
          </form>
        {/if}
      </section>

      <section class="table-section">
        {#if displayContext.length === 0}
          <p class="empty">No context nodes{searchQuery ? " matching your query" : ""}.</p>
        {:else}
          <table>
            <thead>
              <tr>
                <th style="width:30%">URI / Name</th>
                <th style="width:40%">Abstract</th>
                {#if hasSearchResults}<th>Score</th>{/if}
                <th>Updated</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {#each displayContext as row}
                <tr>
                  <td>
                    <div class="uri-cell">
                      <button class="uri-text" on:click={() => copy(row.uri)} title="Copy URI">{row.uri}</button>
                      <span class="node-name">{row.name}</span>
                      {#if row.parent_uri}
                        <span class="parent-uri">↳ {row.parent_uri}</span>
                      {/if}
                    </div>
                  </td>
                  <td class="abstract-cell">{row.abstract}</td>
                  {#if hasSearchResults}
                    <td>
                      <span class="score-badge" style="color:{scoreColor(row._hybrid_score ?? 0)}">
                        {((row._hybrid_score ?? 0) * 100).toFixed(0)}%
                      </span>
                      <div class="score-detail">
                        vec {((row._vector_score ?? 0)*100).toFixed(0)}
                        kw {((row._keyword_score ?? 0)*100).toFixed(0)}
                      </div>
                    </td>
                  {/if}
                  <td class="date-cell">{fmtDate(row.updated_at || row.created_at)}</td>
                  <td><button class="btn-del" on:click={() => del("context", "uri", row.uri)}>✕</button></td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </section>
    {/if}
  </div>
</main>

<style>
  :global(*) { box-sizing: border-box; margin: 0; padding: 0; }
  :global(body) {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    font-size: 14px;
    background: #0f172a;
    color: #e2e8f0;
    min-height: 100vh;
  }

  main { display: flex; flex-direction: column; min-height: 100vh; }

  /* Header */
  header {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 12px 20px;
    background: #1e293b;
    border-bottom: 1px solid #334155;
    position: sticky; top: 0; z-index: 10;
  }

  .header-brand { display: flex; align-items: center; gap: 6px; }
  .logo { font-size: 20px; color: #6366f1; }
  .brand-name { font-weight: 700; font-size: 15px; color: #f1f5f9; }
  .brand-version { font-size: 10px; color: #64748b; background: #334155; padding: 1px 5px; border-radius: 4px; }

  .tabs { display: flex; gap: 2px; flex: 1; }
  .tabs button {
    background: none; border: none; color: #94a3b8; padding: 6px 14px;
    border-radius: 6px; cursor: pointer; font-size: 13px; font-weight: 500;
    display: flex; align-items: center; gap: 6px;
    transition: background 0.15s, color 0.15s;
  }
  .tabs button:hover { background: #334155; color: #e2e8f0; }
  .tabs button.active { background: #312e81; color: #a5b4fc; }
  .count {
    background: #334155; color: #94a3b8; font-size: 11px;
    padding: 1px 6px; border-radius: 10px;
  }

  .header-actions { display: flex; align-items: center; gap: 8px; }

  .search-wrap {
    display: flex; align-items: center; gap: 4px;
    background: #0f172a; border: 1px solid #334155; border-radius: 8px;
    padding: 2px 8px; width: 320px;
  }
  .search-wrap select {
    background: none; border: none; color: #64748b; font-size: 12px;
    outline: none; cursor: pointer; padding-right: 4px;
  }
  .search-wrap input {
    background: none; border: none; color: #e2e8f0; font-size: 13px;
    outline: none; flex: 1; padding: 4px 0;
  }
  .search-wrap input::placeholder { color: #475569; }
  .spin { color: #6366f1; animation: spin 1s linear infinite; font-size: 16px; }
  @keyframes spin { to { transform: rotate(360deg); } }
  .clear-btn { background: none; border: none; color: #475569; cursor: pointer; font-size: 14px; }
  .search-badge {
    font-size: 10px; background: #312e81; color: #a5b4fc;
    padding: 2px 8px; border-radius: 10px; font-weight: 600;
  }

  .btn-refresh {
    background: #1e293b; border: 1px solid #334155; color: #94a3b8;
    padding: 6px 10px; border-radius: 8px; cursor: pointer; font-size: 16px;
  }
  .btn-refresh:hover { background: #334155; }

  /* Banners */
  .error-bar {
    display: flex; align-items: center; gap: 12px;
    padding: 10px 20px; background: #450a0a; color: #fca5a5;
    border-bottom: 1px solid #7f1d1d;
  }
  .error-bar button { margin-left: auto; background: none; border: none; color: #fca5a5; cursor: pointer; }

  .write-result {
    display: flex; align-items: center; gap: 12px;
    padding: 10px 20px; border-bottom: 1px solid #334155;
    background: #0f2a1c; color: #86efac;
  }
  .write-result.skipped { background: #1c1a0f; color: #fde68a; }
  .write-result.updated { background: #0f1a2a; color: #93c5fd; }
  .write-result button { margin-left: auto; background: none; border: none; color: inherit; cursor: pointer; }
  .write-result code { font-size: 12px; opacity: 0.8; }

  /* Content */
  .content { flex: 1; padding: 20px; max-width: 1400px; margin: 0 auto; width: 100%; }

  /* Overview */
  .overview-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 16px; margin-bottom: 28px; }
  .stat-card {
    background: #1e293b; border: 1px solid #334155; border-radius: 12px;
    padding: 20px 24px; display: flex; gap: 16px; align-items: center;
    cursor: pointer; transition: border-color 0.15s, background 0.15s;
  }
  .stat-card:hover { border-color: #6366f1; background: #1e2a4a; }
  .stat-icon { font-size: 28px; color: #6366f1; }
  .stat-num { font-size: 32px; font-weight: 700; color: #f1f5f9; line-height: 1; }
  .stat-label { font-size: 13px; color: #94a3b8; margin-top: 2px; }

  .recent-section { margin-bottom: 20px; }
  .recent-section h3 { font-size: 13px; color: #64748b; text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 10px; }
  .recent-list { list-style: none; display: flex; flex-direction: column; gap: 6px; }
  .recent-list li {
    display: flex; align-items: baseline; gap: 10px;
    padding: 8px 12px; background: #1e293b; border-radius: 8px; border: 1px solid #334155;
  }
  .cat-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
  .recent-content { flex: 1; color: #cbd5e1; font-size: 13px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .recent-date { color: #475569; font-size: 12px; flex-shrink: 0; }

  /* Add panel */
  .add-panel { margin-bottom: 16px; }
  .toggle-add {
    background: #1e293b; border: 1px solid #334155; color: #94a3b8;
    padding: 7px 14px; border-radius: 8px; cursor: pointer; font-size: 13px;
    transition: background 0.15s;
  }
  .toggle-add:hover { background: #334155; color: #e2e8f0; }

  .add-form {
    display: flex; flex-direction: column; gap: 10px;
    background: #1e293b; border: 1px solid #334155; border-radius: 10px;
    padding: 16px; margin-top: 10px;
  }
  .add-form input, .add-form textarea, .add-form select {
    background: #0f172a; border: 1px solid #334155; color: #e2e8f0;
    border-radius: 6px; padding: 8px 10px; font-size: 13px; outline: none; width: 100%;
    font-family: inherit; resize: vertical;
  }
  .add-form input:focus, .add-form textarea:focus { border-color: #6366f1; }
  .form-row { display: flex; gap: 12px; }
  .form-row label { display: flex; flex-direction: column; gap: 4px; font-size: 12px; color: #64748b; flex: 1; }
  .form-row label select { margin-top: 2px; }
  .form-footer { display: flex; align-items: center; gap: 12px; justify-content: flex-end; }
  .router-toggle { display: flex; align-items: center; gap: 6px; color: #64748b; font-size: 12px; cursor: pointer; margin-right: auto; }
  .btn-primary {
    background: #4f46e5; color: #fff; border: none;
    padding: 8px 18px; border-radius: 7px; cursor: pointer; font-size: 13px; font-weight: 500;
  }
  .btn-primary:hover { background: #4338ca; }
  .btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }

  /* Tables */
  .table-section { overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; }
  thead th {
    text-align: left; font-size: 11px; font-weight: 600;
    color: #64748b; text-transform: uppercase; letter-spacing: 0.05em;
    padding: 8px 12px; border-bottom: 1px solid #334155;
  }
  tbody tr { border-bottom: 1px solid #1e293b; transition: background 0.1s; }
  tbody tr:hover { background: #1e293b; }
  tbody td { padding: 10px 12px; vertical-align: top; }

  .name-cell { display: flex; flex-direction: column; gap: 3px; }
  .name-cell strong { color: #f1f5f9; font-size: 14px; }
  .desc { color: #94a3b8; font-size: 12px; line-height: 1.4; }

  .content-cell { display: flex; flex-direction: column; gap: 3px; color: #cbd5e1; }

  .copy-btn {
    background: none; border: none; color: #475569; cursor: pointer; font-size: 11px;
    padding: 0; width: fit-content;
  }
  .copy-btn:hover { color: #94a3b8; }

  .cat-badge {
    display: inline-block; padding: 2px 7px; border-radius: 4px;
    font-size: 11px; font-weight: 600; color: #fff;
  }
  .owner-text { font-size: 11px; color: #475569; margin-top: 3px; }

  .imp-badge {
    display: inline-block; width: 28px; height: 28px; border-radius: 50%;
    text-align: center; line-height: 28px; font-size: 12px; font-weight: 700;
  }
  .imp-high { background: #14532d; color: #86efac; }
  .imp-med { background: #1c1917; color: #fde68a; border: 1px solid #78350f; }
  .imp-low { background: #1e293b; color: #64748b; }

  .score-badge { font-weight: 700; font-size: 15px; }
  .score-detail { font-size: 10px; color: #475569; margin-top: 2px; white-space: nowrap; }

  .date-cell { color: #475569; font-size: 12px; white-space: nowrap; }

  .uri-cell { display: flex; flex-direction: column; gap: 2px; }
  .uri-text {
    background: none; border: none; color: #818cf8; cursor: pointer; font-size: 12px;
    font-family: monospace; text-align: left; padding: 0;
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 260px;
  }
  .uri-text:hover { text-decoration: underline; }
  .node-name { font-size: 13px; color: #e2e8f0; font-weight: 500; }
  .parent-uri { font-size: 11px; color: #475569; font-family: monospace; }

  .abstract-cell { color: #94a3b8; font-size: 13px; line-height: 1.5; }

  .btn-del {
    background: none; border: none; color: #475569; cursor: pointer;
    font-size: 14px; padding: 4px 8px; border-radius: 6px;
  }
  .btn-del:hover { background: #450a0a; color: #fca5a5; }

  .empty { color: #475569; font-size: 14px; padding: 40px 0; text-align: center; }
</style>
