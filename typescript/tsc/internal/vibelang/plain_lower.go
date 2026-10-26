package vibelang

import (
	"strconv"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/printer"
)

// visitPlainFunctionDeclaration turns a function that compiles to plain
// JavaScript into its $direct variant and its Effect form:
//
//	function double(n) { return n * 2 }
//
// becomes
//
//	function double$direct(n) { return n * 2 }
//	function double(...args) { return Effect.sync(() => double$direct(...args)) }
//
// The variant is exported when the function is, so other .vibe modules can
// import it. The Effect form keeps the declaration's modifiers and stays a
// hoisted function declaration. The variant keeps no link to the source
// declaration: TypeScript's CommonJS transform keys an exported function by
// its original node, so a link would merge the variant's export into the
// Effect form's and leave exports.double$direct unset.
func (tx *lowering) visitPlainFunctionDeclaration(node *ast.Node) *ast.Node {
	f := tx.Factory()
	ec := tx.EmitContext()
	name := node.Name().Text()

	saved, savedPlain := tx.inEffect, tx.plain
	tx.inEffect, tx.plain = false, true // its catch clauses rethrow defects (D54)
	parameters := ec.VisitParameters(node.ParameterList(), tx.Visitor())
	body := ec.VisitFunctionBody(node.Body(), tx.Visitor())
	tx.inEffect, tx.plain = saved, savedPlain

	var modifiers *ast.ModifierList
	if ast.HasSyntacticModifier(node, ast.ModifierFlagsExport) {
		modifiers = f.NewModifierList([]*ast.Node{f.NewModifier(ast.KindExportKeyword)})
	}
	directName := f.NewIdentifier(name + "$direct")
	direct := f.NewFunctionDeclaration(modifiers, nil /*asteriskToken*/, directName, nil /*typeParameters*/, parameters, nil /*type*/, nil /*fullSignature*/, body)
	// .vibe callers call the variant, so it keeps the source function's
	// position, and the Effect wrapper maps to none (generated.go).
	ec.SetSourceMapRange(direct, node.Loc)
	ec.SetSourceMapRange(directName, node.Name().Loc)

	// function double(...args) { return Effect.sync(() => double$direct(...args)) }
	args := f.NewIdentifier("args")
	rest := f.NewParameterDeclaration(nil /*modifiers*/, f.NewToken(ast.KindDotDotDotToken), args, nil /*questionToken*/, nil /*type*/, nil /*initializer*/)
	call := tx.generated(f.NewCallExpression(f.NewIdentifier(name+"$direct"), nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{f.NewSpreadElement(args)}), ast.NodeFlagsNone))
	thunk := tx.generated(f.NewArrowFunction(nil /*modifiers*/, nil /*typeParameters*/, f.NewNodeList(nil), nil /*type*/, nil /*fullSignature*/, f.NewToken(ast.KindEqualsGreaterThanToken), call))
	sync := f.NewCallExpression(f.NewPropertyAccessExpression(tx.effectName(), nil /*questionDotToken*/, f.NewIdentifier("sync"), ast.NodeFlagsNone), nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{thunk}), ast.NodeFlagsNone)
	wrapperBody := f.NewBlock(f.NewNodeList([]*ast.Node{tx.generated(f.NewReturnStatement(sync))}), false /*multiLine*/)
	wrapper := f.UpdateFunctionDeclaration(node.AsFunctionDeclaration(), node.Modifiers(), nil /*asteriskToken*/, node.Name(), nil /*typeParameters*/, f.NewNodeList([]*ast.Node{rest}), nil /*type*/, nil /*fullSignature*/, wrapperBody)
	ec.SetOriginal(wrapper, node)
	tx.generated(wrapper)
	statements := []*ast.Node{direct, wrapper}
	if tx.plan.budgetDirects[node] {
		savedBudget, savedWeight, savedEscapes := tx.eagerBudget, tx.eagerWeight, tx.eagerEscapes
		savedEffect, savedPlain := tx.inEffect, tx.plain
		tx.eagerBudget, tx.eagerWeight, tx.eagerEscapes = f.NewUniqueName("depth"), plainFrameWeight(node), false
		tx.inEffect, tx.plain = false, true
		ec.VisitParameters(node.ParameterList(), tx.Visitor())
		body := ec.VisitFunctionBody(node.Body(), tx.Visitor())
		if tx.eagerEscapes {
			body = tx.restorePlainBudget(body)
		}
		params := f.NewNodeList(append([]*ast.Node{f.NewParameterDeclaration(nil, nil, tx.eagerBudget, nil, nil, nil)}, parameters.Nodes...))
		recur := f.NewFunctionDeclaration(nil, nil, f.NewIdentifier(name+"$recur"), nil, params, nil, nil, body)
		tx.eagerBudget, tx.eagerWeight, tx.eagerEscapes = savedBudget, savedWeight, savedEscapes
		tx.inEffect, tx.plain = savedEffect, savedPlain
		register := f.NewPropertyAccessExpression(tx.runtimeMember("$eager"), nil, f.NewIdentifier("register"), ast.NodeFlagsNone)
		statements = append(statements, recur, f.NewExpressionStatement(f.NewCallExpression(register, nil, nil,
			f.NewNodeList([]*ast.Node{directName, directName, f.NewIdentifier(name + "$recur")}), ast.NodeFlagsNone)))
	}
	return f.NewSyntaxList(append(statements, tx.directMarker(node)...))
}

