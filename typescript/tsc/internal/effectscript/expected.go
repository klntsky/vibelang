package effectscript

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// The expected type of a lambda is the declared, uninstantiated parameter type
// of the candidate signature its call is being checked against (design D44):
// in ids.map((id) => find(id)), map declares (value: T, ...) => U, a plain
// function type, whatever U is inferred to be. The instantiated contextual
// type is used only where the declared type says nothing, as with a bare type
// parameter that another argument fixes, such as Layer.succeed's NoInfer<S>.

// expectedAt returns the types expected at path below the arguments of call
// when they are checked against signature. fn is the lambda at the end of the
// path, whose contextual type is the fallback.
func (a *analyzer) expectedAt(call *ast.Node, signature *checker.Signature, path []pathStep, fn *ast.Node) []*checker.Type {
	if declared := a.declaredSignature(signature); declared != nil {
		if t := a.checker.GetTypeAtPosition(declared, path[0].argument); t != nil {
			types := []*checker.Type{t}
			for _, step := range path[1:] {
				types = a.stepInto(types, step)
			}
			if useful := a.usefulTypes(types); len(useful) > 0 && a.classify(useful).any() {
				return useful
			}
		}
	}
	return a.contextualTypes(fn)
}

// declaredSignature returns the uninstantiated signature behind a candidate.
func (a *analyzer) declaredSignature(signature *checker.Signature) *checker.Signature {
	if d := signature.Declaration(); d != nil && !ast.IsJSDocSignature(d) && ast.IsFunctionLike(d) {
		return a.checker.GetSignatureFromDeclaration(d)
	}
	for signature.Target() != nil {
		signature = signature.Target()
	}
	return signature
}

// contextualTypes returns the type TypeScript expects at node, instantiated
// with whatever inference has found so far, as for a contextual signature.
func (a *analyzer) contextualTypes(node *ast.Node) []*checker.Type {
	if t := a.checker.GetContextualType(node, checker.ContextFlagsSignature); t != nil {
		return []*checker.Type{t}
	}
	return nil
}

func (a *analyzer) stepInto(types []*checker.Type, step pathStep) []*checker.Type {
	var out []*checker.Type
	for _, t := range types {
		if step.element {
			out = append(out, a.elementTypes(t, 0)...)
		} else {
			out = append(out, a.propertyTypes(t, step.property, 0)...)
		}
	}
	return out
}

// usefulTypes drops bare type parameters, which say nothing about form.
func (a *analyzer) usefulTypes(types []*checker.Type) []*checker.Type {
	var out []*checker.Type
	for _, t := range types {
		if !a.isBareTypeParameter(t) {
			out = append(out, t)
		}
	}
	return out
}

// isBareTypeParameter reports whether t is a type parameter, possibly wrapped
// in NoInfer, whose constraint says nothing.
func (a *analyzer) isBareTypeParameter(t *checker.Type) bool {
	t = unwrapSubstitution(t)
	if t.Flags()&checker.TypeFlagsTypeParameter == 0 {
		return false
	}
	constraint := a.checker.GetBaseConstraintOfType(t)
	return constraint == nil || constraint == t || constraint.Flags()&checker.TypeFlagsUnknown != 0
}

func unwrapSubstitution(t *checker.Type) *checker.Type {
	for t.Flags()&checker.TypeFlagsSubstitution != 0 {
		t = t.AsSubstitutionType().BaseType()
	}
	return t
}

// expectation lists the forms a set of expected types accepts.
type expectation struct {
	program  bool // an Effect value
	effectFn bool // a function that returns an Effect
	plainFn  bool // a function that returns something else
}

func (e expectation) any() bool { return e.program || e.effectFn || e.plainFn }

