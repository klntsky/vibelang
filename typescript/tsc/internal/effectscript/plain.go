package effectscript

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
)

// A function declaration that performs no Effect operation and cannot fail
// compiles to plain JavaScript (design D10). Eligibility is a greatest
// fixpoint over the call graph (D56): a group of functions that call one
// another compiles to plain JavaScript when each would with the calls inside
// the group counted as plain, so pure recursion runs on the JavaScript stack
// instead of on Effect frames.

type plainState uint8

const (
	plainUnknown   plainState = iota
	plainExploring            // in the group being decided: a call to it counts as plain
	plainYes
	plainNo
)

// isPlainCandidate reports whether D10 decides how fn compiles: a named .es
// function declaration with a body and no overloads.
func isPlainCandidate(fn *ast.Node) bool {
	if fn == nil || fn.Kind != ast.KindFunctionDeclaration || fn.Name() == nil || !isLowerable(fn) {
		return false
	}
	symbol := fn.Symbol()
	return symbol != nil && len(symbol.Declarations()) == 1
}

// isPlainFunction reports whether fn, a plain candidate, compiles to plain
// JavaScript.
func (a *analyzer) isPlainFunction(fn *ast.Node) bool {
	if !isPlainCandidate(fn) {
		return false
	}
	switch a.plain[fn] {
	case plainYes, plainExploring:
		return true
	case plainNo:
		return false
	}
	type member struct {
		ok   bool
		deps []*ast.Node
	}
	group := map[*ast.Node]*member{}
	queue := []*ast.Node{fn}
	for len(queue) > 0 {
		d := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if _, seen := group[d]; seen || a.plain[d] == plainYes || a.plain[d] == plainNo {
			continue
		}
		a.plain[d] = plainExploring
		m := &member{ok: !hasEscapingThrow(d) && !declaresUsing(d)}
		group[d] = m
		forEachCallInBody(d, func(call *ast.Node) bool {
			if callee := a.candidateCallee(call); callee != nil {
				switch a.plain[callee] {
				case plainNo:
					m.ok = false
				case plainUnknown, plainExploring:
					m.deps = append(m.deps, callee)
					queue = append(queue, callee)
				}
				return false
			}
			if a.performs(call) {
				m.ok = false
			}
			return false
		})
	}
	for changed := true; changed; {
		changed = false
		for _, m := range group {
			if !m.ok {
				continue
			}
			for _, dep := range m.deps {
				if dm, ok := group[dep]; ok && !dm.ok || a.plain[dep] == plainNo {
					m.ok = false
					changed = true
					break
				}
			}
		}
	}
	for d, m := range group {
		if m.ok {
			a.plain[d] = plainYes
		} else {
			a.plain[d] = plainNo
		}
	}
	return a.plain[fn] == plainYes
}

// candidateCallee returns the plain candidate that call calls, or nil.
func (a *analyzer) candidateCallee(call *ast.Node) *ast.Node {
	signature := a.checker.GetResolvedSignature(call)
	if signature == nil {
		return nil
	}
	if fn := implementationOf(signature.Declaration()); isPlainCandidate(fn) {
		return fn
	}
	return nil
}

// declaresUsing reports whether fn's own body declares a using variable,
// which acquires an Effect resource, so fn compiles to frames.
func declaresUsing(fn *ast.Node) bool {
	var visit func(n *ast.Node) bool
	visit = func(n *ast.Node) bool {
		if isFunctionBoundary(n) {
			return false
		}
		if n.Kind == ast.KindVariableDeclarationList && n.Flags&ast.NodeFlagsBlockScoped&(ast.NodeFlagsUsing|ast.NodeFlagsAwaitUsing) != 0 {
			return true
		}
		return n.ForEachChild(visit)
	}
	return visit(fn.Body())
}
