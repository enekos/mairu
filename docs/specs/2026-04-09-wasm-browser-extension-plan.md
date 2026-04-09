# Mairu WASM Browser Extension — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Rust/WASM Chrome extension that gives `mairu chat` real-time browser awareness — extracting page content, maintaining session memory, and syncing to mairu's context store.

**Architecture:** Rust workspace with a pure `core` crate (no WASM deps, testable with `cargo test`) and a thin `wasm` crate wrapping it via `wasm-bindgen`. A Chrome MV3 extension loads the WASM in its service worker, with a JS content script forwarding DOM snapshots. A native messaging host (small Rust binary) bridges HTTP from `mairu chat` to the extension.

**Tech Stack:** Rust, wasm-pack, wasm-bindgen, lol_html, serde, Chrome MV3 APIs, Go (mairu tool integration)

---

## File Structure

```
browser-extension/
├── Cargo.toml                        # Workspace root
├── crates/
│   ├── core/
│   │   ├── Cargo.toml
│   │   └── src/
│   │       ├── lib.rs                # Re-exports all modules
│   │       ├── types.rs              # PageSnapshot, ContentSection, BrowserSession, etc.
│   │       ├── extractor.rs          # HTML → structured ContentSection list (lol_html)
│   │       ├── chunker.rs            # Split text into sized chunks
│   │       ├── dedup.rs              # SimHash content fingerprinting
│   │       ├── scorer.rs             # TF-IDF relevance scoring across session
│   │       └── session.rs            # BrowserSession state management
│   └── wasm/
│       ├── Cargo.toml
│       └── src/
│           └── lib.rs                # wasm_bindgen exports wrapping core
├── native-host/
│   ├── Cargo.toml
│   └── src/
│       └── main.rs                   # HTTP ↔ native messaging bridge
├── extension/
│   ├── manifest.json                 # Chrome MV3 manifest
│   ├── content.js                    # DOM observer, forwards HTML to service worker
│   ├── service-worker.js             # Loads WASM, processes pages, handles native messaging
│   └── popup/
│       ├── popup.html                # Minimal status popup
│       └── popup.js                  # Session stats display
└── install.sh                        # Registers native messaging host with Chrome
```

**Mairu integration (existing codebase):**
- Modify: `mairu/internal/llm/tools.go` — add `browser_context` tool declaration
- Modify: `mairu/internal/agent/agent.go` — add `browser_context` case to `executeToolCall`
- Create: `mairu/internal/agent/browser.go` — HTTP client to query the native messaging host

---

### Task 1: Workspace Scaffolding

**Files:**
- Create: `browser-extension/Cargo.toml`
- Create: `browser-extension/crates/core/Cargo.toml`
- Create: `browser-extension/crates/core/src/lib.rs`
- Create: `browser-extension/crates/wasm/Cargo.toml`
- Create: `browser-extension/crates/wasm/src/lib.rs`
- Create: `browser-extension/native-host/Cargo.toml`
- Create: `browser-extension/native-host/src/main.rs`

- [ ] **Step 1: Create workspace Cargo.toml**

```toml
# browser-extension/Cargo.toml
[workspace]
resolver = "2"
members = [
    "crates/core",
    "crates/wasm",
    "native-host",
]
```

- [ ] **Step 2: Create core crate**

```toml
# browser-extension/crates/core/Cargo.toml
[package]
name = "browser-extension-core"
version = "0.1.0"
edition = "2024"

[dependencies]
lol_html = "2"
serde = { version = "1", features = ["derive"] }
serde_json = "1"
```

```rust
// browser-extension/crates/core/src/lib.rs
pub mod types;
pub mod extractor;
pub mod chunker;
pub mod dedup;
pub mod scorer;
pub mod session;
```

- [ ] **Step 3: Create wasm crate**

```toml
# browser-extension/crates/wasm/Cargo.toml
[package]
name = "browser-extension-wasm"
version = "0.1.0"
edition = "2024"

[lib]
crate-type = ["cdylib", "rlib"]

[dependencies]
browser-extension-core = { path = "../core" }
wasm-bindgen = "0.2"
serde = { version = "1", features = ["derive"] }
serde_json = "1"
serde-wasm-bindgen = "0.6"

[dependencies.web-sys]
version = "0.3"
features = []
```

```rust
// browser-extension/crates/wasm/src/lib.rs
use wasm_bindgen::prelude::*;

#[wasm_bindgen]
pub fn ping() -> String {
    "browser-extension ready".to_string()
}
```

- [ ] **Step 4: Create native-host crate stub**

```toml
# browser-extension/native-host/Cargo.toml
[package]
name = "browser-extension-host"
version = "0.1.0"
edition = "2024"

[dependencies]
tiny_http = "0.12"
serde = { version = "1", features = ["derive"] }
serde_json = "1"
```

```rust
// browser-extension/native-host/src/main.rs
fn main() {
    println!("browser-extension native host starting");
}
```

- [ ] **Step 5: Verify workspace builds**

Run: `cd browser-extension && cargo build`
Expected: Compiles with no errors.

- [ ] **Step 6: Verify WASM builds**

Run: `cd browser-extension && wasm-pack build crates/wasm --target web`
Expected: `crates/wasm/pkg/` directory created with `.wasm` and `.js` files.

- [ ] **Step 7: Commit**

```bash
git add browser-extension/
git commit -m "feat(ext): scaffold Rust workspace with core, wasm, and native-host crates"
```

---

### Task 2: Core Types

**Files:**
- Create: `browser-extension/crates/core/src/types.rs`
- Test: inline `#[cfg(test)]` module

- [ ] **Step 1: Write the types with serialization tests**

```rust
// browser-extension/crates/core/src/types.rs
use serde::{Deserialize, Serialize};
use std::collections::VecDeque;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum SectionKind {
    Heading,
    Paragraph,
    CodeBlock,
    Table,
    List,
    Link,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ContentSection {
    pub kind: SectionKind,
    pub text: String,
    pub depth: u8,
    pub selector: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct PageMetadata {
    pub og_title: Option<String>,
    pub og_description: Option<String>,
    pub lang: Option<String>,
    pub canonical_url: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PageSnapshot {
    pub url: String,
    pub title: String,
    pub timestamp: u64,
    pub content_hash: u64,
    pub sections: Vec<ContentSection>,
    pub metadata: PageMetadata,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NavEvent {
    pub url: String,
    pub timestamp: u64,
    pub referrer: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BrowserSession {
    pub id: String,
    pub started_at: u64,
    pub pages: VecDeque<PageSnapshot>,
    pub nav_history: Vec<NavEvent>,
}

impl BrowserSession {
    pub fn new(id: String, started_at: u64) -> Self {
        Self {
            id,
            started_at,
            pages: VecDeque::new(),
            nav_history: Vec::new(),
        }
    }
}

/// Response format for the browser_context tool
#[derive(Debug, Serialize, Deserialize)]
#[serde(tag = "type", content = "data")]
#[serde(rename_all = "snake_case")]
pub enum BrowserResponse {
    Current(PageSnapshot),
    History(Vec<NavEvent>),
    Search(Vec<SearchResult>),
    Session(SessionSummary),
    Error(String),
}

#[derive(Debug, Serialize, Deserialize)]
pub struct SearchResult {
    pub url: String,
    pub title: String,
    pub snippet: String,
    pub score: f64,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct SessionSummary {
    pub id: String,
    pub started_at: u64,
    pub page_count: usize,
    pub urls: Vec<String>,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_section_kind_serializes_snake_case() {
        let kind = SectionKind::CodeBlock;
        let json = serde_json::to_string(&kind).unwrap();
        assert_eq!(json, r#""code_block""#);
    }

    #[test]
    fn test_page_snapshot_roundtrip() {
        let snap = PageSnapshot {
            url: "https://example.com".to_string(),
            title: "Test".to_string(),
            timestamp: 1000,
            content_hash: 42,
            sections: vec![ContentSection {
                kind: SectionKind::Heading,
                text: "Hello".to_string(),
                depth: 1,
                selector: "h1".to_string(),
            }],
            metadata: PageMetadata::default(),
        };
        let json = serde_json::to_string(&snap).unwrap();
        let back: PageSnapshot = serde_json::from_str(&json).unwrap();
        assert_eq!(back.url, "https://example.com");
        assert_eq!(back.sections.len(), 1);
    }

    #[test]
    fn test_browser_session_new() {
        let session = BrowserSession::new("sess-1".to_string(), 1000);
        assert_eq!(session.id, "sess-1");
        assert!(session.pages.is_empty());
        assert!(session.nav_history.is_empty());
    }

    #[test]
    fn test_browser_response_tagged_enum() {
        let resp = BrowserResponse::Error("not found".to_string());
        let json = serde_json::to_string(&resp).unwrap();
        assert!(json.contains(r#""type":"error""#));
        assert!(json.contains("not found"));
    }
}
```

