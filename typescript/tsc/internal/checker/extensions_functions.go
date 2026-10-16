package checker

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// SkippedFunctionTypeHook can replace the type that fn, a context-sensitive
// function expression, arrow function, or object literal method, has while
// overload resolution skips it in its first inference pass. TypeScript uses
// anyFunctionType there, a wildcard for function types only; a hook can
// return another placeholder, such as GetNonInferrableAnyType, where a
// function is passed for a type that is not a function. It returns nil when
// it has nothing to say.
type SkippedFunctionTypeHook func(c *Checker, fn *ast.Node) *Type

var skippedFunctionTypeHooks []SkippedFunctionTypeHook

// RegisterSkippedFunctionTypeHook adds hook to every checker. Call it only
// from the init function of an in-tree package that tsc/cmd/tsc imports;
// registration is not synchronized.
func RegisterSkippedFunctionTypeHook(hook SkippedFunctionTypeHook) {
	skippedFunctionTypeHooks = append(skippedFunctionTypeHooks, hook)
}

func (c *Checker) skippedFunctionType(fn *ast.Node) *Type {
	for _, hook := range skippedFunctionTypeHooks {
		if t := hook(c, fn); t != nil {
			return t
		}
	}
	return c.anyFunctionType
}

// GetNonInferrableAnyType returns the any type that type inference ignores.
func (c *Checker) GetNonInferrableAnyType() *Type {
	return c.nonInferrableAnyType
}
