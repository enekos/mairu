// Mairu MV3 service worker.
// Responsibilities: WASM session host, native-host bridge, durable sync queue,
// context menu, page-context trap injection (MAIN world via chrome.scripting).

import init, {
  init_session,
  process_page,
  get_current,
  get_history,
  search_session,
  get_session_summary,
  get_pending_sync,
  mark_synced,
  export_session,
  import_session,
} from './pkg/browser_extension_wasm.js';

import { createLogger } from './lib/logger.js';
import { validate as validateMessage } from './lib/messages.js';
import { validateApiUrl } from './lib/url.js';
import { createQueue } from './lib/queue.js';
import { syncOnce, buildPayload } from './lib/sync.js';
import { createMailbox } from './lib/mailbox.js';

const SESSION_STORAGE_KEY = 'mairu_session_state';
const API_URL_KEY = 'mairu_api_url';
const QUEUE_KEY = 'mairu_queue_v1';
const DEFAULT_API_URL = 'http://127.0.0.1:7080';

const logger = createLogger({ capacity: 1000 });
logger.subscribe((entry) => {
  try { chrome.runtime.sendMessage({ type: 'dev_log', entry }); } catch (err) { void err; }
});
globalThis.__mairuLogger = logger;

let wasmReady = false;
let currentSessionId = null;
let mairApiUrl = DEFAULT_API_URL;
let lastSyncMs = 0;

const queue = createQueue({ storageKey: QUEUE_KEY, maxEntries: 500 });
const mailbox = createMailbox({ cap: 20, logger });

async function initialize() {
  await init();

  try {
    const stored = await chrome.storage.session.get(SESSION_STORAGE_KEY);
    const json = stored[SESSION_STORAGE_KEY];
    if (json && import_session(json)) {
      const summary = get_session_summary();
      currentSessionId = summary?.id || `session-${Date.now()}`;
      logger.info('session.restored', { id: currentSessionId });
    } else {
      throw new Error('no stored session');
    }
  } catch (err) {
    void err;
    currentSessionId = `session-${Date.now()}`;
    init_session(currentSessionId, BigInt(Math.floor(Date.now() / 1000)));
    logger.info('session.new', { id: currentSessionId });
  }

  try {
    const stored = await chrome.storage.local.get(API_URL_KEY);
    if (stored[API_URL_KEY]) {
      const v = validateApiUrl(stored[API_URL_KEY]);
      if (v.ok) mairApiUrl = v.url;
      else logger.warn('config.api_url.invalid', { stored: stored[API_URL_KEY] });
    }
  } catch (err) {
    void err;
  }

  await queue.load();
  logger.info('queue.loaded', { size: queue.size(), dropped: queue.droppedCount() });

  wasmReady = true;
  mailbox.flush(({ message, sender, sendResponse }) => handleMessage(message, sender, sendResponse));
}

async function persistSession() {
  if (!wasmReady) return;
  try {
    const json = export_session();
    if (json) await chrome.storage.session.set({ [SESSION_STORAGE_KEY]: json });
  } catch (err) {
    logger.warn('session.persist_fail', { err: String(err) });
  }
}

async function enqueueProcessedPages() {
  const pending = get_pending_sync() || [];
  for (const page of pending) {
    try {
      await queue.enqueue(buildPayload(page));
      // content_hash is hex-serialized by the core crate; parse to BigInt for the wasm boundary.
      mark_synced(BigInt('0x' + page.content_hash));
    } catch (err) {
      logger.warn('queue.enqueue_fail', { err: String(err), url: page.url });
    }
  }
  if (pending.length) {
    logger.debug('queue.enqueued', { count: pending.length, size: queue.size() });
    persistSession();
  }
}

async function syncTick() {
  try {
    const r = await syncOnce(queue, mairApiUrl);
    if (r.ok > 0) {
      lastSyncMs = Date.now();
      logger.info('sync.ok', { acked: r.ok, remaining: queue.size() });
    }
    if (r.fail > 0) logger.warn('sync.fail', { failed: r.fail, remaining: queue.size() });
  } catch (err) {
    logger.warn('sync.loop_fail', { err: String(err) });
  }
}

