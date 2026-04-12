package ast

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"mairu/internal/approved"
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
	".ts":     TypeScriptDescriber{},
	".js":     TypeScriptDescriber{},
	".go":     GoDescriber{},
	".py":     PythonDescriber{},
	".php":    PHPDescriber{},
	".vue":    VueDescriber{},
	".svelte": SvelteDescriber{},
	".tsx":    TSXDescriber{},
	".jsx":    TSXDescriber{},
	".md":     MarkdownDescriber{},
	".mdx":    MarkdownDescriber{},
}

func TestExtractFileGraph_Approved(t *testing.T) {
	inputs, err := approved.DiscoverInputs("testdata/*/*.input.*")
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) == 0 {
		t.Fatal("no testdata/*/*.input.* fixtures found")
	}

	for _, inputFile := range inputs {
		name := strings.TrimPrefix(inputFile, "testdata/")
		t.Run(name, func(t *testing.T) {
			ext := filepath.Ext(inputFile)
			describer, ok := describers[ext]
			if !ok {
				t.Skipf("no describer for extension %s", ext)
			}

			src, err := os.ReadFile(inputFile)
			if err != nil {
				t.Fatalf("reading input: %v", err)
			}

			got := describer.ExtractFileGraph(filepath.Base(inputFile), string(src))
			approved.AssertJSON(t, approved.MapInputToApprovedJSON(inputFile), toApproved(got))
		})
	}
}

func TestNLDescribe_Approved(t *testing.T) {
	inputs, err := approved.DiscoverInputs("testdata/nl/*.input.ts")
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) == 0 {
		t.Skip("no testdata/nl/*.input.ts fixtures found")
	}

	for _, inputFile := range inputs {
		name := filepath.Base(inputFile)
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(inputFile)
			if err != nil {
				t.Fatalf("reading input: %v", err)
			}

			graph := TypeScriptDescriber{}.ExtractFileGraph(name, string(src))

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

			approved.AssertString(t, approved.MapInputToApprovedMD(inputFile), actualOutput)
		})
	}
}

func TestNLDescribe_QualityGuard(t *testing.T) {
	inputs, err := approved.DiscoverInputs("testdata/nl/*.input.ts")
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) == 0 {
		t.Skip("no testdata/nl/*.input.ts fixtures found")
	}

	for _, inputFile := range inputs {
		name := filepath.Base(inputFile)
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(inputFile)
			if err != nil {
				t.Fatalf("reading input: %v", err)
			}

			graph := TypeScriptDescriber{}.ExtractFileGraph(name, string(src))

			approved.AssertQuality(t, graph.Symbols,
				approved.QualityCheck[LogicSymbol]{
					Name:   "fn/method symbols have descriptions",
					Filter: func(s LogicSymbol) bool { return s.Kind == "fn" || s.Kind == "method" },
					Assert: func(t testing.TB, s LogicSymbol) {
						desc, ok := graph.SymbolDescriptions[s.ID]
						if !ok || strings.TrimSpace(desc) == "" {
							t.Errorf("symbol %q (%s) has no description", s.Name, s.Kind)
							return
						}
						if len(strings.TrimSpace(desc)) < 10 {
							t.Errorf("symbol %q description too short: %q", s.Name, desc)
						}
					},
				},
			)
		})
	}
}

func TestFixtureCompleteness(t *testing.T) {
	missing, err := approved.CheckFixtures(
		approved.FixtureRule{
			InputGlob: "testdata/*/*.input.*",
			MapFunc:   approved.MapInputToApprovedJSON,
		},
		approved.FixtureRule{
			InputGlob: "testdata/nl/*.input.ts",
			MapFunc:   approved.MapInputToApprovedMD,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range missing {
		t.Errorf("missing approved fixture: %s (run UPDATE_APPROVED=1 go test ./internal/ast/... to generate)", m)
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
