<script lang="ts">
  import { fade } from 'svelte/transition';
  import { onMount } from "svelte";
  import { connectWs } from "./lib/store";
  import Chat from "./lib/Chat.svelte";
  import "./context/App.css";

  import OverviewTab from "./context/components/OverviewTab.svelte";
  import SkillsTab from "./context/components/SkillsTab.svelte";
  import MemoriesTab from "./context/components/MemoriesTab.svelte";
  import ContextTab from "./context/components/ContextTab.svelte";
  import SearchLabTab from "./context/components/SearchLabTab.svelte";
  import VibeTab from "./context/components/VibeTab.svelte";

  const API_BASE = import.meta.env.VITE_DASHBOARD_API_BASE || "";

  type Tab = "agent" | "overview" | "skills" | "memories" | "context" | "search" | "vibe";

  let loading = false;
  let searching = false;
  let error = "";
  let skills: Record<string, any>[] = [];
  let memories: Record<string, any>[] = [];
  let contextNodes: Record<string, any>[] = [];
  let activeTab: Tab = "agent";

  let searchQuery = "";
  let searchMode: "filter" | "vector" = "filter";
  let searchResults: { skills?: any[]; memories?: any[]; contextNodes?: any[] } = {};
  let hasSearchResults = false;
  let searchDebounce: ReturnType<typeof setTimeout>;
  let lastWriteResult: any = null;

  $: displaySkills = hasSearchResults
    ? (searchResults.skills ?? [])
    : searchQuery
      ? skills.filter((s) =>
          String(s.name || "").toLowerCase().includes(searchQuery.toLowerCase()) ||
          String(s.description || "").toLowerCase().includes(searchQuery.toLowerCase())
        )
      : skills;

  $: displayMemories = hasSearchResults
    ? (searchResults.memories ?? [])
    : searchQuery
      ? memories.filter((m) =>
          String(m.content || "").toLowerCase().includes(searchQuery.toLowerCase()) ||
          String(m.category || "").toLowerCase().includes(searchQuery.toLowerCase())
        )
      : memories;

  $: displayContext = hasSearchResults
    ? (searchResults.contextNodes ?? [])
    : searchQuery
      ? contextNodes.filter((c) =>
          String(c.name || "").toLowerCase().includes(searchQuery.toLowerCase()) ||
          String(c.uri || "").toLowerCase().includes(searchQuery.toLowerCase()) ||
          String(c.abstract || "").toLowerCase().includes(searchQuery.toLowerCase())
        )
      : contextNodes;

  async function load() {
    loading = true;
    error = "";
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
      searchDebounce = setTimeout(runVectorSearch, 350);
    } else {
      hasSearchResults = false;
      searchResults = {};
    }
  }

  async function runVectorSearch() {
    if (!searchQuery.trim()) {
      hasSearchResults = false;
      return;
    }
    searching = true;
    try {
      const type = activeTab === "skills" ? "skills"
        : activeTab === "memories" ? "memories"
        : activeTab === "context" ? "context"
        : "all";
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
    if (tab !== "skills" && tab !== "memories" && tab !== "context") {
      clearSearch();
    }
  }

  onMount(() => {
    connectWs();
    load();
  });
</script>

<main>
  <header>
    <div class="header-brand">
      <span class="brand-name">mairu</span>
    </div>
    <nav class="tabs">
      <button class:active={activeTab === "agent"} on:click={() => setActiveTab("agent")}>Agent</button>
      <button class:active={activeTab === "overview"} on:click={() => setActiveTab("overview")}>Overview</button>
      <button class:active={activeTab === "skills"} on:click={() => setActiveTab("skills")}>
        Skills <span class="count">{skills.length}</span>
      </button>
      <button class:active={activeTab === "memories"} on:click={() => setActiveTab("memories")}>
        Memories <span class="count">{memories.length}</span>
      </button>
      <button class:active={activeTab === "context"} on:click={() => setActiveTab("context")}>
        Context <span class="count">{contextNodes.length}</span>
      </button>
      <div class="tab-separator"></div>
      <button class:active={activeTab === "search"} on:click={() => setActiveTab("search")}>Search Lab</button>
      <button class:active={activeTab === "vibe"} on:click={() => setActiveTab("vibe")}>Vibe</button>
    </nav>
    <div class="header-actions">
      {#if ["skills", "memories", "context"].includes(activeTab)}
        <div class="search-wrap">
          <select bind:value={searchMode} on:change={clearSearch}>
            <option value="filter">Filter</option>
            <option value="vector">Vector search</option>
          </select>
          <input
            type="text"
            placeholder={searchMode === "vector" ? "Semantic query..." : "Filter..."}
            bind:value={searchQuery}
            on:input={onSearchInput}
          />
          {#if searching}
            <span class="spin">&#10227;</span>
          {:else if searchQuery}
            <button class="clear-btn" on:click={clearSearch}>&#10005;</button>
          {/if}
        </div>
      {/if}
    </div>
  </header>

  {#if error}
    <div class="error-bar">
      <strong>Error:</strong> {error}
      <button on:click={() => (error = "")}>&#10005;</button>
    </div>
  {/if}

  {#if lastWriteResult}
    <div class="write-result" class:skipped={lastWriteResult.skipped} class:updated={lastWriteResult.updated}>
      {#if lastWriteResult.skipped}
        Skipped - {lastWriteResult.reason}
      {:else if lastWriteResult.updated}
        Updated {lastWriteResult.id || lastWriteResult.uri}
      {:else}
        Created {lastWriteResult.id || lastWriteResult.uri}
      {/if}
      <button on:click={() => (lastWriteResult = null)}>&#10005;</button>
    </div>
  {/if}

  <div class="content">
    {#key activeTab}
      <div in:fade={{ duration: 200, delay: 100 }} out:fade={{ duration: 100 }} style="display: contents;">
        {#if activeTab === "agent"}
          <Chat />
        {:else if activeTab === "overview"}
          <OverviewTab {skills} {memories} {contextNodes} {setActiveTab} />
        {:else if activeTab === "skills"}
          <SkillsTab
            {displaySkills}
            {hasSearchResults}
            {searchQuery}
            {API_BASE}
            {load}
            setLoading={(v) => (loading = v)}
            setError={(e) => (error = e)}
            setLastWriteResult={(r) => (lastWriteResult = r)}
            {loading}
          />
        {:else if activeTab === "memories"}
          <MemoriesTab
            {displayMemories}
            {hasSearchResults}
            {searchQuery}
            {API_BASE}
            {load}
            setLoading={(v) => (loading = v)}
            setError={(e) => (error = e)}
            setLastWriteResult={(r) => (lastWriteResult = r)}
            {loading}
          />
        {:else if activeTab === "context"}
          <ContextTab
            {displayContext}
            {hasSearchResults}
            {searchQuery}
            {API_BASE}
            {load}
            setLoading={(v) => (loading = v)}
            setError={(e) => (error = e)}
            setLastWriteResult={(r) => (lastWriteResult = r)}
            {loading}
          />
        {:else if activeTab === "search"}
          <SearchLabTab {API_BASE} />
        {:else if activeTab === "vibe"}
          <VibeTab {API_BASE} />
        {/if}
      </div>
    {/key}
  </div>
</main>
