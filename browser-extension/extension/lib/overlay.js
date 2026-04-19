const TAG = 'mairu-overlay';
const STYLE = `
  :host { all: initial; position: fixed; bottom: 20px; right: 20px;
          z-index: 2147483647; pointer-events: none;
          font: 14px/1.4 -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
  .card { max-width: 300px; background: #1e1e1e; color: #fff;
          border: 1px solid #333; border-radius: 8px; padding: 12px 16px;
          box-shadow: 0 4px 12px rgba(0,0,0,0.15);
          opacity: 0; transform: translateY(10px);
          transition: opacity .2s ease, transform .2s ease; }
  .card.visible { opacity: 1; transform: translateY(0); }
  .row { display: flex; align-items: flex-start; gap: 10px; }
  .icon { font-size: 18px; }
`;

function define() {
  if (typeof customElements === 'undefined' || customElements === null) return;
  if (customElements.get(TAG)) return;
  class MairuOverlay extends HTMLElement {
    connectedCallback() {
      if (this._shadow) return;
      const root = this.attachShadow({ mode: 'closed' });
      root.innerHTML = `<style>${STYLE}</style>
        <div class="card"><div class="row"><div class="icon">🤖</div><div class="text"></div></div></div>`;
      this._root = root;
      this._card = root.querySelector('.card');
      this._text = root.querySelector('.text');
      this._shadow = true;
    }
    show(t) {
      if (!this._text) return;
      this._text.textContent = t;
      this._card.classList.add('visible');
    }
    hide() {
      if (!this._card) return;
      this._card.classList.remove('visible');
    }
    // For tests — exposes a shadow node reference without breaking encapsulation in prod.
    _testShadow() {
      return this._root;
    }
  }
  customElements.define(TAG, MairuOverlay);
}

export function installOverlay(doc = document) {
  define();
  let el = doc.querySelector(TAG);
  if (!el) {
    el = doc.createElement(TAG);
    (doc.documentElement || doc.body).appendChild(el);
  }
  return {
    showThought: (t) => el.show && el.show(t),
    hideThought: () => el.hide && el.hide(),
    destroy: () => { el.remove(); },
    _element: el,
  };
}
