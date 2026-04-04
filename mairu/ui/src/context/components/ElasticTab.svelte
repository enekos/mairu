<script lang="ts">
  export let API_BASE: string;

  let loading = false;
  let error = "";
  type IndexStats = { docs: number; sizeBytes: number; deletedDocs: number };
  type ClusterStats = {
    status: string;
    clusterName: string;
    numberOfNodes: number;
    activeShards: number;
    relocatingShards: number;
    unassignedShards: number;
    indices: Record<string, IndexStats>;
  };
  let stats: ClusterStats | null = null;

  const indexLabels: Record<string, string> = {
    contextfs_skills: "Skills",
    contextfs_memories: "Memories",
    contextfs_context_nodes: "Context Nodes",
  };

  function statusColor(status: string): string {
    if (status === "green") return "#22c55e";
    if (status === "yellow") return "#f59e0b";
    return "#ef4444";
  }

  function fmtBytes(b: number): string {
    if (b < 1024) return `${b} B`;
    if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} KB`;
    return `${(b / (1024 * 1024)).toFixed(1)} MB`;
  }

  async function loadStats() {
    loading = true; error = "";
    try {
      const res = await fetch(`${API_BASE}/api/cluster`);
      if (!res.ok) throw new Error(`API ${res.status}`);
      stats = await res.json();
    } catch (e) {
      error = e instanceof Error ? e.message : String(e);
    } finally {
      loading = false;
    }
  }

  loadStats();
</script>

<section class="elastic-section">
  <div class="elastic-header">
    <h2 class="elastic-title">Elasticsearch Cluster</h2>
    <button class="btn-refresh-sm" on:click={loadStats} disabled={loading}>
      {loading ? "..." : "Refresh"}
    </button>
  </div>

  {#if error}
    <div class="elastic-error">{error}</div>
  {/if}

  {#if stats}
    <!-- Cluster Health -->
    <div class="elastic-cluster">
      <div class="cluster-status" style="border-color:{statusColor(stats.status)}">
        <div class="cluster-dot" style="background:{statusColor(stats.status)}"></div>
        <div class="cluster-info">
          <div class="cluster-name">{stats.clusterName}</div>
          <div class="cluster-status-text" style="color:{statusColor(stats.status)}">
            {stats.status.toUpperCase()}
          </div>
        </div>
      </div>

      <div class="cluster-metrics">
        <div class="metric">
          <div class="metric-val">{stats.numberOfNodes}</div>
          <div class="metric-label">Nodes</div>
        </div>
        <div class="metric">
          <div class="metric-val">{stats.activeShards}</div>
          <div class="metric-label">Active Shards</div>
        </div>
        <div class="metric">
          <div class="metric-val">{stats.relocatingShards}</div>
          <div class="metric-label">Relocating</div>
        </div>
        <div class="metric">
          <div class="metric-val">{stats.unassignedShards}</div>
          <div class="metric-label">Unassigned</div>
        </div>
      </div>
    </div>

    <!-- Index Cards -->
    <h3 class="elastic-subtitle">Indices</h3>
    <div class="index-grid">
      {#each Object.entries(stats.indices) as [name, idx]}
        <div class="index-card">
          <div class="index-card-header">
            <span class="index-badge">{indexLabels[name] || name}</span>
            <code class="index-name">{name}</code>
          </div>
          <div class="index-stats">
            <div class="index-stat">
              <div class="index-stat-val">{idx.docs.toLocaleString()}</div>
              <div class="index-stat-label">Documents</div>
            </div>
            <div class="index-stat">
              <div class="index-stat-val">{fmtBytes(idx.sizeBytes)}</div>
              <div class="index-stat-label">Size</div>
            </div>
            <div class="index-stat">
              <div class="index-stat-val">{idx.deletedDocs}</div>
              <div class="index-stat-label">Deleted</div>
            </div>
          </div>
          <div class="index-bar-wrap">
            <div
              class="index-bar"
              style="width:{stats.indices ? Math.max(5, (idx.docs / Math.max(...Object.values(stats.indices).map(i => i.docs), 1)) * 100) : 5}%"
            ></div>
          </div>
        </div>
      {/each}
    </div>

    <!-- Total storage -->
    <div class="elastic-total">
      Total storage: <strong>{fmtBytes(Object.values(stats.indices).reduce((sum, i) => sum + i.sizeBytes, 0))}</strong>
      across <strong>{Object.values(stats.indices).reduce((sum, i) => sum + i.docs, 0).toLocaleString()}</strong> documents
    </div>
  {:else if !loading}
    <div class="elastic-empty">Unable to connect to Elasticsearch</div>
  {/if}
</section>

<style>
  .elastic-section { display: flex; flex-direction: column; gap: 24px; }

  .elastic-header {
    display: flex; align-items: center; justify-content: space-between;
  }
  .elastic-title { font-size: 18px; font-weight: 700; color: var(--text-bold); }
  .btn-refresh-sm {
    background: var(--bg-card); border: 1px solid var(--border-main); color: var(--text-secondary);
    padding: 6px 14px; border-radius: 7px; cursor: pointer; font-size: 12px;
  }
  .btn-refresh-sm:hover { background: var(--border-main); }

  .elastic-error {
    padding: 10px 14px; background: var(--bg-error); color: var(--text-error);
    border-radius: 8px; font-size: 13px;
  }

  .elastic-cluster {
    display: flex; gap: 24px; align-items: center; flex-wrap: wrap;
    background: var(--bg-card); border: 1px solid var(--border-main); border-radius: 12px;
    padding: 20px 24px;
  }

  .cluster-status {
    display: flex; align-items: center; gap: 14px;
    border-left: 3px solid; padding-left: 14px;
  }
  .cluster-dot {
    width: 14px; height: 14px; border-radius: 50%;
    animation: pulse 2s ease-in-out infinite;
  }
  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.5; }
  }
  .cluster-name { font-size: 16px; font-weight: 600; color: var(--text-bold); }
  .cluster-status-text { font-size: 12px; font-weight: 700; letter-spacing: 0.05em; }

  .cluster-metrics {
    display: flex; gap: 28px; margin-left: auto;
  }
  .metric { text-align: center; }
  .metric-val { font-size: 24px; font-weight: 700; color: var(--text-bold); }
  .metric-label { font-size: 11px; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.05em; margin-top: 2px; }

  .elastic-subtitle {
    font-size: 13px; color: var(--text-muted); text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .index-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 16px; }

  .index-card {
    background: var(--bg-card); border: 1px solid var(--border-main); border-radius: 12px;
    padding: 18px 20px; display: flex; flex-direction: column; gap: 14px;
    transition: border-color 0.15s;
  }
  .index-card:hover { border-color: var(--text-light); }

  .index-card-header { display: flex; flex-direction: column; gap: 4px; }
  .index-badge {
    font-size: 15px; font-weight: 600; color: var(--text-bold);
  }
  .index-name { font-size: 11px; color: var(--text-light); }

  .index-stats { display: flex; gap: 20px; }
  .index-stat { }
  .index-stat-val { font-size: 20px; font-weight: 700; color: var(--text-main); }
  .index-stat-label { font-size: 10px; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.04em; }

  .index-bar-wrap {
    height: 4px; background: var(--bg-main); border-radius: 2px; overflow: hidden;
  }
  .index-bar {
    height: 100%; background: var(--accent-main); border-radius: 2px;
    transition: width 0.3s ease;
  }

  .elastic-total {
    font-size: 13px; color: var(--text-muted); text-align: center;
    padding: 12px; background: var(--bg-card); border-radius: 8px;
    border: 1px solid var(--border-main);
  }
  .elastic-total strong { color: var(--text-active); }

  .elastic-empty {
    color: var(--text-light); font-size: 14px; text-align: center; padding: 40px;
  }
</style>
