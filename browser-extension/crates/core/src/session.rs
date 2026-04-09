// mairu-ext/crates/core/src/session.rs
use crate::dedup;
use crate::scorer::TfIdfIndex;
use crate::types::*;

const MAX_PAGES: usize = 50;

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

    pub fn add_page(&mut self, snapshot: PageSnapshot) -> bool {
        // Check for duplicate content
        if self
            .session
            .pages
            .iter()
            .any(|p| dedup::is_near_duplicate(p.content_hash, snapshot.content_hash, 3))
        {
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
        let full_text: String = snapshot
            .sections
            .iter()
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
            let text = format!("{} ", i.to_string()).repeat(20);
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
}
