<script lang="ts">
  import { fmtDate, categoryColors, impColor } from "../lib/utils";

  export let skills: Record<string, unknown>[];
  export let memories: Record<string, unknown>[];
  export let contextNodes: Record<string, unknown>[];
  export let setActiveTab: (tab: string) => void;

  // Category distribution
  $: catCounts = memories.reduce((acc, m) => {
    const cat = (m.category as string) || "other";
    acc[cat] = (acc[cat] || 0) + 1;
    return acc;
  }, {} as Record<string, number>);

  $: catEntries = Object.entries(catCounts).sort((a, b) => b[1] - a[1]);
  $: catMax = Math.max(...Object.values(catCounts), 1);

  // Importance distribution
  $: impCounts = memories.reduce((acc, m) => {
    const imp = (m.importance as number) || 1;
    acc[imp] = (acc[imp] || 0) + 1;
    return acc;
  }, {} as Record<number, number>);

  $: impMax = Math.max(...Object.values(impCounts), 1);

  // Owner distribution
  $: ownerCounts = memories.reduce((acc, m) => {
    const owner = (m.owner as string) || "unknown";
    acc[owner] = (acc[owner] || 0) + 1;
    return acc;
  }, {} as Record<string, number>);

  $: totalDocs = skills.length + memories.length + contextNodes.length;
</script>

<section class="overview-grid">
  <div class="stat-card" on:click={() => setActiveTab("skills")} on:keydown={(e) => e.key === 'Enter' && setActiveTab("skills")} role="button" tabindex="0">
    <div class="stat-icon">&#9889;</div>
    <div class="stat-body">
      <div class="stat-num">{skills.length}</div>
      <div class="stat-label">Skills</div>
    </div>
  </div>
  <div class="stat-card" on:click={() => setActiveTab("memories")} on:keydown={(e) => e.key === 'Enter' && setActiveTab("memories")} role="button" tabindex="0">
    <div class="stat-icon">&#9672;</div>
    <div class="stat-body">
      <div class="stat-num">{memories.length}</div>
      <div class="stat-label">Memories</div>
    </div>
  </div>
  <div class="stat-card" on:click={() => setActiveTab("context")} on:keydown={(e) => e.key === 'Enter' && setActiveTab("context")} role="button" tabindex="0">
    <div class="stat-icon">&#11041;</div>
    <div class="stat-body">
      <div class="stat-num">{contextNodes.length}</div>
      <div class="stat-label">Context Nodes</div>
    </div>
  </div>
</section>

{#if totalDocs > 0}
  <div class="overview-charts">
    <!-- Category distribution -->
    {#if catEntries.length > 0}
      <div class="chart-card">
        <h3 class="chart-title">Memory Categories</h3>
        <div class="bar-chart">
          {#each catEntries as [cat, count]}
            <div class="bar-row">
              <span class="bar-label">
                <span class="bar-dot" style="background:{categoryColors[cat] || '#64748b'}"></span>
                {cat}
              </span>
              <div class="bar-track">
                <div class="bar-fill" style="width:{(count / catMax * 100).toFixed(0)}%;background:{categoryColors[cat] || '#64748b'}"></div>
              </div>
              <span class="bar-count">{count}</span>
            </div>
          {/each}
        </div>
      </div>
    {/if}

    <!-- Importance distribution -->
    <div class="chart-card">
      <h3 class="chart-title">Importance Distribution</h3>
      <div class="imp-chart">
        {#each Array.from({length: 10}, (_, i) => i + 1) as imp}
          <div class="imp-col">
            <div class="imp-bar-track">
              <div
                class="imp-bar-fill {impColor(imp)}"
                style="height:{((impCounts[imp] || 0) / impMax * 100).toFixed(0)}%"
              ></div>
            </div>
            <span class="imp-label">{imp}</span>
            {#if impCounts[imp]}
              <span class="imp-count">{impCounts[imp]}</span>
            {/if}
          </div>
        {/each}
      </div>

      <div class="owner-pills">
        {#each Object.entries(ownerCounts) as [owner, count]}
          <span class="owner-pill">
            {owner} <strong>{count}</strong>
          </span>
        {/each}
      </div>
    </div>
  </div>
{/if}

{#if memories.length > 0}
  <section class="recent-section">
    <h3>Recent memories</h3>
    <ul class="recent-list">
      {#each memories.slice(0, 5) as m}
        <li>
          <span class="cat-dot" style="background:{categoryColors[m.category] || '#64748b'}"></span>
          <span class="recent-content">{m.content}</span>
          <span class="imp-pill {impColor(m.importance)}">{m.importance}</span>
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
          <span class="recent-content"><code>{c.uri}</code> — {c.abstract}</span>
          <span class="recent-date">{fmtDate(c.updated_at || c.created_at)}</span>
        </li>
      {/each}
    </ul>
  </section>
{/if}

<style>
  .overview-charts {
    display: grid; grid-template-columns: 1fr 1fr; gap: 16px;
    margin-bottom: 24px;
  }

  .chart-card {
    background: #1e293b; border: 1px solid #334155; border-radius: 12px;
    padding: 18px 20px;
  }
  .chart-title {
    font-size: 12px; color: #64748b; text-transform: uppercase;
    letter-spacing: 0.05em; margin-bottom: 14px;
  }

  /* Bar chart */
  .bar-chart { display: flex; flex-direction: column; gap: 8px; }
  .bar-row { display: flex; align-items: center; gap: 10px; }
  .bar-label {
    display: flex; align-items: center; gap: 6px;
    font-size: 12px; color: #94a3b8; width: 100px; flex-shrink: 0;
  }
  .bar-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
  .bar-track {
    flex: 1; height: 6px; background: #0f172a; border-radius: 3px;
    overflow: hidden;
  }
  .bar-fill {
    height: 100%; border-radius: 3px; transition: width 0.3s ease;
    min-width: 2px;
  }
  .bar-count { font-size: 12px; color: #64748b; width: 28px; text-align: right; }

  /* Importance histogram */
  .imp-chart {
    display: flex; gap: 6px; align-items: flex-end;
    height: 100px; padding-bottom: 4px;
  }
  .imp-col {
    flex: 1; display: flex; flex-direction: column; align-items: center; gap: 4px;
  }
  .imp-bar-track {
    width: 100%; height: 70px; background: #0f172a; border-radius: 3px;
    overflow: hidden; display: flex; flex-direction: column; justify-content: flex-end;
  }
  .imp-bar-fill {
    border-radius: 3px 3px 0 0; transition: height 0.3s ease;
    min-height: 0;
  }
  .imp-bar-fill.imp-high { background: #22c55e; }
  .imp-bar-fill.imp-med { background: #f59e0b; }
  .imp-bar-fill.imp-low { background: #475569; }
  .imp-label { font-size: 11px; color: #64748b; }
  .imp-count { font-size: 10px; color: #94a3b8; }

  .owner-pills {
    display: flex; gap: 8px; margin-top: 14px; padding-top: 12px;
    border-top: 1px solid #334155;
  }
  .owner-pill {
    font-size: 11px; color: #94a3b8; background: #0f172a;
    padding: 3px 10px; border-radius: 12px; border: 1px solid #334155;
  }
  .owner-pill strong { color: #e2e8f0; margin-left: 4px; }

  .imp-pill {
    display: inline-block; width: 22px; height: 22px; border-radius: 50%;
    text-align: center; line-height: 22px; font-size: 10px; font-weight: 700;
    flex-shrink: 0;
  }
</style>
