<script lang="ts">
  import { Send, Bot, User, Loader2, Wrench, ChevronDown, ChevronRight, CheckCircle2, XCircle, Plus } from 'lucide-svelte';
  import { messages, sendMessage, isGenerating, connectionState, sessions, currentSession, switchSession, createSession, loadSessions } from './store';
  import { marked } from 'marked';
  import DOMPurify from 'dompurify';
  import { onMount, tick } from 'svelte';

  let inputStr = "";
  let messagesContainer: HTMLDivElement;
  let expandedTools: Record<string, boolean> = {};
  let creatingSession = false;

  onMount(() => {
    void loadSessions();
  });

  $: {
    $messages;
    scrollToBottom();
  }

  function toggleTool(id: string) {
    expandedTools[id] = !expandedTools[id];
  }

  async function scrollToBottom() {
    await tick();
    if (messagesContainer) {
      messagesContainer.scrollTop = messagesContainer.scrollHeight;
    }
  }

  function handleSend() {
    if (!inputStr.trim() || $isGenerating || $connectionState !== "connected") return;
    sendMessage(inputStr.trim());
    inputStr = "";
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  }

  function renderMarkdown(md: string) {
    if (!md) return "";
    return DOMPurify.sanitize(marked.parse(md) as string);
  }

  async function handleSessionSwitch(event: Event) {
    const target = event.target as HTMLSelectElement;
    const selected = target.value;
    if (!selected || selected === $currentSession) return;
    await switchSession(selected);
  }

  async function handleCreateSession() {
    if (creatingSession) return;
    const name = window.prompt("New session name");
    if (!name) return;

    creatingSession = true;
    try {
      await createSession(name);
    } catch (error) {
      const message = error instanceof Error ? error.message : "failed to create session";
      window.alert(message);
    } finally {
      creatingSession = false;
    }
  }
</script>

