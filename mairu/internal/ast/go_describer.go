package ast

import (
	"context"
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

type GoDescriber struct{}

func (d GoDescriber) LanguageID() string   { return "go" }
func (d GoDescriber) Extensions() []string { return []string{".go"} }
func (d GoDescriber) ExtractFileGraph(_ string, source string) FileGraph {
	sourceBytes := []byte(source)
	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(golang.GetLanguage())
	tree, _ := parser.ParseCtx(context.Background(), nil, sourceBytes)
	if tree != nil {
		defer tree.Close()
	}
	root := tree.RootNode()

	symbols := []LogicSymbol{}
	edgesMap := make(map[string]LogicEdge)
	symbolDescriptions := make(map[string]string)

	addEdge := func(edge LogicEdge) {
		k := edge.Kind + "|" + edge.From + "|" + edge.To
		if _, ok := edgesMap[k]; !ok {
			edgesMap[k] = edge
		}
	}

	imports := []string{}

	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)

		if child.Type() == "import_declaration" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				spec := child.NamedChild(j)
				if spec.Type() == "import_spec" {
					pathNode := spec.ChildByFieldName("path")
					if pathNode != nil {
						imports = append(imports, stripQuotes(pathNode.Content(sourceBytes)))
					}
				}
			}
		} else if child.Type() == "function_declaration" || child.Type() == "method_declaration" {
			nameNode := child.ChildByFieldName("name")
			fnName := "anonymous_fn"
			if nameNode != nil {
				fnName = nameNode.Content(sourceBytes)
			}

			kind := "fn"
			if child.Type() == "method_declaration" {
				kind = "mtd"
				// extract receiver to append to name? For now just use fnName.
				rcv := child.ChildByFieldName("receiver")
				if rcv != nil && rcv.NamedChildCount() > 0 {
					param := rcv.NamedChild(0)
					typ := param.ChildByFieldName("type")
					if typ != nil {
						if typ.Type() == "pointer_type" && typ.NamedChildCount() > 0 {
							typ = typ.NamedChild(0)
						}
						fnName = typ.Content(sourceBytes) + "." + fnName
					}
				}
			}

			fnId := kind + ":" + fnName

			// Exported if first letter is upper
			exported := len(fnName) > 0 && fnName[0] >= 'A' && fnName[0] <= 'Z'
			if kind == "mtd" {
				parts := strings.Split(fnName, ".")
				if len(parts) == 2 && len(parts[1]) > 0 && parts[1][0] >= 'A' && parts[1][0] <= 'Z' {
					exported = true
				} else {
					exported = false
				}
			}

			sym := LogicSymbol{
				ID:          fnId,
				Kind:        kind,
				Name:        fnName,
				Line:        int(child.StartPoint().Row) + 1,
				Exported:    exported,
				ControlFlow: SummarizeControlFlow(child, sourceBytes),
			}

			symbolDescriptions[fnId] = "Describes " + fnName
			symbols = append(symbols, sym)
		} else if child.Type() == "type_declaration" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				typeSpec := child.NamedChild(j)
				if typeSpec.Type() == "type_spec" {
					nameNode := typeSpec.ChildByFieldName("name")
					if nameNode != nil {
						name := nameNode.Content(sourceBytes)
						exported := len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z'

						symbols = append(symbols, LogicSymbol{
							ID:       "type:" + name,
							Kind:     "type",
							Name:     name,
							Line:     int(typeSpec.StartPoint().Row) + 1,
							Exported: exported,
						})
					}
				}
			}
		}
	}

	// Add call edges
	idsByName := map[string]string{}
	for _, s := range symbols {
		parts := strings.Split(s.Name, ".")
		name := parts[len(parts)-1]
		idsByName[name] = s.ID
	}

	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		if child.Type() == "function_declaration" || child.Type() == "method_declaration" {
			nameNode := child.ChildByFieldName("name")
			fnName := "anonymous_fn"
			if nameNode != nil {
				fnName = nameNode.Content(sourceBytes)
			}
			kind := "fn"
			if child.Type() == "method_declaration" {
				kind = "mtd"
				rcv := child.ChildByFieldName("receiver")
				if rcv != nil && rcv.NamedChildCount() > 0 {
					param := rcv.NamedChild(0)
					typ := param.ChildByFieldName("type")
					if typ != nil {
						if typ.Type() == "pointer_type" && typ.NamedChildCount() > 0 {
							typ = typ.NamedChild(0)
						}
						fnName = typ.Content(sourceBytes) + "." + fnName
					}
				}
			}
			fnId := kind + ":" + fnName

			walkForType(child, "call_expression", func(callExpr *sitter.Node) {
				fn := callExpr.ChildByFieldName("function")
				if fn != nil {
					targetName := fn.Content(sourceBytes)
					if strings.Contains(targetName, ".") {
						parts := strings.Split(targetName, ".")
						targetName = parts[len(parts)-1]
					}
					if targetId, ok := idsByName[targetName]; ok && targetId != fnId {
						addEdge(LogicEdge{Kind: "call", From: fnId, To: targetId})
					}
				}
			})
		}
	}

	var finalEdges []LogicEdge
	for _, v := range edgesMap {
		finalEdges = append(finalEdges, v)
	}

	return FileGraph{
		FileSummary:        fmt.Sprintf("Go file with %d symbols.", len(symbols)),
		Symbols:            SortSymbols(symbols),
		Edges:              SortEdges(finalEdges),
		Imports:            imports,
		SymbolDescriptions: symbolDescriptions,
	}
}
