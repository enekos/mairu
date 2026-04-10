<script lang="ts">
  import { fmtDate, impColor, categoryColors, scoreColor } from "../lib/utils";
  import { search } from "../../lib/api";

  let labQuery = "";
  let labType: "all" | "skills" | "memories" | "context" = "all";
  let labTopK = 10;
  let labSearching = false;
  let labResults: Record<string, any>[] = [];
  let labError = "";
  let labSearched = false;

  let minScore = 0;
  let highlight = false;
  let weightVector = 2.0;
  let weightKeyword = 1.0;
  let weightRecency = 0.5;
  let weightImportance = 1.0;
  let recencyScale = "30d";
  let recencyDecay = 0.5;
  let latencyMs = 0;

  async function runLabSearch() {
    if (!labQuery.trim()) return;
    labSearching = true;
    labError = "";
    labSearched = false;
    const start = performance.now();
    try {
      const opts: any = {
        q: labQuery,
        type: labType,
        topK: labTopK,
        minScore: minScore,
        weightVector: weightVector,
        weightKeyword: weightKeyword,
        weightRecency: weightRecency,
        weightImportance: weightImportance,
        recencyScale,
        recencyDecay: recencyDecay,
      };
      if (highlight) opts.highlight = true;

      const data = await search(opts);

      const flat: Record<string, any>[] = [];
      (data.skills ?? []).forEach((r: any) => flat.push({ ...r, _type: "skill" }));
      (data.memories ?? []).forEach((r: any) => flat.push({ ...r, _type: "memory" }));
      (data.contextNodes ?? []).forEach((r: any) => flat.push({ ...r, _type: "context" }));
      flat.sort((a, b) => (b._hybrid_score ?? 0) - (a._hybrid_score ?? 0));
      labResults = flat;
      labSearched = true;
      latencyMs = Math.round(performance.now() - start);
    } catch (e) {
      labError = e instanceof Error ? e.message : String(e);
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
    <p class="lab-desc">Tune and inspect search behavior with the same API used by agents.</p>
  </div>
  <div class="lab-controls">
    <input class="lab-input" bind:value={labQuery} on:keydown={labKeydown} placeholder="Search query..." />
    <select class="lab-select" bind:value={labType}>
      <option value="all">All</option>
      <option value="skills">Skills</option>
      <option value="memories">Memories</option>
      <option value="context">Context</option>
    </select>
    <label class="lab-topk-label">Top <input class="lab-topk" type="number" min="1" max="50" bind:value={labTopK} /></label>
    <button class="btn-primary lab-run" on:click={runLabSearch} disabled={labSearching || !labQuery.trim()}>
      {labSearching ? "Searching..." : "Search"}
    </button>
  </div>

  <div class="lab-controls">
    <label>minScore <input class="lab-topk" type="number" step="0.1" bind:value={minScore} /></label>
    <label>wVector <input class="lab-topk" type="number" step="0.1" bind:value={weightVector} /></label>
    <label>wKeyword <input class="lab-topk" type="number" step="0.1" bind:value={weightKeyword} /></label>
    <label>wRecency <input class="lab-topk" type="number" step="0.1" bind:value={weightRecency} /></label>
    <label>wImp <input class="lab-topk" type="number" step="0.1" bind:value={weightImportance} /></label>
    <label>scale <input class="lab-topk" bind:value={recencyScale} /></label>
    <label>decay <input class="lab-topk" type="number" step="0.05" min="0" max="1" bind:value={recencyDecay} /></label>
    <label><input type="checkbox" bind:checked={highlight} /> highlight</label>
  </div>

  {#if labError}<p class="lab-error">{labError}</p>{/if}
  {#if labSearched}
    <div class="lab-meta">
      <span><strong>{labResults.length}</strong> results for <em>"{labQuery}"</em></span>
      <span class="lab-latency">{latencyMs}ms</span>
    </div>
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
                <span class="lab-type-cat" style="background:{categoryColors[row.category] || 'var(--text-muted)'}">{row.category}</span>
                <span class="lab-imp-badge {impColor(row.importance)}">{row.importance}</span>
              {:else}
                <code class="lab-uri">{row.uri}</code>
              {/if}
              <span class="lab-date">{fmtDate(row.updated_at || row.created_at)}</span>
            </div>
            <div class="lab-card-content">
              {row.content || row.description || row.abstract || row.name}
            </div>
            <div class="lab-score-main">
              <span class="lab-score-label">hybrid</span>
              <span class="lab-score-val" style="color:{scoreColor(row._hybrid_score ?? 0)}">
                {((row._hybrid_score ?? 0) * 100).toFixed(1)}%
              </span>
            </div>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</section>
