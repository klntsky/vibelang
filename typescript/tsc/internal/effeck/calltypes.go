package effeck

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// callExpressionType gives a call that runs an Effect the type of the
// Effect's success value. Inside a function that the lowering can turn into
// an Effect, `const name = findName(id)` has type string, not
// Effect<string>, because the call runs. A call that runs a plain function
// inline through its $direct variant has its success type anywhere in an .effeck
// file (plain.go). Every other call keeps its type.
func callExpressionType(c *checker.Checker, call *ast.Node, t *checker.Type) *checker.Type {
	if !ast.IsCallExpression(call) || !inLowerableBody(call) && analyzerFor(c).directCallee(call) == nil {
		return t
	}
	if isExitMarkerCall(call) {
		return exitCallType(c, call, t) // finally (exit) (primitives.go)
	}
	if success := successType(c, t); success != nil {
		return success
	}
	return t
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
