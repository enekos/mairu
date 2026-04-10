<script lang="ts">
  import { slide } from 'svelte/transition';
  import { fmtDate, scoreColor, copy } from "../lib/utils";
  import { createContextNode as apiCreateContextNode, updateContextNode, deleteContextNode } from "../../lib/api";

  export let displayContext: any[];
  export let hasSearchResults: boolean;
  export let searchQuery: string;
  export let load: () => Promise<void>;
  export let setLoading: (loading: boolean) => void;
  export let setError: (error: string) => void;
  export let setLastWriteResult: (res: any) => void;
  export let loading: boolean;

  import ContextGraph from "./ContextGraph.svelte";

  let newContext = { uri: "", parent_uri: "", name: "", abstract: "", overview: "", useRouter: true };
  let addingContext = false;
  let viewMode: "list" | "graph" = "list";

  let editingId: string | null = null;
  let editForm: any = {};

  async function createContext() {
    addingContext = true; setLastWriteResult(null);
    try {
      const result = await apiCreateContextNode(newContext);
      setLastWriteResult(result);
      newContext = { uri: "", parent_uri: "", name: "", abstract: "", overview: "", useRouter: true };
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { addingContext = false; }
  }

  function startEdit(item: any) {
    editingId = item.uri;
    editForm = { ...item };
  }

  function cancelEdit() {
    editingId = null;
    editForm = {};
  }

  async function saveEdit() {
    setLoading(true); setError(""); setLastWriteResult(null);
    try {
      const result = await updateContextNode(editForm);
      setLastWriteResult(result);
      cancelEdit();
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { setLoading(false); }
  }

  async function del(uri: string) {
    if (!confirm(`Delete this context node?`)) return;
    setLoading(true);
    try {
      await deleteContextNode(uri);
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { setLoading(false); }
  }
</script>

<section class="add-panel" style="display: flex; justify-content: space-between; align-items: center;">
  <div>
    <button class="toggle-add" on:click={() => addingContext = !addingContext}>
      {addingContext ? "▲ Close" : "+ Add Context Node"}
    </button>
  </div>
  <div style="display: flex; gap: 10px;">
    <button class="btn-primary" on:click={() => viewMode = 'list'} disabled={viewMode === 'list'}>List</button>
    <button class="btn-primary" on:click={() => viewMode = 'graph'} disabled={viewMode === 'graph'}>Graph</button>
  </div>
</section>

{#if addingContext}
  <section class="add-panel" style="margin-top: 0; padding-top: 0; border-top: none;" transition:slide>
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
  </section>
{/if}

{#if viewMode === 'list'}
  <section class="table-section">
    {#if displayContext.length === 0}
      <p class="empty">No context nodes{searchQuery ? " matching your query" : ""}.</p>
    {:else}
    <table>
      <thead>
        <tr>
          <th style="width:30%">URI / Name</th>
          <th style="width:40%">Details</th>
          {#if hasSearchResults}<th>Score</th>{/if}
          <th>Updated</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        {#each displayContext as row}
          <tr>
            {#if editingId === row.uri}
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
              <td class="abstract-cell">
                <div style="font-weight: 500; margin-bottom: 6px; color: var(--text-main);">{row.abstract}</div>
                {#if row.overview}
                  <div style="margin-top: 8px; color: var(--text-dim); font-size: 12px;"><strong>Overview:</strong><br>{row.overview}</div>
                {/if}
                {#if row.content}
                  <div style="margin-top: 8px; color: var(--text-secondary); font-size: 12px;"><strong>Content:</strong><br>{row.content}</div>
                {/if}
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
                <button class="btn-edit" on:click={() => startEdit(row)} title="Edit">✎</button>
                <button class="btn-del" on:click={() => del(row.uri)} title="Delete">✕</button>
              </td>
            {/if}
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
  </section>
{:else}
  <ContextGraph nodesData={displayContext} {hasSearchResults} />
{/if}