- [ ] **Step 2: Run tests**

Run: `cd browser-extension && cargo test -p browser-extension-core`
Expected: 4 tests pass.

- [ ] **Step 3: Commit**

```bash
git add browser-extension/crates/core/src/types.rs
git commit -m "feat(ext): add core types — PageSnapshot, BrowserSession, ContentSection"
```

---

### Task 3: SimHash Deduplicator

**Files:**
- Create: `browser-extension/crates/core/src/dedup.rs`

- [ ] **Step 1: Write failing tests**

```rust
// browser-extension/crates/core/src/dedup.rs

/// 64-bit SimHash for content fingerprinting.
/// Near-duplicate detection via Hamming distance.
pub fn simhash(text: &str) -> u64 {
    todo!()
}

/// Hamming distance between two SimHash values.
pub fn hamming_distance(a: u64, b: u64) -> u32 {
    (a ^ b).count_ones()
}

/// Returns true if two hashes are near-duplicates (hamming distance <= threshold).
pub fn is_near_duplicate(a: u64, b: u64, threshold: u32) -> bool {
    hamming_distance(a, b) <= threshold
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_identical_text_same_hash() {
        let h1 = simhash("the quick brown fox jumps over the lazy dog");
        let h2 = simhash("the quick brown fox jumps over the lazy dog");
        assert_eq!(h1, h2);
    }

    #[test]
    fn test_similar_text_low_hamming() {
        let h1 = simhash("the quick brown fox jumps over the lazy dog");
        let h2 = simhash("the quick brown fox leaps over the lazy dog");
        assert!(hamming_distance(h1, h2) < 10, "similar texts should have low hamming distance");
    }

    #[test]
    fn test_different_text_high_hamming() {
        let h1 = simhash("the quick brown fox jumps over the lazy dog");
        let h2 = simhash("rust programming language systems memory safety");
        assert!(hamming_distance(h1, h2) > 10, "different texts should have high hamming distance");
    }

    #[test]
    fn test_hamming_distance_identical() {
        assert_eq!(hamming_distance(0, 0), 0);
    }

    #[test]
    fn test_hamming_distance_all_different() {
        assert_eq!(hamming_distance(0, u64::MAX), 64);
    }

    #[test]
    fn test_near_duplicate_threshold() {
        let h1 = simhash("hello world foo bar");
        assert!(is_near_duplicate(h1, h1, 3));
    }

    #[test]
    fn test_empty_string() {
        let h = simhash("");
        assert_eq!(h, 0);
    }
}
```

- [ ] **Step 2: Run tests to confirm failure**

Run: `cd browser-extension && cargo test -p browser-extension-core dedup`
Expected: FAIL — `not yet implemented`

- [ ] **Step 3: Implement simhash**

Replace the `todo!()` in `simhash`:

```rust
pub fn simhash(text: &str) -> u64 {
    if text.is_empty() {
        return 0;
    }

    let mut counts = [0i32; 64];
    let words: Vec<&str> = text.split_whitespace().collect();

    // Generate shingles (2-grams of words) and hash each
    if words.len() < 2 {
        let h = hash_token(words[0]);
        return h;
    }

    for window in words.windows(2) {
        let shingle = format!("{} {}", window[0], window[1]);
        let h = hash_token(&shingle);
        for i in 0..64 {
            if (h >> i) & 1 == 1 {
                counts[i] += 1;
            } else {
                counts[i] -= 1;
            }
        }
    }

    let mut result: u64 = 0;
    for i in 0..64 {
        if counts[i] > 0 {
            result |= 1u64 << i;
        }
    }
    result
}

fn hash_token(token: &str) -> u64 {
    // FNV-1a 64-bit
    let mut hash: u64 = 0xcbf29ce484222325;
    for byte in token.bytes() {
        hash ^= byte as u64;
        hash = hash.wrapping_mul(0x100000001b3);
    }
    hash
}
```

- [ ] **Step 4: Run tests**

Run: `cd browser-extension && cargo test -p browser-extension-core dedup`
Expected: 7 tests pass.

- [ ] **Step 5: Commit**

```bash
git add browser-extension/crates/core/src/dedup.rs
git commit -m "feat(ext): add SimHash deduplicator with hamming distance"
```

---

### Task 4: HTML Extractor

**Files:**
- Create: `browser-extension/crates/core/src/extractor.rs`

- [ ] **Step 1: Write failing tests**

```rust
// browser-extension/crates/core/src/extractor.rs
use crate::types::{ContentSection, PageMetadata, SectionKind};

pub struct ExtractedPage {
    pub title: String,
    pub sections: Vec<ContentSection>,
    pub metadata: PageMetadata,
}

/// Extract structured content from raw HTML.
/// Strips nav, footer, script, style, and ad-related elements.
pub fn extract(html: &str) -> ExtractedPage {
    todo!()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_extract_title() {
        let html = r#"<html><head><title>My Page</title></head><body><p>Hello</p></body></html>"#;
        let result = extract(html);
        assert_eq!(result.title, "My Page");
    }

    #[test]
    fn test_extract_headings() {
        let html = r#"<html><body><h1>Main Title</h1><h2>Sub Title</h2></body></html>"#;
        let result = extract(html);
        let headings: Vec<_> = result.sections.iter()
            .filter(|s| s.kind == SectionKind::Heading)
            .collect();
        assert_eq!(headings.len(), 2);
        assert_eq!(headings[0].text, "Main Title");
        assert_eq!(headings[0].depth, 1);
        assert_eq!(headings[1].text, "Sub Title");
        assert_eq!(headings[1].depth, 2);
    }

    #[test]
    fn test_extract_paragraphs() {
        let html = r#"<html><body><p>First paragraph.</p><p>Second paragraph.</p></body></html>"#;
        let result = extract(html);
        let paras: Vec<_> = result.sections.iter()
            .filter(|s| s.kind == SectionKind::Paragraph)
            .collect();
        assert_eq!(paras.len(), 2);
        assert_eq!(paras[0].text, "First paragraph.");
    }

    #[test]
    fn test_extract_code_blocks() {
        let html = r#"<html><body><pre><code>fn main() {}</code></pre></body></html>"#;
        let result = extract(html);
        let codes: Vec<_> = result.sections.iter()
            .filter(|s| s.kind == SectionKind::CodeBlock)
            .collect();
        assert_eq!(codes.len(), 1);
        assert_eq!(codes[0].text, "fn main() {}");
    }

    #[test]
    fn test_strips_nav_and_footer() {
        let html = r#"<html><body>
            <nav><a href="/">Home</a></nav>
            <main><p>Content here.</p></main>
            <footer><p>Copyright 2026</p></footer>
        </body></html>"#;
        let result = extract(html);
        let texts: Vec<_> = result.sections.iter().map(|s| &s.text).collect();
        assert!(texts.iter().any(|t| t.contains("Content here")));
        assert!(!texts.iter().any(|t| t.contains("Copyright")));
        assert!(!texts.iter().any(|t| t.contains("Home")));
    }

    #[test]
    fn test_strips_script_and_style() {
        let html = r#"<html><body>
            <script>alert('xss')</script>
            <style>.foo { color: red }</style>
            <p>Real content</p>
        </body></html>"#;
        let result = extract(html);
        let texts: Vec<_> = result.sections.iter().map(|s| &s.text).collect();
        assert!(!texts.iter().any(|t| t.contains("alert")));
        assert!(!texts.iter().any(|t| t.contains("color")));
        assert!(texts.iter().any(|t| t.contains("Real content")));
    }

    #[test]
    fn test_extract_og_metadata() {
        let html = r#"<html><head>
            <meta property="og:title" content="OG Title">
            <meta property="og:description" content="OG Desc">
            <meta name="language" content="en">
            <link rel="canonical" href="https://example.com/page">
        </head><body><p>Text</p></body></html>"#;
        let result = extract(html);
        assert_eq!(result.metadata.og_title.as_deref(), Some("OG Title"));
        assert_eq!(result.metadata.og_description.as_deref(), Some("OG Desc"));
        assert_eq!(result.metadata.canonical_url.as_deref(), Some("https://example.com/page"));
    }

    #[test]
    fn test_extract_lists() {
        let html = r#"<html><body><ul><li>Item 1</li><li>Item 2</li></ul></body></html>"#;
        let result = extract(html);
        let lists: Vec<_> = result.sections.iter()
            .filter(|s| s.kind == SectionKind::List)
            .collect();
        assert_eq!(lists.len(), 1);
        assert!(lists[0].text.contains("Item 1"));
        assert!(lists[0].text.contains("Item 2"));
    }
}
```

