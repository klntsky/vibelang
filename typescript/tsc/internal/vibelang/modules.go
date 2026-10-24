package vibelang

import (
	"slices"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
	"github.com/microsoft/TypeScript/tsc/internal/printer"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
	"github.com/microsoft/TypeScript/tsc/internal/transformers"
)

// Module declarations (D283).
//
//	module text {
//	  export function clean(value: string): string { return value.trim() }
//	}
//	import { clean } from text
//
// The parser gives each module declaration's body a TypeScript source file
// of its own, beside the .vibe file and with its positions
// (parser/vibelang_modules.go), and the program resolves `from text` to it.
// The body is therefore checked as any .ts file is, and the .vibe file sees
// it only through its imports.
//
// The emitted JavaScript holds the module inline (D303): a module that the
// file imports becomes a function that returns the module's exports, called
// once before the file's own code, and its imports move to the file's top
// level as namespace imports:
//
//	import * as node_path_1 from "node:path"
//	const text_1 = (() => {
//	  function clean(value) { return node_path_1.basename(value.trim()) }
//	  return moduleRuntime_1.$freeze({ __proto__: null, clean: clean })
//	})()
//	const { clean } = text_1
//
// One .vibe file is still one JavaScript module, so the Node loader and the
// bundler plugins load it as before, import.meta is the file's, and relative
// imports resolve from the file's directory, as TC39's module declarations
// specify. Imports the body uses only as types, and a module that the file
// imports only types from, emit nothing, as for a .ts file. The module's
// declarations are its own declaration file, main.vibe.text.d.ts, which the
// .vibe file's declarations import as ./main.vibe.text.js.

func init() {
	fileChecks = append(fileChecks, checkModuleDeclarations)
	typeScriptFileChecks = append(typeScriptFileChecks, checkModuleBody)
}

// checkModuleDeclarations reports misplaced and misspelled module
// declarations (EK4408) and module specifiers that name no module
// declaration of file (EK4407).
func checkModuleDeclarations(c *checker.Checker, file *ast.SourceFile) {
	seen := map[string]bool{}
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		switch {
		case parser.IsVibeLangModuleDeclaration(node):
			// TypeScript's own checks read the node as a namespace declared with module.
			cover(c, node)
			name := node.Name().Text()
			key := strings.ToLower(name)
			switch {
			case node.Parent != file.AsNode():
				report(c, node.Name(), EK4408.With("A module declaration stands at the top level of a file."))
			case node.Modifiers() != nil && len(node.Modifiers().Nodes) != 0:
				report(c, node.Modifiers().Nodes[0], EK4408.With("A module declaration is not exported; export what it exports with export { … } from {0}."), name)
			case seen[key]:
				report(c, node.Name(), EK4408.With("This file already declares a module named {0}."), name)
			}
			seen[key] = true
			if symbol := c.ResolveName(name, node, ast.SymbolFlagsValue, true); symbol != nil &&
				c.GetSymbolFlags(symbol)&ast.SymbolFlagsValue != 0 && c.GetTypeOnlyAliasDeclaration(symbol) == nil {
				// Unused modules with a colliding value already run correctly. Report
				// the collision without rejecting those programs (D303).
				collision := EK4408.With("A module named {0} shares its name with a value binding of this file; use a distinct module name.")
				collision.Category = warningCategory
				report(c, node.Name(), collision, name)
			}
			return false
		case node.Kind == ast.KindModuleDeclaration && node.AsModuleDeclaration().Keyword == ast.KindModuleKeyword && node.Name() != nil && ast.IsIdentifier(node.Name()):
			// declare module x { }, module a.b { }: in a .vibe file module is never a namespace.
			cover(c, node.Name())
			report(c, node.Name(), EK4408.With("In a .vibe file, module declares a module whose body is TypeScript: module {0} { … }. Write a namespace with namespace."), node.Name().Text())
			return false
		case parser.IsVibeLangModuleReference(node) && file.EmbeddedModuleBySpecifier(node.Text()) == nil:
			cover(c, node)
			report(c, node, EK4407, specifierName(file, node))
			return false
		}
		node.ForEachChild(visit)
		return false
	}
	file.AsNode().ForEachChild(visit)
}

// specifierName returns the identifier a module reference was written as.
func specifierName(file *ast.SourceFile, reference *ast.Node) string {
	return file.Text()[scanner.SkipTrivia(file.Text(), reference.Pos()):reference.End()]
}

