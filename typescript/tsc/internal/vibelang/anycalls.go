package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

func init() { fileChecks = append(fileChecks, checkAnyCallResults) }

// checkAnyCallResults reports EK4409 where an Effect function or a program
// returns the result of calling a value of type any. D24 runs a call's result
// only when the call's type is Effect itself, so the call's value is the
// function's success value, and a program the callee returns never runs:
// `Effect.updateContext(() => execute(), f)` with `execute: any` succeeded
// with the Effect execute returned. Nothing in the program says which the
// callee returns, so the warning names the call.
func checkAnyCallResults(c *checker.Checker, file *ast.SourceFile) {
	a := analyzerFor(c)
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if isLowerable(node) && !isProducer(node) && a.isEffect(node) {
			factoryReturns(node, func(value *ast.Node) { checkAnyCallResult(c, node, value) })
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// checkAnyCallResult reports EK4409 when value, what fn returns, is a call
// of a value of type any.
func checkAnyCallResult(c *checker.Checker, fn *ast.Node, value *ast.Node) {
	if value == nil {
		return
	}
	value = ast.SkipParentheses(value)
	if !ast.IsCallExpression(value) {
		return
	}
	callee := value.Expression()
	if t := c.GetTypeAtLocation(callee); t == nil || t.Flags()&checker.TypeFlagsAny == 0 {
		return
	}
	d := report(c, value, EK4409, messageText(value, "This call"), functionDisplayName(fn))
	named := callee
	for ast.IsParenthesizedExpression(named) || ast.IsAsExpression(named) || ast.IsTypeAssertion(named) || ast.IsSatisfiesExpression(named) {
		named = named.Expression() // (loader as any)() names loader
	}
	note(d, "Give {0} a type: a function type that returns Effect<A, E, R> runs the program the call returns, and one that returns a value type keeps the value.", messageText(named, "the callee"))
}
