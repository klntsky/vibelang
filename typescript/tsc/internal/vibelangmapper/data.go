package vibelangmapper

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// The mapper does not lower a data declaration (D307) yet. Its type alias
// draws one EK9001 that says so; the constant and the interface the parser
// writes beside it, which have no source of their own, draw nothing.

// isDataDefinition reports whether node is the interface a generic data
// declaration writes for Data.taggedEnum, NameDefinition.
func isDataDefinition(node *ast.Node) bool {
	if node.Kind != ast.KindInterfaceDeclaration || node.Flags&ast.NodeFlagsReparsed == 0 {
		return false
	}
	clauses := node.AsInterfaceDeclaration().HeritageClauses
	if clauses == nil || len(clauses.Nodes) != 1 {
		return false
	}
	types := clauses.Nodes[0].AsHeritageClause().Types.Nodes
	return len(types) == 1 && types[0].Expression().Kind == ast.KindPropertyAccessExpression && types[0].Expression().Name().Text() == "WithGenerics"
}

// dataDeclaration reports node, a data declaration's type alias, as a form
// the mapper does not lower.
func (p *projector) dataDeclaration(node *ast.Node) {
	if !parser.IsDataDeclaration(node) || p.reported[p.statement] {
		return
	}
	p.reported[p.statement] = true
	start := scanner.SkipTrivia(p.text, node.Pos())
	p.diagnostics = append(p.diagnostics, Diagnostic{
		Start:   start,
		Length:  node.End() - start,
		Code:    CodeUnprojectedForm,
		Message: "The TypeScript 7 mapper does not lower a data declaration (D307) yet; compile this file with vibelang.",
	})
}
