package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// Entry modules (D135).
//
//	export function main() {
//	  Console.log("Hello")
//	}
//
// A module that exports a function declaration named main with no parameters
// runs it when the host starts the module: the emitted JavaScript ends with
//
//	if (import.meta.main) NodeRuntime.runMain(main());
//
// with NodeRuntime imported from @effect/platform-node when that package
// resolves from the module, and runtime.$runMain, the runtime's copy of the
// same runner, otherwise. import.meta.main is true only in the module the
// host started, so importing the module, from a test or another module, runs
// nothing. A module that calls a runner at its top level starts its program
// itself and gets no entry statement.

// platformNodeModule is the package whose NodeRuntime runs an entry module.
const platformNodeModule = "@effect/platform-node"

// runnerNames are the functions that start a program from a module's top level.
var runnerNames = map[string]bool{"runMain": true, "runPromise": true, "runFork": true, "runSync": true, "runCallback": true}

// entryMain returns the declaration of file's exported main function with no
// parameters, or nil when file has none or calls a runner at its top level.
func entryMain(file *ast.SourceFile) *ast.Node {
	if !isVibeLangFile(file) {
		return nil
	}
	var main *ast.Node
	for _, statement := range file.Statements.Nodes {
		switch {
		case statement.Kind == ast.KindFunctionDeclaration && statement.Name() != nil && statement.Name().Text() == "main" &&
			ast.HasSyntacticModifier(statement, ast.ModifierFlagsExport) && !ast.HasSyntacticModifier(statement, ast.ModifierFlagsDefault) &&
			len(statement.Parameters()) == 0 && statement.Body() != nil:
			main = statement
		case containsTopLevelRunner(statement):
			return nil
		}
	}
	return main
}

// containsTopLevelRunner finds runner calls in expressions and initializers
// that execute during module evaluation, without descending into delayed
// function bodies or instance field initializers.
func containsTopLevelRunner(node *ast.Node) bool {
	if ast.IsFunctionLike(node) {
		return false
	}
	if node.Kind == ast.KindPropertyDeclaration && !ast.HasSyntacticModifier(node, ast.ModifierFlagsStatic) {
		return false
	}
	return callsRunner(node) || node.ForEachChild(containsTopLevelRunner)
}

// callsRunner reports whether expression calls a runner, such as
// NodeRuntime.runMain(…) or runPromise(…).
func callsRunner(expression *ast.Node) bool {
	expression = ast.SkipParentheses(expression)
	if !ast.IsCallExpression(expression) {
		return false
	}
	switch callee := expression.Expression(); {
	case ast.IsIdentifier(callee):
		return runnerNames[callee.Text()]
	case ast.IsPropertyAccessExpression(callee):
		return runnerNames[callee.Name().Text()]
	}
	return false
}

// An entryPlan records how the lowering runs an entry module's main.
type entryPlan struct {
	main        *ast.Node // the main function's declaration
	nodeRuntime bool      // @effect/platform-node resolves from the module
}

// planEntry returns the entry plan of file, or nil when file is not an entry
// module. The program resolves @effect/platform-node from an entry module as
// an implicit import (implicitRuntimeImport), so the plan can ask whether it
// resolved.
func planEntry(c *checker.Checker, file *ast.SourceFile) *entryPlan {
	main := entryMain(file)
	if main == nil {
		return nil
	}
	entry := &entryPlan{main: main}
	for key, resolved := range c.Program().GetResolvedModules()[file.PathKey()] {
		if key.Name == platformNodeModule && resolved.IsResolved() {
			entry.nodeRuntime = true
		}
	}
	return entry
}

// entryStatement makes `if (import.meta.main) NodeRuntime.runMain(main())`,
// or the same with runtime.$runMain. A main that compiles to plain JavaScript
// runs through Effect.sync.
func (tx *lowering) entryStatement() *ast.Node {
	f := tx.Factory()
	entry := tx.plan.entry
	program := f.NewCallExpression(f.NewIdentifier("main"), nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList(nil), ast.NodeFlagsNone)
	if !tx.plan.isEffect(entry.main) {
		sync := f.NewPropertyAccessExpression(tx.effectName(), nil /*questionDotToken*/, f.NewIdentifier("sync"), ast.NodeFlagsNone)
		program = f.NewCallExpression(sync, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{f.NewIdentifier("main")}), ast.NodeFlagsNone)
	}
	var runner *ast.Node
	if entry.nodeRuntime {
		nodeRuntime := tx.classReference(classRef{module: platformNodeModule, name: "NodeRuntime"})
		runner = f.NewPropertyAccessExpression(nodeRuntime, nil /*questionDotToken*/, f.NewIdentifier("runMain"), ast.NodeFlagsNone)
	} else {
		runner = tx.runtimeMember("$runMain")
	}
	run := f.NewExpressionStatement(f.NewCallExpression(runner, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{program}), ast.NodeFlagsNone))
	meta := f.NewPropertyAccessExpression(f.NewMetaProperty(ast.KindImportKeyword, f.NewIdentifier("meta")), nil /*questionDotToken*/, f.NewIdentifier("main"), ast.NodeFlagsNone)
	return f.NewIfStatement(meta, run, nil /*elseStatement*/)
}