// visitCyclicPlainFunctionDeclaration preserves the JS body for shallow
// recursion, sharing the eager budget with the projector's $fast entry.
// Once that budget is exhausted the Effect body continues on heap frames.
// A synchronous callback cannot yield; its original JS body is retained as
// $stack, so crossing that boundary does not nest synchronous Effect runners.
func (tx *lowering) visitCyclicPlainFunctionDeclaration(node *ast.Node) *ast.Node {
	f := tx.Factory()
	name := node.Name().Text()
	savedCallbacks := tx.rawCallbacks
	tx.rawCallbacks = true
	effect := tx.visitEffectFunctionDeclaration(node)
	cached := false
	if statements := effect.Body().Statements(); len(statements) > 0 {
		ret := statements[len(statements)-1]
		if ast.IsReturnStatement(ret) && ast.IsCallExpression(ret.Expression()) {
			invoke := ret.Expression()
			apply := invoke.Expression()
			if ast.IsPropertyAccessExpression(apply) && apply.Name().Text() == "apply" {
				cached = true
				heapMethod := func(method string, args ...*ast.Node) *ast.Node {
					return f.NewCallExpression(f.NewPropertyAccessExpression(tx.runtimeMember("$eager"), nil, f.NewIdentifier(method), ast.NodeFlagsNone), nil, nil, f.NewNodeList(args), ast.NodeFlagsNone)
				}
				cache := heapMethod("getHeap", f.NewIdentifier(name))
				install := heapMethod("setHeap", f.NewIdentifier(name), apply.Expression())
				factory := f.NewParenthesizedExpression(f.NewBinaryExpression(nil, cache, nil, f.NewToken(ast.KindQuestionQuestionToken), install))
				call := f.NewCallExpression(f.NewPropertyAccessExpression(factory, nil, apply.Name(), ast.NodeFlagsNone), nil, nil, invoke.ArgumentList(), ast.NodeFlagsNone)
				body := f.NewBlock(f.NewNodeList(append(append([]*ast.Node{}, statements[:len(statements)-1]...), f.NewReturnStatement(call))), true)
				effect = f.UpdateFunctionDeclaration(effect.AsFunctionDeclaration(), effect.Modifiers(), nil, effect.Name(), nil, effect.ParameterList(), nil, nil, body)
			}
		}
	}

	if !cached {
		// A direct-Effect optimization binds the real source parameters
		// in its declaration. Retain that binding in a private factory
		// when restoring the public plain wrapper's zero arity.
		factory := f.NewFunctionExpression(nil, nil, nil, nil, effect.ParameterList(), nil, nil, effect.Body())
		tx.EmitContext().SetOriginal(factory, node)
		method := func(key string, args ...*ast.Node) *ast.Node {
			return f.NewCallExpression(f.NewPropertyAccessExpression(tx.runtimeMember("$eager"), nil, f.NewIdentifier(key), ast.NodeFlagsNone), nil, nil, f.NewNodeList(args), ast.NodeFlagsNone)
		}
		cachedFactory := f.NewParenthesizedExpression(f.NewBinaryExpression(nil, method("getHeap", f.NewIdentifier(name)), nil, f.NewToken(ast.KindQuestionQuestionToken), method("setHeap", f.NewIdentifier(name), factory)))
		invoke := f.NewCallExpression(f.NewPropertyAccessExpression(cachedFactory, nil, f.NewIdentifier("apply"), ast.NodeFlagsNone), nil, nil, f.NewNodeList([]*ast.Node{f.NewThisExpression(), f.NewIdentifier("arguments")}), ast.NodeFlagsNone)
		var statements []*ast.Node
		for _, statement := range effect.Body().Statements() {
			if !ast.IsPrologueDirective(statement) {
				break
			}
			statements = append(statements, f.NewExpressionStatement(f.NewStringLiteralFromNode(statement.Expression())))
		}
		statements = append(statements, f.NewReturnStatement(invoke))
		effect = f.UpdateFunctionDeclaration(effect.AsFunctionDeclaration(), effect.Modifiers(), nil, effect.Name(), nil, effect.ParameterList(), nil, nil, f.NewBlock(f.NewNodeList(statements), true))
	}

	if tx.frames.binders[node] {
		effect = tx.hoistFrameReferences(node, effect)
	}
	// The public Effect entry also takes the native path when no native
	// reservation is active. Calls made by the heap continuation see the
	// reservation and stay on heap frames. The thunk preserves laziness,
	// including parameter binding, this, and arguments.
	eagerState := tx.runtimeMember("$eager")
	eagerDepth := f.NewPropertyAccessExpression(eagerState, nil, f.NewIdentifier("depth"), ast.NodeFlagsNone)
	eagerLimit := f.NewPropertyAccessExpression(eagerState, nil, f.NewIdentifier("limit"), ast.NodeFlagsNone)
	eagerCall := f.NewCallExpression(f.NewPropertyAccessExpression(f.NewIdentifier(name+"$direct"), nil, f.NewIdentifier("apply"), ast.NodeFlagsNone), nil, nil,
		f.NewNodeList([]*ast.Node{f.NewThisExpression(), f.NewIdentifier("arguments")}), ast.NodeFlagsNone)
	thunk := f.NewArrowFunction(nil, nil, f.NewNodeList(nil), nil, nil, f.NewToken(ast.KindEqualsGreaterThanToken), eagerCall)
	sync := f.NewCallExpression(f.NewPropertyAccessExpression(tx.effectName(), nil, f.NewIdentifier("sync"), ast.NodeFlagsNone), nil, nil, f.NewNodeList([]*ast.Node{thunk}), ast.NodeFlagsNone)
	eagerGuard := f.NewIfStatement(f.NewBinaryExpression(nil, eagerDepth, nil, f.NewToken(ast.KindLessThanToken), eagerLimit), f.NewReturnStatement(sync), nil)
	effectStatements := effect.Body().Statements()
	prologue := 0
	for prologue < len(effectStatements) && ast.IsPrologueDirective(effectStatements[prologue]) {
		prologue++
	}
	statementsWithGuard := append([]*ast.Node{}, effectStatements[:prologue]...)
	statementsWithGuard = append(statementsWithGuard, eagerGuard)
	statementsWithGuard = append(statementsWithGuard, effectStatements[prologue:]...)
	// Plain functions previously exposed a rest-only Effect wrapper (length
	// zero). The heap generator still binds the original parameters.
	effect = f.UpdateFunctionDeclaration(effect.AsFunctionDeclaration(), effect.Modifiers(), nil, effect.Name(), nil, f.NewNodeList(nil), nil, nil,
		f.NewBlock(f.NewNodeList(statementsWithGuard), true))

	tx.rawCallbacks = savedCallbacks

	var modifiers *ast.ModifierList
	if ast.HasSyntacticModifier(node, ast.ModifierFlagsExport) {
		modifiers = f.NewModifierList([]*ast.Node{f.NewModifier(ast.KindExportKeyword)})
	}
	savedEffect, savedPlain, savedRaw := tx.inEffect, tx.plain, tx.rawPlain
	tx.inEffect, tx.plain = false, true
	parameters := tx.EmitContext().VisitParameters(node.ParameterList(), tx.Visitor())
	tx.rawPlain = true
	rawBody := tx.EmitContext().VisitFunctionBody(node.Body(), tx.Visitor())
	tx.rawPlain = false
	tx.rawCallbacks = true
	savedBudget, savedWeight, savedEscapes := tx.eagerBudget, tx.eagerWeight, tx.eagerEscapes
	tx.eagerWeight = plainFrameWeight(node)
	var budgetName *ast.Node
	if plainBudgetParameters(node) {
		budgetName = f.NewUniqueName("depth")
	}
	tx.eagerBudget, tx.eagerEscapes = budgetName, false
	tx.EmitContext().VisitParameters(node.ParameterList(), tx.Visitor())
	eagerBody := tx.EmitContext().VisitFunctionBody(node.Body(), tx.Visitor())
	if tx.eagerEscapes {
		eagerBody = tx.restorePlainBudget(eagerBody)
	}
	tx.eagerBudget, tx.eagerWeight, tx.eagerEscapes = savedBudget, savedWeight, savedEscapes
	tx.inEffect, tx.plain, tx.rawPlain, tx.rawCallbacks = savedEffect, savedPlain, savedRaw, savedCallbacks

	raw := f.NewFunctionDeclaration(nil, nil, f.NewIdentifier(name+"$stack"), nil, parameters, nil, nil, rawBody)
	call := func(callee *ast.Node, args ...*ast.Node) *ast.Node {
		return f.NewCallExpression(callee, nil, nil, f.NewNodeList(args), ast.NodeFlagsNone)
	}
	member := func(object *ast.Node, key string) *ast.Node {
		return f.NewPropertyAccessExpression(object, nil, f.NewIdentifier(key), ast.NodeFlagsNone)
	}
	// Forward simple parameters by name: keeping arguments off the hot path
	// lets the JS engine optimize ordinary recursion. Complex bindings must
	// be evaluated once, so a forwarding entry guards before entering them.
	var args []*ast.Node
	simple := plainBudgetParameters(node)
	for _, parameter := range node.Parameters() {
		if ast.IsIdentifier(parameter.Name()) && parameter.Name().Text() != "this" {
			arg := f.NewIdentifier(parameter.Name().Text())
			if parameter.AsParameterDeclaration().DotDotDotToken != nil {
				arg = f.NewSpreadElement(arg)
			}
			args = append(args, arg)
		}
	}
	var heapCall *ast.Node
	var extra []*ast.Node
	if simple {
		if usesThis(node.Body()) {
			heapCall = call(member(f.NewIdentifier(name), "call"), append([]*ast.Node{f.NewThisExpression()}, args...)...)
		} else {
			heapCall = call(f.NewIdentifier(name), args...)
		}
	} else {
		heapCall = call(member(f.NewIdentifier(name), "apply"), f.NewThisExpression(), f.NewIdentifier("arguments"))
		eager := f.NewFunctionDeclaration(nil, nil, f.NewIdentifier(name+"$eager"), nil, parameters, nil, nil, eagerBody)
		extra = append(extra, eager)
		eagerBody = f.NewBlock(f.NewNodeList([]*ast.Node{f.NewReturnStatement(call(member(f.NewIdentifier(name+"$eager"), "apply"), f.NewThisExpression(), f.NewIdentifier("arguments")))}), true)
		var forwarded []*ast.Node
		for _, parameter := range node.Parameters() {
			if ast.IsIdentifier(parameter.Name()) && parameter.Name().Text() == "this" {
				continue
			}
			if parameter.Initializer() != nil || parameter.AsParameterDeclaration().DotDotDotToken != nil {
				break
			}
			forwarded = append(forwarded, f.NewParameterDeclaration(nil, nil, f.NewUniqueName("arg"), nil, nil, nil))
		}
		parameters = f.NewNodeList(forwarded)
	}
	budget := tx.runtimeMember("$eager")
	depth := member(budget, "depth")
	// Charge large native frames more, keeping the eager limit conservative
	// without sending tiny recursive helpers through Effect at depth 100.
	weight := f.NewNumericLiteral(strconv.Itoa(plainFrameWeight(node)), ast.TokenFlagsNone)
	update := func(op ast.Kind) *ast.Node {
		return f.NewExpressionStatement(f.NewBinaryExpression(nil, depth, nil, f.NewToken(op), weight))
	}
	var body *ast.Node
	var recur *ast.Node
	if simple {
		// Cycle edges pass the remaining budget as a value. Only a heap
		// continuation reserves all of it; opaque calls see the published
		// depth, and bodies restore publication on every exit.
		guard := f.NewIfStatement(f.NewBinaryExpression(nil, budgetName, nil, f.NewToken(ast.KindLessThanEqualsToken), f.NewNumericLiteral("0", ast.TokenFlagsNone)),
			f.NewReturnStatement(call(member(budget, "call"), f.NewNumericLiteral("0", ast.TokenFlagsNone), f.NewArrowFunction(nil, nil, f.NewNodeList(nil), nil, nil, f.NewToken(ast.KindEqualsGreaterThanToken), call(member(tx.effectName(), "runSync"), heapCall)))), nil)
		recurBody := f.NewBlock(f.NewNodeList(append([]*ast.Node{guard}, eagerBody.Statements()...)), true)
		recurParams := f.NewNodeList(append([]*ast.Node{f.NewParameterDeclaration(nil, nil, budgetName, nil, nil, nil)}, parameters.Nodes...))
		recur = f.NewFunctionDeclaration(nil, nil, f.NewIdentifier(name+"$recur"), nil, recurParams, nil, nil, recurBody)
		// Every native body preserves the incoming shared state: only
		// opaque exits publish it, and only heap entry reserves it. An
		// ordinary direct entry can therefore forward the available
		// budget without holding it away from nested acyclic helpers.
		remaining := f.NewBinaryExpression(nil, member(budget, "limit"), nil, f.NewToken(ast.KindMinusToken), depth)
		invoke := call(f.NewIdentifier(name+"$recur"), append([]*ast.Node{remaining}, args...)...)
		if usesThis(node.Body()) {
			invoke = call(member(f.NewIdentifier(name+"$recur"), "call"), append([]*ast.Node{f.NewThisExpression(), remaining}, args...)...)
		}
		body = f.NewBlock(f.NewNodeList([]*ast.Node{f.NewReturnStatement(invoke)}), true)
		if tx.hasPlainCallbackCycle(node) {
			// Enter callback-bearing bodies without an extra forwarding
			// frame. Their raw callback recursion retains the host limit.
			declaration := f.NewVariableStatement(nil, f.NewVariableDeclarationList(f.NewNodeList([]*ast.Node{f.NewVariableDeclaration(budgetName, nil, nil, remaining)}), ast.NodeFlagsConst))
			body = f.NewBlock(f.NewNodeList(append([]*ast.Node{declaration, guard}, eagerBody.Statements()...)), true)
		}
	} else {
		guard := f.NewIfStatement(f.NewBinaryExpression(nil, depth, nil, f.NewToken(ast.KindGreaterThanEqualsToken), member(budget, "limit")),
			f.NewReturnStatement(call(member(tx.effectName(), "runSync"), heapCall)), nil)
		finally := f.NewBlock(f.NewNodeList([]*ast.Node{update(ast.KindMinusEqualsToken)}), true)
		body = f.NewBlock(f.NewNodeList([]*ast.Node{guard, update(ast.KindPlusEqualsToken), f.NewTryStatement(eagerBody, nil, finally)}), true)
	}
	direct := f.NewFunctionDeclaration(modifiers, nil, f.NewIdentifier(name+"$direct"), nil, parameters, nil, nil, body)
	tx.EmitContext().SetSourceMapRange(direct, node.Loc)
	// Register private cross-module entries without mutating the function.
	statements := append([]*ast.Node{effect, direct, raw}, extra...)
	registration := []*ast.Node{f.NewIdentifier(name + "$direct"), f.NewIdentifier(name + "$stack")}
	if recur != nil {
		statements = append(statements, recur)
		registration = append(registration, f.NewIdentifier(name+"$recur"))
	}
	statements = append(statements, f.NewExpressionStatement(call(member(budget, "register"), registration...)))
	return f.NewSyntaxList(append(statements, tx.directMarker(node)...))
}

