package effeckmapper

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/locale"
)

// checkDoControlFlow preserves the parser's return rejection and the checker's
// function boundary for jumps, so neither rendering gives rejected do code meaning.
func (p *projector) checkDoControlFlow() {
	for _, d := range p.file.Diagnostics() {
		if d.Code() == 1108 {
			p.diagnostics = append(p.diagnostics, Diagnostic{Start: d.Pos(), Length: d.End() - d.Pos(), Code: d.Code(), Message: d.Localize(locale.Default)})
		}
	}
	var visit func(*ast.Node) bool
	visit = func(n *ast.Node) bool {
		if n.Kind == ast.KindBreakStatement || n.Kind == ast.KindContinueStatement {
			for parent := n.Parent; parent != nil; parent = parent.Parent {
				if ast.IsFunctionLike(parent) {
					start := p.tokenStart(parent)
					if parent.Kind == ast.KindArrowFunction {
						start = parent.AsArrowFunction().EqualsGreaterThanToken.Pos()
					}
					if strings.HasPrefix(p.text[start:], "do") {
						p.diagnostics = append(p.diagnostics, Diagnostic{Start: p.tokenStart(n), Length: n.End() - p.tokenStart(n), Code: 1107, Message: "Jump target cannot cross function boundary."})
					}
					break
				}
				if n.Label() != nil {
					if parent.Kind == ast.KindLabeledStatement && parent.Label().Text() == n.Label().Text() {
						break
					}
				} else if ast.IsIterationStatement(parent, false) || (n.Kind == ast.KindBreakStatement && parent.Kind == ast.KindSwitchStatement) {
					break
				}
			}
		}
		n.ForEachChild(visit)
		return false
	}
	p.file.AsNode().ForEachChild(visit)
}
