<script lang="ts">
  type Row = Record<string, unknown>;

  const API_BASE = import.meta.env.VITE_DASHBOARD_API_BASE || "http://localhost:8787";

  let loading = false;
  let error = "";
  let skills: Row[] = [];
  let memories: Row[] = [];
  let contextNodes: Row[] = [];

  async function loadDashboard() {
    loading = true;
    error = "";
    try {
      const res = await fetch(`${API_BASE}/api/dashboard?limit=200`);
      if (!res.ok) {
        throw new Error(`API error: ${res.status}`);
      }
      const payload = await res.json();
      skills = payload.skills ?? [];
      memories = payload.memories ?? [];
      contextNodes = payload.contextNodes ?? [];
    } catch (err: unknown) {
      error = err instanceof Error ? err.message : "Unknown error";
    } finally {
      loading = false;
    }
  }

  function pretty(value: unknown): string {
    if (value === null || value === undefined) return "";
    if (typeof value === "object") return JSON.stringify(value, null, 2);
    return String(value);
  }

  loadDashboard();
</script>

<main>
  <header>
    <h1>Turso Context Dashboard</h1>
    <button on:click={loadDashboard} disabled={loading}>
      {loading ? "Refreshing..." : "Refresh"}
    </button>
  </header>

  <section class="cards">
    <article>
      <h2>Skills</h2>
      <p>{skills.length}</p>
    </article>
    <article>
      <h2>Memories</h2>
      <p>{memories.length}</p>
    </article>
    <article>
      <h2>Context Nodes</h2>
      <p>{contextNodes.length}</p>
    </article>
  </section>

  {#if error}
    <p class="error">{error}</p>
  {/if}

  <section class="table-wrap">
    <h3>Skills</h3>
    <table>
      <thead>
        <tr><th>ID</th><th>Name</th><th>Description</th><th>Metadata</th><th>Created</th></tr>
      </thead>
      <tbody>
        {#each skills as row}
          <tr>
            <td>{pretty(row.id)}</td>
            <td>{pretty(row.name)}</td>
            <td>{pretty(row.description)}</td>
            <td><pre>{pretty(row.metadata)}</pre></td>
            <td>{pretty(row.created_at)}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </section>

  <section class="table-wrap">
    <h3>Memories</h3>
    <table>
      <thead>
        <tr><th>ID</th><th>Content</th><th>Category</th><th>Owner</th><th>Importance</th><th>Created</th></tr>
      </thead>
      <tbody>
        {#each memories as row}
          <tr>
            <td>{pretty(row.id)}</td>
            <td>{pretty(row.content)}</td>
            <td>{pretty(row.category)}</td>
            <td>{pretty(row.owner)}</td>
            <td>{pretty(row.importance)}</td>
            <td>{pretty(row.created_at)}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </section>

  <section class="table-wrap">
    <h3>Context Nodes</h3>
    <table>
      <thead>
        <tr><th>URI</th><th>Parent</th><th>Name</th><th>Abstract</th><th>Created</th></tr>
      </thead>
      <tbody>
        {#each contextNodes as row}
          <tr>
            <td>{pretty(row.uri)}</td>
            <td>{pretty(row.parent_uri)}</td>
            <td>{pretty(row.name)}</td>
            <td>{pretty(row.abstract)}</td>
            <td>{pretty(row.created_at)}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </section>
</main>
