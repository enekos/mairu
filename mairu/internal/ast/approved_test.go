package ast

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// approvedGraph is the serializable form of FileGraph for approved fixtures.
type approvedGraph struct {
	FileSummary        string            `json:"file_summary"`
	Symbols            []LogicSymbol     `json:"symbols"`
	Edges              []LogicEdge       `json:"edges"`
	Imports            []string          `json:"imports"`
	SymbolDescriptions map[string]string `json:"symbol_descriptions"`
}

func toApproved(g FileGraph) approvedGraph {
	return approvedGraph{
		FileSummary:        g.FileSummary,
		Symbols:            g.Symbols,
		Edges:              g.Edges,
		Imports:            g.Imports,
		SymbolDescriptions: g.SymbolDescriptions,
	}
}

var describers = map[string]LanguageDescriber{
	".ts":  TypeScriptDescriber{},
	".js":  TypeScriptDescriber{},
	".go":  GoDescriber{},
	".py":  PythonDescriber{},
	".vue": VueDescriber{},
	".tsx": TSXDescriber{},
	".jsx": TSXDescriber{},
	".md":  MarkdownDescriber{},
	".mdx": MarkdownDescriber{},
}

// TestExtractFileGraph_Approved runs all testdata/*/*.input.* files through
// the appropriate describer and compares the output against *.approved.json.
//
// Set UPDATE_APPROVED=1 to regenerate the approved files from actual output.
func TestExtractFileGraph_Approved(t *testing.T) {
	entries, err := filepath.Glob("testdata/*/*.input.*")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no testdata/*/*.input.* fixtures found")
	}

	update := os.Getenv("UPDATE_APPROVED") != ""

	for _, inputFile := range entries {
		name := strings.TrimPrefix(inputFile, "testdata/")
		t.Run(name, func(t *testing.T) {
			ext := filepath.Ext(inputFile)
			// Strip .input from the path to get the source extension:
			// e.g., "foo.input.ts" → ext is ".ts" after we parse correctly
			// Actually filepath.Ext gives the last extension, which is what we want.
			describer, ok := describers[ext]
			if !ok {
				t.Skipf("no describer for extension %s", ext)
			}

			src, err := os.ReadFile(inputFile)
			if err != nil {
				t.Fatalf("reading input: %v", err)
			}

			got := describer.ExtractFileGraph(filepath.Base(inputFile), string(src))
			actual := toApproved(got)

			// Normalize nil slices to empty for stable JSON comparison
			if actual.Symbols == nil {
				actual.Symbols = []LogicSymbol{}
			}
			if actual.Edges == nil {
				actual.Edges = []LogicEdge{}
			}
			if actual.Imports == nil {
				actual.Imports = []string{}
			}
			if actual.SymbolDescriptions == nil {
				actual.SymbolDescriptions = map[string]string{}
			}

			approvedFile := toApprovedPath(inputFile)

			if update {
				data, _ := json.MarshalIndent(actual, "", "  ")
				data = append(data, '\n')
				if err := os.WriteFile(approvedFile, data, 0644); err != nil {
					t.Fatalf("writing approved file: %v", err)
				}
				t.Logf("updated %s", approvedFile)
				return
			}

			expectedData, err := os.ReadFile(approvedFile)
			if err != nil {
				t.Fatalf("reading approved file (run with UPDATE_APPROVED=1 to generate): %v", err)
			}

			var expected approvedGraph
			if err := json.Unmarshal(expectedData, &expected); err != nil {
				t.Fatalf("parsing approved file: %v", err)
			}

			// Compare via re-serialized JSON for stable ordering
			actualJSON, _ := json.MarshalIndent(actual, "", "  ")
			expectedJSON, _ := json.MarshalIndent(expected, "", "  ")

			if string(actualJSON) != string(expectedJSON) {
				t.Errorf("output differs from approved.\n\nExpected:\n%s\n\nActual:\n%s", string(expectedJSON), string(actualJSON))
			}
		})
	}
}

// TestNLDescribe_Approved runs all testdata/nl/*.input.ts files through
// the TypeScript parser and compares DescribeStatements output against *.approved.md.
//
// Set UPDATE_APPROVED=1 to regenerate.
func TestNLDescribe_Approved(t *testing.T) {
	entries, err := filepath.Glob("testdata/nl/*.input.ts")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Skip("no testdata/nl/*.input.ts fixtures found")
	}

	update := os.Getenv("UPDATE_APPROVED") != ""

	for _, inputFile := range entries {
		name := filepath.Base(inputFile)
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(inputFile)
			if err != nil {
				t.Fatalf("reading input: %v", err)
			}

			graph := TypeScriptDescriber{}.ExtractFileGraph(name, string(src))

			// Build the approved markdown: one section per symbol with NL description
			var sections []string
			keys := sortedKeys(graph.SymbolDescriptions)
			for _, id := range keys {
				desc := graph.SymbolDescriptions[id]
				sym := findSymbol(graph.Symbols, id)
				header := id
				if sym != nil {
					header = sym.Name + " (" + sym.Kind + ")"
				}
				sections = append(sections, "## "+header+"\n\n"+desc)
			}
			actualOutput := strings.Join(sections, "\n\n") + "\n"

			approvedFile := strings.Replace(inputFile, ".input.ts", ".approved.md", 1)

			if update {
				if err := os.WriteFile(approvedFile, []byte(actualOutput), 0644); err != nil {
					t.Fatalf("writing approved file: %v", err)
				}
				t.Logf("updated %s", approvedFile)
				return
			}

			expectedData, err := os.ReadFile(approvedFile)
			if err != nil {
				t.Fatalf("reading approved file (run with UPDATE_APPROVED=1 to generate): %v", err)
			}

			if string(expectedData) != actualOutput {
				t.Errorf("NL output differs from approved.\n\nExpected:\n%s\n\nActual:\n%s", string(expectedData), actualOutput)
			}
		})
	}
}

// toApprovedPath converts "testdata/ts/foo.input.ts" → "testdata/ts/foo.approved.json"
func toApprovedPath(inputFile string) string {
	dir := filepath.Dir(inputFile)
	base := filepath.Base(inputFile)
	// Remove ".input.ext" and replace with ".approved.json"
	parts := strings.SplitN(base, ".input.", 2)
	if len(parts) != 2 {
		return inputFile + ".approved.json"
	}
	return filepath.Join(dir, parts[0]+".approved.json")
}

// TestNLDescribe_QualityGuard is the Canary in the Code Mine: it fires when the
// NL describer silently drops or degrades descriptions for function symbols.
//
// It checks three invariants across all NL test fixtures:
//  1. Every function/method symbol has an entry in SymbolDescriptions.
//  2. No description is suspiciously short (< 10 chars — likely a stub).
//  3. The description count for fn/method symbols matches the symbol count
//     (no silent gaps).
func TestNLDescribe_QualityGuard(t *testing.T) {
	entries, err := filepath.Glob("testdata/nl/*.input.ts")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Skip("no testdata/nl/*.input.ts fixtures found")
	}

	for _, inputFile := range entries {
		name := filepath.Base(inputFile)
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(inputFile)
			if err != nil {
				t.Fatalf("reading input: %v", err)
			}

			graph := TypeScriptDescriber{}.ExtractFileGraph(name, string(src))

			// Count function/method symbols and verify coverage.
			var fnSymbols []LogicSymbol
			for _, sym := range graph.Symbols {
				if sym.Kind == "fn" || sym.Kind == "method" {
					fnSymbols = append(fnSymbols, sym)
				}
			}

			if len(fnSymbols) == 0 {
				t.Skipf("no fn/method symbols in %s", name)
			}

			for _, sym := range fnSymbols {
				desc, ok := graph.SymbolDescriptions[sym.ID]
				if !ok || strings.TrimSpace(desc) == "" {
					t.Errorf("symbol %q (%s) has no description — NL describer may have regressed", sym.Name, sym.Kind)
					continue
				}
				if len(strings.TrimSpace(desc)) < 10 {
					t.Errorf("symbol %q description is suspiciously short (%q) — possible stub or parse failure", sym.Name, desc)
				}
			}

			descCount := 0
			for _, sym := range fnSymbols {
				if _, ok := graph.SymbolDescriptions[sym.ID]; ok {
					descCount++
				}
			}
			if descCount != len(fnSymbols) {
				t.Errorf("%s: %d fn/method symbols but only %d descriptions — %d silent gaps",
					name, len(fnSymbols), descCount, len(fnSymbols)-descCount)
			}
		})
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func findSymbol(symbols []LogicSymbol, id string) *LogicSymbol {
	for i := range symbols {
		if symbols[i].ID == id {
			return &symbols[i]
		}
	}
	return nil
}
