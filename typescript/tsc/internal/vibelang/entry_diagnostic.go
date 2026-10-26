package vibelang

import (
	"slices"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

var EK1016 = message(1016, warningCategory, "main is never referenced and is not exported; write export function main() to run it when this module starts.")

func init() {
	registry[EK1016.Code] = EK1016
	checker.RegisterDiagnosticsFilterHook(warnUnusedMain)
}

// The compiler has no unique entry filename. Diagnose an otherwise valid
// module's unused, zero-argument main, the shape D135 would start if exported.
// Symbol identity keeps shadowed names from hiding the missing export.
func warnUnusedMain(c *checker.Checker, file *ast.SourceFile, all []*ast.Diagnostic) []*ast.Diagnostic {
	if !isVibeLangFile(file) || slices.ContainsFunc(all, func(d *ast.Diagnostic) bool { return d.Category() == errorCategory }) {
		return all
	}
	for _, declaration := range file.Statements.Nodes {
		if !ast.IsFunctionDeclaration(declaration) || declaration.Name() == nil || declaration.Name().Text() != "main" ||
			declaration.Body() == nil || len(declaration.Parameters()) != 0 || ast.HasSyntacticModifier(declaration, ast.ModifierFlagsExport) {
			continue
		}
		symbol := c.GetSymbolAtLocation(declaration.Name())
		if symbol == nil {
			continue
		}
		var referenced func(*ast.Node) bool
		referenced = func(n *ast.Node) bool {
			if n != declaration.Name() && ast.IsIdentifier(n) && n.Text() == "main" {
				target := c.GetSymbolAtLocation(n)
				if target != nil && target.Flags()&ast.SymbolFlagsAlias != 0 {
					target = c.GetAliasedSymbol(target)
				}
				if target == symbol {
					return true
				}
			}
			return n.ForEachChild(referenced)
		}
		if !file.AsNode().ForEachChild(referenced) {
			if d := newDiagnostic(optionsFor(c, file), declaration.Name(), EK1016); d != nil {
				all = append(all, d)
			}
		}
	}
	return all
}
