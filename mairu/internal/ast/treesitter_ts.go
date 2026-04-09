package ast

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/typescript/typescript"
)

type CallableNodeRef struct {
	SymbolID  string
	ClassName string
	Node      *sitter.Node
}

func parseTypeScript(source string) FileGraph {
	sourceBytes := []byte(source)
	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(typescript.GetLanguage())
	tree, _ := parser.ParseCtx(context.Background(), nil, sourceBytes)
	if tree != nil {
		defer tree.Close()
	}
	root := tree.RootNode()

	symbols := []LogicSymbol{}
	edgesMap := make(map[string]LogicEdge)
	nameToSymbolIds := make(map[string][]string)
	symbolById := make(map[string]LogicSymbol)
	methodByClassAndName := make(map[string]string)
	methodIdsByName := make(map[string][]string)
	callableNodes := []CallableNodeRef{}
	moduleVariableByName := make(map[string]string)

	pushSymbol := func(sym LogicSymbol) {
		symbols = append(symbols, sym)
		symbolById[sym.ID] = sym
		nameToSymbolIds[sym.Name] = append(nameToSymbolIds[sym.Name], sym.ID)
	}
	addEdge := func(edge LogicEdge) {
		k := edge.Kind + "|" + edge.From + "|" + edge.To
		if _, ok := edgesMap[k]; !ok {
			edgesMap[k] = edge
		}
	}

	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		isExport := child.Type() == "export_statement"
		var declNode *sitter.Node
		if isExport {
			declNode = child.ChildByFieldName("declaration")
			if declNode == nil {
				for j := 0; j < int(child.NamedChildCount()); j++ {
					if isDeclarationType(child.NamedChild(j).Type()) {
						declNode = child.NamedChild(j)
						break
					}
				}
			}
		} else {
			declNode = child
		}

		if declNode == nil {
			if isExport {
				sourceNode := child.ChildByFieldName("source")
				if sourceNode != nil {
					modName := stripQuotes(sourceNode.Content(sourceBytes))
					if modName != "" {
						addEdge(LogicEdge{Kind: "import", From: "file", To: "module:" + modName})
					}
				}
			}
			continue
		}

		switch declNode.Type() {
		case "import_statement":
			sourceNode := declNode.ChildByFieldName("source")
			if sourceNode != nil {
				modName := stripQuotes(sourceNode.Content(sourceBytes))
				if modName != "" {
					addEdge(LogicEdge{Kind: "import", From: "file", To: "module:" + modName})
				}
			}

		case "class_declaration":
			nameNode := declNode.ChildByFieldName("name")
			className := "anonymous_class"
			if nameNode != nil {
				className = nameNode.Content(sourceBytes)
			}
			classId := "cls:" + className
			pushSymbol(LogicSymbol{ID: classId, Kind: "cls", Name: className, Exported: isExport, Doc: extractJsDoc(child, sourceBytes), Line: int(declNode.StartPoint().Row) + 1})

			superClass := getExtendsClause(declNode, sourceBytes)
			if superClass != "" {
				addEdge(LogicEdge{Kind: "extends", From: classId, To: "type:" + superClass})
			}
			for _, impl := range getImplementsClause(declNode, sourceBytes) {
				addEdge(LogicEdge{Kind: "implements", From: classId, To: "type:" + impl})
			}

			for _, method := range getClassMethods(declNode) {
				mNameNode := method.ChildByFieldName("name")
				methodName := ""
				if mNameNode != nil {
					methodName = mNameNode.Content(sourceBytes)
				}
				methodId := "mtd:" + className + "." + methodName
				pushSymbol(LogicSymbol{ID: methodId, Kind: "mtd", Name: methodName, Exported: isExport, Doc: extractJsDoc(method, sourceBytes), Line: int(method.StartPoint().Row) + 1})
				methodByClassAndName[className+"."+methodName] = methodId
				methodIdsByName[methodName] = append(methodIdsByName[methodName], methodId)
				callableNodes = append(callableNodes, CallableNodeRef{SymbolID: methodId, ClassName: className, Node: method})
			}

		case "function_declaration":
			nameNode := declNode.ChildByFieldName("name")
			fnName := "anonymous_fn"
			if nameNode != nil {
				fnName = nameNode.Content(sourceBytes)
			}
			fnId := "fn:" + fnName
			pushSymbol(LogicSymbol{ID: fnId, Kind: "fn", Name: fnName, Exported: isExport, Doc: extractJsDoc(child, sourceBytes), Line: int(declNode.StartPoint().Row) + 1})
			callableNodes = append(callableNodes, CallableNodeRef{SymbolID: fnId, ClassName: "", Node: declNode})

		case "lexical_declaration", "variable_declaration":
			doc := extractJsDoc(child, sourceBytes)
			for j := 0; j < int(declNode.NamedChildCount()); j++ {
				declarator := declNode.NamedChild(j)
				if declarator.Type() != "variable_declarator" {
					continue
				}
				vNameNode := declarator.ChildByFieldName("name")
				variableName := ""
				if vNameNode != nil {
					variableName = vNameNode.Content(sourceBytes)
				}
				symbolId := "var:" + variableName
				pushSymbol(LogicSymbol{ID: symbolId, Kind: "var", Name: variableName, Exported: isExport, Doc: doc, Line: int(declarator.StartPoint().Row) + 1})
				moduleVariableByName[variableName] = symbolId
			}

		case "interface_declaration":
			nameNode := declNode.ChildByFieldName("name")
			name := ""
			if nameNode != nil {
				name = nameNode.Content(sourceBytes)
			}
			pushSymbol(LogicSymbol{ID: "iface:" + name, Kind: "iface", Name: name, Exported: isExport, Doc: extractJsDoc(child, sourceBytes), Line: int(declNode.StartPoint().Row) + 1})

		case "enum_declaration":
			nameNode := declNode.ChildByFieldName("name")
			name := ""
			if nameNode != nil {
				name = nameNode.Content(sourceBytes)
			}
			pushSymbol(LogicSymbol{ID: "enum:" + name, Kind: "enum", Name: name, Exported: isExport, Doc: extractJsDoc(child, sourceBytes), Line: int(declNode.StartPoint().Row) + 1})

		case "type_alias_declaration":
			nameNode := declNode.ChildByFieldName("name")
			name := ""
			if nameNode != nil {
				name = nameNode.Content(sourceBytes)
			}
			pushSymbol(LogicSymbol{ID: "type:" + name, Kind: "type", Name: name, Exported: isExport, Doc: extractJsDoc(child, sourceBytes), Line: int(declNode.StartPoint().Row) + 1})
		}
	}

	for _, callable := range callableNodes {
		walkForType(callable.Node, "call_expression", func(callExpr *sitter.Node) {
			targetId := resolveCallTarget(callExpr, callable.ClassName, methodByClassAndName, methodIdsByName, nameToSymbolIds, symbolById, sourceBytes)
			if targetId != "" {
				addEdge(LogicEdge{Kind: "call", From: callable.SymbolID, To: targetId})
			}
		})
		walkForType(callable.Node, "identifier", func(identifier *sitter.Node) {
			name := identifier.Content(sourceBytes)
			variableId, ok := moduleVariableByName[name]
			if !ok {
				return
			}
			if isDeclarationIdentifier(identifier) {
				return
			}
			isWrite := isWriteIdentifier(identifier)
			k := "read"
			if isWrite {
				k = "write"
			}
			addEdge(LogicEdge{Kind: k, From: callable.SymbolID, To: variableId})
		})
	}

	imports := collectImports(root, sourceBytes)

	var finalEdges []LogicEdge
	for _, v := range edgesMap {
		finalEdges = append(finalEdges, v)
	}

	symbolDescriptions := make(map[string]string)
	for _, callable := range callableNodes {
		symbolDescriptions[callable.SymbolID] = DescribeStatements(callable.Node, sourceBytes)
	}

	for i, sym := range symbols {
		for _, callable := range callableNodes {
			if callable.SymbolID == sym.ID {
				symbols[i].ControlFlow = SummarizeControlFlow(callable.Node, sourceBytes)
				break
			}
		}
	}

	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		if child.Type() == "class_declaration" {
			nameNode := child.ChildByFieldName("name")
			className := "anonymous_class"
			if nameNode != nil {
				className = nameNode.Content(sourceBytes)
			}
			classId := "cls:" + className
			superClass := getExtendsClause(child, sourceBytes)
			extendsStr := ""
			if superClass != "" {
				extendsStr = " extends " + superClass
			}
			var methodNames []string
			for _, m := range getClassMethods(child) {
				n := m.ChildByFieldName("name")
				if n != nil {
					methodNames = append(methodNames, n.Content(sourceBytes))
				}
			}
			symbolDescriptions[classId] = "Class `" + className + "`" + extendsStr + " with methods: " + strings.Join(methodNames, ", ")
		}
	}

	fileSummary := ""
	exportedCount := 0
	var exportedNames []string
	kindCounts := make(map[string]int)
	for _, s := range symbols {
		if s.Exported {
			exportedCount++
			exportedNames = append(exportedNames, s.Name)
			kindCounts[s.Kind]++
		}
	}
	if len(symbols) == 0 {
		fileSummary = "Empty or declaration-free source file."
	} else {
		var kindParts []string
		var kinds []string
		for k := range kindCounts {
			kinds = append(kinds, k)
		}
		sort.Strings(kinds)
		for _, k := range kinds {
			c := kindCounts[k]
			plural := ""
			if c > 1 {
				plural = "s"
			}
			kindParts = append(kindParts, fmt.Sprintf("%d %s%s", c, k, plural))
		}
		fileSummary = fmt.Sprintf("File containing %d exported symbols (%s): %s.", exportedCount, strings.Join(kindParts, ", "), strings.Join(exportedNames, ", "))
	}

	return FileGraph{
		FileSummary:        fileSummary,
		Symbols:            SortSymbols(symbols),
		Edges:              SortEdges(finalEdges),
		Imports:            imports,
		SymbolDescriptions: symbolDescriptions,
	}
}