// checkModuleBody reports what a module declaration's body cannot hold
// (EK4408): it is an ES module, as TC39's module declarations are. Where
// TypeScript emits ES modules it reports export = and import = require()
// itself (TS1203, TS1202); EK4408 covers a CommonJS emit.
func checkModuleBody(c *checker.Checker, file *ast.SourceFile) {
	if file.EmbeddedModule() == nil {
		return
	}
	kind := c.CompilerOptions().GetEmitModuleKind()
	if kind >= core.ModuleKindES2015 && kind != core.ModuleKindPreserve && c.Program().GetImpliedNodeFormatForEmit(file) != core.ModuleKindCommonJS {
		return
	}
	for _, statement := range file.Statements.Nodes {
		var text string
		switch {
		case ast.IsExportAssignment(statement) && statement.AsExportAssignment().IsExportEquals:
			text = "A module declaration's body is an ES module, which export = cannot end; use export default."
		case ast.IsImportEqualsDeclaration(statement) && ast.IsExternalModuleReference(statement.AsImportEqualsDeclaration().ModuleReference):
			text = "A module declaration's body is an ES module, which imports with import, not import = require()."
		default:
			continue
		}
		// D266 keeps EK diagnostics out of TypeScript files; a module body is part of its .vibe file.
		c.AddDiagnostic(ast.NewExternalDiagnostic(file, scanner.GetErrorRangeForNode(file, statement), diagnosticSource, errorCategory, EK4408.Code, text))
	}
}

// moduleInliner writes the module declarations a .vibe file imports into the
// file's JavaScript. It runs after the lowering, before TypeScript's
// transforms, which then erase the bodies' types as they do the file's.
type moduleInliner struct {
	transformers.Transformer
	opts     *transformers.TransformOptions
	resolver printer.EmitResolver
	file     *ast.SourceFile // the .vibe file's parse tree
	modules  []*inlineModule
	verbatim bool      // verbatimModuleSyntax: imports are kept unless type only
	cjs      bool      // the file is emitted as CommonJS
	runtime  *ast.Node // the emitter-owned runtime namespace for module helpers
}

// inlineModule is one module declaration being written inline.
type inlineModule struct {
	file    *ast.SourceFile
	holder  *ast.Node               // a unique name, never the module's source-level name
	needed  bool                    // the .vibe file imports a value from it, or imports it for its effects
	imports map[*ast.Node]*ast.Node // import binding declarations of the body, by the statement whose generated name holds their module
	locals  map[string]*ast.Node    // import binding declarations of the body, by name
	hoisted map[string]*ast.Node    // the first import of each specifier, whose generated name holds that module
}

func newModuleInliner(opts *transformers.TransformOptions, file *ast.SourceFile) *transformers.Transformer {
	if !isVibeLangFile(file) || len(file.EmbeddedModules()) == 0 || opts.EmitResolver == nil {
		return nil
	}
	tx := &moduleInliner{
		opts:     opts,
		resolver: opts.EmitResolver,
		file:     file,
		verbatim: opts.CompilerOptions.VerbatimModuleSyntax.IsTrue(),
		cjs:      opts.GetEmitModuleFormatOfFile != nil && opts.GetEmitModuleFormatOfFile(file) == core.ModuleKindCommonJS,
	}
	for _, module := range file.EmbeddedModules() {
		tx.modules = append(tx.modules, &inlineModule{file: module, imports: map[*ast.Node]*ast.Node{}, locals: map[string]*ast.Node{}, hoisted: map[string]*ast.Node{}})
	}
	return tx.NewTransformer(tx.visit, opts.Context)
}

func (tx *moduleInliner) visit(node *ast.Node) *ast.Node {
	if node.Kind != ast.KindSourceFile {
		return node
	}
	tx.resolver.MarkLinkedReferencesRecursively(tx.file)
	for _, module := range tx.modules {
		tx.resolver.MarkLinkedReferencesRecursively(module.file)
	}
	file := node.AsSourceFile()
	var bindings, rest []*ast.Node
	var moved *ast.Node // the file's first statement, when it does not stay where it was
	for _, statement := range file.Statements.Nodes {
		original := tx.EmitContext().MostOriginal(statement)
		switch {
		case parser.IsVibeLangModuleDeclaration(original):
			// The declaration itself emits nothing; its module is written below.
		case ast.IsImportDeclaration(statement) && tx.moduleOf(statement) != nil:
			bindings = append(bindings, tx.importBindings(original, tx.moduleOf(statement))...)
		case ast.IsExportDeclaration(statement) && tx.moduleOf(statement) != nil:
			bindings = append(bindings, tx.reexports(original, tx.moduleOf(statement))...)
		default:
			rest = append(rest, statement)
			continue
		}
		if original == tx.file.Statements.Nodes[0] {
			moved = original
		}
	}
	var head []*ast.Node
	for _, module := range tx.modules {
		if module.needed {
			head = append(head, tx.inline(module)...)
		}
	}
	if tx.runtime != nil {
		f := tx.Factory()
		declaration := f.NewImportDeclaration(nil,
			f.NewImportClause(ast.KindUnknown, nil, f.NewNamespaceImport(tx.runtime.Clone(f))),
			f.NewStringLiteral("vibelang-runtime", ast.TokenFlagsNone), nil)
		ast.SetParentInChildren(declaration)
		head = append([]*ast.Node{declaration}, head...)
	}
	if moved != nil {
		// The comments before the file's first statement, such as its header, stay first.
		if top := append(append([]*ast.Node{}, head...), bindings...); len(top) != 0 {
			tx.EmitContext().SetCommentRange(top[0], core.NewTextRange(moved.Pos(), scanner.SkipTrivia(tx.file.Text(), moved.Pos())))
		}
	}
	statements := tx.Factory().NewNodeList(append(append(head, bindings...), rest...))
	statements.Loc = file.Statements.Loc
	return tx.Factory().UpdateSourceFile(file, statements, file.EndOfFileToken)
}

