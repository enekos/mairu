// mairu-ext/extension/content.js

// Capture page content and forward to service worker.
// Runs at document_idle — DOM is ready.

(function () {
  const MUTATION_DEBOUNCE_MS = 2000;
  let debounceTimer = null;
  let consoleErrors = [];
  let networkErrors = [];

  // 1. Inject script to trap console errors & network requests
  const script = document.createElement('script');
  script.textContent = `
    (function() {
      // Console Errors
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

      // Fetch interception
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

      // XHR interception
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
    })();
  `;
  document.documentElement.appendChild(script);
  script.remove();

  window.addEventListener('message', (event) => {
    if (!event.data) return;
    if (event.data.type === '__MAIRU_ERROR') {
      consoleErrors.push(event.data.error);
      if (consoleErrors.length > 50) consoleErrors.shift(); // keep last 50
    } else if (event.data.type === '__MAIRU_NETWORK_ERROR') {
      networkErrors.push(event.data.error);
      if (networkErrors.length > 50) networkErrors.shift(); // keep last 50
    }
  });

  // 2. Helper to get CSS selector for an element
  function getCssSelector(el) {
    if (!el || el.nodeType !== 1) return '';
    let path = [];
    while (el && el.nodeType === 1) {
      let selector = el.localName;
      if (el.id) {
        selector += '#' + el.id;
        path.unshift(selector);
        break;
      } else {
        let sib = el, nth = 1;
        while (sib = sib.previousElementSibling) {
          if (sib.localName === el.localName) nth++;
        }
        if (nth !== 1) selector += ":nth-of-type(" + nth + ")";
      }
      path.unshift(selector);
      el = el.parentNode;
    }
    return path.join(' > ');
  }

  // 3. Shadow DOM Serializer + Form value sync
  function getSerializedHtml() {
    function serializeNode(node) {
      if (node.nodeType === Node.TEXT_NODE) {
        // Escape HTML for text nodes by using a dummy div
        const div = document.createElement('div');
        div.textContent = node.textContent;
        return div.innerHTML;
      }
      if (node.nodeType !== Node.ELEMENT_NODE) return '';

      const tag = node.localName;
      
      // Inline sync for inputs, textareas, selects (handles shadow DOM automatically)
      if (tag === 'input' || tag === 'textarea' || tag === 'select') {
        if (node.type === 'checkbox' || node.type === 'radio') {
          if (node.checked) node.setAttribute('checked', '');
          else node.removeAttribute('checked');
        } else if (node.value !== undefined) {
          node.setAttribute('value', node.value);
          if (tag === 'textarea') {
              node.textContent = node.value;
          }
        }
      }

      let html = '<' + tag;
      for (const attr of node.attributes) {
        html += ` ${attr.name}="${attr.value.replace(/"/g, '&quot;')}"`;
      }
      html += '>';

      // Inject declarative shadow DOM if exists
      if (node.shadowRoot) {
        html += '<template shadowrootmode="open">';
        html += Array.from(node.shadowRoot.childNodes).map(serializeNode).join('');
        html += '</template>';
      }

      html += Array.from(node.childNodes).map(serializeNode).join('');
      html += `</${tag}>`;
      return html;
    }

    return serializeNode(document.documentElement);
  }

  function captureAndSend() {
    const html = getSerializedHtml();
    const selection = window.getSelection().toString();
    
    // Find real active element (piercing shadow dom)
    let activeEl = document.activeElement;
    while (activeEl && activeEl.shadowRoot && activeEl.shadowRoot.activeElement) {
        activeEl = activeEl.shadowRoot.activeElement;
    }
    const active_element = getCssSelector(activeEl);

    // Capture visual layout of primary structural elements
    const visual_rects = {};
    const primaryElements = document.querySelectorAll('header, nav, main, article, aside, footer, h1, h2, form, button, input');
    primaryElements.forEach(el => {
       const selector = getCssSelector(el);
       if (selector && !visual_rects[selector]) {
         const rect = el.getBoundingClientRect();
         // Only capture visible elements
         if (rect.width > 0 && rect.height > 0) {
            visual_rects[selector] = \`x:\${Math.round(rect.x)},y:\${Math.round(rect.y)},w:\${Math.round(rect.width)},h:\${Math.round(rect.height)}\`;
         }
       }
    });

    // Capture storage state securely
    const storage_state = {};
    try {
        for (let i = 0; i < localStorage.length; i++) {
            const key = localStorage.key(i);
            const val = localStorage.getItem(key);
            storage_state[\`localStorage[\${key}]\`] = val.length > 200 ? val.substring(0, 200) + '...' : val;
        }
    } catch(e) {}
    try {
        for (let i = 0; i < sessionStorage.length; i++) {
            const key = sessionStorage.key(i);
            const val = sessionStorage.getItem(key);
            storage_state[\`sessionStorage[\${key}]\`] = val.length > 200 ? val.substring(0, 200) + '...' : val;
        }
    } catch(e) {}

    chrome.runtime.sendMessage({
      type: "page_content",
      payload: {
        url: location.href,
        html: html,
        timestamp: Date.now(),
        selection: selection || null,
        active_element: active_element || null,
        console_errors: consoleErrors,
        network_errors: networkErrors,
        visual_rects: visual_rects,
        storage_state: storage_state,
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

  // Listen for execution commands from background script (from Agent)
  chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
    if (message.type === "execute") {
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
          const el = document.querySelector(message.selector);
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
             sendResponse({ error: `Element not found or no direction specified` });
          }
        } else if (message.command === "navigate") {
          if (message.url) {
             window.location.href = message.url;
             sendResponse({ success: true, message: `Navigating to ${message.url}` });
          } else if (message.back) {
             window.history.back();
             sendResponse({ success: true, message: `Navigating back` });
          } else {
             sendResponse({ error: `No URL specified for navigation` });
          }
        } else {
          sendResponse({ error: `Unknown execute command: ${message.command}` });
        }
      } catch (e) {
         sendResponse({ error: `Execution failed: ${e.message}` });
      }
      return true; // Indicates async response
    }
  });

  observer.observe(document.body, {
    childList: true,
    subtree: true,
  });

  // Also listen for selection changes and focus changes
  document.addEventListener('selectionchange', () => {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(captureAndSend, 1000);
  });
  
  document.addEventListener('focusin', () => {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(captureAndSend, 1000);
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
