package ast

import (
	"context"
	"strings"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

func parseStatement(source string) (*sitter.Node, []byte) {
	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(typescript.GetLanguage())
	fullSource := "function f() { " + source + " }"
	srcBytes := []byte(fullSource)
	tree, _ := parser.ParseCtx(context.Background(), nil, srcBytes)
	root := tree.RootNode()
	fn := root.NamedChild(0)
	body := fn.ChildByFieldName("body")
	if body.NamedChildCount() > 0 {
		return body.NamedChild(0), srcBytes
	}
	return nil, srcBytes
}

func parseExpression(source string) (*sitter.Node, []byte) {
	stmt, src := parseStatement(source + ";")
	if stmt != nil && stmt.Type() == "expression_statement" {
		return stmt.NamedChild(0), src
	}
	return stmt, src
}

func TestDescribeStatement(t *testing.T) {
	tests := []struct {
		code     string
		expected string
	}{
		{"let x = 1;", "Assigns `1` to `x`"},
		{"const y = 2;", "Assigns `2` to `y`"},
		{"var z;", "Declares `z`"},
		{"return x;", "Returns `x`"},
		{"if (x) { return 1; }", "If `x`, Returns `1`"},
		{"if (x) { return 1; } else { return 2; }", "If `x`, Returns `1`. Otherwise, Returns `2`"},
		{"for (let x in y) {}", "Iterates over each key `x` in `y`, does nothing"},
		{"for (const z of arr) {}", "Iterates over each `z` in `arr`, does nothing"},
		{"for (let x of y) {}", "Iterates over each `x` in `y`, does nothing"},
		{"for (let i = 0; i < 10; i++) {}", "Loops with let i = 0; while i < 10; i++: does nothing"},
		{"while (true) {}", "Loops while `true`: does nothing"},
		{"do {} while (true);", "Loops (do-while `true`): does nothing"},
		{"try { foo(); } catch (e) { bar(); }", "Attempts to calling `foo`. If an error occurs (e), calling `bar`"},
		{"try { foo(); } finally { bar(); }", "Attempts to calling `foo`. Finally, calling `bar`"},
		{"throw new Error('msg');", "Throws a `Error` with message `'msg'`"},
		{"switch(x) { case 1: break; default: break; }", "Based on `x`: case 1: `break;`, default: `break;`"},
		{"foo();", "calling `foo`"},
	}

	for _, tt := range tests {
		node, src := parseStatement(tt.code)
		if node == nil {
			t.Fatalf("Failed to parse statement: %s", tt.code)
		}
		actual := describeStatement(node, src)
		if actual != tt.expected {
			t.Errorf("describeStatement(%q):\nExpected: %s\nGot:      %s", tt.code, tt.expected, actual)
		}
	}
}

func TestDescribeCondition(t *testing.T) {
	tests := []struct {
		code     string
		expected string
	}{
		{"!x", "`x` is falsy"},
		{"typeof x === 'string'", "`x` is a string"},
		{"typeof y !== \"number\"", "`y` is not a number"},
		{"x === null", "`x` is null"},
		{"y !== undefined", "`y` is not undefined"},
		{"x instanceof Error", "`x` is an instance of `Error`"},
		{"arr.length > 0", "`arr` is non-empty"},
		{"a > b", "`a` is greater than `b`"},
		{"a < b", "`a` is less than `b`"},
		{"a >= b", "`a` is greater than or equal to `b`"},
		{"a <= b", "`a` is less than or equal to `b`"},
		{"a === b", "`a` equals `b`"},
		{"a !== b", "`a` does not equal `b`"},
		{"(a > b)", "`a` is greater than `b`"},
	}

	for _, tt := range tests {
		node, src := parseExpression(tt.code)
		if node == nil {
			t.Fatalf("Failed to parse expression: %s", tt.code)
		}
		actual := DescribeCondition(node, src)
		if actual != tt.expected {
			t.Errorf("DescribeCondition(%q):\nExpected: %s\nGot:      %s", tt.code, tt.expected, actual)
		}
	}
}

func TestDescribeExpression(t *testing.T) {
	tests := []struct {
		code     string
		expected string
	}{
		{"await foo()", "awaits calling `foo`"},
		{"foo(1, 'a')", "calling `foo` with `1`, `'a'`"},
		{"x = y", "assigning `y` to `x`"},
		{"new Foo(1)", "a new `Foo` with `1`"},
		{"x.y", "`x.y`"},
		{"a + b", "`a` concatenated with `b`"},
		{"a - b", "`a` minus `b`"},
		{"a * b", "`a` times `b`"},
		{"a / b", "`a` divided by `b`"},
		{"a && b", "`a` and `b`"},
		{"a || b", "`a` or `b`"},
		{"`temp`", "a template string ``temp``"},
		{"'str'", "`'str'`"},
		{"null", "`null`"},
		{"myVar", "`myVar`"},
	}

	for _, tt := range tests {
		node, src := parseExpression(tt.code)
		if node == nil {
			t.Fatalf("Failed to parse expression: %s", tt.code)
		}
		actual := DescribeExpression(node, src)
		if actual != tt.expected {
			t.Errorf("DescribeExpression(%q):\nExpected: %s\nGot:      %s", tt.code, tt.expected, actual)
		}
	}
}

func TestDescribeStatements(t *testing.T) {
	code := "function f() { let x = 1; foo(); return x; }"
	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(typescript.GetLanguage())
	src := []byte(code)
	tree, _ := parser.ParseCtx(context.Background(), nil, src)
	fn := tree.RootNode().NamedChild(0)

	actual := DescribeStatements(fn, src)
	expected := "1. Assigns `1` to `x`\n2. calling `foo`\n3. Returns `x`"
	if actual != expected {
		t.Errorf("DescribeStatements():\nExpected:\n%s\nGot:\n%s", expected, actual)
	}
}

func TestDescribeSymbols(t *testing.T) {
	out := DescribeSymbols(
		[]LogicSymbol{{ID: "fn:validate", Name: "validate", Kind: "fn"}},
		[]LogicEdge{{From: "fn:validate", To: "fn:trim", Kind: "call"}},
	)
	if !strings.Contains(out, "validate") || !strings.Contains(out, "Returns") {
		t.Fatalf("unexpected output: %s", out)
	}
}
