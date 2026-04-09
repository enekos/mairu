package ast

import (
	"regexp"
	"sort"
	"strings"
)

type LogicSymbol struct {
	ID          string
	Name        string
	Kind        string
	Exported    bool
	Doc         string
	Line        int
	Signature   string
	ControlFlow string
}

// LogicEdge represents a relationship between two LogicSymbols, such as a function call.
type LogicEdge struct {
	From string
	To   string
	Kind string
}

// FileGraph is the result of parsing a source file. It contains the symbols,
// their relationships, and human-readable natural language descriptions.
type FileGraph struct {
	FileSummary        string
	RawContent         string // if non-empty, used as-is for the content field instead of DescribeSymbols
	Symbols            []LogicSymbol
	Edges              []LogicEdge
	Imports            []string
	SymbolDescriptions map[string]string
}

// LanguageDescriber is the interface that must be implemented to support AST
// extraction and natural language description generation for a specific programming language.
type LanguageDescriber interface {
	LanguageID() string
	Extensions() []string
	ExtractFileGraph(filePath, source string) FileGraph
}

var (
	reFunc   = regexp.MustCompile(`(?m)(?:export\s+)?function\s+([A-Za-z_]\w*)\s*\(`)
	reClass  = regexp.MustCompile(`(?m)(?:export\s+)?class\s+([A-Za-z_]\w*)`)
	reMethod = regexp.MustCompile(`(?m)^\s*(?:public\s+|private\s+|protected\s+)?([A-Za-z_]\w*)\s*\(`)
	reImport = regexp.MustCompile(`(?m)^\s*import\s+.*?from\s+['"]([^'"]+)['"]`)
	reCalls  = regexp.MustCompile(`([A-Za-z_]\w*)\s*\(`)
)

// BaseExtract provides a fallback regex-based extraction mechanism for languages
// that do not have a robust Tree-sitter implementation yet.
func BaseExtract(source string) FileGraph {
	symbols := []LogicSymbol{}
	lines := strings.Split(source, "\n")
	for i, line := range lines {
		if m := reFunc.FindStringSubmatch(line); m != nil {
			symbols = append(symbols, LogicSymbol{ID: "fn:" + m[1], Name: m[1], Kind: "fn", Exported: true, Line: i + 1})
		}
		if m := reClass.FindStringSubmatch(line); m != nil {
			symbols = append(symbols, LogicSymbol{ID: "cls:" + m[1], Name: m[1], Kind: "cls", Exported: true, Line: i + 1})
		}
	}

	seenClass := ""
	for _, m := range reClass.FindAllStringSubmatch(source, -1) {
		name := m[1]
		seenClass = name
	}

	if seenClass != "" {
		for _, m := range reMethod.FindAllStringSubmatch(source, -1) {
			n := m[1]
			if n == "if" || n == "for" || n == "while" || n == "switch" || n == "catch" || n == "function" || n == "return" {
				continue
			}
			symbols = append(symbols, LogicSymbol{ID: "mtd:" + seenClass + "." + n, Name: n, Kind: "mtd", Exported: true})
		}
	}

	idsByName := map[string]string{}
	for _, s := range symbols {
		idsByName[s.Name] = s.ID
	}
	edges := []LogicEdge{}
	for _, s := range symbols {
		for _, c := range reCalls.FindAllStringSubmatch(source, -1) {
			to := idsByName[c[1]]
			if to != "" && to != s.ID {
				edges = append(edges, LogicEdge{From: s.ID, To: to, Kind: "call"})
			}
		}
	}
	descs := map[string]string{}
	for _, s := range symbols {
		descs[s.ID] = "Describes " + s.Name
	}
	imports := []string{}
	for _, m := range reImport.FindAllStringSubmatch(source, -1) {
		imports = append(imports, m[1])
	}
	sort.Slice(symbols, func(i, j int) bool { return symbols[i].ID < symbols[j].ID })
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From == edges[j].From {
			return edges[i].To < edges[j].To
		}
		return edges[i].From < edges[j].From
	})
	return FileGraph{
		FileSummary:        "File graph extracted.",
		Symbols:            dedupeSymbols(symbols),
		Edges:              dedupeEdges(edges),
		Imports:            imports,
		SymbolDescriptions: descs,
	}
}

func dedupeSymbols(in []LogicSymbol) []LogicSymbol {
	seen := map[string]bool{}
	var out []LogicSymbol
	for _, s := range in {
		if seen[s.ID] {
			continue
		}
		seen[s.ID] = true
		out = append(out, s)
	}
	return out
}

func CompareSymbols(a, b LogicSymbol) bool { return a.ID < b.ID }

func SortSymbols(in []LogicSymbol) []LogicSymbol {
	out := append([]LogicSymbol(nil), in...)
	sort.Slice(out, func(i, j int) bool { return CompareSymbols(out[i], out[j]) })
	return out
}

func SortEdges(in []LogicEdge) []LogicEdge {
	out := append([]LogicEdge(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].From == out[j].From {
			return out[i].To < out[j].To
		}
		return out[i].From < out[j].From
	})
	return out
}

func dedupeEdges(edges []LogicEdge) []LogicEdge {
	seen := map[string]bool{}
	out := []LogicEdge{}
	for _, e := range edges {
		k := e.From + "|" + e.To + "|" + e.Kind
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, e)
	}
	return out
}
