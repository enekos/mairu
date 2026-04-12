<script lang="ts">
  import { Terminal, Database, Cpu, Activity, ShieldAlert } from 'lucide-svelte';
  import { connectionState, messages } from './store';
  import { onMount, onDestroy } from 'svelte';

  type TabType = "overview" | "logs" | "graph";
  let activeTab: TabType = "overview";

  interface LogEntry {
    time: string;
    level: string;
    message: string;
    attrs: Record<string, any>;
  }

  let sysLogs: LogEntry[] = [];
  let sysLogsContainer: HTMLDivElement;

  onMount(() => {
    if (window.runtime) {
      window.runtime.EventsOn('sys:log', (log: LogEntry) => {
        sysLogs = [...sysLogs, log];
        // Keep only last 1000 logs to prevent memory issues
        if (sysLogs.length > 1000) {
          sysLogs = sysLogs.slice(sysLogs.length - 1000);
        }
        setTimeout(() => {
          if (sysLogsContainer) {
            sysLogsContainer.scrollTop = sysLogsContainer.scrollHeight;
          }
        }, 10);
      });
    }
  });

  onDestroy(() => {
    if (window.runtime) {
      window.runtime.EventsOff('sys:log');
    }
  });
</script>

<div class="flex-1 flex flex-col bg-[#09090b] text-gray-300">
  <div class="h-12 border-b border-indigo-900/40 flex items-center px-4 shrink-0 bg-[#09090b]/95 backdrop-blur-sm">
    <div class="flex gap-4 h-full">
      <button 
        class="flex items-center gap-2 hover:text-cyan-400 transition-colors h-12 text-[11px] uppercase tracking-wider font-semibold {activeTab === 'overview' ? 'text-cyan-500 border-b-2 border-cyan-500' : 'text-indigo-400/70 border-b-2 border-transparent'}"
        on:click={() => activeTab = 'overview'}
      >
        <Activity size={14} /> Overview
      </button>
      <button 
        class="flex items-center gap-2 hover:text-cyan-400 transition-colors h-12 text-[11px] uppercase tracking-wider font-semibold {activeTab === 'logs' ? 'text-cyan-500 border-b-2 border-cyan-500' : 'text-indigo-400/70 border-b-2 border-transparent'}"
        on:click={() => activeTab = 'logs'}
      >
        <Terminal size={14} /> System Logs
      </button>
      <button 
        class="flex items-center gap-2 hover:text-cyan-400 transition-colors h-12 text-[11px] uppercase tracking-wider font-semibold {activeTab === 'graph' ? 'text-cyan-500 border-b-2 border-cyan-500' : 'text-indigo-400/70 border-b-2 border-transparent'}"
        on:click={() => activeTab = 'graph'}
      >
        <Database size={14} /> Graph
      </button>
    </div>
  </div>

  <div class="flex-1 overflow-y-auto p-6" bind:this={sysLogsContainer}>
    {#if activeTab === 'overview'}
      <div class="max-w-4xl mx-auto space-y-6">
        <div>
          <h2 class="text-xl font-semibold mb-1 text-pink-200">Workspace Overview</h2>
          <p class="text-xs text-indigo-400/80">Mairu is connected and ready to navigate your codebase.</p>
        </div>

        <div class="grid grid-cols-2 gap-4">
          <div class="p-5 bg-[#11111b]/40 border border-indigo-900/40 rounded-sm">
            <div class="w-8 h-8 bg-emerald-500/10 text-emerald-400 flex items-center justify-center mb-3 rounded-sm">
              <Cpu size={16} />
            </div>
            <h3 class="text-sm font-semibold mb-1 text-pink-200">Mairu Agent</h3>
            <div class="text-xs text-indigo-400/80 flex items-center gap-2">
              Status: 
              {#if $connectionState === "connected"}
                <span class="text-emerald-400 font-medium">Online</span>
              {:else}
                <span class="text-amber-400 font-medium">Connecting...</span>
              {/if}
            </div>
          </div>

          <div class="p-5 bg-[#11111b]/40 border border-indigo-900/40 rounded-sm">
            <div class="w-8 h-8 bg-blue-500/10 text-blue-400 flex items-center justify-center mb-3 rounded-sm">
              <Database size={16} />
            </div>
            <h3 class="text-sm font-semibold mb-1 text-pink-200">Code Graph</h3>
            <div class="text-xs text-indigo-400/80">
              Backed by Meilisearch
            </div>
          </div>
        </div>

        <div class="p-5 bg-indigo-900/10 border border-indigo-900/30 rounded-sm">
          <h3 class="font-semibold text-cyan-400 mb-4 flex items-center gap-2 text-sm">
            <ShieldAlert size={16} /> Capabilities
          </h3>
          <ul class="space-y-4 text-xs text-indigo-300/80">
            <li class="flex items-start gap-3">
              <div class="w-1.5 h-1.5 bg-cyan-500 mt-1 rounded-full"></div>
              <div>
                <strong class="text-cyan-300 font-medium">Surgical Reading</strong> <br/>
                Reads specific AST nodes instead of dumping entire files into context.
              </div>
            </li>
            <li class="flex items-start gap-3">
              <div class="w-1.5 h-1.5 bg-cyan-500 mt-1 rounded-full"></div>
              <div>
                <strong class="text-cyan-300 font-medium">Multi-Agent Dispatch</strong> <br/>
                Spawns sub-agents to parallelize codebase research.
              </div>
            </li>
            <li class="flex items-start gap-3">
              <div class="w-1.5 h-1.5 bg-cyan-500 mt-1 rounded-full"></div>
              <div>
                <strong class="text-cyan-300 font-medium">Terminal Native</strong> <br/>
                Executes bash commands, tests, and git operations autonomously.
              </div>
            </li>
          </ul>
        </div>
      </div>
    {:else if activeTab === 'logs'}
      <div class="w-full h-full flex flex-col font-mono text-[10px]">
        <div class="flex-1 bg-[#0c0c10] border border-indigo-900/40 p-3 overflow-y-auto space-y-1 custom-scrollbar rounded-sm">
          {#each sysLogs as log}
            <div class="flex gap-3 py-1 border-b border-indigo-900/20 last:border-0 hover:bg-indigo-900/10 transition-colors">
              <span class="text-indigo-400/60 shrink-0">{log.time.split('T')[1]?.split('Z')[0] || log.time}</span>
              <span class="shrink-0 w-12 font-bold {log.level === 'ERROR' ? 'text-rose-400' : log.level === 'WARN' ? 'text-amber-400' : log.level === 'DEBUG' ? 'text-indigo-400/80' : 'text-emerald-400'}">{log.level}</span>
              <div class="flex flex-col gap-0.5">
                <span class="text-pink-100/90">{log.message}</span>
                {#if Object.keys(log.attrs).length > 0}
                  <span class="text-indigo-400/70 text-[9px] break-all">{JSON.stringify(log.attrs)}</span>
                {/if}
              </div>
            </div>
          {/each}
          {#if sysLogs.length === 0}
            <div class="text-indigo-500/50 text-center py-10 font-sans text-xs">No system logs available yet.</div>
          {/if}
        </div>
      </div>
    {:else if activeTab === 'graph'}
      <div class="max-w-4xl mx-auto h-full flex flex-col">
        <h2 class="text-xl font-semibold mb-4 text-pink-200">Code Graph Explorer</h2>
        <div class="flex-1 bg-[#11111b]/40 border border-indigo-900/40 flex items-center justify-center text-indigo-400/60 p-8 rounded-sm">
          <div class="text-center">
            <Database size={40} class="mx-auto mb-4 opacity-30" />
            <h3 class="text-sm font-semibold text-cyan-400 mb-2">Graph Visualization Not Connected</h3>
            <p class="max-w-md mx-auto text-xs">Connect to a live Meilisearch instance to see context nodes and vector representations of your codebase.</p>
          </div>
        </div>
      </div>
    {/if}
  </div>
</div>
