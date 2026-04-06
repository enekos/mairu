package crawler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchNode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body>Hello World</body></html>"))
	}))
	defer ts.Close()

	node := &FetchNode{}
	state := State{"url": ts.URL}

	newState, err := node.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("FetchNode failed: %v", err)
	}

	html, ok := newState["html"].(string)
	if !ok {
		t.Fatalf("FetchNode did not return html string")
	}

	if html != "<html><body>Hello World</body></html>" {
		t.Errorf("Unexpected html content: %s", html)
	}
}

func TestParseNodeHTML(t *testing.T) {
	node := &ParseNode{}
	html := `<html><head><title>Test</title></head><body><h1>Header</h1><p>Some paragraph text.</p></body></html>`
	state := State{"html": html, "url": "http://localhost"}

	newState, err := node.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("ParseNode failed: %v", err)
	}

	doc, ok := newState["doc"].(string)
	if !ok {
		t.Fatalf("ParseNode did not return doc string")
	}

	// We expect markdown conversion
	if doc == "" {
		t.Errorf("Doc should not be empty")
	}
}

func TestParseNodeJSONBypass(t *testing.T) {
	node := &ParseNode{}
	jsonContent := `{"name":"test", "value":123}`
	state := State{"html": jsonContent, "url": "http://localhost"}

	newState, err := node.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("ParseNode failed on JSON: %v", err)
	}

	doc, ok := newState["doc"].(string)
	if !ok {
		t.Fatalf("ParseNode did not return doc string")
	}

	if doc != jsonContent {
		t.Errorf("ParseNode modified JSON content")
	}
}

func TestMinifyHTMLNode(t *testing.T) {
	node := &MinifyHTMLNode{}
	html := `<html><head><script>alert('hi');</script><style>body { color: red; }</style></head><body class="test">Hello</body></html>`
	state := State{"html": html}

	newState, err := node.Execute(context.Background(), state)
	if err != nil {
		t.Fatalf("MinifyHTMLNode failed: %v", err)
	}

	minified, ok := newState["minified_html"].(string)
	if !ok {
		t.Fatalf("MinifyHTMLNode did not return minified_html string")
	}

	// Should remove script and style but keep class
	if len(minified) >= len(html) {
		t.Errorf("Minified HTML not smaller: %d vs %d", len(minified), len(html))
	}
}