// moduleOf returns the module declaration that statement imports from or
// exports from, or nil.
func (tx *moduleInliner) moduleOf(statement *ast.Node) *inlineModule {
	specifier := statement.ModuleSpecifier()
	if specifier == nil || !ast.IsStringLiteral(specifier) || specifier.Flags&ast.NodeFlagsVibeLang == 0 {
		return nil
	}
	for _, module := range tx.modules {
		if module.file.EmbeddedModule().Specifier == specifier.Text() {
			return module
		}
	}
	return nil
}

// keepsImport reports whether an import binding of the .vibe file or of a
// module body stays in the JavaScript: TypeScript's import elision.
func (tx *moduleInliner) keepsImport(declaration *ast.Node) bool {
	if declaration.IsTypeOnly() {
		return false
	}
	return tx.verbatim || tx.resolver.IsReferencedAliasDeclaration(declaration)
}

// keepsExport reports whether an export specifier or default export names a
// value, which the JavaScript exports.
func (tx *moduleInliner) keepsExport(declaration *ast.Node) bool {
	if declaration.IsTypeOnly() {
		return false
	}
	return tx.verbatim || tx.resolver.IsValueAliasDeclaration(declaration)
}

// importBindings turns an import from a module declaration into the
// constants it binds: const { clean, default: base } = text_1. In CommonJS,
// TypeScript's module transform reads an imported name as a property of the
// import's generated name, so that name holds the module instead.
func (tx *moduleInliner) importBindings(declaration *ast.Node, module *inlineModule) []*ast.Node {
	f := tx.Factory()
	clause := declaration.AsImportDeclaration().ImportClause
	if clause == nil {
		module.needed = true // import text;
		return nil
	}
	if clause.IsTypeOnly() {
		return nil
	}
	var declarations []*ast.Node
	var elements []*ast.Node
	if name := clause.Name(); name != nil && tx.keepsImport(clause) {
		elements = append(elements, f.NewBindingElement(nil, f.NewIdentifier("default"), f.NewIdentifier(name.Text()), nil))
	}
	if bindings := clause.AsImportClause().NamedBindings; bindings != nil {
		switch bindings.Kind {
		case ast.KindNamespaceImport:
			if tx.keepsImport(bindings) {
				declarations = append(declarations, f.NewVariableDeclaration(f.NewIdentifier(bindings.Name().Text()), nil, nil, module.name(f)))
				module.needed = true
			}
		case ast.KindNamedImports:
			for _, specifier := range bindings.Elements() {
				if !tx.keepsImport(specifier) {
					continue
				}
				var property *ast.Node
				if specifier.PropertyName() != nil {
					property = cloneName(f, specifier.PropertyName())
				}
				elements = append(elements, f.NewBindingElement(nil, property, f.NewIdentifier(specifier.Name().Text()), nil))
			}
		}
	}
	if len(elements) != 0 {
		module.needed = true
		if tx.cjs {
			declarations = append(declarations, f.NewVariableDeclaration(f.NewGeneratedNameForNode(declaration), nil, nil, module.name(f)))
		} else {
			declarations = append(declarations, f.NewVariableDeclaration(f.NewBindingPattern(ast.KindObjectBindingPattern, f.NewNodeList(elements)), nil, nil, module.name(f)))
		}
	}
	if len(declarations) == 0 {
		return nil
	}
	return []*ast.Node{constStatement(f, declarations)}
}

