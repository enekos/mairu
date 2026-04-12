// mairu-ext/crates/core/src/extractor.rs
use crate::types::{ContentSection, PageMetadata, SectionKind};
use scraper::{ElementRef, Html, Selector};

#[derive(serde::Serialize)]
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

    // 2. Identify the main content root (fallback to body, then document root)
    let root = Selector::parse("body")
        .ok()
        .and_then(|sel| document.select(&sel).next())
        .or_else(|| Some(document.root_element()));

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
            title = collect_text_trimmed(el);
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


// Nodes we completely ignore
const SKIP_TAGS: &[&str] = &[
    "script", "style", "noscript", "canvas", "map", "template", "object", "embed", "math",
];

fn is_hidden(node: ElementRef) -> bool {
    let style = node.value().attr("style").unwrap_or("");
    let mut display_none = false;
    let mut visibility_hidden = false;
    let mut opacity_zero = false;
    for decl in style.split(';') {
        let decl = decl.trim();
        if let Some((prop, val)) = decl.split_once(':') {
            let prop = prop.trim();
            let val = val.trim();
            if prop == "display" && val == "none" {
                display_none = true;
            } else if prop == "visibility" && val == "hidden" {
                visibility_hidden = true;
            } else if prop == "opacity" && val == "0" {
                opacity_zero = true;
            }
        }
    }
    if display_none || visibility_hidden || opacity_zero {
        return true;
    }

    if node.value().attr("aria-hidden") == Some("true") {
        return true;
    }

    if node.value().attr("hidden").is_some() {
        return true;
    }

    if node.value().name() == "dialog" && node.value().attr("open").is_none() {
        return true;
    }

    false
}

