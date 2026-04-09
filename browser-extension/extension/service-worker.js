// mairu-ext/extension/service-worker.js

import init, {
  init_session,
  process_page,
  get_current,
  get_history,
  search_session,
  get_session_summary,
  get_pending_sync,
  mark_synced,
} from "./pkg/mairu_ext_wasm.js";

let wasmReady = false;
const MAIRU_API_URL = "http://127.0.0.1:7080"; // default mairu API port
const SYNC_INTERVAL_MS = 10000;
const SYNC_BATCH_SIZE = 5;

// Initialize WASM and session
async function initialize() {
  await init();
  const sessionId = `session-${Date.now()}`;
  init_session(sessionId);
  wasmReady = true;
  console.log("[mairu-ext] WASM initialized, session:", sessionId);
}

initialize();

// Handle messages from content scripts
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (!wasmReady) return;

  if (message.type === "page_content") {
    const { url, html, timestamp, selection, active_element, console_errors, network_errors, visual_rects, storage_state } = message.payload;
    const result = process_page(
        url, 
        html, 
        timestamp, 
        selection, 
        active_element, 
        JSON.stringify(console_errors || []),
        JSON.stringify(network_errors || []),
        JSON.stringify(visual_rects || {}),
        JSON.stringify(storage_state || {})
    );
    console.log("[mairu-ext] Processed page:", url, result);
  }
});

// Handle native messaging from mairu chat (via native host)
chrome.runtime.onConnectExternal.addListener((port) => {
  port.onMessage.addListener((msg) => {
    if (!wasmReady) {
      port.postMessage({ error: "WASM not ready" });
      return;
    }

    let response;
    switch (msg.command) {
      case "current":
        response = get_current();
        break;
      case "history":
        response = get_history();
        break;
      case "search":
        response = search_session(msg.query || "", msg.limit || 5);
        break;
      case "session":
        response = get_session_summary();
        break;
      default:
        response = { error: `Unknown command: ${msg.command}` };
    }
    port.postMessage(response);
  });
});

// Background sync to mairu API
async function syncToMairu() {
  if (!wasmReady) return;

  const pending = get_pending_sync();
  if (!pending || pending.length === 0) return;

  const batch = pending.slice(0, SYNC_BATCH_SIZE);
  for (const page of batch) {
    try {
      const urlHash = page.content_hash.toString(16);
      
      let extraContent = "";
      if (page.selection) {
        extraContent += `\n\n### Current Selection\n${page.selection}\n`;
      }
      if (page.active_element) {
        extraContent += `\n\n### Active Element (Focus)\n${page.active_element}\n`;
      }
      if (page.console_errors && page.console_errors.length > 0) {
        extraContent += `\n\n### Console Errors\n${page.console_errors.join("\n")}\n`;
      }
      if (page.network_errors && page.network_errors.length > 0) {
        extraContent += `\n\n### Network Errors\n${page.network_errors.join("\n")}\n`;
      }
      
      if (page.storage_state && Object.keys(page.storage_state).length > 0) {
        extraContent += `\n\n### Storage State\n`;
        for (const [key, value] of Object.entries(page.storage_state)) {
            extraContent += `- **${key}**: ${value}\n`;
        }
      }

      if (page.visual_rects && Object.keys(page.visual_rects).length > 0) {
        extraContent += `\n\n### Visual Layout (Bounding Rects)\n`;
        for (const [key, value] of Object.entries(page.visual_rects)) {
            extraContent += `- \`${key}\`: ${value}\n`;
        }
      }

      const body = {
        uri: `contextfs://browser/${btoa(page.url)}`,
        project: "browser",
        name: page.title,
        abstract: page.sections.slice(0, 1).map((s) => s.text).join(" ").slice(0, 200),
        overview: page.sections
          .filter((s) => s.kind === "heading" || s.kind === "Heading")
          .map((s) => s.text)
          .join("\n"),
        content: page.sections.map((s) => s.text).join("\n\n") + extraContent,
      };

      const resp = await fetch(`${MAIRU_API_URL}/api/context`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });

      if (resp.ok) {
        mark_synced(page.content_hash);
        console.log("[mairu-ext] Synced:", page.url);
      }
    } catch (e) {
      console.error("[mairu-ext] Sync failed for", page.url, e);
    }
  }
}

// Start periodic sync
setInterval(syncToMairu, SYNC_INTERVAL_MS);
