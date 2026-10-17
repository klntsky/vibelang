package effeck

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// The boundary hatches (design D148) run plain TypeScript inside Effeck:
//
//	const id = sync(crypto.randomUUID())                   // Effect.sync
//	const page = sync(async () => (await fetch(url)).text()) // Effect.promise
//	const parsed = try(() => JSON.parse(text))             // Effect.try: a throw is UnknownError
//	const body = try(async (signal) => fetch(url, { signal }), (error) => new Offline({ error }))
//	const tick = callback<number>((resume) => {            // Effect.callback
//	  const timer = setTimeout(() => resume(1), 10)
//	  return () => clearTimeout(timer)
//	})
//
// A hatch's arguments are not Effeck: a call there does not run, so calling an
// .effeck function gives an inert Effect value for runPromise or runFork;
// throw, async, and await keep their JavaScript meaning; platform APIs are
// allowed (no EK4401, EK4402, or EK4403); and a runner may start a program
// (no EK1005). A hatch is a call whose callee is the prelude's sync, try, or
// callback: the name, with no declaration in any scope around the call that
// shadows it. try is a keyword, so nothing can shadow it.

// hatchNames are the prelude's hatches.
var hatchNames = map[string]bool{"sync": true, "try": true, "callback": true}

// hatchOf returns the name of the hatch that call calls, or "".
func hatchOf(call *ast.Node) string {
	if call == nil || !ast.IsCallExpression(call) {
		return ""
	}
	callee := call.Expression()
	if !ast.IsIdentifier(callee) || !hatchNames[callee.Text()] {
		return ""
	}
	name := callee.Text()
	if !isEffeckFile(ast.GetSourceFileOfNode(call)) || name != "try" && shadowed(call, name) {
		return ""
	}
	return name
}

// shadowed reports whether a scope around node declares name: a block, a
// function, or the module, whose locals the binder recorded.
func shadowed(node *ast.Node, name string) bool {
	for n := node; n != nil; n = n.Parent {
		if locals := n.Locals(); locals != nil {
			if _, ok := locals[name]; ok {
				return true
			}
		}
	}
	return false
}

// isHatchArgument reports whether node is an argument of a hatch call.
func isHatchArgument(node *ast.Node) bool {
	parent := node.Parent
	return parent != nil && ast.IsCallExpression(parent) && parent.Expression() != node && hatchOf(parent) != ""
}

// inHatch reports whether node is a hatch's argument or inside one, where
// TypeScript, not Effeck, gives the code its meaning.
func inHatch(node *ast.Node) bool {
	for n := node; n != nil && n.Parent != nil; n = n.Parent {
		if isHatchArgument(n) {
			return true
		}
	}
	return false
}

func init() {
	fileChecks = append(fileChecks, checkHatches)
}

// checkHatches reports EK4404 at try({ try: …, catch: … }): Effect's options
// object is a value to the hatch, which would succeed with the object itself.
// The hatch takes the code and the error mapping as two arguments.
func checkHatches(c *checker.Checker, file *ast.SourceFile) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if hatchOf(node) == "try" {
			if arguments := node.Arguments(); len(arguments) == 1 && ast.IsObjectLiteralExpression(arguments[0]) && hasProperty(arguments[0], "try") {
				d := report(c, arguments[0], EK4404, "try")
				note(d, "Write try(code, (error) => …), as in try(parse(text), (error) => new BadInput({ error })).")
				cover(c, arguments[0]) // TypeScript's errors inside the object restate EK4404
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// hasProperty reports whether object, an object literal, has a property
// named name.
func hasProperty(object *ast.Node, name string) bool {
	for _, property := range object.AsObjectLiteralExpression().Properties.Nodes {
		if n := property.Name(); n != nil && ast.IsIdentifier(n) && n.Text() == name {
			return true
		}
	}
	return false
}

// hatchForm is what the lowering emits for a sync or try hatch: the Effect
// function, and whether the first argument is an expression that the lowering
// wraps in a function.
type hatchForm struct {
	effect string // "sync", "promise", "try", or "tryPromise"
	value  bool   // sync(expression), try(expression): wrap it in () =>
}

// hatchFormOf returns the form of call, a sync or try hatch, from the overload
// the checker selected. The runtime names each overload's first parameter for
// the compiler: evaluateAsync, evaluate, promise, or value.
func hatchFormOf(c *checker.Checker, call *ast.Node) (hatchForm, bool) {
	name := hatchOf(call)
	if name != "sync" && name != "try" {
		return hatchForm{}, false
	}
	signature := c.GetResolvedSignature(call)
	if signature == nil || signature.Declaration() == nil {
		return hatchForm{}, false
	}
	parameters := signature.Declaration().Parameters()
	if len(parameters) == 0 || !ast.IsIdentifier(parameters[0].Name()) {
		return hatchForm{}, false
	}
	var async, value bool
	switch parameters[0].Name().Text() {
	case "evaluateAsync":
		async = true
	case "evaluate":
	case "promise":
		async, value = true, true
	case "value":
		value = true
	default:
		return hatchForm{}, false
	}
	effect := name
	switch {
	case name == "sync" && async:
		effect = "promise"
	case name == "try" && async:
		effect = "tryPromise"
	}
	return hatchForm{effect: effect, value: value}, true
}

// visitHatchCall lowers a sync or try hatch to Effect's function:
//
//	sync(() => f())              Effect.sync(() => f())
//	sync(fetch(url))             Effect.promise(() => fetch(url))
//	try(() => parse(s), wrap)    Effect.try({ try: () => parse(s), catch: wrap })
//
// The arguments stay TypeScript: their calls do not run. The call itself
// returns an Effect, so it runs as yield* where calls run.
func (tx *lowering) visitHatchCall(node *ast.Node) *ast.Node {
	form := tx.plan.hatches[node]
	f := tx.Factory()
	saved, savedPlain := tx.inEffect, tx.plain
	tx.inEffect, tx.plain = false, false
	var arguments []*ast.Node
	for _, argument := range node.Arguments() {
		arguments = append(arguments, tx.Visitor().VisitNode(argument))
	}
	tx.inEffect, tx.plain = saved, savedPlain
	if form.value && len(arguments) > 0 {
		arguments[0] = f.NewArrowFunction(nil /*modifiers*/, nil /*typeParameters*/, f.NewNodeList(nil), nil /*returnType*/, nil /*fullSignature*/, f.NewToken(ast.KindEqualsGreaterThanToken), arguments[0])
	}
	if len(arguments) == 2 {
		options := f.NewObjectLiteralExpression(f.NewNodeList([]*ast.Node{
			f.NewPropertyAssignment(nil /*modifiers*/, f.NewIdentifier("try"), nil /*postfixToken*/, nil /*typeNode*/, arguments[0]),
			f.NewPropertyAssignment(nil /*modifiers*/, f.NewIdentifier("catch"), nil /*postfixToken*/, nil /*typeNode*/, arguments[1]),
		}), false /*multiLine*/)
		arguments = []*ast.Node{options}
	}
	callee := f.NewPropertyAccessExpression(tx.effectName(), nil /*questionDotToken*/, f.NewIdentifier(form.effect), ast.NodeFlagsNone)
	lowered := f.NewCallExpression(callee, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList(arguments), ast.NodeFlagsNone)
	tx.EmitContext().SetOriginal(lowered, node)
	lowered.Loc = node.Loc
	if tx.inEffect && tx.plan.runsCall(node) {
		run := f.NewYieldExpression(f.NewToken(ast.KindAsteriskToken), lowered)
		tx.EmitContext().SetOriginal(run, node)
		return run
	}
	return lowered
}
