package vibelang

import (
	"slices"
	"strconv"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/printer"
	"github.com/microsoft/TypeScript/tsc/internal/transformers"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

// The declaration file of a .vibe file is ordinary TypeScript, so it must import
// what the source took from the prelude and name its imports by output:
//
//	// main.vibe                               // main.d.ts
//	import { find } from "./users.vibe"        import type { Effect } from "vibelang-runtime";
//	export const one: Effect<number> = …     import { find } from "./users.js";
//	                                         export declare const one: Effect<number>;
//
// Declaration emit asks whether each name a declaration writes is visible;
// preludeNameVisible says yes for a prelude name and records it, and
// transformDeclarations imports the recorded names and rewrites specifiers.

// preludeNameVisible lets the declaration file of a .vibe file write a prelude
// name or effect root export that the source wrote, such as Effect in
// Effect<number> or Option in Option.Option<string>.
func preludeNameVisible(c *checker.Checker, entityName *ast.Node, symbol *ast.Symbol) bool {
	first := ast.GetFirstIdentifier(entityName)
	name := first.Text()
	_, isPrelude := preludeModules[name]
	_, isRoot := effectRootNames[name]
	if !isPrelude && !isRoot {
		return false
	}
	file := ast.GetSourceFileOfNode(entityName)
	if !isVibeLangFile(file) {
		return false
	}
	state := preludeStateOf(c)
	if preludeSymbol(c, state, file, name) != symbol {
		return false
	}
	if isServiceShapeFactory(first) {
		// service Name: Type's extends clause, which the service's
		// declarations replace (services_declarations.go), so it imports nothing.
		return true
	}
	recordTypeUse(state, file, first)
	return true
}

// recordTypeUse records that the declaration file of file writes identifier,
// a prelude name or effect root export, so transformDeclarations imports it.
func recordTypeUse(state *preludeState, file *ast.SourceFile, identifier *ast.Node) {
	if state.typeUses[file] == nil {
		state.typeUses[file] = map[string]bool{}
	}
	state.typeUses[file][identifier.Text()] = true
	state.typeRefs[identifier] = true
}

// transformDeclarations returns the transform for the declaration file of a
// .vibe file, or nil for any other file.
func transformDeclarations(resolver printer.EmitResolver, sourceFile *ast.SourceFile) func(*ast.SourceFile) *ast.SourceFile {
	r, ok := resolver.(*checker.EmitResolver)
	if !ok || !isVibeLangFile(sourceFile) {
		return nil
	}
	return func(declarations *ast.SourceFile) *ast.SourceFile {
		rewriter := &specifierRewriter{sourceDirectory: tspath.GetDirectoryPath(string(sourceFile.FileName()))}
		keys := &computedKeyRewriter{keys: map[string]computedKey{}, bound: map[string]bool{}}
		r.WithChecker(func(c *checker.Checker) {
			if namespace := preludeSymbol(c, preludeStateOf(c), sourceFile, "Effect"); namespace != nil {
				if effect := effectInterfaceOfModule(c, namespace); effect != nil {
					rewriter.effectPath = effectModulePath(c, effect)
				}
			}
			var visit func(*ast.Node) bool
			visit = func(n *ast.Node) bool {
				// An inferred arrow may load Effect only through its callee,
				// without any prelude import in this source file.
				if rewriter.effectPath == "" && isLowerable(n) && n.Body() != nil {
					for _, signature := range c.GetSignaturesOfType(c.GetTypeAtLocation(n), checker.SignatureKindCall) {
						if path := effectModulePath(c, c.GetReturnTypeOfSignature(signature)); path != "" {
							rewriter.effectPath = path
							break
						}
					}
				}
				if ast.IsComputedPropertyName(n) {
					keys.add(c, sourceFile, n.Expression())
				}
				return n.ForEachChild(visit)
			}
			sourceFile.AsNode().ForEachChild(visit)
		})
		rewriter.NewTransformer(rewriter.visit, resolver.EmitContext())
		declarations = rewriter.TransformSourceFile(declarations)
		if len(keys.keys) > 0 {
			for _, statement := range declarations.Statements.Nodes {
				forEachTopLevelName(statement, func(name string) { keys.bound[name] = true })
			}
			keys.NewTransformer(keys.visit, resolver.EmitContext())
			declarations = keys.TransformSourceFile(declarations)
		}

		var names []string
		var refs map[*ast.Node]bool
		r.WithChecker(func(c *checker.Checker) {
			state := preludeStateOf(c)
			// S19-1: nodebuilder can recycle a written return annotation on an
			// arrow without asking entity visibility. Inspect only references
			// retained in the declaration, and resolve their original source
			// identifiers so local names and explicit imports still shadow the
			// prelude. D67 keeps the written contract; S17-1's specifier rewrite
			// above still handles inferred and linked dependency types.
			//
			// The node builder also copies a library's own type node into an
			// inferred type, such as SchemaAST.ParseOptions from effect's
			// Schema.d.ts for Schema.decodeUnknownEffect(…), when its first
			// name resolves from the .vibe file, as an effect root export or
			// prelude name does, without asking entity visibility either.
			bound := map[string]bool{}
			for _, statement := range declarations.Statements.Nodes {
				forEachTopLevelName(statement, func(name string) { bound[name] = true })
			}
			var visit func(*ast.Node) bool
			visit = func(n *ast.Node) bool {
				if name := typeEntityName(n); name != nil {
					original := resolver.EmitContext().MostOriginal(n)
					if written := typeEntityName(original); written != nil && original.Kind == n.Kind && !ast.NodeIsSynthesized(original) {
						first := ast.GetFirstIdentifier(written)
						if file := ast.GetSourceFileOfNode(original); file == sourceFile {
							preludeNameVisible(c, first, c.GetSymbolAtLocation(first))
						} else if file != nil && file != declarations && !bound[first.Text()] {
							recordLibraryTypeUse(c, state, sourceFile, first)
						}
					}
				}
				return n.ForEachChild(visit)
			}
			declarations.AsNode().ForEachChild(visit)
			for _, identifier := range keys.written {
				recordTypeUse(state, sourceFile, identifier)
			}
			for name := range state.typeUses[sourceFile] {
				names = append(names, name)
			}
			refs = state.typeRefs
		})
		if len(names) == 0 {
			return declarations
		}
		slices.Sort(names)
		// A file that imports Effect from effect and writes Effect<…> keeps
		// its import, so the prelude's Effect needs another local name.
		aliases := preludeAliases(declarations, names)
		if len(aliases) > 0 {
			renamer := &preludeRenamer{aliases: aliases, refs: refs}
			renamer.NewTransformer(renamer.visit, resolver.EmitContext())
			declarations = renamer.TransformSourceFile(declarations)
		}
		// import type { Option } from "effect"
		// import type { Effect, Scope } from "vibelang-runtime"
		f := resolver.EmitContext().Factory
		var imports []*ast.Node
		for _, module := range []string{effectRootModule, runtimeModule} {
			var specifiers []*ast.Node
			for _, name := range names {
				if _, isPrelude := preludeModules[name]; isPrelude != (module == runtimeModule) {
					continue
				}
				if alias, ok := aliases[name]; ok {
					specifiers = append(specifiers, f.NewImportSpecifier(false /*isTypeOnly*/, f.NewIdentifier(name), f.NewIdentifier(alias)))
					continue
				}
				specifiers = append(specifiers, f.NewImportSpecifier(false /*isTypeOnly*/, nil /*propertyName*/, f.NewIdentifier(name)))
			}
			if len(specifiers) > 0 {
				clause := f.NewImportClause(ast.KindTypeKeyword, nil /*name*/, f.NewNamedImports(f.NewNodeList(specifiers)))
				imports = append(imports, f.NewImportDeclaration(nil /*modifiers*/, clause, f.NewStringLiteral(module, ast.TokenFlagsNone), nil /*attributes*/))
			}
		}
		// The list keeps the source range, so the printer still emits the
		// comments before the file's first statement, such as its header, first.
		statements := f.NewNodeList(append(imports, declarations.Statements.Nodes...))
		statements.Loc = declarations.Statements.Loc
		return f.UpdateSourceFile(declarations, statements, declarations.EndOfFileToken).AsSourceFile()
	}
}

// typeEntityName returns the name that n, a type reference such as
// Effect<number> or a type query such as typeof Schema.Json.Type, writes;
// or nil.
func typeEntityName(n *ast.Node) *ast.Node {
	switch {
	case ast.IsTypeReferenceNode(n):
		return n.AsTypeReferenceNode().TypeName
	case ast.IsTypeQueryNode(n):
		return n.AsTypeQueryNode().ExprName
	}
	return nil
}

// recordLibraryTypeUse records that the declaration file of file writes
// identifier, the first name of a type node the node builder copied from a
// library's declarations, when it names the symbol that the prelude name or
// effect root export of the same name does from file.
func recordLibraryTypeUse(c *checker.Checker, state *preludeState, file *ast.SourceFile, identifier *ast.Node) {
	name := identifier.Text()
	_, isPrelude := preludeModules[name]
	_, isRoot := effectRootNames[name]
	if !isPrelude && !isRoot {
		return
	}
	resolve := func(symbol *ast.Symbol) *ast.Symbol {
		if symbol != nil && symbol.Flags()&ast.SymbolFlagsAlias != 0 {
			return c.GetAliasedSymbol(symbol)
		}
		return symbol
	}
	if want := resolve(preludeSymbol(c, state, file, name)); want != nil && want == resolve(c.GetSymbolAtLocation(identifier)) {
		recordTypeUse(state, file, identifier)
	}
}

// effectModulePath identifies Effect's defining module by its interface symbol,
// so declaration import rewriting never guesses from a package-path suffix.
func effectModulePath(c *checker.Checker, t *checker.Type) string {
	if symbol := effectInterfaceSymbol(c, t); symbol != nil {
		for _, declaration := range symbol.Declarations() {
			return tspath.RemoveFileExtension(string(ast.GetSourceFileOfNode(declaration).FileName()))
		}
	}
	return ""
}

// preludeAliases returns a free local name for each prelude name in names
// that a top-level declaration or import of declarations already binds.
func preludeAliases(declarations *ast.SourceFile, names []string) map[string]string {
	bound := map[string]bool{}
	for _, statement := range declarations.Statements.Nodes {
		forEachTopLevelName(statement, func(name string) { bound[name] = true })
	}
	used := map[string]bool{}
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if ast.IsIdentifier(node) {
			used[node.Text()] = true
		}
		return node.ForEachChild(visit)
	}
	declarations.AsNode().ForEachChild(visit)
	aliases := map[string]string{}
	for _, name := range names {
		if !bound[name] {
			continue
		}
		for i := 1; ; i++ {
			if alias := name + "_" + strconv.Itoa(i); !bound[alias] && !used[alias] {
				aliases[name] = alias
				used[alias] = true
				break
			}
		}
	}
	return aliases
}

