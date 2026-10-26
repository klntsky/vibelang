package vibelang

import (
	"context"
	"path"
	"slices"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/vibelangmapper"
)

// The prelude (design decisions D4 and D48) is the names every .vibe file sees
// without an import: Effect, Schema, Layer, TaggedError, Control, Scope, pipe,
// wire, fork, handle, resume, resumeWith, and abandon. They are the exports of
// vibelang-runtime, which declares each with the right meanings: Effect
// is the namespace and the type Effect<A, E, R>, Scope the namespace and the
// type Scope.Scope.
//
// The program resolves vibelang-runtime from every .vibe file as an
// implicit import, and the checker resolves a prelude name that no scope
// declares to the runtime's export, before it searches the global scope. So a
// local declaration or import shadows a prelude name, and .ts files never see
// the prelude. The emitted JavaScript imports each prelude name the file uses
// as a value: the Effect modules from effect, the rest from the runtime.

// runtimeModule is the package that holds the prelude and the helpers that
// generated code calls.
const runtimeModule = "vibelang-runtime"

// preludeModules maps each prelude name to the module the emitted JavaScript
// imports it from. A name from an effect module is imported as that module's
// namespace (import * as Effect from "effect/Effect").
var preludeModules = map[string]string{
	"Effect":      "effect/Effect",
	"Layer":       "effect/Layer",
	"Schema":      "effect/Schema",
	"Scope":       "effect/Scope",
	"TaggedError": runtimeModule,
	// An error that extends another, which the parser writes (D278, parser/vibelang_errors.go).
	"$errorSubclass": runtimeModule,
	"$ErrorArgs":     runtimeModule,
	// How a schema error joins its family, which the parser writes (D304).
	"$schemaErrorFamily": runtimeModule,
	"$schemaErrorMember": runtimeModule,
	"Control":            runtimeModule,
	"pipe":               runtimeModule,
	"wire":               runtimeModule,
	"fork":               runtimeModule,
	"handle":             runtimeModule,
	"resume":             runtimeModule,
	"resumeWith":         runtimeModule,
	"abandon":            runtimeModule,
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
	// A catch expression, which the parser writes (D236, catch_expressions.go).
	"$catchExpression": runtimeModule,
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
	// The extends clause the parser writes for service Name: Type (D281,
	// services.go). The lowering replaces it, so it is never a use.
	"$service": runtimeModule,
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

// implicitRuntimeImport makes a .vibe file that may use the prelude resolve the
// runtime, and one that may use an export of effect's root (effectroot.go)
// resolve effect, so their declarations are in the program. A file with a
// function that may compile to plain JavaScript resolves effect too: such a
// function is still an Effect to TypeScript (plain.go), and its type needs
// effect's Effect interface. A file with none of these compiles exactly as
// TypeScript would. So does a file whose functions are Effects only by what
// they do (mayLowerToEffect): one that throws, a producer, and one that
// declares `using`.
func implicitRuntimeImport(file *ast.SourceFile) []string {
	if !isVibeLangFile(file) {
		return nil
	}
	prelude, root := mayUsePrelude(file)
	effect, runtime := mayLowerToEffect(file)
	entry := entryMain(file) != nil
	var modules []string
	if prelude || entry || runtime {
		modules = append(modules, runtimeModule)
	}
	if root || effect || maySplitPlainFunction(file) {
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

// maySplitPlainFunction reports whether file declares a function that may
// compile to plain JavaScript with an Effect form (isSplittable): a named
// function declaration with a body and no return type annotation that is
// neither async nor a generator. A function annotated with Effect<A, E, R> names
// effect's Effect, so mayUsePrelude already loads effect for it, and one
// annotated with a plain type keeps that type.
func maySplitPlainFunction(file *ast.SourceFile) bool {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if node.Kind == ast.KindFunctionDeclaration && node.Name() != nil && node.Body() != nil && node.Type() == nil &&
			ast.GetFunctionFlags(node)&(ast.FunctionFlagsAsync|ast.FunctionFlagsGenerator) == 0 {
			return true
		}
		return node.ForEachChild(visit)
	}
	return file.AsNode().ForEachChild(visit)
}

// mayLowerToEffect reports whether file has a function that may compile to an
// Effect though it names no prelude or effect root export: one with a throw
// (D282), one that calls a built-in whose failures are expected (D101), such
// as JSON.parse, a producer, a function* without a JavaScript iterator
// annotation, or one that declares `using`. The last two also call the
// runtime's helpers. Their types name effect's Effect, so the program needs
// effect's declarations: without them, a function that only throws, or only
// parses JSON, was declared plain while its JavaScript returned an Effect. A
// module declaration's body is TypeScript (D283), so it is not read.
func mayLowerToEffect(file *ast.SourceFile) (effect bool, runtime bool) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		switch {
		case node.Kind == ast.KindModuleDeclaration:
			return false
		case node.Kind == ast.KindThrowStatement, namesParsingBuiltin(node):
			effect = true
		case node.Kind == ast.KindVariableDeclarationList && isUsingList(node),
			(node.Kind == ast.KindFunctionDeclaration || node.Kind == ast.KindFunctionExpression) && node.Body() != nil &&
				ast.GetFunctionFlags(node)&(ast.FunctionFlagsAsync|ast.FunctionFlagsGenerator) == ast.FunctionFlagsGenerator &&
				!vibelangmapper.HasJavaScriptIteratorReturn(node):
			effect, runtime = true, true
		}
		if effect && runtime {
			return true
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
	return effect, runtime
}

// namesParsingBuiltin reports whether node names a built-in whose failures
// are expected (builtinContracts, D101): decodeURI, decodeURIComponent, or
// JSON.parse, globalThis.JSON.parse included. It reads syntax, before binding,
// so a local of the same name counts too, which only loads a module.
func namesParsingBuiltin(node *ast.Node) bool {
	switch {
	case ast.IsIdentifier(node):
		_, ok := builtinContracts[node.Text()]
		return ok
	case ast.IsPropertyAccessExpression(node):
		object := node.Expression()
		if ast.IsPropertyAccessExpression(object) {
			object = object.Name()
		}
		if !ast.IsIdentifier(object) || !ast.IsIdentifier(node.Name()) {
			return false
		}
		_, ok := builtinContracts[object.Text()+"."+node.Name().Text()]
		return ok
	}
	return false
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
	modules  map[*ast.SourceFile]*ast.Symbol     // the runtime module each .vibe file resolves
	roots    map[*ast.SourceFile]*ast.Symbol     // the effect root module each .vibe file resolves
	uses     map[*ast.SourceFile]map[string]bool // prelude names each .vibe file uses as values
	typeUses map[*ast.SourceFile]map[string]bool // prelude names each .vibe file's declaration file writes (declarations.go)
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
// when location is in a .vibe file: a prelude name to the runtime's export,
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
	if !isVibeLangFile(file) {
		return nil
	}
	state := preludeStateOf(c)
	symbol := preludeSymbol(c, state, file, name)
	if symbol == nil || !hasMeaning(c, symbol, meaning) {
		return nil
	}
	if isValueReference(location, name, meaning) && !isLoweredHatchCallee(location) && !isClauseCallee(location) && !isRemovedPrimitiveMarker(location) && !isServiceShapeFactory(location) {
		recordPreludeUse(state, file, name)
	}
	return symbol
}

// preludeSpellingCandidates offers the prelude's names and effect's root
// exports, which no symbol table of a .vibe file lists, to TypeScript's "Did
// you mean" suggestion for a value there: Consle.log suggests Console, which
// is one edit away, not the global console (TS2552). Names the parser
// writes, which start with $, are not offered.
func preludeSpellingCandidates(c *checker.Checker, location *ast.Node, meaning ast.SymbolFlags) []*ast.Symbol {
	file := ast.GetSourceFileOfNode(location)
	if meaning&ast.SymbolFlagsValue == 0 || !isVibeLangFile(file) {
		return nil
	}
	state := preludeStateOf(c)
	var candidates []*ast.Symbol
	offer := func(name string) {
		if !strings.HasPrefix(name, "$") {
			if symbol := preludeSymbol(c, state, file, name); symbol != nil {
				candidates = append(candidates, symbol)
			}
		}
	}
	for name := range preludeModules {
		offer(name)
	}
	for name := range effectRootNames {
		offer(name)
	}
	for name := range effectFunctionNames {
		offer(name)
	}
	return candidates
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

// isBuiltinGlobal gives ECMAScript and shared web-standard globals precedence
// over effect root names (D146, D264). DOM-only globals such as Option lose.
func isBuiltinGlobal(c *checker.Checker, name string) bool {
	symbol := c.GlobalSymbol(name, ast.SymbolFlagsValue)
	if symbol == nil {
		return false
	}
	switch name {
	case "Request", "Response", "Headers", "URL", "URLSearchParams", "TextEncoder", "TextDecoder",
		"AbortController", "AbortSignal", "Blob", "FormData", "structuredClone", "fetch":
		return true
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
// resolves in the current program. Cache successful resolutions only; a miss
// during an editor update must not poison later lookups.
func resolvedModuleSymbol(c *checker.Checker, cache map[*ast.SourceFile]*ast.Symbol, file *ast.SourceFile, name string) *ast.Symbol {
	if module := cache[file]; module != nil {
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
	if module != nil {
		cache[file] = module
	}
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

// requireNamedImports returns declaration, an import the lowering adds, as
// it must stand in a file emitted as CommonJS. TypeScript's CommonJS
// transform rewrites each reference to a named import through the checker's
// binding, which the names the lowering writes have none of, so
// `import { TaggedError } from "vibelang-runtime"` would leave TaggedError
// unbound. A named import becomes a destructuring require,
//
//	const { TaggedError, pipe } = require("vibelang-runtime")
//
// which binds the names themselves; a namespace import, which the transform
// writes as a const of its own name, stays as it is.
func requireNamedImports(f *ast.NodeFactory, declaration *ast.Node) *ast.Node {
	clause := declaration.AsImportDeclaration().ImportClause
	if clause == nil || clause.AsImportClause().NamedBindings == nil || !ast.IsNamedImports(clause.AsImportClause().NamedBindings) {
		return declaration
	}
	var elements []*ast.Node
	for _, specifier := range clause.AsImportClause().NamedBindings.Elements() {
		var property *ast.Node
		if name := specifier.PropertyName(); name != nil {
			property = f.NewIdentifier(name.Text())
		}
		elements = append(elements, f.NewBindingElement(nil /*dotDotDotToken*/, property, f.NewIdentifier(specifier.Name().Text()), nil /*initializer*/))
	}
	module := declaration.AsImportDeclaration().ModuleSpecifier.Text()
	require := f.NewCallExpression(f.NewIdentifier("require"), nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{f.NewStringLiteral(module, ast.TokenFlagsNone)}), ast.NodeFlagsNone)
	pattern := f.NewBindingPattern(ast.KindObjectBindingPattern, f.NewNodeList(elements))
	list := f.NewVariableDeclarationList(f.NewNodeList([]*ast.Node{f.NewVariableDeclaration(pattern, nil /*exclamationToken*/, nil /*typeNode*/, require)}), ast.NodeFlagsConst)
	statement := f.NewVariableStatement(nil /*modifiers*/, list)
	ast.SetParentInChildren(statement)
	return statement
}

// preludeImports returns import declarations for names, the prelude names and
// effect root exports a file uses as values, sorted:
//
//	import * as Console from "effect/Console"
//	import * as Effect from "effect/Effect"
//	import { flow } from "effect/Function"
//	import { TaggedError, pipe } from "vibelang-runtime"
//
// The checker resolved each use to the runtime's or effect's export, so the
// declarations bind the names the emitted code already refers to. A root
// export is imported from its own module, never from effect's root, which
// would load every module. They are ES module imports; for a file emitted
// as CommonJS, requireNamedImports rewrites the named ones.
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
