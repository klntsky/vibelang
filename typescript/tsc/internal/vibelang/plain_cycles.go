package vibelang

import (
	"slices"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
)

// Plain recursion uses the same bounded-eager protocol as runtime $fast.
// A cycle member retains its native JS body, with a remaining budget passed
// between cycle calls and acyclic helpers. Opaque calls publish the remaining
// budget; their native bodies restore shared state in finally. At the bound,
// a scoped reservation makes Effect.runSync continue through
// the Effect generator, whose cycle calls yield heap frames.
// Synchronous callbacks retain the original JS recursion; entering an Effect
// runner per callback would reduce the host's existing recursion limit.

// plainEdge is a call in a function's own body that runs a splittable plain
// function's $direct variant, and that function.
type plainEdge struct {
	call, callee *ast.Node
}

// plainEdgesOf returns the calls in fn's own body that run the $direct
// variant of a splittable plain function.
func (a *analyzer) plainEdgesOf(fn *ast.Node) []plainEdge {
	if edges, ok := a.plainEdges[fn]; ok {
		return edges
	}
	var edges []plainEdge
	a.forEachRunningCallInBody(fn, func(call *ast.Node) bool {
		if a.directCallee(call) != nil {
			if callee := esFunctionOf(a.checker, ast.GetInvokedExpression(call)); callee != nil {
				edges = append(edges, plainEdge{call, callee})
			}
		}
		return false
	})
	a.plainEdges[fn] = edges
	return edges
}

// plainReaches reports whether $direct calls lead from from's body to a call
// of to.
func (a *analyzer) plainReaches(from, to *ast.Node) bool {
	seen := map[*ast.Node]bool{}
	stack := []*ast.Node{from}
	for len(stack) > 0 {
		fn := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, edge := range a.plainEdgesOf(fn) {
			if edge.callee == to {
				return true
			}
			if !seen[edge.callee] {
				seen[edge.callee] = true
				stack = append(stack, edge.callee)
			}
		}
	}
	return false
}

// onPlainCycle reports whether fn is a splittable plain function whose
// $direct variant can call itself again, so that it compiles to an Effect
// generator.
func (a *analyzer) onPlainCycle(fn *ast.Node) bool {
	if cyclic, ok := a.plainCycles[fn]; ok {
		return cyclic
	}
	cyclic := a.isSplittable(fn) && a.isPlainFunction(fn) && a.plainReaches(fn, fn)
	a.plainCycles[fn] = cyclic
	return cyclic
}

// isCycleCall reports whether call, in the own body of a function on a plain
// cycle, calls a member of that cycle. It runs the callee's Effect form, as
// yield*, instead of its $direct variant.
func (a *analyzer) isCycleCall(call *ast.Node) bool {
	owner := ownerFunction(call)
	if owner == nil || !a.onPlainCycle(owner) {
		return false
	}
	edges := a.plainEdgesOf(owner)
	i := slices.IndexFunc(edges, func(edge plainEdge) bool { return edge.call == call })
	if i < 0 {
		return false
	}
	callee := edges[i].callee
	return callee == owner || a.onPlainCycle(callee) && a.plainReaches(callee, owner)
}

// plainBudgetParameters permits a hidden leading budget only when binding
// parameters has no effects and the source never reads its arguments object.
func plainBudgetParameters(fn *ast.Node) bool {
	if fn == nil || !hasSimpleParameters(fn) {
		return false
	}
	var reads func(*ast.Node) bool
	reads = func(n *ast.Node) bool {
		if ast.IsIdentifier(n) && n.Text() == "arguments" {
			return true
		}
		if ast.IsFunctionLike(n) && n.Kind != ast.KindArrowFunction {
			return false
		}
		return n.ForEachChild(reads)
	}
	return !fn.Body().ForEachChild(reads)
}

// plainFrameWeight charges large native frames more of the shared budget.
// It is a conservative syntactic estimate, not a host stack-size guarantee.
func plainFrameWeight(fn *ast.Node) int {
	slots := len(fn.Parameters())
	var count func(*ast.Node) bool
	count = func(n *ast.Node) bool {
		if ast.IsFunctionLike(n) {
			return false
		}
		if n.Kind == ast.KindVariableDeclaration || n.Kind == ast.KindBindingElement {
			slots++
		}
		n.ForEachChild(count)
		return false
	}
	fn.Body().ForEachChild(count)
	return 1 + slots/8
}

// forwardsPlainBudget identifies acyclic helpers whose native calls can enter
// a plain cycle. Their private entry forwards the budget without reserving it
// again. Ordinary callers keep the original direct entry.
func (a *analyzer) forwardsPlainBudget(fn *ast.Node) bool {
	if !a.isSplittable(fn) || !a.isPlainFunction(fn) || !plainBudgetParameters(fn) {
		return false
	}
	seen := map[*ast.Node]bool{fn: true}
	stack := []*ast.Node{fn}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, edge := range a.plainEdgesOf(current) {
			if a.onPlainCycle(edge.callee) {
				return true
			}
			if !seen[edge.callee] {
				seen[edge.callee] = true
				stack = append(stack, edge.callee)
			}
		}
	}
	return false
}
