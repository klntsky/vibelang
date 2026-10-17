// Package effeck compiles Effeck (.effeck) files.
//
// Effeck is TypeScript syntax in which every function is an Effect: a
// call to a function that returns an Effect runs it, and the compiler infers
// each function's Effect<A, E, R>. The package plugs into upstream tsc only
// through Register* hooks, which it calls from init; tsc/cmd/tsc imports it
// for that side effect. See EFFECK.md at the repository root.
package effeck

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/compiler"
	"github.com/microsoft/TypeScript/tsc/internal/transformers/declarations"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

func init() {
	compiler.RegisterScriptTransformerHook(newLowering)
	compiler.RegisterDirectiveExemptSource(diagnosticSource)
	compiler.RegisterImplicitImportHook(implicitRuntimeImport)
	compiler.RegisterDeclarationTransformerHook(declareCallbackContracts) // before transformDeclarations, which imports what it writes
	compiler.RegisterDeclarationTransformerHook(transformDeclarations)
	checker.RegisterCallExpressionTypeHook(callExpressionType)
	checker.RegisterInferredReturnTypeHook(inferredReturnType)
	checker.RegisterCallCandidateHook(callCandidate)
	checker.RegisterCandidateFilterHook(filterCandidate)
	checker.RegisterFunctionExpressionTypeHook(functionExpressionType)
	checker.RegisterContextualReturnTypeHook(contextualReturnType)
	checker.RegisterAnnotatedReturnInferenceHook(annotatedReturnInference)
	checker.RegisterSourceFileCheckHook(checkSourceFile)
	checker.RegisterDiagnosticsFilterHook(filterCoveredDiagnostics)
	checker.RegisterClassBaseHook(classBases)
	checker.RegisterMemberReceiverHook(memberReceiver)
	checker.RegisterCallSignaturesHook(effectValueCall)
	checker.RegisterExpressionTypeHook(expressionType)
	checker.RegisterCatchVariableTypeHook(catchVariableType)
	checker.RegisterUnwrappedReturnTypeHook(unwrappedReturnType)
	checker.RegisterAnnotatedReturnTypeHook(annotatedReturnType)
	checker.RegisterInferTypeHook(inferChannel)
	declarations.RegisterTypeAnnotationHook(hasInferredChannel)
	checker.RegisterNameResolutionHook(resolvePreludeName)
	checker.RegisterEntityNameVisibilityHook(preludeNameVisible)
	checker.RegisterTypeParametersHook(extraTypeParameters)
	checker.RegisterParameterTypeHook(callbackParameterType)
	checker.RegisterIteratedElementTypeHook(iteratedElementType)
}

// isEffeckFile reports whether file is an Effeck source file.
func isEffeckFile(file *ast.SourceFile) bool {
	return file != nil && !file.IsDeclarationFile && file.FileName().ExtensionIs(tspath.ExtensionEffeck)
}

// isLowerable reports whether node is a function that the lowering can turn
// into an Effect: a function declaration, function expression, arrow
// function, or method with a body, declared in an .effeck file, that is not
// async, and is not a generator unless it is a producer (producers.go), whose
// body is an Effect body too. Accessors and constructors stay as they are.
func isLowerable(node *ast.Node) bool {
	switch node.Kind {
	case ast.KindFunctionDeclaration, ast.KindFunctionExpression, ast.KindArrowFunction, ast.KindMethodDeclaration:
	default:
		return false
	}
	flags := ast.GetFunctionFlags(node)
	if node.Body() == nil || flags&ast.FunctionFlagsAsync != 0 || flags&ast.FunctionFlagsGenerator != 0 && !isProducer(node) {
		return false
	}
	return isEffeckFile(ast.GetSourceFileOfNode(node))
}

// isFunctionBoundary reports whether node starts code that runs in a different
// function from its parent, so a call inside it cannot be run with the
// parent's yield*.
func isFunctionBoundary(node *ast.Node) bool {
	return ast.IsFunctionLike(node) || ast.IsClassLike(node) || node.Kind == ast.KindClassStaticBlockDeclaration
}
