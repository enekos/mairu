// Runs in the page's MAIN world (injected via chrome.scripting).
// Must be CSP-safe: no inline evals, no new Function, no script-src violations.
(function () {
  if (window.__mairu_traps_installed) return;
  window.__mairu_traps_installed = true;

  const post = (type, error) => {
    try { window.postMessage({ type, error }, '*'); } catch (_err) { /* noop */ }
  };

  const origError = console.error;
  console.error = function (...args) {
    post('__MAIRU_ERROR', args.map((a) => String(a)).join(' '));
    return origError.apply(this, args);
  };
  window.addEventListener('error', (e) => {
    post('__MAIRU_ERROR', `${e.message} at ${e.filename}:${e.lineno}`);
  });
  window.addEventListener('unhandledrejection', (e) => {
    post('__MAIRU_ERROR', `Unhandled Rejection: ${e.reason}`);
  });

  const origFetch = window.fetch;
  if (origFetch) {
    window.fetch = async function (...args) {
      try {
        const r = await origFetch.apply(this, args);
        if (!r.ok) post('__MAIRU_NETWORK_ERROR', `Fetch ${r.status} ${r.statusText} ${args[0]}`);
        return r;
      } catch (err) {
        post('__MAIRU_NETWORK_ERROR', `Fetch error: ${err.message} ${args[0]}`);
        throw err;
      }
    };
  }

  if (window.XMLHttpRequest) {
    const origOpen = XMLHttpRequest.prototype.open;
    XMLHttpRequest.prototype.open = function (_m, url) {
      this.addEventListener('load', function () {
        if (this.status >= 400) post('__MAIRU_NETWORK_ERROR', `XHR ${this.status} ${this.statusText} ${url}`);
      });
      this.addEventListener('error', () => post('__MAIRU_NETWORK_ERROR', `XHR error ${url}`));
      return origOpen.apply(this, arguments);
    };
  }

  const patch = (t) => {
    const o = history[t];
    history[t] = function () {
      const r = o.apply(this, arguments);
      window.dispatchEvent(new Event('__mairu_spa_nav'));
      return r;
    };
  };
  patch('pushState');
  patch('replaceState');
})();
