document.addEventListener('DOMContentLoaded', () => {
  // Navigation
  const navItems = document.querySelectorAll('.nav-item');
  const views = document.querySelectorAll('.view');

  navItems.forEach(item => {
    item.addEventListener('click', (e) => {
      e.preventDefault();
      const targetId = item.getAttribute('data-target');
      
      navItems.forEach(nav => nav.classList.remove('active'));
      item.classList.add('active');
      
      views.forEach(view => {
        if (view.id === targetId) {
          view.classList.add('active');
        } else {
          view.classList.remove('active');
        }
      });
    });
  });

  // State
  let currentPending = [];

  // DOM Elements - System
  const nativeDot = document.getElementById('sys-native-dot');
  const nativeStatus = document.getElementById('sys-native-status');
  const wasmDot = document.getElementById('sys-wasm-dot');
  const wasmStatus = document.getElementById('sys-wasm-status');
  const sessionIdEl = document.getElementById('sys-session-id');
  const apiUrlEl = document.getElementById('sys-api-url');
  const pagesCountEl = document.getElementById('sys-pages-count');
  const queueCountEl = document.getElementById('sys-queue-count');

  // DOM Elements - Queue
  const queueList = document.getElementById('queue-list');
  const queueEmpty = document.getElementById('queue-empty');
  const payloadInspector = document.getElementById('payload-inspector');
  const inspectorTitle = document.getElementById('inspector-title');
  const inspectorContent = document.getElementById('inspector-content');
  const btnCloseInspector = document.getElementById('btn-close-inspector');

  // DOM Elements - Logs
  const logContainer = document.getElementById('log-container');
  const btnClearLogs = document.getElementById('btn-clear-logs');

  // DOM Elements - Actions
  const btnForceSync = document.getElementById('btn-force-sync');
  const btnClearQueue = document.getElementById('btn-clear-queue');
  const btnResetSession = document.getElementById('btn-reset-session');
  const btnEvalTab = document.getElementById('btn-eval-tab');
  const liveEvalResult = document.getElementById('live-eval-result');
  const liveEvalContent = document.getElementById('live-eval-content');

  // Polling for State Updates
  function updateState() {
    chrome.runtime.sendMessage({ type: 'get_dev_state' }, (res) => {
      if (chrome.runtime.lastError || !res) {
        setSystemError();
        return;
      }
      renderSystemState(res);
      renderQueue(res.pendingQueue || []);
    });
  }

  function setSystemError() {
    nativeDot.className = 'dot err';
    nativeStatus.textContent = 'Disconnected (Extension Error)';
    wasmDot.className = 'dot err';
    wasmStatus.textContent = 'Unknown';
  }

  function renderSystemState(state) {
    nativeDot.className = state.nativeHostConnected ? 'dot ok' : 'dot err';
    nativeStatus.textContent = state.nativeHostConnected ? 'Connected' : 'Disconnected';
    
    wasmDot.className = state.wasmReady ? 'dot ok' : 'dot err';
    wasmStatus.textContent = state.wasmReady ? 'Ready' : 'Not Initialized';

    sessionIdEl.textContent = state.sessionId || 'None';
    apiUrlEl.textContent = state.apiUrl || 'None';
    
    pagesCountEl.textContent = state.summary?.page_count ?? 0;
    queueCountEl.textContent = state.pendingCount ?? 0;
  }

  function renderQueue(queue) {
    // Only re-render if queue changed to avoid jumping UI
    if (JSON.stringify(queue.map(q => q.content_hash)) === JSON.stringify(currentPending.map(q => q.content_hash))) {
      return;
    }
    
    currentPending = queue;

    if (queue.length === 0) {
      queueEmpty.classList.remove('hidden');
      queueList.classList.add('hidden');
      payloadInspector.classList.add('hidden');
      return;
    }

    queueEmpty.classList.add('hidden');
    queueList.classList.remove('hidden');
    
    queueList.innerHTML = queue.map((item, idx) => `
      <div class="queue-item" data-idx="${idx}">
        <div class="queue-title">${escapeHtml(item.title || 'Untitled')}</div>
        <div class="queue-url">${escapeHtml(item.url)}</div>
      </div>
    `).join('');

    document.querySelectorAll('.queue-item').forEach(el => {
      el.addEventListener('click', () => {
        document.querySelectorAll('.queue-item').forEach(i => i.classList.remove('active'));
        el.classList.add('active');
        const idx = parseInt(el.getAttribute('data-idx'), 10);
        showPayload(currentPending[idx]);
      });
    });
  }

  function showPayload(payload) {
    if (!payload) return;
    inspectorTitle.textContent = payload.title || payload.url;
    
    // Parse json strings if present for prettier viewing
    const prettyPayload = { ...payload };
    ['console_errors_json', 'network_errors_json', 'visual_rects_json', 'storage_state_json', 'iframes_json'].forEach(k => {
      if (prettyPayload[k] && typeof prettyPayload[k] === 'string') {
        try { prettyPayload[k.replace('_json', '')] = JSON.parse(prettyPayload[k]); delete prettyPayload[k]; } catch (e) {}
      }
    });

    inspectorContent.textContent = JSON.stringify(prettyPayload, null, 2);
    payloadInspector.classList.remove('hidden');
  }

  btnCloseInspector.addEventListener('click', () => {
    payloadInspector.classList.add('hidden');
    document.querySelectorAll('.queue-item').forEach(i => i.classList.remove('active'));
  });

  // Actions
  btnForceSync.addEventListener('click', () => {
    btnForceSync.textContent = 'Syncing...';
    btnForceSync.disabled = true;
    chrome.runtime.sendMessage({ type: 'force_sync' }, () => {
      setTimeout(() => {
        btnForceSync.textContent = 'Force Sync';
        btnForceSync.disabled = false;
        updateState();
      }, 500);
    });
  });

  btnClearQueue.addEventListener('click', () => {
    if (confirm('Clear the entire pending sync queue?')) {
       chrome.runtime.sendMessage({ type: 'clear_queue' }, updateState);
    }
  });

  btnResetSession.addEventListener('click', () => {
    if (confirm('Completely reset the WASM session and clear local storage state?')) {
      chrome.runtime.sendMessage({ type: 'reset_session' }, () => {
        window.location.reload();
      });
    }
  });

  btnEvalTab.addEventListener('click', () => {
    chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
      if (!tabs || tabs.length === 0) {
        alert('No active tab found.');
        return;
      }
      const activeTabId = tabs[0].id;
      btnEvalTab.textContent = 'Evaluating...';
      btnEvalTab.disabled = true;
      
      chrome.tabs.sendMessage(activeTabId, { type: 'dev_force_eval' }, (response) => {
        btnEvalTab.textContent = 'Evaluate Active Tab Now';
        btnEvalTab.disabled = false;
        
        if (chrome.runtime.lastError) {
          liveEvalContent.textContent = 'Error: ' + chrome.runtime.lastError.message + '\n\nMake sure the page is fully loaded and is not a chrome:// URL.';
          liveEvalResult.classList.remove('hidden');
          return;
        }

        liveEvalContent.textContent = JSON.stringify(response, null, 2);
        liveEvalResult.classList.remove('hidden');
      });
    });
  });

  // Logs
  btnClearLogs.addEventListener('click', () => {
    logContainer.innerHTML = '';
  });

  chrome.runtime.onMessage.addListener((msg) => {
    if (msg.type !== 'dev_log' || !msg.entry) return;
    const el = document.createElement('div');
    el.className = `log-entry log-${msg.entry.level}`;
    const time = new Date(msg.entry.t).toLocaleTimeString();
    const fields = Object.keys(msg.entry.fields || {}).length
      ? ' ' + JSON.stringify(msg.entry.fields)
      : '';
    el.textContent = `[${time}] [${msg.entry.level}] ${msg.entry.event}${fields}`;
    logContainer.appendChild(el);
    if (logContainer.scrollHeight - logContainer.scrollTop < logContainer.clientHeight + 100) {
      logContainer.scrollTop = logContainer.scrollHeight;
    }
  });

  // Diagnostics — on-demand snapshot of the SW's ring buffer.
  const btnDiagRefresh = document.getElementById('btn-diag-refresh');
  const diagList = document.getElementById('diag-list');
  const diagFilter = document.getElementById('diag-filter');
  const diagLevel = document.getElementById('diag-level');

  function renderDiagnostics(entries) {
    const q = diagFilter.value.trim();
    const lvl = diagLevel.value;
    const re = q ? new RegExp(q.replace(/\./g, '\\.').replace(/\*/g, '.*')) : null;
    diagList.innerHTML = '';
    for (const e of entries || []) {
      if (lvl && e.level !== lvl) continue;
      if (re && !re.test(e.event)) continue;
      const d = document.createElement('div');
      d.className = `log-entry log-${e.level}`;
      const fields = Object.keys(e.fields || {}).length ? ' ' + JSON.stringify(e.fields) : '';
      d.textContent = `${new Date(e.t).toISOString()} [${e.level}] ${e.event}${fields}`;
      diagList.appendChild(d);
    }
  }

  function loadDiagnostics() {
    chrome.runtime.sendMessage({ type: 'get_logs' }, (res) => {
      if (chrome.runtime.lastError || !res) {
        diagList.innerHTML = '<div class="empty-state">Unable to reach service worker.</div>';
        return;
      }
      renderDiagnostics(res.entries);
    });
  }

  if (btnDiagRefresh) btnDiagRefresh.addEventListener('click', loadDiagnostics);
  if (diagFilter) diagFilter.addEventListener('input', loadDiagnostics);
  if (diagLevel) diagLevel.addEventListener('change', loadDiagnostics);

  document.querySelector('[data-target="diagnostics"]').addEventListener('click', loadDiagnostics);

  // Init
  updateState();
  setInterval(updateState, 2000);

  function escapeHtml(str) {
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }
});