var (
	reJsDocStar     = regexp.MustCompile(`^\s*\*\s?`)
	reJsDocSentence = regexp.MustCompile(`^(.+?\.)\s`)
)

func extractJsDoc(node *sitter.Node, source []byte) string {
	candidate := node.PrevNamedSibling()
	for candidate != nil && candidate.Type() == "decorator" {
		candidate = candidate.PrevNamedSibling()
	}

	if candidate == nil || candidate.Type() != "comment" {
		return ""
	}

	text := candidate.Content(source)
	if !strings.HasPrefix(text, "/**") {
		return ""
	}

	stripped := strings.TrimPrefix(text, "/**")
	stripped = strings.TrimSpace(stripped)
	stripped = strings.TrimSuffix(stripped, "*/")

	lines := strings.Split(stripped, "\n")
	var cleanedLines []string
	for _, line := range lines {
		cleanedLines = append(cleanedLines, reJsDocStar.ReplaceAllString(line, ""))
	}
	stripped = strings.Join(cleanedLines, " ")
	stripped = strings.TrimSpace(stripped)

	if stripped == "" {
		return ""
	}

	beforeTags := strings.SplitN(stripped, " @", 2)[0]
	beforeTags = strings.TrimSpace(beforeTags)

	matches := reJsDocSentence.FindStringSubmatch(beforeTags + " ")
	firstSentence := beforeTags
	if len(matches) > 1 {
		firstSentence = matches[1]
	}

	if len(firstSentence) > 200 {
		return firstSentence[:200] + "..."
	}
	return firstSentence
}

