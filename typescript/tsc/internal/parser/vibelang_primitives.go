package parser

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
)

// The primitives of a .vibe file that have their own syntax (D176-D180):
// `catch (cause: Cause)` (vibelang.go's catch chain), `finally (exit)`, and
// the interruption blocks `uninterruptible { }`, `restore { }`, and
// `interruptible { }`.

// Names the parser writes for the checker; the lowering removes or replaces
// each one.
const (
	vibelangCauseName      = "$cause"     // the catch variable when a clause catches a Cause
	vibelangExitMarker     = "$exit"      // what `finally (exit)` binds
	vibelangFailureOf      = "$failureOf" // the expected failure a caught Cause holds
	vibelangNoFailure      = "$noFailure" // what $failureOf returns for a Cause with no expected failure
	vibelangMaskBlockLabel = "Block"      // the suffix of a mask block's marker, as in $uninterruptibleBlock
)

// isVibeLangMaskWord reports whether text starts an interruption block when an
// open brace follows it on the same line. An identifier followed by a block
// on one line is a TypeScript syntax error, so the form takes nothing away.
func isVibeLangMaskWord(text string) bool {
	switch text {
	case "uninterruptible", "restore", "interruptible":
		return true
	}
	return false
}

// isStartOfVibeLangMaskBlock reports whether the current token starts
// `uninterruptible {`, `restore {`, or `interruptible {`.
func (p *Parser) isStartOfVibeLangMaskBlock() bool {
	return p.token == ast.KindIdentifier && isVibeLangMaskWord(p.scanner.TokenValue()) && p.isVibeLangFile() && p.lookAhead((*Parser).nextTokenIsOpenBraceOnSameLine)
}

// parseVibeLangMaskBlock parses an interruption block:
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
// uninterruptible, restore, or interruptible (primitives.go in the vibelang
// package), the same functions `uninterruptible(program)` calls.
func (p *Parser) parseVibeLangMaskBlock() *ast.Node {
	pos := p.nodePos()
	word := p.scanner.TokenValue()
	anchor := p.scanner.TokenRange()
	p.nextToken()
	callee := p.vibelangSynthesized(p.factory.NewIdentifier("$"+word+vibelangMaskBlockLabel), anchor)
	call := p.vibelangSynthesized(p.factory.NewCallExpression(callee, nil /*questionDotToken*/, nil /*typeArguments*/, p.newNodeList(anchor, nil), ast.NodeFlagsNone), anchor)
	marker := p.vibelangSynthesized(p.factory.NewExpressionStatement(call), anchor)
	statements := []*ast.Node{marker}
	if p.parseExpected(ast.KindOpenBraceToken) {
		body := p.parseList(PCBlockStatements, (*Parser).parseStatement)
		statements = append(statements, body.Nodes...)
		p.parseExpected(ast.KindCloseBraceToken)
	}
	block := p.factory.NewBlock(p.newNodeList(core.NewTextRange(anchor.Pos(), p.nodePos()), statements), true /*multiLine*/)
	return p.finishNode(block, pos)
}

// parseVibeLangFinallyBlock parses what follows `finally` when a binding comes
// first:
//
//	finally (exit) { log(exit) }
//
// as the finally block with `const exit = $exit()` in front. The checker
// types $exit() as the Exit of the try statement (D178); the lowering
// removes the declaration and passes the Exit to the block's closure.
func (p *Parser) parseVibeLangFinallyBlock() *ast.Node {
	p.parseExpected(ast.KindOpenParenToken)
	name := p.parseIdentifier()
	p.parseExpected(ast.KindCloseParenToken)
	block := p.parseBlock(false /*ignoreMissingOpenBrace*/, nil)
	marker := p.vibelangSynthesized(p.factory.NewIdentifier(vibelangExitMarker), name.Loc)
	call := p.vibelangSynthesized(p.factory.NewCallExpression(marker, nil /*questionDotToken*/, nil /*typeArguments*/, p.newNodeList(name.Loc, nil), ast.NodeFlagsNone), name.Loc)
	declaration := p.vibelangSpanning(p.factory.NewVariableDeclaration(name, nil /*exclamationToken*/, nil /*typeNode*/, call), name.Loc)
	list := p.vibelangSpanning(p.factory.NewVariableDeclarationList(p.newNodeList(name.Loc, []*ast.Node{declaration}), ast.NodeFlagsConst), name.Loc)
	statement := p.vibelangSpanning(p.factory.NewVariableStatement(nil /*modifiers*/, list), name.Loc)
	statements := append([]*ast.Node{statement}, block.AsBlock().Statements.Nodes...)
	span := core.NewTextRange(name.Pos(), block.End())
	return p.vibelangSpanning(p.factory.NewBlock(p.newNodeList(span, statements), true /*multiLine*/), span)
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
