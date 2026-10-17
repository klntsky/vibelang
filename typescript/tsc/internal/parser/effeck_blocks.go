package parser

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
)

// Two TC39 expression forms parse, in an .effeck file, into an immediately
// invoked arrow function, so the checker, the analyzer, and the lowering
// treat them as the call they abbreviate (D186, D187):
//
//	x ?? throw new NotFound({ id })   x ?? (() => { throw new NotFound({ id }) })()
//	do { const u = getUser(id); u.name }
//	                                  (() => { const u = getUser(id); return u.name })()
//
// The arrow is an Effeck function like any other: when its body fails or runs
// an Effect, the call runs it where it stands (D24), and a parameter that
// expects a program takes the call by name (D147). A throw expression
// therefore fails exactly as the throw statement does.

// isStartOfEffeckThrowExpression reports whether the current token starts a
// throw expression: throw in expression position in an .effeck file. A
// statement that starts with throw is the throw statement, which the
// statement parser takes first.
func (p *Parser) isStartOfEffeckThrowExpression() bool {
	return p.token == ast.KindThrowKeyword && p.isEffeckFile()
}

// parseEffeckThrowExpression parses `throw operand`, whose operand is a unary
// expression, as the proposal's grammar has it
// (https://github.com/tc39/proposal-throw-expressions).
func (p *Parser) parseEffeckThrowExpression() *ast.Node {
	pos := p.nodePos()
	anchor := p.tokenRange()
	p.nextToken() // throw
	operand := p.parseSimpleUnaryExpression()
	loc := core.NewTextRange(anchor.Pos(), operand.End())
	statement := p.effeckSpanning(p.factory.NewThrowStatement(operand), loc)
	body := p.effeckSpanning(p.factory.NewBlock(p.newNodeList(loc, []*ast.Node{statement}), false /*multiLine*/), loc)
	return p.effeckInvoked(body, anchor, pos)
}

// isStartOfEffeckDoExpression reports whether the current token starts a do
// expression: do followed by { in expression position in an .effeck file.
// A statement that starts with do is the do-while statement.
func (p *Parser) isStartOfEffeckDoExpression() bool {
	return p.token == ast.KindDoKeyword && p.isEffeckFile() && p.lookAhead((*Parser).nextTokenIsOpenBraceToken)
}

func (p *Parser) nextTokenIsOpenBraceToken() bool {
	p.nextToken()
	return p.token == ast.KindOpenBraceToken
}

// parseEffeckDoExpression parses `do { statements }`, whose value is the
// completion value of its last statement
// (https://github.com/tc39/proposal-do-expressions). The last statement
// becomes a return; an if needs an else, and a declaration or a loop cannot
// end the block, as the proposal requires. A return inside the block is an
// error, because it would leave the expression rather than the function.
func (p *Parser) parseEffeckDoExpression() *ast.Node {
	pos := p.nodePos()
	anchor := p.tokenRange()
	p.nextToken() // do
	body := p.parseFunctionBlock(ParseFlagsNone, nil /*diagnosticMessage*/)
	p.reportReturnsInDo(body)
	statements := body.AsBlock().Statements
	if len(statements.Nodes) != 0 {
		p.effeckCompletion(statements)
	}
	return p.effeckInvoked(body, anchor, pos)
}

// effeckInvoked returns (() => body)(), the arrow anchored on the keyword
// that starts the form, finished at pos.
func (p *Parser) effeckInvoked(body *ast.Node, anchor core.TextRange, pos int) *ast.Node {
	arrow := p.effeckProgramArrow(nil, body, anchor)
	loc := core.NewTextRange(anchor.Pos(), body.End())
	callee := p.effeckSpanning(p.factory.NewParenthesizedExpression(arrow), loc)
	arguments := p.newNodeList(core.NewTextRange(body.End(), body.End()), nil)
	return p.finishNode(p.factory.NewCallExpression(callee, nil, nil, arguments, ast.NodeFlagsNone), pos)
}

// effeckCompletion turns the last statement of statements into the
// statement that returns its completion value, in place.
func (p *Parser) effeckCompletion(statements *ast.NodeList) {
	last := statements.Nodes[len(statements.Nodes)-1]
	if replaced := p.effeckCompletionOf(last); replaced != last {
		statements.Nodes[len(statements.Nodes)-1] = replaced
	}
}

