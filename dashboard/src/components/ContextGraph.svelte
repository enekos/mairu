<script lang="ts">
  import { SvelteFlow, Background, Controls } from "@xyflow/svelte";
  import "@xyflow/svelte/dist/style.css";

  export let nodesData: any[] = [];

  // Parse `nodesData` into `nodes` and `edges` for SvelteFlow
  let nodes: any[] = [];
  let edges: any[] = [];

  $: {
    const layoutNodes = [];
    const layoutEdges = [];
    const layerMap: Record<string, number> = {};
    
    // Quick calculate depth
    nodesData.forEach(n => {
      let depth = 0;
      let current = n;
      while (current.parent_uri) {
        depth++;
        current = nodesData.find(x => x.uri === current.parent_uri) || {};
      }
      layerMap[n.uri] = depth;
    });

    const levelCounts: Record<number, number> = {};

    nodesData.forEach(node => {
      const depth = layerMap[node.uri] || 0;
      if (!levelCounts[depth]) levelCounts[depth] = 0;
      
      const x = levelCounts[depth] * 250;
      const y = depth * 150;
      levelCounts[depth]++;

      layoutNodes.push({
        id: node.uri,
        position: { x, y },
        data: { label: node.name },
        type: 'default',
        style: 'background: #1e293b; color: white; border: 1px solid #334155; border-radius: 8px; padding: 10px; font-size: 12px; white-space: pre-wrap; word-break: break-word;'
      });

      if (node.parent_uri) {
        layoutEdges.push({
          id: `e-${node.parent_uri}-${node.uri}`,
          source: node.parent_uri,
          target: node.uri,
          animated: true,
          style: 'stroke: #475569;'
        });
      }
    });

    nodes = layoutNodes;
    edges = layoutEdges;
  }
</script>

<div style="height: 600px; width: 100%; border: 1px solid #1e293b; border-radius: 8px; background: #0f172a;">
  {#if nodes.length > 0}
    <SvelteFlow {nodes} {edges} fitView>
      <Background bgColor="#0f172a" patternColor="#334155" />
      <Controls />
    </SvelteFlow>
  {:else}
    <div style="display: flex; align-items: center; justify-content: center; height: 100%; color: #64748b;">
      No context nodes to display
    </div>
  {/if}
</div>