func isDeclarationType(t string) bool {
	types := map[string]bool{
		"class_declaration": true, "function_declaration": true, "lexical_declaration": true,
		"variable_declaration": true, "interface_declaration": true, "enum_declaration": true,
		"type_alias_declaration": true,
	}
	return types[t]
}

func stripQuotes(s string) string {
	s = strings.TrimPrefix(s, "'")
	s = strings.TrimPrefix(s, "\"")
	s = strings.TrimSuffix(s, "'")
	s = strings.TrimSuffix(s, "\"")
	return s
}

func getExtendsClause(classNode *sitter.Node, source []byte) string {
	for i := 0; i < int(classNode.ChildCount()); i++ {
		child := classNode.Child(i)
		if child.Type() == "extends_clause" {
			if child.NamedChildCount() > 0 {
				return child.NamedChild(0).Content(source)
			}
		}
	}
	return ""
}

func getImplementsClause(classNode *sitter.Node, source []byte) []string {
	var res []string
	for i := 0; i < int(classNode.ChildCount()); i++ {
		child := classNode.Child(i)
		if child.Type() == "implements_clause" {
			for j := 0; j < int(child.NamedChildCount()); j++ {
				res = append(res, child.NamedChild(j).Content(source))
			}
		}
	}
	return res
}

