document.addEventListener('DOMContentLoaded', () => {
  const sessionIdEl = document.getElementById('session-id');
  const pageCountEl = document.getElementById('page-count');
  const pendingCountEl = document.getElementById('pending-count');
  const nativeLabel = document.getElementById('native-label');
  const statusDot = document.getElementById('status-dot');
  const syncBtn = document.getElementById('sync-btn');
  const searchInput = document.getElementById('search-input');
  const resultsList = document.getElementById('results-list');
  const apiUrlInput = document.getElementById('api-url');
  const saveBtn = document.getElementById('save-btn');
  const testBtn = document.getElementById('test-btn');
  const testResult = document.getElementById('test-result');
  const toast = document.getElementById('toast');

  // Health chips
  const hSw = document.getElementById('h-sw');
  const hWasm = document.getElementById('h-wasm');
  const hNative = document.getElementById('h-native');
  const hQueue = document.getElementById('h-queue');
  const hLast = document.getElementById('h-last');

  let toastTimer = null;

  function showToast(msg) {
    toast.textContent = msg;
    toast.classList.add('show');
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => toast.classList.remove('show'), 2000);
  }

  chrome.storage.local.get('mairu_api_url', ({ mairu_api_url }) => {
    if (mairu_api_url) apiUrlInput.value = mairu_api_url;
  });

  saveBtn.addEventListener('click', () => {
    const url = apiUrlInput.value.trim();
    if (!url) return;
    chrome.runtime.sendMessage({ type: 'set_api_url', url }, (resp) => {
      if (resp && resp.ok) {
        apiUrlInput.value = resp.url;
        chrome.storage.local.set({ mairu_api_url: resp.url });
        showToast('Saved');
      } else {
        showToast(`Invalid URL: ${resp?.error || 'unknown'}`);
      }
    });
  });

  testBtn.addEventListener('click', async () => {
    const url = apiUrlInput.value.trim().replace(/\/$/, '');
    testResult.textContent = 'Testing…';
    try {
      const r = await fetch(`${url}/api/healthz`, { method: 'GET' });
      testResult.textContent = r.ok ? 'OK' : `HTTP ${r.status}`;
    } catch (err) {
      void err;
      testResult.textContent = 'Unreachable';
    }
  });

  const devModeBtn = document.getElementById('dev-mode-btn');
  if (devModeBtn) {
    devModeBtn.addEventListener('click', () => {
      chrome.tabs.create({ url: chrome.runtime.getURL('dev/dev.html') });
    });
  }

  function formatAgo(ms) {
    if (!ms) return 'never';
    const s = Math.floor((Date.now() - ms) / 1000);
    if (s < 60) return `${s}s`;
    const m = Math.floor(s / 60);
    if (m < 60) return `${m}m`;
    return `${Math.floor(m / 60)}h`;
  }

  function updateHealth(response) {
    // SW pill
    hSw.className = 'pill ok';
    hSw.textContent = 'ok';
    // WASM
    hWasm.className = response.wasmReady ? 'pill ok' : 'pill warn';
    hWasm.textContent = response.wasmReady ? 'ready' : 'init';
    // Native
    hNative.className = response.nativeHostConnected ? 'pill ok' : 'pill err';
    hNative.textContent = response.nativeHostConnected ? 'up' : 'off';
    // Queue
    const q = response.queueSize ?? response.pendingCount ?? 0;
    hQueue.textContent = String(q);
    hQueue.className = q > 100 ? 'pill warn' : q > 0 ? 'pill ok' : 'pill';
    // Last sync
    hLast.textContent = formatAgo(response.lastSyncMs);
    hLast.className = 'pill' + (response.lastSyncMs && (Date.now() - response.lastSyncMs) > 300_000 ? ' warn' : '');
  }

  function updateStats() {
    chrome.runtime.sendMessage({ type: 'get_status' }, (response) => {
      if (chrome.runtime.lastError || !response) {
        statusDot.className = 'dot err';
        nativeLabel.textContent = 'error';
        hSw.className = 'pill err';
        hSw.textContent = 'down';
        return;
      }

      const connected = response.nativeHostConnected;
      statusDot.className = connected ? 'dot ok' : 'dot err';
      nativeLabel.textContent = connected ? 'connected' : 'disconnected';

      if (response.sessionId) {
        const parts = response.sessionId.split('-');
        sessionIdEl.textContent = parts[parts.length - 1] || response.sessionId;
        sessionIdEl.title = response.sessionId;
      }

      const summary = response.summary;
      pageCountEl.textContent = summary?.page_count ?? 0;
      pendingCountEl.textContent = response.pendingCount ?? 0;
      updateHealth(response);
    });
  }

  updateStats();
  const statsTimer = setInterval(updateStats, 3000);
  window.addEventListener('unload', () => clearInterval(statsTimer));

  let searchTimer = null;
  searchInput.addEventListener('input', () => {
    clearTimeout(searchTimer);
    const q = searchInput.value.trim();
    if (!q) {
      resultsList.innerHTML = '';
      return;
    }
    searchTimer = setTimeout(() => {
      chrome.runtime.sendMessage({ type: 'search', query: q, limit: 5 }, (res) => {
        if (!res || !res.results) {
          resultsList.innerHTML = '<div class="empty">No results</div>';
          return;
        }
        if (res.results.length === 0) {
          resultsList.innerHTML = '<div class="empty">No results</div>';
          return;
        }
        resultsList.innerHTML = res.results.map((r) => `
          <div class="result-item" data-url="${escapeAttr(r.url)}">
            <div class="title">${escapeHtml(r.title || r.url)}</div>
            <div class="url">${escapeHtml(r.url)}</div>
            ${r.snippet ? `<div class="snippet">${escapeHtml(r.snippet)}</div>` : ''}
          </div>
        `).join('');
        resultsList.querySelectorAll('.result-item').forEach((el) => {
          el.addEventListener('click', () => {
            chrome.tabs.create({ url: el.dataset.url });
          });
        });
      });
    }, 300);
  });

  syncBtn.addEventListener('click', () => {
    syncBtn.textContent = 'Syncing…';
    syncBtn.disabled = true;
    chrome.runtime.sendMessage({ type: 'force_sync' }, () => {
      syncBtn.textContent = 'Force Sync';
      syncBtn.disabled = false;
      updateStats();
      showToast('Sync complete');
    });
  });

  function escapeHtml(str) {
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }

  function escapeAttr(str) {
    return String(str).replace(/"/g, '&quot;');
  }
});
