package checker

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// IteratedElementTypeHook can give the type of the elements that node
// iterates, where node is a for...of statement or a yield* expression and
// iterated is the type of the value it iterates. An in-tree language
// extension uses it for a value that JavaScript cannot iterate but the
// extension can, such as an Effect Stream. It returns nil when it has nothing
// to say, and TypeScript then iterates iterated as usual.
type IteratedElementTypeHook func(c *Checker, node *ast.Node, iterated *Type) *Type

var iteratedElementTypeHooks []IteratedElementTypeHook

// RegisterIteratedElementTypeHook adds hook to every checker. Call it only
// from the init function of an in-tree package that tsc/cmd/tsc imports;
// registration is not synchronized.
func RegisterIteratedElementTypeHook(hook IteratedElementTypeHook) {
	iteratedElementTypeHooks = append(iteratedElementTypeHooks, hook)
}

func (c *Checker) iteratedElementTypeFromHooks(node *ast.Node, iterated *Type) *Type {
	for _, hook := range iteratedElementTypeHooks {
		if t := hook(c, node, iterated); t != nil {
			return t
		}
	}
	return nil
}