- [ ] **Step 2: Run tests to confirm failure**

Run: `cd browser-extension && cargo test -p browser-extension-core extractor`
Expected: FAIL — `not yet implemented`

- [ ] **Step 3: Implement extractor using lol_html**

Replace the `todo!()` body of `extract`:

```rust
use lol_html::{element, rewrite_str, RewriteStrSettings};

/// Tags to skip entirely — their content is dropped.
const SKIP_TAGS: &[&str] = &["nav", "footer", "script", "style", "noscript", "iframe", "svg"];

pub fn extract(html: &str) -> ExtractedPage {
    let mut title = String::new();
    let mut sections: Vec<ContentSection> = Vec::new();
    let mut metadata = PageMetadata::default();
    let mut skip_depth: usize = 0;
    let mut current_list_items: Vec<String> = Vec::new();
    let mut in_list = false;
    let mut in_pre = false;

    // We use a two-pass approach:
    // Pass 1: extract metadata from <head>
    // Pass 2: extract content from <body>
    // lol_html is streaming, so we do both in one pass with state.

    // For simplicity, use a non-streaming approach with scraper for v1.
    // lol_html is better for large pages, but scraper gives us DOM tree access
    // which makes section extraction much simpler.
    // TODO: migrate to lol_html if performance requires it.

    use lol_html::html_content::ContentType;

    // Collect raw text sections using element content handlers
    let mut depth_stack: Vec<String> = Vec::new();

    // Simple approach: use lol_html to strip unwanted tags and collect text
    // For structured extraction, we process the output HTML with a state machine.

    // Actually, let's use a straightforward approach with lol_html handlers:
    let mut pending_kind: Option<SectionKind> = None;
    let mut pending_depth: u8 = 0;
    let mut pending_selector = String::new();
    let mut pending_text = String::new();
    let mut section_idx: usize = 0;

    let handler_result = rewrite_str(
        html,
        RewriteStrSettings {
            element_content_handlers: vec![
                // Title
                element!("title", |el| {
                    el.on_end_tag(|_| Ok(()))?;
                    Ok(())
                }),
                // Metadata
                element!("meta[property='og:title']", |el| {
                    if let Some(content) = el.get_attribute("content") {
                        metadata.og_title = Some(content);
                    }
                    Ok(())
                }),
                element!("meta[property='og:description']", |el| {
                    if let Some(content) = el.get_attribute("content") {
                        metadata.og_description = Some(content);
                    }
                    Ok(())
                }),
                element!("meta[name='language']", |el| {
                    if let Some(content) = el.get_attribute("content") {
                        metadata.lang = Some(content);
                    }
                    Ok(())
                }),
                element!("link[rel='canonical']", |el| {
                    if let Some(href) = el.get_attribute("href") {
                        metadata.canonical_url = Some(href);
                    }
                    Ok(())
                }),
            ],
            ..RewriteStrSettings::default()
        },
    );

    // For the actual content extraction, use a simpler regex/string approach
    // since lol_html's streaming model makes stateful extraction complex.
    // Parse the HTML and extract sections.
    extract_sections_simple(html, &mut title, &mut sections, &mut metadata);

    ExtractedPage {
        title,
        sections,
        metadata,
    }
}

fn extract_sections_simple(
    html: &str,
    title: &mut String,
    sections: &mut Vec<ContentSection>,
    _metadata: &mut PageMetadata,
) {
    // Strip content inside skip tags first
    let mut clean = html.to_string();
    for tag in SKIP_TAGS {
        // Remove <tag>...</tag> blocks (non-greedy)
        let pattern = format!("<{}[^>]*>[\\s\\S]*?</{}>", tag, tag);
        if let Ok(re) = regex::Regex::new(&pattern) {
            clean = re.replace_all(&clean, "").to_string();
        }
    }

    // Extract title
    if let Ok(re) = regex::Regex::new(r"<title[^>]*>([\s\S]*?)</title>") {
        if let Some(caps) = re.captures(&clean) {
            *title = caps[1].trim().to_string();
        }
    }

    // Extract headings (h1-h6)
    if let Ok(re) = regex::Regex::new(r"<(h([1-6]))[^>]*>([\s\S]*?)</\1>") {
        for caps in re.captures_iter(&clean) {
            let depth: u8 = caps[2].parse().unwrap_or(1);
            let text = strip_html_tags(&caps[3]).trim().to_string();
            if !text.is_empty() {
                sections.push(ContentSection {
                    kind: SectionKind::Heading,
                    text,
                    depth,
                    selector: format!("h{}", depth),
                });
            }
        }
    }

    // Extract code blocks (pre > code)
    if let Ok(re) = regex::Regex::new(r"<pre[^>]*>\s*<code[^>]*>([\s\S]*?)</code>\s*</pre>") {
        for caps in re.captures_iter(&clean) {
            let text = decode_html_entities(&caps[1]).trim().to_string();
            if !text.is_empty() {
                sections.push(ContentSection {
                    kind: SectionKind::CodeBlock,
                    text,
                    depth: 0,
                    selector: "pre > code".to_string(),
                });
            }
        }
    }

    // Remove code blocks from clean before extracting paragraphs
    if let Ok(re) = regex::Regex::new(r"<pre[^>]*>[\s\S]*?</pre>") {
        clean = re.replace_all(&clean, "").to_string();
    }

    // Extract lists (ul/ol)
    if let Ok(re) = regex::Regex::new(r"<(ul|ol)[^>]*>([\s\S]*?)</\1>") {
        for caps in re.captures_iter(&clean) {
            let list_html = &caps[2];
            if let Ok(li_re) = regex::Regex::new(r"<li[^>]*>([\s\S]*?)</li>") {
                let items: Vec<String> = li_re.captures_iter(list_html)
                    .map(|c| strip_html_tags(&c[1]).trim().to_string())
                    .filter(|s| !s.is_empty())
                    .collect();
                if !items.is_empty() {
                    sections.push(ContentSection {
                        kind: SectionKind::List,
                        text: items.join("\n"),
                        depth: 0,
                        selector: caps[1].to_string(),
                    });
                }
            }
        }
    }

    // Remove lists from clean before extracting paragraphs
    if let Ok(re) = regex::Regex::new(r"<(ul|ol)[^>]*>[\s\S]*?</\1>") {
        clean = re.replace_all(&clean, "").to_string();
    }

    // Extract paragraphs
    if let Ok(re) = regex::Regex::new(r"<p[^>]*>([\s\S]*?)</p>") {
        for caps in re.captures_iter(&clean) {
            let text = strip_html_tags(&caps[1]).trim().to_string();
            if !text.is_empty() {
                sections.push(ContentSection {
                    kind: SectionKind::Paragraph,
                    text,
                    depth: 0,
                    selector: "p".to_string(),
                });
            }
        }
    }
}

fn strip_html_tags(html: &str) -> String {
    let re = regex::Regex::new(r"<[^>]+>").unwrap();
    re.replace_all(html, "").to_string()
}

fn decode_html_entities(text: &str) -> String {
    text.replace("&amp;", "&")
        .replace("&lt;", "<")
        .replace("&gt;", ">")
        .replace("&quot;", "\"")
        .replace("&#39;", "'")
        .replace("&#x27;", "'")
}
```