fn extract_from_node(node: ElementRef, sections: &mut Vec<ContentSection>) {
    let tag = node.value().name();

    // 1. Skip boilerplate tags
    if SKIP_TAGS.contains(&tag) {
        return;
    }

    // 2. Skip explicitly hidden elements
    if is_hidden(node) {
        return;
    }

    // 3. Process structural nodes
    let role = node.value().attr("role").unwrap_or("");

    // Special handling for accessible roles that mimic structural elements
    if role == "button" && tag != "button" {
        let text = collect_text_trimmed(node);
        if !text.is_empty() {
            sections.push(ContentSection {
                kind: SectionKind::Paragraph,
                text: format!("[Button: {}]", text),
                depth: None,
                selector: format!("{}[role=button]", tag),
            });
        }
        return;
    }

    if role == "link" && tag != "a" {
        let text = collect_text_trimmed(node);
        if !text.is_empty() {
            sections.push(ContentSection {
                kind: SectionKind::Link,
                text: format!("{} (link role)", text),
                depth: None,
                selector: format!("{}[role=link]", tag),
            });
        }
        return;
    }

    match tag {
        "h1" | "h2" | "h3" | "h4" | "h5" | "h6" => {
            let text = collect_text_trimmed(node);
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
        "summary" | "legend" | "figcaption" => {
            let text = collect_text_trimmed(node);
            if !text.is_empty() {
                sections.push(ContentSection {
                    kind: SectionKind::Heading,
                    text: format!("[{}: {}]", tag, text),
                    depth: None,
                    selector: tag.to_string(),
                });
            }
            return;
        }
        "hr" => {
            sections.push(ContentSection {
                kind: SectionKind::Paragraph,
                text: "---".to_string(),
                depth: None,
                selector: "hr".to_string(),
            });
            return;
        }
        "p" | "blockquote" => {
            let text = collect_text_trimmed(node);
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
            let text = collect_text_trimmed(node);
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
        "dl" => {
            let items = extract_definition_list(node);
            if !items.is_empty() {
                sections.push(ContentSection {
                    kind: SectionKind::List,
                    text: items.join("\n"),
                    depth: None,
                    selector: "dl".to_string(),
                });
            }
            return;
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
            let text = collect_text_trimmed(node);
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
        "img" => {
            let alt = node.value().attr("alt").unwrap_or("");
            let aria_label = node.value().attr("aria-label").unwrap_or("");

            let mut text = String::new();
            if !alt.is_empty() {
                text.push_str(&format!("[Image: {}]", alt));
            } else if !aria_label.is_empty() {
                text.push_str(&format!("[Image: {}]", aria_label));
            }

            if !text.is_empty() {
                sections.push(ContentSection {
                    kind: SectionKind::Paragraph,
                    text,
                    depth: None,
                    selector: "img".to_string(),
                });
            }
            return;
        }
        "button" => {
            let text = collect_text_trimmed(node);
            if !text.is_empty() {
                sections.push(ContentSection {
                    kind: SectionKind::Paragraph,
                    text: format!("[Button: {}]", text),
                    depth: None,
                    selector: "button".to_string(),
                });
            }
            return;
        }
        "input" | "textarea" | "select" => {
            if tag == "input" && node.value().attr("type") == Some("hidden") {
                return;
            }

            let name = node.value().attr("name").unwrap_or("");
            let id = node.value().attr("id").unwrap_or("");
            let placeholder = node.value().attr("placeholder").unwrap_or("");
            let value = node.value().attr("value").unwrap_or("");

            let mut desc = vec![];
            if !name.is_empty() {
                desc.push(format!("name=\"{}\"", name));
            }
            if !id.is_empty() {
                desc.push(format!("id=\"{}\"", id));
            }
            if !placeholder.is_empty() {
                desc.push(format!("placeholder=\"{}\"", placeholder));
            }
            if !value.is_empty() {
                desc.push(format!("value=\"{}\"", value));
            }

            let attrs = if desc.is_empty() {
                String::new()
            } else {
                format!(" {}", desc.join(" "))
            };

            sections.push(ContentSection {
                kind: SectionKind::Paragraph,
                text: format!("[Form Element: {}{}]", tag, attrs),
                depth: None,
                selector: tag.to_string(),
            });
            return;
        }
        "iframe" => {
            let src = node.value().attr("src").unwrap_or("");
            let title = node.value().attr("title").unwrap_or("");
            if !src.is_empty() {
                let text = if !title.is_empty() {
                    format!("[Iframe: {}]({})", title, src)
                } else {
                    format!("[Iframe]({})", src)
                };
                sections.push(ContentSection {
                    kind: SectionKind::Paragraph,
                    text,
                    depth: None,
                    selector: "iframe".to_string(),
                });
            }
            return;
        }
        "video" | "audio" => {
            let src = node.value().attr("src").unwrap_or("");
            let aria_label = node.value().attr("aria-label").unwrap_or("");
            let title = node.value().attr("title").unwrap_or("");

            let mut desc = vec![];
            if !aria_label.is_empty() {
                desc.push(aria_label);
            } else if !title.is_empty() {
                desc.push(title);
            }

            let label = if desc.is_empty() { tag } else { desc[0] };

            let text = if !src.is_empty() {
                format!("[{}: {}]({})", tag, label, src)
            } else {
                format!("[{}: {}]", tag, label)
            };

            sections.push(ContentSection {
                kind: SectionKind::Paragraph,
                text,
                depth: None,
                selector: tag.to_string(),
            });
            return;
        }
        "svg" => {
            let aria_label = node.value().attr("aria-label").unwrap_or("");
            let mut title_text = String::new();
            if let Ok(sel) = Selector::parse("title") {
                if let Some(t) = node.select(&sel).next() {
                    title_text = collect_text_trimmed(t);
                }
            }

            let label = if !aria_label.is_empty() {
                aria_label
            } else if !title_text.is_empty() {
                &title_text
            } else {
                ""
            };

            if !label.is_empty() {
                sections.push(ContentSection {
                    kind: SectionKind::Paragraph,
                    text: format!("[Icon: {}]", label),
                    depth: None,
                    selector: "svg".to_string(),
                });
            }
            return;
        }
        "form" => {
            sections.push(ContentSection {
                kind: SectionKind::Paragraph,
                text: "[Form Start]".to_string(),
                depth: None,
                selector: "form".to_string(),
            });
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

    for child in node.children() {
        if let Some(child_el) = ElementRef::wrap(child) {
            let tag = child_el.value().name();
            if tag == "a" {
                let href = child_el.value().attr("href").unwrap_or("").to_string();
                let text = collect_text_trimmed(child_el);
                if !text.is_empty() {
                    result.push_str(&format!(" [{}]({}) ", text, href));
                }
            } else if tag == "img" {
                let alt = child_el.value().attr("alt").unwrap_or("");
                let aria_label = child_el.value().attr("aria-label").unwrap_or("");
                if !alt.is_empty() {
                    result.push_str(&format!(" [Image: {}] ", alt));
                } else if !aria_label.is_empty() {
                    result.push_str(&format!(" [Image: {}] ", aria_label));
                }
            } else if tag == "br" {
                result.push('\n');
            } else if tag == "hr" {
                result.push_str("\n---\n");
            } else if tag == "del" || tag == "s" || tag == "strike" {
                let text = collect_text_trimmed(child_el);
                if !text.is_empty() {
                    result.push_str(&format!(" ~{}~ ", text));
                }
            } else if tag == "svg" {
                let aria_label = child_el.value().attr("aria-label").unwrap_or("");
                let mut title_text = String::new();
                if let Ok(sel) = Selector::parse("title") {
                    if let Some(t) = child_el.select(&sel).next() {
                        title_text = collect_text_trimmed(t);
                    }
                }
                let label = if !aria_label.is_empty() {
                    aria_label
                } else if !title_text.is_empty() {
                    &title_text
                } else {
                    ""
                };
                if !label.is_empty() {
                    result.push_str(&format!(" [Icon: {}] ", label));
                }
            } else {
                result.push_str(&collect_text(child_el));
            }
        } else if let Some(text_node) = child.value().as_text() {
            result.push_str(text_node);
        }
    }

    let mut cleaned = result.split_whitespace().collect::<Vec<_>>().join(" ");

    // Fallback to accessibility attributes if text is empty
    if cleaned.is_empty() {
        if let Some(aria_label) = node.value().attr("aria-label") {
            cleaned = aria_label.to_string();
        } else if let Some(alt) = node.value().attr("alt") {
            cleaned = alt.to_string();
        } else if let Some(title) = node.value().attr("title") {
            cleaned = title.to_string();
        }
    }

    cleaned
}

fn collect_text_trimmed(node: ElementRef) -> String {
    collect_text(node).trim().to_string()
}

fn extract_list(node: ElementRef) -> Vec<String> {
    fn extract_list_recursive(node: ElementRef, depth: usize) -> Vec<String> {
        let mut items = Vec::new();
        let indent = "  ".repeat(depth);

        for child in node.children() {
            if let Some(child_el) = ElementRef::wrap(child) {
                let tag = child_el.value().name();
                if tag == "li" {
                    // Extract text directly within the li (excluding nested lists initially)
                    let mut text = String::new();
                    let mut nested_items = Vec::new();

                    for li_child in child_el.children() {
                        if let Some(li_child_el) = ElementRef::wrap(li_child) {
                            let child_tag = li_child_el.value().name();
                            if child_tag == "ul" || child_tag == "ol" {
                                nested_items.extend(extract_list_recursive(li_child_el, depth + 1));
                            } else {
                                text.push_str(&collect_text(li_child_el));
                            }
                        } else if let Some(text_node) = li_child.value().as_text() {
                            text.push_str(text_node);
                        }
                    }

                    let cleaned_text = text.trim();
                    if !cleaned_text.is_empty() {
                        items.push(format!("{}- {}", indent, cleaned_text));
                    }
                    items.extend(nested_items);
                } else if tag == "ul" || tag == "ol" {
                    items.extend(extract_list_recursive(child_el, depth));
                }
            }
        }
        items
    }

    extract_list_recursive(node, 0)
}

fn extract_definition_list(node: ElementRef) -> Vec<String> {
    let mut items = Vec::new();
    for child in node.children() {
        if let Some(child_el) = ElementRef::wrap(child) {
            let tag = child_el.value().name();
            if tag == "dt" {
                let text = collect_text_trimmed(child_el);
                if !text.is_empty() {
                    items.push(format!("- **{}**", text));
                }
            } else if tag == "dd" {
                let text = collect_text_trimmed(child_el);
                if !text.is_empty() {
                    items.push(format!("  {}", text));
                }
            } else if tag == "div" {
                // Some modern DLs use div wrappers for styling
                items.extend(extract_definition_list(child_el));
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
            row_data.push(collect_text_trimmed(th));
        }

        // Extract cells
        for td in tr.select(&td_sel) {
            row_data.push(collect_text_trimmed(td));
        }

        if !row_data.is_empty() {
            markdown.push_str("| ");
            markdown.push_str(&row_data.join(" | "));
            markdown.push_str(" |\n");

            if is_first_row {
                markdown.push('|');
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
    fn test_does_not_strip_nav_and_footer() {
        let html = r#"<html><body>
            <nav><a href="/">Home</a></nav>
            <main><p>Content here.</p></main>
            <footer><p>Copyright 2026</p></footer>
        </body></html>"#;
        let result = extract(html);
        let texts: Vec<_> = result.sections.iter().map(|s| &s.text).collect();
        assert!(texts.iter().any(|t| t.contains("Content here.")));
        // We now EXPECT these to be present so the agent can see them!
        assert!(texts.iter().any(|t| t.contains("Copyright")));
        assert!(texts.iter().any(|t| t.contains("Home")));
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
    fn test_semantic_main_extraction_disabled() {
        // We disabled aggressive main extraction and noise classes
        // so the agent can see the whole page.
        let html = r#"<html><body>
            <div class="sidebar">Sidebar crap that is very long indeed</div>
            <main>
                <h1>Real Title</h1>
                <p>Real content</p>
            </main>
            <div class="ad-banner">Buy now! Limited time offer!</div>
        </body></html>"#;
        let result = extract(html);
        let texts: Vec<_> = result.sections.iter().map(|s| &s.text).collect();
        assert!(texts.iter().any(|t| t.contains("Real Title")));
        assert!(texts.iter().any(|t| t.contains("Real content")));
        // We now EXPECT these to be present
        assert!(texts.iter().any(|t| t.contains("Sidebar crap")));
        assert!(texts.iter().any(|t| t.contains("Buy now!")));
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

    #[test]
    fn test_extract_form_elements() {
        let html = r#"<html><body>
            <form>
                <input name="username" placeholder="Enter Username">
                <input type="password" id="pass">
                <select name="role"></select>
                <textarea id="bio">My bio</textarea>
                <button>Submit</button>
            </form>
        </body></html>"#;
        let result = extract(html);
        let texts: Vec<_> = result.sections.iter().map(|s| s.text.as_str()).collect();

        assert!(texts.contains(&"[Form Start]"));
        assert!(texts.iter().any(|t| t
            .contains("[Form Element: input name=\"username\" placeholder=\"Enter Username\"]")));
        assert!(texts
            .iter()
            .any(|t| t.contains("[Form Element: input id=\"pass\"]")));
        assert!(texts
            .iter()
            .any(|t| t.contains("[Form Element: select name=\"role\"]")));
        assert!(texts
            .iter()
            .any(|t| t.contains("[Form Element: textarea id=\"bio\"]")));
        assert!(texts.iter().any(|t| t.contains("[Button: Submit]")));
    }

    #[test]
    fn test_ignores_hidden_elements() {
        let html = r#"<html><body>
            <div style="display: none">Hidden 1</div>
            <div style="visibility:hidden;">Hidden 2</div>
            <div style="display: block"><p>Visible</p></div>
        </body></html>"#;
        let result = extract(html);
        let texts: Vec<_> = result.sections.iter().map(|s| s.text.as_str()).collect();

        assert!(!texts.iter().any(|t| t.contains("Hidden 1")));
        assert!(!texts.iter().any(|t| t.contains("Hidden 2")));
        assert!(texts.iter().any(|t| t.contains("Visible")));
    }
}
