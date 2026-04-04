package ast

import (
	"context"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

func parseTypeScript(source string) FileGraph {
	parser := sitter.NewParser()
	parser.SetLanguage(typescript.GetLanguage())
	tree, _ := parser.ParseCtx(context.Background(), nil, []byte(source))

	symbols := []LogicSymbol{}
	edges := []LogicEdge{}

	queryStr := `
	(function_declaration
		name: (identifier) @fn.name) @fn

	(class_declaration
		name: (identifier) @cls.name) @cls

	(method_definition
		name: (property_identifier) @mtd.name) @mtd

	(lexical_declaration
		(variable_declarator
			name: (identifier) @var.name
			value: (arrow_function))) @arrow

	(call_expression
		function: [
			(identifier) @call.name
			(member_expression property: (property_identifier) @call.name)
		]) @call
	`

	q, _ := sitter.NewQuery([]byte(queryStr), typescript.GetLanguage())
	if q == nil {
		return BaseExtract(source)
	}
	qc := sitter.NewQueryCursor()
	qc.Exec(q, tree.RootNode())

	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, cap := range m.Captures {
			name := cap.Node.Content([]byte(source))
			nodeType := q.CaptureNameForId(cap.Index)

			switch nodeType {
			case "fn.name", "arrow":
				symbols = append(symbols, LogicSymbol{
					ID:       "fn:" + name,
					Name:     name,
					Kind:     "fn",
					Exported: true, // simplified
				})
			case "cls.name":
				symbols = append(symbols, LogicSymbol{
					ID:       "cls:" + name,
					Name:     name,
					Kind:     "cls",
					Exported: true,
				})
			case "mtd.name":
				symbols = append(symbols, LogicSymbol{
					ID:       "mtd:" + name, // needs class context but simplified for now
					Name:     name,
					Kind:     "mtd",
					Exported: true,
				})
			}
		}
	}

	// Simplistic edge detection: every call found in the file points from a dummy "file" node
	// In a real implementation we would traverse nodes and keep track of parent function context.
	// We'll leave the deeper tree traversal for a more complete parser.
	qc.Exec(q, tree.RootNode())
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, cap := range m.Captures {
			if q.CaptureNameForId(cap.Index) == "call.name" {
				callName := cap.Node.Content([]byte(source))
				edges = append(edges, LogicEdge{
					From: "file",
					To:   callName,
					Kind: "call",
				})
			}
		}
	}

	// Resolve IDs
	idsByName := map[string]string{}
	for _, s := range symbols {
		idsByName[s.Name] = s.ID
	}

	finalEdges := []LogicEdge{}
	for _, e := range edges {
		if targetID, ok := idsByName[e.To]; ok {
			finalEdges = append(finalEdges, LogicEdge{
				From: e.From,
				To:   targetID,
				Kind: e.Kind,
			})
		}
	}

	return FileGraph{
		FileSummary:        "TypeScript module with extracted symbols.",
		Symbols:            dedupeSymbols(symbols),
		Edges:              dedupeEdges(finalEdges),
		Imports:            []string{}, // Add import extraction logic later
		SymbolDescriptions: map[string]string{},
	}
}
