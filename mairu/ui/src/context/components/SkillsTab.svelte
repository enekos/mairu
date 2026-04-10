<script lang="ts">
  import { slide } from 'svelte/transition';
  import { fmtDate, scoreColor, copy } from "../lib/utils";
  import { createSkill as apiCreateSkill, updateSkill, deleteSkill } from "../../lib/api";

  export let displaySkills: any[];
  export let hasSearchResults: boolean;
  export let searchQuery: string;
  export let load: () => Promise<void>;
  export let setLoading: (loading: boolean) => void;
  export let setError: (error: string) => void;
  export let setLastWriteResult: (res: any) => void;
  export let loading: boolean;

  let newSkill = { name: "", description: "" };
  let addingSkill = false;

  let editingId: string | null = null;
  let editForm: any = {};

  async function createSkill() {
    addingSkill = true; setLastWriteResult(null);
    try {
      await apiCreateSkill(newSkill);
      newSkill = { name: "", description: "" };
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { addingSkill = false; }
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
      const result = await updateSkill(editForm);
      setLastWriteResult(result);
      cancelEdit();
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { setLoading(false); }
  }

  async function del(id: string) {
    if (!confirm(`Delete this skill?`)) return;
    setLoading(true);
    try {
      await deleteSkill(id);
      await load();
    } catch (e) { setError(e instanceof Error ? e.message : String(e)); }
    finally { setLoading(false); }
  }
</script>

<section class="add-panel">
  <button class="toggle-add" on:click={() => addingSkill = !addingSkill}>
    {addingSkill ? "▲ Close" : "+ Add Skill"}
  </button>
  {#if addingSkill}
    <div transition:slide>
      <form on:submit|preventDefault={createSkill} class="add-form">
        <input type="text" placeholder="Skill name" bind:value={newSkill.name} required />
        <textarea rows="3" placeholder="Description — what this skill does and when to use it" bind:value={newSkill.description} required></textarea>
        <div class="form-footer">
          <button type="submit" class="btn-primary" disabled={loading}>Save skill</button>
        </div>
      </form>
    </div>
  {/if}
</section>

<section class="table-section">
  {#if displaySkills.length === 0}
    <p class="empty">No skills{searchQuery ? " matching your query" : ""}.</p>
  {:else}
    <table>
      <thead>
        <tr>
          <th style="width:40%">Name / Description</th>
          {#if hasSearchResults}<th>Score</th>{/if}
          <th>Updated</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        {#each displaySkills as row}
          <tr>
            {#if editingId === row.id}
              <td colspan={hasSearchResults ? 4 : 3}>
                <div class="edit-form-inline">
                  <input type="text" bind:value={editForm.name} placeholder="Name" class="edit-input" />
                  <textarea bind:value={editForm.description} placeholder="Description" rows="2" class="edit-textarea"></textarea>
                  <div class="edit-actions">
                    <button class="btn-primary" on:click={saveEdit} disabled={loading}>Save</button>
                    <button class="btn-cancel" on:click={cancelEdit} disabled={loading}>Cancel</button>
                  </div>
                </div>
              </td>
            {:else}
              <td>
                <div class="name-cell">
                  <strong>{row.name}</strong>
                  <span class="desc">{row.description}</span>
                  <button class="copy-btn" on:click={() => copy(row.id)} title="Copy ID">⎘ {row.id?.slice(0,8)}</button>
                </div>
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
                <button class="btn-del" on:click={() => del(row.id)} title="Delete">✕</button>
              </td>
            {/if}
          </tr>
        {/each}
      </tbody>
    </table>
  {/if}
</section>