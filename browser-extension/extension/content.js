// mairu-ext/extension/content.js

// Capture page content and forward to service worker.
// Runs at document_idle — DOM is ready.

(function () {
  const MUTATION_DEBOUNCE_MS = 2000;
  const HTML_SIZE_LIMIT = 2 * 1024 * 1024; // 2 MB
  // Attribute changes that indicate meaningful content updates (not just style/animation)
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

  // Skip internal / non-web pages
  const scheme = location.protocol;
  if (!scheme.startsWith('http')) return;

  // 1. Inject script to trap console errors & network requests
  const script = document.createElement('script');
  script.textContent = `
    (function() {
      const originalError = console.error;
      console.error = function(...args) {
        window.postMessage({ type: '__MAIRU_ERROR', error: args.map(a => String(a)).join(' ') }, '*');
        return originalError.apply(this, args);
      };
      window.addEventListener('error', function(e) {
        window.postMessage({ type: '__MAIRU_ERROR', error: e.message + ' at ' + e.filename + ':' + e.lineno }, '*');
      });
      window.addEventListener('unhandledrejection', function(e) {
        window.postMessage({ type: '__MAIRU_ERROR', error: 'Unhandled Rejection: ' + e.reason }, '*');
      });

      const originalFetch = window.fetch;
      window.fetch = async function(...args) {
        try {
          const response = await originalFetch.apply(this, args);
          if (!response.ok) {
            window.postMessage({ type: '__MAIRU_NETWORK_ERROR', error: \`Fetch failed: \${response.status} \${response.statusText} for \${args[0]}\` }, '*');
          }
          return response;
        } catch (err) {
          window.postMessage({ type: '__MAIRU_NETWORK_ERROR', error: \`Fetch network error: \${err.message} for \${args[0]}\` }, '*');
          throw err;
        }
      };

      const originalXHR = window.XMLHttpRequest.prototype.open;
      window.XMLHttpRequest.prototype.open = function(method, url) {
        this.addEventListener('load', function() {
          if (this.status >= 400) {
             window.postMessage({ type: '__MAIRU_NETWORK_ERROR', error: \`XHR failed: \${this.status} \${this.statusText} for \${url}\` }, '*');
          }
        });
        this.addEventListener('error', function() {
           window.postMessage({ type: '__MAIRU_NETWORK_ERROR', error: \`XHR network error for \${url}\` }, '*');
        });
        return originalXHR.apply(this, arguments);
      };

      // History API interception for SPA navigation
      const patchHistory = (type) => {
        const original = history[type];
        return function(...args) {
          const result = original.apply(this, args);
          window.dispatchEvent(new Event('__mairu_spa_nav'));
          return result;
        };
      };
      history.pushState = patchHistory('pushState');
      history.replaceState = patchHistory('replaceState');
    })();
  `;
  document.documentElement.appendChild(script);
  script.remove();

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

  // 2. CSS visibility check
  function isVisible(el) {
    try {
      const style = window.getComputedStyle(el);
      return style.display !== 'none' &&
             style.visibility !== 'hidden' &&
             style.opacity !== '0';
    } catch {
      return true;
    }
  }

  // 3. Helper to get CSS selector for an element
  function getCssSelector(el) {
    if (!el || el.nodeType !== 1) return '';
    let path = [];
    while (el && el.nodeType === 1) {
      if (el.id) {
        path.unshift('#' + el.id);
        break;
      } else {
        let selector = el.localName;
        let sib = el, nth = 1;
        while ((sib = sib.previousElementSibling)) {
          if (sib.localName === el.localName) nth++;
        }
        if (nth !== 1) selector += ':nth-of-type(' + nth + ')';
        path.unshift(selector);
      }
      el = el.parentNode;
    }
    return path.join(' > ');
  }

  // 4. Iframe content extraction (same-origin only)
  function extractIframeContent() {
    const iframes = [];
    document.querySelectorAll('iframe').forEach(frame => {
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
          // Serialize the iframe body for extraction
          entry._html = doc.documentElement.outerHTML.slice(0, 50000);
        }
      } catch {
        // Cross-origin — skip content
      }
      iframes.push(entry);
    });
    return iframes;
  }

  // 5. Shadow DOM Serializer + Form value sync
  function getSerializedHtml() {
    function serializeNode(node) {
      if (node.nodeType === Node.TEXT_NODE) {
        const div = document.createElement('div');
        div.textContent = node.textContent;
        return div.innerHTML;
      }
      if (node.nodeType !== Node.ELEMENT_NODE) return '';

      const tag = node.localName;
      if (tag === 'script' || tag === 'style' || tag === 'svg' || tag === 'noscript') {
        return `<${tag}></${tag}>`;
      }

      // Skip invisible elements to reduce noise
      if (!isVisible(node)) {
        return `<${tag} data-mairu-hidden="true"></${tag}>`;
      }

      if (tag === 'input' || tag === 'textarea' || tag === 'select') {
        if (node.type === 'checkbox' || node.type === 'radio') {
          if (node.checked) node.setAttribute('checked', '');
          else node.removeAttribute('checked');
        } else if (node.value !== undefined) {
          node.setAttribute('value', node.value);
          if (tag === 'textarea') node.textContent = node.value;
        }
      }

      let html = '<' + tag;
      for (const attr of node.attributes) {
        html += ` ${attr.name}="${attr.value.replace(/"/g, '&quot;')}"`;
      }
      html += '>';

      if (node.shadowRoot) {
        html += '<template shadowrootmode="open">';
        html += Array.from(node.shadowRoot.childNodes).map(serializeNode).join('');
        html += '</template>';
      }

      html += Array.from(node.childNodes).map(serializeNode).join('');
      html += `</${tag}>`;
      return html;
    }

    const full = serializeNode(document.documentElement);
    return full.length > HTML_SIZE_LIMIT ? full.slice(0, HTML_SIZE_LIMIT) : full;
  }

  function captureVisualRects() {
    const visual_rects = {};
    document.querySelectorAll('header, nav, main, article, aside, footer, h1, h2, form, button, input').forEach(el => {
      const selector = getCssSelector(el);
      if (selector && !visual_rects[selector]) {
        const rect = el.getBoundingClientRect();
        if (rect.width > 0 && rect.height > 0) {
          visual_rects[selector] = `x:${Math.round(rect.x)},y:${Math.round(rect.y)},w:${Math.round(rect.width)},h:${Math.round(rect.height)}`;
        }
      }
    });
    return visual_rects;
  }

  function gatherPageState() {
    const html = getSerializedHtml();
    const selection = window.getSelection().toString();

    let activeEl = document.activeElement;
    while (activeEl && activeEl.shadowRoot && activeEl.shadowRoot.activeElement) {
      activeEl = activeEl.shadowRoot.activeElement;
    }
    const active_element = getCssSelector(activeEl);
    const visual_rects = captureVisualRects();

    return { html, selection, active_element, visual_rects };
  }

  function captureAndSend() {
    const { html, selection, active_element, visual_rects } = gatherPageState();

    const storage_state = {};
    try {
      for (let i = 0; i < localStorage.length; i++) {
        const key = localStorage.key(i);
        const val = localStorage.getItem(key);
        storage_state[`localStorage[${key}]`] = val.length > 200 ? val.substring(0, 200) + '...' : val;
      }
    } catch (e) {}
    try {
      for (let i = 0; i < sessionStorage.length; i++) {
        const key = sessionStorage.key(i);
        const val = sessionStorage.getItem(key);
        storage_state[`sessionStorage[${key}]`] = val.length > 200 ? val.substring(0, 200) + '...' : val;
      }
    } catch (e) {}

    const iframes = extractIframeContent();

    chrome.runtime.sendMessage({
      type: "page_content",
      payload: {
        url: location.href,
        html,
        timestamp: Date.now(),
        selection: selection || null,
        active_element: active_element || null,
        console_errors: consoleErrors,
        network_errors: networkErrors,
        visual_rects,
        storage_state,
        dwell_ms: Date.now() - dwellStart,
        interaction_count: interactionCount,
        iframes: iframes.map(f => ({
          src: f.src,
          title: f.title,
          is_same_origin: f.is_same_origin,
          html: f.is_same_origin ? f._html : undefined,
        })),
      },
    });
  }

  // Initial capture
  captureAndSend();

  // 6. Mutation observer — filtered to avoid noise from style/animation changes
  const observer = new MutationObserver((mutations) => {
    const meaningful = mutations.some(m => {
      if (m.type === 'childList') return m.addedNodes.length > 0 || m.removedNodes.length > 0;
      if (m.type === 'attributes') return MEANINGFUL_ATTRS.has(m.attributeName);
      return false;
    });
    if (!meaningful) return;
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(captureAndSend, MUTATION_DEBOUNCE_MS);
  });

  // 7. Interaction tracking
  const trackInteraction = () => { interactionCount++; };
  document.addEventListener('click', trackInteraction, { passive: true, capture: true });
  document.addEventListener('keydown', trackInteraction, { passive: true, capture: true });
  document.addEventListener('scroll', trackInteraction, { passive: true, capture: true });

  function captureStateForEval() {
    return gatherPageState();
  }

  // 8. Listen for execution commands from background script (from Agent)
  chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
    if (message.type === 'dev_force_eval') {
      sendResponse(captureStateForEval());
      return true;
    }
    if (message.type !== "execute") return;
    try {
      if (message.command === "click") {
        const el = document.querySelector(message.selector);
        if (el) {
          el.click();
          sendResponse({ success: true, message: `Clicked ${message.selector}` });
        } else {
          sendResponse({ error: `Element not found: ${message.selector}` });
        }
      } else if (message.command === "fill") {
        const el = document.querySelector(message.selector);
        if (el) {
          el.value = message.value;
          el.dispatchEvent(new Event('input', { bubbles: true }));
          el.dispatchEvent(new Event('change', { bubbles: true }));
          sendResponse({ success: true, message: `Filled ${message.selector}` });
        } else {
          sendResponse({ error: `Element not found: ${message.selector}` });
        }
      } else if (message.command === "highlight") {
        const el = document.querySelector(message.selector);
        if (el) {
          const origBorder = el.style.border;
          el.style.border = "3px solid red";
          setTimeout(() => { el.style.border = origBorder; }, 3000);
          el.scrollIntoView({ behavior: 'smooth', block: 'center' });
          sendResponse({ success: true, message: `Highlighted ${message.selector}` });
        } else {
          sendResponse({ error: `Element not found: ${message.selector}` });
        }
      } else if (message.command === "scroll") {
        const el = message.selector ? document.querySelector(message.selector) : null;
        if (el) {
          el.scrollIntoView({ behavior: 'smooth', block: 'center' });
          sendResponse({ success: true, message: `Scrolled to ${message.selector}` });
        } else if (message.direction) {
          const amount = message.amount || window.innerHeight * 0.8;
          if (message.direction === "down") window.scrollBy({ top: amount, behavior: 'smooth' });
          else if (message.direction === "up") window.scrollBy({ top: -amount, behavior: 'smooth' });
          else if (message.direction === "top") window.scrollTo({ top: 0, behavior: 'smooth' });
          else if (message.direction === "bottom") window.scrollTo({ top: document.body.scrollHeight, behavior: 'smooth' });
          sendResponse({ success: true, message: `Scrolled ${message.direction}` });
        } else {
          sendResponse({ error: "No element or direction specified" });
        }
      } else if (message.command === "navigate") {
        if (message.url) {
          window.location.href = message.url;
          sendResponse({ success: true, message: `Navigating to ${message.url}` });
        } else if (message.back) {
          window.history.back();
          sendResponse({ success: true, message: "Navigating back" });
        } else {
          sendResponse({ error: "No URL specified for navigation" });
        }
      } else if (message.command === "get_text") {
        const el = message.selector ? document.querySelector(message.selector) : document.body;
        sendResponse({ success: true, text: el ? el.innerText : '' });
      } else if (message.command === "select_text") {
        const el = document.querySelector(message.selector);
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
      } else if (message.command === "query") {
        const els = document.querySelectorAll(message.selector || '*');
        const results = Array.from(els).slice(0, 20).map(el => ({
          selector: getCssSelector(el),
          text: el.textContent.trim().slice(0, 100),
          tag: el.tagName.toLowerCase(),
        }));
        sendResponse({ success: true, results });
      } else if (message.command === "set_storage") {
        try {
          if (message.storage_type === "sessionStorage") {
            window.sessionStorage.setItem(message.key, message.value);
            sendResponse({ success: true, message: `Set sessionStorage[${message.key}]` });
          } else {
            window.localStorage.setItem(message.key, message.value);
            sendResponse({ success: true, message: `Set localStorage[${message.key}]` });
          }
        } catch (e) {
          sendResponse({ error: `Failed to set storage: ${e.message}` });
        }
      } else if (message.command === "show_thought") {
        let overlay = document.getElementById('__mairu_agent_overlay');
        if (!overlay) {
          overlay = document.createElement('div');
          overlay.id = '__mairu_agent_overlay';
          overlay.style.cssText = `
            position: fixed;
            bottom: 20px;
            right: 20px;
            max-width: 300px;
            background: #1e1e1e;
            color: #ffffff;
            border: 1px solid #333;
            border-radius: 8px;
            padding: 12px 16px;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
            font-size: 14px;
            line-height: 1.4;
            box-shadow: 0 4px 12px rgba(0,0,0,0.15);
            z-index: 2147483647;
            pointer-events: none;
            opacity: 0;
            transition: opacity 0.2s ease-in-out, transform 0.2s ease-in-out;
            transform: translateY(10px);
            display: flex;
            align-items: flex-start;
            gap: 10px;
          `;
          
          const icon = document.createElement('div');
          icon.innerHTML = '🤖';
          icon.style.fontSize = '18px';
          
          const text = document.createElement('div');
          text.id = '__mairu_agent_text';
          
          overlay.appendChild(icon);
          overlay.appendChild(text);
          document.body.appendChild(overlay);
          
          // Trigger reflow
          void overlay.offsetWidth;
        }
        
        const textEl = document.getElementById('__mairu_agent_text');
        if (textEl) textEl.textContent = message.text;
        
        overlay.style.opacity = '1';
        overlay.style.transform = 'translateY(0)';
        sendResponse({ success: true });
      } else if (message.command === "hide_thought") {
        const overlay = document.getElementById('__mairu_agent_overlay');
        if (overlay) {
          overlay.style.opacity = '0';
          overlay.style.transform = 'translateY(10px)';
        }
        
        // Remove old highlights
        document.querySelectorAll('.__mairu_agent_highlight').forEach(el => {
          el.classList.remove('__mairu_agent_highlight');
          el.style.outline = el.dataset.origOutline || '';
          el.style.outlineOffset = el.dataset.origOutlineOffset || '';
        });
        
        sendResponse({ success: true });
      } else if (message.command === "highlight_thought") {
        // Remove old highlights
        document.querySelectorAll('.__mairu_agent_highlight').forEach(el => {
          el.classList.remove('__mairu_agent_highlight');
          el.style.outline = el.dataset.origOutline || '';
          el.style.outlineOffset = el.dataset.origOutlineOffset || '';
        });
        
        const el = document.querySelector(message.selector);
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
    return true; // async response
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

  // SPA navigation via history API (injected above) and popstate
  const handleSpaNav = () => {
    dwellStart = Date.now();
    interactionCount = 0;
    setTimeout(captureAndSend, 500);
  };
  window.addEventListener('__mairu_spa_nav', handleSpaNav);
  window.addEventListener('popstate', handleSpaNav);

  // Cleanup on unload
  window.addEventListener("unload", () => {
    observer.disconnect();
    clearTimeout(debounceTimer);
    document.removeEventListener('click', trackInteraction, { capture: true });
    document.removeEventListener('keydown', trackInteraction, { capture: true });
    document.removeEventListener('scroll', trackInteraction, { capture: true });
  });
})();
