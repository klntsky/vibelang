package effeckmapper

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// scopeStatement keeps a region a single statement when its parent is a
// loop, label, or conditional. The source braces belong to its generator.
func (p *projector) scopeStatement(node *ast.Node, e env) {
	embedded := !isStatementListItem(node)
	if embedded {
		p.plan.insert(p.tokenStart(node), "{ ")
	}
	p.scopeBlock(node, e)
	if embedded {
		p.plan.insert(node.End(), " }")
	}
}
