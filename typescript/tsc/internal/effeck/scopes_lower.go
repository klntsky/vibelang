package effeck

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
)

// scopePlan records the Scope regions of one file (scopes.go).
type scopePlan struct {
	scopeBlocks map[*ast.Node]bool      // blocks that are Scope regions
	usings      map[*ast.Node]bool      // using statements in those blocks
	forks       map[*ast.Node]*ast.Node // fork calls, mapped to the block whose region owns the fiber
	usingLoops  map[*ast.Node]bool      // for (using x of values) statements
}

func newScopePlan() scopePlan {
	return scopePlan{scopeBlocks: map[*ast.Node]bool{}, usings: map[*ast.Node]bool{}, forks: map[*ast.Node]*ast.Node{}, usingLoops: map[*ast.Node]bool{}}
}

// planScopes records the Scope regions of fn, a function the lowering turns
// into an Effect, and the using statements and fork calls they own.
func (p *scopePlan) planScopes(a *analyzer, fn *ast.Node) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		switch {
		case isFunctionBoundary(node):
			return false
		case isBlockUsing(node):
			p.scopeBlocks[node.Parent] = true
			p.usings[node] = true
		case isUsingForOf(node):
			p.usingLoops[node] = true
		case node.Kind == ast.KindCallExpression:
			if owner := a.forkOwner(node); owner != nil {
				p.scopeBlocks[owner] = true
				p.forks[node] = owner
			}
		}
		return node.ForEachChild(visit)
	}
	if body := fn.Body(); body != nil {
		visit(body)
	}
}

// scoped reports whether the lowering of node belongs to a Scope region: node
// is a Scope region's block, a using statement in one, a fork it owns, or a
// for...of loop that takes its values with using.
func (p *scopePlan) scoped(node *ast.Node) bool {
	return p.scopeBlocks[node] || p.usings[node] || p.forks[node] != nil || p.usingLoops[node]
}

// visitScoped lowers a node that scoped reports.
func (tx *lowering) visitScoped(node *ast.Node) *ast.Node {
	switch node.Kind {
	case ast.KindBlock:
		return tx.visitScopeBlock(node)
	case ast.KindVariableStatement:
		return tx.visitUsing(node)
	case ast.KindForOfStatement:
		return tx.visitUsingForOf(node)
	default:
		return tx.visitFork(node)
	}
}

// scopeName returns a new name for the Scope of the region that block runs
// in, and records it for the using statements and forks the block owns.
func (tx *lowering) scopeName(block *ast.Node) *ast.Node {
	scope := tx.Factory().NewUniqueName("scope")
	if tx.scopes == nil {
		tx.scopes = map[*ast.Node]*ast.Node{}
	}
	tx.scopes[block] = scope
	return scope
}

// scopeRegion makes `runtime.$region(Effect.fnUntraced(function* <name>$using(scope) {
// statements }))`, the Effect that runs a Scope region whose closure, with
// statements as its body, belongs to region r and lowers block.
func (tx *lowering) scopeRegion(r *region, scope *ast.Node, block *ast.Node, statements []*ast.Node) *ast.Node {
	f := tx.Factory()
	parameter := f.NewVariableDeclaration(scope, nil /*exclamationToken*/, nil /*typeNode*/, nil /*initializer*/)
	closure := tx.regionFunction(r, "using", parameter, block, statements)
	if usesThis(block) {
		closure = f.NewFunctionBindCall(closure, f.NewThisExpression(), nil /*argumentsList*/)
	}
	run := tx.runtimeCall("$region", closure)
	run.Loc = block.Loc // ends where the closure ends, so the printer does not repeat the comments after the block inside the call
	return run
}

// statementsOf returns the statements that node, a visited statement, stands
// for: the children of a SyntaxList, or node itself.
func statementsOf(node *ast.Node) []*ast.Node {
	switch {
	case node == nil:
		return nil
	case node.Kind == ast.KindSyntaxList:
		return node.AsSyntaxList().Children
	}
	return []*ast.Node{node}
}

