package checker

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// FlowNarrowingHook can narrow a reference using a language-specific test.
// Return nil to use normal narrowing. narrowAlias applies the same test and
// assumption using an equivalent reference and its type, without changing flow state.
type FlowNarrowingHook func(c *Checker, reference *ast.Node, t *Type, test *ast.Node, assumeTrue bool, narrowAlias func(*ast.Node, *Type) *Type) *Type

var flowNarrowingHooks []FlowNarrowingHook

// RegisterFlowNarrowingHook registers an in-tree extension at init time.
func RegisterFlowNarrowingHook(hook FlowNarrowingHook) {
	flowNarrowingHooks = append(flowNarrowingHooks, hook)
}

// IsMatchingReference reports whether two expressions refer to the same value.
func (c *Checker) IsMatchingReference(a, b *ast.Node) bool {
	return c.isMatchingReference(a, b)
}

// NarrowTypeTo applies a positive or negative type test to t.
func (c *Checker) NarrowTypeTo(t, candidate *Type, assumeTrue bool) *Type {
	return c.getNarrowedType(t, candidate, assumeTrue, false)
}