// directMarker returns the statements that follow a plain function's two
// forms: the D268 marker, which puts purity on the function object so export
// aliases preserve it, and for a default export, the variant's export as
// default$direct.
func (tx *lowering) directMarker(node *ast.Node) []*ast.Node {
	f := tx.Factory()
	name := node.Name().Text()
	marker := f.NewElementAccessExpression(f.NewIdentifier(name), nil, tx.runtimeMember("$direct"), ast.NodeFlagsNone)
	assign := f.NewBinaryExpression(nil, marker, nil, f.NewToken(ast.KindEqualsToken), f.NewIdentifier(name+"$direct"))
	statements := []*ast.Node{f.NewExpressionStatement(assign)}
	if ast.HasSyntacticModifier(node, ast.ModifierFlagsDefault) {
		alias := f.NewExportSpecifier(false, f.NewIdentifier(name+"$direct"), f.NewIdentifier("default$direct"))
		statements = append(statements, f.NewExportDeclaration(nil, false, f.NewNamedExports(f.NewNodeList([]*ast.Node{alias})), nil, nil))
	}
	return statements
}

// visitDirectReference writes the $direct variant in place of node: the
// callee of a call that runs inline, or a function value passed where a plain
// function is expected.
func (tx *lowering) visitDirectReference(node *ast.Node, ref *directReference) *ast.Node {
	f := tx.Factory()
	var call *ast.Node
	switch {
	case ast.IsTaggedTemplateExpression(node):
		template := node.AsTaggedTemplateExpression()
		tag := tx.directExpression(template.Tag, ref)
		if tx.rawPlain && tx.plan.rawRefs[ref] && ref.access != nil {
			bind := f.NewPropertyAccessExpression(tag, nil, f.NewIdentifier("bind"), ast.NodeFlagsNone)
			tag = f.NewCallExpression(bind, nil, nil, f.NewNodeList([]*ast.Node{tx.Visitor().VisitNode(ref.access.Expression())}), ast.NodeFlagsNone)
		}
		call = f.UpdateTaggedTemplateExpression(template, tag, template.QuestionDotToken, nil, tx.Visitor().VisitNode(template.Template), node.Flags)
	case node.Kind == ast.KindCallExpression && tx.eagerBudget != nil && tx.plan.fastRefs[ref] && !tx.rawPlain:
		var callee, lookup *ast.Node
		if ref.specifier == nil && ref.access == nil {
			callee = f.NewIdentifier(ref.name[:len(ref.name)-len("$direct")] + "$recur")
		} else {
			callee = f.NewUniqueName("recur")
			tx.EmitContext().AddVariableDeclaration(callee)
			lookup = tx.cachedPlainEntry(tx.directExpression(node.Expression(), ref))
		}
		next := f.NewBinaryExpression(nil, tx.eagerBudget, nil, f.NewToken(ast.KindMinusToken), f.NewNumericLiteral(strconv.Itoa(tx.eagerWeight), ast.TokenFlagsNone))
		arguments := append([]*ast.Node{next}, tx.Visitor().VisitNodes(node.ArgumentList()).Nodes...)
		call = f.UpdateCallExpression(node.AsCallExpression(), callee, node.QuestionDotToken(), nil, f.NewNodeList(arguments), node.Flags)
		if ref.specifier != nil {
			tx.EmitContext().AddEmitFlags(call, printer.EFIndirectCall)
		}
		if ref.specifier != nil || ref.access != nil {
			// During circular module initialization the hoisted direct
			// function can exist before its private entry marker is set.
			// Keep the ordinary entry as a synchronous fallback.
			direct := tx.directExpression(node.Expression(), ref)
			present := f.NewParenthesizedExpression(f.NewBinaryExpression(nil, callee, nil, f.NewToken(ast.KindEqualsToken), lookup))
			fallback := f.NewCallExpression(direct, nil, nil, tx.Visitor().VisitNodes(node.ArgumentList()), ast.NodeFlagsNone)
			if ref.specifier != nil {
				tx.EmitContext().AddEmitFlags(fallback, printer.EFIndirectCall)
			}
			if ref.access != nil {
				// Namespace calls retain their namespace receiver.
				invoke := f.NewPropertyAccessExpression(callee, nil, f.NewIdentifier("call"), ast.NodeFlagsNone)
				call = f.NewCallExpression(invoke, nil, nil, f.NewNodeList(append([]*ast.Node{tx.Visitor().VisitNode(ref.access.Expression())}, arguments...)), ast.NodeFlagsNone)
			}
			call = f.NewConditionalExpression(present, f.NewToken(ast.KindQuestionToken), call, f.NewToken(ast.KindColonToken), tx.scopedPlainBudget(fallback))
		}
	case node.Kind == ast.KindCallExpression:
		callee := tx.directExpression(node.Expression(), ref)
		arguments := tx.Visitor().VisitNodes(node.ArgumentList())
		if tx.rawPlain && tx.plan.rawRefs[ref] && ref.access != nil {
			// A namespace call keeps its receiver even when its private
			// stack entry is selected through a nullish fallback.
			callee = f.NewPropertyAccessExpression(callee, nil, f.NewIdentifier("call"), ast.NodeFlagsNone)
			arguments = f.NewNodeList(append([]*ast.Node{tx.Visitor().VisitNode(ref.access.Expression())}, arguments.Nodes...))
		}
		call = f.UpdateCallExpression(node.AsCallExpression(), callee, node.QuestionDotToken(), nil /*typeArguments*/, arguments, node.Flags)
	default:
		return tx.directExpression(node, ref)
	}
	if tx.commonJS && ref.specifier != nil {
		// (0, m_1.double$direct)(n): the variant is called without a receiver,
		// as TypeScript's CommonJS transform calls an imported function.
		tx.EmitContext().AddEmitFlags(call, printer.EFIndirectCall)
	}
	return call
}

