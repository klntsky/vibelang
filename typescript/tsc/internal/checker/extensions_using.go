package checker

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// UsingInitializerHook reports whether the initializer of declaration, a
// variable declaration in a `using` list, acquires a resource that a language
// extension releases itself. The checker then skips its check that the
// initializer is a Disposable, null, or undefined, and does not need the
// global Disposable type for it. The declared variable keeps the initializer's
// type.
type UsingInitializerHook func(c *Checker, declaration *ast.Node) bool

var usingInitializerHooks []UsingInitializerHook

// RegisterUsingInitializerHook adds hook to every checker. Call it only from
// the init function of an in-tree package that tsc/cmd/tsc imports;
// registration is not synchronized.
func RegisterUsingInitializerHook(hook UsingInitializerHook) {
	usingInitializerHooks = append(usingInitializerHooks, hook)
}

func (c *Checker) applyUsingInitializerHooks(declaration *ast.Node) bool {
	for _, hook := range usingInitializerHooks {
		if hook(c, declaration) {
			return true
		}
	}
	return false
}