// reexports turns an export from a module declaration into constants and an
// export of them: export { clean } from text.
func (tx *moduleInliner) reexports(declaration *ast.Node, module *inlineModule) []*ast.Node {
	f := tx.Factory()
	d := declaration.AsExportDeclaration()
	if d.IsTypeOnly {
		return nil
	}
	moduleName := func() *ast.Node { return module.name(f) }
	var names []*ast.Node  // the exported names
	var values []*ast.Node // their values
	switch {
	case d.ExportClause == nil: // export * from text
		for _, name := range tx.starExports(module.file, nil) {
			names = append(names, exportName(f, name))
			values = append(values, member(f, moduleName(), name))
		}
	case d.ExportClause.Kind == ast.KindNamespaceExport: // export * as text from text
		names = append(names, cloneName(f, d.ExportClause.Name()))
		values = append(values, moduleName())
	default:
		for _, specifier := range d.ExportClause.Elements() {
			if !tx.keepsExport(specifier) {
				continue
			}
			names = append(names, cloneName(f, specifier.Name()))
			values = append(values, member(f, moduleName(), moduleExportNameText(specifier.PropertyNameOrName())))
		}
	}
	module.needed = true
	if len(names) == 0 {
		return nil
	}
	if tx.cjs {
		// These exports were synthesized after binding. TypeScript's CommonJS
		// transform cannot connect their locals to the original export symbols.
		var statements []*ast.Node
		for i, name := range names {
			var value *ast.Node
			if name.Text() == "__proto__" {
				// Assignment would invoke Object.prototype's inherited setter.
				properties := []*ast.Node{
					f.NewPropertyAssignment(nil, f.NewIdentifier("value"), nil, nil, values[i]),
				}
				for _, flag := range []string{"enumerable", "configurable", "writable"} {
					properties = append(properties, f.NewPropertyAssignment(nil, f.NewIdentifier(flag), nil, nil, f.NewKeywordExpression(ast.KindTrueKeyword)))
				}
				value = f.NewCallExpression(tx.runtimeMember("$defineProperty"), nil, nil, f.NewNodeList([]*ast.Node{
					f.NewIdentifier("exports"), f.NewStringLiteral(name.Text(), ast.TokenFlagsNone),
					f.NewObjectLiteralExpression(f.NewNodeList(properties), false),
				}), ast.NodeFlagsNone)
			} else {
				value = f.NewAssignmentExpression(member(f, f.NewIdentifier("exports"), name.Text()), values[i])
			}
			statements = append(statements, f.NewExpressionStatement(value))
		}
		return statements
	}
	var declarations, specifiers []*ast.Node
	for i, name := range names {
		local := f.NewUniqueName(exportNameText(name))
		declarations = append(declarations, f.NewVariableDeclaration(local, nil, nil, values[i]))
		specifiers = append(specifiers, f.NewExportSpecifier(false, local, name))
	}
	return []*ast.Node{
		constStatement(f, declarations),
		f.NewExportDeclaration(nil, false, f.NewNamedExports(f.NewNodeList(specifiers)), nil, nil),
	}
}

// runtimeMember returns an intrinsic captured outside the user's scopes.
func (tx *moduleInliner) runtimeMember(name string) *ast.Node {
	f := tx.Factory()
	if tx.runtime == nil {
		tx.runtime = f.NewUniqueName("moduleRuntime")
	}
	return member(f, tx.runtime.Clone(f), name)
}

// name returns the unique name that holds the module's exports. The source
// name is only a module specifier, never a value binding in the emitted file.
func (module *inlineModule) name(f *printer.NodeFactory) *ast.Node {
	if module.holder == nil {
		module.holder = f.NewUniqueName(module.file.EmbeddedModule().Declaration.Name().Text())
	}
	return module.holder.Clone(f)
}