// directExpression returns the expression that names ref's variant in place
// of node: double$direct, M.double$direct through a namespace import, or, in a
// file emitted as CommonJS, m_1.double$direct through a named or default
// import (importedVariant).
func (tx *lowering) directExpression(node *ast.Node, ref *directReference) *ast.Node {
	f := tx.Factory()
	var expression *ast.Node
	switch {
	case ref.access != nil:
		receiver := tx.Visitor().VisitNode(ref.access.Expression())
		expression = f.NewPropertyAccessExpression(receiver, nil /*questionDotToken*/, f.NewIdentifier(ref.name), ast.NodeFlagsNone)
	case ref.specifier != nil && tx.commonJS:
		expression = tx.importedVariant(ref.specifier)
	default:
		expression = f.NewIdentifier(ref.name)
	}
	if tx.rawPlain && tx.plan.rawRefs[ref] {
		if ref.specifier == nil && ref.access == nil {
			expression = f.NewIdentifier(ref.name[:len(ref.name)-len("$direct")] + "$stack")
		} else {
			expression = tx.eagerEntry("stack", expression)
		}
	}
	tx.EmitContext().SetOriginal(expression, node)
	expression.Loc = node.Loc
	return expression
}

// importedVariant returns, in a file emitted as CommonJS, the expression that
// reads the $direct variant of binding, an import specifier or default import
// clause, from the module its import requires: m_1.double$direct, or
// m_1.default$direct. TypeScript's CommonJS transform finds what an identifier
// names through the checker, which knows nothing of the variants import an ES
// module gets (visitImportWithDirectVariants): it would rewrite double$direct
// to the import's own binding, m_1.double. This is the property it writes for
// that binding, with the variant's name.
func (tx *lowering) importedVariant(binding *ast.Node) *ast.Node {
	f := tx.Factory()
	module := f.NewGeneratedNameForNode(ast.FindAncestor(binding, ast.IsImportDeclaration))
	if ast.IsImportClause(binding) {
		return f.NewPropertyAccessExpression(module, nil /*questionDotToken*/, f.NewIdentifier("default$direct"), ast.NodeFlagsNone)
	}
	imported := binding.AsImportSpecifier().PropertyNameOrName()
	if ast.IsStringLiteral(imported) {
		return f.NewElementAccessExpression(module, nil /*questionDotToken*/, f.NewStringLiteral(imported.Text()+"$direct", ast.TokenFlagsNone), ast.NodeFlagsNone)
	}
	return f.NewPropertyAccessExpression(module, nil /*questionDotToken*/, f.NewIdentifier(imported.Text()+"$direct"), ast.NodeFlagsNone)
}

