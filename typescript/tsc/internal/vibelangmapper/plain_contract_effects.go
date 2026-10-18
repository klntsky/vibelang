package vibelangmapper

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// checkPlainContractEffects enforces D263 independently of throw: a plain
// contract keeps JavaScript exceptions but cannot run a named Effect operation.
// Nested functions own their contracts; merely declaring one runs nothing.
func (p *projector) checkPlainContractEffects(fn *ast.Node) {
	if fn.Body() == nil {
		return
	}
	var visit func(*ast.Node) bool
	visit = func(node *ast.Node) bool {
		if ast.IsFunctionLike(node) || ast.IsClassLike(node) {
			return false
		}
		if node.Kind == ast.KindCallExpression && p.plainContractEffectCall(node) {
			start := p.tokenStart(node)
			p.diagnostics = append(p.diagnostics, Diagnostic{
				Start: start, Length: node.End() - start, Code: CodePlainContractEffect,
				Message: "A function with a plain return contract cannot run an Effect. Write an Effect return contract instead.",
			})
			return true
		}
		return node.ForEachChild(visit)
	}
	visit(fn.Body())
}

// plainContractEffectCall reuses purity's named-call classification (D54),
// excluding shipped JavaScript failures and known synchronous module functions.
// An unknown foreign call supplies no evidence that a plain contract runs Effects.
func (p *projector) plainContractEffectCall(call *ast.Node) bool {
	if p.builtinFailure(call) != "" || p.callObstacle(call, func(string) (obstacle, bool) { return obstacleUnknown, false }) != obstacleBlocking {
		return false
	}
	if r, ok := p.forms.(*Resolver); ok && r.Callee(call).Kind == CalleeTable {
		return r.returnsEffect(call)
	}
	return true
}
