package effeck

import (
	"context"
	"path"
	"slices"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// The prelude (design decisions D4 and D48) is the names every .effeck file sees
// without an import: Effect, Schema, Layer, TaggedError, Control, Scope, pipe,
// wire, fork, handle, resume, resumeWith, and abandon. They are the exports of
// effeck-runtime, which declares each with the right meanings: Effect
// is the namespace and the type Effect<A, E, R>, Scope the namespace and the
// type Scope.Scope.
//
// The program resolves effeck-runtime from every .effeck file as an
// implicit import, and the checker resolves a prelude name that no scope
// declares to the runtime's export, before it searches the global scope. So a
// local declaration or import shadows a prelude name, and .ts files never see
// the prelude. The emitted JavaScript imports each prelude name the file uses
// as a value: the Effect modules from effect, the rest from the runtime.

// runtimeModule is the package that holds the prelude and the helpers that
// generated code calls.
const runtimeModule = "effeck-runtime"

// preludeModules maps each prelude name to the module the emitted JavaScript
// imports it from. A name from an effect module is imported as that module's
// namespace (import * as Effect from "effect/Effect").
var preludeModules = map[string]string{
	"Effect":      "effect/Effect",
	"Layer":       "effect/Layer",
	"Schema":      "effect/Schema",
	"Scope":       "effect/Scope",
	"TaggedError": runtimeModule,
	"Control":     runtimeModule,
	"pipe":        runtimeModule,
	"wire":        runtimeModule,
	"fork":        runtimeModule,
	"handle":      runtimeModule,
	"resume":      runtimeModule,
	"resumeWith":  runtimeModule,
	"abandon":     runtimeModule,
	// The boundary hatches and the runners that lead back in (D148, hatches.go).
	"sync":     runtimeModule,
	"try":      runtimeModule,
	"callback": runtimeModule,
	// The type of callback's resume parameter. Inputs keep callback's type
	// arguments from being written, so the parameter's annotation names them.
	"Resume":      runtimeModule,
	"runPromise":  runtimeModule,
	"runFork":     runtimeModule,
	"runSync":     runtimeModule,
	"runCallback": runtimeModule,
	// The test of a typed catch clause, which the parser writes (D131, catches.go).
	"$isFailure": runtimeModule,
	// What a handle statement's clause binds, which the parser writes for the
	// checker and the lowering removes (D132, handles.go).
	"$clause": runtimeModule,
	// The fiber forms (D166-D170, fibers.go): the parser writes await, select,
	// and for concurrent as calls of these; detach and forkIn are fork's variants.
	"$await":  runtimeModule,
	"$join":   runtimeModule,
	"$select": runtimeModule,
	// The await operations (D196): await.all, await.allSettled, await.race, await.any.
	"$awaitAll":        runtimeModule,
	"$awaitAllSettled": runtimeModule,
	"$awaitRace":       runtimeModule,
	"$awaitAny":        runtimeModule,
	"$forConcurrent":   runtimeModule,
	"detach":           runtimeModule,
	"forkIn":           runtimeModule,
	// A provide block's Key = value provider, which the parser writes (D173, provide.go).
	"$provideService": runtimeModule,
	// The primitives with their own syntax (D176-D180, primitives.go): the bare
	// functions, the functions an interruption block calls, and the names the
	// parser writes for a catch clause typed Cause, finally (exit), and the
	// interruption blocks' markers.
	"interrupt":             runtimeModule,
	"yieldNow":              runtimeModule,
	"uninterruptible":       runtimeModule,
	"restore":               runtimeModule,
	"interruptible":         runtimeModule,
	"$failureOf":            runtimeModule,
	"$noFailure":            runtimeModule,
	"$exit":                 runtimeModule,
	"$uninterruptibleBlock": runtimeModule,
	"$restoreBlock":         runtimeModule,
	"$interruptibleBlock":   runtimeModule,
	// Pattern matching (D206-D225, match.go): the parser writes match and is
	// with these helpers; customMatcher is the proposal's Symbol.customMatcher.
	"$is":           runtimeModule,
	"$extract":      runtimeModule,
	"$rest":         runtimeModule,
	"$noMatch":      runtimeModule,
	"$patternError": runtimeModule,
	"customMatcher": runtimeModule,
}

// implicitRuntimeImport makes a .effeck file that may use the prelude resolve the
// runtime, and one that may use an export of effect's root (effectroot.go)
// resolve effect, so their declarations are in the program. A file that uses
// neither compiles exactly as TypeScript would.
func implicitRuntimeImport(file *ast.SourceFile) []string {
	if !isEffeckFile(file) {
		return nil
	}
	prelude, root := mayUsePrelude(file)
	entry := entryMain(file) != nil
	var modules []string
	if prelude || entry {
		modules = append(modules, runtimeModule)
	}
	if root {
		modules = append(modules, effectRootModule)
	}
	if entry {
		// An entry module runs main with NodeRuntime when the package resolves (entry.go).
		modules = append(modules, platformNodeModule)
	}
	return modules
}

// mayUsePrelude reports whether file has an identifier with a prelude name,
// and one with the name of an effect root export, that no top-level
// declaration or import of file declares. It runs before binding, so it reads
// names from the syntax, and a property named pipe or an inner declaration
// counts as a use, which only loads a module.
func mayUsePrelude(file *ast.SourceFile) (prelude bool, root bool) {
	declared := map[string]bool{}
	for _, statement := range file.Statements.Nodes {
		forEachTopLevelName(statement, func(name string) { declared[name] = true })
	}
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if ast.IsIdentifier(node) && !declared[node.Text()] {
			if _, ok := preludeModules[node.Text()]; ok {
				prelude = true
			} else if _, ok := effectRootNames[node.Text()]; ok || effectFunctionNames[node.Text()] {
				root = true
			}
			return prelude && root
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
	return prelude, root
}

// forEachTopLevelName calls fn with each name that statement, a statement at
// a module's top level, declares or imports.
func forEachTopLevelName(statement *ast.Node, fn func(name string)) {
	switch statement.Kind {
	case ast.KindImportDeclaration:
		clause := statement.AsImportDeclaration().ImportClause
		if clause == nil {
			return
		}
		if name := clause.Name(); name != nil {
			fn(name.Text())
		}
		if bindings := clause.AsImportClause().NamedBindings; bindings != nil {
			if ast.IsNamespaceImport(bindings) {
				fn(bindings.Name().Text())
			} else {
				for _, specifier := range bindings.AsNamedImports().Elements.Nodes {
					fn(specifier.Name().Text())
				}
			}
		}
	case ast.KindVariableStatement:
		for _, declaration := range statement.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
			forEachBindingName(declaration.Name(), fn)
		}
	case ast.KindImportEqualsDeclaration, ast.KindFunctionDeclaration, ast.KindClassDeclaration, ast.KindEnumDeclaration,
		ast.KindInterfaceDeclaration, ast.KindTypeAliasDeclaration, ast.KindModuleDeclaration:
		if name := statement.Name(); name != nil && ast.IsIdentifier(name) {
			fn(name.Text())
		}
	}
}

// forEachBindingName calls fn with each identifier that name, a binding name
// or pattern, binds.
func forEachBindingName(name *ast.Node, fn func(name string)) {
	switch {
	case name == nil:
	case ast.IsIdentifier(name):
		fn(name.Text())
	case ast.IsBindingPattern(name):
		for _, element := range name.AsBindingPattern().Elements.Nodes {
			forEachBindingName(element.Name(), fn)
		}
	}
}

// preludeState is what one checker remembers about the prelude.
type preludeState struct {
	modules  map[*ast.SourceFile]*ast.Symbol     // the runtime module each .effeck file resolves
	roots    map[*ast.SourceFile]*ast.Symbol     // the effect root module each .effeck file resolves
	uses     map[*ast.SourceFile]map[string]bool // prelude names each .effeck file uses as values
	typeUses map[*ast.SourceFile]map[string]bool // prelude names each .effeck file's declaration file writes (declarations.go)
	typeRefs map[*ast.Node]bool                  // the identifiers that write those names
}

type preludeStateKey struct{}

func preludeStateOf(c *checker.Checker) *preludeState {
	return c.ExtensionState(preludeStateKey{}, func() any {
		return &preludeState{
			modules:  map[*ast.SourceFile]*ast.Symbol{},
			roots:    map[*ast.SourceFile]*ast.Symbol{},
			uses:     map[*ast.SourceFile]map[string]bool{},
			typeUses: map[*ast.SourceFile]map[string]bool{},
			typeRefs: map[*ast.Node]bool{},
		}
	}).(*preludeState)
}

// resolvePreludeName resolves name, which no scope around location declares,
// when location is in a .effeck file: a prelude name to the runtime's export,
// and the name of an effect root export to that export unless a global value
// has the name, in each case when the export has the meaning the checker asks
// for.
func resolvePreludeName(c *checker.Checker, location *ast.Node, name string, meaning ast.SymbolFlags) *ast.Symbol {
	_, isPrelude := preludeModules[name]
	_, isRoot := effectRootNames[name]
	if !isPrelude && !isRoot && !effectFunctionNames[name] {
		return nil
	}
	file := ast.GetSourceFileOfNode(location)
	if !isEffeckFile(file) {
		return nil
	}
	state := preludeStateOf(c)
	symbol := preludeSymbol(c, state, file, name)
	if symbol == nil || !hasMeaning(c, symbol, meaning) {
		return nil
	}
	if isValueReference(location, name, meaning) && !isLoweredHatchCallee(location) && !isClauseCallee(location) && !isRemovedPrimitiveMarker(location) {
		recordPreludeUse(state, file, name)
	}
	return symbol
}

// recordPreludeUse records that file uses name, a prelude name or effect root
// export, as a value, so the lowering imports it.
func recordPreludeUse(state *preludeState, file *ast.SourceFile, name string) {
	if state.uses[file] == nil {
		state.uses[file] = map[string]bool{}
	}
	state.uses[file][name] = true
}

// preludeSymbol returns the export that name, a prelude name or the name of an
// effect root export, refers to in file, or nil when its module is not
// installed or, for an effect root export, JavaScript defines a global value
// with the name.
func preludeSymbol(c *checker.Checker, state *preludeState, file *ast.SourceFile, name string) *ast.Symbol {
	if _, ok := preludeModules[name]; ok {
		if module := runtimeModuleSymbol(c, state, file); module != nil {
			return c.TryGetMemberInModuleExports(name, module)
		}
		return nil
	}
	if isBuiltinGlobal(c, name) {
		return nil
	}
	module := effectRootModuleSymbol(c, state, file)
	if module == nil {
		return nil
	}
	if effectFunctionNames[name] {
		// A function of the Effect module, reached through the root's Effect.
		effect := c.TryGetMemberInModuleExports("Effect", module)
		if effect == nil {
			return nil
		}
		if effect.Flags()&ast.SymbolFlagsAlias != 0 {
			effect = c.GetAliasedSymbol(effect)
		}
		return c.TryGetMemberInModuleExports(name, effect)
	}
	return c.TryGetMemberInModuleExports(name, module)
}

// isLoweredHatchCallee reports whether location names the callee of a sync or
// try hatch, which the lowering replaces with Effect's function, so the
// emitted JavaScript needs no import of it. try cannot be imported at all.
func isLoweredHatchCallee(location *ast.Node) bool {
	parent := location.Parent
	return parent != nil && ast.IsCallExpression(parent) && parent.Expression() == location && (location.Text() == "sync" || location.Text() == "try")
}

// isBuiltinGlobal reports whether name is a global value that JavaScript
// itself defines, declared by an ECMAScript lib file (lib.es5.d.ts,
// lib.es2015.core.d.ts, …), such as Array, String, Number, or Symbol. Such a
// global keeps its meaning in a .effeck file. A host's globals, such as the
// DOM's Option and Request, do not: .effeck code reaches the host through
// effect's modules (D102), so effect's Option wins.
func isBuiltinGlobal(c *checker.Checker, name string) bool {
	symbol := c.GlobalSymbol(name, ast.SymbolFlagsValue)
	if symbol == nil {
		return false
	}
	for _, declaration := range symbol.Declarations() {
		if file := ast.GetSourceFileOfNode(declaration); file != nil && strings.HasPrefix(path.Base(string(file.FileName())), "lib.es") {
			return true
		}
	}
	return false
}

// isValueReference reports whether resolving name at location, with
// meaning, is a reference in the source that the emitted JavaScript keeps:
// an identifier with that name, outside every type. Checks that resolve a
// name for their own purposes, such as the namespace Effect at a function,
// pass another location and are not uses.
func isValueReference(location *ast.Node, name string, meaning ast.SymbolFlags) bool {
	return meaning&ast.SymbolFlagsValue != 0 && ast.IsIdentifier(location) && location.Text() == name && !inTypeNode(location)
}

// inTypeNode reports whether node is inside a type, such as the Scope of
// Scope.Scope in a type reference or the X of typeof X. A name there is erased
// from the emitted JavaScript, so it needs no import.
func inTypeNode(node *ast.Node) bool {
	for n := node; n != nil; n = n.Parent {
		if n.Kind >= ast.KindFirstTypeNode && n.Kind <= ast.KindLastTypeNode {
			return true
		}
	}
	return false
}

// hasMeaning reports whether symbol, or the symbol it aliases, has meaning,
// as the checker's own scope lookup requires.
func hasMeaning(c *checker.Checker, symbol *ast.Symbol, meaning ast.SymbolFlags) bool {
	if symbol.Flags()&meaning != 0 {
		return true
	}
	if symbol.Flags()&ast.SymbolFlagsAlias != 0 {
		if target := c.GetAliasedSymbol(symbol); target != nil && target.Flags()&meaning != 0 {
			return true
		}
	}
	return false
}

// runtimeModuleSymbol returns the symbol of the runtime module that file
// resolves, or nil when the runtime is not installed.
func runtimeModuleSymbol(c *checker.Checker, state *preludeState, file *ast.SourceFile) *ast.Symbol {
	return resolvedModuleSymbol(c, state.modules, file, runtimeModule)
}

// effectRootModuleSymbol returns the symbol of the effect root module that
// file resolves, or nil when effect is not installed.
func effectRootModuleSymbol(c *checker.Checker, state *preludeState, file *ast.SourceFile) *ast.Symbol {
	return resolvedModuleSymbol(c, state.roots, file, effectRootModule)
}

// resolvedModuleSymbol returns the symbol of the module named name that file
// resolves, remembered in cache, or nil when it does not resolve.
func resolvedModuleSymbol(c *checker.Checker, cache map[*ast.SourceFile]*ast.Symbol, file *ast.SourceFile, name string) *ast.Symbol {
	if module, ok := cache[file]; ok {
		return module
	}
	var module *ast.Symbol
	program := c.Program()
	for key, resolved := range program.GetResolvedModules()[file.PathKey()] {
		if key.Name == name && resolved.IsResolved() {
			if moduleFile := program.GetSourceFileForResolvedModule(resolved); moduleFile != nil {
				module = c.GetSymbolOfDeclaration(moduleFile.AsNode())
				break
			}
		}
	}
	cache[file] = module
	return module
}

// preludeUses returns the prelude names that file uses as values, sorted. It
// checks file first, because the checker records uses while it checks.
func preludeUses(c *checker.Checker, file *ast.SourceFile) []string {
	c.GetDiagnostics(context.Background(), file)
	var names []string
	for name := range preludeStateOf(c).uses[file] {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// preludeImports returns import declarations for names, the prelude names and
// effect root exports a file uses as values, sorted:
//
//	import * as Console from "effect/Console"
//	import * as Effect from "effect/Effect"
//	import { flow } from "effect/Function"
//	import { TaggedError, pipe } from "effeck-runtime"
//
// The checker resolved each use to the runtime's or effect's export, so the
// declarations bind the names the emitted code already refers to. A root
// export is imported from its own module, never from effect's root, which
// would load every module. They are ES module imports; .effeck files compile
// to ES modules.
func preludeImports(f *ast.NodeFactory, names []string) []*ast.Node {
	var imports []*ast.Node
	named := map[string][]*ast.Node{} // module -> named import specifiers
	var namedModules []string
	for _, name := range names {
		module, namespace := preludeModules[name], true
		if module == "" && effectFunctionNames[name] {
			module, namespace = "effect/Effect", false
		} else if module == "" {
			root := effectRootNames[name]
			module, namespace = root.module, root.namespace
		}
		if module == runtimeModule || !namespace {
			if named[module] == nil {
				namedModules = append(namedModules, module)
			}
			named[module] = append(named[module], f.NewImportSpecifier(false /*isTypeOnly*/, nil /*propertyName*/, f.NewIdentifier(name)))
			continue
		}
		clause := f.NewImportClause(ast.KindUnknown, nil /*name*/, f.NewNamespaceImport(f.NewIdentifier(name)))
		imports = append(imports, f.NewImportDeclaration(nil /*modifiers*/, clause, f.NewStringLiteral(module, ast.TokenFlagsNone), nil /*attributes*/))
	}
	// The runtime's named imports come last, as before effect's root joined.
	slices.SortStableFunc(namedModules, func(a, b string) int {
		if (a == runtimeModule) != (b == runtimeModule) {
			if a == runtimeModule {
				return 1
			}
			return -1
		}
		return strings.Compare(a, b)
	})
	for _, module := range namedModules {
		clause := f.NewImportClause(ast.KindUnknown, nil /*name*/, f.NewNamedImports(f.NewNodeList(named[module])))
		imports = append(imports, f.NewImportDeclaration(nil /*modifiers*/, clause, f.NewStringLiteral(module, ast.TokenFlagsNone), nil /*attributes*/))
	}
	for _, declaration := range imports {
		ast.SetParentInChildren(declaration)
	}
	return imports
}
