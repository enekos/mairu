/// 64-bit SimHash for content fingerprinting.
/// Near-duplicate detection via Hamming distance.
pub fn simhash(text: &str) -> u64 {
    if text.is_empty() {
        return 0;
    }

    let mut counts = [0i32; 64];
    let words: Vec<&str> = text.split_whitespace().collect();

    // Generate shingles (2-grams of words) and hash each
    if words.len() < 2 {
        return words.first().map(|w| hash_token(w)).unwrap_or(0);
    }

    for window in words.windows(2) {
        let shingle = format!("{} {}", window[0], window[1]);
        let h = hash_token(&shingle);
        for (i, count) in counts.iter_mut().enumerate() {
            if (h >> i) & 1 == 1 {
                *count += 1;
            } else {
                *count -= 1;
            }
        }
    }

    let mut result: u64 = 0;
    for (i, &count) in counts.iter().enumerate() {
        if count > 0 {
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
        assert!(
            hamming_distance(h1, h2) < 10,
            "similar texts should have low hamming distance"
        );
    }

    #[test]
    fn test_different_text_high_hamming() {
        let h1 = simhash("the quick brown fox jumps over the lazy dog");
        let h2 = simhash("rust programming language systems memory safety");
        assert!(
            hamming_distance(h1, h2) > 10,
            "different texts should have high hamming distance"
        );
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
