package effectscript

import (
	"context"
	"slices"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// The prelude (design decisions D4 and D48) is the names every .es file sees
// without an import: Effect, Schema, Layer, TaggedError, Control, Scope, pipe,
// wire, fork, handle, resume, resumeWith, and abandon. They are the exports of
// @effectscript/runtime, which declares each with the right meanings: Effect
// is the namespace and the type Effect<A, E, R>, Scope the namespace and the
// type Scope.Scope.
//
// The program resolves @effectscript/runtime from every .es file as an
// implicit import, and the checker resolves a prelude name that no scope
// declares to the runtime's export, before it searches the global scope. So a
// local declaration or import shadows a prelude name, and .ts files never see
// the prelude. The emitted JavaScript imports each prelude name the file uses
// as a value: the Effect modules from effect, the rest from the runtime.

// runtimeModule is the package that holds the prelude and the helpers that
// generated code calls.
const runtimeModule = "@effectscript/runtime"

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
}

// implicitRuntimeImport makes a .es file that may use the prelude resolve the
// runtime, so the runtime's declarations are in the program. A file that uses
// no prelude name compiles exactly as TypeScript would.
func implicitRuntimeImport(file *ast.SourceFile) []string {
	if isEffectScriptFile(file) && mayUsePrelude(file) {
		return []string{runtimeModule}
	}
	return nil
}

// mayUsePrelude reports whether file has an identifier with a prelude name
// that no top-level declaration or import of file declares. It runs before
// binding, so it reads names from the syntax, and a property named pipe or
// an inner declaration counts as a use, which only loads the runtime.
func mayUsePrelude(file *ast.SourceFile) bool {
	declared := map[string]bool{}
	for _, statement := range file.Statements.Nodes {
		forEachTopLevelName(statement, func(name string) { declared[name] = true })
	}
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if ast.IsIdentifier(node) {
			_, isPrelude := preludeModules[node.Text()]
			return isPrelude && !declared[node.Text()]
		}
		return node.ForEachChild(visit)
	}
	return file.AsNode().ForEachChild(visit)
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
	modules  map[*ast.SourceFile]*ast.Symbol     // the runtime module each .es file resolves
	uses     map[*ast.SourceFile]map[string]bool // prelude names each .es file uses as values
	typeUses map[*ast.SourceFile]map[string]bool // prelude names each .es file's declaration file writes (declarations.go)
}

type preludeStateKey struct{}

func preludeStateOf(c *checker.Checker) *preludeState {
	return c.ExtensionState(preludeStateKey{}, func() any {
		return &preludeState{
			modules:  map[*ast.SourceFile]*ast.Symbol{},
			uses:     map[*ast.SourceFile]map[string]bool{},
			typeUses: map[*ast.SourceFile]map[string]bool{},
		}
	}).(*preludeState)
}

// resolvePreludeName resolves name, which no scope around location declares,
// to the runtime's export when location is in a .es file and name is a
// prelude name with the meaning the checker asks for.
func resolvePreludeName(c *checker.Checker, location *ast.Node, name string, meaning ast.SymbolFlags) *ast.Symbol {
	if _, ok := preludeModules[name]; !ok {
		return nil
	}
	file := ast.GetSourceFileOfNode(location)
	if !isEffectScriptFile(file) {
		return nil
	}
	state := preludeStateOf(c)
	module := runtimeModuleSymbol(c, state, file)
	if module == nil {
		return nil
	}
	symbol := c.TryGetMemberInModuleExports(name, module)
	if symbol == nil || !hasMeaning(c, symbol, meaning) {
		return nil
	}
	if isValueReference(location, name, meaning) {
		if state.uses[file] == nil {
			state.uses[file] = map[string]bool{}
		}
		state.uses[file][name] = true
	}
	return symbol
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
	if module, ok := state.modules[file]; ok {
		return module
	}
	var module *ast.Symbol
	program := c.Program()
	for key, resolved := range program.GetResolvedModules()[file.PathKey()] {
		if key.Name == runtimeModule && resolved.IsResolved() {
			if moduleFile := program.GetSourceFileForResolvedModule(resolved); moduleFile != nil {
				module = c.GetSymbolOfDeclaration(moduleFile.AsNode())
				break
			}
		}
	}
	state.modules[file] = module
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

// preludeImports returns import declarations for names, the prelude names a
// file uses as values:
//
//	import * as Effect from "effect/Effect"
//	import { TaggedError, pipe } from "@effectscript/runtime"
//
// The checker resolved each use to the runtime's export, so the declarations
// bind the names the emitted code already refers to. They are ES module
// imports; .es files compile to ES modules.
func preludeImports(f *ast.NodeFactory, names []string) []*ast.Node {
	var imports, fromRuntime []*ast.Node
	for _, name := range names {
		module := preludeModules[name]
		if module == runtimeModule {
			fromRuntime = append(fromRuntime, f.NewImportSpecifier(false /*isTypeOnly*/, nil /*propertyName*/, f.NewIdentifier(name)))
			continue
		}
		clause := f.NewImportClause(ast.KindUnknown, nil /*name*/, f.NewNamespaceImport(f.NewIdentifier(name)))
		imports = append(imports, f.NewImportDeclaration(nil /*modifiers*/, clause, f.NewStringLiteral(module, ast.TokenFlagsNone), nil /*attributes*/))
	}
	if len(fromRuntime) > 0 {
		clause := f.NewImportClause(ast.KindUnknown, nil /*name*/, f.NewNamedImports(f.NewNodeList(fromRuntime)))
		imports = append(imports, f.NewImportDeclaration(nil /*modifiers*/, clause, f.NewStringLiteral(runtimeModule, ast.TokenFlagsNone), nil /*attributes*/))
	}
	for _, declaration := range imports {
		ast.SetParentInChildren(declaration)
	}
	return imports
}
