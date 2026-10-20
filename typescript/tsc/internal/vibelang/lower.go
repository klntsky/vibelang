package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/printer"
	"github.com/microsoft/TypeScript/tsc/internal/transformers"
)

// lowering rewrites one .vibe file so it runs on Effect 4. It runs before the
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
//	export function greet(arg) {
//	  return Effect.fnUntraced(function* greet(name: string) {
//	    yield* Console.log(`hello, ${name}`)
//	    return name.length
//	  }).apply(this, arguments)
//	}
type lowering struct {
	transformers.Transformer
	plan     *plan
	effect   *ast.IdentifierNode // local name for effect's Effect module, imported on first use
	runtime  *ast.IdentifierNode // local name for vibelang-runtime, imported on first use
	inEffect bool                // visiting code that runs inside a lowered generator
	frame    *frame              // the generator being emitted while inEffect
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
	if !isVibeLangFile(file) {
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
	visited := tx.visitNode(node)
	if tx.inEffect && tx.plan.returns[node] {
		result := tx.Factory().NewYieldExpression(tx.Factory().NewToken(ast.KindAsteriskToken), visited)
		tx.EmitContext().SetOriginal(result, node)
		result.Loc = node.Loc
		return result
	}
	return visited
}

func (tx *lowering) visitNode(node *ast.Node) *ast.Node {
	switch {
	case node.Kind == ast.KindShorthandPropertyAssignment && tx.plan.conversionOf(node.Name()) != convertNone:
		property := tx.Factory().NewPropertyAssignment(nil, node.Name(), nil, nil, tx.visit(node.Name()))
		tx.EmitContext().SetOriginal(property, node)
		property.Loc = node.Loc
		return property
	case node.Kind == ast.KindSourceFile:
		return tx.visitSourceFile(node.AsSourceFile())
	case node.Kind == ast.KindImportDeclaration && len(tx.plan.directImports[node]) != 0:
		return tx.visitImportWithDirectVariants(node)
	case node.Kind == ast.KindExportDeclaration && len(tx.plan.directExports[node]) != 0:
		return tx.visitExportWithDirectVariants(node)
	case node.Kind == ast.KindImportDeclaration || node.Kind == ast.KindExportDeclaration:
		return tx.visitImportOrExport(node)
	case isPipe(node):
		return tx.visitPipe(node)
	case isTopic(node):
		return tx.visitTopic(node)
	case node.Kind == ast.KindCallExpression && tx.plan.blockForms[node] != blockAppended:
		return tx.visitBlockParamCall(node, tx.plan.blockForms[node]) // f(args) { block } for an Effect dual (blockparams.go)
	case node.Kind == ast.KindCallExpression && tx.plan.hatches[node] != (hatchForm{}):
		return tx.visitHatchCall(node)
	case node.Kind == ast.KindCallExpression && tx.plan.methods[node].module != "":
		return tx.visitModuleMethodCall(node)
	// A provide block may also own using resources or forked fibers. Keep
	// provision outside its Scope region; regionStatements lowers that region.
	case tx.inEffect && isProvideBlock(node):
		return tx.visitProvideBlock(node)
	case tx.inEffect && tx.plan.scoped(node):
		return tx.visitScoped(node)
	case node.Kind == ast.KindCallExpression && tx.inEffect && tx.plan.values[node]:
		return tx.visitEffectValueCall(node)
	case node.Kind == ast.KindCallExpression && tx.inEffect && tx.frame.handleClause != nil && tx.isClauseResume(node):
		return tx.visitClauseResume(node)
	case isInvocation(node) && tx.inEffect && tx.plan.runsCall(node):
		return tx.visitRunningCall(node)
	case node.Kind == ast.KindCallExpression && tx.inEffect && tx.plan.trusted[node] != nil:
		return tx.visitTrustedCall(node)
	case node.Kind == ast.KindPropertyAccessExpression && tx.inEffect && tx.plan.looksUp(node):
		return tx.visitServiceLookup(node)
	case node.Kind == ast.KindClassDeclaration && tx.plan.keys[node] != "":
		return tx.visitServiceClass(node)
	case tx.inEffect && isHandleStatement(node):
		return tx.visitHandleStatement(node) // try { ... } handle Control.op(...) { ... } (handles.go)
	case tx.inEffect && node.Kind == ast.KindTryStatement:
		return tx.visitTryStatement(node)
	case tx.inEffect && maskWord(node) != "":
		return tx.visitMaskBlock(node, maskWord(node)) // uninterruptible { ... } (primitives.go)
	case maskMarkerWord(node) != "" || exitBindingOf(node) != nil:
		return nil // the marker of an interruption block, or finally (exit)'s binding (primitives.go)
	case isFailureTest(node):
		return tx.visitFailureTest(node)
	case isProvideLayer(node):
		return nil // visitProvideBlock lowers the layer expression itself
	case isClauseBinding(node):
		return nil // clauseFunction binds a handle clause's parameters itself
	case tx.inEffect && tx.frame.handleClause != nil && node.Kind == ast.KindReturnStatement && tx.isClauseResume(node.Expression()):
		return tx.visitClauseResumeReturn(node)
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
		saved := tx.inEffect
		tx.inEffect = false
		defer func() { tx.inEffect = saved }()
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
		imports = append(imports, tx.namespaceImport(tx.runtime, "vibelang-runtime"))
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
	callee := f.NewPropertyAccessExpression(tx.effectName(), nil /*questionDotToken*/, f.NewIdentifier("fail"), ast.NodeFlagsNone)
	if tx.plan.causeThrows[node] {
		callee = tx.runtimeMember("$failCause") // throw cause (D177, primitives.go)
	}
	fail := f.NewCallExpression(
		callee,
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
// bound to vibelang-runtime.
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
	case convertSyncProgram:
		callee = f.NewPropertyAccessExpression(tx.effectName(), nil, f.NewIdentifier("sync"), ast.NodeFlagsNone)
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

// byNameProgram lowers a by-name expression (D147), evaluating it on every
// run. A tail program is returned by suspend, never yielded by a wrapper:
// Effect 4.0.5 internal/effect.ts:976-982 returns the thunk's Effect without
// pushing a continuation. In contrast, gen (1222-1244) installs an Iterator
// (1417-1428), whose post-program step can be interrupted before restore
// reinstates the mask (4582-4585, 4617-4637).
// If operands run Effects, a generator computes the tail program first and
// flatten (1867-1869, flatMap with identity) runs it after that generator
// has finished. Expressions that compute a value after their last call
// retain their generator.
func (tx *lowering) byNameProgram(node *ast.Node) *ast.Node {
	f := tx.Factory()
	ec := tx.EmitContext()
	saved, savedFrame := tx.inEffect, tx.frame
	tx.inEffect = true
	tx.frame = &frame{}
	subject := byNameSubject(node) // a topic passes its pipe's left side (pipes.go)
	value, tailProgram := tx.byNameResult(subject)
	tx.inEffect, tx.frame = saved, savedFrame
	if tailProgram && !containsYield(value) {
		lowered := f.NewCallExpression(f.NewPropertyAccessExpression(tx.effectName(), nil, f.NewIdentifier("suspend"), ast.NodeFlagsNone), nil, nil, f.NewNodeList([]*ast.Node{tx.arrow(nil, value)}), ast.NodeFlagsNone)
		ec.SetOriginal(lowered, node)
		lowered.Loc = node.Loc
		return lowered
	}
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
	if tailProgram {
		lowered = f.NewCallExpression(f.NewPropertyAccessExpression(tx.effectName(), nil, f.NewIdentifier("flatten"), ast.NodeFlagsNone), nil, nil, f.NewNodeList([]*ast.Node{lowered}), ast.NodeFlagsNone)
	}
	ec.SetOriginal(lowered, node)
	lowered.Loc = node.Loc
	return lowered
}

// byNameResult computes the program in each result position without running
// it. Only operands and conditions keep yield*: they finish before flatten
// enters the chosen program, with no generator continuation after it.
func (tx *lowering) byNameResult(node *ast.Node) (*ast.Node, bool) {
	f := tx.Factory()
	node = ast.SkipOuterExpressions(node, ast.OEKParentheses|ast.OEKTypeAssertions|ast.OEKNonNullAssertions|ast.OEKSatisfies)
	switch node.Kind {
	case ast.KindConditionalExpression:
		conditional := node.AsConditionalExpression()
		condition := tx.Visitor().VisitNode(conditional.Condition)
		left, leftProgram := tx.byNameResult(conditional.WhenTrue)
		right, rightProgram := tx.byNameResult(conditional.WhenFalse)
		if leftProgram != rightProgram {
			if !leftProgram {
				left = tx.succeedByName(left)
			} else {
				right = tx.succeedByName(right)
			}
		}
		return f.UpdateConditionalExpression(conditional, condition, conditional.QuestionToken, left, conditional.ColonToken, right), leftProgram || rightProgram
	case ast.KindBinaryExpression:
		binary := node.AsBinaryExpression()
		switch binary.OperatorToken.Kind {
		case ast.KindCommaToken:
			left := tx.Visitor().VisitNode(binary.Left)
			right, program := tx.byNameResult(binary.Right)
			return f.UpdateBinaryExpression(binary, binary.Modifiers(), left, binary.Type, binary.OperatorToken, right), program
		case ast.KindBarBarToken, ast.KindQuestionQuestionToken:
			left, leftProgram := tx.byNameResult(binary.Left)
			if !leftProgram {
				// A scalar left operand tests its value, including the success
				// of any calls inside it, before selecting the right side.
				return tx.visitNode(node), false
			}
			right, rightProgram := tx.byNameResult(binary.Right)
			if !rightProgram {
				right = tx.succeedByName(right)
			}
			return f.UpdateBinaryExpression(binary, binary.Modifiers(), left, binary.Type, binary.OperatorToken, right), true
		}
	}
	var value *ast.Node
	if ref := tx.plan.directRefs[node]; ref != nil {
		value = tx.visitDirectReference(node, ref)
	} else {
		value = tx.visitNode(node)
	}
	tail := ast.SkipOuterExpressions(value, ast.OEKParentheses|ast.OEKTypeAssertions|ast.OEKNonNullAssertions|ast.OEKSatisfies)
	if tail.Kind == ast.KindYieldExpression && tail.AsYieldExpression().AsteriskToken != nil {
		return tail.Expression(), true
	}
	return value, tx.plan.heldPrograms[node]
}

// succeedByName makes `Effect.succeed(value)` with value's source range: the
// printer emits the comments after a list's last element unless it ends
// where its parent does, which would move the next statement's comment into
// the call.
func (tx *lowering) succeedByName(value *ast.Node) *ast.Node {
	f := tx.Factory()
	succeed := f.NewCallExpression(f.NewPropertyAccessExpression(tx.effectName(), nil, f.NewIdentifier("succeed"), ast.NodeFlagsNone), nil, nil, f.NewNodeList([]*ast.Node{value}), ast.NodeFlagsNone)
	succeed.Loc = value.Loc
	return succeed
}

// runtimeCall makes `runtime.name(arguments...)`.
func (tx *lowering) runtimeCall(name string, arguments ...*ast.Node) *ast.Node {
	return tx.Factory().NewCallExpression(tx.runtimeMember(name), nil /*questionDotToken*/, nil /*typeArguments*/, tx.Factory().NewNodeList(arguments), ast.NodeFlagsNone)
}

// visitRunningCall turns `f(x)` into `yield* f(x)`.
func (tx *lowering) visitRunningCall(node *ast.Node) *ast.Node {
	call := tx.Visitor().VisitEachChild(node)
	if ast.IsOptionalChain(node) {
		// Keep the native chain intact: it preserves the receiver and skips
		// arguments when absent. Run its present Effect exactly once, and lift
		// the absent branch to a successful undefined for yield*.
		f := tx.Factory()
		absent := f.NewCallExpression(f.NewPropertyAccessExpression(tx.effectName(), nil, f.NewIdentifier("succeed"), ast.NodeFlagsNone), nil, nil, f.NewNodeList([]*ast.Node{f.NewVoidExpression(f.NewNumericLiteral("0", ast.TokenFlagsNone))}), ast.NodeFlagsNone)
		call = f.NewBinaryExpression(nil, call, nil, f.NewToken(ast.KindQuestionQuestionToken), absent)
	}
	run := tx.Factory().NewYieldExpression(tx.Factory().NewToken(ast.KindAsteriskToken), call)
	tx.EmitContext().SetOriginal(run, node)
	return run
}

// lowerFunction turns a function expression or arrow function into
// `Effect.fnUntraced(function* (params) { body })`, or into the same kind of
// function returning its direct Effect (direct.go).
func (tx *lowering) lowerFunction(node *ast.Node) *ast.Node {
	parameters, body := tx.loweredBody(node)
	if hasSimpleParameters(node) {
		if effect := tx.directEffect(node, body); effect != nil {
			return tx.directFunction(node, parameters, effect)
		}
	}
	lowered := tx.wrapFunction(node, tx.generatorFrom(node, parameters, body))
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
	parameters, body := tx.loweredBody(node)
	// Only an arrow: Effect.gen gives a function expression's body no this or
	// arguments, which a thunk in its place would take from its scope.
	if node.Kind == ast.KindArrowFunction {
		if effect := tx.directEffect(node, body); effect != nil {
			effect.Loc = node.Loc
			return effect
		}
	}
	generator := tx.generatorFrom(node, parameters, body)
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
	parameters, body := tx.loweredBody(node)
	return tx.generatorFrom(node, parameters, body)
}

// generatorFrom returns `function* (params) { body }` for node, given its
// lowered parameters and body.
func (tx *lowering) generatorFrom(node *ast.Node, parameters *ast.NodeList, body *ast.Node) *ast.Node {
	generator := tx.generatorExpression(node, sourceName(node), parameters, body)
	tx.EmitContext().SetOriginal(generator, node)
	return generator
}

// loweredBody returns node's parameters and its body as a block, with the
// body's calls that run lowered to yield*.
func (tx *lowering) loweredBody(node *ast.Node) (*ast.NodeList, *ast.Node) {
	ec := tx.EmitContext()

	saved, savedFrame := tx.inEffect, tx.frame
	tx.inEffect = false // a generator's parameter list cannot yield
	parameters := ec.VisitParameters(node.ParameterList(), tx.Visitor())
	tx.inEffect = true
	tx.frame = &frame{name: sourceName(node), clause: tx.plan.resumesInTail(node)}
	body := ec.VisitFunctionBody(node.Body(), tx.Visitor())
	tx.inEffect, tx.frame = saved, savedFrame
	if !ast.IsBlock(body) {
		body = ec.ConvertToFunctionBlock(body, false /*multiLine*/)
		// The printer emits the comments before the closing brace from the end
		// of the statement list, which would be the expression's source end:
		// the comment after the arrow would also appear inside the block.
		body.AsBlock().Statements.Loc = core.UndefinedTextRange()
	}
	return parameters, body
}

// directFunction returns node, an arrow function or function expression,
// with parameters and a body that returns effect.
func (tx *lowering) directFunction(node *ast.Node, parameters *ast.NodeList, effect *ast.Node) *ast.Node {
	f := tx.Factory()
	var lowered *ast.Node
	if node.Kind == ast.KindArrowFunction {
		lowered = f.NewArrowFunction(nil /*modifiers*/, nil /*typeParameters*/, parameters, nil /*typeNode*/, nil /*fullSignature*/, f.NewToken(ast.KindEqualsGreaterThanToken), effect)
	} else {
		body := f.NewBlock(f.NewNodeList([]*ast.Node{f.NewReturnStatement(effect)}), false /*multiLine*/)
		lowered = f.NewFunctionExpression(nil /*modifiers*/, nil /*asteriskToken*/, node.Name(), nil /*typeParameters*/, parameters, nil /*typeNode*/, nil /*fullSignature*/, body)
	}
	tx.EmitContext().SetOriginal(lowered, node)
	lowered.Loc = node.Loc
	return lowered
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

// visitEffectFunctionDeclaration keeps a hoisted JavaScript declaration in
// every lexical block (including declarations after a return). Only its body
// becomes an Effect factory; moving a const cannot cover switch clauses or
// cyclic module imports. Keep the existing generator and tracing wrapper so
// defaults, destructuring, this, arguments, and recursive references still run
// inside the Effect, rather than eagerly in the forwarding declaration. A
// body with a direct Effect (direct.go) is the declaration's own body.
func (tx *lowering) visitEffectFunctionDeclaration(node *ast.Node) *ast.Node {
	f := tx.Factory()
	loweredParameters, loweredBody := tx.loweredBody(node)
	if hasSimpleParameters(node) {
		if effect := tx.directEffect(node, loweredBody); effect != nil {
			body := f.NewBlock(f.NewNodeList([]*ast.Node{f.NewReturnStatement(effect)}), true /*multiLine*/)
			return tx.effectFunctionDeclaration(node, loweredParameters, body)
		}
	}
	lowered := tx.wrapFunction(node, tx.generatorFrom(node, loweredParameters, loweredBody))

	// Forward all arguments without evaluating the source parameters twice.
	// Placeholder parameters preserve fnUntraced's observable function length.
	var parameters []*ast.Node
	for _, parameter := range node.Parameters() {
		if ast.IsIdentifier(parameter.Name()) && parameter.Name().Text() == "this" {
			continue
		}
		p := parameter.AsParameterDeclaration()
		if p.Initializer != nil || p.DotDotDotToken != nil {
			break
		}
		parameters = append(parameters, f.NewParameterDeclaration(nil, nil, f.NewUniqueName("arg"), nil, nil, nil))
	}
	apply := f.NewPropertyAccessExpression(lowered, nil, f.NewIdentifier("apply"), ast.NodeFlagsNone)
	call := f.NewCallExpression(apply, nil, nil, f.NewNodeList([]*ast.Node{f.NewThisExpression(), f.NewIdentifier("arguments")}), ast.NodeFlagsNone)
	// A source directive such as "use strict" also governs the forwarding
	// declaration's receiver coercion when the file emits as a script.
	var statements []*ast.Node
	for _, statement := range node.Body().Statements() {
		if !ast.IsPrologueDirective(statement) {
			break
		}
		statements = append(statements, f.NewExpressionStatement(f.NewStringLiteralFromNode(statement.Expression())))
	}
	statements = append(statements, f.NewReturnStatement(call))
	body := f.NewBlock(f.NewNodeList(statements), true /*multiLine*/)
	return tx.effectFunctionDeclaration(node, f.NewNodeList(parameters), body)
}

// effectFunctionDeclaration returns the declaration that replaces node, an
// Effect function declaration, with parameters and body.
func (tx *lowering) effectFunctionDeclaration(node *ast.Node, parameters *ast.NodeList, body *ast.Node) *ast.Node {
	f := tx.Factory()
	// async is implemented by the Effect generator, not by this wrapper.
	var modifiers []*ast.Node
	if ast.HasSyntacticModifier(node, ast.ModifierFlagsExport) {
		modifiers = append(modifiers, f.NewModifier(ast.KindExportKeyword))
	}
	if ast.HasSyntacticModifier(node, ast.ModifierFlagsDefault) {
		modifiers = append(modifiers, f.NewModifier(ast.KindDefaultKeyword))
	}
	statement := f.NewFunctionDeclaration(f.NewModifierList(modifiers), nil, node.Name(), nil, parameters, nil, nil, body)
	tx.EmitContext().SetOriginal(statement, node)
	statement.Loc = node.Loc
	return statement
}
