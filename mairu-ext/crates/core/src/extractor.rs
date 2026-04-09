// mairu-ext/crates/core/src/extractor.rs
use crate::types::{ContentSection, PageMetadata, SectionKind};
use scraper::{ElementRef, Html, Selector};

pub struct ExtractedPage {
    pub title: String,
    pub sections: Vec<ContentSection>,
    pub metadata: PageMetadata,
}

pub fn extract(html: &str) -> ExtractedPage {
    let document = Html::parse_document(html);
    let mut metadata = PageMetadata::default();
    let mut sections = Vec::new();

    // 1. Extract Metadata
    let title = extract_metadata(&document, &mut metadata);

    // 2. Identify the main content root
    let root = find_main_content(&document);

    // 3. Traverse and extract sections
    if let Some(root_node) = root {
        extract_from_node(root_node, &mut sections);
    }

    ExtractedPage {
        title,
        sections,
        metadata,
    }
}

fn extract_metadata(doc: &Html, meta: &mut PageMetadata) -> String {
    let mut title = String::new();

    if let Ok(sel) = Selector::parse("title") {
        if let Some(el) = doc.select(&sel).next() {
            title = collect_text(el).trim().to_string();
        }
    }

    if let Ok(sel) = Selector::parse("meta[property='og:title'], meta[name='og:title']") {
        if let Some(el) = doc.select(&sel).next() {
            meta.og_title = el.value().attr("content").map(String::from);
        }
    }

    if let Ok(sel) = Selector::parse("meta[property='og:description'], meta[name='description']") {
        if let Some(el) = doc.select(&sel).next() {
            meta.og_description = el.value().attr("content").map(String::from);
        }
    }

    if let Ok(sel) = Selector::parse("html[lang]") {
        if let Some(el) = doc.select(&sel).next() {
            meta.lang = el.value().attr("lang").map(String::from);
        }
    }

    if let Ok(sel) = Selector::parse("link[rel='canonical']") {
        if let Some(el) = doc.select(&sel).next() {
            meta.canonical_url = el.value().attr("href").map(String::from);
        }
    }

    title
}

fn find_main_content(doc: &Html) -> Option<ElementRef> {
    // Priority heuristics to find the real content
    let candidate_selectors = [
        "article",
        "main",
        "[role='main']",
        ".main-content",
        "#main-content",
        ".post-content",
        ".article-content",
        ".content",
        "#content",
    ];

    for selector_str in candidate_selectors {
        if let Ok(sel) = Selector::parse(selector_str) {
            if let Some(el) = doc.select(&sel).next() {
                // Return the first match
                return Some(el);
            }
        }
    }

    // Fallback to body
    if let Ok(sel) = Selector::parse("body") {
        return doc.select(&sel).next();
    }

    // Fallback to root document if body is missing
    Some(doc.root_element())
}

// Nodes we completely ignore
const SKIP_TAGS: &[&str] = &[
    "nav", "footer", "header", "aside", "script", "style", "noscript", "iframe", "svg", "form",
    "button", "img", "canvas", "video", "audio", "map", "menu",
];

// Substrings in id or class that usually indicate boilerplate
const NOISE_CLASSES: &[&str] = &[
    "ad-", "ads", "banner", "sidebar", "menu", "popup", "modal", "cookie", "comments", "share",
    "social", "widget",
];