Add `regex` to core's dependencies:

```toml
# add to browser-extension/crates/core/Cargo.toml [dependencies]
regex = "1"
```

- [ ] **Step 4: Run tests**

Run: `cd browser-extension && cargo test -p browser-extension-core extractor`
Expected: 8 tests pass.

- [ ] **Step 5: Commit**

```bash
git add browser-extension/crates/core/src/extractor.rs browser-extension/crates/core/Cargo.toml
git commit -m "feat(ext): add HTML extractor — strips noise, extracts structured sections"
```

---

### Task 5: Text Chunker

**Files:**
- Create: `browser-extension/crates/core/src/chunker.rs`

- [ ] **Step 1: Write failing tests**

```rust
// browser-extension/crates/core/src/chunker.rs

/// Split text into chunks of approximately `max_chars` characters,
/// breaking at sentence boundaries when possible.
pub fn chunk_text(text: &str, max_chars: usize) -> Vec<String> {
    todo!()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_short_text_single_chunk() {
        let chunks = chunk_text("Hello world.", 100);
        assert_eq!(chunks.len(), 1);
        assert_eq!(chunks[0], "Hello world.");
    }

    #[test]
    fn test_splits_at_sentence_boundary() {
        let text = "First sentence. Second sentence. Third sentence.";
        let chunks = chunk_text(text, 30);
        assert!(chunks.len() >= 2);
        // Each chunk should end at a sentence boundary
        for chunk in &chunks {
            assert!(chunk.ends_with('.') || chunk.ends_with('!') || chunk.ends_with('?'),
                "Chunk should end at sentence boundary: {:?}", chunk);
        }
    }

    #[test]
    fn test_respects_max_chars() {
        let text = "Word. ".repeat(100);
        let chunks = chunk_text(&text, 50);
        for chunk in &chunks {
            assert!(chunk.len() <= 60, "Chunk too long: {} chars", chunk.len());
        }
    }

    #[test]
    fn test_empty_text() {
        let chunks = chunk_text("", 100);
        assert!(chunks.is_empty());
    }

    #[test]
    fn test_single_long_sentence_not_dropped() {
        let text = "a ".repeat(100); // 200 chars, no sentence boundary
        let chunks = chunk_text(&text, 50);
        assert!(!chunks.is_empty());
        // All text should be represented
        let total: usize = chunks.iter().map(|c| c.len()).sum();
        assert!(total >= 190, "Should not drop content");
    }
}
```

- [ ] **Step 2: Run tests to confirm failure**

Run: `cd browser-extension && cargo test -p browser-extension-core chunker`
Expected: FAIL — `not yet implemented`

- [ ] **Step 3: Implement chunker**

```rust
pub fn chunk_text(text: &str, max_chars: usize) -> Vec<String> {
    let text = text.trim();
    if text.is_empty() {
        return Vec::new();
    }
    if text.len() <= max_chars {
        return vec![text.to_string()];
    }

    let mut chunks = Vec::new();
    let mut start = 0;

    while start < text.len() {
        let end = (start + max_chars).min(text.len());

        if end == text.len() {
            chunks.push(text[start..end].trim().to_string());
            break;
        }

        // Look backward for a sentence boundary (. ! ?)
        let window = &text[start..end];
        let split_at = window.rfind(|c: char| c == '.' || c == '!' || c == '?')
            .map(|pos| pos + 1) // include the punctuation
            .unwrap_or_else(|| {
                // No sentence boundary — split at last space
                window.rfind(' ').unwrap_or(window.len())
            });

        let chunk = text[start..start + split_at].trim().to_string();
        if !chunk.is_empty() {
            chunks.push(chunk);
        }
        start += split_at;
    }

    chunks
}
```

- [ ] **Step 4: Run tests**

Run: `cd browser-extension && cargo test -p browser-extension-core chunker`
Expected: 5 tests pass.

- [ ] **Step 5: Commit**

```bash
git add browser-extension/crates/core/src/chunker.rs
git commit -m "feat(ext): add sentence-aware text chunker"
```

---

### Task 6: TF-IDF Scorer

**Files:**
- Create: `browser-extension/crates/core/src/scorer.rs`

- [ ] **Step 1: Write failing tests**

```rust
// browser-extension/crates/core/src/scorer.rs
use std::collections::HashMap;

/// A lightweight TF-IDF index over documents.
pub struct TfIdfIndex {
    /// doc_id -> term -> count
    docs: HashMap<String, HashMap<String, usize>>,
    /// term -> number of docs containing it
    doc_freq: HashMap<String, usize>,
    total_docs: usize,
}

impl TfIdfIndex {
    pub fn new() -> Self {
        todo!()
    }

    /// Add a document to the index.
    pub fn add(&mut self, doc_id: &str, text: &str) {
        todo!()
    }

    /// Search for a query, returning doc_ids ranked by TF-IDF score.
    pub fn search(&self, query: &str) -> Vec<(String, f64)> {
        todo!()
    }
}

fn tokenize(text: &str) -> Vec<String> {
    text.to_lowercase()
        .split(|c: char| !c.is_alphanumeric())
        .filter(|w| w.len() > 1)
        .map(|w| w.to_string())
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_empty_index_returns_empty() {
        let index = TfIdfIndex::new();
        let results = index.search("anything");
        assert!(results.is_empty());
    }

    #[test]
    fn test_exact_match_ranks_first() {
        let mut index = TfIdfIndex::new();
        index.add("doc1", "rust programming language");
        index.add("doc2", "python programming language");
        index.add("doc3", "rust metal oxidation");
        let results = index.search("rust programming");
        assert_eq!(results[0].0, "doc1", "doc1 matches both terms");
    }

    #[test]
    fn test_idf_boosts_rare_terms() {
        let mut index = TfIdfIndex::new();
        index.add("doc1", "the common word unique_xyz");
        index.add("doc2", "the common word something");
        index.add("doc3", "the common word another");
        let results = index.search("unique_xyz");
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].0, "doc1");
    }

    #[test]
    fn test_scores_are_descending() {
        let mut index = TfIdfIndex::new();
        index.add("doc1", "rust rust rust");
        index.add("doc2", "rust python");
        let results = index.search("rust");
        assert!(results.len() == 2);
        assert!(results[0].1 >= results[1].1, "scores should be descending");
    }

    #[test]
    fn test_tokenize() {
        let tokens = tokenize("Hello, World! This is a test.");
        assert!(tokens.contains(&"hello".to_string()));
        assert!(tokens.contains(&"world".to_string()));
        assert!(tokens.contains(&"test".to_string()));
        // Single-char words filtered out
        assert!(!tokens.contains(&"a".to_string()));
    }
}
```

- [ ] **Step 2: Run tests to confirm failure**

Run: `cd browser-extension && cargo test -p browser-extension-core scorer`
Expected: FAIL — `not yet implemented`

- [ ] **Step 3: Implement TF-IDF**

