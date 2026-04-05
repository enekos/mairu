<script lang="ts">
  import { Send, Bot, User, Loader2, Wrench, ChevronDown, ChevronRight, CheckCircle2, XCircle, Plus } from 'lucide-svelte';
  import { messages, sendMessage, isGenerating, connectionState, sessions, currentSession, switchSession, createSession, loadSessions } from './store';
  import { marked } from 'marked';
  import DOMPurify from 'dompurify';
  import { onMount, tick } from 'svelte';
  import { fade, slide, fly } from 'svelte/transition';

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

<div class="w-full max-w-4xl mx-auto flex-1 flex flex-col relative bg-[#0a0a0a] border border-green-900/50 rounded-2xl overflow-hidden shadow-[0_0_40px_rgba(0,255,0,0.1)] transition-all max-h-[calc(100vh-140px)]">
    <!-- Faded green background image -->
    <div class="absolute inset-0 pointer-events-none z-0 opacity-[0.08] mix-blend-screen" style="background-image: url('/mairu.png'); background-size: cover; background-position: center; background-repeat: no-repeat; filter: invert(1) sepia(1) hue-rotate(90deg) saturate(500%);"></div>

  <div class="relative z-10 h-16 border-b border-green-900/60 flex items-center px-6 sm:px-12 shrink-0 bg-[#0a0a0a]/95 backdrop-blur-md sticky top-0 shadow-sm">
    <h1 class="text-[15px] font-bold tracking-wide flex items-center gap-3 text-green-400">
      Mairu Agent
      {#if $connectionState === "connected"}
        <span class="w-2.5 h-2.5 rounded-full bg-green-500 shadow-[0_0_10px_rgba(34,197,94,0.6)] animate-pulse"></span>
      {:else}
        <span class="w-2.5 h-2.5 rounded-full bg-amber-500 shadow-[0_0_10px_rgba(245,158,11,0.6)]"></span>
      {/if}
    </h1>
    <div class="ml-auto flex items-center gap-3">
      <label class="text-[10px] font-bold uppercase tracking-widest text-green-600/80" for="session-select">Session</label>
      <select
        id="session-select"
        class="bg-[#050505] border border-green-900/60 rounded-lg px-2.5 py-1.5 text-xs font-medium text-green-300 min-w-36 focus:border-green-500 focus:ring-1 focus:ring-green-500 outline-none transition-colors"
        value={$currentSession}
        on:change={handleSessionSwitch}
        disabled={$isGenerating}
      >
        {#each $sessions as session}
          <option value={session}>{session}</option>
        {/each}
      </select>
      <button
        class="p-2 rounded-lg border border-green-900/60 hover:border-green-500 hover:bg-green-900/20 hover:text-green-300 text-green-600 transition-all active:scale-95 disabled:opacity-50"
        on:click={handleCreateSession}
        disabled={$isGenerating || creatingSession}
        title="Create session"
      >
        <Plus size={14} />
      </button>
    </div>
  </div>

  <div class="flex-1 overflow-y-auto relative z-10 px-6 py-6 sm:px-12 sm:py-8 space-y-8" bind:this={messagesContainer}>
    {#if $messages.length === 0}
      <div in:fade={{ duration: 400 }} class="h-full flex flex-col items-center justify-center text-green-600 px-4">
        <Bot size={56} class="mb-6 opacity-20" />
        <p class="text-lg">I am Mairu, your codebase agent.</p>
        <p class="text-sm opacity-70 mt-2">How can I help you today?</p>
      </div>
    {/if}

    {#each $messages as msg (msg.id)}
      <div class="flex flex-col gap-3" in:fly={{ y: 20, duration: 400 }}>
        <div class="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-green-600">
          {#if msg.role === 'user'}
            <User size={14} class="text-green-400" /> You
          {:else if msg.role === 'assistant'}
            <Bot size={14} class="text-green-400" /> Mairu
          {:else}
            <span class="text-rose-400">System</span>
          {/if}
        </div>
        
        {#if msg.role === 'assistant'}
          {#if msg.toolCalls && msg.toolCalls.length > 0}
            <div class="flex flex-col gap-2 my-2">
              {#each msg.toolCalls as tc}
                <div class="flex flex-col text-xs bg-green-950/30 border border-green-900/50 rounded-lg overflow-hidden font-mono shadow-sm transition-all hover:border-green-800/50">
                  <button class="flex items-center gap-3 px-4 py-3 text-green-400 hover:bg-green-900/20 transition-colors" on:click={() => toggleTool(tc.id)}>
                    {#if expandedTools[tc.id]}
                      <ChevronDown size={14} class="shrink-0" />
                    {:else}
                      <ChevronRight size={14} class="shrink-0" />
                    {/if}
                    <Wrench size={14} class={`shrink-0 ${tc.status === 'running' ? 'text-amber-400' : 'text-green-600'}`} />
                    <span class="font-semibold text-sm">{tc.name}</span>
                    <span class="flex-1 text-left text-green-600/70 truncate ml-2">
                      {JSON.stringify(tc.args)}
                    </span>
                    {#if tc.status === 'running'}
                      <Loader2 size={14} class="animate-spin text-amber-400 shrink-0" />
                    {:else if tc.status === 'completed' && !tc.result?.error}
                      <CheckCircle2 size={14} class="text-green-400 shrink-0" />
                    {:else}
                      <XCircle size={14} class="text-rose-400 shrink-0" />
                    {/if}
                  </button>
                  {#if expandedTools[tc.id]}
                    <div transition:slide={{ duration: 250 }} class="px-4 py-3 bg-[#050505] border-t border-green-900/50 flex flex-col gap-3 overflow-x-auto">
                      <div class="text-green-600/80 font-semibold uppercase tracking-wider text-[10px]">Args</div>
                      <pre class="text-green-400 text-xs">{JSON.stringify(tc.args, null, 2)}</pre>
                      {#if tc.result}
                        <div class="text-green-600/80 font-semibold uppercase tracking-wider text-[10px] mt-2">Result</div>
                        <pre class="text-green-400 text-xs max-h-64 overflow-y-auto custom-scrollbar">{JSON.stringify(tc.result, null, 2)}</pre>
                      {/if}
                    </div>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}
          {#if msg.statuses && msg.statuses.length > 0}
            <div class="flex flex-col gap-1.5 my-2">
              {#each msg.statuses as status}
                <div in:slide={{ duration: 200 }} class="text-xs text-green-500/90 bg-green-950/40 rounded-lg px-4 py-2.5 flex items-start gap-3 border border-green-900/40 font-mono shadow-sm">
                  <Wrench size={14} class="mt-0.5 shrink-0 opacity-70" />
                  <span class="leading-relaxed">{status}</span>
                </div>
              {/each}
            </div>
          {/if}
          <div class="prose prose-invert prose-sm max-w-none prose-pre:bg-[#050505] prose-pre:border prose-pre:border-green-900/50 prose-a:text-green-400 prose-p:leading-relaxed prose-headings:text-green-300 prose-strong:text-green-300" 
               class:animate-pulse={$isGenerating && msg.id === $messages[$messages.length-1].id && !msg.content}
               style="word-wrap: break-word;">
            {#if msg.content}
              {@html renderMarkdown(msg.content)}
            {:else if $isGenerating && msg.id === $messages[$messages.length-1].id}
              <span class="text-green-600 flex items-center gap-2 mt-2"><Loader2 size={14} class="animate-spin" /> Thinking...</span>
            {/if}
          </div>
        {:else if msg.role === 'user'}
          <div class="bg-green-900/10 border border-green-900/60 rounded-2xl rounded-tl-sm px-5 py-4 text-green-300 shadow-sm leading-relaxed">
            {msg.content}
          </div>
        {:else}
          <div class="bg-rose-500/10 border border-rose-500/20 rounded-2xl rounded-tl-sm px-5 py-4 text-rose-200 font-mono text-sm shadow-sm leading-relaxed">
            {msg.content}
          </div>
        {/if}
      </div>
    {/each}
  </div>

  <div class="relative z-10 px-6 py-4 sm:px-12 sm:py-6 bg-[#0a0a0a]/90 backdrop-blur-md border-t border-green-900/60 shrink-0">
    <div class="relative bg-[#050505] rounded-xl border border-green-900 focus-within:border-green-500 focus-within:ring-2 focus-within:ring-green-500/50 focus-within:shadow-[0_0_15px_rgba(0,255,0,0.2)] transition-all shadow-inner">
      <textarea
        bind:value={inputStr}
        on:keydown={handleKeydown}
        disabled={$isGenerating || $connectionState !== "connected"}
        placeholder={$connectionState === "connected" ? "Ask anything about your codebase..." : "Connecting..."}
        class="w-full bg-transparent resize-none outline-none p-4 pr-16 min-h-[56px] max-h-64 overflow-y-auto text-green-300 disabled:opacity-50 placeholder:text-green-800"
        rows="1"
      ></textarea>
      
      <div class="absolute bottom-2.5 right-2.5 flex items-center">
        <button 
          on:click={handleSend}
          disabled={!inputStr.trim() || $isGenerating || $connectionState !== "connected"}
          class="p-2 rounded-lg bg-green-900/80 text-green-300 disabled:opacity-50 disabled:bg-green-950/50 disabled:text-green-700 transition-all hover:bg-green-800 hover:text-green-200 active:scale-95"
        >
          {#if $isGenerating}
            <Loader2 size={18} class="animate-spin" />
          {:else}
            <Send size={18} />
          {/if}
        </button>
      </div>
    </div>
    <div class="text-[10px] text-center text-green-600 mt-3 font-medium opacity-70">
      Press <kbd class="px-1.5 py-0.5 bg-green-950 rounded border border-green-900 mx-0.5">Enter</kbd> to send, <kbd class="px-1.5 py-0.5 bg-green-950 rounded border border-green-900 mx-0.5">Shift+Enter</kbd> for new line
    </div>
  </div>
</div>
