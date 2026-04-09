use browser_extension_core::chunker::chunk_text;
use browser_extension_core::extractor::extract;
use insta::assert_json_snapshot;
use std::fs;

#[test]
fn test_extractor_approved() {
    insta::glob!("testdata/html/*.input.html", |path| {
        let input = fs::read_to_string(path).unwrap();
        let extracted = extract(&input);

        // We use string_snapshot or json_snapshot. Since ExtractedPage implements Serialize,
        // we can use json_snapshot to store the serialized JSON.
        assert_json_snapshot!(extracted);
    });
}

#[test]
fn test_chunker_approved() {
    insta::glob!("testdata/text/*.input.txt", |path| {
        let input = fs::read_to_string(path).unwrap();
        // Use a consistent max_chars across these tests so snapshots are stable
        let max_chars = 100;
        let chunks = chunk_text(&input, max_chars);

        assert_json_snapshot!(chunks);
    });
}