// inline returns the statements that evaluate module: its imports, moved to
// the file's top level, and the constant that holds its exports.
func (tx *moduleInliner) inline(module *inlineModule) []*ast.Node {
	f := tx.Factory()
	var hoisted []*ast.Node
	statements := module.file.Statements.Nodes
	// Imports first: the body may use a name before the import that binds it.
	for _, statement := range statements {
		if ast.IsImportDeclaration(statement) {
			hoisted = append(hoisted, tx.hoistImport(module, statement)...)
		}
	}
	visitor := tx.EmitContext().NewNodeVisitor(nil)
	visitor.Visit = func(node *ast.Node) *ast.Node { return tx.visitBody(module, visitor, node) }
	var body []*ast.Node
	var properties []*ast.Node
	export := func(name string, value *ast.Node) {
		property := propertyName(f, name)
		if name == "__proto__" {
			// An export is an own data property, not the object literal's prototype setter.
			property = f.NewComputedPropertyName(f.NewStringLiteral(name, ast.TokenFlagsNone))
		}
		properties = append(properties, f.NewPropertyAssignment(nil, property, nil, nil, value))
	}
	for _, statement := range statements {
		switch statement.Kind {
		case ast.KindImportDeclaration:
			continue
		case ast.KindExportDeclaration:
			hoisted = append(hoisted, tx.moduleExports(module, statement, export)...)
			continue
		case ast.KindExportAssignment:
			if statement.AsExportAssignment().IsExportEquals || !tx.keepsExport(statement) {
				continue
			}
			value := f.NewUniqueName("default")
			declaration := constStatement(f, []*ast.Node{f.NewVariableDeclaration(value, nil, nil, visitor.VisitNode(statement.Expression()))})
			tx.EmitContext().SetOriginal(declaration, statement)
			declaration.Loc = statement.Loc
			body = append(body, declaration)
			export("default", value.Clone(f))
			continue
		}
		if !ast.HasSyntacticModifier(statement, ast.ModifierFlagsExport) {
			body = append(body, visitor.VisitNode(statement))
			continue
		}
		local, names := tx.unexported(module, visitor, statement)
		body = append(body, local)
		for _, name := range names {
			value := name.value
			export(name.exported, value)
		}
	}
	properties = append([]*ast.Node{f.NewPropertyAssignment(nil, f.NewIdentifier("__proto__"), nil, nil, f.NewKeywordExpression(ast.KindNullKeyword))}, properties...)
	freeze := f.NewCallExpression(
		tx.runtimeMember("$freeze"),
		nil, nil, f.NewNodeList([]*ast.Node{f.NewObjectLiteralExpression(f.NewNodeList(properties), true)}), ast.NodeFlagsNone)
	body = append(body, f.NewReturnStatement(freeze))
	var modifiers *ast.ModifierList
	awaits := containsTopLevelAwait(module.file)
	if awaits {
		modifiers = f.NewModifierList([]*ast.Node{f.NewModifier(ast.KindAsyncKeyword)})
	}
	function := f.NewArrowFunction(modifiers, nil, f.NewNodeList(nil), nil, nil, f.NewToken(ast.KindEqualsGreaterThanToken), f.NewBlock(f.NewNodeList(body), true))
	tx.EmitContext().AddEmitFlags(function, printer.EFNoSourcePosition) // code no source stands for (generated.go)
	var value *ast.Node = f.NewCallExpression(f.NewParenthesizedExpression(function), nil, nil, f.NewNodeList(nil), ast.NodeFlagsNone)
	if awaits {
		value = f.NewAwaitExpression(value)
	}
	holder := constStatement(f, []*ast.Node{f.NewVariableDeclaration(module.name(f), nil, nil, value)})
	return append(hoisted, holder)
}

// hoistImport returns the top-level import that stands for an import of a
// module body: `import * as node_path_1 from "node:path"`, whose bindings the
// body reads as members of node_path_1. An import whose bindings the body
// uses only as types emits nothing.
func (tx *moduleInliner) hoistImport(module *inlineModule, statement *ast.Node) []*ast.Node {
	f := tx.Factory()
	d := statement.AsImportDeclaration()
	clause := d.ImportClause
	if clause == nil {
		return []*ast.Node{f.NewImportDeclaration(nil, nil, d.ModuleSpecifier, d.Attributes)} // import "./setup.ts"
	}
	if clause.IsTypeOnly() {
		return nil
	}
	used := false
	bind := func(declaration *ast.Node, name *ast.Node) {
		if tx.keepsImport(declaration) {
			module.imports[declaration] = statement
			module.locals[name.Text()] = declaration
			used = true
		}
	}
	if clause.Name() != nil {
		bind(clause, clause.Name())
	}
	if bindings := clause.AsImportClause().NamedBindings; bindings != nil {
		if bindings.Kind == ast.KindNamespaceImport {
			bind(bindings, bindings.Name())
		} else {
			for _, specifier := range bindings.Elements() {
				bind(specifier, specifier.Name())
			}
		}
	}
	if !used {
		return nil
	}
	phase := clause.AsImportClause().PhaseModifier
	if phase == ast.KindUnknown && d.Attributes == nil {
		// One namespace import per specifier: later imports of it read the first's.
		key := d.ModuleSpecifier.Text()
		if first := module.hoisted[key]; first != nil {
			for declaration, owner := range module.imports {
				if owner == statement {
					module.imports[declaration] = first
				}
			}
			return nil
		}
		module.hoisted[key] = statement
	}
	if phase == ast.KindSourceKeyword {
		// import source wasm from "./app.wasm" binds the source itself.
		return []*ast.Node{f.NewImportDeclaration(nil, f.NewImportClause(phase, f.NewGeneratedNameForNode(statement), nil), d.ModuleSpecifier, d.Attributes)}
	}
	return []*ast.Node{f.NewImportDeclaration(nil, f.NewImportClause(phase, nil, f.NewNamespaceImport(f.NewGeneratedNameForNode(statement))), d.ModuleSpecifier, d.Attributes)}
}

