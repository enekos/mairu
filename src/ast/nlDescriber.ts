import {
  Node,
  SyntaxKind,
  type FunctionDeclaration,
  type MethodDeclaration,
} from "ts-morph";

/**
 * Describes the body of a function/method as numbered English sentences.
 */
export function describeStatements(
  node: FunctionDeclaration | MethodDeclaration
): string {
  const body = node.getBody();
  if (!body || !Node.isBlock(body)) {
    return "Empty function body.";
  }
  const statements = body.getStatements();
  if (statements.length === 0) {
    return "Empty function body.";
  }
  return statements
    .map((stmt, i) => `${i + 1}. ${describeStatement(stmt)}`)
    .join("\n");
}

function describeStatement(node: Node): string {
  const kind = node.getKind();

  switch (kind) {
    case SyntaxKind.VariableStatement:
      return describeVariableStatement(node);
    case SyntaxKind.ReturnStatement:
      return describeReturnStatement(node);
    case SyntaxKind.IfStatement:
      return describeIfStatement(node);
    case SyntaxKind.ForOfStatement:
      return describeForOfStatement(node);
    case SyntaxKind.ForInStatement:
      return describeForInStatement(node);
    case SyntaxKind.ForStatement:
      return describeForStatement(node);
    case SyntaxKind.WhileStatement:
      return describeWhileStatement(node);
    case SyntaxKind.DoStatement:
      return describeDoStatement(node);
    case SyntaxKind.TryStatement:
      return describeTryStatement(node);
    case SyntaxKind.ThrowStatement:
      return describeThrowStatement(node);
    case SyntaxKind.SwitchStatement:
      return describeSwitchStatement(node);
    case SyntaxKind.ExpressionStatement:
      return describeExpressionStatement(node);
    default:
      return `\`${node.getText().trim()}\``;
  }
}

function describeVariableStatement(node: Node): string {
  const declarations = node
    .getChildrenOfKind(SyntaxKind.VariableDeclarationList)
    .flatMap((list) => list.getChildrenOfKind(SyntaxKind.VariableDeclaration));

  const parts = declarations.map((decl) => {
    const name = decl.getName();
    const initializer = decl.getInitializer();
    if (initializer) {
      return `Assigns ${describeExpression(initializer)} to \`${name}\``;
    }
    return `Declares \`${name}\``;
  });
  return parts.join(". ");
}

function describeReturnStatement(node: Node): string {
  const children = node.getChildren();
  // Find the expression (skip ReturnKeyword and possible whitespace)
  for (const child of children) {
    const ck = child.getKind();
    if (
      ck !== SyntaxKind.ReturnKeyword &&
      ck !== SyntaxKind.SemicolonToken &&
      ck !== SyntaxKind.WhitespaceTrivia
    ) {
      return `Returns ${describeExpression(child)}`;
    }
  }
  return "Returns";
}

function describeIfStatement(node: Node): string {
  const ifNode = node as unknown as {
    getExpression(): Node;
    getThenStatement(): Node;
    getElseStatement(): Node | undefined;
  };
  const condition = ifNode.getExpression();
  const thenBlock = ifNode.getThenStatement();
  const elseBlock = ifNode.getElseStatement();

  let result = `If ${describeCondition(condition)}, ${describeBlockInline(thenBlock)}`;
  if (elseBlock) {
    if (elseBlock.getKind() === SyntaxKind.IfStatement) {
      result += `. Otherwise, ${describeStatement(elseBlock)}`;
    } else {
      result += `. Otherwise, ${describeBlockInline(elseBlock)}`;
    }
  }
  return result;
}

function describeForOfStatement(node: Node): string {
  const initializer = findChildOfKind(node, SyntaxKind.VariableDeclarationList);
  const binding = initializer
    ? initializer
        .getChildrenOfKind(SyntaxKind.VariableDeclaration)
        .map((d) => d.getName())
        .join(", ")
    : "element";

  // The iterable expression is after 'of' keyword
  const forOfNode = node as unknown as {
    getExpression(): Node;
    getInitializer(): Node;
    getStatement(): Node;
  };
  const iterable = forOfNode.getExpression();
  const body = forOfNode.getStatement();

  return `Iterates over each \`${binding}\` in \`${iterable.getText()}\`, ${describeBlockInline(body)}`;
}

function describeForInStatement(node: Node): string {
  const forInNode = node as unknown as {
    getExpression(): Node;
    getInitializer(): Node;
    getStatement(): Node;
  };
  const initializer = findChildOfKind(node, SyntaxKind.VariableDeclarationList);
  const binding = initializer
    ? initializer
        .getChildrenOfKind(SyntaxKind.VariableDeclaration)
        .map((d) => d.getName())
        .join(", ")
    : "key";
  const obj = forInNode.getExpression();
  const body = forInNode.getStatement();
  return `Iterates over each key \`${binding}\` in \`${obj.getText()}\`, ${describeBlockInline(body)}`;
}

