package effectscript

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/ls"
)

// The language server's hooks register here rather than in effectscript.go,
// so editor features stay in this file and completions.go.
func init() {
	ls.RegisterQuickInfoHook(plainFunctionQuickInfo)
	ls.RegisterQuickInfoHook(runningCallQuickInfo)
	ls.RegisterCompletionScopeHook(preludeCompletions)
	ls.RegisterMemberCompletionHook(serviceMemberCompletions)
}

const hoverTypeFlags = checker.TypeFormatFlagsUseAliasDefinedOutsideCurrentScope

// runningCallQuickInfo tells a hover on the callee of a call that runs what
// the call evaluates to: the success value of the Effect it runs. Hovering
// lookup in `const name = lookup(1)` shows lookup's contract, then
// "Runs here and returns `string`." A call that does not run, such as one
// in a .ts file, gets nothing.
func runningCallQuickInfo(c *checker.Checker, node *ast.Node) (string, string) {
	call := callOfCallee(node)
	if call == nil || !runsInLoweredBody(c, call) {
		return "", ""
	}
	signature := c.GetResolvedSignature(call)
	if signature == nil {
		return "", ""
	}
	success := successType(c, c.GetReturnTypeOfSignature(signature))
	if success == nil {
		return "", ""
	}
	return "", "Runs here and returns `" + c.TypeToStringEx(success, call, hoverTypeFlags, nil) + "`."
}

// callOfCallee returns the call that node names the callee of: the call
// f(x) for f, and Console.log(x) for log. It returns nil for any other node.
func callOfCallee(node *ast.Node) *ast.Node {
	callee := node
	if parent := node.Parent; parent != nil && ast.IsPropertyAccessExpression(parent) && parent.Name() == node {
		callee = parent
	}
	if parent := callee.Parent; parent != nil && ast.IsCallExpression(parent) && parent.Expression() == callee {
		return parent
	}
	return nil
}

// runsInLoweredBody reports whether the lowering emits call as yield*: the
// function whose own body evaluates call is an Effect, and call runs an
// Effect. It asks the same questions as planFile.
func runsInLoweredBody(c *checker.Checker, call *ast.Node) bool {
	for n := call; n.Parent != nil; n = n.Parent {
		if fn := n.Parent; isFunctionBoundary(fn) {
			if n != fn.Body() || !isLowerable(fn) {
				return false
			}
			a := analyzerFor(c)
			return a.isEffect(fn) && a.runs(call)
		}
	}
	return false
}

// plainFunctionQuickInfo shows a function that compiles to plain JavaScript
// the way EffectScript calls it. TypeScript sees `function add(a: number,
// b: number)` as returning Effect<number, never, never>, its one calling
// convention (Chapter 15), but EffectScript code calls add$direct and gets
// the number. So a hover on add, or on a call to it, in an .es file shows
// `function add(a: number, b: number): number` and says what TypeScript
// callers see. A hover in a .ts file keeps TypeScript's view.
func plainFunctionQuickInfo(c *checker.Checker, node *ast.Node) (string, string) {
	if !ast.IsIdentifier(node) || !isEffectScriptFile(ast.GetSourceFileOfNode(node)) {
		return "", ""
	}
	symbol := c.GetSymbolAtLocation(node)
	if symbol != nil && symbol.Flags()&ast.SymbolFlagsAlias != 0 {
		symbol = c.GetAliasedSymbol(symbol)
	}
	if symbol == nil || symbol.Flags()&ast.SymbolFlagsFunction == 0 || len(symbol.Declarations()) != 1 {
		return "", ""
	}
	fn := symbol.Declarations()[0]
	a := analyzerFor(c)
	if !a.isSplittable(fn) || !a.isPlainFunction(fn) {
		return "", ""
	}
	signature := c.GetSignatureFromDeclaration(fn)
	effect := c.GetReturnTypeOfSignature(signature)
	success := successType(c, effect)
	if success == nil {
		return "", ""
	}
	text := "function " + symbol.Name() + c.SignatureToStringEx(c.SignatureWithReturnType(signature, success), node, hoverTypeFlags, nil)
	return text, "Plain JavaScript: EffectScript code calls it directly. TypeScript callers see `" + c.TypeToStringEx(effect, node, hoverTypeFlags, nil) + "`."
}
