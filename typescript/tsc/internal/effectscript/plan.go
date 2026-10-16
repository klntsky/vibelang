package effectscript

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
)

// plan records what the lowering does to one .es file.
type plan struct {
	effects map[*ast.Node]bool // functions lowered to Effect.fnUntraced generators
}

func (p *plan) isEffect(fn *ast.Node) bool { return p.effects[fn] }

// planFile marks the functions in file that become Effects. Without types it
// cannot tell which calls run an Effect, so it marks every function whose own
// body makes a call. A function that makes no call stays plain JavaScript.
func planFile(file *ast.SourceFile) *plan {
	p := &plan{effects: map[*ast.Node]bool{}}
	anyCall := func(*ast.Node) bool { return true }
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if isLowerable(node) && forEachCallInBody(node, anyCall) {
			p.effects[node] = true
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
	return p
}

// forEachCallInBody calls cb for each call that fn's own body evaluates,
// skipping nested functions and classes, until cb returns true. It reports
// whether cb returned true.
func forEachCallInBody(fn *ast.Node, cb func(call *ast.Node) bool) bool {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if isFunctionBoundary(node) {
			return false
		}
		if node.Kind == ast.KindCallExpression && cb(node) {
			return true
		}
		return node.ForEachChild(visit)
	}
	return visit(fn.Body())
}
