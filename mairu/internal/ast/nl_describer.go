package ast

import (
	"fmt"
	"regexp"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

func DescribeStatements(node *sitter.Node, source []byte) string {
	body := node.ChildByFieldName("body")
	if body == nil {
		return "Empty function body."
	}

	// Block body
	if body.Type() == "statement_block" || body.Type() == "block" {
		count := body.NamedChildCount()
		if count == 0 {
			return "Empty function body."
		}
		var lines []string
		for i := 0; i < int(count); i++ {
			stmt := body.NamedChild(i)
			lines = append(lines, fmt.Sprintf("%d. %s", i+1, describeStatement(stmt, source)))
		}
		return strings.Join(lines, "\n")
	}

	// Expression body (arrow function)
	return fmt.Sprintf("1. Returns %s", describeExpression(body, source))
}

func describeStatement(node *sitter.Node, source []byte) string {
	switch node.Type() {
	case "variable_declaration", "lexical_declaration":
		return describeVariableStatement(node, source)
	case "return_statement":
		return describeReturnStatement(node, source)
	case "if_statement":
		return describeIfStatement(node, source)
	case "for_in_statement":
		return describeForInStatement(node, source)
	case "for_statement":
		return describeForStatement(node, source)
	case "while_statement":
		return describeWhileStatement(node, source)
	case "do_statement":
		return describeDoStatement(node, source)
	case "try_statement":
		return describeTryStatement(node, source)
	case "throw_statement":
		return describeThrowStatement(node, source)
	case "switch_statement":
		return describeSwitchStatement(node, source)
	case "expression_statement":
		return describeExpressionStatement(node, source)
	default:
		return fmt.Sprintf("`%s`", strings.TrimSpace(node.Content(source)))
	}
}

func describeVariableStatement(node *sitter.Node, source []byte) string {
	var parts []string
	count := node.NamedChildCount()
	for i := 0; i < int(count); i++ {
		decl := node.NamedChild(i)
		if decl.Type() != "variable_declarator" {
			continue
		}
		nameNode := decl.ChildByFieldName("name")
		valueNode := decl.ChildByFieldName("value")
		name := "unknown"
		if nameNode != nil {
			name = nameNode.Content(source)
		}
		if valueNode != nil {
			parts = append(parts, fmt.Sprintf("Assigns %s to `%s`", describeExpression(valueNode, source), name))
		} else {
			parts = append(parts, fmt.Sprintf("Declares `%s`", name))
		}
	}
	if len(parts) == 0 {
		return "Declares variable"
	}
	return strings.Join(parts, ". ")
}

func describeReturnStatement(node *sitter.Node, source []byte) string {
	if node.NamedChildCount() > 0 {
		child := node.NamedChild(0)
		return fmt.Sprintf("Returns %s", describeExpression(child, source))
	}
	return "Returns"
}

func describeIfStatement(node *sitter.Node, source []byte) string {
	condition := node.ChildByFieldName("condition")
	consequence := node.ChildByFieldName("consequence")
	alternative := node.ChildByFieldName("alternative")

	result := fmt.Sprintf("If %s, %s", describeCondition(unwrapParens(condition), source), describeBlockInline(consequence, source))
	if alternative != nil {
		if alternative.Type() == "if_statement" {
			result += fmt.Sprintf(". Otherwise, %s", describeStatement(alternative, source))
		} else if alternative.Type() == "else_clause" {
			if alternative.NamedChildCount() > 0 {
				elseBody := alternative.NamedChild(0)
				if elseBody.Type() == "if_statement" {
					result += fmt.Sprintf(". Otherwise, %s", describeStatement(elseBody, source))
				} else {
					result += fmt.Sprintf(". Otherwise, %s", describeBlockInline(elseBody, source))
				}
			}
		} else {
			result += fmt.Sprintf(". Otherwise, %s", describeBlockInline(alternative, source))
		}
	}
	return result
}

func describeForInStatement(node *sitter.Node, source []byte) string {
	left := node.ChildByFieldName("left")
	right := node.ChildByFieldName("right")
	body := node.ChildByFieldName("body")

	binding := extractBindingName(left, source)
	iterable := "unknown"
	if right != nil {
		iterable = right.Content(source)
	}

	isForOf := false
	for i := 0; i < int(node.ChildCount()); i++ {
		if node.Child(i).Type() == "of" {
			isForOf = true
			break
		}
	}

	if isForOf {
		return fmt.Sprintf("Iterates over each `%s` in `%s`, %s", binding, iterable, describeBlockInline(body, source))
	}
	return fmt.Sprintf("Iterates over each key `%s` in `%s`, %s", binding, iterable, describeBlockInline(body, source))
}

func describeForStatement(node *sitter.Node, source []byte) string {
	left := node.ChildByFieldName("left")
	right := node.ChildByFieldName("right")
	body := node.ChildByFieldName("body")

	if left != nil && right != nil {
		return fmt.Sprintf("Iterates over each `%s` in `%s`, %s", left.Content(source), right.Content(source), describeBlockInline(body, source))
	}

	initializer := node.ChildByFieldName("initializer")
	condition := node.ChildByFieldName("condition")
	increment := node.ChildByFieldName("increment")

	initText := ""
	if initializer != nil {
		initText = initializer.Content(source)
	}
	condText := ""
	if condition != nil {
		condText = condition.Content(source)
	}
	incText := ""
	if increment != nil {
		incText = increment.Content(source)
	}
	return fmt.Sprintf("Loops with %s; while %s; %s: %s", initText, condText, incText, describeBlockInline(body, source))
}

func describeWhileStatement(node *sitter.Node, source []byte) string {
	condition := node.ChildByFieldName("condition")
	body := node.ChildByFieldName("body")
	condText := ""
	if unwrap := unwrapParens(condition); unwrap != nil {
		condText = unwrap.Content(source)
	}
	return fmt.Sprintf("Loops while `%s`: %s", condText, describeBlockInline(body, source))
}

func describeDoStatement(node *sitter.Node, source []byte) string {
	condition := node.ChildByFieldName("condition")
	body := node.ChildByFieldName("body")
	condText := ""
	if unwrap := unwrapParens(condition); unwrap != nil {
		condText = unwrap.Content(source)
	}
	return fmt.Sprintf("Loops (do-while `%s`): %s", condText, describeBlockInline(body, source))
}

func describeTryStatement(node *sitter.Node, source []byte) string {
	body := node.ChildByFieldName("body")
	handler := node.ChildByFieldName("handler")
	finalizer := node.ChildByFieldName("finalizer")

	result := fmt.Sprintf("Attempts to %s", describeBlockInline(body, source))
	if handler != nil {
		catchParam := handler.ChildByFieldName("parameter")
		catchBody := handler.ChildByFieldName("body")
		paramName := "error"
		if catchParam != nil {
			paramName = catchParam.Content(source)
		}
		result += fmt.Sprintf(". If an error occurs (%s), %s", paramName, describeBlockInline(catchBody, source))
	}
	if finalizer != nil {
		finallyBody := finalizer.ChildByFieldName("body")
		if finallyBody == nil {
			finallyBody = finalizer
		}
		result += fmt.Sprintf(". Finally, %s", describeBlockInline(finallyBody, source))
	}
	return result
}

func describeThrowStatement(node *sitter.Node, source []byte) string {
	if node.NamedChildCount() > 0 {
		child := node.NamedChild(0)
		if child.Type() == "new_expression" {
			constructor := child.ChildByFieldName("constructor")
			args := child.ChildByFieldName("arguments")
			className := "Error"
			if constructor != nil {
				className = constructor.Content(source)
			}
			if args != nil && args.NamedChildCount() > 0 {
				return fmt.Sprintf("Throws a `%s` with message %s", className, describeExpression(args.NamedChild(0), source))
			}
			return fmt.Sprintf("Throws a new `%s`", className)
		}
		return fmt.Sprintf("Throws %s", describeExpression(child, source))
	}
	return "Throws"
}

func describeSwitchStatement(node *sitter.Node, source []byte) string {
	value := node.ChildByFieldName("value")
	body := node.ChildByFieldName("body")
	var cases []string

	if body != nil {
		for i := 0; i < int(body.NamedChildCount()); i++ {
			clause := body.NamedChild(i)
			if clause.Type() == "switch_case" {
				caseValue := clause.ChildByFieldName("value")
				var stmts []string
				for j := 0; j < int(clause.NamedChildCount()); j++ {
					c := clause.NamedChild(j)
					if caseValue != nil && c.Equal(caseValue) {
						continue
					}
					if c.Type() == "comment" {
						continue
					}
					stmts = append(stmts, describeStatement(c, source))
				}
				valText := "?"
				if caseValue != nil {
					valText = caseValue.Content(source)
				}
				cases = append(cases, fmt.Sprintf("case %s: %s", valText, strings.Join(stmts, "; ")))
			} else if clause.Type() == "switch_default" {
				var stmts []string
				for j := 0; j < int(clause.NamedChildCount()); j++ {
					c := clause.NamedChild(j)
					if c.Type() == "comment" {
						continue
					}
					stmts = append(stmts, describeStatement(c, source))
				}
				cases = append(cases, fmt.Sprintf("default: %s", strings.Join(stmts, "; ")))
			}
		}
	}

	valText := ""
	if unwrap := unwrapParens(value); unwrap != nil {
		valText = unwrap.Content(source)
	}
	return fmt.Sprintf("Based on `%s`: %s", valText, strings.Join(cases, ", "))
}

func describeExpressionStatement(node *sitter.Node, source []byte) string {
	if node.NamedChildCount() > 0 {
		return describeExpression(node.NamedChild(0), source)
	}
	return strings.TrimSpace(node.Content(source))
}

func describeBlockInline(node *sitter.Node, source []byte) string {
	if node == nil {
		return "does nothing"
	}
	if node.Type() == "statement_block" || node.Type() == "block" {
		count := node.NamedChildCount()
		if count == 0 {
			return "does nothing"
		}
		if count == 1 {
			stmt := describeStatement(node.NamedChild(0), source)
			re := regexp.MustCompile(`^\d+\.\s*`)
			return re.ReplaceAllString(stmt, "")
		}
		var stmts []string
		for i := 0; i < int(count); i++ {
			stmts = append(stmts, describeStatement(node.NamedChild(i), source))
		}
		return strings.Join(stmts, "; ")
	}
	return describeStatement(node, source)
}

func unwrapParens(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}
	if node.Type() == "parenthesized_expression" {
		if node.NamedChildCount() > 0 {
			return unwrapParens(node.NamedChild(0))
		}
	}
	return node
}