// preludeRenamer gives the identifiers that write a prelude name the alias
// the declaration file imports it under.
type preludeRenamer struct {
	transformers.Transformer
	aliases map[string]string
	refs    map[*ast.Node]bool // the identifiers that preludeNameVisible allowed
}

func (tx *preludeRenamer) visit(node *ast.Node) *ast.Node {
	if ast.IsIdentifier(node) {
		if alias, ok := tx.aliases[node.Text()]; ok && tx.refs[tx.EmitContext().MostOriginal(node)] {
			return tx.Factory().NewIdentifier(alias)
		}
		return node
	}
	return tx.Visitor().VisitEachChild(node)
}

// specifierRewriter makes a declaration file name modules by output: the
// specifier of an import or export declaration, and the argument of an
// import type that the checker wrote for an inferred type, such as
// import("./users.vibe").NotFound. Effect's physical path is named by its
// public module even when a symlink led the checker outside node_modules.
type specifierRewriter struct {
	transformers.Transformer
	sourceDirectory string
	effectPath      string // extensionless path of the resolved Effect interface's module
}

func (tx *specifierRewriter) visit(node *ast.Node) *ast.Node {
	f := tx.Factory()
	switch node.Kind {
	case ast.KindImportDeclaration:
		if platformImport(node) != nil {
			return nil // a platform import loads nothing (D312, entry.go)
		}
		d := node.AsImportDeclaration()
		if specifier, changed := outputSpecifier(f, d.ModuleSpecifier); changed {
			return f.UpdateImportDeclaration(d, d.Modifiers(), d.ImportClause, specifier, d.Attributes)
		}
		return node
	case ast.KindExportDeclaration:
		d := node.AsExportDeclaration()
		if specifier, changed := outputSpecifier(f, d.ModuleSpecifier); changed {
			return f.UpdateExportDeclaration(d, d.Modifiers(), d.IsTypeOnly, d.ExportClause, specifier, d.Attributes)
		}
		return node
	case ast.KindImportType:
		visited := tx.Visitor().VisitEachChild(node).AsImportTypeNode()
		if argument := visited.Argument; argument != nil && ast.IsLiteralTypeNode(argument) {
			literal := argument.AsLiteralTypeNode().Literal
			// Match the resolved interface's file, not a package-path suffix:
			// only that module is known to have the public name effect/Effect.
			if ast.IsStringLiteral(literal) && tx.effectPath != "" && tspath.IsExternalModuleNameRelative(literal.Text()) && tspath.RemoveFileExtension(tspath.ResolvePath(tx.sourceDirectory, literal.Text())) == tx.effectPath {
				specifier := f.NewStringLiteral("effect/Effect", ast.TokenFlagsNone)
				return f.UpdateImportTypeNode(visited, visited.IsTypeOf, f.UpdateLiteralTypeNode(argument.AsLiteralTypeNode(), specifier), visited.Attributes, visited.Qualifier, visited.TypeArguments)
			}
			if specifier, changed := outputSpecifier(f, literal); changed {
				return f.UpdateImportTypeNode(visited, visited.IsTypeOf, f.UpdateLiteralTypeNode(argument.AsLiteralTypeNode(), specifier), visited.Attributes, visited.Qualifier, visited.TypeArguments)
			}
		}
		return visited.AsNode()
	}
	return tx.Visitor().VisitEachChild(node)
}

