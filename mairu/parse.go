package main

import (
	"fmt"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
)

func main() {
	parser := sitter.NewParser()
	parser.SetLanguage(javascript.GetLanguage())
	source := []byte("function* gen() { yield 1; }")
	tree := parser.Parse(nil, source)
	fmt.Println(tree.RootNode().String())
}
