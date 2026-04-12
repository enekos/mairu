// mairu-ext/crates/core/src/session.rs
use crate::dedup;
use crate::importance::compute_importance;
use crate::scorer::TfIdfIndex;
use crate::types::*;

const MAX_PAGES: usize = 50;
const DEDUP_THRESHOLD: u32 = 3;

pub struct SessionManager {
    session: BrowserSession,
    index: TfIdfIndex,
    synced_hashes: std::collections::HashSet<u64>,
}

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

    pub fn add_page(&mut self, mut snapshot: PageSnapshot) -> AddPageResult {
        // Compute importance before inserting
        snapshot.importance_score = compute_importance(&snapshot);

        // SPA dedup: same URL already exists → update in-place with revision bump
        if let Some(existing) = self
            .session
            .pages
            .iter_mut()
            .find(|p| p.url == snapshot.url)
        {
            // Skip if content hasn't changed meaningfully
            if dedup::is_near_duplicate(existing.content_hash, snapshot.content_hash, DEDUP_THRESHOLD) {
                return AddPageResult::Duplicate;
            }
            snapshot.revision = existing.revision + 1;
            // Remove old index entry before replacing
            self.synced_hashes.remove(&existing.content_hash);
            *existing = snapshot;
            // Re-index with updated content
            let doc_id = existing.url.clone();
            let full_text = existing.full_text();
            self.index.add(&doc_id, &full_text);
            return AddPageResult::Updated;
        }

        // Cross-URL near-duplicate check (same content, different URL)
        if self
            .session
            .pages
            .iter()
            .any(|p| dedup::is_near_duplicate(p.content_hash, snapshot.content_hash, DEDUP_THRESHOLD))
        {
            return AddPageResult::Duplicate;
        }

        // Record nav event
        self.session.nav_history.push(NavEvent {
            url: snapshot.url.clone(),
            timestamp: snapshot.timestamp,
            referrer: None,
        });

        // Index text for search
        let doc_id = snapshot.url.clone();
        let full_text = snapshot.full_text();
        self.index.add(&doc_id, &full_text);

        // Add to pages, evict oldest if needed
        self.session.pages.push_back(snapshot);
        while self.session.pages.len() > MAX_PAGES {
            if let Some(evicted) = self.session.pages.pop_front() {
                self.synced_hashes.remove(&evicted.content_hash);
            }
        }

        AddPageResult::Added
    }

    pub fn current_page(&self) -> Option<&PageSnapshot> {
        self.session.pages.back()
    }

    pub fn history(&self) -> &[NavEvent] {
        &self.session.nav_history
    }

    pub fn search(&self, query: &str, limit: usize) -> Vec<SearchResult> {
        // Re-rank with page importance scores
        let ranked = self.index.search_with_importance(query, |url| {
            self.session
                .pages
                .iter()
                .find(|p| p.url == url)
                .map(|p| p.importance_score)
                .unwrap_or(0.0)
        });
        ranked
            .into_iter()
            .take(limit)
            .filter_map(|(url, score)| {
                let page = self.session.pages.iter().find(|p| p.url == url)?;
                let snippet = page
                    .sections
                    .first()
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
        self.session
            .pages
            .iter()
            .filter(|p| !self.synced_hashes.contains(&p.content_hash))
            .collect()
    }

    pub fn mark_synced(&mut self, content_hash: u64) {
        self.synced_hashes.insert(content_hash);
    }

    /// Serialize the full session state to JSON for persistence (e.g. chrome.storage.session).
    pub fn export_session(&self) -> Result<String, serde_json::Error> {
        serde_json::to_string(&self.session)
    }

    /// Restore session state from a previously exported JSON blob.
    /// Re-indexes all pages into the TF-IDF index.
    pub fn import_session(json: &str) -> Result<Self, serde_json::Error> {
        let session: BrowserSession = serde_json::from_str(json)?;
        let mut index = TfIdfIndex::new();
        for page in &session.pages {
            let doc_id = page.url.clone();
            index.add(&doc_id, &page.full_text());
        }
        Ok(Self {
            session,
            index,
            synced_hashes: std::collections::HashSet::new(),
        })
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
                depth: None,
                selector: "p".to_string(),
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
        }
    }

    #[test]
    fn test_add_page_and_current() {
        let mut mgr = SessionManager::new("s1".to_string());
        let snap = make_snapshot("https://a.com", "A", "hello world", 1000);
        assert_eq!(mgr.add_page(snap), AddPageResult::Added);
        let current = mgr.current_page().unwrap();
        assert_eq!(current.url, "https://a.com");
    }

    #[test]
    fn test_duplicate_page_rejected() {
        let mut mgr = SessionManager::new("s1".to_string());
        let snap1 = make_snapshot("https://a.com", "A", "hello world", 1000);
        let snap2 = make_snapshot("https://a.com", "A", "hello world", 1001);
        assert_eq!(mgr.add_page(snap1), AddPageResult::Added);
        assert_eq!(
            mgr.add_page(snap2),
            AddPageResult::Duplicate,
            "duplicate content should be rejected"
        );
    }

    #[test]
    fn test_spa_url_update_in_place() {
        let mut mgr = SessionManager::new("s1".to_string());
        let snap1 = make_snapshot(
            "https://app.com/page",
            "Page v1",
            "initial content here",
            1000,
        );
        assert_eq!(mgr.add_page(snap1), AddPageResult::Added);
        assert_eq!(mgr.session.pages.len(), 1);

        // Same URL, different content (SPA navigation)
        let snap2 = make_snapshot(
            "https://app.com/page",
            "Page v2",
            "completely different updated content",
            2000,
        );
        assert_eq!(mgr.add_page(snap2), AddPageResult::Updated);

        // Should still have only 1 page (updated in-place)
        assert_eq!(mgr.session.pages.len(), 1);
        assert_eq!(mgr.session.pages[0].title, "Page v2");
        assert_eq!(mgr.session.pages[0].revision, 1);

        // Search should find the new content
        let results = mgr.search("updated content", 5);
        assert!(!results.is_empty());
        assert_eq!(results[0].url, "https://app.com/page");
    }

    #[test]
    fn test_evicts_oldest_beyond_max() {
        let mut mgr = SessionManager::new("s1".to_string());
        for i in 0..(MAX_PAGES + 5) {
            let text = format!("{} ", i).repeat(20);
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
        mgr.add_page(make_snapshot(
            "https://a.com",
            "Rust Guide",
            "rust programming language systems",
            1,
        ));
        mgr.add_page(make_snapshot(
            "https://b.com",
            "Python Guide",
            "python programming scripting",
            2,
        ));
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

    #[test]
    fn test_export_import_roundtrip() {
        let mut mgr = SessionManager::new("s1".to_string());
        mgr.add_page(make_snapshot(
            "https://a.com",
            "A",
            "rust systems programming",
            1,
        ));
        mgr.add_page(make_snapshot(
            "https://b.com",
            "B",
            "python scripting language",
            2,
        ));

        let json = mgr.export_session().expect("export should succeed");
        let restored = SessionManager::import_session(&json).expect("import should succeed");

        assert_eq!(restored.summary().page_count, 2);
        assert_eq!(restored.summary().id, "s1");

        // Search should work on restored session
        let results = restored.search("rust programming", 5);
        assert!(!results.is_empty());
        assert_eq!(results[0].url, "https://a.com");
    }

    #[test]
    fn test_import_invalid_json_fails() {
        let result = SessionManager::import_session("not valid json");
        assert!(result.is_err());
    }
}
