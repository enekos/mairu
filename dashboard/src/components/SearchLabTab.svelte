<script lang="ts">
  import { fmtDate, impColor, categoryColors, scoreColor } from "../lib/utils";

  export let API_BASE: string;

  let labQuery = "";
  let labType: "all" | "skills" | "memories" | "context" = "all";
  let labTopK = 10;
  let labSearching = false;
  let labResults: Record<string, unknown>[] = [];
  let labError = "";
  let labSearched = false;

  // Advanced ES controls
  let showAdvanced = false;
  let fuzziness: "auto" | "0" | "1" | "2" = "auto";
  let phraseBoost = 2.0;
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
    labSearching = true; labError = ""; labSearched = false;
    const start = performance.now();
    try {
      const params = new URLSearchParams({
        q: labQuery,
        type: labType,
        topK: String(labTopK),
      });
      if (fuzziness !== "auto") params.set("fuzziness", fuzziness);
      else params.set("fuzziness", "auto");
      if (phraseBoost !== 2.0) params.set("phraseBoost", String(phraseBoost));
      if (minScore > 0) params.set("minScore", String(minScore));
      if (highlight) params.set("highlight", "true");

      if (weightVector !== 2.0) params.set("weightVector", String(weightVector));
      if (weightKeyword !== 1.0) params.set("weightKeyword", String(weightKeyword));
      if (weightRecency !== 0.5) params.set("weightRecency", String(weightRecency));
      if (weightImportance !== 1.0) params.set("weightImportance", String(weightImportance));
      
      if (recencyScale !== "30d") params.set("recencyScale", recencyScale);
      if (recencyDecay !== 0.5) params.set("recencyDecay", String(recencyDecay));

      const res = await fetch(`${API_BASE}/api/search?${params}`);
      if (!res.ok) throw new Error(`Search API ${res.status}`);
      const data = await res.json();
      const flat: Record<string, unknown>[] = [];
      (data.skills ?? []).forEach((r: unknown) => flat.push({ ...r, _type: "skill" }));
      (data.memories ?? []).forEach((r: unknown) => flat.push({ ...r, _type: "memory" }));
      (data.contextNodes ?? []).forEach((r: unknown) => flat.push({ ...r, _type: "context" }));
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

  function maxScore(): number {
    if (labResults.length === 0) return 1;
    return Math.max(...labResults.map(r => r._hybrid_score ?? 0), 0.01);
  }
</script>

<section class="lab-section">
  <div class="lab-header">
    <h2 class="lab-title">Search Lab</h2>
    <p class="lab-desc">Test semantic search exactly as an agent would — same query path, same scoring.</p>
  </div>

  <div class="lab-controls">
    <div class="lab-main-row">
      <input
        class="lab-input"
        type="text"
        placeholder="Enter a search query..."
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
        {labSearching ? "Searching..." : "Search"}
      </button>
    </div>

    <!-- Advanced toggle -->
    <button class="lab-advanced-toggle" on:click={() => showAdvanced = !showAdvanced}>
      {showAdvanced ? "Hide" : "Show"} advanced controls
      <span class="lab-chevron" class:open={showAdvanced}>&#9662;</span>
    </button>

    {#if showAdvanced}
      <div class="lab-advanced">
        <label class="lab-adv-field">
          <span class="lab-adv-label">Fuzziness</span>
          <select class="lab-adv-select" bind:value={fuzziness}>
            <option value="auto">auto</option>
            <option value="0">0 (exact)</option>
            <option value="1">1</option>
            <option value="2">2</option>
          </select>
          <span class="lab-adv-hint">Typo tolerance (Levenshtein distance)</span>
        </label>
        <label class="lab-adv-field">
          <span class="lab-adv-label">Phrase Boost</span>
          <input class="lab-adv-input" type="number" min="0" max="20" step="0.5" bind:value={phraseBoost} />
          <span class="lab-adv-hint">Bonus for exact phrase ordering (0 = disabled)</span>
        </label>
        <label class="lab-adv-field">
          <span class="lab-adv-label">Min Score</span>
          <input class="lab-adv-input" type="number" min="0" max="100" step="0.5" bind:value={minScore} />
          <span class="lab-adv-hint">Hard threshold — drop results below this score</span>
        </label>
        <label class="lab-adv-field lab-adv-checkbox">
          <input type="checkbox" bind:checked={highlight} />
          <span class="lab-adv-label">Highlights</span>
          <span class="lab-adv-hint">Return matched terms wrapped in &lt;mark&gt; tags</span>
        </label>
        <label class="lab-adv-field">
          <span class="lab-adv-label">Vector Weight</span>
          <input class="lab-adv-input" type="number" min="0" max="10" step="0.1" bind:value={weightVector} />
          <span class="lab-adv-hint">Importance of kNN semantic match</span>
        </label>
        <label class="lab-adv-field">
          <span class="lab-adv-label">Keyword Weight</span>
          <input class="lab-adv-input" type="number" min="0" max="10" step="0.1" bind:value={weightKeyword} />
          <span class="lab-adv-hint">Importance of BM25 full-text match</span>
        </label>
        <label class="lab-adv-field">
          <span class="lab-adv-label">Recency Weight</span>
          <input class="lab-adv-input" type="number" min="0" max="10" step="0.1" bind:value={weightRecency} />
          <span class="lab-adv-hint">Importance of recency decay boost</span>
        </label>
        <label class="lab-adv-field">
          <span class="lab-adv-label">Importance Weight</span>
          <input class="lab-adv-input" type="number" min="0" max="10" step="0.1" bind:value={weightImportance} />
          <span class="lab-adv-hint">Importance of user/agent importance rating</span>
        </label>
      </div>
    {/if}
  </div>

  {#if labError}
    <p class="lab-error">{labError}</p>
  {/if}

  {#if labSearched}
    <div class="lab-meta">
      <span>
        <strong>{labResults.length}</strong> result{labResults.length !== 1 ? "s" : ""}
        {#if labType !== "all"}· type: <strong>{labType}</strong>{/if}
        · topK: <strong>{labTopK}</strong>
        · query: <em>"{labQuery}"</em>
      </span>
      <span class="lab-latency">{latencyMs}ms</span>
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
                  {#if highlight && row._highlight?.description}
                    {@html row._highlight.description[0]}
                  {:else}
                    {row.description}
                  {/if}
                {:else if row._type === "memory"}
                  {#if highlight && row._highlight?.content}
                    {@html row._highlight.content[0]}
                  {:else}
                    {row.content}
                  {/if}
                  {#if row.owner}<span class="lab-owner">— {row.owner}</span>{/if}
                {:else}
                  <strong>{row.name}</strong>
                  {#if highlight && row._highlight?.abstract}
                    <span class="lab-abstract"> — {@html row._highlight.abstract[0]}</span>
                  {:else if row.abstract}
                    <span class="lab-abstract"> — {row.abstract}</span>
                  {/if}
                {/if}
              </div>

              <!-- Score breakdown -->
              <div class="lab-scores">
                <div class="lab-score-main">
                  <span class="lab-score-label">hybrid</span>
                  <div class="lab-bar-wrap">
                    <div class="lab-bar" style="width:{((row._hybrid_score ?? 0) / maxScore() * 100).toFixed(1)}%;background:{scoreColor(row._hybrid_score ?? 0)}"></div>
                  </div>
                  <span class="lab-score-val" style="color:{scoreColor(row._hybrid_score ?? 0)}">
                    {((row._hybrid_score ?? 0) * 100).toFixed(1)}%
                  </span>
                </div>
                <div class="lab-score-subs">
                  {#each [
                    { key: "_vector_score", label: "vector", color: "#818cf8" },
                    { key: "_keyword_score", label: "keyword", color: "#38bdf8" },
                    { key: "_recency_score", label: "recency", color: "#34d399" },
                    { key: "_importance_score", label: "importance", color: "#fbbf24" },
                  ] as s}
                    {#if row[s.key] !== undefined && row[s.key] !== null}
                      <div class="lab-score-sub">
                        <span class="lab-score-sub-label">{s.label}</span>
                        <div class="lab-bar-wrap lab-bar-wrap-sm">
                          <div class="lab-bar lab-bar-sm" style="width:{(row[s.key] * 100).toFixed(1)}%;background:{s.color}"></div>
        <label class="lab-adv-field">
          <span class="lab-adv-label">Recency Decay</span>
          <input class="lab-adv-input" type="number" min="0" max="1" step="0.05" bind:value={recencyDecay} />
          <span class="lab-adv-hint">Decay factor at scale distance</span>
        </label>
        <label class="lab-adv-field">
          <span class="lab-adv-label">Recency Scale</span>
          <input class="lab-adv-input" type="text" bind:value={recencyScale} />
          <span class="lab-adv-hint">e.g., 7d, 30d</span>
        </label>
      </div>
                        <span class="lab-score-sub-val" style="color:{s.color}">{(row[s.key] * 100).toFixed(1)}%</span>
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
      <div class="lab-empty-icon">&#8981;</div>
      <p>Enter a query above and press <kbd>Enter</kbd> or click <strong>Search</strong>.</p>
      <p class="lab-empty-hint">Results are ranked using the same hybrid scorer agents use — vector similarity + keyword overlap + recency + importance.</p>
      <div class="lab-features">
        <span class="lab-feature">kNN vectors</span>
        <span class="lab-feature">BM25 full-text</span>
        <span class="lab-feature">Fuzzy matching</span>
        <span class="lab-feature">Phrase boost</span>
        <span class="lab-feature">Recency decay</span>
        <span class="lab-feature">Importance boost</span>
      </div>
    </div>
  {/if}
</section>

<style>
  .lab-main-row {
    display: flex; align-items: center; gap: 8px;
  }

  .lab-advanced-toggle {
    background: none; border: none; color: #64748b; font-size: 12px;
    cursor: pointer; display: flex; align-items: center; gap: 4px;
    padding: 0; width: fit-content;
  }
  .lab-advanced-toggle:hover { color: #94a3b8; }
  .lab-chevron { font-size: 10px; transition: transform 0.15s; }
  .lab-chevron.open { transform: rotate(180deg); }

  .lab-advanced {
    display: grid; grid-template-columns: repeat(2, 1fr); gap: 12px;
    padding: 14px; background: #0f172a; border: 1px solid #1e293b;
    border-radius: 8px;
  }

  .lab-adv-field {
    display: flex; flex-direction: column; gap: 4px;
  }
  .lab-adv-label { font-size: 12px; color: #94a3b8; font-weight: 500; }
  .lab-adv-hint { font-size: 10px; color: #475569; }
  .lab-adv-select, .lab-adv-input {
    background: #1e293b; border: 1px solid #334155; color: #e2e8f0;
    border-radius: 6px; padding: 6px 8px; font-size: 13px; outline: none;
    width: 100%;
  }
  .lab-adv-select:focus, .lab-adv-input:focus { border-color: #6366f1; }

  .lab-adv-checkbox {
    flex-direction: row; align-items: center; gap: 8px;
    flex-wrap: wrap;
  }
  .lab-adv-checkbox input { accent-color: #6366f1; width: 16px; height: 16px; }
  .lab-adv-checkbox .lab-adv-hint { width: 100%; }

  .lab-latency {
    font-size: 11px; color: #475569; background: #1e293b;
    padding: 2px 8px; border-radius: 4px;
  }

  .lab-meta {
    display: flex; align-items: center; justify-content: space-between;
  }

  .lab-features {
    display: flex; gap: 8px; flex-wrap: wrap; justify-content: center;
    margin-top: 8px;
  }
  .lab-feature {
    font-size: 11px; color: #64748b; background: #1e293b;
    border: 1px solid #334155; padding: 3px 10px; border-radius: 12px;
  }

  /* Color-coded score sub-bars */
  .lab-score-sub-val { font-weight: 600; }
</style>
