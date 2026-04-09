# Mairu WASM Browser Extension — Design Spec

## Summary

A Rust/WASM Chrome extension that gives `mairu chat` real-time browser awareness. The extension extracts and indexes page content locally, maintains a session memory of recent browsing, and syncs relevant findings to mairu's context nodes in the background. The agent queries the extension via a `browser_context` tool.

## Goals

- Give `mairu chat` the ability to read and search the content of the active browser session
- Preprocess content locally in WASM to reduce API noise (dedup, relevance scoring)
- Maintain temporal session awareness (navigation history, recently seen content)
- Background-sync high-signal content to mairu's persistent context store

## Non-Goals (v1)

- Browser control (click, navigate, fill forms) — deferred to v2
- Embedded vector search in WASM — mairu server handles this
- Firefox/Safari support
- LLM calls in the extension
- Page screenshots or visual understanding
- Authentication/credential management

## Architecture

Three layers:

### 1. Content Script (JS)

Thin shim injected into every page. Responsibilities:
- Observe DOM load and mutations (MutationObserver)
- Detect SPA route changes (popstate, pushState interception)
- Capture raw HTML/text snapshots
- Forward to service worker via `chrome.runtime.sendMessage`

### 2. WASM Core (Rust)

Runs inside the extension service worker. Receives raw page content, produces structured output.

**Components:**

| Component | Responsibility |
|---|---|
| Extractor | HTML to structured `ContentSection` list. Strips nav/footer/ad noise. Uses `lol_html` for streaming HTML parsing. |
| Chunker | Splits extracted content into sized chunks suitable for mairu context nodes. |
| Deduplicator | SimHash-based content hashing. Skips pages/chunks already processed. |
| Session Store | In-memory rolling window (last 50 pages) with `chrome.storage.local` persistence. |
| Relevance Scorer | Lightweight TF-IDF to rank chunks before sync. Only high-signal content reaches mairu. |

### 3. Background Sync (JS, service worker)

Bridges WASM core and mairu API:
- Batches processed chunks and syncs to mairu as context nodes via REST API
- Flush trigger: every 10 seconds or when queue reaches 5 items
- Skips content that hash-matches already-synced nodes

### Communication

```
mairu chat  ──HTTP──>  native messaging host (Rust binary, 127.0.0.1:PORT)
                            |
                    chrome.runtime.connectNative
                            |
                     service worker (WASM core)
                            |
                     content script <──DOM──> browser tab
```

MV3 service workers cannot bind TCP listeners. A small native messaging host (~100 LOC Rust binary) bridges HTTP requests from `mairu chat` to the extension via Chrome's native messaging protocol.

## Data Model

### Page Snapshot

```rust
struct PageSnapshot {
    url: String,
    title: String,
    timestamp: u64,
    content_hash: u64,          // SimHash for dedup
    sections: Vec<ContentSection>,
    metadata: PageMetadata,      // og tags, lang, canonical URL
}

struct ContentSection {
    kind: SectionKind,           // Heading, Paragraph, CodeBlock, Table, List, Link
    text: String,
    depth: u8,                   // heading level / nesting depth
    selector: String,            // CSS selector path for re-location
}
```

### Browser Session

```rust
struct NavEvent {
    url: String,
    timestamp: u64,
    referrer: Option<String>,
}

struct BrowserSession {
    id: String,
    started_at: u64,
    pages: VecDeque<PageSnapshot>,  // rolling window, last 50 pages
    nav_history: Vec<NavEvent>,
}
```

### Mapping to Mairu Context Nodes

| mairu field | Source |
|---|---|
| `uri` | `contextfs://browser/{session_id}/{url_hash}` |
| `parent_uri` | `contextfs://browser/{session_id}` |
| `name` | Page title |
| `abstract` | First ~200 chars of main body + metadata summary |
| `overview` | Section outline (headings + code block labels) |
| `content` | Full extracted text, chunked |

## Agent Integration

New `browser_context` tool for `mairu chat`:

| Command | Returns |
|---|---|
| `browser_context current` | Current page snapshot (title, URL, extracted sections) |
| `browser_context history` | Recent navigation path with timestamps |
| `browser_context search "query"` | TF-IDF search across session store — ranked chunks from visited pages |
| `browser_context session` | Session summary — pages visited, time spent, topics covered |

The agent queries the extension in real-time via the native messaging host. Background sync to mairu context nodes happens independently — the agent doesn't wait for it.

## Project Structure

```
browser-extension/
├── crates/
│   ├── core/              # Pure Rust library — no WASM deps
│   │   ├── extractor.rs   # HTML -> ContentSection (lol_html)
│   │   ├── chunker.rs     # Text splitting with size/overlap control
│   │   ├── dedup.rs       # SimHash implementation
│   │   ├── scorer.rs      # TF-IDF relevance scoring
│   │   ├── session.rs     # BrowserSession + PageSnapshot state
│   │   └── types.rs       # Shared structs
│   └── wasm/              # Thin wasm-bindgen wrapper over core
│       └── lib.rs         # #[wasm_bindgen] exports
├── extension/
│   ├── manifest.json      # Chrome MV3 manifest
│   ├── content.js         # Content script — DOM observation
│   ├── service-worker.js  # Loads WASM, runs core, exposes messaging
│   └── popup/             # Minimal popup (status, session info, settings)
├── native-host/           # Native messaging host binary
│   └── main.rs            # HTTP server <-> chrome.runtime.connectNative bridge
└── Cargo.toml             # Workspace
```

Key decision: `core` crate is pure Rust with no WASM dependencies. This makes it testable with standard `cargo test` and reusable as a native binary.

## Key Libraries

| Crate | Purpose |
|---|---|
| `wasm-pack` + `wasm-bindgen` | WASM build and JS interop |
| `lol_html` | Streaming HTML parsing/rewriting (Cloudflare) |
| `serde` + `serde_json` | JS <-> WASM serialization |
| `web-sys` | Minimal DOM API access from WASM |

## Success Criteria

- `browser_context current` returns structured extraction of active tab within 200ms
- `browser_context search` finds relevant content from previously visited pages
- Background sync populates mairu context nodes without noticeable browser slowdown
- Handles docs sites (MDN, Go docs, GitHub), Stack Overflow, and general articles cleanly

## Future (v2)

- Browser control tools (`browser_click`, `browser_navigate`, `browser_fill`) — Approach B
- Firefox/Safari support
- Visual page understanding (screenshots + multimodal)
- Agent-agnostic interface (expose to any agent, not just mairu chat)
