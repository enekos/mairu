<script lang="ts">
  import { Send, Bot, User, Loader2, Wrench, ChevronDown, ChevronRight, CheckCircle2, XCircle, Plus } from 'lucide-svelte';
  import { messages, sendMessage, isGenerating, connectionState, sessions, currentSession, switchSession, createSession, loadSessions } from './store';
  import { marked } from 'marked';
  import DOMPurify from 'dompurify';
  import { onMount, onDestroy, tick } from 'svelte';
  import { fade, slide, fly } from 'svelte/transition';
  import { search, createMemory, listContextNodes } from './api';

  let inputStr = "";
  let messagesContainer: HTMLDivElement;
  let expandedTools: Record<string, boolean> = {};
  let creatingSession = false;
  let thinkingGlyph = "◴";
  let thinkingPhrase = "Igürikatzen...";
  let thinkingTicker: ReturnType<typeof window.setInterval> | null = null;

  const quirkySpinnerFrames = ["◴", "◷", "◶", "◵", "✶", "✸", "✺", "✸", "✶"];
  const xiberokoLoadingPhrases = [
    "Igürikatzen...",
    "Eijerki apailatzen...",
    "Mustrakan ari...",
    "Hitzak txarrantxatzen...",
    "Bürüa khilikatzen...",
    "Zühürtziaz ehuntzen...",
    "Bürü-hausterietan...",
    "Egiari hüllantzen...",
    "Aitzindarien urratsetan...",
    "Sükhalteko süan txigortzen...",
    "Mündia iraulikatzen...",
    "Satanen pheredikia asmatzen...",
    "Khordokak xuxentzen...",
    "Ülünpetik argitara jalkitzen...",
    "Düdak lürruntzen...",
    "Erran-zaharrak marraskatzen...",
    "Khexatü gabe phentsatzen...",
    "Ahapetik xuxurlatzen...",
    "Bortüetako haizea behatzen...",
    "Gogoa eküratzen...",
    "Orhoikizünak xahatzen...",
    "Belagileen artean...",
    "Ilhintiak phizten...",
    "Xühürki barnebistatzen...",
    "Errejent gisa moldatzen...",
    "Basa-ahaideak asmatzen...",
    "Zamaltzainaren jauzia prestatzen...",
    "Txülülen hotsari behatzen..."
  ];

  function randomFrom<T>(items: T[]): T {
    return items[Math.floor(Math.random() * items.length)];
  }

  function refreshThinkingText() {
    thinkingGlyph = randomFrom(quirkySpinnerFrames);
    thinkingPhrase = randomFrom(xiberokoLoadingPhrases);
  }

  function startThinkingTicker() {
    if (thinkingTicker !== null) return;
    refreshThinkingText();
    thinkingTicker = window.setInterval(() => {
      refreshThinkingText();
    }, 800 + Math.floor(Math.random() * 1000));
  }

  function stopThinkingTicker() {
    if (thinkingTicker === null) return;
    window.clearInterval(thinkingTicker);
    thinkingTicker = null;
  }

  onMount(() => {
    void loadSessions();
  });
  onDestroy(() => stopThinkingTicker());

  $: if ($isGenerating) {
    startThinkingTicker();
  } else {
    stopThinkingTicker();
  }

  $: {
    $messages;
    scrollToBottom();
  }

  function toggleTool(id: string) {
    expandedTools[id] = !expandedTools[id];
  }

  function formatToolOutput(obj: any): string {
    if (typeof obj === 'string') return obj;
    if (!obj) return '';
    if (typeof obj === 'object') {
      try {
        return Object.entries(obj).map(([k, v]) => {
          const valStr = typeof v === 'string' ? v : JSON.stringify(v, null, 2);
          return `${k}:\n${valStr}`;
        }).join('\n\n');
      } catch (e) {
        return JSON.stringify(obj, null, 2);
      }
    }
    return JSON.stringify(obj, null, 2);
  }

  async function scrollToBottom() {
    await tick();
    if (messagesContainer) {
      messagesContainer.scrollTop = messagesContainer.scrollHeight;
    }
  }

  async function handleSlashCommand(cmd: string) {
    const parts = cmd.split(" ");
    const command = parts[0].toLowerCase();
    const args = parts.slice(1).join(" ");

    // Add user message to history
    messages.update(msgs => [
      ...msgs, 
      { id: crypto.randomUUID(), role: "user", content: cmd, bashOutput: "", statuses: [], logs: [], toolCalls: [] }
    ]);

    try {
      if (command === "/clear") {
        messages.set([]);
        return;
      } else if (command === "/approve" || command === "/deny") {
        sendMessage(cmd); // Standard handling for agent approvals
        return;
      } else if (command === "/memory") {
        const subCmd = parts[1];
        const memArgs = parts.slice(2).join(" ");
        if (subCmd === "search" || subCmd === "read") {
          const data = await search({ q: memArgs, type: 'memory', topK: 15 });
          messages.update(msgs => [
            ...msgs, 
            { id: crypto.randomUUID(), role: "system", content: "Searched memories:\n" + JSON.stringify(data, null, 2), bashOutput: "", statuses: [], logs: [], toolCalls: [] }
          ]);
        } else if (subCmd === "store" || subCmd === "write") {
          await createMemory({ content: memArgs, category: "user_provided", project: "" });
          messages.update(msgs => [
            ...msgs, 
            { id: crypto.randomUUID(), role: "system", content: "Stored memory.", bashOutput: "", statuses: [], logs: [], toolCalls: [] }
          ]);
        }
      } else if (command === "/node") {
        const subCmd = parts[1];
        const nodeArgs = parts.slice(2).join(" ");
        if (subCmd === "search" || subCmd === "read") {
          const data = await search({ q: nodeArgs, type: 'context', topK: 15 });
          messages.update(msgs => [
            ...msgs, 
            { id: crypto.randomUUID(), role: "system", content: "Searched context nodes:\n" + JSON.stringify(data, null, 2), bashOutput: "", statuses: [], logs: [], toolCalls: [] }
          ]);
        } else if (subCmd === "ls") {
          const data = await listContextNodes("", nodeArgs);
          messages.update(msgs => [
            ...msgs, 
            { id: crypto.randomUUID(), role: "system", content: "Context node listing:\n" + JSON.stringify(data, null, 2), bashOutput: "", statuses: [], logs: [], toolCalls: [] }
          ]);
        }
      } else if (command === "/model") {
        if (args) {
          // Reconnect with new model
          const currentSess = $currentSession;
          // Store connection URL dynamically
          const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
          window.location.hash = `#model=${args}`; // simple way to keep state for demo, though better done in store
          
          messages.update(msgs => [
            ...msgs, 
            { id: crypto.randomUUID(), role: "system", content: `Switching model to: ${args}`, bashOutput: "", statuses: [], logs: [], toolCalls: [] }
          ]);
          
          // Reconnect with new model
          const wsUrl = `${protocol}//${window.location.host}/api/chat?session=${encodeURIComponent(currentSess)}&model=${encodeURIComponent(args)}`;
          // We would actually need to update store.ts connectWs to accept model, but for now we'll just log it
          messages.update(msgs => [
            ...msgs, 
            { id: crypto.randomUUID(), role: "system", content: `Model switch requested (Note: fully dynamic switching requires refreshing connection with ?model=${args})`, bashOutput: "", statuses: [], logs: [], toolCalls: [] }
          ]);
        }
      } else {
        // Fallback for unknown slash commands, send to agent anyway
        sendMessage(cmd);
      }
    } catch (e) {
      messages.update(msgs => [
        ...msgs, 
        { id: crypto.randomUUID(), role: "system", content: `Command failed: ${e instanceof Error ? e.message : String(e)}`, bashOutput: "", statuses: [], logs: [], toolCalls: [] }
      ]);
    }
  }

  function handleSend() {
    const input = inputStr.trim();
    if (!input || $isGenerating || $connectionState !== "connected") return;
    
    if (input.startsWith("/")) {
      handleSlashCommand(input);
    } else {
      sendMessage(input);
    }
    
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

<div class="w-full max-w-4xl mx-auto flex-1 flex flex-col relative bg-[#0d0d12] border border-indigo-800/50  overflow-hidden shadow-[0_0_40px_rgba(137,220,235,0.15)] transition-all max-h-[calc(100vh-140px)]">
    <!-- Faded green background image -->
    <div class="absolute inset-0 pointer-events-none z-0 opacity-[0.08] mix-blend-screen" style="background-image: url('/mairu.png'); background-size: cover; background-position: center; background-repeat: no-repeat; filter: invert(1) sepia(1) hue-rotate(280deg) saturate(300%);"></div>

  <div class="relative z-10 min-h-16 border-b border-indigo-800/60 flex items-center gap-4 chat-gutter py-4 sm:py-5 shrink-0 bg-[#0d0d12]/95 backdrop-blur-md sticky top-0 shadow-sm">
    <div class="flex flex-col gap-1 pl-0.5">
      <div class="text-[15px] font-bold tracking-wide flex items-center gap-3 text-cyan-300">
        Mairu Agent
        {#if $connectionState === "connected"}
          <span class="w-2.5 h-2.5  bg-pink-400 shadow-[0_0_10px_rgba(245,194,231,0.6)] animate-pulse"></span>
        {:else}
          <span class="w-2.5 h-2.5  bg-amber-500 shadow-[0_0_10px_rgba(245,158,11,0.6)]"></span>
        {/if}
      </div>
      <p class="text-[11px] tracking-wide text-indigo-400/90">Codebase assistant with live tool traces</p>
    </div>
    <div class="ml-auto flex items-center gap-3 pr-0.5">
      <label class="text-[10px] font-bold uppercase tracking-widest text-cyan-400/80 px-1.5" for="session-select">Session</label>
      <select
        id="session-select"
        class="bg-[#11111b] border border-indigo-800/60  px-3.5 py-2 text-xs font-medium text-pink-200 min-w-40 focus:border-cyan-400 focus:ring-1 focus:ring-cyan-400 outline-none transition-colors"
        value={$currentSession}
        on:change={handleSessionSwitch}
        disabled={$isGenerating}
      >
        {#each $sessions as session}
          <option value={session}>{session}</option>
        {/each}
      </select>
      <button
        class="p-2.5  border border-indigo-800/60 hover:border-cyan-400 hover:bg-indigo-800/20 hover:text-pink-200 text-cyan-400 transition-all active:scale-95 disabled:opacity-50"
        on:click={handleCreateSession}
        disabled={$isGenerating || creatingSession}
        title="Create session"
      >
        <Plus size={14} />
      </button>
    </div>
  </div>

  <div class="flex-1 overflow-y-auto relative z-10 chat-gutter py-10 sm:py-12 lg:py-14 space-y-10" bind:this={messagesContainer}>
    {#if $messages.length === 0}
      <div in:fade={{ duration: 400 }} class="h-full flex flex-col items-center justify-center text-cyan-400 chat-gutter py-14">
        <Bot size={56} class="mb-6 opacity-20" />
        <p class="text-lg font-semibold text-pink-300">Mairu is ready.</p>
        <p class="text-sm opacity-70 mt-2">Ask a codebase question or request a code change.</p>
      </div>
    {/if}

    {#each $messages as msg (msg.id)}
      <div class="flex flex-col gap-4 chat-item-inset" in:fly={{ y: 20, duration: 400 }}>
        <div class="flex items-center gap-2.5 text-[11px] font-semibold uppercase tracking-[0.16em] text-cyan-400 px-7 py-2.5 mx-4 sm:mx-8 sm:px-9 sm:py-3">
          {#if msg.role === 'user'}
            <User size={14} class="text-cyan-300" /> You
          {:else if msg.role === 'assistant'}
            <Bot size={14} class="text-cyan-300" /> Mairu
          {:else}
            <span class="text-rose-400">System</span>
          {/if}
        </div>
        
        {#if msg.role === 'assistant'}
          {#if msg.toolCalls && msg.toolCalls.length > 0}
            <div class="flex flex-col gap-3 my-1">
              <div class="px-8 sm:px-10 mx-4 sm:mx-8 text-[10px] font-bold uppercase tracking-[0.2em] text-indigo-400/90">Tool activity</div>
              {#each msg.toolCalls as tc}
                <div class="flex flex-col text-xs bg-[#11111b]/30 border border-indigo-800/50 mx-4 sm:mx-8 rounded-sm overflow-hidden font-mono shadow-sm transition-all hover:border-indigo-600/50">
                  <button class="flex items-center gap-3 px-8 py-4 text-cyan-300 hover:bg-indigo-800/20 transition-colors" on:click={() => toggleTool(tc.id)}>
                    {#if expandedTools[tc.id]}
                      <ChevronDown size={14} class="shrink-0" />
                    {:else}
                      <ChevronRight size={14} class="shrink-0" />
                    {/if}
                    <Wrench size={14} class={`shrink-0 ${tc.status === 'running' ? 'text-amber-400' : 'text-cyan-400'}`} />
                    <span class="font-semibold text-sm">{tc.name}</span>
                    <span class="flex-1 text-left text-cyan-400/70 truncate ml-2">
                      {JSON.stringify(tc.args)}
                    </span>
                    {#if tc.status === 'running'}
                      <Loader2 size={14} class="animate-spin text-amber-400 shrink-0" />
                    {:else if tc.status === 'completed' && !tc.result?.error}
                      <CheckCircle2 size={14} class="text-cyan-300 shrink-0" />
                    {:else}
                      <XCircle size={14} class="text-rose-400 shrink-0" />
                    {/if}
                  </button>
                  {#if expandedTools[tc.id]}
                    <div transition:slide={{ duration: 250 }} class="px-8 py-4 bg-[#11111b] border-t border-indigo-800/50 flex flex-col gap-4 overflow-x-auto">
                      <div class="text-cyan-400/80 font-semibold uppercase tracking-wider text-[10px] px-1">Args</div>
                      <pre class="text-cyan-300 text-xs pl-1 pr-2">{formatToolOutput(tc.args)}</pre>
                      {#if tc.result}
                        <div class="text-cyan-400/80 font-semibold uppercase tracking-wider text-[10px] mt-1 px-1">Result</div>
                        <pre class="text-cyan-300 text-xs max-h-64 overflow-y-auto custom-scrollbar pl-1 pr-2">{formatToolOutput(tc.result)}</pre>
                      {/if}
                    </div>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}
          {#if msg.statuses && msg.statuses.length > 0}
            <div class="flex flex-col gap-2.5 my-1">
              <div class="px-8 sm:px-10 mx-4 sm:mx-8 text-[10px] font-bold uppercase tracking-[0.2em] text-indigo-400/90">Progress</div>
              {#each msg.statuses as status}
                <div in:slide={{ duration: 200 }} class="text-xs text-pink-300/90 bg-[#11111b]/40 mx-4 sm:mx-8 rounded-sm px-8 py-3.5 flex items-start gap-4 border border-indigo-800/40 font-mono shadow-sm">
                  <Wrench size={14} class="mt-0.5 shrink-0 opacity-70" />
                  <span class="leading-relaxed">{status}</span>
                </div>
              {/each}
            </div>
          {/if}
          {#if msg.bashOutput}
            <div class="flex flex-col gap-2.5 my-1" transition:slide={{ duration: 200 }}>
              <div class="px-8 sm:px-10 mx-4 sm:mx-8 text-[10px] font-bold uppercase tracking-[0.2em] text-indigo-400/90">Running Bash...</div>
              <div class="text-xs text-pink-300/90 bg-[#11111b]/80 mx-4 sm:mx-8 rounded-sm px-8 py-3.5 border border-indigo-800/80 font-mono shadow-sm overflow-x-auto">
                <pre class="whitespace-pre-wrap">{msg.bashOutput}</pre>
              </div>
            </div>
          {/if}
          <div class="px-8 sm:px-10 mx-4 sm:mx-8 text-[10px] font-bold uppercase tracking-[0.2em] text-indigo-400/90">Response</div>
          <div class="prose prose-invert prose-sm max-w-none mx-4 sm:mx-8 rounded-sm px-8 sm:px-10 py-6 bg-[#11111b]/50 border border-indigo-800/35 prose-pre:bg-[#11111b] prose-pre:border prose-pre:border-indigo-800/50 prose-pre:px-6 prose-pre:py-3 prose-a:text-cyan-300 prose-p:leading-relaxed prose-headings:text-pink-200 prose-strong:text-pink-200" 
               class:animate-pulse={$isGenerating && msg.id === $messages[$messages.length-1].id && !msg.content}
               style="word-wrap: break-word;">
            {#if msg.content}
              {@html renderMarkdown(msg.content)}
            {:else if $isGenerating && msg.id === $messages[$messages.length-1].id}
              <span class="text-cyan-400 flex items-center gap-2 mt-2">
                <Loader2 size={14} class="animate-spin" />
                <span class="font-semibold text-pink-300">{thinkingGlyph}</span>
                <span class="italic">{thinkingPhrase}</span>
              </span>
            {/if}
          </div>
        {:else if msg.role === 'user'}
          <div class="px-8 sm:px-10 mx-4 sm:mx-8 text-[10px] font-bold uppercase tracking-[0.2em] text-indigo-400/90">Prompt</div>
          <div class="bg-indigo-800/10 border border-indigo-800/60 mx-4 sm:mx-8 rounded-sm px-8 sm:px-10 py-5 text-pink-200 shadow-sm leading-relaxed">
            {msg.content}
          </div>
        {:else}
          <div class="px-8 sm:px-10 mx-4 sm:mx-8 text-[10px] font-bold uppercase tracking-[0.2em] text-rose-400/80">System</div>
          <div class="bg-rose-500/10 border border-rose-500/20 mx-4 sm:mx-8 rounded-sm px-8 sm:px-10 py-5 text-rose-200 font-mono text-sm shadow-sm leading-relaxed whitespace-pre-wrap">
            {msg.content}
            {#if msg.content.includes('/approve')}
              <div class="mt-4 flex gap-4">
                <button on:click={() => sendMessage('/approve')} class="px-4 py-2 bg-green-500/20 text-green-400 border border-green-500/50  hover:bg-green-500/30 transition-colors cursor-pointer">Approve</button>
                <button on:click={() => sendMessage('/deny')} class="px-4 py-2 bg-red-500/20 text-red-400 border border-red-500/50  hover:bg-red-500/30 transition-colors cursor-pointer">Deny</button>
              </div>
            {/if}
          </div>
        {/if}
      </div>
    {/each}
  </div>

  <div class="relative z-10 chat-gutter py-5 sm:py-7 bg-[#0d0d12]/90 backdrop-blur-md border-t border-indigo-800/60 shrink-0">
    <div class="chat-item-inset mx-4 sm:mx-8">
      <div class="text-[10px] font-semibold uppercase tracking-[0.16em] text-indigo-400/85 mb-3 px-1.5">Compose</div>
      <div class="relative bg-[#11111b] rounded-sm border border-indigo-800 focus-within:border-cyan-400 focus-within:ring-2 focus-within:ring-cyan-400/50 focus-within:shadow-[0_0_15px_rgba(137,220,235,0.2)] transition-all shadow-inner p-1">
        <textarea
          bind:value={inputStr}
          on:keydown={handleKeydown}
          disabled={$isGenerating || $connectionState !== "connected"}
          placeholder={$connectionState === "connected" ? "Ask anything about your codebase..." : "Connecting..."}
          class="w-full bg-transparent resize-none outline-none px-8 sm:px-10 py-5 pr-[4.75rem] min-h-[56px] max-h-64 overflow-y-auto text-pink-200 disabled:opacity-50 placeholder:text-indigo-500"
          rows="1"
        ></textarea>
        
        <div class="absolute bottom-3 right-3 flex items-center">
          <button 
            on:click={handleSend}
            disabled={!inputStr.trim() || $isGenerating || $connectionState !== "connected"}
            class="p-2.5 rounded-sm bg-indigo-800/80 text-pink-200 disabled:opacity-50 disabled:bg-[#11111b]/50 disabled:text-indigo-400 transition-all hover:bg-indigo-700 hover:text-pink-100 active:scale-95"
          >
            {#if $isGenerating}
              <Loader2 size={18} class="animate-spin" />
            {:else}
              <Send size={18} />
            {/if}
          </button>
        </div>
      </div>
      <div class="text-[10px] text-center text-cyan-400 mt-4 font-medium opacity-70 leading-relaxed">
        Press <kbd class="px-1.5 py-0.5 bg-[#11111b] rounded-sm border border-indigo-800 mx-0.5">Enter</kbd> to send, <kbd class="px-1.5 py-0.5 bg-[#11111b] rounded-sm border border-indigo-800 mx-0.5">Shift+Enter</kbd> for new line
      </div>
    </div>
  </div>
</div>

<style>
  .chat-gutter {
    padding-left: clamp(1.75rem, 4vw, 4.5rem);
    padding-right: clamp(1.75rem, 4vw, 4.5rem);
  }

  .chat-item-inset {
    padding-left: clamp(0.5rem, 1.2vw, 1rem);
    padding-right: clamp(0.5rem, 1.2vw, 1rem);
  }
</style>