func extractBindingName(node *sitter.Node, source []byte) string {
	if node == nil {
		return "element"
	}
	if node.Type() == "lexical_declaration" || node.Type() == "variable_declaration" {
		for i := 0; i < int(node.NamedChildCount()); i++ {
			c := node.NamedChild(i)
			if c.Type() == "variable_declarator" {
				nameNode := c.ChildByFieldName("name")
				if nameNode != nil {
					return nameNode.Content(source)
				}
			}
		}
	}
	return node.Content(source)
}

func DescribeCondition(node *sitter.Node, source []byte) string {
	return describeCondition(node, source)
}

func describeCondition(node *sitter.Node, source []byte) string {
	if node == nil {
		return "unknown"
	}

	if node.Type() == "unary_expression" {
		operator := node.ChildByFieldName("operator")
		operand := node.ChildByFieldName("argument")
		if operator != nil && operator.Content(source) == "!" {
			opText := ""
			if operand != nil {
				opText = operand.Content(source)
			}
			return fmt.Sprintf("`%s` is falsy", opText)
		}
	}

	if node.Type() == "binary_expression" {
		left := node.ChildByFieldName("left")
		right := node.ChildByFieldName("right")
		op := ""
		if opNode := node.ChildByFieldName("operator"); opNode != nil {
			op = opNode.Content(source)
		}

		if left != nil && left.Type() == "unary_expression" {
			hasTypeof := false
			for i := 0; i < int(left.ChildCount()); i++ {
				if left.Child(i).Type() == "typeof" {
					hasTypeof = true
					break
				}
			}
			if hasTypeof && left.NamedChildCount() > 0 {
				typeofArg := left.NamedChild(0)
				varName := typeofArg.Content(source)
				typeName := ""
				if right != nil {
					typeName = strings.ReplaceAll(right.Content(source), "'", "")
					typeName = strings.ReplaceAll(typeName, "\"", "")
				}
				if op == "===" || op == "==" {
					return fmt.Sprintf("`%s` is a %s", varName, typeName)
				}
				if op == "!==" || op == "!=" {
					return fmt.Sprintf("`%s` is not a %s", varName, typeName)
				}
			}
		}

		leftText := ""
		if left != nil {
			leftText = left.Content(source)
		}
		rightText := ""
		if right != nil {
			rightText = right.Content(source)
		}

		if (op == "===" || op == "==") && (rightText == "null" || rightText == "undefined") {
			return fmt.Sprintf("`%s` is %s", leftText, rightText)
		}
		if (op == "!==" || op == "!=") && (rightText == "null" || rightText == "undefined") {
			return fmt.Sprintf("`%s` is not %s", leftText, rightText)
		}

		if op == "instanceof" {
			return fmt.Sprintf("`%s` is an instance of `%s`", leftText, rightText)
		}

		if left != nil && left.Type() == "member_expression" && strings.HasSuffix(leftText, ".length") && op == ">" && rightText == "0" {
			obj := strings.TrimSuffix(leftText, ".length")
			return fmt.Sprintf("`%s` is non-empty", obj)
		}

		opMap := map[string]string{
			">":   "is greater than",
			"<":   "is less than",
			">=":  "is greater than or equal to",
			"<=":  "is less than or equal to",
			"===": "equals",
			"==":  "equals",
			"!==": "does not equal",
			"!=":  "does not equal",
		}
		if opWord, ok := opMap[op]; ok {
			return fmt.Sprintf("`%s` %s %s", leftText, opWord, rightText)
		}

		return fmt.Sprintf("`%s`", node.Content(source))
	}

	if node.Type() == "parenthesized_expression" {
		if node.NamedChildCount() > 0 {
			return describeCondition(node.NamedChild(0), source)
		}
	}

	return fmt.Sprintf("`%s`", node.Content(source))
}

