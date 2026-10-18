package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// Scope regions (design D14, D19, D25, D47; spec Chapters 6, 7, and 26).
//
// A block of an Effect function that declares using or calls fork is a Scope
// region. It runs under a Scope of its own, which only its using
// acquisitions and the fibers it forks receive, and which closes when the
// block exits by any path: success, failure, or interruption. Closing runs
// the releases and interrupts the forked fibers, newest first, and waits for
// each before going on:
//
//	function serve(port: number) {
//	  using server = Http.listen(port)   // Effect<Server, never, Scope>: gets the block's Scope
//	  fork(() => heartbeat())            // interrupted when serve exits, before the server closes
//	  return server.awaitShutdown()
//	}
//	// Effect<string, never, Http>
//
// Everything else in the block runs under the Scope its caller supplies, so a
// resource acquired without using belongs to the caller, and Scope stays in
// R (D14). A fork that no block encloses, in an arrow function whose body is
// an expression, is Effect.forkScoped and adds Scope to R. A function that
// declares using is an Effect function (D54), even when it runs nothing else,
// so its using is always a Scope region; await using is EK1003, and the
// lowering treats it as using.
//
// using decides by its initializer: a call that runs an Effect is an
// acquisition, whose Effect runs with the block's Scope and whose own release
// frees it; any other initializer is a value, and TypeScript checks that it
// is a Disposable, null, or undefined. A value that is a Disposable, from
// either kind, is also released with [Symbol.dispose]() when the block exits.
// using adds no masking of its own: an acquisition is masked or interruptible
// as its Effect.acquireRelease says (D25). for (using x of values) takes each
// value with using in a Scope region around that iteration's body, so the
// value is released when the iteration ends, by any path.

func init() {
	checker.RegisterUsingInitializerHook(usingInitializer)
}

// isBlockUsing reports whether node is a using statement directly in a
// block, the form a Scope region lowers. A using in a for head, or in a case
// clause, which TypeScript rejects, stays as TypeScript emits it.
func isBlockUsing(node *ast.Node) bool {
	return node.Kind == ast.KindVariableStatement && isUsingList(node.AsVariableStatement().DeclarationList) &&
		node.Parent != nil && node.Parent.Kind == ast.KindBlock
}

// isUsingForOf reports whether node is `for (using x of values) body`, whose
// body the lowering makes a Scope region for each iteration.
func isUsingForOf(node *ast.Node) bool {
	if node.Kind != ast.KindForOfStatement {
		return false
	}
	initializer := node.AsForInOrOfStatement().Initializer
	return initializer != nil && initializer.Kind == ast.KindVariableDeclarationList && isUsingList(initializer)
}

// ownerBlock returns the innermost block around node within its function, or
// nil when no block encloses node there, as in an arrow function whose body is
// an expression.
func ownerBlock(node *ast.Node) *ast.Node {
	for n := node.Parent; n != nil && !isFunctionBoundary(n); n = n.Parent {
		if n.Kind == ast.KindBlock {
			return n
		}
	}
	return nil
}

// usingDeclarationOf returns the declaration whose initializer is node,
// skipping parentheses, when that declaration belongs to a using statement
// directly in a block; otherwise nil.
func usingDeclarationOf(node *ast.Node) *ast.Node {
	parent := node.Parent
	for parent != nil && parent.Kind == ast.KindParenthesizedExpression {
		node, parent = parent, parent.Parent
	}
	if parent == nil || parent.Kind != ast.KindVariableDeclaration || parent.Initializer() != node {
		return nil
	}
	if list := parent.Parent; list != nil && list.Parent != nil && isBlockUsing(list.Parent) {
		return parent
	}
	return nil
}

// isAcquisition reports whether call is an acquisition: the initializer of a
// using declaration in a block, and a call that runs an Effect. Its Effect
// runs with the block's Scope, so Scope leaves its R.
func (a *analyzer) isAcquisition(call *ast.Node) bool {
	return usingDeclarationOf(call) != nil && a.runs(call)
}

