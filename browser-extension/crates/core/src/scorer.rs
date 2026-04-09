// mairu-ext/crates/core/src/scorer.rs
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

        let mut scores: Vec<(String, f64)> = self
            .docs
            .iter()
            .map(|(doc_id, term_counts)| {
                let total_terms: usize = term_counts.values().sum();
                let score: f64 = query_tokens
                    .iter()
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
