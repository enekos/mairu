package codetools

import (
	"testing"

	"mairu/internal/ast"
)

func TestGetDescriberForKnownExtension(t *testing.T) {
	if GetDescriber("example.go") == nil {
		t.Fatal("expected Go describer for .go file")
	}
	if GetDescriber("example.unknown") != nil {
		t.Fatal("expected nil describer for unknown extension")
	}
}

func TestIsExportedSymbolLanguageRules(t *testing.T) {
	if !IsExportedSymbol(ast.LogicSymbol{Name: "Foo"}, "go") {
		t.Fatal("expected Go upper-case symbol to be exported")
	}
	if IsExportedSymbol(ast.LogicSymbol{Name: "foo"}, "go") {
		t.Fatal("expected Go lower-case symbol to be unexported")
	}
	if !IsExportedSymbol(ast.LogicSymbol{Name: "public_fn"}, "python") {
		t.Fatal("expected Python non-underscore symbol to be exported")
	}
	if IsExportedSymbol(ast.LogicSymbol{Name: "_private_fn"}, "python") {
		t.Fatal("expected Python underscore symbol to be unexported")
	}
	if !IsExportedSymbol(ast.LogicSymbol{Name: "anything", Exported: true}, "unknown") {
		t.Fatal("expected explicit Exported flag to win")
	}
}
