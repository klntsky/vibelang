package compiler

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
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
