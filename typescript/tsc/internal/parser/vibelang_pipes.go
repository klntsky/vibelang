package parser

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
)

// The pipe operator (design D154) in a .vibe file, as in TC39's Hack-style
// proposal:
//
//	fetchUser(id) |> retry(%, policy) |> timeout(%, "1 second")
//
// x |> body evaluates body with the topic % standing for x. The operator is
// | followed directly by >, which TypeScript never parses: > cannot start an
// expression. A pipe parses as a BinaryExpression flagged NodeFlagsVibeLang
// whose operator token is the BarToken; the > after it belongs to the binary
// expression as a token of no child, so every token node stays one scanner
// token for the formatter. The binder and the language service walk a pipe
// as ordinary code; the checker gives it the body's type. A pipe binds
// looser than every binary operator and the conditional operator, as an
// assignment does, and associates to the left: a |> f(%) |> g(%) is
// (a |> f(%)) |> g(%). A body is a conditional expression, so an arrow
// function, an assignment, or a yield there needs parentheses.
//
// % where an expression starts is the topic: an identifier named % flagged
// NodeFlagsVibeLang, which the checker resolves to the topic of the innermost
// pipe whose body holds it. After an operand, % is still the remainder
// operator, so % % 2 is the topic's remainder.
//
// D272 normalizes the special body f(args)(%) to left.pipe(f(args)) before
// checking, so the stage receives the method pipe's contextual type.

// isPipeOperator reports whether the current token, |, starts the pipe
// operator |> in a .vibe file.
func (p *Parser) isPipeOperator() bool {
	end := p.scanner.TokenEnd()
	return p.token == ast.KindBarToken && end < len(p.sourceText) && p.sourceText[end] == '>' && p.isVibeLangFile()
}

// parsePipeRest parses the pipes that follow left, an expression that starts
// at pos.
func (p *Parser) parsePipeRest(left *ast.Expression, pos int) *ast.Expression {
	// D272 evaluates the left side as a value, x.pipe(g), where D154 passes it
	// by name. Only a left side that calls nothing reads the same both ways: a
	// call there may run a program, and .pipe would then reach its result
	// (Smithers twin agent/model, client.execute(request) |> f(id)(%)).
	curried := !runsCall(left)
	for p.isPipeOperator() {
		operatorPos := p.nodePos()
		p.nextToken() // |
		operator := p.finishNode(p.factory.NewToken(ast.KindBarToken), operatorPos)
		p.nextToken() // >
		bodyPos := p.nodePos()
		body := p.parseBinaryExpressionOrHigher(ast.OperatorPrecedenceLowest)
		body = p.parseVibeLangCatchRest(body, bodyPos) // x |> f(%) catch y catches inside the body (vibelang_catch.go)
		body = p.parseConditionalExpressionRest(body, bodyPos, false /*allowReturnTypeInArrowFunction*/)
		if stage := curriedPipeStage(body); stage != nil && curried {
			// D272: give the stage exactly the contextual type of x.pipe(g).
			name := p.vibelangSynthesized(p.factory.NewIdentifier("pipe"), operator.Loc)
			access := p.vibelangSpanning(p.factory.NewPropertyAccessExpression(left, nil, name, ast.NodeFlagsNone), core.NewTextRange(pos, operator.End()))
			arguments := p.newNodeList(stage.Loc, []*ast.Node{stage})
			left = p.finishNode(p.factory.NewCallExpression(access, nil, nil, arguments, ast.NodeFlagsNone), pos)
			continue
		}
		left = p.makeBinaryExpression(left, operator, body, pos)
		left.Flags |= ast.NodeFlagsVibeLang
		curried = false // the next stage's left side runs this body
	}
	return left
}

// runsCall reports whether evaluating node calls something: a call, a tagged
// template (D267), or a new expression, outside nested functions and classes.
func runsCall(node *ast.Node) bool {
	var visit func(n *ast.Node) bool
	visit = func(n *ast.Node) bool {
		switch {
		case ast.IsFunctionLike(n) || ast.IsClassLike(n):
			return false
		case ast.IsCallExpression(n) || ast.IsTaggedTemplateExpression(n) || ast.IsNewExpression(n):
			return true
		}
		return n.ForEachChild(visit)
	}
	return visit(node)
}

// parseTopicReference parses %, the topic of a pipe, where an expression
// starts in a .vibe file.
func (p *Parser) parseTopicReference() *ast.Node {
	pos := p.nodePos()
	p.nextToken()
	topic := p.finishNode(p.factory.NewIdentifier("%"), pos)
	topic.Flags |= ast.NodeFlagsVibeLang
	return topic
}

// curriedPipeStage recognizes only g(%) with a call as g and no other
// references to this pipe's topic. All other bodies retain D154/D155.
func curriedPipeStage(body *ast.Node) *ast.Node {
	if !ast.IsCallExpression(body) || ast.IsOptionalChain(body) || body.TypeArgumentList() != nil || len(body.Arguments()) != 1 {
		return nil
	}
	topic := body.Arguments()[0]
	if topic.Kind != ast.KindIdentifier || topic.Text() != "%" || topic.Flags&ast.NodeFlagsVibeLang == 0 {
		return nil
	}
	stage := body.Expression()
	if !ast.IsCallExpression(stage) || ast.IsOptionalChain(stage) {
		return nil
	}
	var hasTopic func(*ast.Node) bool
	hasTopic = func(n *ast.Node) bool {
		if n.Kind == ast.KindIdentifier && n.Text() == "%" && n.Flags&ast.NodeFlagsVibeLang != 0 {
			return true
		}
		if n.Kind == ast.KindBinaryExpression && n.Flags&ast.NodeFlagsVibeLang != 0 && n.AsBinaryExpression().OperatorToken.Kind == ast.KindBarToken {
			return hasTopic(n.AsBinaryExpression().Left) // nested pipe bodies own their topics
		}
		return n.ForEachChild(hasTopic)
	}
	if hasTopic(stage) {
		return nil
	}
	return stage
}