// classify reads which forms the types accept. A union of an Effect and a
// function type accepts both (design D39).
func (a *analyzer) classify(types []*checker.Type) expectation {
	var e expectation
	var visit func(t *checker.Type, depth int)
	visit = func(t *checker.Type, depth int) {
		t = unwrapSubstitution(t)
		if depth > 4 {
			return
		}
		switch {
		case t.Flags()&checker.TypeFlagsUnion != 0:
			for _, member := range t.Types() {
				visit(member, depth+1)
			}
			return
		case t.Flags()&checker.TypeFlagsTypeParameter != 0:
			if constraint := a.checker.GetBaseConstraintOfType(t); constraint != nil && constraint != t && constraint.Flags()&checker.TypeFlagsUnknown == 0 {
				visit(constraint, depth+1)
			}
			return
		}
		signatures := a.checker.GetSignaturesOfType(t, checker.SignatureKindCall)
		if len(signatures) == 0 {
			if a.isEffectLike(t) {
				e.program = true
			}
			return
		}
		for _, s := range signatures {
			if a.isEffectLike(unwrapSubstitution(a.checker.GetReturnTypeOfSignature(s))) {
				e.effectFn = true
			} else {
				e.plainFn = true
			}
		}
	}
	for _, t := range types {
		visit(t, 0)
	}
	return e
}

// isEffectLike reports whether t is an Effect or a type that extends one, or
// a type parameter constrained to one.
func (a *analyzer) isEffectLike(t *checker.Type) bool {
	if t.Flags()&(checker.TypeFlagsTypeParameter|checker.TypeFlagsIndexedAccess|checker.TypeFlagsConditional) != 0 {
		constraint := a.checker.GetBaseConstraintOfType(t)
		return constraint != nil && constraint != t && a.isEffectLike(constraint)
	}
	if t.Flags()&checker.TypeFlagsUnion != 0 {
		for _, member := range t.Types() {
			if !a.isEffectLike(member) {
				return false
			}
		}
		return len(t.Types()) > 0
	}
	if t.Flags()&checker.TypeFlagsObject == 0 && t.Flags()&checker.TypeFlagsIntersection == 0 {
		return false
	}
	if a.checker.GetPropertyOfType(t, effectTypeID) == nil {
		return false
	}
	if a.effectInterface == nil {
		a.effectInterface = effectInterface(a.checker, t)
	}
	return true
}

// elementTypes returns what iterating t yields, for an array literal element's
// expected type: index signature types, else the first type argument.
func (a *analyzer) elementTypes(t *checker.Type, depth int) []*checker.Type {
	t = unwrapSubstitution(t)
	if depth > 4 {
		return nil
	}
	if t.Flags()&checker.TypeFlagsUnion != 0 {
		var out []*checker.Type
		for _, member := range t.Types() {
			out = append(out, a.elementTypes(member, depth+1)...)
		}
		return out
	}
	if t.Flags()&checker.TypeFlagsTypeParameter != 0 {
		if constraint := a.checker.GetBaseConstraintOfType(t); constraint != nil && constraint != t {
			return a.elementTypes(constraint, depth+1)
		}
		return nil
	}
	var out []*checker.Type
	for _, info := range a.checker.GetIndexInfosOfType(t) {
		out = append(out, info.ValueType())
	}
	if len(out) == 0 && t.Flags()&checker.TypeFlagsObject != 0 && t.ObjectFlags()&checker.ObjectFlagsReference != 0 {
		if args := a.checker.GetTypeArguments(t); len(args) > 0 {
			out = append(out, args[0])
		}
	}
	return out
}

// propertyTypes returns the type of property name of t, else its string index
// type, for an object literal property's expected type.
func (a *analyzer) propertyTypes(t *checker.Type, name string, depth int) []*checker.Type {
	t = unwrapSubstitution(t)
	if depth > 4 {
		return nil
	}
	if t.Flags()&checker.TypeFlagsUnion != 0 {
		var out []*checker.Type
		for _, member := range t.Types() {
			out = append(out, a.propertyTypes(member, name, depth+1)...)
		}
		return out
	}
	if t.Flags()&checker.TypeFlagsTypeParameter != 0 {
		if constraint := a.checker.GetBaseConstraintOfType(t); constraint != nil && constraint != t {
			return a.propertyTypes(constraint, name, depth+1)
		}
		return nil
	}
	if p := a.checker.GetTypeOfPropertyOfType(t, name); p != nil {
		return []*checker.Type{p}
	}
	if s := a.checker.GetIndexTypeOfType(t, a.checker.GetStringType()); s != nil {
		return []*checker.Type{s}
	}
	return nil
}
