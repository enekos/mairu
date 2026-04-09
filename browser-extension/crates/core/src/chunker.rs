// mairu-ext/crates/core/src/chunker.rs

/// Split text into chunks of approximately `max_chars` characters,
/// breaking at sentence boundaries when possible.
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
        let mut end = (start + max_chars).min(text.len());
        while end < text.len() && !text.is_char_boundary(end) {
            end -= 1;
        }

        if end == text.len() {
            chunks.push(text[start..end].trim().to_string());
            break;
        }

        // Look backward for a sentence boundary (. ! ?)
        let window = &text[start..end];
        let split_at = window
            .rfind(['.', '!', '?'])
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
            assert!(
                chunk.ends_with('.') || chunk.ends_with('!') || chunk.ends_with('?'),
                "Chunk should end at sentence boundary: {:?}",
                chunk
            );
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