// visitImportWithDirectVariants keeps an import of plain functions and adds
// one that brings in their $direct variants:
//
//	import { double } from "./math.js"
//	import { double$direct } from "./math.js"
func (tx *lowering) visitImportWithDirectVariants(node *ast.Node) *ast.Node {
	f := tx.Factory()
	visited := tx.visitImportOrExport(node)
	var specifiers []*ast.Node
	for _, specifier := range tx.plan.directImports[node] {
		local := specifier.Name().Text()
		var propertyName *ast.Node
		if ast.IsImportClause(specifier) {
			propertyName = f.NewIdentifier("default$direct")
		} else if property := specifier.AsImportSpecifier().PropertyName; property != nil {
			imported := property.Text()
			if ast.IsStringLiteral(property) {
				propertyName = f.NewStringLiteral(imported+"$direct", ast.TokenFlagsNone)
			} else if imported != local {
				propertyName = f.NewIdentifier(imported + "$direct")
			}
		}
		specifiers = append(specifiers, f.NewImportSpecifier(false /*isTypeOnly*/, propertyName, f.NewIdentifier(local+"$direct")))
	}
	clause := f.NewImportClause(ast.KindUnknown, nil /*name*/, f.NewNamedImports(f.NewNodeList(specifiers)))
	specifier := f.NewStringLiteral(tx.outputSpecifier(node.ModuleSpecifier()).Text(), ast.TokenFlagsNone)
	variants := f.NewImportDeclaration(nil /*modifiers*/, clause, specifier, nil /*attributes*/)
	ast.SetParentInChildren(variants)
	return f.NewSyntaxList([]*ast.Node{visited, variants})
}