function describeForStatement(node: Node): string {
  const forNode = node as unknown as {
    getInitializer(): Node | undefined;
    getCondition(): Node | undefined;
    getIncrementor(): Node | undefined;
    getStatement(): Node;
  };
  const init = forNode.getInitializer();
  const cond = forNode.getCondition();
  const inc = forNode.getIncrementor();
  const body = forNode.getStatement();

  const initText = init ? init.getText() : "";
  const condText = cond ? cond.getText() : "";
  const incText = inc ? inc.getText() : "";
  return `Loops with ${initText}; while ${condText}; ${incText}: ${describeBlockInline(body)}`;
}

function describeWhileStatement(node: Node): string {
  const whileNode = node as unknown as {
    getExpression(): Node;
    getStatement(): Node;
  };
  const condition = whileNode.getExpression();
  const body = whileNode.getStatement();
  return `Loops while \`${condition.getText()}\`: ${describeBlockInline(body)}`;
}

function describeDoStatement(node: Node): string {
  const doNode = node as unknown as {
    getExpression(): Node;
    getStatement(): Node;
  };
  const condition = doNode.getExpression();
  const body = doNode.getStatement();
  return `Loops (do-while \`${condition.getText()}\`): ${describeBlockInline(body)}`;
}

function describeTryStatement(node: Node): string {
  const tryNode = node as unknown as {
    getTryBlock(): Node;
    getCatchClause(): Node | undefined;
    getFinallyBlock(): Node | undefined;
  };
  const tryBlock = tryNode.getTryBlock();
  const catchClause = tryNode.getCatchClause();
  const finallyBlock = tryNode.getFinallyBlock();

  let result = `Attempts to ${describeBlockInline(tryBlock)}`;
  if (catchClause) {
    const catchClauseTyped = catchClause as unknown as {
      getVariableDeclaration(): { getName(): string } | undefined;
      getBlock(): Node;
    };
    const catchVar = catchClauseTyped.getVariableDeclaration();
    const catchParam = catchVar ? catchVar.getName() : "error";
    const catchBlock = catchClauseTyped.getBlock();
    result += `. If an error occurs (${catchParam}), ${describeBlockInline(catchBlock)}`;
  }
  if (finallyBlock) {
    result += `. Finally, ${describeBlockInline(finallyBlock)}`;
  }
  return result;
}

function describeThrowStatement(node: Node): string {
  const children = node.getChildren();
  for (const child of children) {
    const ck = child.getKind();
    if (
      ck !== SyntaxKind.ThrowKeyword &&
      ck !== SyntaxKind.SemicolonToken
    ) {
      // Check for `new X(msg)` pattern
      if (ck === SyntaxKind.NewExpression) {
        const newExpr = child as unknown as {
          getExpression(): Node;
          getArguments(): Node[];
        };
        const className = newExpr.getExpression().getText();
        const args = newExpr.getArguments();
        if (args.length > 0) {
          return `Throws a \`${className}\` with message ${describeExpression(args[0])}`;
        }
        return `Throws a new \`${className}\``;
      }
      return `Throws ${describeExpression(child)}`;
    }
  }
  return "Throws";
}

function describeSwitchStatement(node: Node): string {
  const switchNode = node as unknown as {
    getExpression(): Node;
    getClauses(): Node[];
  };
  const discriminant = switchNode.getExpression();
  const clauses = switchNode.getClauses();

  const cases = clauses.map((clause) => {
    const isDefault = clause.getKind() === SyntaxKind.DefaultClause;
    if (isDefault) {
      const stmts = (clause as unknown as { getStatements(): Node[] }).getStatements();
      return `default: ${stmts.map((s) => describeStatement(s)).join("; ")}`;
    }
    const caseClause = clause as unknown as {
      getExpression(): Node;
      getStatements(): Node[];
    };
    const val = caseClause.getExpression().getText();
    const stmts = caseClause.getStatements();
    return `case ${val}: ${stmts.map((s) => describeStatement(s)).join("; ")}`;
  });

  return `Based on \`${discriminant.getText()}\`: ${cases.join(", ")}`;
}

function describeExpressionStatement(node: Node): string {
  const children = node.getChildren();
  for (const child of children) {
    if (child.getKind() !== SyntaxKind.SemicolonToken) {
      return describeExpression(child);
    }
  }
  return node.getText().trim();
}

// ---- Helpers ----

function describeBlockInline(node: Node): string {
  if (Node.isBlock(node)) {
    const stmts = node.getStatements();
    if (stmts.length === 0) return "does nothing";
    if (stmts.length === 1) return describeStatement(stmts[0]).replace(/^\d+\.\s*/, "");
    return stmts.map((s) => describeStatement(s)).join("; ");
  }
  // Single statement (no braces)
  return describeStatement(node);
}

