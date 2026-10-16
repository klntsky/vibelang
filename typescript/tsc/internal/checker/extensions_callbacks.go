package checker

import (
	"slices"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
)

// TypeParametersHook can give declaration, a function-like declaration, type
// parameters beyond the ones its syntax declares. They join the type
// parameters of the signature declaration declares, and the outer type
// parameters of anything declared inside it, so a type that mentions them
// is instantiated with the signature. It returns nil when it has nothing to
// add, and the same type parameters each time it is asked.
type TypeParametersHook func(c *Checker, declaration *ast.Node) []*Type

var typeParametersHooks []TypeParametersHook

// RegisterTypeParametersHook adds hook to every checker. Call it only from the
// init function of an in-tree package that tsc/cmd/tsc imports; registration
// is not synchronized.
func RegisterTypeParametersHook(hook TypeParametersHook) {
	typeParametersHooks = append(typeParametersHooks, hook)
}

// appendTypeParametersFromHooks returns typeParameters followed by the type
// parameters that hooks add to declaration.
func (c *Checker) appendTypeParametersFromHooks(typeParameters []*Type, declaration *ast.Node) []*Type {
	for _, hook := range typeParametersHooks {
		if extra := hook(c, declaration); len(extra) != 0 {
			typeParameters = append(slices.Clip(typeParameters), extra...)
		}
	}
	return typeParameters
}

// ParameterTypeHook can replace t, the type that the annotation of parameter
// declares, before optionality is added.
type ParameterTypeHook func(c *Checker, parameter *ast.Node, t *Type) *Type

var parameterTypeHooks []ParameterTypeHook

// RegisterParameterTypeHook adds hook to every checker. Call it only from the
// init function of an in-tree package that tsc/cmd/tsc imports; registration
// is not synchronized.
func RegisterParameterTypeHook(hook ParameterTypeHook) {
	parameterTypeHooks = append(parameterTypeHooks, hook)
}

func (c *Checker) applyParameterTypeHooks(parameter *ast.Node, t *Type) *Type {
	for _, hook := range parameterTypeHooks {
		t = hook(c, parameter, t)
	}
	return t
}

// NewTypeParameter returns a new type parameter named name, without a
// constraint or a declaration.
func (c *Checker) NewTypeParameter(name string) *Type {
	return c.getDeclaredTypeOfTypeParameter(c.newSymbol(ast.SymbolFlagsTypeParameter, name))
}
