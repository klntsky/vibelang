package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// callExpressionType gives a call that runs an Effect the type of the
// Effect's success value. Inside a function that the lowering can turn into
// an Effect, `const name = findName(id)` has type string, not
// Effect<string>, because the call runs. A call that runs a plain function
// inline through its $direct variant has its success type anywhere in a .vibe
// file (plain.go). Every other call keeps its type.
func callExpressionType(c *checker.Checker, call *ast.Node, t *checker.Type) *checker.Type {
	if !isInvocation(call) || holdsFieldProgram(c, call) || !inLowerableBody(call) && analyzerFor(c).directCallee(call) == nil {
		return t
	}
	if isExitMarkerCall(call) {
		return exitCallType(c, call, t) // finally (exit) (primitives.go)
	}
	if !callRunsResult(c, call, t) {
		return t
	}
	if ast.IsOptionalChain(call) {
		parts := []*checker.Type{}
		for _, part := range t.Distributed() {
			if success := successType(c, part); success != nil {
				parts = append(parts, success)
			} else {
				parts = append(parts, part)
			}
		}
		return c.GetUnionType(parts)
	}
	if success := successType(c, t); success != nil {
		return success
	}
	return t
}

// callRunsResult is the shared checker/lowering predicate for D24: a call
// runs an Effect-valued result even when its callee is plain JavaScript.
// Optional chaining keeps the absent branch; structured subtypes stay data.
func callRunsResult(c *checker.Checker, call *ast.Node, t *checker.Type) bool {
	if ast.IsOptionalChain(call) {
		t = presentCallResult(c, t)
	}
	return isEffectType(c, t)
}

// inLowerableBody reports whether node is evaluated by the body of a function
// that the lowering can turn into an Effect, not by a nested function, a
// parameter list, or the module's top level.
func inLowerableBody(node *ast.Node) bool {
	for n := node; n.Parent != nil; n = n.Parent {
		if isHatchArgument(n) {
			return false // TypeScript, inside a boundary hatch (hatches.go)
		}
		if fn := n.Parent; isFunctionBoundary(fn) {
			return n == fn.Body() && isLowerable(fn)
		}
	}
	return false
}

// successType returns A for an Effect<A, E, R>, or nil if t is not an Effect.
func successType(c *checker.Checker, t *checker.Type) *checker.Type {
	return channelType(c, t, "_A")
}

// channelType returns one channel of an Effect<A, E, R>, or nil if t is not
// an Effect. name is "_A", "_E", or "_R": a member of the Effect's
// `~effect/Effect` variance property whose type is `(_: never) => Channel`.
func channelType(c *checker.Checker, t *checker.Type, name string) *checker.Type {
	if !isEffectType(c, t) {
		return nil
	}
	variance := c.GetTypeOfPropertyOfType(t, effectTypeID) // Variance<A, E, R>
	if variance == nil {
		return nil
	}
	covariant := c.GetTypeOfPropertyOfType(variance, name) // (_: never) => Channel
	if covariant == nil {
		return nil
	}
	signatures := c.GetSignaturesOfType(covariant, checker.SignatureKindCall)
	if len(signatures) != 1 {
		return nil
	}
	return c.GetReturnTypeOfSignature(signatures[0])
}

// runningCallResult removes only the undefined added by optional chaining.
// D24 still decides from the present branch's exact type, never from a
// structural Effect subtype or an arbitrary union of return values.
func runningCallResult(c *checker.Checker, call *ast.Node, sig *checker.Signature) *checker.Type {
	t := c.GetReturnTypeOfSignature(sig)
	if !ast.IsOptionalChain(call) {
		return t
	}
	return presentCallResult(c, t)
}

func presentCallResult(c *checker.Checker, t *checker.Type) *checker.Type {
	parts := []*checker.Type{}
	for _, part := range t.Distributed() {
		if part.Flags()&checker.TypeFlagsUndefined == 0 {
			parts = append(parts, part)
		}
	}
	return c.GetUnionType(parts)
}

// isInvocation includes tagged templates: invoking a tag is a call (D267).
func isInvocation(node *ast.Node) bool {
	return ast.IsCallExpression(node) || ast.IsTaggedTemplateExpression(node)
}

func init() {
	fileChecks = append(fileChecks, checkTemplateRuns)
}

// D24 keeps structured Effect results as data. Discarding one as a template
// statement must ask rather than silently leaving a SQL migration cold.
func checkTemplateRuns(c *checker.Checker, file *ast.SourceFile) {
	var visit func(*ast.Node) bool
	visit = func(node *ast.Node) bool {
		if ast.IsTaggedTemplateExpression(node) && !inHatch(node) {
			sig := c.GetResolvedSignature(node)
			if sig != nil {
				t := c.GetReturnTypeOfSignature(sig)
				if effectInterfaceSymbol(c, t) != nil && !isEffectType(c, t) && ast.IsExpressionStatement(ast.WalkUpParenthesizedExpressions(node).Parent) {
					report(c, node, EK4301.With("This tag returns an Effect subtype kept as data (D24). Call the template result explicitly to run it, or annotate the tag's return type as Effect<A, E, R>."))
				} else if isEffectType(c, t) && !inLowerableBody(node) && !atTopLevel(node) {
					report(c, node, EK4301.With("This tagged template runs an Effect, but this position cannot run a program. Move it into an Effect function body."))
				}
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}