<div class="w-full max-w-2xl mx-auto h-full flex flex-col bg-slate-50 border-r border-slate-200">
  <div class="h-14 border-b border-slate-200 flex items-center px-4 shrink-0 bg-slate-50/50 backdrop-blur-sm z-10 sticky top-0">
    <h1 class="text-sm font-semibold tracking-wide flex items-center gap-2">
      Mairu Agent
      {#if $connectionState === "connected"}
        <span class="w-2 h-2 rounded-full bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.5)]"></span>
      {:else}
        <span class="w-2 h-2 rounded-full bg-amber-500 shadow-[0_0_8px_rgba(245,158,11,0.5)]"></span>
      {/if}
    </h1>
    <div class="ml-auto flex items-center gap-2">
      <label class="text-xs uppercase tracking-wider text-slate-400" for="session-select">Session</label>
      <select
        id="session-select"
        class="bg-white border border-slate-200 rounded-md px-2 py-1 text-xs text-slate-800 min-w-36"
        value={$currentSession}
        on:change={handleSessionSwitch}
        disabled={$isGenerating}
      >
        {#each $sessions as session}
          <option value={session}>{session}</option>
        {/each}
      </select>
      <button
        class="p-1.5 rounded-md border border-slate-200 hover:border-slate-500 hover:text-slate-100 text-slate-400 transition-colors disabled:opacity-50"
        on:click={handleCreateSession}
        disabled={$isGenerating || creatingSession}
        title="Create session"
      >
        <Plus size={14} />
      </button>
    </div>
  </div>

  <div class="flex-1 overflow-y-auto p-4 space-y-6" bind:this={messagesContainer}>
    {#if $messages.length === 0}
      <div class="h-full flex flex-col items-center justify-center text-slate-400">
        <Bot size={48} class="mb-4 opacity-20" />
        <p>I am Mairu, your codebase agent.</p>
        <p class="text-sm opacity-70">How can I help you today?</p>
      </div>
    {/if}

    {#each $messages as msg (msg.id)}
      <div class="flex flex-col gap-2">
        <div class="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-slate-400">
          {#if msg.role === 'user'}
            <User size={14} class="text-indigo-400" /> You
          {:else if msg.role === 'assistant'}
            <Bot size={14} class="text-emerald-400" /> Mairu
          {:else}
            <span class="text-rose-400">System</span>
          {/if}
        </div>
        
        {#if msg.role === 'assistant'}
          {#if msg.toolCalls && msg.toolCalls.length > 0}
            <div class="flex flex-col gap-2 my-2">
              {#each msg.toolCalls as tc}
                <div class="flex flex-col text-xs bg-slate-100/40 border border-slate-200/50 rounded-md overflow-hidden font-mono">
                  <button class="flex items-center gap-2 px-3 py-2 text-slate-700 hover:bg-slate-700/30 transition-colors" on:click={() => toggleTool(tc.id)}>
                    {#if expandedTools[tc.id]}
                      <ChevronDown size={14} />
                    {:else}
                      <ChevronRight size={14} />
                    {/if}
                    <Wrench size={12} class={tc.status === 'running' ? 'text-amber-400' : 'text-slate-400'} />
                    <span class="font-semibold">{tc.name}</span>
                    <span class="flex-1 text-left text-slate-400 truncate">
                      {JSON.stringify(tc.args)}
                    </span>
                    {#if tc.status === 'running'}
                      <Loader2 size={12} class="animate-spin text-amber-400" />
                    {:else if tc.status === 'completed' && !tc.result?.error}
                      <CheckCircle2 size={12} class="text-emerald-400" />
                    {:else}
                      <XCircle size={12} class="text-rose-400" />
                    {/if}
                  </button>
                  {#if expandedTools[tc.id]}
                    <div class="px-3 py-2 bg-slate-50/50 border-t border-slate-200/50 flex flex-col gap-2 overflow-x-auto">
                      <div class="text-slate-400">Args:</div>
                      <pre class="text-slate-700">{JSON.stringify(tc.args, null, 2)}</pre>
                      {#if tc.result}
                        <div class="text-slate-400 mt-1">Result:</div>
                        <pre class="text-slate-700 max-h-48 overflow-y-auto">{JSON.stringify(tc.result, null, 2)}</pre>
                      {/if}
                    </div>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}
          {#if msg.statuses && msg.statuses.length > 0}
            <div class="flex flex-col gap-1 my-2">
              {#each msg.statuses as status}
                <div class="text-xs text-slate-400 bg-slate-100/50 rounded-md px-3 py-1.5 flex items-start gap-2 border border-slate-200/50 font-mono">
                  <Wrench size={12} class="mt-0.5 shrink-0" />
                  <span>{status}</span>
                </div>
              {/each}
            </div>
          {/if}
          <div class="prose prose-invert prose-sm max-w-none prose-pre:bg-white prose-pre:border prose-pre:border-slate-200 prose-a:text-indigo-400" 
               class:animate-pulse={$isGenerating && msg.id === $messages[$messages.length-1].id && !msg.content}
               style="word-wrap: break-word;">
            {#if msg.content}
              {@html renderMarkdown(msg.content)}
            {:else if $isGenerating && msg.id === $messages[$messages.length-1].id}
              <span class="text-slate-400">Thinking...</span>
            {/if}
          </div>
        {:else if msg.role === 'user'}
          <div class="bg-indigo-500/10 border border-indigo-500/20 rounded-xl rounded-tl-sm p-4 text-slate-800">
            {msg.content}
          </div>
        {:else}
          <div class="bg-rose-500/10 border border-rose-500/20 rounded-xl p-4 text-rose-200 font-mono text-sm">
            {msg.content}
          </div>
        {/if}
      </div>
    {/each}
  </div>

  <div class="p-4 bg-slate-50 border-t border-slate-200 shrink-0">
    <div class="relative bg-white rounded-xl border border-slate-200 focus-within:border-indigo-500/50 focus-within:ring-1 focus-within:ring-indigo-500/50 transition-all shadow-inner">
      <textarea
        bind:value={inputStr}
        on:keydown={handleKeydown}
        disabled={$isGenerating || $connectionState !== "connected"}
        placeholder={$connectionState === "connected" ? "Ask anything about your codebase..." : "Connecting..."}
        class="w-full bg-transparent resize-none outline-none p-4 min-h-[56px] max-h-64 overflow-y-auto text-slate-800 disabled:opacity-50"
        rows="1"
      ></textarea>
      
      <div class="absolute bottom-3 right-3 flex items-center">
        <button 
          on:click={handleSend}
          disabled={!inputStr.trim() || $isGenerating || $connectionState !== "connected"}
          class="p-2 rounded-lg bg-indigo-500 text-white disabled:opacity-50 disabled:bg-slate-100 disabled:text-slate-400 transition-colors"
        >
          {#if $isGenerating}
            <Loader2 size={18} class="animate-spin" />
          {:else}
            <Send size={18} />
          {/if}
        </button>
      </div>
    </div>
    <div class="text-[10px] text-center text-slate-400 mt-2">
      Press Enter to send, Shift+Enter for new line
    </div>
  </div>
</div>
