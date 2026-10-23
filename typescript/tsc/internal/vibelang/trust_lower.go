package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// A call whose contract lists expected failures compiles to a runtime helper
// that turns those exceptions into failures (D5, D23, D51). In an Effect
// function,
//
//	const port = parsePortTrusted(text)
//
// becomes
//
//	const port = yield* runtime.$trusted(() => parsePortTrusted(text), [PortError])
//
// and inside a try with a catch in a function that compiles to plain
// JavaScript (D54) it becomes
//
//	const port = runtime.$trustedPlain(() => parsePortTrusted(text), [PortError])
//
// The generated code tests instanceof against each class. It imports a class
// from the module this file imports it, or the trusted function, from
// (reachClass); a global class such as SyntaxError needs no import. In a
// function with a plain contract, outside a try whose catch belongs to it,
// the call stays an ordinary call and its exceptions propagate as JavaScript
// exceptions (D282).

// trustedPlan records the calls whose expected failures the lowering
// converts.
type trustedPlan struct {
	trusted map[*ast.Node][]classRef // call -> the classes of its expected failures
}

func newTrustedPlan() trustedPlan {
	return trustedPlan{trusted: map[*ast.Node][]classRef{}}
}

// classRef says how generated code names a class at run time.
type classRef struct {
	module string // the module specifier to import it from, or "" for a name in scope
	name   string // its export's name, or its name in scope
}

// planTrustedCalls records the calls with expected failures in file's
// functions: each one in a function the lowering turns into an Effect or a
// program, and each one inside a try with a catch in a function that
// compiles to plain JavaScript.
func (p *plan) planTrustedCalls(c *checker.Checker, a *analyzer, file *ast.SourceFile) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if isLowerable(node) {
			effect := p.isEffect(node) || p.isProgram(node)
			a.forEachFailingCall(node, func(call *ast.Node, caught bool) bool {
				if !effect && !caught {
					return false
				}
				var refs []classRef
				for _, f := range a.expectedFailures(call) {
					module, name, ok := reachClass(c, file, f)
					if !ok {
						return false // EK4401 says why
					}
					refs = append(refs, classRef{module: module, name: name})
				}
				p.trusted[call] = refs
				return false
			})
		}
		node.ForEachChild(visit)
		return false
	}
	file.AsNode().ForEachChild(visit)
}

// expectedFailures returns the exceptions of call that are expected failures
// and join E: those its trusted contract lists, or those of a built-in's
// shipped contract. A call that runs an Effect has none.
func (a *analyzer) expectedFailures(call *ast.Node) []failure {
	if !isVibeLangFile(ast.GetSourceFileOfNode(call)) {
		return nil
	}
	// Trust first: a call to VibeLang code has no contract of its own, so
	// this never asks whether a .vibe callee is an Effect.
	t := callTrust(a.checker, call)
	if len(t.failures) == 0 || t.promise || a.performs(call) {
		return nil
	}
	if a.hasPlainContract(enclosingFunction(call)) && !inCatchingTry(call) {
		return nil // D282: a plain function's call of TypeScript code is a JavaScript call
	}
	return t.failures
}

// addFailures adds the expected failures of call to info.
func (a *analyzer) addFailures(info *effectInfo, call *ast.Node) {
	for _, f := range a.expectedFailures(call) {
		info.addError(f.t, call)
	}
}

// hasEscapingFailure reports whether a call in fn's own body has expected
// failures that can end fn: one that no try block of fn with a catch clause
// encloses. Such a function is an Effect, as one with an escaping throw is.
func (a *analyzer) hasEscapingFailure(fn *ast.Node) bool {
	if a.isBodyDeferred(fn) {
		return false
	}
	return a.forEachFailingCall(fn, func(call *ast.Node, caught bool) bool { return !caught })
}

// forEachFailingCall calls cb for each call in fn's own body that has
// expected failures, with whether a try block of fn with a catch clause
// encloses it, until cb returns true. It reports whether cb returned true.
func (a *analyzer) forEachFailingCall(fn *ast.Node, cb func(call *ast.Node, caught bool) bool) bool {
	var visit func(n *ast.Node, caught bool) bool
	visit = func(n *ast.Node, caught bool) bool {
		switch {
		case isFunctionBoundary(n):
			return false
		case n.Kind == ast.KindTryStatement:
			try := n.AsTryStatement()
			return visit(try.TryBlock, caught || catches(n)) ||
				try.CatchClause != nil && visit(try.CatchClause, caught) ||
				try.FinallyBlock != nil && visit(try.FinallyBlock, caught)
		case n.Kind == ast.KindCallExpression && len(a.expectedFailures(n)) > 0 && cb(n, caught):
			return true
		}
		return n.ForEachChild(func(child *ast.Node) bool { return visit(child, caught) })
	}
	return fn.Body() != nil && visit(fn.Body(), false /*caught*/)
}

// namedImport is an import that the lowering adds: `import { name as local } from "module"`.
type namedImport struct {
	module string
	name   string
	local  *ast.IdentifierNode
}

// classReference returns an expression for ref's class.
func (tx *lowering) classReference(ref classRef) *ast.Node {
	if ref.module == "" {
		return tx.Factory().NewIdentifier(ref.name)
	}
	for _, imported := range tx.imports {
		if imported.module == ref.module && imported.name == ref.name {
			return imported.local
		}
	}
	local := tx.fileName(ref.name)
	tx.imports = append(tx.imports, namedImport{module: ref.module, name: ref.name, local: local})
	return local
}

