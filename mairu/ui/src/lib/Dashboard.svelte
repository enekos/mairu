<script lang="ts">
  import { LayoutDashboard, Zap, Code, BrainCircuit, FileCode, CheckCircle2, MessageSquare } from 'lucide-svelte';
  import { messages, isGenerating } from './store';
  import { derived } from 'svelte/store';

  // Constants
  const CONTEXT_LIMIT = 2000000; // Assuming Gemini Pro context limit

  // Compute stats reactively from messages store
  const stats = derived([messages, isGenerating], ([$messages, $isGenerating]) => {
    let userMessages = 0;
    let assistantMessages = 0;
    let systemMessages = 0;
    let toolCallsCount = 0;
    let toolResultsCount = 0;
    
    let userChars = 0;
    let agentChars = 0;
    
    let toolUsageCounts: Record<string, number> = {};

    for (const msg of $messages) {
      if (msg.role === 'user') {
        userMessages++;
        userChars += msg.content.length;
      } else if (msg.role === 'assistant') {
        assistantMessages++;
        agentChars += msg.content.length;
        if (msg.bashOutput) agentChars += msg.bashOutput.length;
      } else if (msg.role === 'system') {
        systemMessages++;
      }
      
      for (const tc of msg.toolCalls) {
        toolCallsCount++;
        if (tc.status === 'completed' || tc.status === 'error') {
          toolResultsCount++;
        }
        
        toolUsageCounts[tc.name] = (toolUsageCounts[tc.name] || 0) + 1;
        
        // Add roughly the length of the tool result to agent tokens if it's there
        if (tc.result && typeof tc.result === 'string') {
          agentChars += tc.result.length;
        } else if (tc.result) {
          agentChars += JSON.stringify(tc.result).length;
        }
      }
    }

    // Rough token estimation: (chars + 3) / 4
    const estimatedUserTokens = Math.floor((userChars + 3) / 4);
    const estimatedAgentTokens = Math.floor((agentChars + 3) / 4);
    const totalTokens = estimatedUserTokens + estimatedAgentTokens;
    
    // Sort tool usages for the bar chart
    const topTools = Object.entries(toolUsageCounts)
      .sort((a, b) => b[1] - a[1])
      .slice(0, 5);
      
    // Find highest tool count to normalize bars
    const maxToolCount = topTools.length > 0 ? topTools[0][1] : 1;

    return {
      userMessages,
      assistantMessages,
      systemMessages,
      toolCallsCount,
      toolResultsCount,
      estimatedUserTokens,
      estimatedAgentTokens,
      totalTokens,
      contextPercentage: Math.min(100, (totalTokens / CONTEXT_LIMIT) * 100),
      topTools,
      maxToolCount,
      isStreaming: $isGenerating
    };
  });
</script>

