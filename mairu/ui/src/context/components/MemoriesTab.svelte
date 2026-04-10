<script lang="ts">
  import { slide } from 'svelte/transition';
  import { fmtDate, scoreColor, copy, impColor, categoryColors } from "../lib/utils";
  import { createMemory as apiCreateMemory, updateMemory, deleteMemory } from "../../lib/api";

  export let displayMemories: any[];
  export let hasSearchResults: boolean;
  export let searchQuery: string;
  export let load: () => Promise<void>;
  export let setLoading: (loading: boolean) => void;
  export let setError: (error: string) => void;
  export let setLastWriteResult: (res: any) => void;
  export let loading: boolean;

  let newMemory = { content: "", category: "observation", owner: "agent", importance: 5, useRouter: true };
  let addingMemory = false;

  let editingId: string | null = null;
  let editForm: any = {};

  async function createMemory() {
    addingMemory = true; setLastWriteResult(null);
    try {
      const result = await apiCreateMemory(newMemory);
      setLastWriteResult(result);
      newMemory = { content: "", category: "observation", owner: "agent", importance: 5, useRouter: true };
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { addingMemory = false; }
  }

  function startEdit(item: any) {
    editingId = item.id;
    editForm = { ...item };
  }

  function cancelEdit() {
    editingId = null;
    editForm = {};
  }

  async function saveEdit() {
    setLoading(true); setError(""); setLastWriteResult(null);
    try {
      const result = await updateMemory(editForm);
      setLastWriteResult(result);
      cancelEdit();
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { setLoading(false); }
  }

  async function del(id: string) {
    if (!confirm(`Delete this memory?`)) return;
    setLoading(true);
    try {
      await deleteMemory(id);
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { setLoading(false); }
  }
</script>

<section class="add-panel">
  <button class="toggle-add" on:click={() => addingMemory = !addingMemory}>
    {addingMemory ? "▲ Close" : "+ Add Memory"}
  </button>
  {#if addingMemory}
    <div transition:slide>
      <form on:submit|preventDefault={createMemory} class="add-form">
        <textarea rows="3" placeholder="Memory content — write as a self-contained fact" bind:value={newMemory.content} required></textarea>
        <div class="form-row">
          <label>
            Category
            <select bind:value={newMemory.category}>
              {#each ["observation","reflection","profile","preferences","entities","events","cases","patterns","decision","constraint","architecture"] as cat}
                <option value={cat}>{cat}</option>
              {/each}
            </select>
          </label>
          <label>
            Owner
            <select bind:value={newMemory.owner}>
              <option value="agent">agent</option>
              <option value="user">user</option>
              <option value="system">system</option>
            </select>
          </label>
          <label>
            Importance
            <input type="number" min="1" max="10" bind:value={newMemory.importance} style="width:60px" />
          </label>
        </div>
        <div class="form-footer">
          <label class="router-toggle">
            <input type="checkbox" bind:checked={newMemory.useRouter} />
            Smart dedup (LLM router)
          </label>
          <button type="submit" class="btn-primary" disabled={loading}>Save memory</button>
        </div>
      </form>
    </div>
  {/if}
</section>

<section class="table-section">
  {#if displayMemories.length === 0}
    <p class="empty">No memories{searchQuery ? " matching your query" : ""}.</p>
  {:else}
    <table>
      <thead>
        <tr>
          <th style="width:45%">Content</th>
          <th>Cat / Owner</th>
          <th>Imp</th>
          {#if hasSearchResults}<th>Score</th>{/if}
          <th>Updated</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        {#each displayMemories as row}
          <tr>
            {#if editingId === row.id}
              <td colspan={hasSearchResults ? 6 : 5}>
                <div class="edit-form-inline">
                  <textarea bind:value={editForm.content} placeholder="Content" rows="3" class="edit-textarea"></textarea>
                  <div class="edit-row">
                    <label>
                      Category:
                      <select bind:value={editForm.category}>
                        {#each ["observation","reflection","profile","preferences","entities","events","cases","patterns","decision","constraint","architecture"] as cat}
                          <option value={cat}>{cat}</option>
                        {/each}
                      </select>
                    </label>
                    <label>
                      Owner:
                      <select bind:value={editForm.owner}>
                        <option value="agent">agent</option>
                        <option value="user">user</option>
                        <option value="system">system</option>
                      </select>
                    </label>
                    <label>
                      Importance:
                      <input type="number" min="1" max="10" bind:value={editForm.importance} style="width:60px" />
                    </label>
                  </div>
                  <div class="edit-actions">
                    <button class="btn-primary" on:click={saveEdit} disabled={loading}>Save</button>
                    <button class="btn-cancel" on:click={cancelEdit} disabled={loading}>Cancel</button>
                  </div>
                </div>
              </td>
            {:else}
              <td>
                <div class="content-cell">
                  {row.content}
                  <button class="copy-btn" on:click={() => copy(row.id)} title="Copy ID">⎘ {row.id?.slice(0,8)}</button>
                </div>
              </td>
              <td>
                <span class="cat-badge" style="background:{categoryColors[row.category] || 'var(--text-muted)'}">{row.category}</span>
                <div class="owner-text">{row.owner}</div>
              </td>
              <td>
                <span class="imp-badge {impColor(row.importance)}">{row.importance}</span>
              </td>
              {#if hasSearchResults}
                <td>
                  <span class="score-badge" style="color:{scoreColor(row._hybrid_score ?? 0)}">
                    {((row._hybrid_score ?? 0) * 100).toFixed(0)}%
                  </span>
                  <div class="score-detail">
                    vec {((row._vector_score ?? 0)*100).toFixed(0)}
                    kw {((row._keyword_score ?? 0)*100).toFixed(0)}
                    imp {((row._importance_score ?? 0)*100).toFixed(0)}
                  </div>
                </td>
              {/if}
              <td class="date-cell">{fmtDate(row.updated_at || row.created_at)}</td>
              <td class="row-actions">
                <button class="btn-edit" on:click={() => startEdit(row)} title="Edit">✎</button>
                <button class="btn-del" on:click={() => del(row.id)} title="Delete">✕</button>
              </td>
            {/if}
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</section>