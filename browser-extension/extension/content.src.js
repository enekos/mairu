// mairu-ext/extension/content.src.js
// Source for content.js. Run scripts/build-content.mjs to regenerate content.js.
// Page-context traps (console/fetch/XHR/History) are injected via chrome.scripting
// from the service worker (see extension/main-world/traps.js) to satisfy strict CSP.

(function () {
  const MUTATION_DEBOUNCE_MS = 2000;
  const HTML_SIZE_LIMIT = 2 * 1024 * 1024;
  const CAPTURE_TIME_BUDGET_MS = 120;
  const VISUAL_RECT_CAP = 200;
  const MEANINGFUL_ATTRS = new Set([
    'src', 'href', 'value', 'checked', 'selected', 'disabled', 'hidden',
    'aria-expanded', 'aria-selected', 'aria-checked', 'aria-label',
    'aria-labelledby', 'aria-describedby', 'data-state', 'data-active',
  ]);

  let debounceTimer = null;
  let consoleErrors = [];
  let networkErrors = [];
  let dwellStart = Date.now();
  let interactionCount = 0;

  const scheme = location.protocol;
  if (!scheme.startsWith('http')) return;

  let overlay;
  try {
    overlay = installOverlay(document);
  } catch (err) {
    void err;
    overlay = { showThought: () => {}, hideThought: () => {} };
  }

  window.addEventListener('message', (event) => {
    if (!event.data) return;
    if (event.data.type === '__MAIRU_ERROR') {
      if (!consoleErrors.includes(event.data.error)) {
        consoleErrors.push(event.data.error);
        if (consoleErrors.length > 50) consoleErrors.shift();
      }
    } else if (event.data.type === '__MAIRU_NETWORK_ERROR') {
      if (!networkErrors.includes(event.data.error)) {
        networkErrors.push(event.data.error);
        if (networkErrors.length > 50) networkErrors.shift();
      }
    }
  });

  function extractIframeContent() {
    const iframes = [];
    document.querySelectorAll('iframe').forEach((frame) => {
      const entry = {
        src: frame.src || '',
        title: frame.title || frame.getAttribute('aria-label') || null,
        is_same_origin: false,
        sections: [],
      };
      try {
        const doc = frame.contentDocument;
        if (doc && doc.body) {
          entry.is_same_origin = true;
          entry._html = doc.documentElement.outerHTML.slice(0, 50000);
        }
      } catch (err) {
        void err;
      }
      iframes.push(entry);
    });
    return iframes;
  }

  function captureVisualRects() {
    const out = {};
    const visited = new Set();
    const structural = ['header', 'nav', 'main', 'article', 'aside', 'footer', 'h1', 'h2', 'form', 'button', 'input'];
    const add = (el) => {
      if (Object.keys(out).length >= VISUAL_RECT_CAP) return;
      const sel = getCssSelector(el);
      if (!sel || visited.has(sel)) return;
      visited.add(sel);
      const r = el.getBoundingClientRect();
      if (r.width > 0 && r.height > 0) {
        out[sel] = `x:${Math.round(r.x)},y:${Math.round(r.y)},w:${Math.round(r.width)},h:${Math.round(r.height)}`;
      }
    };
    for (const tag of structural) {
      if (Object.keys(out).length >= VISUAL_RECT_CAP) break;
      for (const el of document.querySelectorAll(tag)) {
        if (Object.keys(out).length >= VISUAL_RECT_CAP) break;
        add(el);
      }
    }
    return out;
  }

  function gatherPageState() {
    const { html, truncated } = serializeWithBudget(document.documentElement, {
      timeBudgetMs: CAPTURE_TIME_BUDGET_MS,
      sizeLimit: HTML_SIZE_LIMIT,
    });
    const selection = window.getSelection().toString();

    let activeEl = document.activeElement;
    while (activeEl && activeEl.shadowRoot && activeEl.shadowRoot.activeElement) {
      activeEl = activeEl.shadowRoot.activeElement;
    }
    const active_element = getCssSelector(activeEl);
    const visual_rects = captureVisualRects();

    return { html, truncated, selection, active_element, visual_rects };
  }

  function captureAndSend() {
    const { html, truncated, selection, active_element, visual_rects } = gatherPageState();

    const storage_state = {};
    try {
      for (let i = 0; i < localStorage.length; i++) {
        const key = localStorage.key(i);
        const val = localStorage.getItem(key);
        storage_state[`localStorage[${key}]`] = val.length > 200 ? val.substring(0, 200) + '...' : val;
      }
    } catch (err) { void err; }
    try {
      for (let i = 0; i < sessionStorage.length; i++) {
        const key = sessionStorage.key(i);
        const val = sessionStorage.getItem(key);
        storage_state[`sessionStorage[${key}]`] = val.length > 200 ? val.substring(0, 200) + '...' : val;
      }
    } catch (err) { void err; }

    const iframes = extractIframeContent();

    try {
      chrome.runtime.sendMessage({
        type: 'page_content',
        payload: {
          url: location.href,
          html,
          truncated,
          timestamp: Date.now(),
          selection: selection || null,
          active_element: active_element || null,
          console_errors: consoleErrors,
          network_errors: networkErrors,
          visual_rects,
          storage_state,
          dwell_ms: Date.now() - dwellStart,
          interaction_count: interactionCount,
          iframes: iframes.map((f) => ({
            src: f.src,
            title: f.title,
            is_same_origin: f.is_same_origin,
            html: f.is_same_origin ? f._html : undefined,
          })),
        },
      }, () => {
        // Swallow any sendResponse channel errors — the SW side may not respond and that's fine.
        void chrome.runtime.lastError;
      });
    } catch (err) {
      // SW may be restarting; next capture will retry.
      void err;
    }
  }

  captureAndSend();

  const observer = new MutationObserver((mutations) => {
    const meaningful = mutations.some((m) => {
      if (m.type === 'childList') return m.addedNodes.length > 0 || m.removedNodes.length > 0;
      if (m.type === 'attributes') return MEANINGFUL_ATTRS.has(m.attributeName);
      return false;
    });
    if (!meaningful) return;
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(captureAndSend, MUTATION_DEBOUNCE_MS);
  });

  const trackInteraction = () => { interactionCount++; };
  document.addEventListener('click', trackInteraction, { passive: true, capture: true });
  document.addEventListener('keydown', trackInteraction, { passive: true, capture: true });
  document.addEventListener('scroll', trackInteraction, { passive: true, capture: true });

  function captureStateForEval() {
    return gatherPageState();
  }

  chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
    if (message.type === 'dev_force_eval') {
      sendResponse(captureStateForEval());
      return true;
    }
    if (message.type !== 'execute') return;
    try {
      if (message.command === 'click') {
        const el = querySelectorDeep(document, message.selector);
        if (el) {
          el.click();
          sendResponse({ success: true, message: `Clicked ${message.selector}` });
        } else {
          sendResponse({ error: `Element not found: ${message.selector}` });
        }
      } else if (message.command === 'fill') {
        const el = querySelectorDeep(document, message.selector);
        if (el) {
          el.value = message.value;
          el.dispatchEvent(new Event('input', { bubbles: true }));
          el.dispatchEvent(new Event('change', { bubbles: true }));
          sendResponse({ success: true, message: `Filled ${message.selector}` });
        } else {
          sendResponse({ error: `Element not found: ${message.selector}` });
        }
      } else if (message.command === 'highlight') {
        const el = querySelectorDeep(document, message.selector);
        if (el) {
          const origBorder = el.style.border;
          el.style.border = '3px solid red';
          setTimeout(() => { el.style.border = origBorder; }, 3000);
          el.scrollIntoView({ behavior: 'smooth', block: 'center' });
          sendResponse({ success: true, message: `Highlighted ${message.selector}` });
        } else {
          sendResponse({ error: `Element not found: ${message.selector}` });
        }
      } else if (message.command === 'scroll') {
        const el = message.selector ? querySelectorDeep(document, message.selector) : null;
        if (el) {
          el.scrollIntoView({ behavior: 'smooth', block: 'center' });
          sendResponse({ success: true, message: `Scrolled to ${message.selector}` });
        } else if (message.direction) {
          const amount = message.amount || window.innerHeight * 0.8;
          if (message.direction === 'down') window.scrollBy({ top: amount, behavior: 'smooth' });
          else if (message.direction === 'up') window.scrollBy({ top: -amount, behavior: 'smooth' });
          else if (message.direction === 'top') window.scrollTo({ top: 0, behavior: 'smooth' });
          else if (message.direction === 'bottom') window.scrollTo({ top: document.body.scrollHeight, behavior: 'smooth' });
          sendResponse({ success: true, message: `Scrolled ${message.direction}` });
        } else {
          sendResponse({ error: 'No element or direction specified' });
        }
      } else if (message.command === 'navigate') {
        if (message.url) {
          window.location.href = message.url;
          sendResponse({ success: true, message: `Navigating to ${message.url}` });
        } else if (message.back) {
          window.history.back();
          sendResponse({ success: true, message: 'Navigating back' });
        } else {
          sendResponse({ error: 'No URL specified for navigation' });
        }
      } else if (message.command === 'get_text') {
        const el = message.selector ? querySelectorDeep(document, message.selector) : document.body;
        sendResponse({ success: true, text: el ? el.innerText : '' });
      } else if (message.command === 'select_text') {
        const el = querySelectorDeep(document, message.selector);
        if (el) {
          const range = document.createRange();
          range.selectNodeContents(el);
          const sel = window.getSelection();
          sel.removeAllRanges();
          sel.addRange(range);
          sendResponse({ success: true, message: `Selected text in ${message.selector}` });
        } else {
          sendResponse({ error: `Element not found: ${message.selector}` });
        }
      } else if (message.command === 'query') {
        const els = querySelectorAllDeep(document, message.selector || '*', 20);
        const results = els.map((el) => ({
          selector: getCssSelector(el),
          text: el.textContent.trim().slice(0, 100),
          tag: el.tagName.toLowerCase(),
        }));
        sendResponse({ success: true, results });
      } else if (message.command === 'set_storage') {
        try {
          if (message.storage_type === 'sessionStorage') {
            window.sessionStorage.setItem(message.key, message.value);
            sendResponse({ success: true, message: `Set sessionStorage[${message.key}]` });
          } else {
            window.localStorage.setItem(message.key, message.value);
            sendResponse({ success: true, message: `Set localStorage[${message.key}]` });
          }
        } catch (e) {
          sendResponse({ error: `Failed to set storage: ${e.message}` });
        }
      } else if (message.command === 'show_thought') {
        overlay.showThought(message.text);
        sendResponse({ success: true });
      } else if (message.command === 'hide_thought') {
        overlay.hideThought();
        document.querySelectorAll('.__mairu_agent_highlight').forEach((el) => {
          el.classList.remove('__mairu_agent_highlight');
          el.style.outline = el.dataset.origOutline || '';
          el.style.outlineOffset = el.dataset.origOutlineOffset || '';
        });
        sendResponse({ success: true });
      } else if (message.command === 'highlight_thought') {
        document.querySelectorAll('.__mairu_agent_highlight').forEach((el) => {
          el.classList.remove('__mairu_agent_highlight');
          el.style.outline = el.dataset.origOutline || '';
          el.style.outlineOffset = el.dataset.origOutlineOffset || '';
        });
        const el = querySelectorDeep(document, message.selector);
        if (el) {
          el.classList.add('__mairu_agent_highlight');
          el.dataset.origOutline = el.style.outline;
          el.dataset.origOutlineOffset = el.style.outlineOffset;
          el.style.outline = '3px dashed #4facfe';
          el.style.outlineOffset = '2px';
          if (message.scroll !== false) {
            el.scrollIntoView({ behavior: 'smooth', block: 'center' });
          }
          sendResponse({ success: true, message: `Highlighted ${message.selector} for thought` });
        } else {
          sendResponse({ error: `Element not found: ${message.selector}` });
        }
      } else {
        sendResponse({ error: `Unknown execute command: ${message.command}` });
      }
    } catch (e) {
      sendResponse({ error: `Execution failed: ${e.message}` });
    }
    return true;
  });

  observer.observe(document.body, {
    childList: true,
    subtree: true,
    attributes: true,
    attributeFilter: Array.from(MEANINGFUL_ATTRS),
  });

  document.addEventListener('selectionchange', () => {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(captureAndSend, 1000);
  });

  document.addEventListener('focusin', () => {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(captureAndSend, 1000);
  });

  const handleSpaNav = async () => {
    dwellStart = Date.now();
    interactionCount = 0;
    await waitForSpaHydration(document, { quietMs: 500, hardCapMs: 3000 });
    captureAndSend();
  };
  window.addEventListener('__mairu_spa_nav', handleSpaNav);
  window.addEventListener('popstate', handleSpaNav);

  window.addEventListener('unload', () => {
    observer.disconnect();
    clearTimeout(debounceTimer);
    document.removeEventListener('click', trackInteraction, { capture: true });
    document.removeEventListener('keydown', trackInteraction, { capture: true });
    document.removeEventListener('scroll', trackInteraction, { capture: true });
  });
})();
