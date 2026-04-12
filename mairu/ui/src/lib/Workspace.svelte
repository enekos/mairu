<script lang="ts">
  import { Search, RefreshCw, Trash2, ChevronRight, ChevronDown, Database, BrainCircuit, Wrench } from 'lucide-svelte';
  import { onMount } from 'svelte';
  import { dashboard, deleteContextNode, deleteMemory, deleteSkill } from './api';

  type Tab = 'context' | 'memories' | 'skills';
  let activeTab: Tab = 'context';
  
  let contextNodes: any[] = [];
  let memories: any[] = [];
  let skills: any[] = [];
  let loading = false;
  let searchQuery = "";
  
  let expandedItems: Record<string, boolean> = {};

  async function loadData() {
    loading = true;
    try {
      const d = await dashboard(1000);
      contextNodes = d.contextNodes ?? [];
      memories = d.memories ?? [];
      skills = d.skills ?? [];
    } catch {
      // ignore load error
    } finally {
      loading = false;
    }
  }

  async function deleteItem(type: Tab, id: string) {
    if (!confirm(`Are you sure you want to delete this ${type} item?`)) return;
    
    try {
      if (type === 'context') await deleteContextNode(id);
      else if (type === 'memories') await deleteMemory(id);
      else if (type === 'skills') await deleteSkill(id);
      
      if (type === 'context') contextNodes = contextNodes.filter(n => n.uri !== id);
      else if (type === 'memories') memories = memories.filter(m => m.id !== id);
      else if (type === 'skills') skills = skills.filter(s => s.id !== id);
    } catch {
      // ignore delete error
    }
  }

  function toggleExpand(id: string) {
    expandedItems[id] = !expandedItems[id];
  }

  onMount(() => {
    loadData();
  });

  $: filteredContextNodes = contextNodes.filter(n => 
    !searchQuery || 
    (n.uri && n.uri.toLowerCase().includes(searchQuery.toLowerCase())) ||
    (n.name && n.name.toLowerCase().includes(searchQuery.toLowerCase()))
  );
  
  $: filteredMemories = memories.filter(m => 
    !searchQuery || 
    (m.content && m.content.toLowerCase().includes(searchQuery.toLowerCase())) ||
    (m.category && m.category.toLowerCase().includes(searchQuery.toLowerCase()))
  );
  
  $: filteredSkills = skills.filter(s => 
    !searchQuery || 
    (s.name && s.name.toLowerCase().includes(searchQuery.toLowerCase())) ||
    (s.description && s.description.toLowerCase().includes(searchQuery.toLowerCase()))
  );
</script>

