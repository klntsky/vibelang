package effectscript

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// inferredReturnType gives an .es function that is an Effect, and has no
// return type annotation, the type Effect<A, E, R>. A is what TypeScript
// inferred from its return statements, where each call that runs already has
// its success type. E and R join the error and requirement channels of every
// Effect the body runs. Outside .es function bodies, for example at the
// module top level or in TypeScript and .d.ts files, a call to the function
// is therefore an Effect value.
func inferredReturnType(c *checker.Checker, fn *ast.Node, t *checker.Type) *checker.Type {
	if !isLowerable(fn) || isLambda(fn) {
		return t // a lambda's type depends on its form (forms.go)
	}
	a := analyzerFor(c)
	if !a.isEffect(fn) {
		return t
	}
	info := a.effectOf(fn)
	if info == nil || info.effect == nil {
		return t
	}
	return c.NewTypeReference(info.effect, []*checker.Type{t, c.GetUnionType(info.errors), c.GetUnionType(info.requirements)})
}

// effectInfo lists what running a function's body adds to the error and
// requirement channels of its Effect.
type effectInfo struct {
	errors       []*checker.Type
	requirements []*checker.Type
	effect       *checker.Type // the generic Effect interface, from any Effect the body runs
}

func (info *effectInfo) add(other *effectInfo) {
	info.errors = append(info.errors, other.errors...)
	info.requirements = append(info.requirements, other.requirements...)
	if info.effect == nil {
		info.effect = other.effect
	}
}

// effectOf collects fn's effectInfo. It reads the channels of a TypeScript
// callee from its Effect return type and recurses into an .es callee instead
// of asking for that callee's return type, which may be the one the checker
// is inferring right now. A call cycle adds nothing, so E and R are
// incomplete on cycles until the checker solves them as a fixed point.
func (a *analyzer) effectOf(fn *ast.Node) *effectInfo {
	if info, ok := a.effects[fn]; ok {
		return info // nil while fn is being collected
	}
	a.effects[fn] = nil
	info := &effectInfo{}
	forEachCallInBody(fn, func(call *ast.Node) bool {
		if !a.runs(call) {
			return false
		}
		sig := a.checker.GetResolvedSignature(call)
		if callee := implementationOf(sig.Declaration()); callee != nil && isLowerable(callee) {
			if calleeInfo := a.effectOf(callee); calleeInfo != nil {
				info.add(calleeInfo)
			}
			return false
		}
		t := a.checker.GetReturnTypeOfSignature(sig)
		if e := channelType(a.checker, t, "_E"); e != nil {
			info.errors = append(info.errors, e)
		}
		if r := channelType(a.checker, t, "_R"); r != nil {
			info.requirements = append(info.requirements, r)
		}
		if info.effect == nil {
			info.effect = effectInterface(a.checker, t)
		}
		return false
	})
	a.effects[fn] = info
	return info
}

// effectInterface returns the generic Effect interface, given an Effect t.
func effectInterface(c *checker.Checker, t *checker.Type) *checker.Type {
	if symbol := effectInterfaceSymbol(c, t); symbol != nil {
		return c.GetDeclaredTypeOfSymbol(symbol)
	}
	return nil
}
