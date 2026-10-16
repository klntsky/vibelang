// Package effectscript compiles EffectScript (.es) files.
//
// EffectScript is TypeScript syntax in which every function is an Effect: a
// call to a function that returns an Effect runs it, and the compiler infers
// each function's Effect<A, E, R>. The package plugs into upstream tsc only
// through Register* hooks, which it calls from init; tsc/cmd/tsc imports it
// for that side effect. See EFFECTSCRIPT.md at the repository root.
package effectscript

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/compiler"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

func init() {
	compiler.RegisterScriptTransformerHook(newLowering)
	compiler.RegisterDirectiveExemptSource(diagnosticSource)
	checker.RegisterCallExpressionTypeHook(callExpressionType)
	checker.RegisterInferredReturnTypeHook(inferredReturnType)
	checker.RegisterCallCandidateHook(callCandidate)
	checker.RegisterCandidateFilterHook(filterCandidate)
	checker.RegisterFunctionExpressionTypeHook(functionExpressionType)
	checker.RegisterContextualReturnTypeHook(contextualReturnType)
	checker.RegisterSourceFileCheckHook(checkSourceFile)
	checker.RegisterDiagnosticsFilterHook(filterCoveredDiagnostics)
}

// isEffectScriptFile reports whether file is an EffectScript source file.
func isEffectScriptFile(file *ast.SourceFile) bool {
	return file != nil && !file.IsDeclarationFile && file.FileName().ExtensionIs(tspath.ExtensionEs)
}

// isLowerable reports whether node is a function that the lowering can turn
// into an Effect: a function declaration, function expression, or arrow
// function with a body, declared in an .es file, that is neither async nor a
// generator. Methods, accessors, and constructors stay as they are for now.
func isLowerable(node *ast.Node) bool {
	switch node.Kind {
	case ast.KindFunctionDeclaration, ast.KindFunctionExpression, ast.KindArrowFunction:
	default:
		return false
	}
	if node.Body() == nil || ast.GetFunctionFlags(node)&(ast.FunctionFlagsAsync|ast.FunctionFlagsGenerator) != 0 {
		return false
	}
	return isEffectScriptFile(ast.GetSourceFileOfNode(node))
}

// isFunctionBoundary reports whether node starts code that runs in a different
// function from its parent, so a call inside it cannot be run with the
// parent's yield*.
func isFunctionBoundary(node *ast.Node) bool {
	return ast.IsFunctionLike(node) || ast.IsClassLike(node) || node.Kind == ast.KindClassStaticBlockDeclaration
}