<div class="flex-1 flex flex-col bg-green-950 border-r border-green-900">
  <div class="h-14 border-b border-green-900 flex items-center px-4 justify-between shrink-0 bg-green-950/50 backdrop-blur-sm z-10 sticky top-0">
    <div class="flex gap-4 h-full items-center">
      <button 
        class="flex items-center gap-2 hover:text-green-300 transition-colors h-14 {activeTab === 'context' ? 'text-green-400 border-b-2 border-green-400' : 'text-green-600 border-b-2 border-transparent'}"
        on:click={() => activeTab = 'context'}
      >
        <Database size={16} /> Context Graph <span class="text-xs bg-green-900 px-1.5 py-0.5 ml-1">{contextNodes.length}</span>
      </button>
      <button 
        class="flex items-center gap-2 hover:text-green-300 transition-colors h-14 {activeTab === 'memories' ? 'text-green-400 border-b-2 border-green-400' : 'text-green-600 border-b-2 border-transparent'}"
        on:click={() => activeTab = 'memories'}
      >
        <BrainCircuit size={16} /> Memories <span class="text-xs bg-green-900 px-1.5 py-0.5 ml-1">{memories.length}</span>
      </button>
      <button 
        class="flex items-center gap-2 hover:text-green-300 transition-colors h-14 {activeTab === 'skills' ? 'text-green-400 border-b-2 border-green-400' : 'text-green-600 border-b-2 border-transparent'}"
        on:click={() => activeTab = 'skills'}
      >
        <Wrench size={16} /> Skills <span class="text-xs bg-green-900 px-1.5 py-0.5 ml-1">{skills.length}</span>
      </button>
    </div>
    
    <div class="flex items-center gap-2">
      <div class="relative">
        <Search size={14} class="absolute left-2.5 top-2 text-green-600" />
        <input 
          type="text" 
          placeholder="Filter..." 
          bind:value={searchQuery}
          class="bg-black border border-green-900  pl-8 pr-3 py-1 text-xs text-green-300 focus:outline-none focus:border-green-500 w-48 transition-colors"
        />
      </div>
      <button class="p-1.5 text-green-600 hover:text-green-400 hover:bg-green-900  transition-colors" on:click={loadData}>
        <RefreshCw size={14} class={loading ? "animate-spin" : ""} />
      </button>
    </div>
  </div>
  
  <div class="flex-1 overflow-y-auto p-4 flex flex-col gap-2 text-sm font-mono">
    {#if activeTab === 'context'}
      {#each filteredContextNodes as node}
        <div class="bg-black border border-green-900 flex flex-col">
          <div class="flex items-center justify-between p-2 hover:bg-green-900/30 cursor-pointer" role="button" tabindex="0" on:click={() => toggleExpand(node.uri)} on:keydown={(e) => e.key === 'Enter' && toggleExpand(node.uri)}>
            <div class="flex items-center gap-2 overflow-hidden">
              {#if expandedItems[node.uri]}
                <ChevronDown size={16} class="text-green-600 shrink-0" />
              {:else}
                <ChevronRight size={16} class="text-green-600 shrink-0" />
              {/if}
              <span class="text-green-400 truncate font-semibold">{node.name || node.uri}</span>
              <span class="text-green-600 text-xs truncate">{node.uri}</span>
            </div>
            <button class="text-green-700 hover:text-red-500 transition-colors p-1" on:click|stopPropagation={() => deleteItem('context', node.uri)}>
              <Trash2 size={14} />
            </button>
          </div>
          {#if expandedItems[node.uri]}
            <div class="p-4 border-t border-green-900 bg-green-950/20 text-green-300 space-y-4">
              {#if node.abstract}
                <div>
                  <div class="text-xs text-green-600 uppercase mb-1">Abstract</div>
                  <div class="whitespace-pre-wrap">{node.abstract}</div>
                </div>
              {/if}
              {#if node.overview}
                <div>
                  <div class="text-xs text-green-600 uppercase mb-1">Overview</div>
                  <div class="whitespace-pre-wrap bg-black/50 p-2 font-mono text-xs overflow-x-auto">{node.overview}</div>
                </div>
              {/if}
              {#if node.content}
                <div>
                  <div class="text-xs text-green-600 uppercase mb-1">Content</div>
                  <div class="whitespace-pre-wrap">{node.content}</div>
                </div>
              {/if}
            </div>
          {/if}
        </div>
      {/each}
      {#if filteredContextNodes.length === 0}
        <div class="text-center py-8 text-green-700 font-sans">No context nodes found.</div>
      {/if}
    {/if}
    
    {#if activeTab === 'memories'}
      {#each filteredMemories as memory}
        <div class="bg-black border border-green-900 flex flex-col">
          <div class="flex items-center justify-between p-2 hover:bg-green-900/30 cursor-pointer" role="button" tabindex="0" on:click={() => toggleExpand(memory.id)} on:keydown={(e) => e.key === 'Enter' && toggleExpand(memory.id)}>
            <div class="flex items-center gap-2 overflow-hidden">
              {#if expandedItems[memory.id]}
                <ChevronDown size={16} class="text-green-600 shrink-0" />
              {:else}
                <ChevronRight size={16} class="text-green-600 shrink-0" />
              {/if}
              <span class="text-green-400 truncate">{memory.content.substring(0, 80)}{memory.content.length > 80 ? '...' : ''}</span>
              {#if memory.category}
                <span class="text-xs bg-amber-900/50 text-amber-400 px-1.5 py-0.5 rounded">{memory.category}</span>
              {/if}
            </div>
            <button class="text-green-700 hover:text-red-500 transition-colors p-1" on:click|stopPropagation={() => deleteItem('memories', memory.id)}>
              <Trash2 size={14} />
            </button>
          </div>
          {#if expandedItems[memory.id]}
            <div class="p-4 border-t border-green-900 bg-green-950/20 text-green-300 space-y-2">
              <div class="whitespace-pre-wrap">{memory.content}</div>
              <div class="flex gap-4 text-xs text-green-600 mt-2">
                <span>Importance: {memory.importance}</span>
                <span>Project: {memory.project}</span>
                <span>ID: {memory.id}</span>
              </div>
            </div>
          {/if}
        </div>
      {/each}
      {#if filteredMemories.length === 0}
        <div class="text-center py-8 text-green-700 font-sans">No memories found.</div>
      {/if}
    {/if}
    
    {#if activeTab === 'skills'}
      {#each filteredSkills as skill}
        <div class="bg-black border border-green-900 flex flex-col">
          <div class="flex items-center justify-between p-2 hover:bg-green-900/30 cursor-pointer" role="button" tabindex="0" on:click={() => toggleExpand(skill.id)} on:keydown={(e) => e.key === 'Enter' && toggleExpand(skill.id)}>
            <div class="flex items-center gap-2 overflow-hidden">
              {#if expandedItems[skill.id]}
                <ChevronDown size={16} class="text-green-600 shrink-0" />
              {:else}
                <ChevronRight size={16} class="text-green-600 shrink-0" />
              {/if}
              <span class="text-green-400 font-semibold">{skill.name}</span>
            </div>
            <button class="text-green-700 hover:text-red-500 transition-colors p-1" on:click|stopPropagation={() => deleteItem('skills', skill.id)}>
              <Trash2 size={14} />
            </button>
          </div>
          {#if expandedItems[skill.id]}
            <div class="p-4 border-t border-green-900 bg-green-950/20 text-green-300 space-y-4">
              <div>
                <div class="text-xs text-green-600 uppercase mb-1">Description</div>
                <div class="whitespace-pre-wrap">{skill.description}</div>
              </div>
              {#if skill.instructions}
                <div>
                  <div class="text-xs text-green-600 uppercase mb-1">Instructions</div>
                  <div class="whitespace-pre-wrap bg-black/50 p-2 font-mono text-xs">{skill.instructions}</div>
                </div>
              {/if}
            </div>
          {/if}
        </div>
      {/each}
      {#if filteredSkills.length === 0}
        <div class="text-center py-8 text-green-700 font-sans">No skills found.</div>
      {/if}
    {/if}
  </div>
</div>
