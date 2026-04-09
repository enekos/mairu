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

let nativePort = null;

function connectNativeHost() {
  nativePort = chrome.runtime.connectNative('com.mairu.browser_context');
  
  nativePort.onMessage.addListener((msg) => {
    if (!wasmReady) {
      nativePort.postMessage({ id: msg.id, error: "WASM not ready" });
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
      case "screenshot":
        chrome.tabs.captureVisibleTab(null, { format: "jpeg", quality: 50 }, (dataUrl) => {
          if (chrome.runtime.lastError) {
             nativePort.postMessage({ id: msg.id, error: chrome.runtime.lastError.message });
          } else {
             nativePort.postMessage({ id: msg.id, dataUrl });
          }
        });
        return; // Async response handled in callback
      default:
        // If it's an execute command, forward it to the active tab's content script
        if (msg.type === "execute") {
          chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
             if (tabs[0]) {
               chrome.tabs.sendMessage(tabs[0].id, msg, (contentResponse) => {
                  nativePort.postMessage({ id: msg.id, ...contentResponse });
               });
             } else {
               nativePort.postMessage({ id: msg.id, error: "No active tab found" });
             }
          });
          return; // Async response handled in callback
        } else {
          response = { error: `Unknown command: ${msg.command}` };
        }
    }
    
    // Always include the request ID in the response so the native host can route it
    nativePort.postMessage({ id: msg.id, ...response });
  });

  nativePort.onDisconnect.addListener(() => {
    console.log("[mairu-ext] Native host disconnected. Retrying in 5s...");
    nativePort = null;
    setTimeout(connectNativeHost, 5000);
  });
}

// Start the connection
connectNativeHost();

// Context Menus
chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.create({
    id: "send-to-mairu",
    title: "Send to Mairu Agent",
    contexts: ["selection", "link", "image", "page"]
  });
});

chrome.contextMenus.onClicked.addListener(async (info, tab) => {
  if (info.menuItemId === "send-to-mairu") {
    let content = "";
    if (info.selectionText) {
      content = `Selected text on ${tab.url}:\n\n${info.selectionText}`;
    } else if (info.linkUrl) {
      content = `Link on ${tab.url}:\n\n${info.linkUrl}`;
    } else if (info.srcUrl) {
      content = `Image on ${tab.url}:\n\n${info.srcUrl}`;
    } else {
      content = `Page: ${tab.url}`;
    }
    
    // We send this as a special manual memory/context block
    const body = {
      uri: `contextfs://browser/manual/${Date.now()}`,
      project: "browser",
      name: `User Selection from ${tab.title || 'Browser'}`,
      abstract: content.slice(0, 100),
      overview: "User explicitly sent this context to the agent.",
      content: content,
    };

    try {
      const resp = await fetch(`${MAIRU_API_URL}/api/context`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      if (resp.ok) {
        console.log("[mairu-ext] Sent manual context successfully.");
      }
    } catch (e) {
      console.error("[mairu-ext] Failed to send manual context:", e);
    }
  }
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
