// mairu-ext/extension/content.js

// Capture page content and forward to service worker.
// Runs at document_idle — DOM is ready.

(function () {
  const MUTATION_DEBOUNCE_MS = 2000;
  let debounceTimer = null;

  function captureAndSend() {
    const html = document.documentElement.outerHTML;
    chrome.runtime.sendMessage({
      type: "page_content",
      payload: {
        url: location.href,
        html: html,
        timestamp: Date.now(),
      },
    });
  }

  // Initial capture
  captureAndSend();

  // Watch for significant DOM mutations (SPA navigation, dynamic content)
  const observer = new MutationObserver(() => {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(captureAndSend, MUTATION_DEBOUNCE_MS);
  });

  observer.observe(document.body, {
    childList: true,
    subtree: true,
  });

  // Detect SPA route changes
  let lastUrl = location.href;
  const urlCheck = setInterval(() => {
    if (location.href !== lastUrl) {
      lastUrl = location.href;
      // Small delay for new content to render
      setTimeout(captureAndSend, 500);
    }
  }, 1000);

  // Cleanup on unload
  window.addEventListener("unload", () => {
    observer.disconnect();
    clearInterval(urlCheck);
    clearTimeout(debounceTimer);
  });
})();
