<script lang="ts">
  import { fmtDate, categoryColors } from "../lib/utils";

  export let skills: any[];
  export let memories: any[];
  export let contextNodes: any[];
  export let setActiveTab: (tab: string) => void;
</script>

<section class="overview-grid">
  <div class="stat-card" on:click={() => setActiveTab("skills")} on:keydown={(e) => e.key === 'Enter' && setActiveTab("skills")} role="button" tabindex="0">
    <div class="stat-icon">⚡</div>
    <div class="stat-body">
      <div class="stat-num">{skills.length}</div>
      <div class="stat-label">Skills</div>
    </div>
  </div>
  <div class="stat-card" on:click={() => setActiveTab("memories")} on:keydown={(e) => e.key === 'Enter' && setActiveTab("memories")} role="button" tabindex="0">
    <div class="stat-icon">◈</div>
    <div class="stat-body">
      <div class="stat-num">{memories.length}</div>
      <div class="stat-label">Memories</div>
    </div>
  </div>
  <div class="stat-card" on:click={() => setActiveTab("context")} on:keydown={(e) => e.key === 'Enter' && setActiveTab("context")} role="button" tabindex="0">
    <div class="stat-icon">⬡</div>
    <div class="stat-body">
      <div class="stat-num">{contextNodes.length}</div>
      <div class="stat-label">Context Nodes</div>
    </div>
  </div>
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