<script lang="ts">
  import { fmtDate, impColor, categoryColors, scoreColor } from "../lib/utils";

  export let API_BASE: string;

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
</script>

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