```rust
impl TfIdfIndex {
    pub fn new() -> Self {
        Self {
            docs: HashMap::new(),
            doc_freq: HashMap::new(),
            total_docs: 0,
        }
    }

    pub fn add(&mut self, doc_id: &str, text: &str) {
        let tokens = tokenize(text);
        let mut term_counts: HashMap<String, usize> = HashMap::new();
        for token in &tokens {
            *term_counts.entry(token.clone()).or_insert(0) += 1;
        }
        // Update doc frequency
        for term in term_counts.keys() {
            *self.doc_freq.entry(term.clone()).or_insert(0) += 1;
        }
        self.docs.insert(doc_id.to_string(), term_counts);
        self.total_docs += 1;
    }

    pub fn search(&self, query: &str) -> Vec<(String, f64)> {
        if self.total_docs == 0 {
            return Vec::new();
        }
        let query_tokens = tokenize(query);
        if query_tokens.is_empty() {
            return Vec::new();
        }

        let mut scores: Vec<(String, f64)> = self.docs.iter()
            .map(|(doc_id, term_counts)| {
                let total_terms: usize = term_counts.values().sum();
                let score: f64 = query_tokens.iter()
                    .map(|qt| {
                        let tf = *term_counts.get(qt).unwrap_or(&0) as f64 / total_terms as f64;
                        let df = *self.doc_freq.get(qt).unwrap_or(&0) as f64;
                        let idf = if df > 0.0 {
                            (self.total_docs as f64 / df).ln() + 1.0
                        } else {
                            0.0
                        };
                        tf * idf
                    })
                    .sum();
                (doc_id.clone(), score)
            })
            .filter(|(_, score)| *score > 0.0)
            .collect();

        scores.sort_by(|a, b| b.1.partial_cmp(&a.1).unwrap_or(std::cmp::Ordering::Equal));
        scores
    }
}
```

- [ ] **Step 4: Run tests**

Run: `cd browser-extension && cargo test -p browser-extension-core scorer`
Expected: 5 tests pass.

- [ ] **Step 5: Commit**

```bash
git add browser-extension/crates/core/src/scorer.rs
git commit -m "feat(ext): add TF-IDF scorer for session-local search"
```

---

### Task 7: Session Store

**Files:**
- Create: `browser-extension/crates/core/src/session.rs`

- [ ] **Step 1: Write failing tests**

```rust
// browser-extension/crates/core/src/session.rs
use crate::types::*;
use crate::dedup;
use crate::scorer::TfIdfIndex;
use std::collections::VecDeque;

const MAX_PAGES: usize = 50;

pub struct SessionManager {
    session: BrowserSession,
    index: TfIdfIndex,
    synced_hashes: std::collections::HashSet<u64>,
}

impl SessionManager {
    pub fn new(session_id: String) -> Self {
        todo!()
    }

    /// Add a page to the session. Returns true if the page is new (not a duplicate).
    pub fn add_page(&mut self, snapshot: PageSnapshot) -> bool {
        todo!()
    }

    /// Get the current (most recent) page.
    pub fn current_page(&self) -> Option<&PageSnapshot> {
        todo!()
    }

    /// Get navigation history.
    pub fn history(&self) -> &[NavEvent] {
        todo!()
    }

    /// Search across all pages in session.
    pub fn search(&self, query: &str, limit: usize) -> Vec<SearchResult> {
        todo!()
    }

    /// Get session summary.
    pub fn summary(&self) -> SessionSummary {
        todo!()
    }

    /// Get pages that need syncing (not yet synced).
    pub fn pending_sync(&self) -> Vec<&PageSnapshot> {
        todo!()
    }

    /// Mark a page hash as synced.
    pub fn mark_synced(&mut self, content_hash: u64) {
        todo!()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_snapshot(url: &str, title: &str, text: &str, ts: u64) -> PageSnapshot {
        let hash = dedup::simhash(text);
        PageSnapshot {
            url: url.to_string(),
            title: title.to_string(),
            timestamp: ts,
            content_hash: hash,
            sections: vec![ContentSection {
                kind: SectionKind::Paragraph,
                text: text.to_string(),
                depth: 0,
                selector: "p".to_string(),
            }],
            metadata: PageMetadata::default(),
        }
    }

    #[test]
    fn test_add_page_and_current() {
        let mut mgr = SessionManager::new("s1".to_string());
        let snap = make_snapshot("https://a.com", "A", "hello world", 1000);
        assert!(mgr.add_page(snap));
        let current = mgr.current_page().unwrap();
        assert_eq!(current.url, "https://a.com");
    }

    #[test]
    fn test_duplicate_page_rejected() {
        let mut mgr = SessionManager::new("s1".to_string());
        let snap1 = make_snapshot("https://a.com", "A", "hello world", 1000);
        let snap2 = make_snapshot("https://a.com", "A", "hello world", 1001);
        assert!(mgr.add_page(snap1));
        assert!(!mgr.add_page(snap2), "duplicate content should be rejected");
    }

    #[test]
    fn test_evicts_oldest_beyond_max() {
        let mut mgr = SessionManager::new("s1".to_string());
        for i in 0..(MAX_PAGES + 5) {
            let text = format!("unique content number {}", i);
            let snap = make_snapshot(
                &format!("https://page{}.com", i),
                &format!("Page {}", i),
                &text,
                i as u64,
            );
            mgr.add_page(snap);
        }
        assert_eq!(mgr.session.pages.len(), MAX_PAGES);
        // Oldest pages should be evicted
        assert_eq!(mgr.session.pages.front().unwrap().url, "https://page5.com");
    }

    #[test]
    fn test_history_records_nav_events() {
        let mut mgr = SessionManager::new("s1".to_string());
        let snap = make_snapshot("https://a.com", "A", "content a", 1000);
        mgr.add_page(snap);
        assert_eq!(mgr.history().len(), 1);
        assert_eq!(mgr.history()[0].url, "https://a.com");
    }

    #[test]
    fn test_search_returns_matching_pages() {
        let mut mgr = SessionManager::new("s1".to_string());
        mgr.add_page(make_snapshot("https://a.com", "Rust Guide", "rust programming language systems", 1));
        mgr.add_page(make_snapshot("https://b.com", "Python Guide", "python programming scripting", 2));
        let results = mgr.search("rust programming", 5);
        assert!(!results.is_empty());
        assert_eq!(results[0].url, "https://a.com");
    }

    #[test]
    fn test_summary() {
        let mut mgr = SessionManager::new("s1".to_string());
        mgr.add_page(make_snapshot("https://a.com", "A", "page a", 1));
        mgr.add_page(make_snapshot("https://b.com", "B", "page b", 2));
        let sum = mgr.summary();
        assert_eq!(sum.page_count, 2);
        assert_eq!(sum.urls.len(), 2);
    }

    #[test]
    fn test_pending_sync_and_mark_synced() {
        let mut mgr = SessionManager::new("s1".to_string());
        let snap = make_snapshot("https://a.com", "A", "content a", 1);
        let hash = snap.content_hash;
        mgr.add_page(snap);
        assert_eq!(mgr.pending_sync().len(), 1);
        mgr.mark_synced(hash);
        assert_eq!(mgr.pending_sync().len(), 0);
    }
}
```

- [ ] **Step 2: Run tests to confirm failure**

Run: `cd browser-extension && cargo test -p browser-extension-core session`
Expected: FAIL — `not yet implemented`

- [ ] **Step 3: Implement SessionManager**

