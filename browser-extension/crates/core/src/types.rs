// mairu-ext/crates/core/src/types.rs
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

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct ContentSection {
    pub kind: SectionKind,
    pub text: String,
    pub depth: Option<u8>,
    pub selector: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default, PartialEq)]
pub struct PageMetadata {
    pub og_title: Option<String>,
    pub og_description: Option<String>,
    pub lang: Option<String>,
    pub canonical_url: Option<String>,
}

/// Content extracted from an iframe.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct IframeContent {
    pub src: String,
    #[serde(default)]
    pub title: Option<String>,
    #[serde(default)]
    pub is_same_origin: bool,
    #[serde(default)]
    pub sections: Vec<ContentSection>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct PageSnapshot {
    pub url: String,
    pub title: String,
    pub timestamp: u64,
    #[serde(with = "hex_serde")]
    pub content_hash: u64,
    pub sections: Vec<ContentSection>,
    pub metadata: PageMetadata,
    pub selection: Option<String>,
    pub active_element: Option<String>,
    pub console_errors: Vec<String>,
    pub network_errors: Vec<String>,
    pub visual_rects: std::collections::HashMap<String, String>,
    pub storage_state: std::collections::HashMap<String, String>,
    #[serde(default)]
    pub revision: u32,
    #[serde(default)]
    pub importance_score: f64,
    #[serde(default)]
    pub dwell_ms: u64,
    #[serde(default)]
    pub interaction_count: u32,
    #[serde(default)]
    pub iframe_content: Vec<IframeContent>,
}

impl PageSnapshot {
    pub fn full_text(&self) -> String {
        self.sections.iter().map(|s| s.text.as_str()).collect::<Vec<_>>().join(" ")
    }
}

/// Result of adding a page to the session.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum AddPageResult {
    Added,
    Updated,
    Duplicate,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct NavEvent {
    pub url: String,
    pub timestamp: u64,
    pub referrer: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
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

mod hex_serde {
    use serde::{Deserialize, Deserializer, Serializer};

    pub fn serialize<S: Serializer>(value: &u64, s: S) -> Result<S::Ok, S::Error> {
        s.serialize_str(&format!("{:016x}", value))
    }

    pub fn deserialize<'de, D: Deserializer<'de>>(d: D) -> Result<u64, D::Error> {
        let s = String::deserialize(d)?;
        u64::from_str_radix(s.trim_start_matches("0x"), 16).map_err(serde::de::Error::custom)
    }
}

/// Response format for the browser_context tool
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", content = "data")]
#[serde(rename_all = "snake_case")]
pub enum BrowserResponse {
    Current(Box<PageSnapshot>),
    History(Vec<NavEvent>),
    Search(Vec<SearchResult>),
    Session(SessionSummary),
    Error(String),
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SearchResult {
    pub url: String,
    pub title: String,
    pub snippet: String,
    pub score: f64,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
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
                depth: Some(1),
                selector: "h1".to_string(),
            }],
            metadata: PageMetadata::default(),
            selection: None,
            active_element: None,
            console_errors: vec![],
            network_errors: vec![],
            visual_rects: std::collections::HashMap::new(),
            storage_state: std::collections::HashMap::new(),
            revision: 0,
            importance_score: 0.0,
            dwell_ms: 0,
            interaction_count: 0,
            iframe_content: vec![],
        };
        let json = serde_json::to_string(&snap).unwrap();
        // content_hash should be a hex string, not a number
        assert!(
            json.contains(r#""content_hash":"000000000000002a""#),
            "content_hash should serialize as hex string, got: {}",
            json
        );
        let back: PageSnapshot = serde_json::from_str(&json).unwrap();
        assert_eq!(back.url, "https://example.com");
        assert_eq!(back.content_hash, 42);
        assert_eq!(back.sections.len(), 1);
        assert_eq!(back.sections[0].text, "Hello"); // verify nested field
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