fn extract_from_node(node: ElementRef, sections: &mut Vec<ContentSection>) {
    let tag = node.value().name();

    // 1. Skip boilerplate tags
    if SKIP_TAGS.contains(&tag) {
        return;
    }

    // 2. Skip noisy classes/ids
    let class_str = node.value().attr("class").unwrap_or("");
    let id_str = node.value().attr("id").unwrap_or("");
    for noise in NOISE_CLASSES {
        if class_str.contains(noise) || id_str.contains(noise) {
            return;
        }
    }

    // 3. Process structural nodes
    match tag {
        "h1" | "h2" | "h3" | "h4" | "h5" | "h6" => {
            let text = collect_text(node).trim().to_string();
            if !text.is_empty() {
                let depth: u8 = tag[1..2].parse().unwrap_or(1);
                sections.push(ContentSection {
                    kind: SectionKind::Heading,
                    text,
                    depth: Some(depth),
                    selector: tag.to_string(),
                });
            }
            // Headings don't typically contain structured blocks we care about traversing deeply
            return;
        }
        "p" | "blockquote" => {
            let text = collect_text(node).trim().to_string();
            if !text.is_empty() {
                sections.push(ContentSection {
                    kind: SectionKind::Paragraph,
                    text,
                    depth: None,
                    selector: tag.to_string(),
                });
            }
            return;
        }
        "pre" => {
            // Check for code inside
            let text = collect_text(node).trim().to_string();
            if !text.is_empty() {
                sections.push(ContentSection {
                    kind: SectionKind::CodeBlock,
                    text,
                    depth: None,
                    selector: "pre".to_string(),
                });
            }
            return;
        }
        "ul" | "ol" => {
            let items = extract_list(node);
            if !items.is_empty() {
                sections.push(ContentSection {
                    kind: SectionKind::List,
                    text: items.join("\n"),
                    depth: None,
                    selector: tag.to_string(),
                });
            }
            return; // don't recurse into children as we already extracted them
        }
        "table" => {
            let markdown_table = extract_table(node);
            if !markdown_table.is_empty() {
                sections.push(ContentSection {
                    kind: SectionKind::Table,
                    text: markdown_table,
                    depth: None,
                    selector: "table".to_string(),
                });
            }
            return;
        }
        "a" => {
            // For block-level link tags containing a lot of text, we can capture them.
            // But usually links are inline. If we hit an <a> directly as a block
            // we will extract it as a Paragraph-like element or Link.
            let href = node.value().attr("href").unwrap_or("").to_string();
            let text = collect_text(node).trim().to_string();
            if !text.is_empty() {
                sections.push(ContentSection {
                    kind: SectionKind::Link,
                    text: format!("{} ({})", text, href),
                    depth: None,
                    selector: "a".to_string(),
                });
            }
            return;
        }
        _ => {}
    }

    // 4. If it's a container (div, article, main, section, span etc), recurse!
    for child in node.children() {
        if let Some(child_el) = ElementRef::wrap(child) {
            extract_from_node(child_el, sections);
        } else if let Some(text_node) = child.value().as_text() {
            // Direct text inside a div/body without a paragraph tag.
            // We shouldn't ignore it if it's substantial.
            let text = text_node.trim();
            if text.len() > 20 {
                // Heuristic: skip short stray text
                // Check if the parent is a paragraph-like container. If we already handled it,
                // we wouldn't be here. So this is stray text in a div.
                sections.push(ContentSection {
                    kind: SectionKind::Paragraph,
                    text: text.to_string(),
                    depth: None,
                    selector: "text".to_string(),
                });
            }
        }
    }
}

// Collects text including children (for p, headings, links)
fn collect_text(node: ElementRef) -> String {
    let mut result = String::new();
    for text in node.text() {
        result.push_str(text);
        // add a space after block elements might be needed, but scraper's .text()
        // just concatenates text nodes. We will pad slightly.
        result.push(' ');
    }
    // Clean up extra whitespace
    result.split_whitespace().collect::<Vec<_>>().join(" ")
}

fn extract_list(node: ElementRef) -> Vec<String> {
    let mut items = Vec::new();
    if let Ok(sel) = Selector::parse("li") {
        for li in node.select(&sel) {
            let text = collect_text(li).trim().to_string();
            if !text.is_empty() {
                items.push(format!("- {}", text));
            }
        }
    }
    items
}

