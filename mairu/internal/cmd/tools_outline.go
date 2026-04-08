package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mairu/internal/ast"

	"github.com/spf13/cobra"
)

func init() {
}

type outlineResult struct {
	F       string   `json:"f"`
	Imports []string `json:"imports"`
	Symbols []string `json:"symbols"`
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

		var syms []string
		for _, s := range graph.Symbols {
			syms = append(syms, fmt.Sprintf("%s: %s", s.Kind, s.Name))
		}

		res := outlineResult{
			F:       file,
			Imports: graph.Imports,
			Symbols: syms,
		}

		out, _ := json.Marshal(res)
		fmt.Println(string(out))
	},
}