// outputSpecifier returns a string literal that names the output of the
// source specifier names, and whether it differs from specifier.
func outputSpecifier(f *printer.NodeFactory, specifier *ast.Node) (*ast.Node, bool) {
	if specifier == nil || !ast.IsStringLiteral(specifier) {
		return specifier, false
	}
	text, changed := outputSpecifierText(specifier.Text())
	if !changed {
		return specifier, false
	}
	return f.NewStringLiteral(text, specifier.AsStringLiteral().TokenFlags), true
}

// computedKeyRewriter writes a computed key that the source took from the
// prelude or effect's root as the source wrote it, and records its first name
// for import:
//
//	// main.vibe                               // main.d.ts
//	export const Even = {                        import type { customMatcher } from "vibelang-runtime";
//	  [customMatcher](x: unknown) { … }          export declare const Even: {
//	}                                                [customMatcher](x: unknown): boolean;
//	                                             };
//
// The node builder prints a unique symbol key by its declared name alone,
// [customMatcher] or [NodeInspectSymbol], because no scope it searches holds
// the prelude, and it never asks preludeNameVisible.
type computedKeyRewriter struct {
	transformers.Transformer
	keys    map[string]computedKey // by the declared name of the key's symbol
	bound   map[string]bool        // names the declaration file declares at its top level
	written []*ast.Node            // the first identifier of each key it wrote
}