// effeckCompletionOf returns the statement that returns statement's
// completion value: a return for an expression statement, or the statement
// itself with its branches converted.
func (p *Parser) effeckCompletionOf(statement *ast.Node) *ast.Node {
	switch statement.Kind {
	case ast.KindExpressionStatement:
		returned := p.factory.NewReturnStatement(statement.Expression())
		returned.Parent = statement.Parent
		return p.effeckSpanning(returned, statement.Loc)
	case ast.KindThrowStatement:
		return statement
	case ast.KindBlock:
		if statements := statement.AsBlock().Statements; len(statements.Nodes) != 0 {
			p.effeckCompletion(statements)
			p.overrideParentInImmediateChildren(statement)
		}
		return statement
	case ast.KindIfStatement:
		ifStatement := statement.AsIfStatement()
		if ifStatement.ElseStatement == nil {
			p.parseErrorAtRange(p.skipRangeTrivia(statement.Loc), diagnostics.Expression_expected)
			return statement
		}
		ifStatement.ThenStatement = p.effeckCompletionOf(ifStatement.ThenStatement)
		ifStatement.ElseStatement = p.effeckCompletionOf(ifStatement.ElseStatement)
		p.overrideParentInImmediateChildren(statement)
		return statement
	case ast.KindTryStatement:
		tryStatement := statement.AsTryStatement()
		p.effeckCompletionOf(tryStatement.TryBlock)
		if tryStatement.CatchClause != nil {
			p.effeckCompletionOf(tryStatement.CatchClause.AsCatchClause().Block)
		}
		return statement
	}
	// A declaration, a loop, or any other statement has no completion value
	// the proposal allows at the end of a do expression.
	p.parseErrorAtRange(p.skipRangeTrivia(statement.Loc), diagnostics.Expression_expected)
	return statement
}

// reportReturnsInDo reports each return statement of a do expression's
// block, not counting those of nested functions and classes.
func (p *Parser) reportReturnsInDo(body *ast.Node) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if node.Kind == ast.KindReturnStatement {
			p.parseErrorAtRange(p.skipRangeTrivia(node.Loc), diagnostics.A_return_statement_can_only_be_used_within_a_function_body)
		}
		if ast.IsFunctionLike(node) || ast.IsClassLike(node) {
			return false
		}
		node.ForEachChild(visit)
		return false
	}
	body.ForEachChild(visit)
}

// isStartOfEffeckBlockParam reports whether a block param follows expression:
// in an .effeck file, a { on the same line after a name, a property access,
// or a call, which TypeScript cannot parse
// (https://github.com/samuelgoto/proposal-block-params). A heritage clause
// and a provide block's providers are followed by their own blocks.
func (p *Parser) isStartOfEffeckBlockParam(expression *ast.Node) bool {
	if p.token != ast.KindOpenBraceToken || p.hasPrecedingLineBreak() || p.effeckNoBlockParams || !p.isEffeckFile() {
		return false
	}
	switch expression.Kind {
	case ast.KindIdentifier, ast.KindPropertyAccessExpression, ast.KindCallExpression:
		return expression.Flags&ast.NodeFlagsOptionalChain == 0
	}
	return false
}

// parseEffeckBlockParam parses the block after expression as a program and
// passes it as the last argument of one call (D188): `f { block }` is
// f(() => { block }) and `f(args) { block }` is f(args, () => { block }). The
// checker offers an Effect dual's call the forms that take the block as the
// program, and the lowering moves it there (effeck/blockparams.go).
func (p *Parser) parseEffeckBlockParam(pos int, expression *ast.Node) *ast.Node {
	anchor := p.tokenRange()
	body := p.parseFunctionBlock(ParseFlagsNone, nil /*diagnosticMessage*/)
	program := p.effeckProgramArrow(nil, body, anchor)
	if expression.Kind == ast.KindCallExpression {
		call := expression.AsCallExpression()
		nodes := append(append([]*ast.Node{}, call.Arguments.Nodes...), program)
		arguments := p.newNodeList(core.NewTextRange(call.Arguments.Pos(), program.End()), nodes)
		return p.finishNode(p.factory.NewCallExpression(call.Expression, nil /*questionDotToken*/, call.TypeArguments, arguments, ast.NodeFlagsNone), pos)
	}
	arguments := p.newNodeList(program.Loc, []*ast.Node{program})
	return p.finishNode(p.factory.NewCallExpression(expression, nil /*questionDotToken*/, nil /*typeArguments*/, arguments, ast.NodeFlagsNone), pos)
}