function handleMessage(message, sender, sendResponse) {
  const v = validateMessage(message);
  if (!v.ok) {
    logger.warn('message.invalid', { type: message?.type, error: v.error });
    return false;
  }

  const needsWasm = message.type === 'page_content' || message.type === 'search';
  if (needsWasm && !wasmReady) {
    mailbox.push({ message, sender, sendResponse });
    return true;
  }

  if (message.type === 'page_content') {
    const p = message.payload;
    const result = process_page({
      url: p.url,
      html: p.html,
      timestamp: p.timestamp,
      selection: p.selection,
      active_element: p.active_element,
      console_errors_json: JSON.stringify(p.console_errors || []),
      network_errors_json: JSON.stringify(p.network_errors || []),
      visual_rects_json: JSON.stringify(p.visual_rects || {}),
      storage_state_json: JSON.stringify(p.storage_state || {}),
      dwell_ms: p.dwell_ms || 0,
      interaction_count: p.interaction_count || 0,
      iframes_json: JSON.stringify(p.iframes || []),
      truncated: !!p.truncated,
    });
    if (result && result.ok === false) {
      logger.warn('wasm.bad_args', { url: p.url, error: result.error });
      return false;
    }
    logger.debug('page.processed', { url: p.url, status: result?.status, truncated: !!p.truncated });
    if (result?.status === 'added' || result?.status === 'updated') {
      enqueueProcessedPages();
    }
  } else if (message.type === 'get_status') {
    sendResponse({
      sessionId: currentSessionId,
      summary: get_session_summary(),
      pendingCount: queue.size(),
      queueSize: queue.size(),
      queueDropped: queue.droppedCount(),
      wasmReady,
      lastSyncMs,
      nativeHostConnected: nativePort !== null,
    });
  } else if (message.type === 'get_dev_state') {
    sendResponse({
      sessionId: currentSessionId,
      summary: get_session_summary(),
      pendingCount: queue.size(),
      queueSize: queue.size(),
      queueDropped: queue.droppedCount(),
      nativeHostConnected: nativePort !== null,
      wasmReady,
      apiUrl: mairApiUrl,
      pendingQueue: queue.all(),
      lastSyncMs,
    });
  } else if (message.type === 'get_logs') {
    sendResponse({ entries: logger.snapshot() });
  } else if (message.type === 'clear_queue') {
    queue.clear().then(() => sendResponse({ ok: true }));
    return true;
  } else if (message.type === 'reset_session') {
    chrome.storage.session.remove(SESSION_STORAGE_KEY, () => {
      currentSessionId = `session-${Date.now()}`;
      init_session(currentSessionId, BigInt(Math.floor(Date.now() / 1000)));
      persistSession();
      sendResponse({ ok: true });
    });
    return true;
  } else if (message.type === 'search') {
    const results = search_session(message.query || '', message.limit || 5);
    sendResponse({ results });
  } else if (message.type === 'set_api_url') {
    const vu = validateApiUrl(message.url || '');
    if (!vu.ok) {
      sendResponse({ ok: false, error: vu.error });
      return false;
    }
    mairApiUrl = vu.url;
    chrome.storage.local.set({ [API_URL_KEY]: vu.url }).catch(() => {});
    logger.info('config.api_url.set', { url: vu.url });
    sendResponse({ ok: true, url: vu.url });
  } else if (message.type === 'force_sync') {
    syncTick().then(() => sendResponse({ success: true }));
    return true;
  }
  return false;
}

chrome.runtime.onMessage.addListener(handleMessage);

// Page-context trap injection (CSP-safe via MAIN world).
chrome.webNavigation.onCommitted.addListener(async (details) => {
  if (details.frameId !== 0) return;
  if (!/^https?:/.test(details.url)) return;
  try {
    await chrome.scripting.executeScript({
      target: { tabId: details.tabId, frameIds: [0] },
      world: 'MAIN',
      files: ['main-world/traps.js'],
    });
  } catch (err) {
    logger.debug('inject.fail', { err: String(err), url: details.url });
  }
});

// Native host bridge
let nativePort = null;
let nativeRetryMs = 1000;

