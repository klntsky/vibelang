package effeck

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/printer"
	"github.com/microsoft/TypeScript/tsc/internal/transformers"
)

// lowering rewrites one .effeck file so it runs on Effect 4. It runs before the
// TypeScript transforms, which then erase types and convert modules as usual.
// A function that the plan marks as an Effect becomes an Effect.fnUntraced
// generator, and each call in it that the plan marks as running becomes a
// yield*:
//
//	export function greet(name: string) {
//	  Console.log(`hello, ${name}`)
//	  return name.length
//	}
//
// becomes
//
//	import * as Effect from "effect/Effect"
//	export const greet = Effect.fnUntraced(function* (name: string) {
//	  yield* Console.log(`hello, ${name}`)
//	  return name.length
//	})
type lowering struct {
	transformers.Transformer
	plan     *plan
	effect   *ast.IdentifierNode // local name for effect's Effect module, imported on first use
	runtime  *ast.IdentifierNode // local name for effeck-runtime, imported on first use
	inEffect bool                // visiting code that runs inside a lowered generator
	frame    *frame              // the generator being emitted while inEffect
	plain    bool                // visiting the body of an .effeck function that compiles to plain JavaScript
	imports  []namedImport       // named imports the lowering adds, such as a @throws class

	// scopes maps each Scope region's block to the name of its Scope (scopes_lower.go).
	scopes map[*ast.Node]*ast.Node

	// modules maps an effect module, such as effect/Stream, to the local name
	// of its namespace import, for method calls (datamethods.go).
	modules     map[string]*ast.IdentifierNode
	moduleOrder []string

	// topicNames maps a pipe that keeps its left side's value in a variable
	// to that variable's name where the lowering is; rebinding marks the
	// function being lowered inside an arrow function that rebinds them
	// (pipes_lower.go).
	topicNames map[*ast.Node]*ast.IdentifierNode
	rebinding  map[*ast.Node]bool
}

func newLowering(opts *transformers.TransformOptions, file *ast.SourceFile) *transformers.Transformer {
	if !isEffeckFile(file) {
		return nil
	}
	tx := &lowering{plan: planFile(opts.EmitResolver, file)}
	return tx.NewTransformer(tx.visit, opts.Context)
}

func (tx *lowering) visit(node *ast.Node) *ast.Node {
	if pipes := tx.plan.topicClosures[node]; len(pipes) != 0 && !tx.rebinding[node] {
		return tx.visitTopicClosure(node, pipes)
	}
	if tx.plan.conversionOf(node) == convertByName {
		return tx.byNameProgram(node)
	}
	if ref := tx.plan.directRefs[node]; ref != nil {
		return tx.visitDirectReference(node, ref)
	}
	if conversion := tx.plan.conversionOf(node); conversion != convertNone {
		return tx.convert(node, conversion)
	}
	return tx.visitNode(node)
}

