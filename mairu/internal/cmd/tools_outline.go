package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"mairu/internal/ast"

	"github.com/spf13/cobra"
)

var outlineExports bool
var outlineTree bool

func init() {
	outlineCmd.Flags().BoolVar(&outlineExports, "exports", false, "Only show exported/public symbols")
	outlineCmd.Flags().BoolVar(&outlineTree, "tree", false, "Nest methods under their parent class")
	outlineCmd.Flags().BoolVar(&outlineFull, "full", false, "Include variables, fields, and properties in output")
}

var outlineFull bool

type outlineSymbol struct {
	Kind        string          `json:"kind"`
	Name        string          `json:"name"`
	Line        int             `json:"l"`
	Exported    bool            `json:"exported,omitempty"`
	Signature   string          `json:"sig,omitempty"`
	Description string          `json:"desc,omitempty"`
	Children    []outlineSymbol `json:"children,omitempty"`
}

type outlineResult struct {
	F       string          `json:"f"`
	Imports []string        `json:"imports"`
	Symbols []outlineSymbol `json:"symbols"`
}

func getDescriber(filePath string) ast.LanguageDescriber {
	ext := strings.ToLower(filepath.Ext(filePath))
	describers := []ast.LanguageDescriber{
		ast.TypeScriptDescriber{},
		ast.TSXDescriber{},
		ast.VueDescriber{},
		ast.GoDescriber{},
		ast.PythonDescriber{},
		ast.MarkdownDescriber{},
	}
	for _, desc := range describers {
		for _, e := range desc.Extensions() {
			if e == ext {
				return desc
			}
		}
	}
	return nil
}

func isExportedSymbol(sym ast.LogicSymbol, langID string) bool {
	if sym.Exported {
		return true
	}
	switch langID {
	case "go":
		if len(sym.Name) > 0 && unicode.IsUpper(rune(sym.Name[0])) {
			return true
		}
	case "python":
		if !strings.HasPrefix(sym.Name, "_") {
			return true
		}
	}
	return false
}

var outlineCmd = &cobra.Command{
	Use:   "outline <file>",
	Short: "AI-optimized file skeleton (JSON)",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		file := args[0]
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		describer := getDescriber(file)
		if describer == nil {
			fmt.Fprintf(os.Stderr, "error: unsupported file type\n")
			os.Exit(1)
		}

		graph := describer.ExtractFileGraph(file, string(content))

		// Build outline symbols
		var symbols []outlineSymbol
		classChildren := make(map[string][]outlineSymbol) // className -> methods

		for _, s := range graph.Symbols {
			// In non-full mode, skip variables, fields, and imports in symbols array
			if !outlineFull && (s.Kind == "var" || s.Kind == "const" || s.Kind == "field" || s.Kind == "prop") {
				continue
			}

			exported := isExportedSymbol(s, describer.LanguageID())

			if outlineExports && !exported {
				continue
			}

			os := outlineSymbol{
				Kind:        s.Kind,
				Name:        s.Name,
				Line:        s.Line,
				Exported:    exported,
				Description: s.ControlFlow,
			}

			// Add signature if the language describer captured it
			if s.Signature != "" {
				os.Signature = s.Signature
			}

			if outlineTree && (s.Kind == "mtd" || s.Kind == "prop" || s.Kind == "field") {
				// Extract class name from ID like "mtd:ClassName.methodName" or "prop:ClassName.propName"
				parts := strings.SplitN(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(s.ID, "mtd:"), "prop:"), "field:"), ".", 2)
				if len(parts) == 2 {
					classChildren[parts[0]] = append(classChildren[parts[0]], os)
					continue
				}
			}

			symbols = append(symbols, os)
		}

		// Attach children in tree mode
		if outlineTree {
			for i, s := range symbols {
				if s.Kind == "cls" {
					if children, ok := classChildren[s.Name]; ok {
						symbols[i].Children = children
					}
				}
			}
		}

		res := outlineResult{
			F:       file,
			Imports: graph.Imports,
			Symbols: symbols,
		}

		if outputFormat == "json" {
			out, _ := json.Marshal(res)
			fmt.Println(string(out))
		} else {
			f := GetFormatter()
			fmt.Printf("File: %s\n", res.F)
			if len(res.Imports) > 0 {
				fmt.Printf("Imports: %d\n", len(res.Imports))
			}

			if len(res.Symbols) > 0 {
				fmt.Println("\nSymbols:")
				var processSymbols func([]outlineSymbol, string) []map[string]any
				processSymbols = func(syms []outlineSymbol, prefix string) []map[string]any {
					var flat []map[string]any
					for _, s := range syms {
						flat = append(flat, map[string]any{
							"kind": s.Kind,
							"name": prefix + s.Name,
							"line": s.Line,
							"sig":  s.Signature,
							"desc": s.Description,
						})
						if len(s.Children) > 0 {
							flat = append(flat, processSymbols(s.Children, prefix+"  ")...)
						}
					}
					return flat
				}

				items := processSymbols(res.Symbols, "")
				f.PrintItems(
					[]string{"line", "kind", "name", "signature", "description"},
					items,
					func(item map[string]any) map[string]string {
						desc := fmt.Sprintf("%v", item["desc"])
						desc = strings.ReplaceAll(desc, "\n", " ")
						if desc == "<nil>" || desc == "" {
							desc = ""
						} else if len(desc) > 100 {
							desc = desc[:97] + "..."
						}

						return map[string]string{
							"line":        fmt.Sprintf("%v", item["line"]),
							"kind":        fmt.Sprintf("%v", item["kind"]),
							"name":        fmt.Sprintf("%v", item["name"]),
							"signature":   fmt.Sprintf("%v", item["sig"]),
							"description": desc,
						}
					},
				)
			}
		}
	},
}