function connectNativeHost() {
  try {
    nativePort = chrome.runtime.connectNative('com.mairu.browser_context');
  } catch (err) {
    logger.warn('native.connect_fail', { err: String(err) });
    setTimeout(connectNativeHost, nativeRetryMs);
    nativeRetryMs = Math.min(nativeRetryMs * 2, 60_000);
    return;
  }
  nativeRetryMs = 1000;

  nativePort.onMessage.addListener((msg) => {
    if (!wasmReady) {
      nativePort.postMessage({ id: msg.id, error: 'WASM not ready' });
      return;
    }

    let response;
    switch (msg.command) {
      case 'current':
        response = get_current();
        break;
      case 'history':
        response = get_history();
        break;
      case 'search':
        response = search_session(msg.query || '', msg.limit || 5);
        break;
      case 'session':
        response = get_session_summary();
        break;
      case 'screenshot':
        chrome.tabs.captureVisibleTab(null, { format: 'jpeg', quality: 50 }, (dataUrl) => {
          if (chrome.runtime.lastError) {
            nativePort.postMessage({ id: msg.id, error: chrome.runtime.lastError.message });
          } else {
            nativePort.postMessage({ id: msg.id, dataUrl });
          }
        });
        return;
      case 'set_cookie':
        chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
          if (tabs[0] && tabs[0].url) {
            const url = new URL(tabs[0].url);
            chrome.cookies.set({
              url: url.origin,
              name: msg.name,
              value: msg.value,
              path: msg.path || '/',
              domain: msg.domain || url.hostname,
            }, (_cookie) => {
              if (chrome.runtime.lastError) {
                nativePort.postMessage({ id: msg.id, error: chrome.runtime.lastError.message });
              } else {
                nativePort.postMessage({ id: msg.id, success: true, message: `Set cookie ${msg.name}` });
              }
            });
          } else {
            nativePort.postMessage({ id: msg.id, error: 'No active tab found to set cookie' });
          }
        });
        return;
      default:
        if (msg.type === 'execute') {
          chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
            if (tabs[0]) {
              chrome.tabs.sendMessage(tabs[0].id, msg, (contentResponse) => {
                nativePort.postMessage({ id: msg.id, ...(contentResponse || { error: 'no response' }) });
              });
            } else {
              nativePort.postMessage({ id: msg.id, error: 'No active tab found' });
            }
          });
          return;
        }
        response = { error: `Unknown command: ${msg.command}` };
    }

    nativePort.postMessage({ id: msg.id, ...response });
  });

  nativePort.onDisconnect.addListener(() => {
    logger.info('native.disconnect');
    nativePort = null;
    const delay = nativeRetryMs + Math.floor(Math.random() * 1000);
    setTimeout(connectNativeHost, delay);
    nativeRetryMs = Math.min(nativeRetryMs * 2, 60_000);
  });
}

connectNativeHost();

// Context menus
chrome.runtime.onInstalled.addListener(() => {
  chrome.contextMenus.create({
    id: 'send-to-mairu',
    title: 'Send to Mairu Agent',
    contexts: ['selection', 'link', 'image', 'page'],
  });
});

chrome.contextMenus.onClicked.addListener(async (info, tab) => {
  if (info.menuItemId !== 'send-to-mairu') return;
  let content = '';
  if (info.selectionText) content = `Selected text on ${tab.url}:\n\n${info.selectionText}`;
  else if (info.linkUrl) content = `Link on ${tab.url}:\n\n${info.linkUrl}`;
  else if (info.srcUrl) content = `Image on ${tab.url}:\n\n${info.srcUrl}`;
  else content = `Page: ${tab.url}`;

  const body = {
    uri: `contextfs://browser/manual/${Date.now()}`,
    project: 'browser',
    name: `User Selection from ${tab.title || 'Browser'}`,
    abstract: content.slice(0, 100),
    overview: 'User explicitly sent this context to the agent.',
    content,
  };
  try {
    const resp = await fetch(`${mairApiUrl}/api/context`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (resp.ok) logger.info('context_menu.sent', { uri: body.uri });
    else logger.warn('context_menu.fail', { status: resp.status });
  } catch (err) {
    logger.warn('context_menu.fail', { err: String(err) });
  }
});

// Periodic sync via alarms (plays nicer with MV3 service-worker suspension than setInterval).
chrome.alarms.create('mairu-sync', { periodInMinutes: 0.25 });
chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === 'mairu-sync') syncTick();
});
self.addEventListener('online', () => syncTick());

initialize().catch((err) => {
  logger.error('init.fail', { err: String(err) });
});
