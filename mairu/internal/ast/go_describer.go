package ast

import (
	"regexp"
	"sort"
)

var (
	reGoFn     = regexp.MustCompile(`(?m)^func\s+(?:\(\w+\s+\*?\w+\)\s*)?([A-Za-z_]\w*)\s*\(`)
	reGoImport = regexp.MustCompile(`(?m)^\s*(?:import\s+)?["']([^"']+)["']`)
)

type GoDescriber struct{}

func (d GoDescriber) LanguageID() string   { return "go" }
func (d GoDescriber) Extensions() []string { return []string{".go"} }
func (d GoDescriber) ExtractFileGraph(_ string, source string) FileGraph {
	symbols := []LogicSymbol{}
	for _, m := range reGoFn.FindAllStringSubmatch(source, -1) {
		symbols = append(symbols, LogicSymbol{ID: "fn:" + m[1], Name: m[1], Kind: "fn"})
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
	for _, m := range reGoImport.FindAllStringSubmatch(source, -1) {
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
		FileSummary:        "Go file graph extracted.",
		Symbols:            dedupeSymbols(symbols),
		Edges:              dedupeEdges(edges),
		Imports:            imports,
		SymbolDescriptions: descs,
	}
}