func (tx *lowering) visitNode(node *ast.Node) *ast.Node {
	switch {
	case node.Kind == ast.KindSourceFile:
		return tx.visitSourceFile(node.AsSourceFile())
	case node.Kind == ast.KindImportDeclaration && len(tx.plan.directImports[node]) != 0:
		return tx.visitImportWithDirectVariants(node)
	case node.Kind == ast.KindImportDeclaration || node.Kind == ast.KindExportDeclaration:
		return tx.visitImportOrExport(node)
	case isPipe(node):
		return tx.visitPipe(node)
	case isTopic(node):
		return tx.visitTopic(node)
	case node.Kind == ast.KindCallExpression && tx.plan.hatches[node] != (hatchForm{}):
		return tx.visitHatchCall(node)
	case node.Kind == ast.KindCallExpression && tx.plan.methods[node].module != "":
		return tx.visitModuleMethodCall(node)
	case tx.inEffect && tx.plan.scoped(node):
		return tx.visitScoped(node)
	case node.Kind == ast.KindCallExpression && tx.inEffect && tx.plan.values[node]:
		return tx.visitEffectValueCall(node)
	case node.Kind == ast.KindCallExpression && tx.inEffect && tx.plan.runsCall(node):
		return tx.visitRunningCall(node)
	case node.Kind == ast.KindCallExpression && tx.inEffect && tx.plan.trusted[node] != nil:
		return tx.visitTrustedCall(node)
	case node.Kind == ast.KindCallExpression && tx.plain && tx.plan.trusted[node] != nil:
		return tx.visitTrustedPlainCall(node)
	case node.Kind == ast.KindPropertyAccessExpression && tx.inEffect && tx.plan.looksUp(node):
		return tx.visitServiceLookup(node)
	case node.Kind == ast.KindClassDeclaration && tx.plan.keys[node] != "":
		return tx.visitServiceClass(node)
	case tx.inEffect && isHandleStatement(node):
		return tx.visitHandleStatement(node) // try { ... } handle Control.op(...) { ... } (handles.go)
	case tx.inEffect && node.Kind == ast.KindTryStatement:
		return tx.visitTryStatement(node)
	case tx.inEffect && isProvideBlock(node):
		return tx.visitProvideBlock(node)
	case isFailureTest(node):
		return tx.visitFailureTest(node)
	case isProvideLayer(node):
		return nil // visitProvideBlock lowers the layer expression itself
	case isClauseBinding(node):
		return nil // clauseFunction binds a handle clause's parameters itself
	case tx.inEffect && tx.frame.handleClause != nil && node.Kind == ast.KindReturnStatement && tx.isClauseResume(node.Expression()):
		return tx.visitClauseResumeReturn(node)
	case tx.inEffect && tx.frame.handleClause != nil && node.Kind == ast.KindExpressionStatement && tx.isClauseResume(node.Expression()):
		return tx.visitClauseResume(node)
	case tx.inEffect && node.Kind == ast.KindThrowStatement:
		return tx.visitThrowStatement(node)
	case tx.inEffect && tx.frame.clause && node.Kind == ast.KindReturnStatement && tx.plan.tailResumeOf(node.Expression()) != nil:
		return tx.visitTailResumeReturn(node)
	case tx.inEffect && tx.frame.clause && node.Kind == ast.KindCallExpression && tx.plan.resumes[node] != "" && tx.plan.isTailResumeBody(node):
		return tx.tailResumeCall(node)
	case tx.inEffect && node.Kind == ast.KindYieldExpression:
		return tx.visitYield(node)
	case tx.inEffect && node.Kind == ast.KindForOfStatement && tx.plan.streamLoops[node]:
		return tx.visitStreamLoop(node)
	case tx.inEffect && node.Kind == ast.KindLabeledStatement && tx.plan.streamLoops[node.AsLabeledStatement().Statement]:
		return tx.visitStreamLoop(node.AsLabeledStatement().Statement) // $forOf takes the label
	case tx.inEffect && (node.Kind == ast.KindReturnStatement || ast.IsBreakOrContinueStatement(node)):
		return tx.visitJump(node)
	case tx.inEffect && isJumpTarget(node):
		return tx.visitJumpTarget(node)
	case tx.inEffect && tx.frame.region != nil && varList(node) != nil:
		return tx.visitVar(node)
	case tx.plain && node.Kind == ast.KindThrowStatement:
		return tx.visitPlainThrowStatement(node)
	case tx.plain && node.Kind == ast.KindCatchClause:
		return tx.visitPlainCatchClause(node)
	case node.Kind == ast.KindFunctionDeclaration && tx.plan.directs[node]:
		return tx.visitPlainFunctionDeclaration(node)
	case node.Kind == ast.KindCallExpression && tx.plan.wireOrders[node] != nil:
		return tx.visitWireCall(node)
	case tx.plan.producers[node]:
		return tx.visitProducer(node)
	case node.Kind == ast.KindFunctionDeclaration && tx.plan.isEffect(node):
		return tx.visitEffectFunctionDeclaration(node)
	case node.Kind == ast.KindMethodDeclaration && tx.plan.isEffect(node):
		return tx.lowerMethod(node)
	case tx.plan.isEffect(node):
		return tx.lowerFunction(node)
	case tx.plan.isProgram(node):
		return tx.lowerProgram(node)
	case isFunctionBoundary(node):
		saved, savedPlain := tx.inEffect, tx.plain
		tx.inEffect = false
		tx.plain = isLowerable(node) // an .effeck function that is not an Effect
		defer func() { tx.inEffect, tx.plain = saved, savedPlain }()
	}
	return tx.Visitor().VisitEachChild(node)
}