// visitScopeBlock lowers block, a Scope region, to a region whose closure
// receives the region's Scope, run by the runtime's $region, which is
// Effect.scopedWith: it makes the Scope, does not add it to the Context, and
// closes it when the closure's Effect exits.
//
//	{ using conn = Pool.acquire(); if (retry) continue; work(conn) }
//
// becomes
//
//	{
//	  const completion = yield* runtime.$region(Effect.fnUntraced(function* attempt$using(scope) {
//	    const conn = yield* runtime.$using(scope, (yield* Pool).acquire());
//	    if (retry) return { kind: "continue", label: "" };
//	    yield* work(conn);
//	    return runtime.$normal;
//	  }));
//	  if (completion.kind === "continue" && completion.label === "") continue;
//	}
//
// A function body that is a Scope region needs no completion record, because
// a return from its closure is a return from the function:
//
//	return yield* runtime.$region(Effect.fnUntraced(function* serve$using(scope) { … }));
func (tx *lowering) visitScopeBlock(block *ast.Node) *ast.Node {
	f := tx.Factory()
	scope := tx.scopeName(block)
	fn := block.Parent
	isBody := fn != nil && isFunctionBoundary(fn) && fn.Body() == block
	r := &region{}
	saved := tx.frame
	if !isBody {
		tx.frame = &frame{name: saved.name, region: r, handleClause: saved.handleClause}
	}
	statements := tx.Visitor().VisitNodes(block.AsBlock().Statements).Nodes
	tx.frame = saved

	run := tx.scopeRegion(r, scope, block, statements)
	var lowered []*ast.Node
	if isBody {
		lowered = []*ast.Node{f.NewReturnStatement(f.NewYieldExpression(f.NewToken(ast.KindAsteriskToken), run))}
	} else {
		lowered = statementsOf(tx.runRegion(block, run, r))
	}
	result := f.NewBlock(f.NewNodeList(lowered), true /*multiLine*/)
	tx.EmitContext().SetOriginal(result, block)
	result.Loc = block.Loc
	return result
}

// visitUsing lowers a using statement in a Scope region. Each declaration
// gets its value from the runtime's $using, which runs the acquisition with
// the region's Scope and releases a Disposable value with [Symbol.dispose]()
// when the region closes:
//
//	using file = Files.open(path), timer = startTimer()
//
// becomes, when Files.open returns an Effect and startTimer a Disposable,
//
//	const file = yield* runtime.$using(scope, (yield* Files).open(path)),
//	  timer = yield* runtime.$using(scope, Effect.succeed(startTimer()));
func (tx *lowering) visitUsing(node *ast.Node) *ast.Node {
	f := tx.Factory()
	scope := tx.scopes[node.Parent]
	if scope == nil {
		return tx.Visitor().VisitEachChild(node)
	}
	var declarations []*ast.Node
	for _, declaration := range node.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
		value := f.NewYieldExpression(f.NewToken(ast.KindAsteriskToken), tx.runtimeCall("$using", scope, tx.acquisition(declaration.Initializer())))
		declarations = append(declarations, f.NewVariableDeclaration(tx.Visitor().VisitNode(declaration.Name()), nil /*exclamationToken*/, nil /*typeNode*/, value))
	}
	statement := f.NewVariableStatement(nil /*modifiers*/, f.NewVariableDeclarationList(f.NewNodeList(declarations), ast.NodeFlagsConst))
	tx.EmitContext().SetOriginal(statement, node)
	statement.Loc = node.Loc
	return statement
}

// acquisition returns the Effect that $using runs for a using initializer: a
// call that runs an Effect, as the call that makes the Effect without running
// it, and any other value v as Effect.succeed(v).
func (tx *lowering) acquisition(initializer *ast.Node) *ast.Node {
	if initializer == nil {
		return tx.succeed(tx.Factory().NewVoidZeroExpression())
	}
	if call := ast.SkipParentheses(initializer); call.Kind == ast.KindCallExpression {
		switch {
		case tx.plan.values[call]:
			return tx.Visitor().VisitNode(call.Expression()) // p(), where p is an Effect value
		case tx.plan.runsCall(call):
			return tx.Visitor().VisitEachChild(call)
		}
	}
	return tx.succeed(tx.Visitor().VisitNode(initializer))
}

