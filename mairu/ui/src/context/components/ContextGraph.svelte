<script lang="ts">
  import { scoreColor } from "../lib/utils";

  export let nodesData: any[] = [];
  export let hasSearchResults = false;

  let selectedNode: any | null = null;

  function depthOf(node: any, map: Map<string, any>): number {
    let depth = 0;
    let current = node;
    while (current?.parent_uri && map.has(current.parent_uri)) {
      depth += 1;
      current = map.get(current.parent_uri);
      if (depth > 100) break;
    }
    return depth;
  }

  $: nodeMap = new Map(nodesData.map((n) => [n.uri, n]));
  $: orderedNodes = [...nodesData].sort((a, b) => {
    const da = depthOf(a, nodeMap);
    const db = depthOf(b, nodeMap);
    if (da !== db) return da - db;
    return String(a.uri).localeCompare(String(b.uri));
  });
</script>

<div class="graph-container">
  <div class="flow-wrapper">
    {#if orderedNodes.length === 0}
      <div class="empty-state">No context nodes to display</div>
    {:else}
      <div class="tree">
        {#each orderedNodes as node}
          <button class="tree-node" style="padding-left:{depthOf(node, nodeMap) * 24 + 12}px" on:click={() => (selectedNode = node)}>
            <span class="tree-name">{node.name || node.uri}</span>
            {#if hasSearchResults && node._hybrid_score !== undefined}
              <span class="tree-score" style="color:{scoreColor(node._hybrid_score)}">
                {(node._hybrid_score * 100).toFixed(1)}%
              </span>
            {/if}
          </button>
        {/each}
      </div>
    {/if}
  </div>

  {#if selectedNode}
    <div class="details-drawer">
      <div class="drawer-header">
        <h3>Node Details</h3>
        <button class="close-btn" on:click={() => (selectedNode = null)}>✕</button>
      </div>
      <div class="drawer-content">
        <div class="field"><div class="label">URI</div><div class="value uri-value">{selectedNode.uri}</div></div>
        <div class="field"><div class="label">Name</div><div class="value">{selectedNode.name}</div></div>
        <div class="field"><div class="label">Abstract</div><div class="value abstract-value">{selectedNode.abstract || "None"}</div></div>
        {#if selectedNode.overview}
          <div class="field"><div class="label">Overview</div><pre class="value pre-value">{selectedNode.overview}</pre></div>
        {/if}
        {#if selectedNode.content}
          <div class="field"><div class="label">Content</div><pre class="value pre-value">{selectedNode.content}</pre></div>
        {/if}
      </div>
    </div>
  {/if}
</div>

<style>
  .graph-container {
    position: relative;
    height: 600px;
    width: 100%;
    border: 1px solid var(--bg-card);
    border-radius: 8px;
    background: var(--bg-main);
    display: flex;
    overflow: hidden;
  }
  
  .flow-wrapper {
    flex: 1;
    height: 100%;
    overflow: auto;
  }

  .empty-state {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    color: var(--text-muted);
  }
  
  .details-drawer {
    width: 400px;
    background: var(--bg-card);
    border-left: 1px solid var(--border-main);
    display: flex;
    flex-direction: column;
    box-shadow: -5px 0 15px rgba(0,0,0,0.3);
    z-index: 10;
  }
  .tree {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 12px;
  }
  .tree-node {
    width: 100%;
    text-align: left;
    background: var(--bg-card);
    border: 1px solid var(--border-main);
    color: var(--text-dim);
    border-radius: 8px;
    padding: 8px 10px;
    cursor: pointer;
    display: flex;
    justify-content: space-between;
    gap: 8px;
  }
  .tree-node:hover {
    border-color: var(--accent-main);
  }
  .tree-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  
  .drawer-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px;
    border-bottom: 1px solid var(--border-main);
    background: var(--bg-main);
  }
  
  .drawer-header h3 {
    margin: 0;
    color: var(--text-main);
    font-size: 16px;
  }
  
  .close-btn {
    background: transparent;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    font-size: 16px;
  }
  
  .close-btn:hover {
    color: #f87171;
  }
  
  .drawer-content {
    flex: 1;
    overflow-y: auto;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
  
  .field {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  
  .field .label {
    font-size: 11px;
    text-transform: uppercase;
    color: var(--text-muted);
    font-weight: bold;
    letter-spacing: 0.5px;
  }
  
  .value {
    color: var(--text-dim);
    font-size: 13px;
    line-height: 1.5;
  }
  
  .uri-value {
    word-break: break-all;
    font-family: monospace;
    background: var(--bg-main);
    padding: 6px 8px;
    border-radius: 4px;
    border: 1px solid var(--border-main);
  }
  
  .abstract-value {
    background: var(--bg-main);
    padding: 10px;
    border-radius: 6px;
    border-left: 3px solid #38bdf8;
  }
  
  .pre-value {
    font-family: monospace;
    background: var(--bg-main);
    padding: 10px;
    border-radius: 6px;
    border: 1px solid var(--border-main);
    white-space: pre-wrap;
    word-break: break-word;
    font-size: 12px;
    max-height: 300px;
    overflow-y: auto;
  }
  
  .score-value {
    font-weight: bold;
    font-size: 14px;
  }
  
  .tags {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
  
  .tag {
    background: var(--bg-main);
    border: 1px solid var(--border-main);
    color: var(--text-secondary);
    padding: 2px 8px;
    border-radius: 12px;
    font-size: 11px;
  }
</style>
