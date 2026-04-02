import type { Node as SyntaxNode } from "web-tree-sitter";

/**
 * Describes the body of a function/method as numbered English sentences.
 * Accepts a tree-sitter SyntaxNode for a function_declaration, method_definition,
 * arrow_function, or similar.
 */
export function describeStatements(node: SyntaxNode): string {
  const body = node.childForFieldName("body");
  if (!body) return "Empty function body.";

  // Block body — walk statements
  if (body.type === "statement_block" || body.type === "block") {
    const statements = body.namedChildren;
    if (statements.length === 0) return "Empty function body.";
    return statements
      .map((stmt, i) => `${i + 1}. ${describeStatement(stmt)}`)
      .join("\n");
  }

  // Expression body (arrow function with implicit return)
  return `1. Returns ${describeExpression(body)}`;
}

function describeStatement(node: SyntaxNode): string {
  switch (node.type) {
    case "variable_declaration":
    case "lexical_declaration":
      return describeVariableStatement(node);
    case "return_statement":
      return describeReturnStatement(node);
    case "if_statement":
      return describeIfStatement(node);
    case "for_in_statement":
      return describeForInStatement(node);
    case "for_statement":
      return describeForStatement(node);
    case "while_statement":
      return describeWhileStatement(node);
    case "do_statement":
      return describeDoStatement(node);
    case "try_statement":
      return describeTryStatement(node);
    case "throw_statement":
      return describeThrowStatement(node);
    case "switch_statement":
      return describeSwitchStatement(node);
    case "expression_statement":
      return describeExpressionStatement(node);
    default:
      return `\`${node.text.trim()}\``;
  }
}

function describeVariableStatement(node: SyntaxNode): string {
  const declarators = node.namedChildren.filter(
    (c) => c.type === "variable_declarator",
  );
  const parts = declarators.map((decl) => {
    const nameNode = decl.childForFieldName("name");
    const valueNode = decl.childForFieldName("value");
    const name = nameNode?.text ?? "unknown";
    if (valueNode) {
      return `Assigns ${describeExpression(valueNode)} to \`${name}\``;
    }
    return `Declares \`${name}\``;
  });
  return parts.join(". ");
}

function describeReturnStatement(node: SyntaxNode): string {
  // The return value is not a named field in tree-sitter-typescript;
  // it's the first named child that isn't the "return" keyword
  for (const child of node.namedChildren) {
    return `Returns ${describeExpression(child)}`;
  }
  return "Returns";
}

function describeIfStatement(node: SyntaxNode): string {
  const condition = node.childForFieldName("condition");
  const consequence = node.childForFieldName("consequence");
  const alternative = node.childForFieldName("alternative");

  let result = `If ${describeCondition(unwrapParens(condition))}, ${describeBlockInline(consequence)}`;
  if (alternative) {
    if (alternative.type === "if_statement") {
      result += `. Otherwise, ${describeStatement(alternative)}`;
    } else if (alternative.type === "else_clause") {
      const elseBody = alternative.namedChildren[0];
      if (elseBody && elseBody.type === "if_statement") {
        result += `. Otherwise, ${describeStatement(elseBody)}`;
      } else if (elseBody) {
        result += `. Otherwise, ${describeBlockInline(elseBody)}`;
      }
    } else {
      result += `. Otherwise, ${describeBlockInline(alternative)}`;
    }
  }
  return result;
}

function describeForInStatement(node: SyntaxNode): string {
  const left = node.childForFieldName("left");
  const right = node.childForFieldName("right");
  const body = node.childForFieldName("body");

  const binding = extractBindingName(left);
  const iterable = right?.text ?? "unknown";

  // tree-sitter uses for_in_statement for both for-in and for-of
  // Check if there's an "of" or "in" keyword
  const isForOf = node.children.some((c) => c.type === "of");

  if (isForOf) {
    return `Iterates over each \`${binding}\` in \`${iterable}\`, ${describeBlockInline(body)}`;
  }
  return `Iterates over each key \`${binding}\` in \`${iterable}\`, ${describeBlockInline(body)}`;
}

function describeForStatement(node: SyntaxNode): string {
  // Python style for loop
  const left = node.childForFieldName("left");
  const right = node.childForFieldName("right");
  if (left && right) {
    const binding = left.text;
    const iterable = right.text;
    const body = node.childForFieldName("body");
    return `Iterates over each \`${binding}\` in \`${iterable}\`, ${describeBlockInline(body)}`;
  }

  // TS style for loop
  const initializer = node.childForFieldName("initializer");
  const condition = node.childForFieldName("condition");
  const increment = node.childForFieldName("increment");
  const body = node.childForFieldName("body");

  const initText = initializer?.text ?? "";
  const condText = condition?.text ?? "";
  const incText = increment?.text ?? "";
  return `Loops with ${initText}; while ${condText}; ${incText}: ${describeBlockInline(body)}`;
}

