document.addEventListener('DOMContentLoaded', () => {
  const sessionIdEl = document.getElementById('session-id');
  const pageCountEl = document.getElementById('page-count');
  const pendingCountEl = document.getElementById('pending-count');
  const nativeStatusEl = document.getElementById('native-status');
  const statusDot = document.getElementById('status-dot');
  const syncBtn = document.getElementById('sync-btn');

  function updateStats() {
    chrome.runtime.sendMessage({ type: "get_status" }, (response) => {
      if (chrome.runtime.lastError || !response) {
        statusDot.className = "status-indicator status-error";
        return;
      }
      
      statusDot.className = "status-indicator status-ok";
      
      if (response.sessionId) {
        sessionIdEl.textContent = response.sessionId.split('-')[1] || response.sessionId;
      }
      
      if (response.summary) {
        pageCountEl.textContent = response.summary.total_pages || 0;
      }
      
      if (response.pendingCount !== undefined) {
        pendingCountEl.textContent = response.pendingCount;
      }
      
      nativeStatusEl.textContent = response.nativeHostConnected ? "Connected" : "Disconnected";
      nativeStatusEl.style.color = response.nativeHostConnected ? "#10b981" : "#ef4444";
    });
  }

  updateStats();
  setInterval(updateStats, 2000);

  syncBtn.addEventListener('click', () => {
    syncBtn.textContent = "Syncing...";
    syncBtn.disabled = true;
    chrome.runtime.sendMessage({ type: "force_sync" }, () => {
      setTimeout(() => {
        syncBtn.textContent = "Force Sync";
        syncBtn.disabled = false;
        updateStats();
      }, 1000);
    });
  });
});