<div class="flex-1 flex flex-col bg-green-950 border-r border-green-900">
  <div class="h-14 border-b border-green-900 flex items-center px-4 shrink-0 bg-green-950/50 backdrop-blur-sm z-10 sticky top-0 justify-between">
    <h1 class="text-sm font-semibold tracking-wide flex items-center gap-2">
      <LayoutDashboard size={16} />
      Session Dashboard
    </h1>
    <div class="flex items-center gap-2 text-xs">
      {#if $stats.isStreaming}
        <span class="flex items-center gap-1 text-amber-400 bg-amber-900/30 px-2 py-1 ">
          <span class="w-2 h-2 rounded-full bg-amber-400 animate-pulse"></span>
          Streaming
        </span>
      {:else}
        <span class="flex items-center gap-1 text-green-400 bg-green-900/30 px-2 py-1 ">
          <CheckCircle2 size={12} />
          Idle
        </span>
      {/if}
    </div>
  </div>
  
  <div class="flex-1 overflow-y-auto p-6 flex flex-col gap-6">
    <!-- Top Stats Row -->
    <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
      <div class="bg-black border border-green-900  p-4 flex flex-col justify-center">
        <div class="flex items-center gap-3 mb-2">
          <div class="w-8 h-8  bg-indigo-500/10 text-indigo-400 flex items-center justify-center shrink-0">
            <Zap size={16} />
          </div>
          <div class="text-xs text-green-600 uppercase tracking-wider">Tool Actions</div>
        </div>
        <div class="text-2xl font-semibold text-green-300">{$stats.toolCallsCount}</div>
        <div class="text-xs text-green-700 mt-1">{$stats.toolResultsCount} completed</div>
      </div>
      
      <div class="bg-black border border-green-900  p-4 flex flex-col justify-center">
        <div class="flex items-center gap-3 mb-2">
          <div class="w-8 h-8  bg-blue-500/10 text-blue-400 flex items-center justify-center shrink-0">
            <MessageSquare size={16} />
          </div>
          <div class="text-xs text-green-600 uppercase tracking-wider">Messages</div>
        </div>
        <div class="text-2xl font-semibold text-green-300">{$stats.userMessages + $stats.assistantMessages + $stats.systemMessages}</div>
        <div class="text-xs text-green-700 mt-1 flex gap-2">
          <span class="text-blue-500">U: {$stats.userMessages}</span>
          <span class="text-indigo-400">A: {$stats.assistantMessages}</span>
          <span class="text-slate-400">S: {$stats.systemMessages}</span>
        </div>
      </div>
      
      <div class="bg-black border border-green-900  p-4 flex flex-col justify-center md:col-span-2">
        <div class="flex items-center gap-3 mb-2">
          <div class="w-8 h-8  bg-amber-500/10 text-amber-400 flex items-center justify-center shrink-0">
            <BrainCircuit size={16} />
          </div>
          <div class="text-xs text-green-600 uppercase tracking-wider">Context Window</div>
        </div>
        <div class="flex items-end gap-2">
          <div class="text-2xl font-semibold text-green-300">{$stats.totalTokens.toLocaleString()}</div>
          <div class="text-sm text-green-700 mb-1">/ 2M tokens</div>
        </div>
        <div class="w-full bg-green-950 h-1.5 mt-3 overflow-hidden">
          <div class="h-full bg-amber-500 transition-all duration-500" style="width: {$stats.contextPercentage}%"></div>
        </div>
      </div>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <!-- Tool Usage -->
      <div class="bg-black border border-green-900  p-5 flex flex-col">
        <h3 class="font-semibold text-green-400 flex items-center gap-2 mb-4">
          <Code size={18} class="text-green-600" />
          Top Tools Used
        </h3>
        <div class="flex-1 flex flex-col gap-4">
          {#each $stats.topTools as [toolName, count], i}
            {@const colors = ['bg-indigo-500', 'bg-emerald-500', 'bg-amber-500', 'bg-blue-500', 'bg-rose-500']}
            {@const color = colors[i % colors.length]}
            <div class="flex items-center justify-between text-sm">
              <span class="text-green-500 font-mono w-32 truncate" title={toolName}>{toolName}</span>
              <div class="flex items-center gap-3 flex-1">
                <div class="h-2 bg-green-900 flex-1 overflow-hidden">
                  <div class="h-full {color} transition-all duration-500" style="width: {(count / $stats.maxToolCount) * 100}%"></div>
                </div>
                <span class="text-green-400 w-8 text-right font-mono">{count}</span>
              </div>
            </div>
          {/each}
          {#if $stats.topTools.length === 0}
            <div class="text-center text-green-700 py-6 font-sans text-sm">No tools have been used in this session yet.</div>
          {/if}
        </div>
      </div>
      
      <!-- Token Breakdown -->
      <div class="bg-black border border-green-900  p-5 flex flex-col">
        <h3 class="font-semibold text-green-400 flex items-center gap-2 mb-4">
          <FileCode size={18} class="text-green-600" />
          Token Distribution
        </h3>
        <div class="flex-1 flex flex-col justify-center">
          {#if $stats.totalTokens === 0}
            <div class="text-center text-green-700 py-6 font-sans text-sm">No data to display. Start chatting!</div>
          {:else}
            <div class="space-y-6">
              <div>
                <div class="flex justify-between text-sm mb-1">
                  <span class="text-green-500">Agent Generated & Outputs</span>
                  <span class="text-indigo-400 font-mono">{$stats.estimatedAgentTokens.toLocaleString()}</span>
                </div>
                <div class="w-full bg-green-950 h-2 overflow-hidden">
                  <div class="h-full bg-indigo-500" style="width: {($stats.estimatedAgentTokens / $stats.totalTokens) * 100}%"></div>
                </div>
              </div>
              
              <div>
                <div class="flex justify-between text-sm mb-1">
                  <span class="text-green-500">User Prompts</span>
                  <span class="text-blue-400 font-mono">{$stats.estimatedUserTokens.toLocaleString()}</span>
                </div>
                <div class="w-full bg-green-950 h-2 overflow-hidden">
                  <div class="h-full bg-blue-500" style="width: {($stats.estimatedUserTokens / $stats.totalTokens) * 100}%"></div>
                </div>
              </div>
            </div>
          {/if}
        </div>
      </div>
    </div>
  </div>
</div>
