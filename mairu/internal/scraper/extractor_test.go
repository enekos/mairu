package scraper

import (
	"strings"
	"testing"
)

func TestExtractContent(t *testing.T) {
	html := `
		<html>
			<head><title>Test Page</title></head>
			<body>
				<h1>Main Heading</h1>
				<p>Some introductory text.</p>
				<h2>Section 1</h2>
				<p>Content for section 1.</p>
				<h3>Subsection 1.1</h3>
				<p>More detailed content.</p>
			</body>
		</html>
	`

	res := ExtractContent(html, "", "")

	if res.Title != "Test Page" && res.Title != "Main Heading" {
		t.Errorf("expected title 'Test Page', got %q", res.Title)
	}

	if res.WordCount == 0 {
		t.Errorf("expected non-zero word count")
	}

	if !strings.Contains(res.Markdown, "Main Heading") && !strings.Contains(res.Markdown, "Some introductory text") {
		t.Errorf("expected markdown to contain content")
	}
}

func TestExtractContentWithSelector(t *testing.T) {
	html := `
		<html>
			<head><title>Test Page</title></head>
			<body>
				<div class="sidebar">Sidebar content</div>
				<div class="content">
					<h2>Main Content</h2>
					<p>This is what we want.</p>
				</div>
			</body>
		</html>
	`

	res := ExtractContent(html, ".content", "")

	if !strings.Contains(res.Markdown, "This is what we want.") {
		t.Errorf("expected markdown to contain target content")
	}
}

func TestSplitSections(t *testing.T) {
	html := `
		<h2>Section 1</h2>
		<p>Content 1</p>
		<h3>Subsection</h3>
		<p>Sub content</p>
	`
	sections := splitSections(html, "")
	if len(sections) != 2 {
		t.Errorf("expected 2 sections, got %d", len(sections))
	} else {
		if sections[0].Heading != "Section 1" {
			t.Errorf("expected 'Section 1', got %q", sections[0].Heading)
		}
		if sections[1].Heading != "Subsection" {
			t.Errorf("expected 'Subsection', got %q", sections[1].Heading)
		}
	}
}