```rust
impl SessionManager {
    pub fn new(session_id: String) -> Self {
        let now = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap_or_default()
            .as_secs();
        Self {
            session: BrowserSession::new(session_id, now),
            index: TfIdfIndex::new(),
            synced_hashes: std::collections::HashSet::new(),
        }
    }

    pub fn add_page(&mut self, snapshot: PageSnapshot) -> bool {
        // Check for duplicate content
        if self.session.pages.iter().any(|p| dedup::is_near_duplicate(p.content_hash, snapshot.content_hash, 3)) {
            return false;
        }

        // Record nav event
        self.session.nav_history.push(NavEvent {
            url: snapshot.url.clone(),
            timestamp: snapshot.timestamp,
            referrer: None,
        });

        // Index text for search
        let doc_id = snapshot.url.clone();
        let full_text: String = snapshot.sections.iter()
            .map(|s| s.text.as_str())
            .collect::<Vec<_>>()
            .join(" ");
        self.index.add(&doc_id, &full_text);

        // Add to pages, evict oldest if needed
        self.session.pages.push_back(snapshot);
        while self.session.pages.len() > MAX_PAGES {
            self.session.pages.pop_front();
        }

        true
    }

    pub fn current_page(&self) -> Option<&PageSnapshot> {
        self.session.pages.back()
    }

    pub fn history(&self) -> &[NavEvent] {
        &self.session.nav_history
    }

    pub fn search(&self, query: &str, limit: usize) -> Vec<SearchResult> {
        let ranked = self.index.search(query);
        ranked.into_iter()
            .take(limit)
            .filter_map(|(url, score)| {
                let page = self.session.pages.iter().find(|p| p.url == url)?;
                let snippet = page.sections.first()
                    .map(|s| {
                        if s.text.len() > 200 {
                            format!("{}...", &s.text[..200])
                        } else {
                            s.text.clone()
                        }
                    })
                    .unwrap_or_default();
                Some(SearchResult {
                    url: page.url.clone(),
                    title: page.title.clone(),
                    snippet,
                    score,
                })
            })
            .collect()
    }

    pub fn summary(&self) -> SessionSummary {
        SessionSummary {
            id: self.session.id.clone(),
            started_at: self.session.started_at,
            page_count: self.session.pages.len(),
            urls: self.session.pages.iter().map(|p| p.url.clone()).collect(),
        }
    }

    pub fn pending_sync(&self) -> Vec<&PageSnapshot> {
        self.session.pages.iter()
            .filter(|p| !self.synced_hashes.contains(&p.content_hash))
            .collect()
    }

    pub fn mark_synced(&mut self, content_hash: u64) {
        self.synced_hashes.insert(content_hash);
    }
}
```

- [ ] **Step 4: Run tests**

Run: `cd browser-extension && cargo test -p browser-extension-core session`
Expected: 7 tests pass.

- [ ] **Step 5: Commit**

```bash
git add browser-extension/crates/core/src/session.rs
git commit -m "feat(ext): add SessionManager — rolling page store with dedup, search, sync tracking"
```

---

### Task 8: WASM Bindings

**Files:**
- Modify: `browser-extension/crates/wasm/src/lib.rs`

- [ ] **Step 1: Implement WASM bindings**

```rust
// browser-extension/crates/wasm/src/lib.rs
use wasm_bindgen::prelude::*;
use mairu_ext_core::{
    extractor, chunker, dedup,
    session::SessionManager,
    types::*,
};
use std::cell::RefCell;

thread_local! {
    static SESSION: RefCell<Option<SessionManager>> = RefCell::new(None);
}

#[wasm_bindgen]
pub fn init_session(session_id: &str) {
    SESSION.with(|s| {
        *s.borrow_mut() = Some(SessionManager::new(session_id.to_string()));
    });
}

#[wasm_bindgen]
pub fn process_page(url: &str, html: &str, timestamp: u64) -> JsValue {
    let extracted = extractor::extract(html);
    let content_hash = dedup::simhash(
        &extracted.sections.iter().map(|s| s.text.as_str()).collect::<Vec<_>>().join(" ")
    );

    let snapshot = PageSnapshot {
        url: url.to_string(),
        title: extracted.title,
        timestamp,
        content_hash,
        sections: extracted.sections,
        metadata: extracted.metadata,
    };

    let is_new = SESSION.with(|s| {
        let mut s = s.borrow_mut();
        match s.as_mut() {
            Some(mgr) => mgr.add_page(snapshot.clone()),
            None => false,
        }
    });

    serde_wasm_bindgen::to_value(&serde_json::json!({
        "is_new": is_new,
        "content_hash": content_hash,
        "section_count": snapshot.sections.len(),
    })).unwrap_or(JsValue::NULL)
}

#[wasm_bindgen]
pub fn get_current() -> JsValue {
    SESSION.with(|s| {
        let s = s.borrow();
        match s.as_ref().and_then(|mgr| mgr.current_page()) {
            Some(page) => serde_wasm_bindgen::to_value(page).unwrap_or(JsValue::NULL),
            None => JsValue::NULL,
        }
    })
}

#[wasm_bindgen]
pub fn get_history() -> JsValue {
    SESSION.with(|s| {
        let s = s.borrow();
        match s.as_ref() {
            Some(mgr) => serde_wasm_bindgen::to_value(mgr.history()).unwrap_or(JsValue::NULL),
            None => JsValue::NULL,
        }
    })
}

#[wasm_bindgen]
pub fn search_session(query: &str, limit: usize) -> JsValue {
    SESSION.with(|s| {
        let s = s.borrow();
        match s.as_ref() {
            Some(mgr) => {
                let results = mgr.search(query, limit);
                serde_wasm_bindgen::to_value(&results).unwrap_or(JsValue::NULL)
            }
            None => JsValue::NULL,
        }
    })
}

#[wasm_bindgen]
pub fn get_session_summary() -> JsValue {
    SESSION.with(|s| {
        let s = s.borrow();
        match s.as_ref() {
            Some(mgr) => serde_wasm_bindgen::to_value(&mgr.summary()).unwrap_or(JsValue::NULL),
            None => JsValue::NULL,
        }
    })
}

#[wasm_bindgen]
pub fn get_pending_sync() -> JsValue {
    SESSION.with(|s| {
        let s = s.borrow();
        match s.as_ref() {
            Some(mgr) => {
                let pending: Vec<_> = mgr.pending_sync().into_iter().cloned().collect();
                serde_wasm_bindgen::to_value(&pending).unwrap_or(JsValue::NULL)
            }
            None => JsValue::NULL,
        }
    })
}

#[wasm_bindgen]
pub fn mark_synced(content_hash: u64) {
    SESSION.with(|s| {
        let mut s = s.borrow_mut();
        if let Some(mgr) = s.as_mut() {
            mgr.mark_synced(content_hash);
        }
    });
}
```

- [ ] **Step 2: Build WASM**

Run: `cd browser-extension && wasm-pack build crates/wasm --target web`
Expected: Build succeeds, `crates/wasm/pkg/` has `.wasm` and `.js` files.

- [ ] **Step 3: Commit**

```bash
git add browser-extension/crates/wasm/src/lib.rs
git commit -m "feat(ext): add wasm-bindgen exports wrapping core session/extraction"
```

---

### Task 9: Chrome Extension — Content Script & Service Worker

**Files:**
- Create: `browser-extension/extension/manifest.json`
- Create: `browser-extension/extension/content.js`
- Create: `browser-extension/extension/service-worker.js`

- [ ] **Step 1: Create MV3 manifest**

```json
{
  "manifest_version": 3,
  "name": "Mairu Browser Context",
  "version": "0.1.0",
  "description": "Gives mairu chat real-time browser awareness",
  "permissions": [
    "activeTab",
    "storage",
    "nativeMessaging"
  ],
  "background": {
    "service_worker": "service-worker.js",
    "type": "module"
  },
  "content_scripts": [
    {
      "matches": ["<all_urls>"],
      "js": ["content.js"],
      "run_at": "document_idle"
    }
  ],
  "action": {
    "default_popup": "popup/popup.html"
  }
}
```

- [ ] **Step 2: Create content script**

```javascript
// browser-extension/extension/content.js

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
```

- [ ] **Step 3: Create service worker**