// namedImportDeclarations returns the declarations of the imports the
// lowering added.
func (tx *lowering) namedImportDeclarations() []*ast.Node {
	f := tx.Factory()
	var declarations []*ast.Node
	for _, imported := range tx.imports {
		specifier := f.NewImportSpecifier(false /*isTypeOnly*/, f.NewIdentifier(imported.name), imported.local)
		declaration := f.NewImportDeclaration(
			nil, /*modifiers*/
			f.NewImportClause(ast.KindUnknown, nil /*name*/, f.NewNamedImports(f.NewNodeList([]*ast.Node{specifier}))),
			f.NewStringLiteral(imported.module, ast.TokenFlagsNone),
			nil, /*attributes*/
		)
		ast.SetParentInChildren(declaration)
		declarations = append(declarations, declaration)
	}
	return declarations
}

// visitTrustedCall turns a call with expected failures in an Effect function
// into `yield* runtime.$trusted(thunk, [classes])`.
func (tx *lowering) visitTrustedCall(node *ast.Node) *ast.Node {
	f := tx.Factory()
	run := f.NewYieldExpression(f.NewToken(ast.KindAsteriskToken), tx.runtimeCall("$trusted", tx.trustedThunk(node), tx.classArray(node)))
	tx.EmitContext().SetOriginal(run, node)
	tx.EmitContext().AssignCommentAndSourceMapRanges(run, node)
	return run
}

// visitTrustedPlainCall turns a call with expected failures in a function
// that compiles to plain JavaScript into `runtime.$trustedPlain(thunk, [classes])`.
func (tx *lowering) visitTrustedPlainCall(node *ast.Node) *ast.Node {
	call := tx.runtimeCall("$trustedPlain", tx.trustedThunk(node), tx.classArray(node))
	tx.EmitContext().SetOriginal(call, node)
	tx.EmitContext().AssignCommentAndSourceMapRanges(call, node)
	return call
}

func (tx *lowering) classArray(node *ast.Node) *ast.Node {
	var classes []*ast.Node
	for _, ref := range tx.plan.trusted[node] {
		classes = append(classes, tx.classReference(ref))
	}
	return tx.Factory().NewArrayLiteralExpression(tx.Factory().NewNodeList(classes), false /*multiLine*/)
}

// trustedThunk returns a function with no parameters that makes node's call:
// `() => f(x)`. When the visited call's operands yield, which a function
// without `*` cannot, the operands are evaluated first, in order, and passed
// in: `((receiver, args) => () => receiver.m(...args))(yield* r(), [yield* x()])`.
func (tx *lowering) trustedThunk(node *ast.Node) *ast.Node {
	f := tx.Factory()
	call := tx.Visitor().VisitEachChild(node)
	if !containsYield(call) {
		return tx.arrow(nil, call)
	}
	var parameters, values []*ast.Node
	hoist := func(name string, value *ast.Node) *ast.Node {
		parameter := f.NewUniqueName(name)
		parameters = append(parameters, f.NewParameterDeclaration(nil /*modifiers*/, nil /*dotDotDotToken*/, parameter, nil /*questionToken*/, nil /*typeNode*/, nil /*initializer*/))
		values = append(values, value)
		return parameter
	}
	callee := call.Expression()
	optional := ast.IsOptionalChain(callee)
	switch {
	case ast.IsPropertyAccessExpression(callee) && !optional:
		callee = f.UpdatePropertyAccessExpression(callee.AsPropertyAccessExpression(), hoist("receiver", callee.Expression()), nil /*questionDotToken*/, callee.Name(), ast.NodeFlagsNone)
	case ast.IsElementAccessExpression(callee) && !optional:
		access := callee.AsElementAccessExpression()
		receiver := hoist("receiver", access.Expression)
		callee = f.UpdateElementAccessExpression(access, receiver, nil /*questionDotToken*/, hoist("key", access.ArgumentExpression), ast.NodeFlagsNone)
	case optional, ast.IsIdentifier(callee):
		// An optional chain stays inside the thunk, which keeps its this; so does a name.
	default:
		callee = hoist("callee", callee)
	}
	arguments := hoist("args", f.NewArrayLiteralExpression(f.NewNodeList(call.Arguments()), false /*multiLine*/))
	inner := f.NewCallExpression(callee, call.QuestionDotToken(), nil /*typeArguments*/, f.NewNodeList([]*ast.Node{f.NewSpreadElement(arguments)}), ast.NodeFlagsNone)
	outer := f.NewParenthesizedExpression(tx.arrow(parameters, tx.arrow(nil, inner)))
	return f.NewCallExpression(outer, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList(values), ast.NodeFlagsNone)
}

// arrow returns `(parameters) => body`.
func (tx *lowering) arrow(parameters []*ast.Node, body *ast.Node) *ast.Node {
	f := tx.Factory()
	return f.NewArrowFunction(nil /*modifiers*/, nil /*typeParameters*/, f.NewNodeList(parameters), nil /*typeNode*/, nil /*fullSignature*/, f.NewToken(ast.KindEqualsGreaterThanToken), body)
}

// containsYield reports whether node, outside the functions nested in it,
// contains a yield expression.
func containsYield(node *ast.Node) bool {
	if node.Kind == ast.KindYieldExpression {
		return true
	}
	if isFunctionBoundary(node) {
		return false
	}
	return node.ForEachChild(containsYield)
}
