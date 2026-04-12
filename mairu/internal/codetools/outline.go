package codetools

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"mairu/internal/ast"
)

type OutlineOptions struct {
	ExportsOnly bool
	TreeMode    bool
	FullMode    bool
}

type OutlineSymbol struct {
	Kind        string          `json:"kind"`
	Name        string          `json:"name"`
	Line        int             `json:"l"`
	Exported    bool            `json:"exported,omitempty"`
	Signature   string          `json:"sig,omitempty"`
	Description string          `json:"desc,omitempty"`
	Children    []OutlineSymbol `json:"children,omitempty"`
}

type OutlineResult struct {
	File    string          `json:"f"`
	Imports []string        `json:"imports"`
	Symbols []OutlineSymbol `json:"symbols"`
}

func GetDescriber(filePath string) ast.LanguageDescriber {
	ext := strings.ToLower(filepath.Ext(filePath))
	describers := []ast.LanguageDescriber{
		ast.TypeScriptDescriber{},
		ast.TSXDescriber{},
		ast.VueDescriber{},
		ast.SvelteDescriber{},
		ast.GoDescriber{},
		ast.PythonDescriber{},
		ast.PHPDescriber{},
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

func IsExportedSymbol(sym ast.LogicSymbol, langID string) bool {
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

func OutlineFile(file string, opts OutlineOptions) (*OutlineResult, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	describer := GetDescriber(file)
	if describer == nil {
		return nil, os.ErrInvalid
	}

	graph := describer.ExtractFileGraph(file, string(content))

	var symbols []OutlineSymbol
	classChildren := make(map[string][]OutlineSymbol) // className -> methods

	for _, s := range graph.Symbols {
		if !opts.FullMode && (s.Kind == "var" || s.Kind == "const" || s.Kind == "field" || s.Kind == "prop") {
			continue
		}

		exported := IsExportedSymbol(s, describer.LanguageID())

		if opts.ExportsOnly && !exported {
			continue
		}

		os := OutlineSymbol{
			Kind:        s.Kind,
			Name:        s.Name,
			Line:        s.Line,
			Exported:    exported,
			Description: s.ControlFlow,
			Signature:   s.Signature,
		}

		if opts.TreeMode && (s.Kind == "mtd" || s.Kind == "prop" || s.Kind == "field") {
			parts := strings.SplitN(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(s.ID, "mtd:"), "prop:"), "field:"), ".", 2)
			if len(parts) == 2 {
				classChildren[parts[0]] = append(classChildren[parts[0]], os)
				continue
			}
		}

		symbols = append(symbols, os)
	}

	if opts.TreeMode {
		for i, s := range symbols {
			if s.Kind == "cls" {
				if children, ok := classChildren[s.Name]; ok {
					symbols[i].Children = children
				}
			}
		}
	}

	return &OutlineResult{
		File:    file,
		Imports: graph.Imports,
		Symbols: symbols,
	}, nil
}