func (tx *lowering) visitSourceFile(file *ast.SourceFile) *ast.Node {
	visited := tx.Visitor().VisitEachChild(file.AsNode())
	if tx.plan.entry != nil {
		// Run main when the host starts this module (entry.go).
		statements := tx.Factory().NewNodeList(append(append([]*ast.Node{}, visited.Statements()...), tx.entryStatement()))
		statements.Loc = visited.AsSourceFile().Statements.Loc
		visited = tx.Factory().UpdateSourceFile(visited.AsSourceFile(), statements, visited.AsSourceFile().EndOfFileToken)
	}
	imports := preludeImports(&tx.Factory().NodeFactory, tx.plan.prelude)
	if tx.effect != nil {
		imports = append(imports, tx.namespaceImport(tx.effect, "effect/Effect"))
	}
	if tx.runtime != nil {
		imports = append(imports, tx.namespaceImport(tx.runtime, "effeck-runtime"))
	}
	for _, module := range tx.moduleOrder {
		imports = append(imports, tx.namespaceImport(tx.modules[module], module))
	}
	imports = append(imports, tx.namedImportDeclarations()...)
	if len(imports) == 0 {
		return visited
	}
	// The list keeps the source range, from whose end the printer emits the
	// comments after the last statement.
	statements := tx.Factory().NewNodeList(append(imports, visited.Statements()...))
	statements.Loc = visited.AsSourceFile().Statements.Loc
	return tx.Factory().UpdateSourceFile(visited.AsSourceFile(), statements, visited.AsSourceFile().EndOfFileToken)
}

// namespaceImport returns `import * as name from "module"`.
func (tx *lowering) namespaceImport(name *ast.IdentifierNode, module string) *ast.Node {
	f := tx.Factory()
	declaration := f.NewImportDeclaration(
		nil, /*modifiers*/
		f.NewImportClause(ast.KindUnknown, nil /*name*/, f.NewNamespaceImport(name)),
		f.NewStringLiteral(module, ast.TokenFlagsNone),
		nil, /*attributes*/
	)
	ast.SetParentInChildren(declaration)
	return declaration
}

// effectName returns the local name bound to effect's Effect module.
func (tx *lowering) effectName() *ast.IdentifierNode {
	if tx.effect == nil {
		// "Effect" when the file does not use that name, else "Effect_1", ...
		tx.effect = tx.Factory().NewUniqueNameEx("Effect", printer.AutoGenerateOptions{Flags: printer.GeneratedIdentifierFlagsOptimistic | printer.GeneratedIdentifierFlagsFileLevel})
	}
	return tx.effect
}

// visitThrowStatement turns `throw e` into `return yield* Effect.fail(e)`
// (D46): the thrown value is an expected failure, and the return keeps
// TypeScript's narrowing after the throw in the generated code.
func (tx *lowering) visitThrowStatement(node *ast.Node) *ast.Node {
	f := tx.Factory()
	fail := f.NewCallExpression(
		f.NewPropertyAccessExpression(tx.effectName(), nil /*questionDotToken*/, f.NewIdentifier("fail"), ast.NodeFlagsNone),
		nil, /*questionDotToken*/
		nil, /*typeArguments*/
		f.NewNodeList([]*ast.Node{tx.Visitor().VisitNode(node.Expression())}),
		ast.NodeFlagsNone,
	)
	statement := f.NewReturnStatement(f.NewYieldExpression(f.NewToken(ast.KindAsteriskToken), fail))
	tx.EmitContext().SetOriginal(statement, node)
	statement.Loc = node.Loc
	return statement
}

// runtimeMember returns `runtime.name`, where runtime is the local name
// bound to effeck-runtime.
func (tx *lowering) runtimeMember(name string) *ast.Node {
	if tx.runtime == nil {
		tx.runtime = tx.Factory().NewUniqueNameEx("runtime", printer.AutoGenerateOptions{Flags: printer.GeneratedIdentifierFlagsOptimistic | printer.GeneratedIdentifierFlagsFileLevel})
	}
	return tx.Factory().NewPropertyAccessExpression(tx.runtime, nil /*questionDotToken*/, tx.Factory().NewIdentifier(name), ast.NodeFlagsNone)
}

// convert lowers a function value that its position converts: a program
// becomes Effect.suspend(value), and an array or record of programs becomes
// $programs(value). An expression passed by name becomes a program that runs
// it (byNameProgram, which visit calls first).
func (tx *lowering) convert(node *ast.Node, conversion conversion) *ast.Node {
	f := tx.Factory()
	value := tx.visitNode(node)
	var callee *ast.Node
	switch conversion {
	case convertProgram:
		callee = f.NewPropertyAccessExpression(tx.effectName(), nil /*questionDotToken*/, f.NewIdentifier("suspend"), ast.NodeFlagsNone)
	case convertPrograms:
		callee = tx.runtimeMember("$programs")
	default:
		return value
	}
	converted := f.NewCallExpression(callee, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{value}), ast.NodeFlagsNone)
	tx.EmitContext().SetOriginal(converted, node)
	converted.Loc = node.Loc
	return converted
}