// Preserve public Effect wrappers and export their plain variants under
// matching names. Export-star declarations already forward both forms.
func (tx *lowering) visitExportWithDirectVariants(node *ast.Node) *ast.Node {
	f := tx.Factory()
	visited := tx.visitImportOrExport(node)
	if tx.commonJS && node.AsExportDeclaration().ModuleSpecifier == nil {
		return f.NewSyntaxList(append([]*ast.Node{visited}, tx.defineDirectExports(node)...))
	}
	var specifiers []*ast.Node
	for _, specifier := range tx.plan.directExports[node] {
		local := specifier.AsExportSpecifier().PropertyName
		if local == nil {
			local = specifier.Name()
		}
		name := func(node *ast.Node) *ast.Node {
			if ast.IsStringLiteral(node) {
				return f.NewStringLiteral(node.Text()+"$direct", ast.TokenFlagsNone)
			}
			return f.NewIdentifier(node.Text() + "$direct")
		}
		specifiers = append(specifiers, f.NewExportSpecifier(false /*isTypeOnly*/, name(local), name(specifier.Name())))
	}
	decl := visited.AsExportDeclaration()
	variants := f.NewExportDeclaration(nil /*modifiers*/, false /*isTypeOnly*/, f.NewNamedExports(f.NewNodeList(specifiers)), decl.ModuleSpecifier, decl.Attributes)
	ast.SetParentInChildren(variants)
	return f.NewSyntaxList([]*ast.Node{visited, variants})
}

