package effeckmapper

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// isCatchExpression recognizes the parser's D236 call, rather than a
// source function that happens to be named $catchExpression. Its two
// arguments are thunks: the left side and a fallback or written handler.
func isCatchExpression(node *ast.Node) bool {
	if !ast.IsCallExpression(node) || len(node.Arguments()) != 2 {
		return false
	}
	callee := node.Expression()
	return ast.IsIdentifier(callee) && callee.Flags&ast.NodeFlagsReparsed != 0 && callee.Text() == "$catchExpression"
}

// catchExpression runs the left side inside $catch and defers the fallback
// until an expected failure (D236-D238). Program and Effect-body edits keep
// both renderings' generator boundaries and preserve the handler's contextual
// error type. No completion records are needed for expression values.
func (p *projector) catchExpression(call *ast.Node, e env) {
	if _, hatch := p.forms.(*Resolver).inHatch(call); hatch {
		return
	}
	start := p.tokenStart(call)
	if !e.effect {
		p.diagnostics = append(p.diagnostics, Diagnostic{
			Start: start, Length: call.End() - start, Code: 1001,
			Message: "A catch expression runs a program. Move it into an Effect function with an explicit contract.",
		})
	} else {
		if p.statementStarts[start] {
			p.plan.insert(start, ";")
			delete(p.statementStarts, start)
		}
		p.plan.mark(editRun, partOpen, start, start, "")
	}
	p.plan.insert(start, helperRef("$catch")+"(")
	left, handler := call.Arguments()[0].Body(), call.Arguments()[1]
	p.catchProgram(left)
	if handler.AsArrowFunction().EqualsGreaterThanToken.Flags&ast.NodeFlagsReparsed != 0 {
		fallback := handler.Body()
		p.plan.replace(left.End(), p.tokenStart(fallback), ", () => ")
		p.catchProgram(fallback)
	} else {
		p.plan.replace(left.End(), p.tokenStart(handler), ", ")
		p.effectLambda(handler)
	}
	p.plan.insert(call.End(), ")")
	if e.effect {
		p.plan.mark(editRun, partClose, call.End(), call.End(), "")
	}
}

// checkCatchExpressionHatches reports D239 independently of lowering, so
// unsupported enclosing forms cannot hide an illegal catch in TypeScript.
func (p *projector) checkCatchExpressionHatches() {
	var visit func(*ast.Node) bool
	visit = func(node *ast.Node) bool {
		if isCatchExpression(node) {
			if _, hatch := p.forms.(*Resolver).inHatch(node); hatch {
				p.diagnostics = append(p.diagnostics, Diagnostic{
					Start: p.tokenStart(node.Expression()), Length: len("catch"), Code: 2114,
					Message: "A catch expression is Effeck syntax, but a hatch body is plain TypeScript. Move the catch out of the hatch, or use a try statement inside it.",
				})
			}
		}
		return node.ForEachChild(visit)
	}
	visit(p.file.AsNode())
}

// catchProgram makes an operand a lazy program, preserving this and keeping
// its calls and failures inside the catch rather than at the enclosing call.
func (p *projector) catchProgram(operand *ast.Node) {
	start := p.tokenStart(operand)
	arg := "expr " + selfArg(operand)
	p.plan.mark(editProgram, partOpen, start, start, arg)
	p.visit(operand, expressionEnv())
	p.plan.mark(editProgram, partClose, operand.End(), operand.End(), arg)
}