// byNameProgram lowers node, an expression passed by name (D147), to
// `Effect.gen(function* () { return node })`, with node's calls that run
// lowered to yield* in that generator, or to
// `Effect.gen({ self: this }, function* () { … })` when node refers to this.
// fork(fetchReport()) becomes fork(Effect.gen(function* () { return yield*
// fetchReport(); })): the program runs fetchReport each time it runs.
func (tx *lowering) byNameProgram(node *ast.Node) *ast.Node {
	f := tx.Factory()
	ec := tx.EmitContext()
	saved, savedFrame, savedPlain := tx.inEffect, tx.frame, tx.plain
	tx.inEffect, tx.plain = true, false
	tx.frame = &frame{}
	subject := byNameSubject(node) // a topic passes its pipe's left side (pipes.go)
	var value *ast.Node
	if ref := tx.plan.directRefs[subject]; ref != nil {
		value = tx.visitDirectReference(subject, ref)
	} else {
		value = tx.visitNode(subject)
	}
	tx.inEffect, tx.frame, tx.plain = saved, savedFrame, savedPlain
	body := f.NewBlock(f.NewNodeList([]*ast.Node{f.NewReturnStatement(value)}), false /*multiLine*/)
	generator := f.NewFunctionExpression(nil /*modifiers*/, f.NewToken(ast.KindAsteriskToken), nil /*name*/, nil /*typeParameters*/, f.NewNodeList(nil), nil /*type*/, nil /*fullSignature*/, body)
	ec.SetOriginal(generator, node)
	arguments := []*ast.Node{generator}
	if usesThis(subject) {
		self := f.NewObjectLiteralExpression(f.NewNodeList([]*ast.Node{
			f.NewPropertyAssignment(nil /*modifiers*/, f.NewIdentifier("self"), nil /*postfixToken*/, nil /*typeNode*/, f.NewThisExpression()),
		}), false /*multiLine*/)
		arguments = []*ast.Node{self, generator}
	}
	callee := f.NewPropertyAccessExpression(tx.effectName(), nil /*questionDotToken*/, f.NewIdentifier("gen"), ast.NodeFlagsNone)
	lowered := f.NewCallExpression(callee, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList(arguments), ast.NodeFlagsNone)
	ec.SetOriginal(lowered, node)
	lowered.Loc = node.Loc
	return lowered
}

// runtimeCall makes `runtime.name(arguments...)`.
func (tx *lowering) runtimeCall(name string, arguments ...*ast.Node) *ast.Node {
	return tx.Factory().NewCallExpression(tx.runtimeMember(name), nil /*questionDotToken*/, nil /*typeArguments*/, tx.Factory().NewNodeList(arguments), ast.NodeFlagsNone)
}

// visitRunningCall turns `f(x)` into `yield* f(x)`.
func (tx *lowering) visitRunningCall(node *ast.Node) *ast.Node {
	call := tx.Visitor().VisitEachChild(node)
	run := tx.Factory().NewYieldExpression(tx.Factory().NewToken(ast.KindAsteriskToken), call)
	tx.EmitContext().SetOriginal(run, node)
	return run
}

// lowerFunction turns a function expression or arrow function into
// `Effect.fnUntraced(function* (params) { body })`.
func (tx *lowering) lowerFunction(node *ast.Node) *ast.Node {
	lowered := tx.wrapFunction(node, tx.generatorOf(node))
	if keepsThis(node) {
		lowered = tx.Factory().NewFunctionBindCall(lowered, tx.Factory().NewThisExpression(), nil /*argumentsList*/)
	}
	return lowered
}

// lowerProgram turns a lambda with no parameters that is a program into
// `Effect.gen(function* () { body })`, an Effect that runs the body each time
// it runs, or `Effect.gen({ self: this }, function* () { body })` when the
// lambda keeps its enclosing function's this.
func (tx *lowering) lowerProgram(node *ast.Node) *ast.Node {
	generator := tx.generatorOf(node)
	if !keepsThis(node) {
		return tx.wrapGenerator("gen", node, generator)
	}
	f := tx.Factory()
	self := f.NewObjectLiteralExpression(f.NewNodeList([]*ast.Node{
		f.NewPropertyAssignment(nil /*modifiers*/, f.NewIdentifier("self"), nil /*postfixToken*/, nil /*typeNode*/, f.NewThisExpression()),
	}), false /*multiLine*/)
	callee := f.NewPropertyAccessExpression(tx.effectName(), nil /*questionDotToken*/, f.NewIdentifier("gen"), ast.NodeFlagsNone)
	lowered := f.NewCallExpression(callee, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{self, generator}), ast.NodeFlagsNone)
	tx.EmitContext().SetOriginal(lowered, node)
	lowered.Loc = node.Loc
	return lowered
}

