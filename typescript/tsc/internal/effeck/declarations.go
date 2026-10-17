package effeck

import (
	"slices"
	"strconv"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/printer"
	"github.com/microsoft/TypeScript/tsc/internal/transformers"
)

// The declaration file of a .effeck file is ordinary TypeScript, so it must import
// what the source took from the prelude and name its imports by output:
//
//	// main.effeck                               // main.d.ts
//	import { find } from "./users.effeck"        import type { Effect } from "@effeck/runtime";
//	export const one: Effect<number> = …     import { find } from "./users.js";
//	                                         export declare const one: Effect<number>;
//
// Declaration emit asks whether each name a declaration writes is visible;
// preludeNameVisible says yes for a prelude name and records it, and
// transformDeclarations imports the recorded names and rewrites specifiers.

// preludeNameVisible lets the declaration file of a .effeck file write a prelude
// name that the source wrote, such as Effect in Effect<number>.
func preludeNameVisible(c *checker.Checker, entityName *ast.Node, symbol *ast.Symbol) bool {
	first := ast.GetFirstIdentifier(entityName)
	name := first.Text()
	if _, ok := preludeModules[name]; !ok {
		return false
	}
	file := ast.GetSourceFileOfNode(entityName)
	if !isEffeckFile(file) {
		return false
	}
	state := preludeStateOf(c)
	module := runtimeModuleSymbol(c, state, file)
	if module == nil || c.TryGetMemberInModuleExports(name, module) != symbol {
		return false
	}
	if state.typeUses[file] == nil {
		state.typeUses[file] = map[string]bool{}
	}
	state.typeUses[file][name] = true
	state.typeRefs[first] = true
	return true
}

// transformDeclarations returns the transform for the declaration file of a
// .effeck file, or nil for any other file.
func transformDeclarations(resolver printer.EmitResolver, sourceFile *ast.SourceFile) func(*ast.SourceFile) *ast.SourceFile {
	r, ok := resolver.(*checker.EmitResolver)
	if !ok || !isEffeckFile(sourceFile) {
		return nil
	}
	return func(declarations *ast.SourceFile) *ast.SourceFile {
		rewriter := &specifierRewriter{}
		rewriter.NewTransformer(rewriter.visit, resolver.EmitContext())
		declarations = rewriter.TransformSourceFile(declarations)

		var names []string
		var refs map[*ast.Node]bool
		r.WithChecker(func(c *checker.Checker) {
			state := preludeStateOf(c)
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
		// import type { Effect, Scope } from "@effeck/runtime"
		f := resolver.EmitContext().Factory
		specifiers := make([]*ast.Node, 0, len(names))
		for _, name := range names {
			if alias, ok := aliases[name]; ok {
				specifiers = append(specifiers, f.NewImportSpecifier(false /*isTypeOnly*/, f.NewIdentifier(name), f.NewIdentifier(alias)))
				continue
			}
			specifiers = append(specifiers, f.NewImportSpecifier(false /*isTypeOnly*/, nil /*propertyName*/, f.NewIdentifier(name)))
		}
		clause := f.NewImportClause(ast.KindTypeKeyword, nil /*name*/, f.NewNamedImports(f.NewNodeList(specifiers)))
		statements := append([]*ast.Node{f.NewImportDeclaration(nil /*modifiers*/, clause, f.NewStringLiteral(runtimeModule, ast.TokenFlagsNone), nil /*attributes*/)}, declarations.Statements.Nodes...)
		return f.UpdateSourceFile(declarations, f.NewNodeList(statements), declarations.EndOfFileToken).AsSourceFile()
	}
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
// import("./users.effeck").NotFound.
type specifierRewriter struct {
	transformers.Transformer
}

func (tx *specifierRewriter) visit(node *ast.Node) *ast.Node {
	f := tx.Factory()
	switch node.Kind {
	case ast.KindImportDeclaration:
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
			if specifier, changed := outputSpecifier(f, argument.AsLiteralTypeNode().Literal); changed {
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
