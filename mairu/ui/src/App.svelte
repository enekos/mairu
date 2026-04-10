<script lang="ts">
  import { onMount } from "svelte";
  import { connectChat, activeView } from "./lib/store";
  import Sidebar from "./lib/Sidebar.svelte";
  import Chat from "./lib/Chat.svelte";
  import Workspace from "./lib/Workspace.svelte";
  import Dashboard from "./lib/Dashboard.svelte";
  import LogViewer from "./lib/LogViewer.svelte";
  import Settings from "./lib/Settings.svelte";
  import "./app.css";

  const isWails = typeof window !== 'undefined' && !!window.go?.desktop?.App;

  let appReady = $state(!isWails); // web mode is always ready
  let statusMessage = $state('Starting...');
  let downloadProgress = $state(-1); // -1 = no download
  let startupError = $state('');

  onMount(() => {
    connectChat();

    // Check url hash for view/model states on load
    const hash = window.location.hash;
    if (hash) {
      if (hash.includes('view=')) {
        const view = hash.split('view=')[1].split('&')[0];
        if (view === 'chat' || view === 'workspace' || view === 'dashboard' || view === 'logs' || view === 'settings') {
          activeView.set(view);
        }
      }
    }

    // Listen for native menu navigation events
    if (window.runtime) {
      const rt = window.runtime;
      rt.EventsOn('app:status', (msg: string) => { statusMessage = msg; });
      rt.EventsOn('app:download-progress', (pct: number) => { downloadProgress = pct; });
      rt.EventsOn('app:error', (err: string) => { startupError = err; });
      rt.EventsOn('app:ready', () => { appReady = true; });
      rt.EventsOn('nav:view', (view: string) => {
        if (view === 'chat' || view === 'workspace' || view === 'dashboard' || view === 'logs' || view === 'settings') {
          activeView.set(view);
        }
      });
    }
  });
</script>

{#if !appReady}
  <div class="fixed inset-0 bg-black flex items-center justify-center">
    <div class="text-center space-y-4">
      <h1 class="text-green-500 text-2xl font-mono">Mairu</h1>
      {#if startupError}
        <p class="text-red-400">{startupError}</p>
        <button class="text-green-500 border border-green-500 px-4 py-1 rounded"
          onclick={() => window.location.reload()}>Retry</button>
      {:else}
        <p class="text-green-300">{statusMessage}</p>
        {#if downloadProgress >= 0}
          <div class="w-64 h-2 bg-green-900 rounded mx-auto">
            <div class="h-full bg-green-500 rounded" style="width: {downloadProgress}%"></div>
          </div>
        {/if}
      {/if}
    </div>
  </div>
{:else}
<div class="flex h-screen bg-black text-green-500 overflow-hidden font-sans">
  <Sidebar />
  <div class="flex-1 flex flex-col relative overflow-hidden">
    {#if $activeView === 'chat'}
      <Chat />
    {:else if $activeView === 'workspace'}
      <Workspace />
    {:else if $activeView === 'dashboard'}
      <Dashboard />
    {:else if $activeView === 'logs'}
      <LogViewer />
    {:else if $activeView === 'settings'}
      <Settings />
    {/if}
  </div>
</div>
{/if}
