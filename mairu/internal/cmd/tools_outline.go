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
}

type outlineSymbol struct {
	Kind     string          `json:"kind"`
	Name     string          `json:"name"`
	Line     int             `json:"l"`
	Exported bool            `json:"exported,omitempty"`
	Children []outlineSymbol `json:"children,omitempty"`
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
			exported := isExportedSymbol(s, describer.LanguageID())

			if outlineExports && !exported {
				continue
			}

			os := outlineSymbol{
				Kind:     s.Kind,
				Name:     s.Name,
				Line:     s.Line,
				Exported: exported,
			}

			if outlineTree && s.Kind == "mtd" {
				// Extract class name from ID like "mtd:ClassName.methodName"
				parts := strings.SplitN(strings.TrimPrefix(s.ID, "mtd:"), ".", 2)
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

		out, _ := json.Marshal(res)
		fmt.Println(string(out))
	},
}
