package parser

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// The pipe operator (design D154) in an .effeck file, as in TC39's Hack-style
// proposal:
//
//	fetchUser(id) |> retry(%, policy) |> timeout(%, "1 second")
//
// x |> body evaluates body with the topic % standing for x. The operator is
// | followed directly by >, which TypeScript never parses: > cannot start an
// expression. A pipe parses as a BinaryExpression flagged NodeFlagsEffeck
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
// NodeFlagsEffeck, which the checker resolves to the topic of the innermost
// pipe whose body holds it. After an operand, % is still the remainder
// operator, so % % 2 is the topic's remainder.

// isPipeOperator reports whether the current token, |, starts the pipe
// operator |> in an .effeck file.
func (p *Parser) isPipeOperator() bool {
	end := p.scanner.TokenEnd()
	return p.token == ast.KindBarToken && end < len(p.sourceText) && p.sourceText[end] == '>' && p.isEffeckFile()
}

// parsePipeRest parses the pipes that follow left, an expression that starts
// at pos.
func (p *Parser) parsePipeRest(left *ast.Expression, pos int) *ast.Expression {
	for p.isPipeOperator() {
		operatorPos := p.nodePos()
		p.nextToken() // |
		operator := p.finishNode(p.factory.NewToken(ast.KindBarToken), operatorPos)
		p.nextToken() // >
		bodyPos := p.nodePos()
		body := p.parseBinaryExpressionOrHigher(ast.OperatorPrecedenceLowest)
		body = p.parseConditionalExpressionRest(body, bodyPos, false /*allowReturnTypeInArrowFunction*/)
		left = p.makeBinaryExpression(left, operator, body, pos)
		left.Flags |= ast.NodeFlagsEffeck
	}
	return left
}

// parseTopicReference parses %, the topic of a pipe, where an expression
// starts in an .effeck file.
func (p *Parser) parseTopicReference() *ast.Node {
	pos := p.nodePos()
	p.nextToken()
	topic := p.finishNode(p.factory.NewIdentifier("%"), pos)
	topic.Flags |= ast.NodeFlagsEffeck
	return topic
}