// keepsThis reports whether node is an arrow function that refers to this.
// An arrow function sees its enclosing function's this, but the generator
// function that replaces it has its own, so the lowering passes the arrow's
// this on.
func keepsThis(node *ast.Node) bool {
	return node.Kind == ast.KindArrowFunction && usesThis(node.Body())
}

// generatorOf returns `function* (params) { body }` for node, with node's
// calls that run lowered to yield*.
func (tx *lowering) generatorOf(node *ast.Node) *ast.Node {
	ec := tx.EmitContext()

	saved, savedFrame, savedPlain := tx.inEffect, tx.frame, tx.plain
	tx.inEffect, tx.plain = false, false // a generator's parameter list cannot yield
	parameters := ec.VisitParameters(node.ParameterList(), tx.Visitor())
	tx.inEffect = true
	tx.frame = &frame{name: sourceName(node), clause: tx.plan.resumesInTail(node)}
	body := ec.VisitFunctionBody(node.Body(), tx.Visitor())
	tx.inEffect, tx.frame, tx.plain = saved, savedFrame, savedPlain
	if !ast.IsBlock(body) {
		body = ec.ConvertToFunctionBlock(body, false /*multiLine*/)
		// The printer emits the comments before the closing brace from the end
		// of the statement list, which would be the expression's source end:
		// the comment after the arrow would also appear inside the block.
		body.AsBlock().Statements.Loc = core.UndefinedTextRange()
	}

	generator := tx.generatorExpression(node, sourceName(node), parameters, body)
	ec.SetOriginal(generator, node)
	return generator
}

// wrapGenerator returns `Effect.<method>(generator)` in place of node.
func (tx *lowering) wrapGenerator(method string, node *ast.Node, generator *ast.Node) *ast.Node {
	f := tx.Factory()
	callee := f.NewPropertyAccessExpression(tx.effectName(), nil /*questionDotToken*/, f.NewIdentifier(method), ast.NodeFlagsNone)
	lowered := f.NewCallExpression(callee, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{generator}), ast.NodeFlagsNone)
	tx.EmitContext().SetOriginal(lowered, node)
	lowered.Loc = node.Loc
	return lowered
}

// functionName returns the name of fn, a function the lowering turns into an
// Effect: its own name, or the name of the variable it initializes, or "".
func functionName(fn *ast.Node) string {
	if name := fn.Name(); name != nil && ast.IsIdentifier(name) {
		return name.Text()
	}
	if parent := fn.Parent; parent != nil && parent.Kind == ast.KindVariableDeclaration && ast.IsIdentifier(parent.Name()) {
		return parent.Name().Text()
	}
	return ""
}

// visitEffectFunctionDeclaration turns `function f() {}` into
// `const f = Effect.fnUntraced(function* () {})`, keeping `export` and
// `export default`. The const is not hoisted like the function was.
func (tx *lowering) visitEffectFunctionDeclaration(node *ast.Node) *ast.Node {
	f := tx.Factory()
	ec := tx.EmitContext()
	lowered := tx.lowerFunction(node)
	exported := ast.HasSyntacticModifier(node, ast.ModifierFlagsExport)
	isDefault := ast.HasSyntacticModifier(node, ast.ModifierFlagsDefault)

	name := node.Name()
	if name == nil {
		// export default function () {}
		statement := f.NewExportAssignment(nil /*modifiers*/, false /*isExportEquals*/, nil /*type*/, lowered)
		ec.SetOriginal(statement, node)
		statement.Loc = node.Loc
		return statement
	}

	var modifiers *ast.ModifierList
	if exported && !isDefault {
		modifiers = f.NewModifierList([]*ast.Node{f.NewModifier(ast.KindExportKeyword)})
	}
	declaration := f.NewVariableDeclaration(name, nil /*exclamationToken*/, nil /*type*/, lowered)
	statement := f.NewVariableStatement(modifiers, f.NewVariableDeclarationList(f.NewNodeList([]*ast.Node{declaration}), ast.NodeFlagsConst))
	ec.SetOriginal(statement, node)
	statement.Loc = node.Loc
	if exported && isDefault {
		exportDefault := f.NewExportAssignment(nil /*modifiers*/, false /*isExportEquals*/, nil /*type*/, f.NewIdentifier(name.Text()))
		return f.NewSyntaxList([]*ast.Node{statement, exportDefault})
	}
	return statement
}