```javascript
// browser-extension/extension/service-worker.js

import init, {
  init_session,
  process_page,
  get_current,
  get_history,
  search_session,
  get_session_summary,
  get_pending_sync,
  mark_synced,
} from "./pkg/mairu_ext_wasm.js";

let wasmReady = false;
const MAIRU_API_URL = "http://127.0.0.1:7080"; // default mairu API port
const SYNC_INTERVAL_MS = 10000;
const SYNC_BATCH_SIZE = 5;

// Initialize WASM and session
async function initialize() {
  await init();
  const sessionId = `session-${Date.now()}`;
  init_session(sessionId);
  wasmReady = true;
  console.log("[browser-extension] WASM initialized, session:", sessionId);
}

initialize();

// Handle messages from content scripts
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (!wasmReady) return;

  if (message.type === "page_content") {
    const { url, html, timestamp } = message.payload;
    const result = process_page(url, html, timestamp);
    console.log("[browser-extension] Processed page:", url, result);
  }
});

// Handle native messaging from mairu chat (via native host)
chrome.runtime.onConnectExternal.addListener((port) => {
  port.onMessage.addListener((msg) => {
    if (!wasmReady) {
      port.postMessage({ error: "WASM not ready" });
      return;
    }

    let response;
    switch (msg.command) {
      case "current":
        response = get_current();
        break;
      case "history":
        response = get_history();
        break;
      case "search":
        response = search_session(msg.query || "", msg.limit || 5);
        break;
      case "session":
        response = get_session_summary();
        break;
      default:
        response = { error: `Unknown command: ${msg.command}` };
    }
    port.postMessage(response);
  });
});

// Background sync to mairu API
async function syncToMairu() {
  if (!wasmReady) return;

  const pending = get_pending_sync();
  if (!pending || pending.length === 0) return;

  const batch = pending.slice(0, SYNC_BATCH_SIZE);
  for (const page of batch) {
    try {
      const urlHash = page.content_hash.toString(16);
      const body = {
        uri: `contextfs://browser/${page.url}`,
        name: page.title,
        abstract: page.sections.slice(0, 1).map((s) => s.text).join(" ").slice(0, 200),
        overview: page.sections
          .filter((s) => s.kind === "heading")
          .map((s) => s.text)
          .join("\n"),
        content: page.sections.map((s) => s.text).join("\n\n"),
      };

      const resp = await fetch(`${MAIRU_API_URL}/api/nodes`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });

      if (resp.ok) {
        mark_synced(page.content_hash);
        console.log("[browser-extension] Synced:", page.url);
      }
    } catch (err) {
      console.warn("[browser-extension] Sync failed for", page.url, err);
    }
  }
}

setInterval(syncToMairu, SYNC_INTERVAL_MS);
```

- [ ] **Step 4: Verify extension loads**

Load unpacked extension in Chrome:
1. Build WASM: `cd browser-extension && wasm-pack build crates/wasm --target web`
2. Copy `crates/wasm/pkg/` to `extension/pkg/`
3. Open `chrome://extensions`, enable Developer mode, "Load unpacked" → select `extension/`
Expected: Extension loads with no errors in the service worker console.

- [ ] **Step 5: Commit**

```bash
git add browser-extension/extension/
git commit -m "feat(ext): add Chrome MV3 extension — content script, service worker, background sync"
```

---

### Task 10: Native Messaging Host

**Files:**
- Modify: `browser-extension/native-host/src/main.rs`
- Create: `browser-extension/install.sh`

- [ ] **Step 1: Implement native messaging host**

```rust
// browser-extension/native-host/src/main.rs
use std::io::{self, Read, Write};
use tiny_http::{Server, Response, Method};
use serde::{Deserialize, Serialize};
use std::sync::{Arc, Mutex};
use std::thread;

/// Chrome native messaging uses length-prefixed JSON over stdin/stdout.

fn read_native_message() -> io::Result<Vec<u8>> {
    let mut len_bytes = [0u8; 4];
    io::stdin().read_exact(&mut len_bytes)?;
    let len = u32::from_ne_bytes(len_bytes) as usize;
    let mut buf = vec![0u8; len];
    io::stdin().read_exact(&mut buf)?;
    Ok(buf)
}

fn write_native_message(data: &[u8]) -> io::Result<()> {
    let len = (data.len() as u32).to_ne_bytes();
    io::stdout().write_all(&len)?;
    io::stdout().write_all(data)?;
    io::stdout().flush()?;
    Ok(())
}

#[derive(Deserialize)]
struct BrowserRequest {
    command: String,
    #[serde(default)]
    query: String,
    #[serde(default = "default_limit")]
    limit: usize,
}

fn default_limit() -> usize {
    5
}

fn main() {
    let port = std::env::var("MAIRU_EXT_PORT")
        .unwrap_or_else(|_| "7091".to_string())
        .parse::<u16>()
        .unwrap_or(7091);

    let server = Server::http(format!("127.0.0.1:{}", port))
        .expect("Failed to start HTTP server");
    eprintln!("[browser-extension-host] Listening on 127.0.0.1:{}", port);

    // In a full implementation, this would connect to the Chrome extension
    // via native messaging. For v1, it serves as an HTTP proxy that relays
    // requests to the extension and returns responses.
    //
    // The native messaging channel is opened by Chrome when the extension
    // calls chrome.runtime.connectNative("com.mairu.ext").
    // For now, we implement direct HTTP -> native messaging bridging.

    for request in server.incoming_requests() {
        let url = request.url().to_string();
        let method = request.method().clone();

        match (&method, url.as_str()) {
            (&Method::Get, "/ping") => {
                let _ = request.respond(Response::from_string("ok"));
            }
            (&Method::Post, "/browser_context") => {
                let mut body = String::new();
                let mut reader = request;
                if reader.as_reader().read_to_string(&mut body).is_ok() {
                    match serde_json::from_str::<BrowserRequest>(&body) {
                        Ok(req) => {
                            // Forward to extension via native messaging
                            let msg = serde_json::json!({
                                "command": req.command,
                                "query": req.query,
                                "limit": req.limit,
                            });
                            let msg_bytes = serde_json::to_vec(&msg).unwrap_or_default();

                            if write_native_message(&msg_bytes).is_ok() {
                                match read_native_message() {
                                    Ok(response_bytes) => {
                                        let _ = reader.respond(
                                            Response::from_data(response_bytes)
                                                .with_header(
                                                    tiny_http::Header::from_bytes(
                                                        &b"Content-Type"[..],
                                                        &b"application/json"[..]
                                                    ).unwrap()
                                                )
                                        );
                                    }
                                    Err(e) => {
                                        let err = serde_json::json!({"error": format!("native messaging read failed: {}", e)});
                                        let _ = reader.respond(
                                            Response::from_string(err.to_string())
                                                .with_status_code(502)
                                        );
                                    }
                                }
                            } else {
                                let err = serde_json::json!({"error": "native messaging write failed"});
                                let _ = reader.respond(
                                    Response::from_string(err.to_string())
                                        .with_status_code(502)
                                );
                            }
                        }
                        Err(e) => {
                            let err = serde_json::json!({"error": format!("bad request: {}", e)});
                            let _ = reader.respond(
                                Response::from_string(err.to_string())
                                    .with_status_code(400)
                            );
                        }
                    }
                }
            }
            _ => {
                let _ = request.respond(Response::from_string("not found").with_status_code(404));
            }
        }
    }
}
```

- [ ] **Step 2: Create install script**

```bash
#!/bin/bash
# browser-extension/install.sh
# Registers the native messaging host with Chrome/Chromium

set -e

HOST_NAME="com.mairu.ext"
BINARY_PATH="$(cd "$(dirname "$0")" && pwd)/target/release/browser-extension-host"

# Build the native host
cargo build --release -p browser-extension-host

# Determine Chrome native messaging directory
if [[ "$OSTYPE" == "darwin"* ]]; then
    NM_DIR="$HOME/Library/Application Support/Google/Chrome/NativeMessagingHosts"
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    NM_DIR="$HOME/.config/google-chrome/NativeMessagingHosts"
else
    echo "Unsupported OS: $OSTYPE"
    exit 1
fi

mkdir -p "$NM_DIR"

cat > "$NM_DIR/$HOST_NAME.json" <<MANIFEST
{
  "name": "$HOST_NAME",
  "description": "Mairu browser extension native messaging host",
  "path": "$BINARY_PATH",
  "type": "stdio",
  "allowed_origins": [
    "chrome-extension://EXTENSION_ID_HERE/"
  ]
}
MANIFEST

echo "Native messaging host registered at $NM_DIR/$HOST_NAME.json"
echo "IMPORTANT: Replace EXTENSION_ID_HERE with your actual extension ID from chrome://extensions"
```

- [ ] **Step 3: Build and verify**

Run: `cd browser-extension && cargo build -p browser-extension-host`
Expected: Compiles successfully.

- [ ] **Step 4: Commit**

```bash
git add browser-extension/native-host/src/main.rs browser-extension/install.sh
git commit -m "feat(ext): add native messaging host — HTTP bridge for mairu chat"
```

