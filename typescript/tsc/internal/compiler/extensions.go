package compiler

import (
	"slices"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/printer"
	"github.com/microsoft/TypeScript/tsc/internal/transformers"
)

// ScriptTransformerHook returns a transformer for sourceFile, or nil to leave
// the file alone. Hooks run first in the script transform pipeline, before
// TypeScript syntax is erased, so the transformer sees the parse tree and can
// ask opts.EmitResolver about types.
type ScriptTransformerHook func(opts *transformers.TransformOptions, sourceFile *ast.SourceFile) *transformers.Transformer

var scriptTransformerHooks []ScriptTransformerHook

// RegisterScriptTransformerHook adds hook to every JavaScript emit. Call it
// only from the init function of an in-tree package that tsc/cmd/tsc imports;
// registration is not synchronized.
func RegisterScriptTransformerHook(hook ScriptTransformerHook) {
	scriptTransformerHooks = append(scriptTransformerHooks, hook)
}

func getHookTransformers(opts *transformers.TransformOptions, sourceFile *ast.SourceFile) []*transformers.Transformer {
	var tx []*transformers.Transformer
	for _, hook := range scriptTransformerHooks {
		if t := hook(opts, sourceFile); t != nil {
			tx = append(tx, t)
		}
	}
	return tx
}

var directiveExemptSources []string

// RegisterDirectiveExemptSource exempts diagnostics whose Source is source
// from @ts-ignore and @ts-expect-error comments, which then neither hide them
// nor count as used. Call it only from the init function of an in-tree
// package that tsc/cmd/tsc imports; registration is not synchronized.
func RegisterDirectiveExemptSource(source string) {
	directiveExemptSources = append(directiveExemptSources, source)
}

func isDirectiveExempt(diagnostic *ast.Diagnostic) bool {
	return diagnostic.Source() != "" && slices.Contains(directiveExemptSources, diagnostic.Source())
}

// ImplicitImportHook returns modules that file uses without importing them.
// The program resolves each one from file, as it resolves tslib for
// importHelpers, and includes the module's file.
type ImplicitImportHook func(file *ast.SourceFile) []string

var implicitImportHooks []ImplicitImportHook

// RegisterImplicitImportHook adds hook to every program. Call it only from the
// init function of an in-tree package that tsc/cmd/tsc imports; registration
// is not synchronized.
func RegisterImplicitImportHook(hook ImplicitImportHook) {
	implicitImportHooks = append(implicitImportHooks, hook)
}

func implicitImports(file *ast.SourceFile) []string {
	var names []string
	for _, hook := range implicitImportHooks {
		names = append(names, hook(file)...)
	}
	return names
}

// DeclarationTransformerHook returns a transform for the declaration file
// emitted for sourceFile, which runs after TypeScript's own declaration
// transforms, or nil to leave the declaration file alone.
type DeclarationTransformerHook func(resolver printer.EmitResolver, sourceFile *ast.SourceFile) func(declarations *ast.SourceFile) *ast.SourceFile

var declarationTransformerHooks []DeclarationTransformerHook

// RegisterDeclarationTransformerHook adds hook to every declaration emit.
// Call it only from the init function of an in-tree package that tsc/cmd/tsc
// imports; registration is not synchronized.
func RegisterDeclarationTransformerHook(hook DeclarationTransformerHook) {
	declarationTransformerHooks = append(declarationTransformerHooks, hook)
}

func getHookDeclarationTransformers(resolver printer.EmitResolver, sourceFile *ast.SourceFile) []declarationTransformer {
	var tx []declarationTransformer
	for _, hook := range declarationTransformerHooks {
		if transform := hook(resolver, sourceFile); transform != nil {
			tx = append(tx, hookDeclarationTransformer(transform))
		}
	}
	return tx
}

// hookDeclarationTransformer adapts a hook's transform to the emitter.
type hookDeclarationTransformer func(declarations *ast.SourceFile) *ast.SourceFile

func (t hookDeclarationTransformer) TransformSourceFile(declarations *ast.SourceFile) *ast.SourceFile {
	return t(declarations)
}

func (t hookDeclarationTransformer) GetDiagnostics() []*ast.Diagnostic { return nil }
