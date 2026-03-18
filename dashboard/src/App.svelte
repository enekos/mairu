<script lang="ts">
  import "./App.css";
  import OverviewTab from "./components/OverviewTab.svelte";
  import SkillsTab from "./components/SkillsTab.svelte";
  import MemoriesTab from "./components/MemoriesTab.svelte";
  import ContextTab from "./components/ContextTab.svelte";
  import SearchLabTab from "./components/SearchLabTab.svelte";
  import VibeTab from "./components/VibeTab.svelte";

  const API_BASE = import.meta.env.VITE_DASHBOARD_API_BASE || "http://localhost:8787";

  type Tab = "overview" | "skills" | "memories" | "context" | "search" | "vibe";

  let loading = false;
  let searching = false;
  let error = "";
  let skills: Record<string, unknown>[] = [];
  let memories: Record<string, unknown>[] = [];
  let contextNodes: Record<string, unknown>[] = [];
  let activeTab: Tab = "overview";

  // Search state
  let searchQuery = "";
  let searchMode: "filter" | "vector" = "filter";
  let searchResults: { skills?: unknown[]; memories?: unknown[]; contextNodes?: unknown[] } = {};
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

  let lastWriteResult: unknown = null;

  async function load() {
    loading = true; error = "";
    try {
      const res = await fetch(`${API_BASE}/api/dashboard?limit=500`);
      if (!res.ok) throw new Error(`API ${res.status}`);
      const d = await res.json();
      skills = d.skills ?? [];
      memories = d.memories ?? [];
      contextNodes = d.contextNodes ?? [];
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      loading = false;
    }
  }

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
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      searching = false;
    }
  }

  function clearSearch() {
    searchQuery = "";
    hasSearchResults = false;
    searchResults = {};
  }

  function setActiveTab(tab: string) {
    activeTab = tab as Tab;
    clearSearch();
  }

  function setLoading(l: boolean) { loading = l; }
  function setError(e: string) { error = e; }
  function setLastWriteResult(r: unknown) { lastWriteResult = r; }

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
      <button class:active={activeTab === "overview"} on:click={() => setActiveTab("overview")}>
        Overview
      </button>
      <button class:active={activeTab === "skills"} on:click={() => setActiveTab("skills")}>
        Skills <span class="count">{skills.length}</span>
      </button>
      <button class:active={activeTab === "memories"} on:click={() => setActiveTab("memories")}>
        Memories <span class="count">{memories.length}</span>
      </button>
      <button class:active={activeTab === "context"} on:click={() => setActiveTab("context")}>
        Context <span class="count">{contextNodes.length}</span>
      </button>
      <button class:active={activeTab === "search"} on:click={() => setActiveTab("search")}>
        Search Lab
      </button>
      <button class:active={activeTab === "vibe"} on:click={() => setActiveTab("vibe")}>
        Vibe
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
    {#if activeTab === "overview"}
      <OverviewTab 
        {skills} 
        {memories} 
        {contextNodes} 
        {setActiveTab} 
      />
    {:else if activeTab === "skills"}
      <SkillsTab 
        {displaySkills} 
        {hasSearchResults} 
        {searchQuery} 
        {API_BASE} 
        {load} 
        {setLoading} 
        {setError} 
        {setLastWriteResult}
        {loading} 
      />
    {:else if activeTab === "memories"}
      <MemoriesTab 
        {displayMemories} 
        {hasSearchResults} 
        {searchQuery} 
        {API_BASE} 
        {load} 
        {setLoading} 
        {setError} 
        {setLastWriteResult} 
        {loading}
      />
    {:else if activeTab === "context"}
      <ContextTab 
        {displayContext} 
        {hasSearchResults} 
        {searchQuery} 
        {API_BASE} 
        {load} 
        {setLoading} 
        {setError} 
        {setLastWriteResult} 
        {loading}
      />
    {:else if activeTab === "search"}
      <SearchLabTab {API_BASE} />
    {:else if activeTab === "vibe"}
      <VibeTab {API_BASE} />
    {/if}
  </div>
</main>
