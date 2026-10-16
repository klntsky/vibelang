package effectscript

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/outputpaths"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

// Imports between files (design decision D31). A .es file imports a module by
// its source name, ./welcome.es or ./format.ts, and the emitted JavaScript
// imports the output, ./welcome.js and ./format.js, whether or not the
// project sets rewriteRelativeImportExtensions. TypeScript code imports a .es
// module as ./welcome.js, which the module resolver resolves to welcome.es as
// it resolves ./format.js to format.ts.

// visitImportOrExport rewrites the module specifier of an import or export
// declaration in a .es file.
func (tx *lowering) visitImportOrExport(node *ast.Node) *ast.Node {
	f := tx.Factory()
	switch node.Kind {
	case ast.KindImportDeclaration:
		d := node.AsImportDeclaration()
		return f.UpdateImportDeclaration(d, d.Modifiers(), tx.Visitor().VisitNode(d.ImportClause), tx.outputSpecifier(d.ModuleSpecifier), tx.Visitor().VisitNode(d.Attributes))
	case ast.KindExportDeclaration:
		d := node.AsExportDeclaration()
		return f.UpdateExportDeclaration(d, d.Modifiers(), d.IsTypeOnly, tx.Visitor().VisitNode(d.ExportClause), tx.outputSpecifier(d.ModuleSpecifier), tx.Visitor().VisitNode(d.Attributes))
	}
	return tx.Visitor().VisitEachChild(node)
}

// outputSpecifier returns the specifier the emitted JavaScript uses for
// specifier: a relative specifier that names a TypeScript or EffectScript
// source names its output instead.
func (tx *lowering) outputSpecifier(specifier *ast.Node) *ast.Node {
	if specifier == nil || !ast.IsStringLiteral(specifier) {
		return specifier
	}
	text, changed := outputSpecifierText(specifier.Text())
	if !changed {
		return specifier
	}
	output := tx.Factory().NewStringLiteral(text, specifier.AsStringLiteral().TokenFlags)
	tx.EmitContext().SetOriginal(output, specifier)
	tx.EmitContext().AssignCommentAndSourceMapRanges(output, specifier)
	return output
}

// outputSpecifierText returns the specifier that output uses for text, and
// whether it differs from text: a relative specifier that names a TypeScript
// or EffectScript source, ./users.es, names its output, ./users.js.
func outputSpecifierText(text string) (string, bool) {
	if !tspath.ToModuleSpecifier(text).ShouldRewriteExtension(nil /*rewrites*/, tspath.CaseSensitive) {
		return text, false
	}
	return tspath.ChangeExtension(text, outputpaths.GetOutputExtension(text, core.JsxEmitNone)), true
}
