<script lang="ts">
  // @ts-nocheck
  import "./App.css";
  const API_BASE = import.meta.env.VITE_DASHBOARD_API_BASE || "http://localhost:8787";

  type Tab = "overview" | "skills" | "memories" | "context" | "search";

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

  // ── Search Lab ──────────────────────────────────────────────────────────────

  let labQuery = "";
  let labType: "all" | "skills" | "memories" | "context" = "all";
  let labTopK = 10;
  let labSearching = false;
  let labResults: any[] = [];
  let labError = "";
  let labSearched = false;

  async function runLabSearch() {
    if (!labQuery.trim()) return;
    labSearching = true; labError = ""; labSearched = false;
    try {
      const res = await fetch(
        `${API_BASE}/api/search?q=${encodeURIComponent(labQuery)}&type=${labType}&topK=${labTopK}`
      );
      if (!res.ok) throw new Error(`Search API ${res.status}`);
      const data = await res.json();
      // Flatten all results into a single ranked list
      const flat: any[] = [];
      (data.skills ?? []).forEach((r: any) => flat.push({ ...r, _type: "skill" }));
      (data.memories ?? []).forEach((r: any) => flat.push({ ...r, _type: "memory" }));
      (data.contextNodes ?? []).forEach((r: any) => flat.push({ ...r, _type: "context" }));
      flat.sort((a, b) => (b._hybrid_score ?? 0) - (a._hybrid_score ?? 0));
      labResults = flat;
      labSearched = true;
    } catch (e: any) {
      labError = e.message;
    } finally {
      labSearching = false;
    }
  }

  function labKeydown(e: KeyboardEvent) {
    if (e.key === "Enter") runLabSearch();
  }

  // Form states
  let newSkill = { name: "", description: "" };
  let newMemory = { content: "", category: "observation", owner: "agent", importance: 5, useRouter: true };
  let newContext = { uri: "", parent_uri: "", name: "", abstract: "", overview: "", useRouter: true };
  let addingSkill = false;
  let addingMemory = false;
  let addingContext = false;
  let lastWriteResult: any = null;

  // Edit states
  let editingId: string | null = null;
  let editingType: "skill" | "memory" | "context" | null = null;
  let editForm: any = {};

  function startEdit(type: "skill" | "memory" | "context", item: any) {
    editingType = type;
    editingId = type === "context" ? item.uri : item.id;
    editForm = { ...item };
  }

  function cancelEdit() {
    editingId = null;
    editingType = null;
    editForm = {};
  }

  async function saveEdit() {
    loading = true; error = ""; lastWriteResult = null;
    try {
      const endpoint = editingType === "skill" ? "skills" : editingType === "memory" ? "memories" : "context";
      const res = await fetch(`${API_BASE}/api/${endpoint}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(editForm),
      });
      if (!res.ok) throw new Error(`Failed to update ${editingType}`);
      const result = await res.json();
      lastWriteResult = result;
      cancelEdit();
      await load();
    } catch (e: any) { error = e.message; }
    finally { loading = false; }
  }

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
      <button class:active={activeTab === "search"} on:click={() => { activeTab = "search"; clearSearch(); }}>
        Search Lab
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
                  {#if editingId === row.id && editingType === "skill"}
                    <td colspan={hasSearchResults ? 4 : 3}>
                      <div class="edit-form-inline">
                        <input type="text" bind:value={editForm.name} placeholder="Name" class="edit-input" />
                        <textarea bind:value={editForm.description} placeholder="Description" rows="2" class="edit-textarea"></textarea>
                        <div class="edit-actions">
                          <button class="btn-primary" on:click={saveEdit} disabled={loading}>Save</button>
                          <button class="btn-cancel" on:click={cancelEdit} disabled={loading}>Cancel</button>
                        </div>
                      </div>
                    </td>
                  {:else}
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
                    <td class="row-actions">
                      <button class="btn-edit" on:click={() => startEdit("skill", row)} title="Edit">✎</button>
                      <button class="btn-del" on:click={() => del("skills", "id", row.id)} title="Delete">✕</button>
                    </td>
                  {/if}
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
                  {#if editingId === row.id && editingType === "memory"}
                    <td colspan={hasSearchResults ? 6 : 5}>
                      <div class="edit-form-inline">
                        <textarea bind:value={editForm.content} placeholder="Content" rows="3" class="edit-textarea"></textarea>
                        <div class="edit-row">
                          <label>
                            Category:
                            <select bind:value={editForm.category}>
                              {#each ["observation","reflection","profile","preferences","entities","events","cases","patterns","decision","constraint","architecture"] as cat}
                                <option value={cat}>{cat}</option>
                              {/each}
                            </select>
                          </label>
                          <label>
                            Owner:
                            <select bind:value={editForm.owner}>
                              <option value="agent">agent</option>
                              <option value="user">user</option>
                              <option value="system">system</option>
                            </select>
                          </label>
                          <label>
                            Importance:
                            <input type="number" min="1" max="10" bind:value={editForm.importance} style="width:60px" />
                          </label>
                        </div>
                        <div class="edit-actions">
                          <button class="btn-primary" on:click={saveEdit} disabled={loading}>Save</button>
                          <button class="btn-cancel" on:click={cancelEdit} disabled={loading}>Cancel</button>
                        </div>
                      </div>
                    </td>
                  {:else}
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
                    <td class="row-actions">
                      <button class="btn-edit" on:click={() => startEdit("memory", row)} title="Edit">✎</button>
                      <button class="btn-del" on:click={() => del("memories", "id", row.id)} title="Delete">✕</button>
                    </td>
                  {/if}
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
                  {#if editingId === row.uri && editingType === "context"}
                    <td colspan={hasSearchResults ? 5 : 4}>
                      <div class="edit-form-inline">
                        <input type="text" bind:value={editForm.name} placeholder="Name" class="edit-input" />
                        <textarea bind:value={editForm.abstract} placeholder="Abstract" rows="2" class="edit-textarea"></textarea>
                        <textarea bind:value={editForm.overview} placeholder="Overview" rows="2" class="edit-textarea"></textarea>
                        <textarea bind:value={editForm.content} placeholder="Content" rows="3" class="edit-textarea"></textarea>
                        <div class="edit-actions">
                          <button class="btn-primary" on:click={saveEdit} disabled={loading}>Save</button>
                          <button class="btn-cancel" on:click={cancelEdit} disabled={loading}>Cancel</button>
                        </div>
                      </div>
                    </td>
                  {:else}
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
                    <td class="row-actions">
                      <button class="btn-edit" on:click={() => startEdit("context", row)} title="Edit">✎</button>
                      <button class="btn-del" on:click={() => del("context", "uri", row.uri)} title="Delete">✕</button>
                    </td>
                  {/if}
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      </section>
    <!-- SEARCH LAB -->
    {:else if activeTab === "search"}
      <section class="lab-section">
        <div class="lab-header">
          <h2 class="lab-title">Search Lab</h2>
          <p class="lab-desc">Test semantic search exactly as an agent would — same query path, same scoring.</p>
        </div>

        <div class="lab-controls">
          <input
            class="lab-input"
            type="text"
            placeholder="Enter a search query…"
            bind:value={labQuery}
            on:keydown={labKeydown}
            disabled={labSearching}
          />
          <select class="lab-select" bind:value={labType}>
            <option value="all">All types</option>
            <option value="skills">Skills</option>
            <option value="memories">Memories</option>
            <option value="context">Context</option>
          </select>
          <label class="lab-topk-label">
            Top <input class="lab-topk" type="number" min="1" max="50" bind:value={labTopK} />
          </label>
          <button class="btn-primary lab-run" on:click={runLabSearch} disabled={labSearching || !labQuery.trim()}>
            {labSearching ? "Searching…" : "Search"}
          </button>
        </div>

        {#if labError}
          <p class="lab-error">{labError}</p>
        {/if}

        {#if labSearched}
          <div class="lab-meta">
            {labResults.length} result{labResults.length !== 1 ? "s" : ""}
            {#if labType !== "all"}· type: <strong>{labType}</strong>{/if}
            · topK: <strong>{labTopK}</strong>
            · query: <em>"{labQuery}"</em>
          </div>

          {#if labResults.length === 0}
            <p class="empty">No results found.</p>
          {:else}
            <div class="lab-results">
              {#each labResults as row, i}
                <div class="lab-card">
                  <div class="lab-card-rank">#{i + 1}</div>

                  <div class="lab-card-body">
                    <div class="lab-card-top">
                      <span class="lab-type-badge lab-type-{row._type}">{row._type}</span>
                      {#if row._type === "skill"}
                        <strong class="lab-card-title">{row.name}</strong>
                      {:else if row._type === "memory"}
                        <span class="lab-type-cat" style="background:{categoryColors[row.category] || '#64748b'}">{row.category}</span>
                        <span class="lab-imp-badge {impColor(row.importance)}">{row.importance}</span>
                      {:else}
                        <code class="lab-uri">{row.uri}</code>
                      {/if}
                      <span class="lab-date">{fmtDate(row.updated_at || row.created_at)}</span>
                    </div>

                    <div class="lab-card-content">
                      {#if row._type === "skill"}
                        {row.description}
                      {:else if row._type === "memory"}
                        {row.content}
                        {#if row.owner}<span class="lab-owner">— {row.owner}</span>{/if}
                      {:else}
                        <strong>{row.name}</strong>
                        {#if row.abstract}<span class="lab-abstract"> — {row.abstract}</span>{/if}
                      {/if}
                    </div>

                    <!-- Score breakdown -->
                    <div class="lab-scores">
                      <div class="lab-score-main">
                        <span class="lab-score-label">hybrid</span>
                        <div class="lab-bar-wrap">
                          <div class="lab-bar" style="width:{((row._hybrid_score ?? 0) * 100).toFixed(1)}%;background:{scoreColor(row._hybrid_score ?? 0)}"></div>
                        </div>
                        <span class="lab-score-val" style="color:{scoreColor(row._hybrid_score ?? 0)}">
                          {((row._hybrid_score ?? 0) * 100).toFixed(1)}%
                        </span>
                      </div>
                      <div class="lab-score-subs">
                        {#each [
                          { key: "_vector_score", label: "vector" },
                          { key: "_keyword_score", label: "keyword" },
                          { key: "_recency_score", label: "recency" },
                          { key: "_importance_score", label: "importance" },
                        ] as s}
                          {#if row[s.key] !== undefined && row[s.key] !== null}
                            <div class="lab-score-sub">
                              <span class="lab-score-sub-label">{s.label}</span>
                              <div class="lab-bar-wrap lab-bar-wrap-sm">
                                <div class="lab-bar lab-bar-sm" style="width:{(row[s.key] * 100).toFixed(1)}%"></div>
                              </div>
                              <span class="lab-score-sub-val">{(row[s.key] * 100).toFixed(1)}%</span>
                            </div>
                          {/if}
                        {/each}
                      </div>
                    </div>
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        {:else if !labSearching}
          <div class="lab-empty-state">
            <div class="lab-empty-icon">⌕</div>
            <p>Enter a query above and press <kbd>Enter</kbd> or click <strong>Search</strong>.</p>
            <p class="lab-empty-hint">Results are ranked using the same hybrid scorer agents use — vector similarity + keyword overlap + recency + importance.</p>
          </div>
        {/if}
      </section>

    {/if}
  </div>
</main>

