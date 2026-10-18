package vibelang

import (
	"slices"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// Where programs start (design D38, Chapter 13). A call never starts a fiber;
// a program starts only when a runner receives it. A module's top level is not
// inside any program, so a call there may build values but not run a program
// (EK1001):
//
//	export const config = loadConfig()          // EK1001
//	NodeRuntime.runMain(() => main())           // a runner: allowed here
//
// A runner may be called only where no fiber runs the code: at a module's top
// level, and in a plain callback that such a place passes to a TypeScript
// function, such as a describe body or a framework's route handler.
// Anywhere else it would start a new root with an empty Context, outside the
// caller's structured concurrency (EK1005):
//
//	function refreshAll(ids: ReadonlyArray<string>) {
//	  for (const id of ids) Effect.runFork(() => refresh(id))   // EK1005
//	}
//
// The compiler knows runners by the declarations their calls resolve to.

// effectRootRunners are the runners that effect's Effect module exports and
// that start their program with an empty Context.
var effectRootRunners = []string{"runPromise", "runPromiseExit", "runSync", "runSyncExit", "runFork", "runCallback"}

// effectContextRunners are their *With forms, which take a Context.
var effectContextRunners = []string{"runPromiseWith", "runPromiseExitWith", "runSyncWith", "runSyncExitWith", "runForkWith", "runCallbackWith"}

// managedRuntimeRunners are the run methods of effect's ManagedRuntime.
var managedRuntimeRunners = []string{"runPromise", "runPromiseExit", "runSync", "runSyncExit", "runFork", "runCallback"}

// runnerKind tells a test registration from a runner that starts a program.
type runnerKind uint8

const (
	notRunner runnerKind = iota
	programRunner
	testRunner
)

// runnerOf reports whether call calls a runner, and which kind: Effect's
// run functions and their *With forms, ManagedRuntime's run methods,
// NodeRuntime.runMain and BunRuntime.runMain, and @effect/vitest's it.effect
// and it.live with their modifiers, .each, and .prop forms, and layer(…).
func runnerOf(c *checker.Checker, call *ast.Node) runnerKind {
	d, ok := calleeIdentity(c, call)
	switch {
	case !ok:
		return notRunner
	case d.pkg == "effect" && d.module == "Effect" && d.owner.Kind == ast.KindVariableDeclaration && slices.Contains(effectContextRunners, d.name),
		d.pkg == "effect" && d.module == "ManagedRuntime" && d.container == "ManagedRuntime" && slices.Contains(managedRuntimeRunners, d.name),
		isRootRunnerIdentity(d):
		return programRunner
	case d.pkg == "@effect/vitest":
		switch {
		case d.owner.Kind == ast.KindInterfaceDeclaration && d.name == "Test",
			d.container == "Tester" && (d.name == "each" || d.name == "prop"),
			d.name == "layer" && (d.container == "" || d.container == "MethodsNonLive" || d.container == "Methods"):
			return testRunner
		}
	}
	return notRunner
}

// isRootRunner reports whether call calls a runner whose program starts with
// an empty Context, so the program must need no service (D27): Effect's run
// functions, but not their *With forms, and NodeRuntime.runMain and
// BunRuntime.runMain.
func isRootRunner(c *checker.Checker, call *ast.Node) bool {
	d, ok := calleeIdentity(c, call)
	return ok && isRootRunnerIdentity(d)
}

func isRootRunnerIdentity(d identity) bool {
	switch {
	case d.pkg == "effect" && d.module == "Effect":
		return d.owner.Kind == ast.KindVariableDeclaration && slices.Contains(effectRootRunners, d.name)
	case d.pkg == "@effect/platform-node" && d.module == "NodeRuntime", d.pkg == "@effect/platform-bun" && d.module == "BunRuntime":
		return d.name == "runMain"
	}
	return false
}

// isTestRegistration reports whether call registers a test with
// @effect/vitest's it.effect or it.live, or a modifier of them, whose
// program a test fiber runs.
func isTestRegistration(c *checker.Checker, call *ast.Node) bool {
	d, ok := calleeIdentity(c, call)
	return ok && d.pkg == "@effect/vitest" && d.owner.Kind == ast.KindInterfaceDeclaration && d.name == "Test"
}

// identity names the declaration that a call resolves to.
type identity struct {
	pkg       string    // the package that declares it, such as "effect"
	module    string    // its module in the package, such as "Effect"
	container string    // the interface, class, or namespace that declares it, or ""
	name      string    // its name
	owner     *ast.Node // the declaration
}

// calleeIdentity returns the identity of the declaration whose signature
// call resolves to.
func calleeIdentity(c *checker.Checker, call *ast.Node) (identity, bool) {
	owner := ownerOf(signatureDeclaration(c, call))
	if owner == nil {
		return identity{}, false
	}
	file := ast.GetSourceFileOfNode(owner)
	return identity{pkg: packageOf(c, file), module: moduleOf(file), container: containerName(owner), name: declarationName(owner), owner: owner}, true
}

func init() {
	fileChecks = append(fileChecks, checkProgramStarts)
}

// checkProgramStarts reports EK1001 for each call at file's top level that runs a
// program, and EK1005 for each runner that code a fiber runs calls.
func checkProgramStarts(c *checker.Checker, file *ast.SourceFile) {
	a := analyzerFor(c)
	var visit func(node *ast.Node, topLevel bool) bool
	visit = func(node *ast.Node, topLevel bool) bool {
		if ast.IsFunctionLike(node) && node.Body() != nil {
			topLevel = false
		}
		if ast.IsTaggedTemplateExpression(node) && topLevel && !a.passedAsProgram(node) && a.runs(node) {
			report(c, node, EK1001, nodeText(file, node))
			return false
		}
		if ast.IsCallExpression(node) {
			if topLevel && !a.passedAsProgram(node) {
				if runs, explains := runsAtTopLevel(c, a, node); runs {
					reportTopLevelRun(c, file, node)
					if explains {
						cover(c, node)
					}
					return false // one error for the outermost program
				}
			}
			if kind := runnerOf(c, node); kind != notRunner && !isRunnerCallee(c, node) && !outsideFibers(c, a, node) {
				reportNestedRunner(c, file, node, kind)
			}
		}
		return node.ForEachChild(func(child *ast.Node) bool { return visit(child, topLevel) })
	}
	file.AsNode().ForEachChild(func(child *ast.Node) bool { return visit(child, true) })
}

// runsAtTopLevel reports whether call, at a module's top level, would run a
// program: a call that runs an Effect, a service member call such as
// Users.find(id), or a call of an Effect value such as Users() or Port(). The
// checker types the last two only inside function bodies, so at the top level
// TypeScript reports a member that does not exist or a value that is not
// callable, which EK1001 explains.
func runsAtTopLevel(c *checker.Checker, a *analyzer, call *ast.Node) (runs bool, explainsTypeScript bool) {
	if a.runs(call) {
		return true, false
	}
	if call.QuestionDotToken() != nil {
		return false, false
	}
	callee := call.Expression()
	if ast.IsPropertyAccessExpression(callee) && !ast.IsPrivateIdentifier(callee.Name()) {
		if identifier, _ := serviceMember(c, callee, c.GetApparentType(c.GetTypeAtLocation(callee.Expression()))); identifier != nil {
			return true, true
		}
	}
	if len(call.Arguments()) == 0 && call.TypeArguments() == nil {
		t := c.GetApparentType(c.GetTypeAtLocation(callee))
		if len(c.GetSignaturesOfType(t, checker.SignatureKindCall)) == 0 && effectOfValue(c, t) != nil {
			return true, true
		}
	}
	return false, false
}

// passedAsProgram reports whether call, at a module's top level, is an
// argument where a program is expected, as main() is in
// NodeRuntime.runMain(main()), or is evaluated by an expression passed by
// name, such as a pipe's left side whose topic is passed by name: a call
// there runs nothing and builds the program the callee receives (D147,
// D155), so EK1001 does not apply.
func (a *analyzer) passedAsProgram(call *ast.Node) bool {
	if a.byNameOf(call) != nil || holdsTopLevelProgram(a.checker, call) {
		return true
	}
	owner, path := ownerCall(call)
	if owner == nil || !isEffectType(a.checker, a.checker.GetTypeAtLocation(call)) {
		return false
	}
	signature := a.checker.GetResolvedSignature(owner)
	if signature == nil {
		return false
	}
	return a.classify(a.expectedAt(owner, signature, path, call)).program
}

// holdsTopLevelProgram recognizes D262's explicit storage of a cold program.
// At module scope calls retain their Effect type and emit without yield*;
// only the annotated const's initializer is a program position. Its operands
// still follow D147, so this does not hide eager calls inside the initializer.
func holdsTopLevelProgram(c *checker.Checker, call *ast.Node) bool {
	node := call
	for node.Parent != nil && ast.IsParenthesizedExpression(node.Parent) {
		node = node.Parent
	}
	decl := node.Parent
	if decl == nil || !ast.IsVariableDeclaration(decl) || decl.Initializer() != node || decl.Type() == nil {
		return false
	}
	list := decl.Parent
	return list != nil && list.Kind == ast.KindVariableDeclarationList && list.Flags&ast.NodeFlagsConst != 0 &&
		atTopLevel(decl) && isEffectType(c, c.GetTypeFromTypeNode(decl.Type()))
}

// reportTopLevelRun reports EK1001 for call, which runs a program at the
// module's top level.
func reportTopLevelRun(c *checker.Checker, file *ast.SourceFile, call *ast.Node) {
	text := nodeText(file, call.Expression()) + "()"
	if catchExpressionOf(call) != nil {
		text = nodeText(file, call) // x catch y, whose callee is the keyword (catch_expressions.go)
	}
	d := report(c, call, EK1001, text)
	if owner, _ := ownerCall(call); owner != nil && ast.IsCallExpression(owner) && runnerOf(c, owner) != notRunner {
		note(d, "Pass a program, a function with no parameters, to {0}:   () => {1}", nodeText(file, owner.Expression()), nodeText(file, call))
		return
	}
	note(d, "Export a function instead, or provide the value as a service with a Layer.")
}

// isRunnerCallee reports whether call is the callee of a runner call, as
// Effect.runPromiseWith(context) is in Effect.runPromiseWith(context)(program):
// the outer call is the one to report.
func isRunnerCallee(c *checker.Checker, call *ast.Node) bool {
	parent := call.Parent
	return parent != nil && ast.IsCallExpression(parent) && parent.Expression() == call && runnerOf(c, parent) != notRunner
}

// reportNestedRunner reports EK1005 for call, a runner inside code a fiber
// runs.
func reportNestedRunner(c *checker.Checker, file *ast.SourceFile, call *ast.Node, kind runnerKind) {
	runner := nodeText(file, call.Expression())
	d := report(c, call.Expression(), EK1005, runner, runningFunctionName(enclosingFunction(call)))
	if kind == testRunner {
		note(d, "Register tests at the module's top level, or in a describe callback.")
		return
	}
	program := "() => …"
	if arguments := call.Arguments(); len(arguments) > 0 {
		// fork receives a call by name (D147), as the runner did.
		program = nodeText(file, arguments[0])
	}
	note(d, "Call the program, or fork it so the block owns it:   fork({0})", program)
}

// runningFunctionName names fn, the function a fiber runs, for EK1005.
func runningFunctionName(fn *ast.Node) string {
	if name := functionName(fn); name != "" {
		return name
	}
	return "the enclosing function"
}