function describeWhileStatement(node: SyntaxNode): string {
  const condition = node.childForFieldName("condition");
  const body = node.childForFieldName("body");
  return `Loops while \`${unwrapParens(condition)?.text ?? ""}\`: ${describeBlockInline(body)}`;
}

function describeDoStatement(node: SyntaxNode): string {
  const condition = node.childForFieldName("condition");
  const body = node.childForFieldName("body");
  return `Loops (do-while \`${unwrapParens(condition)?.text ?? ""}\`): ${describeBlockInline(body)}`;
}

function describeTryStatement(node: SyntaxNode): string {
  const body = node.childForFieldName("body");
  const handler = node.childForFieldName("handler");
  const finalizer = node.childForFieldName("finalizer");

  let result = `Attempts to ${describeBlockInline(body)}`;
  if (handler) {
    // catch_clause has parameter and body
    const catchParam = handler.childForFieldName("parameter");
    const catchBody = handler.childForFieldName("body");
    const paramName = catchParam?.text ?? "error";
    result += `. If an error occurs (${paramName}), ${describeBlockInline(catchBody)}`;
  }
  if (finalizer) {
    const finallyBody = finalizer.childForFieldName("body") ?? finalizer;
    result += `. Finally, ${describeBlockInline(finallyBody)}`;
  }
  return result;
}

function describeThrowStatement(node: SyntaxNode): string {
  for (const child of node.namedChildren) {
    if (child.type === "new_expression") {
      const constructor = child.childForFieldName("constructor");
      const args = child.childForFieldName("arguments");
      const className = constructor?.text ?? "Error";
      const argNodes = args?.namedChildren ?? [];
      if (argNodes.length > 0) {
        return `Throws a \`${className}\` with message ${describeExpression(argNodes[0])}`;
      }
      return `Throws a new \`${className}\``;
    }
    return `Throws ${describeExpression(child)}`;
  }
  return "Throws";
}

function describeSwitchStatement(node: SyntaxNode): string {
  const value = node.childForFieldName("value");
  const body = node.childForFieldName("body");
  const cases: string[] = [];

  if (body) {
    for (const clause of body.namedChildren) {
      if (clause.type === "switch_case") {
        const caseValue = clause.childForFieldName("value");
        const stmts = clause.namedChildren.filter(
          (c) => c !== caseValue && c.type !== "comment",
        );
        const desc = stmts.map((s) => describeStatement(s)).join("; ");
        cases.push(`case ${caseValue?.text ?? "?"}: ${desc}`);
      } else if (clause.type === "switch_default") {
        const stmts = clause.namedChildren.filter(
          (c) => c.type !== "comment",
        );
        const desc = stmts.map((s) => describeStatement(s)).join("; ");
        cases.push(`default: ${desc}`);
      }
    }
  }

  return `Based on \`${unwrapParens(value)?.text ?? ""}\`: ${cases.join(", ")}`;
}

function describeExpressionStatement(node: SyntaxNode): string {
  for (const child of node.namedChildren) {
    return describeExpression(child);
  }
  return node.text.trim();
}

// ---- Helpers ----

function describeBlockInline(node: SyntaxNode | null): string {
  if (!node) return "does nothing";
  if (node.type === "statement_block") {
    const stmts = node.namedChildren;
    if (stmts.length === 0) return "does nothing";
    if (stmts.length === 1)
      return describeStatement(stmts[0]).replace(/^\d+\.\s*/, "");
    return stmts.map((s) => describeStatement(s)).join("; ");
  }
  return describeStatement(node);
}

/** Unwrap parenthesized expressions to get the inner node. */
function unwrapParens(node: SyntaxNode | null): SyntaxNode | null {
  if (!node) return null;
  if (node.type === "parenthesized_expression") {
    const inner = node.namedChildren[0];
    return inner ? unwrapParens(inner) : node;
  }
  return node;
}

function extractBindingName(node: SyntaxNode | null): string {
  if (!node) return "element";
  // lexical_declaration or variable_declaration wrapping a variable_declarator
  if (
    node.type === "lexical_declaration" ||
    node.type === "variable_declaration"
  ) {
    const declarator = node.namedChildren.find(
      (c) => c.type === "variable_declarator",
    );
    const nameNode = declarator?.childForFieldName("name");
    return nameNode?.text ?? "element";
  }
  return node.text;
}