// computedKey is a key the source writes, such as Inspectable.NodeInspectSymbol.
// Two keys whose symbols share a declared name make it ambiguous.
type computedKey struct {
	expression *ast.Node
	symbol     *ast.Symbol
	ambiguous  bool
}

// add records key, a computed property name's expression in file, when its
// first name resolves to the prelude or effect's root.
func (tx *computedKeyRewriter) add(c *checker.Checker, file *ast.SourceFile, key *ast.Node) {
	if !ast.IsEntityNameExpression(key) {
		return
	}
	first := ast.GetFirstIdentifier(key)
	_, isPrelude := preludeModules[first.Text()]
	_, isRoot := effectRootNames[first.Text()]
	if !isPrelude && !isRoot {
		return
	}
	if prelude := preludeSymbol(c, preludeStateOf(c), file, first.Text()); prelude == nil || c.GetSymbolAtLocation(first) != prelude {
		return
	}
	symbol := c.GetSymbolAtLocation(key)
	if symbol == nil {
		return
	}
	if symbol.Flags()&ast.SymbolFlagsAlias != 0 {
		symbol = c.GetAliasedSymbol(symbol)
	}
	if existing, ok := tx.keys[symbol.Name()]; ok {
		existing.ambiguous = existing.ambiguous || existing.symbol != symbol
		tx.keys[symbol.Name()] = existing
		return
	}
	tx.keys[symbol.Name()] = computedKey{expression: key, symbol: symbol}
}

// visit rewrites a key printed as a bare name that the declaration file does
// not declare, so a local binding with the same name keeps its key.
func (tx *computedKeyRewriter) visit(node *ast.Node) *ast.Node {
	if ast.IsComputedPropertyName(node) {
		name := node.Expression()
		if !ast.IsIdentifier(name) || tx.bound[name.Text()] {
			return node
		}
		if key, ok := tx.keys[name.Text()]; ok && !key.ambiguous {
			return tx.Factory().UpdateComputedPropertyName(node.AsComputedPropertyName(), tx.entityExpression(key.expression))
		}
		return node
	}
	return tx.Visitor().VisitEachChild(node)
}

// entityExpression returns a new copy of key, an identifier or a property
// access of one, and keeps its first identifier in written.
func (tx *computedKeyRewriter) entityExpression(key *ast.Node) *ast.Node {
	f := tx.Factory()
	if ast.IsPropertyAccessExpression(key) {
		return f.NewPropertyAccessExpression(tx.entityExpression(key.Expression()), nil /*questionDotToken*/, f.NewIdentifier(key.Name().Text()), ast.NodeFlagsNone)
	}
	identifier := f.NewIdentifier(key.Text())
	tx.written = append(tx.written, identifier)
	return identifier
}