// succeed makes `Effect.succeed(value)`.
func (tx *lowering) succeed(value *ast.Node) *ast.Node {
	f := tx.Factory()
	succeed := f.NewPropertyAccessExpression(tx.effectName(), nil /*questionDotToken*/, f.NewIdentifier("succeed"), ast.NodeFlagsNone)
	return f.NewCallExpression(succeed, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{value}), ast.NodeFlagsNone)
}

// visitFork lowers fork(program, options) in a Scope region to
// `yield* runtime.$fork(program, scope, options)`, Effect.forkIn with the
// region's Scope: the region interrupts the fiber when it closes, and waits
// for the fiber's cleanup.
func (tx *lowering) visitFork(node *ast.Node) *ast.Node {
	scope := tx.scopes[tx.plan.forks[node]]
	if scope == nil || len(node.Arguments()) == 0 {
		return tx.visitRunningCall(node)
	}
	visited := tx.Visitor().VisitNodes(node.ArgumentList()).Nodes
	arguments := append([]*ast.Node{visited[0], scope}, visited[1:]...)
	run := tx.Factory().NewYieldExpression(tx.Factory().NewToken(ast.KindAsteriskToken), tx.runtimeCall("$fork", arguments...))
	tx.EmitContext().SetOriginal(run, node)
	return run
}

// visitUsingForOf lowers `for (using x of values) body` in an Effect
// function. Each iteration's body is a Scope region that takes the value
// with using first, so the value is released when the iteration ends, by any
// path, after what the body acquired:
//
//	for (using conn of connections) { if (conn.closed) break; send(conn) }
//
// becomes
//
//	for (const value of connections) {
//	  const completion = yield* runtime.$region(Effect.fnUntraced(function* f$using(scope) {
//	    const conn = yield* runtime.$using(scope, Effect.succeed(value));
//	    if (conn.closed) return { kind: "break", label: "" };
//	    yield* send(conn);
//	    return runtime.$normal;
//	  }));
//	  if (completion.kind === "break" && completion.label === "") break;
//	}
//
// The using declarations and forks of a block body join the same region.
func (tx *lowering) visitUsingForOf(node *ast.Node) *ast.Node {
	f := tx.Factory()
	loop := node.AsForInOrOfStatement()
	declaration := loop.Initializer.AsVariableDeclarationList().Declarations.Nodes[0]
	expression := tx.Visitor().VisitNode(loop.Expression)
	value := f.NewUniqueName("value")
	body := loop.Statement
	scope := tx.scopeName(body)

	// The loop is the target of the jumps its region dispatches, as
	// visitJumpTarget makes it for any other loop.
	tx.frame.targets = append(tx.frame.targets, target{loop: true})
	r := &region{}
	saved := tx.frame
	tx.frame = &frame{name: saved.name, region: r, handleClause: saved.handleClause}
	using := f.NewYieldExpression(f.NewToken(ast.KindAsteriskToken), tx.runtimeCall("$using", scope, tx.succeed(value)))
	statements := []*ast.Node{f.NewVariableStatement(nil /*modifiers*/, f.NewVariableDeclarationList(f.NewNodeList([]*ast.Node{
		f.NewVariableDeclaration(tx.Visitor().VisitNode(declaration.Name()), nil /*exclamationToken*/, nil /*typeNode*/, using),
	}), ast.NodeFlagsConst))}
	if body.Kind == ast.KindBlock {
		statements = append(statements, tx.Visitor().VisitNodes(body.AsBlock().Statements).Nodes...)
	} else {
		statements = append(statements, statementsOf(tx.Visitor().VisitNode(body))...)
	}
	tx.frame = saved
	dispatched := statementsOf(tx.runRegion(body, tx.scopeRegion(r, scope, body, statements), r))
	tx.frame.targets = tx.frame.targets[:len(tx.frame.targets)-1]

	initializer := f.NewVariableDeclarationList(f.NewNodeList([]*ast.Node{
		f.NewVariableDeclaration(value, nil /*exclamationToken*/, nil /*typeNode*/, nil /*initializer*/),
	}), ast.NodeFlagsConst)
	return f.UpdateForInOrOfStatement(loop, nil /*awaitModifier*/, initializer, expression, f.NewBlock(f.NewNodeList(dispatched), true /*multiLine*/))
}
