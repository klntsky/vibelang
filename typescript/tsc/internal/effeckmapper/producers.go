package effeckmapper

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
)

// Producers (D26, D98). A generator function in an .effeck file is a
// producer, a Stream: `yield x` emits x, a call between yields runs, as in
// an Effect body, `yield* p` emits every element of another producer, and a
// throw fails the Stream:
//
//	function* numbers(n) { ... yield i ... }
//	function numbers(n) { return $produce(function* () { ... yield $emit(i) ... }) }
//
// $produce types the generator as Stream<A, E, R>: A from what it emits, E
// and R from what it runs. A for...of in an Effect body or a producer
// iterates `(yield* $each(xs))`, which runs a Stream for its elements, with
// the Stream's E and R, and leaves any other iterable as it is (D26).

// isProducer reports whether node is a producer: a generator function
// declaration with a body, not async.
func isProducer(node *ast.Node) bool {
	if node.Kind != ast.KindFunctionDeclaration || node.Flags&ast.NodeFlagsReparsed != 0 {
		return false
	}
	fn := node.AsFunctionDeclaration()
	return fn.Body != nil && fn.AsteriskToken != nil && !ast.HasSyntacticModifier(node, ast.ModifierFlagsAsync)
}

// producerDeclaration lowers a producer to the function that returns its
// Stream.
func (p *projector) producerDeclaration(node *ast.Node) {
	fn := node.AsFunctionDeclaration()
	body := fn.Body
	open := p.tokenStart(body)
	closing := body.End() - 1
	if open >= len(p.text) || p.text[open] != '{' || closing <= open || p.text[closing] != '}' {
		return
	}
	asterisk := fn.AsteriskToken
	p.plan.replace(p.tokenStart(asterisk), asterisk.End(), "")
	node.ForEachChild(func(child *ast.Node) bool {
		if child != body && child != asterisk {
			p.visit(child, env{})
		}
		return false
	})
	e := newEffectEnv()
	e.producer = true
	p.plan.mark(editProducer, partOpen, open+1, open+1, selfArg(body))
	p.visitList(body.Statements(), e)
	p.plan.mark(editProducer, partClose, closing, closing, "")
}

// yieldExpression lowers a yield in a producer: `yield x` emits x, and
// `yield* p` emits each element of the producer or Stream p.
func (p *projector) yieldExpression(node *ast.Node, e env) {
	y := node.AsYieldExpression()
	expression := y.Expression
	if y.AsteriskToken != nil {
		if expression == nil {
			return
		}
		start := p.tokenStart(expression)
		p.plan.mark(editEmit, partOpen, start, start, "delegate")
		p.visit(expression, e)
		p.plan.mark(editEmit, partClose, expression.End(), expression.End(), "delegate")
		return
	}
	if expression == nil {
		// yield emits undefined.
		p.plan.mark(editEmit, partWhole, node.End(), node.End(), "undefined")
		return
	}
	start := p.tokenStart(expression)
	p.plan.mark(editEmit, partOpen, start, start, "")
	p.visit(expression, e)
	p.plan.mark(editEmit, partClose, expression.End(), expression.End(), "")
}

// forOfStatement lowers a for...of in an Effect body or a producer: the
// iterated value goes through $each, which runs a Stream for its elements
// and leaves any other iterable as it is (D26).
func (p *projector) forOfStatement(node *ast.Node, e env) {
	statement := node.AsForInOrOfStatement()
	if statement.AwaitModifier != nil {
		p.jumpTarget(node, e)
		return
	}
	expression := statement.Expression
	t := target{loop: true}
	e.frame.targets = append(e.frame.targets, t)
	defer func() { e.frame.targets = e.frame.targets[:len(e.frame.targets)-1] }()
	p.visit(statement.Initializer, e)
	start := p.tokenStart(expression)
	p.plan.mark(editEach, partOpen, start, start, "")
	p.visit(expression, e)
	p.plan.mark(editEach, partClose, expression.End(), expression.End(), "")
	p.visit(statement.Statement, e)
}
