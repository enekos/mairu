// Floating "Ask Mairu" affordance shown on text selection.
// On click, opens a small composer near the selection where the user can
// type an extra prompt; submit hands {selection, prompt, url, title} to
// the service worker via chrome.runtime.sendMessage({type:'send_to_agent'}).

const HOST_TAG = 'mairu-selection-prompt';
const MIN_SELECTION_LEN = 3;
const MAX_PREVIEW_LEN = 280;

const STYLE = `
  :host { all: initial; position: absolute; z-index: 2147483647;
          font: 13px/1.4 -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
          color: #111827; }
  .pill {
    display: inline-flex; align-items: center; gap: 6px;
    background: #2563eb; color: #fff; border: none; border-radius: 999px;
    padding: 6px 12px; cursor: pointer; box-shadow: 0 4px 12px rgba(0,0,0,0.18);
    font-size: 12px; font-weight: 500;
  }
  .pill:hover { background: #1d4ed8; }
  .panel {
    width: 320px; background: #fff; border: 1px solid #e5e7eb; border-radius: 10px;
    box-shadow: 0 10px 30px rgba(0,0,0,0.18); overflow: hidden;
  }
  .panel header {
    display: flex; align-items: center; justify-content: space-between;
    padding: 8px 12px; border-bottom: 1px solid #f3f4f6;
    font-weight: 600; color: #2563eb; font-size: 12px;
  }
  .panel header button {
    background: none; border: none; cursor: pointer; color: #9ca3af; font-size: 16px;
    line-height: 1; padding: 0;
  }
  .panel .preview {
    font-size: 11px; color: #6b7280; padding: 6px 12px; max-height: 64px; overflow: hidden;
    border-bottom: 1px dashed #f3f4f6; background: #fafbfc; white-space: pre-wrap;
  }
  .panel textarea {
    width: 100%; min-height: 70px; max-height: 200px; resize: vertical;
    border: none; outline: none; padding: 10px 12px; font: inherit;
    font-size: 13px; color: #111827; box-sizing: border-box;
  }
  .panel .actions {
    display: flex; justify-content: space-between; align-items: center;
    padding: 8px 12px; border-top: 1px solid #f3f4f6; background: #fafbfc;
  }
  .panel .hint { font-size: 10px; color: #9ca3af; }
  .panel .btn {
    border: none; cursor: pointer; padding: 6px 12px; border-radius: 6px;
    font-size: 12px; font-weight: 500;
  }
  .panel .btn.primary { background: #2563eb; color: #fff; }
  .panel .btn.primary:hover { background: #1d4ed8; }
  .panel .btn.primary:disabled { background: #93c5fd; cursor: default; }
  .panel .btn.ghost { background: transparent; color: #6b7280; }
  .panel .btn.ghost:hover { color: #111827; }
  .panel .status { font-size: 11px; color: #6b7280; }
  .panel .status.err { color: #b91c1c; }
  .panel .status.ok  { color: #047857; }
`;

function define() {
  if (typeof customElements === 'undefined' || customElements === null) return;
  if (customElements.get(HOST_TAG)) return;
  class MairuSelectionPrompt extends HTMLElement {
    connectedCallback() {
      if (this._shadow) return;
      const root = this.attachShadow({ mode: 'closed' });
      root.innerHTML = `<style>${STYLE}</style><div class="container"></div>`;
      this._root = root;
      this._container = root.querySelector('.container');
      this._shadow = true;
    }
    setContent(node) {
      if (!this._container) return;
      this._container.replaceChildren(node);
    }
    clear() {
      if (this._container) this._container.replaceChildren();
    }
    _testShadow() { return this._root; }
  }
  customElements.define(HOST_TAG, MairuSelectionPrompt);
}

function clamp(n, min, max) { return Math.max(min, Math.min(max, n)); }

function positionNear(host, rect) {
  const scrollX = window.scrollX || window.pageXOffset || 0;
  const scrollY = window.scrollY || window.pageYOffset || 0;
  const desiredWidth = 340; // wider than panel for safety
  const top = scrollY + rect.bottom + 8;
  const left = clamp(
    scrollX + rect.left,
    scrollX + 8,
    scrollX + Math.max(8, (window.innerWidth || document.documentElement.clientWidth) - desiredWidth - 8),
  );
  host.style.top = `${Math.round(top)}px`;
  host.style.left = `${Math.round(left)}px`;
}

function getSelectionInfo() {
  const sel = window.getSelection ? window.getSelection() : null;
  if (!sel || sel.rangeCount === 0 || sel.isCollapsed) return null;
  const text = sel.toString();
  if (!text || text.trim().length < MIN_SELECTION_LEN) return null;
  let rect = null;
  try {
    rect = sel.getRangeAt(0).getBoundingClientRect();
  } catch (err) { void err; }
  if (!rect || (rect.width === 0 && rect.height === 0)) return null;
  return { text, rect };
}

