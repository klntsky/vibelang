package checker

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
)

// WithChecker calls fn with the checker behind r while holding the checker's
// lock, the way the resolver's own methods do. In-tree language extensions use
// it to ask type questions while they transform a file for emit.
func (r *EmitResolver) WithChecker(fn func(c *Checker)) {
	r.checkerMu.Lock()
	defer r.checkerMu.Unlock()
	fn(r.checker)
}

// CallExpressionTypeHook can replace t, the type the checker gives call. It
// runs on every call the checker types, so it must be cheap, and it returns t
// when it has nothing to say.
type CallExpressionTypeHook func(c *Checker, call *ast.Node, t *Type) *Type

var callExpressionTypeHooks []CallExpressionTypeHook

// RegisterCallExpressionTypeHook adds hook to every checker. Call it only from
// the init function of an in-tree package that tsc/cmd/tsc imports;
// registration is not synchronized.
func RegisterCallExpressionTypeHook(hook CallExpressionTypeHook) {
	callExpressionTypeHooks = append(callExpressionTypeHooks, hook)
}

func (c *Checker) applyCallExpressionTypeHooks(call *ast.Node, t *Type) *Type {
	for _, hook := range callExpressionTypeHooks {
		if t == nil {
			break
		}
		t = hook(c, call, t)
	}
	return t
}

// InferredReturnTypeHook can replace t, the return type the checker inferred
// from the body of fn, a function without a return type annotation.
type InferredReturnTypeHook func(c *Checker, fn *ast.Node, t *Type) *Type

var inferredReturnTypeHooks []InferredReturnTypeHook

// RegisterInferredReturnTypeHook adds hook to every checker. Call it only from
// the init function of an in-tree package that tsc/cmd/tsc imports;
// registration is not synchronized.
func RegisterInferredReturnTypeHook(hook InferredReturnTypeHook) {
	inferredReturnTypeHooks = append(inferredReturnTypeHooks, hook)
}

func (c *Checker) applyInferredReturnTypeHooks(fn *ast.Node, t *Type) *Type {
	for _, hook := range inferredReturnTypeHooks {
		t = hook(c, fn, t)
	}
	return t
}

// ExtensionState returns the value an in-tree language extension keeps for c
// under key, calling create to make it on first use. The value lives exactly
// as long as c, so a language server that replaces its checkers on every edit
// does not accumulate it. A checker is used by one goroutine at a time, so
// neither the state map nor the value needs a lock.
func (c *Checker) ExtensionState(key any, create func() any) any {
	if value, ok := c.extensionState[key]; ok {
		return value
	}
	if c.extensionState == nil {
		c.extensionState = map[any]any{}
	}
	value := create()
	c.extensionState[key] = value
	return value
}

// NewTypeReference instantiates target, a generic class or interface type,
// with typeArguments.
func (c *Checker) NewTypeReference(target *Type, typeArguments []*Type) *Type {
	return c.createTypeReference(target, typeArguments)
}

// CompilerOptions returns the options of the program that c checks.
func (c *Checker) CompilerOptions() *core.CompilerOptions {
	return c.compilerOptions
}

// AddDiagnostic records d with c's diagnostics. d must belong to a node of
// the file that c is checking: the program reads each file's diagnostics from
// the checker that checks that file.
func (c *Checker) AddDiagnostic(d *ast.Diagnostic) *ast.Diagnostic {
	return c.addDiagnostic(d)
}

// SourceFileCheckHook runs after c has checked file, once for each checker
// and file, so it can report diagnostics for file with AddDiagnostic.
type SourceFileCheckHook func(c *Checker, file *ast.SourceFile)

var sourceFileCheckHooks []SourceFileCheckHook

// RegisterSourceFileCheckHook adds hook to every checker. Call it only from
// the init function of an in-tree package that tsc/cmd/tsc imports;
// registration is not synchronized.
func RegisterSourceFileCheckHook(hook SourceFileCheckHook) {
	sourceFileCheckHooks = append(sourceFileCheckHooks, hook)
}

func (c *Checker) applySourceFileCheckHooks(file *ast.SourceFile) {
	for _, hook := range sourceFileCheckHooks {
		if c.isCanceled() {
			return
		}
		hook(c, file)
	}
}

// DiagnosticsFilterHook can drop diagnostics from what c reports for file. It
// returns diagnostics itself when it drops nothing, and a new slice otherwise.
type DiagnosticsFilterHook func(c *Checker, file *ast.SourceFile, diagnostics []*ast.Diagnostic) []*ast.Diagnostic

var diagnosticsFilterHooks []DiagnosticsFilterHook

// RegisterDiagnosticsFilterHook adds hook to every checker. Call it only from
// the init function of an in-tree package that tsc/cmd/tsc imports;
// registration is not synchronized.
func RegisterDiagnosticsFilterHook(hook DiagnosticsFilterHook) {
	diagnosticsFilterHooks = append(diagnosticsFilterHooks, hook)
}

func (c *Checker) applyDiagnosticsFilterHooks(file *ast.SourceFile, diagnostics []*ast.Diagnostic) []*ast.Diagnostic {
	for _, hook := range diagnosticsFilterHooks {
		diagnostics = hook(c, file, diagnostics)
	}
	return diagnostics
}