// defineDirectExports returns, for a local export list in a file emitted as
// CommonJS, a statement per plain function that defines its variant on
// exports as a live binding, the form TypeScript writes for an exported
// import:
//
//	Object.defineProperty(exports, "alias$direct", { enumerable: true, get: () => double$direct })
//
// TypeScript's CommonJS transform finds a local export's value through the
// checker, which knows no $direct binding, so an export list that named the
// variants would leave them undefined. An imported function's variant is read
// from its module (importedVariant), so the import may stand anywhere.
func (tx *lowering) defineDirectExports(node *ast.Node) []*ast.Node {
	f := tx.Factory()
	var statements []*ast.Node
	for _, specifier := range tx.plan.directExports[node] {
		var value *ast.Node
		if binding := tx.plan.exportedImports[specifier]; binding != nil {
			value = tx.importedVariant(binding)
		} else {
			local := specifier.AsExportSpecifier().PropertyName
			if local == nil {
				local = specifier.Name()
			}
			value = f.NewIdentifier(local.Text() + "$direct")
		}
		getter := f.NewArrowFunction(nil /*modifiers*/, nil /*typeParameters*/, f.NewNodeList(nil), nil /*type*/, nil /*fullSignature*/, f.NewToken(ast.KindEqualsGreaterThanToken), value)
		descriptor := f.NewObjectLiteralExpression(f.NewNodeList([]*ast.Node{
			f.NewPropertyAssignment(nil /*modifiers*/, f.NewIdentifier("enumerable"), nil /*postfixToken*/, nil /*typeNode*/, f.NewKeywordExpression(ast.KindTrueKeyword)),
			f.NewPropertyAssignment(nil /*modifiers*/, f.NewIdentifier("get"), nil /*postfixToken*/, nil /*typeNode*/, getter),
		}), false /*multiLine*/)
		define := f.NewPropertyAccessExpression(f.NewIdentifier("Object"), nil /*questionDotToken*/, f.NewIdentifier("defineProperty"), ast.NodeFlagsNone)
		name := f.NewStringLiteral(specifier.Name().Text()+"$direct", ast.TokenFlagsNone)
		call := f.NewCallExpression(define, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{f.NewIdentifier("exports"), name, descriptor}), ast.NodeFlagsNone)
		statements = append(statements, f.NewExpressionStatement(call))
	}
	return statements
}

// visitExportDefaultWithDirectVariant keeps `export default double` and
// exports double's variant beside it, since a default import of the module
// calls default$direct:
//
//	export default double
//	export const default$direct = double$direct
//
// The variant is double$direct, an imported variant (visitImportWithDirectVariants,
// or importedVariant in CommonJS), or M.double$direct, and is bound when the
// default is.
func (tx *lowering) visitExportDefaultWithDirectVariant(node *ast.Node) *ast.Node {
	f := tx.Factory()
	visited := tx.Visitor().VisitEachChild(node)
	variant := tx.directExpression(node.AsExportAssignment().Expression, tx.plan.directDefaults[node])
	declaration := f.NewVariableDeclaration(f.NewIdentifier("default$direct"), nil /*exclamationToken*/, nil /*typeNode*/, variant)
	list := f.NewVariableDeclarationList(f.NewNodeList([]*ast.Node{declaration}), ast.NodeFlagsConst)
	export := f.NewVariableStatement(f.NewModifierList([]*ast.Node{f.NewModifier(ast.KindExportKeyword)}), list)
	return f.NewSyntaxList([]*ast.Node{visited, export})
}

// eagerEntry finds a private entry through the shared runtime registry.
func (tx *lowering) eagerEntry(kind string, direct *ast.Node) *ast.Node {
	f := tx.Factory()
	method := f.NewPropertyAccessExpression(tx.runtimeMember("$eager"), nil, f.NewIdentifier(kind), ast.NodeFlagsNone)
	return f.NewCallExpression(method, nil, nil, f.NewNodeList([]*ast.Node{direct}), ast.NodeFlagsNone)
}

// publishPlainBudget makes the current native depth visible to opaque calls
// and accessors. Bodies that publish restore their incoming state in finally.
// Known calls forward the budget directly and require neither shared writes
// nor finally on their hot path.
func (tx *lowering) publishPlainBudget(expression *ast.Node) *ast.Node {
	f := tx.Factory()
	tx.eagerEscapes = true
	state := tx.runtimeMember("$eager")
	depth := f.NewPropertyAccessExpression(state, nil, f.NewIdentifier("depth"), ast.NodeFlagsNone)
	limit := f.NewPropertyAccessExpression(state, nil, f.NewIdentifier("limit"), ast.NodeFlagsNone)
	remaining := f.NewBinaryExpression(nil, tx.eagerBudget, nil, f.NewToken(ast.KindMinusToken), f.NewNumericLiteral(strconv.Itoa(tx.eagerWeight), ast.TokenFlagsNone))
	used := f.NewBinaryExpression(nil, limit, nil, f.NewToken(ast.KindMinusToken), f.NewParenthesizedExpression(remaining))
	assign := f.NewBinaryExpression(nil, depth, nil, f.NewToken(ast.KindEqualsToken), used)
	return f.NewParenthesizedExpression(f.NewBinaryExpression(nil, assign, nil, f.NewToken(ast.KindCommaToken), expression))
}

