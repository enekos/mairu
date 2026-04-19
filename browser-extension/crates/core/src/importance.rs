// mairu-ext/crates/core/src/importance.rs
//
// Heuristic importance scoring for page snapshots.
// Returns a score in [0.0, 10.0] — higher means more useful to an agent.

use crate::types::{PageSnapshot, SectionKind};

/// Compute an importance score for a page snapshot.
///
/// Factors:
/// - Content depth: number of meaningful sections and total text length
/// - Structural richness: headings, code blocks, lists
/// - User signals: has selection, active element, console/network errors
/// - Dwell time: longer dwell → more relevant (capped contribution)
/// - Interaction count: scrolls, clicks, keypresses
pub fn compute_importance(snap: &PageSnapshot) -> f64 {
    let mut score = 0.0_f64;

    // --- Content depth ---
    let total_text: usize = snap.sections.iter().map(|s| s.text.len()).sum();
    // Up to 2.0 pts for text length (saturates at ~5000 chars)
    score += (total_text as f64 / 2500.0).min(2.0);

    // --- Structural richness ---
    let headings = snap
        .sections
        .iter()
        .filter(|s| s.kind == SectionKind::Heading)
        .count();
    let code_blocks = snap
        .sections
        .iter()
        .filter(|s| s.kind == SectionKind::CodeBlock)
        .count();
    let lists = snap
        .sections
        .iter()
        .filter(|s| s.kind == SectionKind::List)
        .count();

    // Up to 1.5 pts for rich structure
    score += (headings as f64 * 0.3 + code_blocks as f64 * 0.5 + lists as f64 * 0.2).min(1.5);

    // --- User signals ---
    if snap.selection.is_some() {
        score += 2.0; // user explicitly selected text
    }
    if snap.active_element.is_some() {
        score += 0.5; // user was interacting with a form/input
    }
    if !snap.console_errors.is_empty() {
        score += 1.0; // page has errors — agent might need to debug
    }
    if !snap.network_errors.is_empty() {
        score += 0.5;
    }

    // --- Dwell time (up to 1.5 pts, saturates at 60s) ---
    let dwell_secs = snap.dwell_ms as f64 / 1000.0;
    score += (dwell_secs / 40.0).min(1.5);

    // --- Interaction count (up to 1.0 pt, saturates at 20) ---
    score += (snap.interaction_count as f64 / 20.0).min(1.0);

    score.min(10.0)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::types::*;

    fn make_snap(text_len: usize, has_selection: bool, dwell_ms: u64) -> PageSnapshot {
        let text = "a".repeat(text_len);
        PageSnapshot {
            url: "https://example.com".to_string(),
            title: "Test".to_string(),
            timestamp: 1000,
            content_hash: 0,
            sections: vec![ContentSection {
                kind: SectionKind::Paragraph,
                text,
                depth: None,
                selector: "p".to_string(),
            }],
            metadata: PageMetadata::default(),
            selection: if has_selection {
                Some("selected text".to_string())
            } else {
                None
            },
            active_element: None,
            console_errors: vec![],
            network_errors: vec![],
            visual_rects: std::collections::HashMap::new(),
            storage_state: std::collections::HashMap::new(),
            revision: 0,
            importance_score: 0.0,
            dwell_ms,
            interaction_count: 0,
            iframe_content: vec![],
            truncated: false,
        }
    }

    #[test]
    fn test_selection_boosts_score() {
        let without = make_snap(500, false, 0);
        let with_sel = make_snap(500, true, 0);
        assert!(
            compute_importance(&with_sel) > compute_importance(&without),
            "selection should boost importance"
        );
    }

    #[test]
    fn test_longer_content_scores_higher() {
        let short = make_snap(100, false, 0);
        let long = make_snap(5000, false, 0);
        assert!(compute_importance(&long) > compute_importance(&short));
    }

    #[test]
    fn test_dwell_time_boosts_score() {
        let no_dwell = make_snap(500, false, 0);
        let long_dwell = make_snap(500, false, 60_000);
        assert!(compute_importance(&long_dwell) > compute_importance(&no_dwell));
    }

    #[test]
    fn test_score_capped_at_ten() {
        let mut snap = make_snap(100_000, true, 120_000);
        snap.console_errors = vec!["err".to_string(); 10];
        snap.network_errors = vec!["net".to_string(); 5];
        snap.interaction_count = 100;
        snap.sections.push(ContentSection {
            kind: SectionKind::CodeBlock,
            text: "code".to_string(),
            depth: None,
            selector: "pre".to_string(),
        });
        let score = compute_importance(&snap);
        assert!(score <= 10.0, "score should be capped at 10, got {}", score);
    }

    #[test]
    fn test_code_blocks_boost_score() {
        let plain = make_snap(500, false, 0);
        let mut rich = make_snap(500, false, 0);
        for _ in 0..3 {
            rich.sections.push(ContentSection {
                kind: SectionKind::CodeBlock,
                text: "fn main() {}".to_string(),
                depth: None,
                selector: "pre".to_string(),
            });
        }
        assert!(compute_importance(&rich) > compute_importance(&plain));
        let _ = plain;
    }
}