---

### Task 11: Minimal Popup

**Files:**
- Create: `browser-extension/extension/popup/popup.html`
- Create: `browser-extension/extension/popup/popup.js`

- [ ] **Step 1: Create popup HTML**

```html
<!-- browser-extension/extension/popup/popup.html -->
<!DOCTYPE html>
<html>
<head>
  <style>
    body { width: 280px; padding: 12px; font-family: system-ui, sans-serif; font-size: 13px; }
    h3 { margin: 0 0 8px; font-size: 14px; }
    .stat { display: flex; justify-content: space-between; padding: 4px 0; }
    .stat-label { color: #666; }
    .stat-value { font-weight: 600; }
    .status { padding: 4px 8px; border-radius: 4px; font-size: 11px; text-align: center; margin-bottom: 8px; }
    .status.ok { background: #d4edda; color: #155724; }
    .status.err { background: #f8d7da; color: #721c24; }
  </style>
</head>
<body>
  <h3>Mairu Browser Context</h3>
  <div id="status" class="status">Checking...</div>
  <div class="stat">
    <span class="stat-label">Pages tracked</span>
    <span class="stat-value" id="page-count">-</span>
  </div>
  <div class="stat">
    <span class="stat-label">Pending sync</span>
    <span class="stat-value" id="pending-count">-</span>
  </div>
  <div class="stat">
    <span class="stat-label">Session</span>
    <span class="stat-value" id="session-id">-</span>
  </div>
  <script src="popup.js"></script>
</body>
</html>
```

- [ ] **Step 2: Create popup script**

```javascript
// browser-extension/extension/popup/popup.js

async function updateStatus() {
  try {
    const response = await chrome.runtime.sendMessage({ type: "get_status" });
    if (response) {
      document.getElementById("status").textContent = "Active";
      document.getElementById("status").className = "status ok";
      document.getElementById("page-count").textContent = response.pageCount || 0;
      document.getElementById("pending-count").textContent = response.pendingCount || 0;
      document.getElementById("session-id").textContent = response.sessionId || "-";
    }
  } catch (err) {
    document.getElementById("status").textContent = "Not connected";
    document.getElementById("status").className = "status err";
  }
}

updateStatus();
```

- [ ] **Step 3: Add status handler to service worker**

Add this block inside the `onMessage` listener in `service-worker.js`, before the `page_content` handler:

```javascript
  if (message.type === "get_status") {
    const summary = get_session_summary();
    const pending = get_pending_sync();
    sendResponse({
      pageCount: summary?.page_count || 0,
      pendingCount: pending?.length || 0,
      sessionId: summary?.id || "-",
    });
    return true; // async response
  }
```

- [ ] **Step 4: Commit**

```bash
git add browser-extension/extension/popup/
git commit -m "feat(ext): add minimal status popup — page count, sync status, session ID"
```

---

### Task 12: Mairu Agent Integration — browser_context Tool

**Files:**
- Create: `mairu/internal/agent/browser.go`
- Modify: `mairu/internal/llm/tools.go`
- Modify: `mairu/internal/agent/agent.go`

- [ ] **Step 1: Write the browser client**

```go
// mairu/internal/agent/browser.go
package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type BrowserClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewBrowserClient(port int) *BrowserClient {
	return &BrowserClient{
		baseURL: fmt.Sprintf("http://127.0.0.1:%d", port),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

type browserRequest struct {
	Command string `json:"command"`
	Query   string `json:"query,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

func (bc *BrowserClient) Ping() error {
	resp, err := bc.httpClient.Get(bc.baseURL + "/ping")
	if err != nil {
		return fmt.Errorf("browser extension not reachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("browser extension returned status %d", resp.StatusCode)
	}
	return nil
}

func (bc *BrowserClient) Query(command string, query string, limit int) (map[string]any, error) {
	reqBody := browserRequest{
		Command: command,
		Query:   query,
		Limit:   limit,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	resp, err := bc.httpClient.Post(
		bc.baseURL+"/browser_context",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("browser extension request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("invalid response from browser extension: %w", err)
	}
	return result, nil
}
```

- [ ] **Step 2: Add tool declaration to tools.go**

Add this `FunctionDeclaration` to the tools slice in `mairu/internal/llm/tools.go` (inside the `SetupTools` function, before the closing of the slice):

```go
{
    Name:        "browser_context",
    Description: "Query the browser extension for information about the current browser session. Use 'current' to get the active page content, 'history' for navigation history, 'search' to find content across visited pages, or 'session' for a session overview.",
    Parameters: &genai.Schema{
        Type: genai.TypeObject,
        Properties: map[string]*genai.Schema{
            "command": {
                Type:        genai.TypeString,
                Description: "One of: current, history, search, session",
                Enum:        []string{"current", "history", "search", "session"},
            },
            "query": {
                Type:        genai.TypeString,
                Description: "Search query (only used with 'search' command)",
            },
            "limit": {
                Type:        genai.TypeInteger,
                Description: "Max results to return (only used with 'search' command, default 5)",
            },
        },
        Required: []string{"command"},
    },
},
```

- [ ] **Step 3: Add browser field to Agent struct**

In `mairu/internal/agent/agent.go`, add to the `Agent` struct:

```go
browser *BrowserClient
```

And in the agent constructor (wherever `Agent` is instantiated), add:

```go
browser: NewBrowserClient(7091),
```

- [ ] **Step 4: Add tool dispatch case**

In the `executeToolCall` switch in `mairu/internal/agent/agent.go`, add:

```go
case "browser_context":
    command, _ := funcCall.Args["command"].(string)
    query, _ := funcCall.Args["query"].(string)
    limit := 5
    if l, ok := funcCall.Args["limit"].(float64); ok {
        limit = int(l)
    }

    outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🌐 Browser: %s", command)}

    if err := a.browser.Ping(); err != nil {
        result = map[string]any{"error": "Browser extension not available. Make sure the native messaging host is running."}
    } else {
        data, err := a.browser.Query(command, query, limit)
        if err != nil {
            result = map[string]any{"error": err.Error()}
        } else {
            result = data
        }
    }
```

- [ ] **Step 5: Verify build**

Run: `cd /Users/enekosarasola/mairu && make build`
Expected: Compiles with no errors.

- [ ] **Step 6: Commit**

```bash
git add mairu/internal/agent/browser.go mairu/internal/llm/tools.go mairu/internal/agent/agent.go
git commit -m "feat(agent): add browser_context tool — queries browser extension for page content and session data"
```

---

### Task 13: End-to-End Verification

- [ ] **Step 1: Build everything**

```bash
cd browser-extension && cargo build --workspace
cd browser-extension && wasm-pack build crates/wasm --target web
cp -r crates/wasm/pkg/ extension/pkg/
cd /Users/enekosarasola/mairu && make build
```

Expected: All build steps succeed.

- [ ] **Step 2: Run all Rust tests**

Run: `cd browser-extension && cargo test --workspace`
Expected: All tests pass (types: 4, dedup: 7, extractor: 8, chunker: 5, scorer: 5, session: 7 = ~36 tests).

- [ ] **Step 3: Run Go tests**

Run: `cd /Users/enekosarasola/mairu && make test`
Expected: Existing tests still pass.

- [ ] **Step 4: Manual integration test**

1. Load extension in Chrome (developer mode, "Load unpacked" → `browser-extension/extension/`)
2. Run native host: `cd browser-extension && cargo run -p browser-extension-host`
3. Browse to a docs page (e.g., doc.rust-lang.org)
4. Test: `curl -X POST http://127.0.0.1:7091/browser_context -d '{"command":"current"}'`
Expected: Returns JSON with the page title and extracted sections.
5. Test search: `curl -X POST http://127.0.0.1:7091/browser_context -d '{"command":"search","query":"ownership","limit":3}'`
Expected: Returns ranked results from visited pages.

- [ ] **Step 5: Commit any fixes**

```bash
git add -A
git commit -m "fix(ext): end-to-end integration fixes"
```