export function describeCondition(node: Node): string {
  const kind = node.getKind();

  // !x -> `x` is falsy
  if (kind === SyntaxKind.PrefixUnaryExpression) {
    const prefix = node as unknown as {
      getOperatorToken(): number;
      getOperand(): Node;
    };
    if (prefix.getOperatorToken() === SyntaxKind.ExclamationToken) {
      const operand = prefix.getOperand();
      return `\`${operand.getText()}\` is falsy`;
    }
  }

  // Binary expressions
  if (kind === SyntaxKind.BinaryExpression) {
    const binary = node as unknown as {
      getLeft(): Node;
      getRight(): Node;
      getOperatorToken(): Node;
    };
    const left = binary.getLeft();
    const right = binary.getRight();
    const op = binary.getOperatorToken().getText();

    // typeof x === "string"
    if (left.getKind() === SyntaxKind.TypeOfExpression) {
      const typeofExpr = left as unknown as { getExpression(): Node };
      const varName = typeofExpr.getExpression().getText();
      const typeName = right.getText().replace(/['"]/g, "");
      if (op === "===" || op === "==") {
        return `\`${varName}\` is a ${typeName}`;
      }
      if (op === "!==" || op === "!=") {
        return `\`${varName}\` is not a ${typeName}`;
      }
    }

    // x === null / x === undefined
    if ((op === "===" || op === "==") && (right.getText() === "null" || right.getText() === "undefined")) {
      return `\`${left.getText()}\` is ${right.getText()}`;
    }
    if ((op === "!==" || op === "!=") && (right.getText() === "null" || right.getText() === "undefined")) {
      return `\`${left.getText()}\` is not ${right.getText()}`;
    }

    // x instanceof Y
    if (op === "instanceof") {
      return `\`${left.getText()}\` is an instance of \`${right.getText()}\``;
    }

    // x.length > 0 -> `x` is non-empty
    if (
      left.getKind() === SyntaxKind.PropertyAccessExpression &&
      left.getText().endsWith(".length") &&
      op === ">" &&
      right.getText() === "0"
    ) {
      const obj = left.getText().replace(/\.length$/, "");
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
      return `\`${left.getText()}\` ${opMap[op]} ${right.getText()}`;
    }

    return `\`${node.getText()}\``;
  }

  // Parenthesized expression - unwrap
  if (kind === SyntaxKind.ParenthesizedExpression) {
    const inner = (node as unknown as { getExpression(): Node }).getExpression();
    return describeCondition(inner);
  }

  return `\`${node.getText()}\``;
}

export function describeExpression(node: Node): string {
  const kind = node.getKind();

  // Await expression
  if (kind === SyntaxKind.AwaitExpression) {
    const awaitExpr = node as unknown as { getExpression(): Node };
    return `awaits ${describeExpression(awaitExpr.getExpression())}`;
  }

  // Call expression
  if (kind === SyntaxKind.CallExpression) {
    const callExpr = node as unknown as {
      getExpression(): Node;
      getArguments(): Node[];
    };
    const callee = callExpr.getExpression();
    const args = callExpr.getArguments();
    const calleeText = callee.getText();
    if (args.length > 0) {
      const argTexts = args.map((a) => `\`${a.getText()}\``).join(", ");
      return `calling \`${calleeText}\` with ${argTexts}`;
    }
    return `calling \`${calleeText}\`()`;
  }

  // New expression
  if (kind === SyntaxKind.NewExpression) {
    const newExpr = node as unknown as {
      getExpression(): Node;
      getArguments(): Node[];
    };
    const className = newExpr.getExpression().getText();
    const args = newExpr.getArguments();
    if (args.length > 0) {
      const argTexts = args.map((a) => `\`${a.getText()}\``).join(", ");
      return `a new \`${className}\` with ${argTexts}`;
    }
    return `a new \`${className}\``;
  }

  // Property access
  if (kind === SyntaxKind.PropertyAccessExpression) {
    return `\`${node.getText()}\``;
  }

  // Binary expression
  if (kind === SyntaxKind.BinaryExpression) {
    const binary = node as unknown as {
      getLeft(): Node;
      getRight(): Node;
      getOperatorToken(): Node;
    };
    const left = binary.getLeft();
    const right = binary.getRight();
    const op = binary.getOperatorToken().getText();

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
    return `${describeExpression(left)} ${opWord} ${describeExpression(right)}`;
  }

  // Template literal
  if (kind === SyntaxKind.TemplateExpression) {
    return `a template string \`${node.getText()}\``;
  }

  // String literal
  if (kind === SyntaxKind.StringLiteral) {
    return `\`${node.getText()}\``;
  }

  // Identifier
  if (kind === SyntaxKind.Identifier) {
    return `\`${node.getText()}\``;
  }

  // Null keyword
  if (kind === SyntaxKind.NullKeyword) {
    return "`null`";
  }

  return `\`${node.getText()}\``;
}

function findChildOfKind(node: Node, kind: SyntaxKind): Node | undefined {
  return node.getChildren().find((c) => c.getKind() === kind);
}