func (tx *lowering) restorePlainBudget(body *ast.Node) *ast.Node {
	f := tx.Factory()
	depth := f.NewPropertyAccessExpression(tx.runtimeMember("$eager"), nil, f.NewIdentifier("depth"), ast.NodeFlagsNone)
	previous := f.NewUniqueName("previousDepth")
	declaration := f.NewVariableStatement(nil, f.NewVariableDeclarationList(f.NewNodeList([]*ast.Node{f.NewVariableDeclaration(previous, nil, nil, depth)}), ast.NodeFlagsConst))
	restore := f.NewExpressionStatement(f.NewBinaryExpression(nil, depth, nil, f.NewToken(ast.KindEqualsToken), previous))
	return f.NewBlock(f.NewNodeList([]*ast.Node{declaration, f.NewTryStatement(body, nil, f.NewBlock(f.NewNodeList([]*ast.Node{restore}), true))}), true)
}

// scopedPlainBudget is for a cold fallback (an early circular-module call).
// It must not add a finally to every ordinary cross-module recursive frame.
func (tx *lowering) scopedPlainBudget(expression *ast.Node) *ast.Node {
	f := tx.Factory()
	remaining := f.NewBinaryExpression(nil, tx.eagerBudget, nil, f.NewToken(ast.KindMinusToken), f.NewNumericLiteral(strconv.Itoa(tx.eagerWeight), ast.TokenFlagsNone))
	thunk := f.NewArrowFunction(nil, nil, f.NewNodeList(nil), nil, nil, f.NewToken(ast.KindEqualsGreaterThanToken), expression)
	invoke := f.NewPropertyAccessExpression(tx.runtimeMember("$eager"), nil, f.NewIdentifier("call"), ast.NodeFlagsNone)
	return f.NewCallExpression(invoke, nil, nil, f.NewNodeList([]*ast.Node{remaining, thunk}), ast.NodeFlagsNone)
}

func (tx *lowering) hasPlainCallbackCycle(fn *ast.Node) bool {
	var visit func(*ast.Node, bool) bool
	visit = func(node *ast.Node, nested bool) bool {
		nested = nested || ast.IsFunctionLike(node)
		if ref := tx.plan.directRefs[node]; ref != nil && nested && tx.plan.rawRefs[ref] {
			return true
		}
		return node.ForEachChild(func(child *ast.Node) bool { return visit(child, nested) })
	}
	return visit(fn.Body(), false)
}

// Cache only private metadata, with the live direct binding as its key. A
// circular import's missing registration is retried; replacing a CommonJS
// export invalidates the cache. Function objects remain untouched.
func (tx *lowering) cachedPlainEntry(direct *ast.Node) *ast.Node {
	f := tx.Factory()
	target, entry := tx.fileName("nativeTarget"), tx.fileName("nativeEntry")
	for _, name := range []*ast.Node{target, entry} {
		tx.eagerCaches = append(tx.eagerCaches, f.NewVariableDeclaration(name, nil, nil, nil))
	}
	equal := f.NewBinaryExpression(nil, target, nil, f.NewToken(ast.KindEqualsEqualsEqualsToken), direct)
	present := f.NewBinaryExpression(nil, entry, nil, f.NewToken(ast.KindAmpersandAmpersandToken), equal)
	assign := f.NewBinaryExpression(nil, target, nil, f.NewToken(ast.KindEqualsToken), direct)
	refresh := f.NewBinaryExpression(nil, entry, nil, f.NewToken(ast.KindEqualsToken), tx.eagerEntry("recur", target))
	missing := f.NewParenthesizedExpression(f.NewBinaryExpression(nil, assign, nil, f.NewToken(ast.KindCommaToken), refresh))
	return f.NewConditionalExpression(present, f.NewToken(ast.KindQuestionToken), entry, f.NewToken(ast.KindColonToken), missing)
}

// Keep property references intact for receivers, assignment targets and delete;
// publication belongs to the enclosing operation. Parentheses preserve a JS
// reference too: wrapping (object.method) alone would discard its receiver.
func plainPropertyEscapes(node *ast.Node) bool {
	if ast.IsWriteAccess(node) {
		return false
	}
	subject := node
	for subject.Parent != nil {
		switch subject.Parent.Kind {
		case ast.KindParenthesizedExpression, ast.KindAsExpression, ast.KindTypeAssertionExpression, ast.KindNonNullExpression, ast.KindSatisfiesExpression:
			subject = subject.Parent
		default:
			parent := subject.Parent
			if parent.Kind == ast.KindDeleteExpression {
				return false
			}
			if isInvocation(parent) && ast.GetInvokedExpression(parent) == subject {
				return false
			}
			if (ast.IsNewExpression(parent) || ast.IsPropertyAccessExpression(parent) || ast.IsElementAccessExpression(parent)) && parent.Expression() == subject {
				return false
			}
			return true
		}
	}
	return true
}