export function describeCondition(node: SyntaxNode | null): string {
  if (!node) return "unknown";

  // !x -> `x` is falsy
  if (node.type === "unary_expression") {
    const operator = node.childForFieldName("operator");
    const operand = node.childForFieldName("argument");
    if (operator?.text === "!") {
      return `\`${operand?.text ?? ""}\` is falsy`;
    }
  }

  // Binary expressions
  if (node.type === "binary_expression") {
    const left = node.childForFieldName("left");
    const right = node.childForFieldName("right");
    const op = node.childForFieldName("operator")?.text ?? "";

    // typeof x === "string"
    if (left?.type === "unary_expression" && left.children.some((c: SyntaxNode) => c.type === "typeof")) {
      const typeofArg = left.namedChildren[0];
      const varName = typeofArg?.text ?? "";
      const typeName = right?.text?.replace(/['"]/g, "") ?? "";
      if (op === "===" || op === "==") {
        return `\`${varName}\` is a ${typeName}`;
      }
      if (op === "!==" || op === "!=") {
        return `\`${varName}\` is not a ${typeName}`;
      }
    }

    // x === null / x === undefined
    if (
      (op === "===" || op === "==") &&
      (right?.text === "null" || right?.text === "undefined")
    ) {
      return `\`${left?.text ?? ""}\` is ${right.text}`;
    }
    if (
      (op === "!==" || op === "!=") &&
      (right?.text === "null" || right?.text === "undefined")
    ) {
      return `\`${left?.text ?? ""}\` is not ${right.text}`;
    }

    // x instanceof Y
    if (op === "instanceof") {
      return `\`${left?.text ?? ""}\` is an instance of \`${right?.text ?? ""}\``;
    }

    // x.length > 0 -> `x` is non-empty
    if (
      left?.type === "member_expression" &&
      left.text.endsWith(".length") &&
      op === ">" &&
      right?.text === "0"
    ) {
      const obj = left.text.replace(/\.length$/, "");
      return `\`${obj}\` is non-empty`;
    }

    // General comparison operators
    const opMap: Record<string, string> = {
      ">": "is greater than",
      "<": "is less than",
      ">=": "is greater than or equal to",
      "<=": "is less than or equal to",
      "===": "equals",
      "==": "equals",
      "!==": "does not equal",
      "!=": "does not equal",
    };
    if (opMap[op]) {
      return `\`${left?.text ?? ""}\` ${opMap[op]} ${right?.text ?? ""}`;
    }

    return `\`${node.text}\``;
  }

  // Parenthesized expression - unwrap
  if (node.type === "parenthesized_expression") {
    const inner = node.namedChildren[0];
    return inner ? describeCondition(inner) : `\`${node.text}\``;
  }

  return `\`${node.text}\``;
}

export function describeExpression(node: SyntaxNode): string {
  // Await expression
  if (node.type === "await_expression") {
    const inner = node.namedChildren[0];
    return inner ? `awaits ${describeExpression(inner)}` : `awaits \`${node.text}\``;
  }

  // Call expression
  if (node.type === "call_expression" || node.type === "call") {
    const fn = node.childForFieldName("function");
    const args = node.childForFieldName("arguments");
    const calleeText = fn?.text ?? "";
    const argNodes = args?.namedChildren ?? [];
    if (argNodes.length > 0) {
      const argTexts = argNodes.map((a) => `\`${a.text}\``).join(", ");
      return `calling \`${calleeText}\` with ${argTexts}`;
    }
    return `calling \`${calleeText}\``;
  }

  // Assignment expression
  if (node.type === "assignment_expression" || node.type === "assignment") {
    const left = node.childForFieldName("left");
    const right = node.childForFieldName("right");
    return `assigning \`${right?.text ?? "?"}\` to \`${left?.text ?? "?"}\``;
  }

  // New expression
  if (node.type === "new_expression") {
    const constructor = node.childForFieldName("constructor");
    const args = node.childForFieldName("arguments");
    const className = constructor?.text ?? "";
    const argNodes = args?.namedChildren ?? [];
    if (argNodes.length > 0) {
      const argTexts = argNodes.map((a) => `\`${a.text}\``).join(", ");
      return `a new \`${className}\` with ${argTexts}`;
    }
    return `a new \`${className}\``;
  }

  // Member expression (property access)
  if (node.type === "member_expression") {
    return `\`${node.text}\``;
  }

  // Binary expression
  if (
    node.type === "binary_expression" ||
    node.type === "augmented_assignment_expression"
  ) {
    const left = node.childForFieldName("left");
    const right = node.childForFieldName("right");
    const op = node.childForFieldName("operator")?.text ?? "";

    const opMap: Record<string, string> = {
      "+": "concatenated with",
      "-": "minus",
      "*": "times",
      "/": "divided by",
      "&&": "and",
      "||": "or",
      "===": "equals",
      "!==": "does not equal",
    };
    const opWord = opMap[op] || op;
    return `${describeExpression(left!)} ${opWord} ${describeExpression(right!)}`;
  }

  // Template literal
  if (node.type === "template_string") {
    return `a template string \`${node.text}\``;
  }

  // String literal
  if (node.type === "string") {
    return `\`${node.text}\``;
  }

  // Identifier
  if (node.type === "identifier" || node.type === "property_identifier") {
    return `\`${node.text}\``;
  }

  // Null keyword
  if (node.type === "null") {
    return "`null`";
  }

  return `\`${node.text}\``;
}