func DescribeExpression(node *sitter.Node, source []byte) string {
	return describeExpression(node, source)
}

func describeExpression(node *sitter.Node, source []byte) string {
	if node == nil {
		return ""
	}

	if node.Type() == "await_expression" {
		if node.NamedChildCount() > 0 {
			return fmt.Sprintf("awaits %s", describeExpression(node.NamedChild(0), source))
		}
		return fmt.Sprintf("awaits `%s`", node.Content(source))
	}

	if node.Type() == "call_expression" || node.Type() == "call" {
		fn := node.ChildByFieldName("function")
		args := node.ChildByFieldName("arguments")
		calleeText := ""
		if fn != nil {
			calleeText = fn.Content(source)
		}
		if args != nil && args.NamedChildCount() > 0 {
			var argTexts []string
			for i := 0; i < int(args.NamedChildCount()); i++ {
				argTexts = append(argTexts, fmt.Sprintf("`%s`", args.NamedChild(i).Content(source)))
			}
			return fmt.Sprintf("calling `%s` with %s", calleeText, strings.Join(argTexts, ", "))
		}
		return fmt.Sprintf("calling `%s`", calleeText)
	}

	if node.Type() == "assignment_expression" || node.Type() == "assignment" {
		left := node.ChildByFieldName("left")
		right := node.ChildByFieldName("right")
		rightText := "?"
		if right != nil {
			rightText = right.Content(source)
		}
		leftText := "?"
		if left != nil {
			leftText = left.Content(source)
		}
		return fmt.Sprintf("assigning `%s` to `%s`", rightText, leftText)
	}

	if node.Type() == "new_expression" {
		constructor := node.ChildByFieldName("constructor")
		args := node.ChildByFieldName("arguments")
		className := ""
		if constructor != nil {
			className = constructor.Content(source)
		}
		if args != nil && args.NamedChildCount() > 0 {
			var argTexts []string
			for i := 0; i < int(args.NamedChildCount()); i++ {
				argTexts = append(argTexts, fmt.Sprintf("`%s`", args.NamedChild(i).Content(source)))
			}
			return fmt.Sprintf("a new `%s` with %s", className, strings.Join(argTexts, ", "))
		}
		return fmt.Sprintf("a new `%s`", className)
	}

	if node.Type() == "member_expression" {
		return fmt.Sprintf("`%s`", node.Content(source))
	}

	if node.Type() == "binary_expression" || node.Type() == "augmented_assignment_expression" {
		left := node.ChildByFieldName("left")
		right := node.ChildByFieldName("right")
		op := ""
		if opNode := node.ChildByFieldName("operator"); opNode != nil {
			op = opNode.Content(source)
		}
		opMap := map[string]string{
			"+":   "concatenated with",
			"-":   "minus",
			"*":   "times",
			"/":   "divided by",
			"&&":  "and",
			"||":  "or",
			"===": "equals",
			"!==": "does not equal",
		}
		opWord := op
		if val, ok := opMap[op]; ok {
			opWord = val
		}
		return fmt.Sprintf("%s %s %s", describeExpression(left, source), opWord, describeExpression(right, source))
	}

	if node.Type() == "template_string" {
		return fmt.Sprintf("a template string `%s`", node.Content(source))
	}

	if node.Type() == "string" {
		return fmt.Sprintf("`%s`", node.Content(source))
	}

	if node.Type() == "identifier" || node.Type() == "property_identifier" {
		return fmt.Sprintf("`%s`", node.Content(source))
	}

	if node.Type() == "null" {
		return "`null`"
	}

	return fmt.Sprintf("`%s`", node.Content(source))
}

func DescribeSymbols(symbols []LogicSymbol, edges []LogicEdge) string {
	if len(symbols) == 0 {
		return ""
	}
	callsByFrom := map[string][]string{}
	for _, e := range edges {
		callsByFrom[e.From] = append(callsByFrom[e.From], e.To)
	}
	var sections []string
	for _, s := range symbols {
		lines := []string{"## " + s.Name, "Symbol kind: " + s.Kind}
		if s.Doc != "" {
			lines = append(lines, s.Doc)
		}
		if calls := callsByFrom[s.ID]; len(calls) > 0 {
			lines = append(lines, "Calls "+strings.Join(calls, ", "))
		}
		lines = append(lines, "Returns a value.")
		sections = append(sections, strings.Join(lines, "\n"))
	}
	return strings.Join(sections, "\n\n")
}