// importReference returns what a reference to an import binding of the body
// becomes, node_path_1.basename, or nil when name refers to no such binding.
func (tx *moduleInliner) importReference(module *inlineModule, name *ast.Node) *ast.Node {
	declaration := tx.resolver.GetReferencedImportDeclaration(name)
	statement := module.imports[declaration]
	if statement == nil {
		return nil
	}
	return tx.bindingValue(declaration, statement, name)
}

// bindingValue is the value of the import binding declaration of statement,
// standing at at.
func (tx *moduleInliner) bindingValue(declaration *ast.Node, statement *ast.Node, at *ast.Node) *ast.Node {
	f := tx.Factory()
	namespace := f.NewGeneratedNameForNode(statement)
	var reference *ast.Node
	switch declaration.Kind {
	case ast.KindImportSpecifier:
		reference = member(f, namespace, moduleExportNameText(declaration.PropertyNameOrName()))
	case ast.KindImportClause:
		if declaration.AsImportClause().PhaseModifier == ast.KindSourceKeyword {
			reference = namespace
		} else {
			reference = member(f, namespace, "default")
		}
	default: // a namespace import
		reference = namespace
	}
	if at != nil {
		tx.EmitContext().AssignCommentAndSourceMapRanges(reference, at)
		reference.Loc = at.Loc
	}
	return reference
}

// visitBody rewrites the body's references to its import bindings.
func (tx *moduleInliner) visitBody(module *inlineModule, visitor *ast.NodeVisitor, node *ast.Node) *ast.Node {
	f := tx.Factory()
	switch node.Kind {
	case ast.KindIdentifier:
		if node.Parent != nil && transformers.IsIdentifierReference(node, node.Parent) {
			if reference := tx.importReference(module, node); reference != nil {
				return reference
			}
		}
		return node
	case ast.KindShorthandPropertyAssignment:
		if reference := tx.importReference(module, node.Name()); reference != nil {
			property := f.NewPropertyAssignment(nil, f.NewIdentifier(node.Name().Text()), nil, nil, reference)
			tx.EmitContext().SetOriginal(property, node)
			property.Loc = node.Loc
			return property
		}
	case ast.KindCallExpression, ast.KindTaggedTemplateExpression:
		// basename(x) calls node_path_1.basename with no receiver: (0, node_path_1.basename)(x).
		var callee *ast.Node
		if node.Kind == ast.KindCallExpression {
			callee = node.Expression()
		} else {
			callee = node.AsTaggedTemplateExpression().Tag
		}
		if ast.IsIdentifier(callee) {
			if reference := tx.importReference(module, callee); reference != nil && !ast.IsIdentifier(reference) {
				unbound := f.NewParenthesizedExpression(f.NewCommaExpression(f.NewNumericLiteral("0", ast.TokenFlagsNone), reference))
				if node.Kind == ast.KindCallExpression {
					call := node.AsCallExpression()
					return f.UpdateCallExpression(call, unbound, call.QuestionDotToken, visitor.VisitNodes(call.TypeArguments), visitor.VisitNodes(call.Arguments), call.Flags)
				}
				tagged := node.AsTaggedTemplateExpression()
				return f.UpdateTaggedTemplateExpression(tagged, unbound, tagged.QuestionDotToken, visitor.VisitNodes(tagged.TypeArguments), visitor.VisitNode(tagged.Template), tagged.Flags)
			}
		}
	}
	return visitor.VisitEachChild(node)
}

// exportedName is one export of a module body: its exported name and value.
type exportedName struct {
	exported string
	value    *ast.Node
}