export function installSelectionPrompt(doc = document, opts = {}) {
  define();
  const onSubmit = typeof opts.onSubmit === 'function' ? opts.onSubmit : async () => ({ ok: true });

  let host = doc.querySelector(HOST_TAG);
  if (!host) {
    host = doc.createElement(HOST_TAG);
    host.style.position = 'absolute';
    host.style.top = '-9999px';
    host.style.left = '-9999px';
    (doc.documentElement || doc.body).appendChild(host);
  }

  let lastSelection = null; // {text, rect}
  let mode = 'hidden';      // 'hidden' | 'pill' | 'panel'

  function hide() {
    mode = 'hidden';
    host.clear && host.clear();
    host.style.top = '-9999px';
    host.style.left = '-9999px';
  }

  function showPill(info) {
    mode = 'pill';
    lastSelection = info;
    const btn = doc.createElement('button');
    btn.className = 'pill';
    btn.type = 'button';
    btn.textContent = 'Ask Mairu';
    btn.addEventListener('mousedown', (e) => {
      // Prevent the click from collapsing the selection before we read it.
      e.preventDefault();
    });
    btn.addEventListener('click', () => showPanel(lastSelection));
    host.setContent(btn);
    positionNear(host, info.rect);
  }

  function showPanel(info) {
    mode = 'panel';
    lastSelection = info;

    const panel = doc.createElement('div');
    panel.className = 'panel';

    const header = doc.createElement('header');
    header.innerHTML = `<span>Ask Mairu about selection</span>`;
    const closeBtn = doc.createElement('button');
    closeBtn.type = 'button';
    closeBtn.setAttribute('aria-label', 'Close');
    closeBtn.textContent = '×';
    closeBtn.addEventListener('click', hide);
    header.appendChild(closeBtn);

    const preview = doc.createElement('div');
    preview.className = 'preview';
    preview.textContent = info.text.length > MAX_PREVIEW_LEN
      ? info.text.slice(0, MAX_PREVIEW_LEN) + '…'
      : info.text;

    const textarea = doc.createElement('textarea');
    textarea.placeholder = 'Type a prompt — e.g. "summarise", "explain in plain English", "save as a memory"…';
    textarea.setAttribute('aria-label', 'Prompt for Mairu agent');

    const actions = doc.createElement('div');
    actions.className = 'actions';
    const status = doc.createElement('span');
    status.className = 'status';
    status.textContent = '';
    const right = doc.createElement('div');
    right.style.display = 'flex';
    right.style.gap = '6px';
    const cancel = doc.createElement('button');
    cancel.className = 'btn ghost';
    cancel.type = 'button';
    cancel.textContent = 'Cancel';
    cancel.addEventListener('click', hide);
    const send = doc.createElement('button');
    send.className = 'btn primary';
    send.type = 'button';
    send.textContent = 'Send';

    async function submit() {
      const prompt = textarea.value.trim();
      send.disabled = true;
      status.classList.remove('err', 'ok');
      status.textContent = 'Sending…';
      try {
        const res = await onSubmit({
          text: info.text,
          prompt,
          url: location.href,
          title: doc.title || '',
        });
        if (res && res.ok) {
          status.classList.add('ok');
          status.textContent = 'Sent to Mairu';
          setTimeout(hide, 900);
        } else {
          status.classList.add('err');
          status.textContent = (res && res.error) || 'Failed to send';
          send.disabled = false;
        }
      } catch (err) {
        status.classList.add('err');
        status.textContent = String(err && err.message ? err.message : err);
        send.disabled = false;
      }
    }
    send.addEventListener('click', submit);
    textarea.addEventListener('keydown', (e) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
        e.preventDefault();
        submit();
      } else if (e.key === 'Escape') {
        e.preventDefault();
        hide();
      }
    });

    right.appendChild(cancel);
    right.appendChild(send);
    actions.appendChild(status);
    actions.appendChild(right);

    panel.appendChild(header);
    panel.appendChild(preview);
    panel.appendChild(textarea);
    panel.appendChild(actions);

    host.setContent(panel);
    positionNear(host, info.rect);

    // Defer focus so the click that opened the panel doesn't immediately steal it.
    setTimeout(() => textarea.focus(), 0);
  }

  function onSelectionMaybeChanged() {
    if (mode === 'panel') return; // user is composing — don't move the host
    const info = getSelectionInfo();
    if (!info) {
      if (mode === 'pill') hide();
      return;
    }
    showPill(info);
  }

  // Selection finalises on mouseup (text) or keyup (shift+arrow keys etc.).
  doc.addEventListener('mouseup', () => {
    // Wait a tick so getSelection() reflects the final selection.
    setTimeout(onSelectionMaybeChanged, 0);
  }, { passive: true });
  doc.addEventListener('keyup', (e) => {
    if (e.shiftKey || e.key === 'Shift' || e.key.startsWith('Arrow')) {
      setTimeout(onSelectionMaybeChanged, 0);
    }
  }, { passive: true });

  // Dismiss the affordance if the user clicks outside it.
  doc.addEventListener('mousedown', (e) => {
    if (mode === 'hidden') return;
    if (e.composedPath && e.composedPath().includes(host)) return;
    if (mode === 'pill') hide();
    // For panel: only close on explicit Cancel/×/Esc, not arbitrary clicks.
  }, { passive: true });

  return {
    hide,
    _host: host,
    _trigger: onSelectionMaybeChanged,
  };
}
