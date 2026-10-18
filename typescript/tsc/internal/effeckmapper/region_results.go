package effeckmapper

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
)

// normalizeRegionResults preserves the source's normal-completion type.
// A handle clause that falls through returns undefined, rather than a
// generator's inferred void. A syntactically non-completing try with no
// outgoing jumps returns its never-valued region, so an enclosing generator
// cannot invent a void success path after it.
func (p *projector) normalizeRegionResults() {
	var visit func(*ast.Node)
	visit = func(n *ast.Node) {
		if n.Kind == ast.KindTryStatement {
			if groups, ok := handleClauses(n); ok && n.Flags&ast.NodeFlagsEffeck != 0 {
				for _, group := range groups {
					for _, clause := range group.clauses {
						if !canCompleteNormally(clause.body) {
							continue
						}
						end := clause.block.End() - 1
						recorded := false
						for _, edit := range p.plan.edits {
							if edit.pos == end && strings.Contains(edit.text, "return "+helperRef("$end")) {
								recorded = true
							}
						}
						if !recorded {
							p.plan.witness(end, "; return undefined; ")
						}
					}
				}
			} else if !completes(n) {
				for i, edit := range p.plan.edits {
					if edit.kind == editRegion && edit.part == partOpen && edit.pos == p.tokenStart(n) && strings.HasPrefix(edit.text, "yield* ") {
						p.plan.edits[i].text = "return " + edit.text
					}
				}
			}
		}
		n.ForEachChild(func(child *ast.Node) bool { visit(child); return false })
	}
	visit(p.file.AsNode())
}
