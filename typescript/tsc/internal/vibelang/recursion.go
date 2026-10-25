package vibelang

import (
	"slices"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// Recursion (D52).
//
// E and R of a function come from its body, and the body reads the E and R of
// each .vibe function it calls. Functions that call each other, directly or
// through others, form a strongly connected component of that call graph. Its
// E and R are a fixpoint, computed inside the component only: every member
// starts with empty E and R, and the analyzer reads each member's body again,
// with the E and R found so far, until no member's E or R grows. The rows are
// finite and each step only adds to them, so the iteration ends. A component
// is solved after every component it calls, so nothing outside it is read
// twice.
//
//	function isEven(n: number): Effect<boolean, infer E, infer R> {
//	  if (n === 0) return true
//	  return isOdd(n - 1)
//	}
//	function isOdd(n: number): Effect<boolean, infer E, infer R> {
//	  if (n === 7) throw new Odd({ n })
//	  if (n === 0) return false
//	  return isEven(n - 1)
//	}
//	// both: Effect<boolean, Odd, never>
//
// A is TypeScript's to infer. When A depends on the function's own result, as
// it would above without the annotations, TypeScript gives up (TS7023), and
// the function needs a written contract: EK2102.

func init() {
	fileChecks = append(fileChecks, checkRecursion)
}

// bodyCallees returns the .vibe functions whose bodies fn's E and R read: the
// callees of the calls that run in fn's own body, except a callee whose
// contract states its E and R, or a generic callee that the call
// instantiates, whose E and R come from the call's type.
func (a *analyzer) bodyCallees(fn *ast.Node) []*ast.Node {
	var callees []*ast.Node
	a.forEachRunningCallInBody(fn, func(call *ast.Node) bool {
		if !a.runs(call) {
			return false
		}
		sig := a.checker.GetResolvedSignature(call)
		callee := implementationOf(sig.Declaration())
		// D269 excludes factory bodies; S3-3 keeps plain returned programs out of body cycles.
		if callee == nil || !isLowerable(callee) || a.isFactory(callee) || sig.Target() != nil || !a.isSplittable(callee) && !a.isEffect(callee) || slices.Contains(callees, callee) {
			return false
		}
		if k := a.contractOf(callee); k != nil && !k.inferredAny() {
			return false
		}
		callees = append(callees, callee)
		return false
	})
	return callees
}

// solve computes the effectInfo of fn and of each function fn's body reads
// that is not solved yet, one strongly connected component at a time, with
// Tarjan's algorithm: a component is solved after every component it calls.
func (a *analyzer) solve(fn *ast.Node) {
	index := map[*ast.Node]int{}
	low := map[*ast.Node]int{}
	var stack []*ast.Node
	onStack := map[*ast.Node]bool{}
	var connect func(v *ast.Node)
	connect = func(v *ast.Node) {
		index[v] = len(index)
		low[v] = index[v]
		stack = append(stack, v)
		onStack[v] = true
		a.connecting[v] = true
		for _, w := range a.bodyCallees(v) {
			if _, done := a.effects[w]; done {
				continue
			}
			if _, solving := a.approx[w]; solving {
				continue // an outer solve owns w's component
			}
			if _, seen := index[w]; !seen {
				connect(w)
				low[v] = min(low[v], low[w])
			} else if onStack[w] {
				low[v] = min(low[v], index[w])
			}
		}
		if low[v] != index[v] {
			return
		}
		var component []*ast.Node
		for {
			w := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[w] = false
			component = append(component, w)
			if w == v {
				break
			}
		}
		a.solveComponent(component)
	}
	connect(fn)
}

// solveComponent computes the effectInfo of each function in component. A
// function that calls none of the component, itself included, is read once.
// The members of a cycle start with empty E and R and are read until their E
// and R stop growing.
func (a *analyzer) solveComponent(component []*ast.Node) {
	cycle := len(component) > 1 || slices.Contains(a.bodyCallees(component[0]), component[0])
	for _, member := range component {
		a.approx[member] = &effectInfo{}
		if cycle {
			a.recursive[member] = true
		}
	}
	for {
		grew := false
		for _, member := range component {
			info := a.flowOf(member.Body())
			if !a.sameChannels(info, a.approx[member]) {
				grew = true
			}
			a.approx[member] = info
		}
		if !cycle || !grew {
			break
		}
	}
	for _, member := range component {
		a.effects[member] = a.approx[member]
		delete(a.approx, member)
		delete(a.connecting, member)
	}
}

// sameChannels reports whether x and y list the same E and R.
func (a *analyzer) sameChannels(x, y *effectInfo) bool {
	c := a.checker
	return c.GetUnionType(x.errors) == c.GetUnionType(y.errors) && c.GetUnionType(x.requirements) == c.GetUnionType(y.requirements)
}

// ownerFunction returns the function whose own body evaluates node, or nil at
// a module's top level.
func ownerFunction(node *ast.Node) *ast.Node {
	for n := node.Parent; n != nil; n = n.Parent {
		if isFunctionBoundary(n) {
			return n
		}
	}
	return nil
}

// checkRecursion reports EK2102 for each function on a call cycle in file
// whose A TypeScript could not infer because it depends on the function's own
// result, and covers TypeScript's TS7022, TS7023, or TS7024 there. It also
// reports EK2102 where the checker typed a catch variable, or a function, on a
// cycle before the cycle's E and R were known: there the written contracts
// break the cycle.
func checkRecursion(c *checker.Checker, file *ast.SourceFile) {
	a := analyzerFor(c)
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if a.reentered[node] {
			a.checkReenteredFunction(node)
		}
		if isLowerable(node) && node.Type() == nil && a.isEffect(node) {
			a.effectOf(node) // solves node's component
			if a.recursive[node] {
				a.checkRecursiveFunction(node)
			}
		}
		if node.Kind == ast.KindCatchClause && catches(node.Parent) && node.AsCatchClause().VariableDeclaration != nil && node.AsCatchClause().VariableDeclaration.Type() == nil {
			if fn := ownerFunction(node); fn != nil && a.recursive[fn] {
				a.checkRecursiveCatch(node)
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

func (a *analyzer) checkRecursiveFunction(fn *ast.Node) {
	c := a.checker
	name := functionDisplayName(fn)
	t := c.GetReturnTypeOfSignature(c.GetSignatureFromDeclaration(fn))
	at := fn
	if fn.Name() != nil {
		at = fn.Name()
	} else if parent := fn.Parent; parent.Kind == ast.KindVariableDeclaration {
		at = parent.Name()
	}
	if t.Flags()&checker.TypeFlagsAny != 0 {
		d := report(c, at, EK2102, name)
		note(d, "Write its contract with its success type as A, such as Effect<number, infer E, infer R>, which still infers E and R.")
		cover(c, at)
		if parent := fn.Parent; parent.Kind == ast.KindVariableDeclaration {
			cover(c, parent.Name())
		}
		return
	}
	info := a.effectOf(fn)
	if channelType(c, t, "_E") != c.GetUnionType(info.errors) || channelType(c, t, "_R") != c.GetUnionType(info.requirements) {
		d := report(c, at, EK2102.With("{0} is on a call cycle that TypeScript typed before the cycle's E and R were known, so its contract must be written."), name)
		note(d, "Write its contract, such as Effect<A, infer E, infer R>, or write the contract of a function it calls on the cycle.")
	}
}

// checkReenteredFunction reports EK2102 at fn, whose E and R TypeScript read
// while fn's own callees were being found: a call of fn passed by name in
// fn's body is typed from fn's contract, so where the contract leaves E or R
// to inference, the call was typed before they were known.
func (a *analyzer) checkReenteredFunction(fn *ast.Node) {
	c := a.checker
	name := functionDisplayName(fn)
	at := fn
	if fn.Name() != nil {
		at = fn.Name()
	} else if parent := fn.Parent; parent.Kind == ast.KindVariableDeclaration {
		at = parent.Name()
	}
	d := report(c, at, EK2102.With("{0} passes a call of itself by name, which TypeScript typed before {0}'s E and R were known, so its contract must write them."), name)
	note(d, "Write every channel of its contract, such as number!Failure! (R = never) or Effect<number, Failure, Needs>.")
}

// checkRecursiveCatch reports EK2102 at clause when the checker typed its
// variable before the failures of the cycle its try block calls were known.
func (a *analyzer) checkRecursiveCatch(clause *ast.Node) {
	c := a.checker
	declaration := clause.AsCatchClause().VariableDeclaration
	typed := c.GetTypeOfSymbol(c.GetSymbolOfDeclaration(declaration))
	if typed == c.GetUnionType(a.tryBlockFlow(clause.Parent.AsTryStatement().TryBlock).errors) {
		return
	}
	d := report(c, declaration, EK2102.With("this catch handles failures from a call cycle, which were not known when its variable was typed, so the cycle needs a written contract."))
	note(d, "Write the contract of the function this try block calls on the cycle, such as Effect<A, E, infer R>.")
	if d != nil {
		cover(c, clause) // TypeScript's errors about the variable's early type
	}
}