// unexported returns statement, an exported declaration of a module body,
// without export, and the names it exports.
func (tx *moduleInliner) unexported(module *inlineModule, visitor *ast.NodeVisitor, statement *ast.Node) (*ast.Node, []exportedName) {
	f := tx.Factory()
	visited := visitor.VisitNode(statement)
	modifiers := statement.Modifiers()
	isDefault := ast.HasSyntacticModifier(statement, ast.ModifierFlagsDefault)
	var kept []*ast.Node
	for _, modifier := range modifiers.Nodes {
		if modifier.Kind != ast.KindExportKeyword && modifier.Kind != ast.KindDefaultKeyword {
			kept = append(kept, modifier)
		}
	}
	var keptList *ast.ModifierList
	if len(kept) != 0 {
		keptList = f.NewModifierList(kept)
	}
	ambient := ast.HasSyntacticModifier(statement, ast.ModifierFlagsAmbient) || statement.Flags&ast.NodeFlagsAmbient != 0
	var names []exportedName
	exported := func(exportedAs string, name *ast.Node) {
		if !ambient {
			names = append(names, exportedName{exportedAs, f.NewIdentifier(name.Text())})
		}
	}
	switch visited.Kind {
	case ast.KindFunctionDeclaration:
		d := visited.AsFunctionDeclaration()
		name := d.Name()
		if name == nil {
			name = f.NewUniqueName("default")
		}
		if d.Body != nil {
			exported(core.IfElse(isDefault, "default", name.Text()), name)
			if name != d.Name() {
				names[len(names)-1].value = name.Clone(f)
			}
		}
		return f.UpdateFunctionDeclaration(d, keptList, d.AsteriskToken, name, d.TypeParameters, d.Parameters, d.Type, d.FullSignature, d.Body), names
	case ast.KindClassDeclaration:
		d := visited.AsClassDeclaration()
		name := d.Name()
		if name == nil {
			name = f.NewUniqueName("default")
		}
		exported(core.IfElse(isDefault, "default", name.Text()), name)
		if len(names) != 0 && name != d.Name() {
			names[len(names)-1].value = name.Clone(f)
		}
		return f.UpdateClassDeclaration(d, keptList, name, d.TypeParameters, d.HeritageClauses, d.Members), names
	case ast.KindVariableStatement:
		d := visited.AsVariableStatement()
		for _, declaration := range d.DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
			for _, name := range bindingNames(declaration.Name()) {
				exported(name.Text(), name)
			}
		}
		return f.UpdateVariableStatement(d, keptList, d.DeclarationList), names
	case ast.KindEnumDeclaration:
		d := visited.AsEnumDeclaration()
		if !ast.HasSyntacticModifier(statement, ast.ModifierFlagsConst) || tx.opts.CompilerOptions.ShouldPreserveConstEnums() {
			exported(d.Name().Text(), d.Name())
		}
		return f.UpdateEnumDeclaration(d, keptList, d.Name(), d.Members), names
	case ast.KindModuleDeclaration:
		d := visited.AsModuleDeclaration()
		if ast.IsIdentifier(d.Name()) && ast.GetModuleInstanceState(statement) != ast.ModuleInstanceStateNonInstantiated {
			exported(d.Name().Text(), d.Name())
		}
		return f.UpdateModuleDeclaration(d, keptList, d.Keyword, d.Name(), d.Attributes, d.Body), names
	case ast.KindImportEqualsDeclaration:
		d := visited.AsImportEqualsDeclaration()
		if !d.IsTypeOnly && tx.resolver.IsReferencedAliasDeclaration(statement) {
			exported(d.Name().Text(), d.Name())
		}
		return f.UpdateImportEqualsDeclaration(d, keptList, d.IsTypeOnly, d.Name(), d.ModuleReference), names
	case ast.KindInterfaceDeclaration:
		d := visited.AsInterfaceDeclaration()
		return f.UpdateInterfaceDeclaration(d, keptList, d.Name(), d.TypeParameters, d.HeritageClauses, d.Members), nil
	case ast.KindTypeAliasDeclaration:
		d := visited.AsTypeAliasDeclaration()
		return f.UpdateTypeAliasDeclaration(d, keptList, d.Name(), d.TypeParameters, d.Type), nil
	}
	return visited, nil
}

// moduleExports handles an export declaration of a module body: it calls
// export for each value it exports, and returns the imports that an export
// from another module needs.
func (tx *moduleInliner) moduleExports(module *inlineModule, statement *ast.Node, export func(string, *ast.Node)) []*ast.Node {
	f := tx.Factory()
	d := statement.AsExportDeclaration()
	if d.IsTypeOnly {
		return nil
	}
	if d.ModuleSpecifier == nil {
		// export { clean as tidy }
		for _, specifier := range d.ExportClause.Elements() {
			if !tx.keepsExport(specifier) {
				continue
			}
			local := specifier.PropertyNameOrName()
			var value *ast.Node
			if declaration := module.locals[local.Text()]; declaration != nil {
				value = tx.bindingValue(declaration, module.imports[declaration], nil)
			} else {
				value = f.NewIdentifier(local.Text())
			}
			export(moduleExportNameText(specifier.Name()), value)
		}
		return nil
	}
	owner := statement
	if d.Attributes == nil {
		if first := module.hoisted[d.ModuleSpecifier.Text()]; first != nil {
			owner = first // the module is already imported
		}
	}
	namespace := func() *ast.Node { return f.NewGeneratedNameForNode(owner) }
	var used bool
	switch {
	case d.ExportClause == nil: // export * from "./more.ts"
		for _, name := range tx.starExports(module.file, statement) {
			export(name, member(f, namespace(), name))
			used = true
		}
	case d.ExportClause.Kind == ast.KindNamespaceExport: // export * as more from "./more.ts"
		export(moduleExportNameText(d.ExportClause.Name()), namespace())
		used = true
	default: // export { a as b } from "./more.ts"
		for _, specifier := range d.ExportClause.Elements() {
			if tx.keepsExport(specifier) {
				export(moduleExportNameText(specifier.Name()), member(f, namespace(), moduleExportNameText(specifier.PropertyNameOrName())))
				used = true
			}
		}
	}
	if !used || owner != statement {
		return nil
	}
	if d.Attributes == nil {
		module.hoisted[d.ModuleSpecifier.Text()] = statement
	}
	return []*ast.Node{f.NewImportDeclaration(nil, f.NewImportClause(ast.KindUnknown, nil, f.NewNamespaceImport(namespace())), d.ModuleSpecifier, d.Attributes)}
}

