package checker

import (
	"maps"
	"slices"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
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

// CatchVariableTypeHook gives the variable of a catch clause that has no type
// annotation its type, or returns nil to leave TypeScript's any or unknown.
// declaration is the catch clause's variable declaration.
type CatchVariableTypeHook func(c *Checker, declaration *ast.Node) *Type

var catchVariableTypeHooks []CatchVariableTypeHook

// RegisterCatchVariableTypeHook adds hook to every checker. Call it only from
// the init function of an in-tree package that tsc/cmd/tsc imports;
// registration is not synchronized.
func RegisterCatchVariableTypeHook(hook CatchVariableTypeHook) {
	catchVariableTypeHooks = append(catchVariableTypeHooks, hook)
}

func (c *Checker) applyCatchVariableTypeHooks(declaration *ast.Node) *Type {
	for _, hook := range catchVariableTypeHooks {
		if t := hook(c, declaration); t != nil {
			return t
		}
	}
	return nil
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

// AddDiagnostic records d in the matching semantic or suggestion collection,
// so editor suggestions do not become CLI semantic errors.
// Its node must belong to the file being checked: the program reads each
// file's diagnostics from the checker that checks that file.
func (c *Checker) AddDiagnostic(d *ast.Diagnostic) *ast.Diagnostic {
	if d.Category() == diagnostics.CategorySuggestion {
		return c.addSuggestionDiagnostic(d)
	}
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

// GetTypeOfExpressionBeforeWidening returns the type the checker gives the
// expression node, keeping literal freshness: TypeScript widens the type of
// "bad" in `return "bad"` to string in an inferred return type, but not the
// type of a variable declared as the literal type "bad". Language extensions
// that infer a type from expressions widen it the same way.
func (c *Checker) GetTypeOfExpressionBeforeWidening(node *ast.Node) *Type {
	return c.checkExpressionCached(node)
}

// NameResolutionHook can supply a symbol for name, which no scope enclosing
// location declares, before the checker searches the global scope. It returns
// nil when it has nothing to supply. It runs for every name that reaches the
// global scope, so it must be cheap.
type NameResolutionHook func(c *Checker, location *ast.Node, name string, meaning ast.SymbolFlags) *ast.Symbol

var nameResolutionHooks []NameResolutionHook

// RegisterNameResolutionHook adds hook to every checker. Call it only from the
// init function of an in-tree package that tsc/cmd/tsc imports; registration
// is not synchronized.
func RegisterNameResolutionHook(hook NameResolutionHook) {
	nameResolutionHooks = append(nameResolutionHooks, hook)
}

// SpellingCandidatesHook returns names that a "Did you mean" suggestion
// (TS2552) may offer at location for meaning, besides those in scope: names
// that a NameResolutionHook supplies, which no symbol table lists.
type SpellingCandidatesHook func(c *Checker, location *ast.Node, meaning ast.SymbolFlags) []*ast.Symbol

var spellingCandidatesHooks []SpellingCandidatesHook

// RegisterSpellingCandidatesHook adds hook to every checker. Call it only
// from the init function of an in-tree package that tsc/cmd/tsc imports;
// registration is not synchronized.
func RegisterSpellingCandidatesHook(hook SpellingCandidatesHook) {
	spellingCandidatesHooks = append(spellingCandidatesHooks, hook)
}

// suggestBeforeGlobals is the suggestion resolver's step before the global
// scope: the closest of the hooks' candidates and the globals, so that a
// supplied name competes with a global one by distance. It returns nil when
// no hook offers a candidate, and the globals are searched as usual.
func (c *Checker) suggestBeforeGlobals(location *ast.Node, name string, meaning ast.SymbolFlags) *ast.Symbol {
	var candidates []*ast.Symbol
	for _, hook := range spellingCandidatesHooks {
		candidates = append(candidates, hook(c, location, meaning)...)
	}
	if len(candidates) == 0 {
		return nil
	}
	return c.getSpellingSuggestionForName(name, core.ConcatenateSeq(slices.Values(candidates), maps.Values(c.globals)), meaning)
}

func (c *Checker) resolveBeforeGlobals(location *ast.Node, name string, meaning ast.SymbolFlags) *ast.Symbol {
	for _, hook := range nameResolutionHooks {
		if symbol := hook(c, location, name, meaning); symbol != nil {
			return symbol
		}
	}
	return nil
}

// EntityNameVisibilityHook reports whether a declaration file may write
// entityName, which resolves to symbol, although no declaration of the file
// declares or imports it. A hook that says yes must make the declaration file
// import the name, for example with a declaration transformer hook.
type EntityNameVisibilityHook func(c *Checker, entityName *ast.Node, symbol *ast.Symbol) bool

var entityNameVisibilityHooks []EntityNameVisibilityHook

// RegisterEntityNameVisibilityHook adds hook to every checker. Call it only
// from the init function of an in-tree package that tsc/cmd/tsc imports;
// registration is not synchronized.
func RegisterEntityNameVisibilityHook(hook EntityNameVisibilityHook) {
	entityNameVisibilityHooks = append(entityNameVisibilityHooks, hook)
}

func (c *Checker) applyEntityNameVisibilityHooks(entityName *ast.Node, symbol *ast.Symbol) bool {
	for _, hook := range entityNameVisibilityHooks {
		if hook(c, entityName, symbol) {
			return true
		}
	}
	return false
}

// unwrapReturnTypeOf returns the type that what fn returns is checked
// against, given returnType, fn's return type: the type an async function or
// a generator wraps, as unwrapReturnType says, unless a hook says otherwise.
func (c *Checker) unwrapReturnTypeOf(fn *ast.Node, returnType *Type) *Type {
	t := c.unwrapReturnType(returnType, ast.GetFunctionFlags(fn))
	for _, hook := range unwrappedReturnTypeHooks {
		t = hook(c, fn, t)
	}
	return t
}

// UnwrappedReturnTypeHook can replace t, the type that the return statements
// and expression body of fn, a function with a return type annotation, are
// checked against. TypeScript checks what an async function returns against
// the type its Promise wraps; an extension can unwrap its own types the same
// way.
type UnwrappedReturnTypeHook func(c *Checker, fn *ast.Node, t *Type) *Type

var unwrappedReturnTypeHooks []UnwrappedReturnTypeHook

// RegisterUnwrappedReturnTypeHook adds hook to every checker. Call it only
// from the init function of an in-tree package that tsc/cmd/tsc imports;
// registration is not synchronized.
func RegisterUnwrappedReturnTypeHook(hook UnwrappedReturnTypeHook) {
	unwrappedReturnTypeHooks = append(unwrappedReturnTypeHooks, hook)
}

// AnnotatedReturnTypeHook can replace t, the return type that the return type
// annotation of fn declares, as the return type of fn's signature.
type AnnotatedReturnTypeHook func(c *Checker, fn *ast.Node, t *Type) *Type

var annotatedReturnTypeHooks []AnnotatedReturnTypeHook

// RegisterAnnotatedReturnTypeHook adds hook to every checker. Call it only
// from the init function of an in-tree package that tsc/cmd/tsc imports;
// registration is not synchronized.
func RegisterAnnotatedReturnTypeHook(hook AnnotatedReturnTypeHook) {
	annotatedReturnTypeHooks = append(annotatedReturnTypeHooks, hook)
}

func (c *Checker) applyAnnotatedReturnTypeHooks(fn *ast.Node, t *Type) *Type {
	for _, hook := range annotatedReturnTypeHooks {
		t = hook(c, fn, t)
	}
	return t
}

// InferTypeHook reports whether an extension gives node, an infer type
// outside the extends clause of a conditional type, a meaning in its
// position. The checker then reports neither that position nor the unused
// name.
type InferTypeHook func(c *Checker, node *ast.Node) bool

var inferTypeHooks []InferTypeHook

// RegisterInferTypeHook adds hook to every checker. Call it only from the init
// function of an in-tree package that tsc/cmd/tsc imports; registration is not
// synchronized.
func RegisterInferTypeHook(hook InferTypeHook) {
	inferTypeHooks = append(inferTypeHooks, hook)
}

func (c *Checker) inferTypeHasExtensionMeaning(node *ast.Node) bool {
	for _, hook := range inferTypeHooks {
		if hook(c, node) {
			return true
		}
	}
	return false
}

// GetReturnTypeFromBody returns the return type that TypeScript infers from
// the body of fn, before any inferred return type hook replaces it.
func (c *Checker) GetReturnTypeFromBody(fn *ast.Node) *Type {
	return c.getReturnTypeFromBodyWorker(fn, CheckModeNormal)
}
