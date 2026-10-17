package parser

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
)

// The primitives of an .effeck file that have their own syntax (D176-D180):
// `catch (cause: Cause)` (effeck.go's catch chain), `finally (exit)`, and
// the interruption blocks `uninterruptible { }`, `restore { }`, and
// `interruptible { }`.

// Names the parser writes for the checker; the lowering removes or replaces
// each one.
const (
	effeckCauseName      = "$cause"     // the catch variable when a clause catches a Cause
	effeckExitMarker     = "$exit"      // what `finally (exit)` binds
	effeckFailureOf      = "$failureOf" // the expected failure a caught Cause holds
	effeckNoFailure      = "$noFailure" // what $failureOf returns for a Cause with no expected failure
	effeckMaskBlockLabel = "Block"      // the suffix of a mask block's marker, as in $uninterruptibleBlock
)

// isEffeckMaskWord reports whether text starts an interruption block when an
// open brace follows it on the same line. An identifier followed by a block
// on one line is a TypeScript syntax error, so the form takes nothing away.
func isEffeckMaskWord(text string) bool {
	switch text {
	case "uninterruptible", "restore", "interruptible":
		return true
	}
	return false
}

// isStartOfEffeckMaskBlock reports whether the current token starts
// `uninterruptible {`, `restore {`, or `interruptible {`.
func (p *Parser) isStartOfEffeckMaskBlock() bool {
	return p.token == ast.KindIdentifier && isEffeckMaskWord(p.scanner.TokenValue()) && p.isEffeckFile() && p.lookAhead((*Parser).nextTokenIsOpenBraceOnSameLine)
}

// parseEffeckMaskBlock parses an interruption block:
//
//	uninterruptible {
//	  const conn = acquire()
//	  try { restore { return use(conn) } } finally { release(conn) }
//	}
//
// as a block whose first statement calls a marker the runtime declares,
// $uninterruptibleBlock(), $restoreBlock(), or $interruptibleBlock(), so the
// checker reads the rest as code of the enclosing function: a return returns
// from it. The lowering runs the statements as a region under the prelude's
// uninterruptible, restore, or interruptible (primitives.go in the effeck
// package), the same functions `uninterruptible(program)` calls.
func (p *Parser) parseEffeckMaskBlock() *ast.Node {
	pos := p.nodePos()
	word := p.scanner.TokenValue()
	anchor := p.scanner.TokenRange()
	p.nextToken()
	callee := p.effeckSynthesized(p.factory.NewIdentifier("$"+word+effeckMaskBlockLabel), anchor)
	call := p.effeckSynthesized(p.factory.NewCallExpression(callee, nil /*questionDotToken*/, nil /*typeArguments*/, p.newNodeList(anchor, nil), ast.NodeFlagsNone), anchor)
	marker := p.effeckSynthesized(p.factory.NewExpressionStatement(call), anchor)
	statements := []*ast.Node{marker}
	if p.parseExpected(ast.KindOpenBraceToken) {
		body := p.parseList(PCBlockStatements, (*Parser).parseStatement)
		statements = append(statements, body.Nodes...)
		p.parseExpected(ast.KindCloseBraceToken)
	}
	block := p.factory.NewBlock(p.newNodeList(core.NewTextRange(anchor.Pos(), p.nodePos()), statements), true /*multiLine*/)
	return p.finishNode(block, pos)
}

// parseEffeckFinallyBlock parses what follows `finally` when a binding comes
// first:
//
//	finally (exit) { log(exit) }
//
// as the finally block with `const exit = $exit()` in front. The checker
// types $exit() as the Exit of the try statement (D178); the lowering
// removes the declaration and passes the Exit to the block's closure.
func (p *Parser) parseEffeckFinallyBlock() *ast.Node {
	p.parseExpected(ast.KindOpenParenToken)
	name := p.parseIdentifier()
	p.parseExpected(ast.KindCloseParenToken)
	block := p.parseBlock(false /*ignoreMissingOpenBrace*/, nil)
	marker := p.effeckSynthesized(p.factory.NewIdentifier(effeckExitMarker), name.Loc)
	call := p.effeckSynthesized(p.factory.NewCallExpression(marker, nil /*questionDotToken*/, nil /*typeArguments*/, p.newNodeList(name.Loc, nil), ast.NodeFlagsNone), name.Loc)
	declaration := p.effeckSpanning(p.factory.NewVariableDeclaration(name, nil /*exclamationToken*/, nil /*typeNode*/, call), name.Loc)
	list := p.effeckSpanning(p.factory.NewVariableDeclarationList(p.newNodeList(name.Loc, []*ast.Node{declaration}), ast.NodeFlagsConst), name.Loc)
	statement := p.effeckSpanning(p.factory.NewVariableStatement(nil /*modifiers*/, list), name.Loc)
	statements := append([]*ast.Node{statement}, block.AsBlock().Statements.Nodes...)
	span := core.NewTextRange(name.Pos(), block.End())
	return p.effeckSpanning(p.factory.NewBlock(p.newNodeList(span, statements), true /*multiLine*/), span)
}

// isCauseCatchClause reports whether clause's variable is typed Cause, as in
// `catch (cause: Cause)` or `catch (c: Cause.Cause<E>)`: the clause catches
// every reason (D176).
func isCauseCatchClause(clause *ast.Node) bool {
	if !isTypedCatchClause(clause) {
		return false
	}
	typeNode := clause.AsCatchClause().VariableDeclaration.Type()
	if typeNode.Kind != ast.KindTypeReference {
		return false
	}
	name := typeNode.AsTypeReferenceNode().TypeName
	switch name.Kind {
	case ast.KindIdentifier:
		return name.Text() == "Cause"
	case ast.KindQualifiedName:
		q := name.AsQualifiedName()
		return q.Right.Text() == "Cause" && q.Left.Kind == ast.KindIdentifier && q.Left.Text() == "Cause"
	}
	return false
}