// usingInitializer is the checker's using initializer hook. An acquisition
// needs no [Symbol.dispose](): the resource's own release frees it.
func usingInitializer(c *checker.Checker, declaration *ast.Node) bool {
	initializer := declaration.Initializer()
	if initializer == nil || !isVibeLangFile(ast.GetSourceFileOfNode(declaration)) {
		return false
	}
	initializer = ast.SkipParentheses(initializer)
	if initializer.Kind != ast.KindCallExpression || !inLowerableBody(initializer) {
		return false
	}
	return analyzerFor(c).isAcquisition(initializer)
}

// forkOwner returns the block whose Scope region owns the fiber that call
// starts, or nil. call must be a call to the prelude's fork that runs, which
// is fork(program), in the body of a function the lowering can turn into an
// Effect, inside a block of that body.
func (a *analyzer) forkOwner(call *ast.Node) *ast.Node {
	if !a.isPreludeFork(call) || !inLowerableBody(call) || !a.runs(call) {
		return nil
	}
	return ownerBlock(call)
}

// isPreludeFork reports whether call calls the runtime's fork: the prelude's,
// or one imported from vibelang-runtime.
func (a *analyzer) isPreludeFork(call *ast.Node) bool {
	return isRuntimeExport(a.checker, ast.SkipParentheses(call.Expression()), "fork")
}

// getsScope reports whether call runs with the Scope of a Scope region: it
// forks a fiber the region owns, or it is a using acquisition.
func (a *analyzer) getsScope(call *ast.Node) bool {
	return a.forkOwner(call) != nil || a.isAcquisition(call)
}

// addScoped adds to info what call, which getsScope, adds to E and R. A fork
// adds the requirements of the program it starts, whose fiber sees this
// fiber's services, and none of its failures, which reach the parent only
// through Fiber.join. An acquisition adds its failures and its requirements
// other than Scope, plus the service its callee looks up.
func (a *analyzer) addScoped(info *effectInfo, call *ast.Node) {
	c := a.checker
	if a.forkOwner(call) != nil {
		if arguments := call.Arguments(); len(arguments) > 0 {
			program := c.GetTypeAtLocation(arguments[0])
			if r := channelType(c, program, "_R"); r != nil {
				info.addRequirement(r, call)
				if info.effect == nil {
					info.effect = effectInterface(c, program)
				}
				return
			}
		}
		forked := &effectInfo{}
		a.addCall(forked, call)
		forked = withoutScope(forked)
		forked.errors, forked.errorSources = nil, nil
		info.add(forked)
		return
	}
	a.addServiceLookup(info, call)
	acquisition := &effectInfo{}
	a.addCall(acquisition, call)
	info.add(withoutScope(acquisition))
}

// withoutScope returns info without effect's Scope among its requirements,
// splitting unions and keeping each requirement's source: a Scope region
// supplies the Scope that its acquisitions and forks ask for.
func withoutScope(info *effectInfo) *effectInfo {
	out := &effectInfo{errors: info.errors, errorSources: info.errorSources, effect: info.effect}
	for i, r := range info.requirements {
		var source *ast.Node
		if i < len(info.requirementSources) {
			source = info.requirementSources[i]
		}
		members := []*checker.Type{r}
		if r.Flags()&checker.TypeFlagsUnion != 0 {
			members = r.Types()
		}
		for _, member := range members {
			if !isScopeType(member) {
				out.addRequirement(member, source)
			}
		}
	}
	return out
}

// isScopeType reports whether t is effect's Scope.Scope.
func isScopeType(t *checker.Type) bool {
	symbol := t.Symbol()
	if symbol == nil || symbol.Name() != "Scope" || symbol.Flags()&ast.SymbolFlagsInterface == 0 {
		return false
	}
	for _, declaration := range symbol.Declarations() {
		if isEffectModuleFile(ast.GetSourceFileOfNode(declaration), "Scope") {
			return true
		}
	}
	return false
}