// starExports returns the value names that `export * from` exports: those of
// the module that statement names, or of file itself when statement is nil,
// except default and, for a statement, the names file declares itself.
func (tx *moduleInliner) starExports(file *ast.SourceFile, statement *ast.Node) []string {
	r, ok := tx.resolver.(*checker.EmitResolver)
	if !ok {
		return nil
	}
	var names []string
	r.WithChecker(func(c *checker.Checker) {
		var source *ast.Symbol
		if statement == nil {
			source = c.GetMergedSymbol(file.Symbol)
		} else {
			source = c.ResolveExternalModuleSymbol(c.GetSymbolAtLocation(statement.AsExportDeclaration().ModuleSpecifier))
		}
		if source == nil {
			return
		}
		own := map[string]bool{}
		if statement != nil {
			for name := range file.Symbol.Exports() {
				own[name] = true
			}
		}
		for _, symbol := range c.GetExportsOfModule(source) {
			if symbol.Name() == ast.InternalSymbolNameDefault || own[symbol.Name()] || c.GetSymbolFlags(symbol)&ast.SymbolFlagsValue == 0 {
				continue
			}
			names = append(names, symbol.Name())
		}
	})
	slices.Sort(names) // the exports are a map
	return names
}

// containsTopLevelAwait reports whether file awaits at its top level, so its
// module's function is async and the file awaits it.
func containsTopLevelAwait(file *ast.SourceFile) bool {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		switch {
		case ast.IsFunctionLike(node) || ast.IsClassLike(node):
			return false
		case ast.IsAwaitExpression(node):
			return true
		case ast.IsForOfStatement(node) && node.AsForInOrOfStatement().AwaitModifier != nil:
			return true
		case ast.IsVariableDeclarationList(node) && node.Flags&ast.NodeFlagsAwaitUsing == ast.NodeFlagsAwaitUsing:
			return true
		}
		return node.ForEachChild(visit)
	}
	return file.AsNode().ForEachChild(visit)
}

// bindingNames returns the identifiers that a binding name declares.
func bindingNames(name *ast.Node) []*ast.Node {
	if ast.IsIdentifier(name) {
		return []*ast.Node{name}
	}
	var names []*ast.Node
	for _, element := range name.Elements() {
		if element.Name() != nil {
			names = append(names, bindingNames(element.Name())...)
		}
	}
	return names
}

// constStatement returns `const <declarations>`.
func constStatement(f *printer.NodeFactory, declarations []*ast.Node) *ast.Node {
	return f.NewVariableStatement(nil, f.NewVariableDeclarationList(f.NewNodeList(declarations), ast.NodeFlagsConst))
}

// member returns object.name, or object["name"] for a name that is not an
// identifier.
func member(f *printer.NodeFactory, object *ast.Node, name string) *ast.Node {
	if scanner.IsIdentifierText(name, core.LanguageVariantStandard) {
		return f.NewPropertyAccessExpression(object, nil, f.NewIdentifier(name), ast.NodeFlagsNone)
	}
	return f.NewElementAccessExpression(object, nil, f.NewStringLiteral(name, ast.TokenFlagsNone), ast.NodeFlagsNone)
}

// propertyName returns name as an object literal's property name.
func propertyName(f *printer.NodeFactory, name string) *ast.Node {
	if scanner.IsIdentifierText(name, core.LanguageVariantStandard) {
		return f.NewIdentifier(name)
	}
	return f.NewStringLiteral(name, ast.TokenFlagsNone)
}

// exportName returns name as an export specifier's name.
func exportName(f *printer.NodeFactory, name string) *ast.Node {
	return propertyName(f, name)
}

// cloneName copies a module export name, an identifier or a string literal.
func cloneName(f *printer.NodeFactory, name *ast.Node) *ast.Node {
	if ast.IsStringLiteral(name) {
		return f.NewStringLiteral(name.Text(), ast.TokenFlagsNone)
	}
	return f.NewIdentifier(name.Text())
}

// exportNameText is a name for the constant that holds an export: the
// export's name, or "reexport" when that is not an identifier.
func exportNameText(name *ast.Node) string {
	if ast.IsIdentifier(name) && name.Text() != "default" {
		return name.Text()
	}
	return "reexport"
}

// moduleExportNameText is the text of a module export name.
func moduleExportNameText(name *ast.Node) string {
	return name.Text()
}
