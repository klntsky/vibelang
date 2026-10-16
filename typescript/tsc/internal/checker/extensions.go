package checker

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// WithChecker calls fn with the checker behind r while holding the checker's
// lock, the way the resolver's own methods do. In-tree language extensions use
// it to ask type questions while they transform a file for emit.
func (r *EmitResolver) WithChecker(fn func(c *Checker)) {
	r.checkerMu.Lock()
	defer r.checkerMu.Unlock()
	fn(r.checker)
}

// CallExpressionTypeHook can replace t, the type the checker gives call. It
// runs on every call the checker types, so it must be cheap, and it returns t
// when it has nothing to say.
type CallExpressionTypeHook func(c *Checker, call *ast.Node, t *Type) *Type

var callExpressionTypeHooks []CallExpressionTypeHook

// RegisterCallExpressionTypeHook adds hook to every checker. Call it only from
// the init function of an in-tree package that tsc/cmd/tsc imports;
// registration is not synchronized.
func RegisterCallExpressionTypeHook(hook CallExpressionTypeHook) {
	callExpressionTypeHooks = append(callExpressionTypeHooks, hook)
}

func (c *Checker) applyCallExpressionTypeHooks(call *ast.Node, t *Type) *Type {
	for _, hook := range callExpressionTypeHooks {
		if t == nil {
			break
		}
		t = hook(c, call, t)
	}
	return t
}
