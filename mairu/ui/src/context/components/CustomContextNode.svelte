<script lang="ts">
  import { Handle, Position } from '@xyflow/svelte';
  import { scoreColor } from '../lib/utils';

  export let data: any;
  export let selected: boolean = false;

  $: hasOverview = !!data.node.overview;
  $: hasContent = !!data.node.content;
  $: hasScore = data.node._hybrid_score !== undefined;
  $: score = data.node._hybrid_score ?? 0;
  $: dim = data.isSearchActive && !hasScore;
  
  $: borderColor = hasScore ? scoreColor(score) : (selected ? '#3b82f6' : 'var(--border-main)');
  $: borderWidth = hasScore || selected ? '2px' : '1px';
  $: bgColor = dim ? 'var(--bg-main)88' : 'var(--bg-card)';
  $: opacity = dim ? 0.4 : 1;
  $: boxShadow = hasScore ? `0 0 15px ${scoreColor(score)}44` : (selected ? '0 0 0 2px #3b82f644' : 'none');

</script>

<div class="custom-node" style="border-color: {borderColor}; border-width: {borderWidth}; background-color: {bgColor}; opacity: {opacity}; box-shadow: {boxShadow};">
  <Handle type="target" position={Position.Top} />
  
  <div class="node-header">
    <div class="node-title" title={data.node.uri}>{data.node.name}</div>
    {#if hasScore}
      <div class="node-score" style="color: {scoreColor(score)}">{Math.round(score * 100)}%</div>
    {/if}
  </div>
  
  <div class="node-abstract">{data.node.abstract || 'No abstract'}</div>
  
  <div class="node-badges">
    {#if hasOverview}
      <span class="badge overview-badge" title="Has Overview">O</span>
    {/if}
    {#if hasContent}
      <span class="badge content-badge" title="Has Content">C</span>
    {/if}
    <span class="badge uri-badge" title={data.node.uri}>{data.node.uri.split('/').pop()}</span>
  </div>

  <Handle type="source" position={Position.Bottom} />
</div>

<style>
  .custom-node {
    position: relative;
    padding: 12px;
    border-radius: 8px;
    border-style: solid;
    width: 280px;
    color: var(--text-main);
    font-family: monospace;
    transition: all 0.2s ease;
  }
  
  .node-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 8px;
    gap: 8px;
  }

  .node-title {
    font-weight: bold;
    font-size: 14px;
    word-break: break-all;
  }

  .node-score {
    font-size: 12px;
    font-weight: bold;
    background: var(--bg-main);
    padding: 2px 6px;
    border-radius: 4px;
  }

  .node-abstract {
    font-size: 11px;
    color: var(--text-secondary);
    margin-bottom: 12px;
    display: -webkit-box;
    -webkit-line-clamp: 3;
    -webkit-box-orient: vertical;
    overflow: hidden;
    line-height: 1.4;
  }

  .node-badges {
    display: flex;
    gap: 6px;
    align-items: center;
  }

  .badge {
    font-size: 10px;
    padding: 2px 6px;
    border-radius: 4px;
    background: var(--border-main);
    color: var(--text-dim);
    font-weight: bold;
  }

  .overview-badge {
    background: #0ea5e922;
    color: #38bdf8;
    border: 1px solid #0ea5e944;
  }

  .content-badge {
    background: #8b5cf622;
    color: #a78bfa;
    border: 1px solid #8b5cf644;
  }
  
  .uri-badge {
    background: var(--bg-card);
    border: 1px solid var(--border-main);
    max-width: 150px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
