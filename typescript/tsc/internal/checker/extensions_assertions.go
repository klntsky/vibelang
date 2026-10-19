package checker

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// AssertionOperandTypeHook replaces an assertion operand's type for the deferred
// compatibility check, after surrounding calls have resolved their arguments.
// It does not change the assertion's result type or the operand's value type.
type AssertionOperandTypeHook func(c *Checker, node *ast.Node, t *Type) *Type

var assertionOperandTypeHooks []AssertionOperandTypeHook

// RegisterAssertionOperandTypeHook adds a hook to every checker. Call it only
// from an in-tree package's init function; registration is not synchronized.
func RegisterAssertionOperandTypeHook(hook AssertionOperandTypeHook) {
	assertionOperandTypeHooks = append(assertionOperandTypeHooks, hook)
}

func (c *Checker) applyAssertionOperandTypeHooks(node *ast.Node, t *Type) *Type {
	for _, hook := range assertionOperandTypeHooks {
		t = hook(c, node, t)
	}
	return t
}