fn extract_table(node: ElementRef) -> String {
    let mut markdown = String::new();
    let tr_sel = Selector::parse("tr").unwrap();
    let th_sel = Selector::parse("th").unwrap();
    let td_sel = Selector::parse("td").unwrap();

    let mut is_first_row = true;
    for tr in node.select(&tr_sel) {
        let mut row_data = Vec::new();

        // Extract headers
        for th in tr.select(&th_sel) {
            row_data.push(collect_text(th).trim().to_string());
        }

        // Extract cells
        for td in tr.select(&td_sel) {
            row_data.push(collect_text(td).trim().to_string());
        }

        if !row_data.is_empty() {
            markdown.push_str("| ");
            markdown.push_str(&row_data.join(" | "));
            markdown.push_str(" |\n");

            if is_first_row {
                markdown.push_str("|");
                for _ in 0..row_data.len() {
                    markdown.push_str("---|");
                }
                markdown.push('\n');
                is_first_row = false;
            }
        }
    }
    markdown.trim().to_string()
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
        let headings: Vec<_> = result
            .sections
            .iter()
            .filter(|s| s.kind == SectionKind::Heading)
            .collect();
        assert_eq!(headings.len(), 2);
        assert_eq!(headings[0].text, "Main Title");
        assert_eq!(headings[0].depth, Some(1));
        assert_eq!(headings[1].text, "Sub Title");
        assert_eq!(headings[1].depth, Some(2));
    }

    #[test]
    fn test_extract_paragraphs() {
        let html = r#"<html><body><p>First paragraph.</p><p>Second paragraph.</p></body></html>"#;
        let result = extract(html);
        let paras: Vec<_> = result
            .sections
            .iter()
            .filter(|s| s.kind == SectionKind::Paragraph)
            .collect();
        assert_eq!(paras.len(), 2);
        assert_eq!(paras[0].text, "First paragraph.");
    }

    #[test]
    fn test_extract_code_blocks() {
        let html = r#"<html><body><pre><code>fn main() {}</code></pre></body></html>"#;
        let result = extract(html);
        let codes: Vec<_> = result
            .sections
            .iter()
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
        assert!(texts.iter().any(|t| t.contains("Content here.")));
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
            <html lang="en"></html>
            <link rel="canonical" href="https://example.com/page">
        </head><body><p>Text</p></body></html>"#;
        let result = extract(html);
        assert_eq!(result.metadata.og_title.as_deref(), Some("OG Title"));
        assert_eq!(result.metadata.og_description.as_deref(), Some("OG Desc"));
        assert_eq!(
            result.metadata.canonical_url.as_deref(),
            Some("https://example.com/page")
        );
        assert_eq!(result.metadata.lang.as_deref(), Some("en"));
    }

    #[test]
    fn test_extract_lists() {
        let html = r#"<html><body><ul><li>Item 1</li><li>Item 2</li></ul></body></html>"#;
        let result = extract(html);
        let lists: Vec<_> = result
            .sections
            .iter()
            .filter(|s| s.kind == SectionKind::List)
            .collect();
        assert_eq!(lists.len(), 1);
        assert!(lists[0].text.contains("Item 1"));
        assert!(lists[0].text.contains("Item 2"));
    }

    #[test]
    fn test_semantic_main_extraction() {
        let html = r#"<html><body>
            <div class="sidebar">Sidebar crap</div>
            <main>
                <h1>Real Title</h1>
                <p>Real content</p>
            </main>
            <div class="ad-banner">Buy now!</div>
        </body></html>"#;
        let result = extract(html);
        let texts: Vec<_> = result.sections.iter().map(|s| &s.text).collect();
        assert!(texts.iter().any(|t| t.contains("Real Title")));
        assert!(texts.iter().any(|t| t.contains("Real content")));
        assert!(!texts.iter().any(|t| t.contains("Sidebar crap")));
        assert!(!texts.iter().any(|t| t.contains("Buy now!")));
    }

    #[test]
    fn test_extract_tables() {
        let html = r#"<html><body>
            <table>
                <tr><th>Name</th><th>Age</th></tr>
                <tr><td>Alice</td><td>30</td></tr>
                <tr><td>Bob</td><td>25</td></tr>
            </table>
        </body></html>"#;
        let result = extract(html);
        let tables: Vec<_> = result
            .sections
            .iter()
            .filter(|s| s.kind == SectionKind::Table)
            .collect();
        assert_eq!(tables.len(), 1);
        assert!(tables[0].text.contains("| Name | Age |"));
        assert!(tables[0].text.contains("|---|---|"));
        assert!(tables[0].text.contains("| Alice | 30 |"));
        assert!(tables[0].text.contains("| Bob | 25 |"));
    }
}