func getClassMethods(classNode *sitter.Node) []*sitter.Node {
	body := classNode.ChildByFieldName("body")
	if body == nil {
		return nil
	}
	var res []*sitter.Node
	for i := 0; i < int(body.NamedChildCount()); i++ {
		c := body.NamedChild(i)
		if c.Type() == "method_definition" {
			res = append(res, c)
		}
	}
	return res
}

func walkForType(node *sitter.Node, t string, cb func(*sitter.Node)) {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == t {
			cb(child)
		}
		walkForType(child, t, cb)
	}
}

func resolveCallTarget(
	callExpr *sitter.Node, callerClassName string,
	methodByClassAndName map[string]string, methodIdsByName map[string][]string,
	nameToSymbolIds map[string][]string, symbolById map[string]LogicSymbol, source []byte,
) string {
	fn := callExpr.ChildByFieldName("function")
	if fn == nil {
		return ""
	}
	if fn.Type() == "identifier" {
		syms := nameToSymbolIds[fn.Content(source)]
		return pickBestCallableSymbolId(syms, symbolById)
	}
	if fn.Type() == "member_expression" {
		prop := fn.ChildByFieldName("property")
		obj := fn.ChildByFieldName("object")
		methodName := ""
		if prop != nil {
			methodName = prop.Content(source)
		}
		if obj != nil && obj.Content(source) == "this" && callerClassName != "" {
			if ownMethod, ok := methodByClassAndName[callerClassName+"."+methodName]; ok {
				return ownMethod
			}
		}
		candidateMethodIds := methodIdsByName[methodName]
		if len(candidateMethodIds) == 1 {
			return candidateMethodIds[0]
		}
	}
	return ""
}

func pickBestCallableSymbolId(symbolIds []string, symbolById map[string]LogicSymbol) string {
	var candidates []LogicSymbol
	for _, id := range symbolIds {
		sym, ok := symbolById[id]
		if ok && (sym.Kind == "fn" || sym.Kind == "mtd") {
			candidates = append(candidates, sym)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	best := candidates[0]
	for _, c := range candidates[1:] {
		k1 := 1
		if best.Kind == "fn" {
			k1 = 0
		}
		k2 := 1
		if c.Kind == "fn" {
			k2 = 0
		}
		if k2 < k1 {
			best = c
		} else if k2 == k1 && c.ID < best.ID {
			best = c
		}
	}
	return best.ID
}

func isDeclarationIdentifier(identifier *sitter.Node) bool {
	parent := identifier.Parent()
	if parent == nil {
		return false
	}
	nameNode := parent.ChildByFieldName("name")
	if nameNode != nil && nameNode.Equal(identifier) {
		t := parent.Type()
		if t == "variable_declarator" || t == "function_declaration" || t == "method_definition" || t == "class_declaration" || t == "interface_declaration" || t == "type_alias_declaration" || t == "enum_declaration" {
			return true
		}
	}
	patternNode := parent.ChildByFieldName("pattern")
	if patternNode != nil && patternNode.Equal(identifier) {
		if parent.Type() == "required_parameter" || parent.Type() == "optional_parameter" {
			return true
		}
	}
	return false
}

func isWriteIdentifier(identifier *sitter.Node) bool {
	parent := identifier.Parent()
	if parent == nil {
		return false
	}
	if parent.Type() == "assignment_expression" || parent.Type() == "augmented_assignment_expression" {
		left := parent.ChildByFieldName("left")
		if left != nil && left.Equal(identifier) {
			return true
		}
	}
	if parent.Type() == "update_expression" {
		arg := parent.ChildByFieldName("argument")
		if arg != nil && arg.Equal(identifier) {
			return true
		}
	}
	return false
}

func collectImports(root *sitter.Node, source []byte) []string {
	modSet := make(map[string]bool)
	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(i)
		if child.Type() == "import_statement" {
			sourceNode := child.ChildByFieldName("source")
			if sourceNode != nil {
				mod := stripQuotes(sourceNode.Content(source))
				if mod != "" {
					modSet[mod] = true
				}
			}
		}
	}
	var res []string
	for k := range modSet {
		res = append(res, k)
	}
	sort.Strings(res)
	return res
}
