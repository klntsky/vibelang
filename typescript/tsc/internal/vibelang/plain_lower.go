package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/printer"
)

// visitPlainFunctionDeclaration turns a function that compiles to plain
// JavaScript into its $direct variant and its Effect form:
//
//	function double(n) { return n * 2 }
//
// becomes
//
//	function double$direct(n) { return n * 2 }
//	function double(...args) { return Effect.sync(() => double$direct(...args)) }
//
// The variant is exported when the function is, so other .vibe modules can
// import it. The Effect form keeps the declaration's modifiers and stays a
// hoisted function declaration. The variant keeps no link to the source
// declaration: TypeScript's CommonJS transform keys an exported function by
// its original node, so a link would merge the variant's export into the
// Effect form's and leave exports.double$direct unset.
func (tx *lowering) visitPlainFunctionDeclaration(node *ast.Node) *ast.Node {
	f := tx.Factory()
	ec := tx.EmitContext()
	name := node.Name().Text()

	saved, savedPlain := tx.inEffect, tx.plain
	tx.inEffect, tx.plain = false, true // its catch clauses rethrow defects (D54)
	parameters := ec.VisitParameters(node.ParameterList(), tx.Visitor())
	body := ec.VisitFunctionBody(node.Body(), tx.Visitor())
	tx.inEffect, tx.plain = saved, savedPlain

	var modifiers *ast.ModifierList
	if ast.HasSyntacticModifier(node, ast.ModifierFlagsExport) {
		modifiers = f.NewModifierList([]*ast.Node{f.NewModifier(ast.KindExportKeyword)})
	}
	direct := f.NewFunctionDeclaration(modifiers, nil /*asteriskToken*/, f.NewIdentifier(name+"$direct"), nil /*typeParameters*/, parameters, nil /*type*/, nil /*fullSignature*/, body)

	// function double(...args) { return Effect.sync(() => double$direct(...args)) }
	args := f.NewIdentifier("args")
	rest := f.NewParameterDeclaration(nil /*modifiers*/, f.NewToken(ast.KindDotDotDotToken), args, nil /*questionToken*/, nil /*type*/, nil /*initializer*/)
	call := f.NewCallExpression(f.NewIdentifier(name+"$direct"), nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{f.NewSpreadElement(args)}), ast.NodeFlagsNone)
	thunk := f.NewArrowFunction(nil /*modifiers*/, nil /*typeParameters*/, f.NewNodeList(nil), nil /*type*/, nil /*fullSignature*/, f.NewToken(ast.KindEqualsGreaterThanToken), call)
	sync := f.NewCallExpression(f.NewPropertyAccessExpression(tx.effectName(), nil /*questionDotToken*/, f.NewIdentifier("sync"), ast.NodeFlagsNone), nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{thunk}), ast.NodeFlagsNone)
	wrapperBody := f.NewBlock(f.NewNodeList([]*ast.Node{f.NewReturnStatement(sync)}), false /*multiLine*/)
	wrapper := f.UpdateFunctionDeclaration(node.AsFunctionDeclaration(), node.Modifiers(), nil /*asteriskToken*/, node.Name(), nil /*typeParameters*/, f.NewNodeList([]*ast.Node{rest}), nil /*type*/, nil /*fullSignature*/, wrapperBody)
	ec.SetOriginal(wrapper, node)
	// D268 puts purity on the function object, so export aliases preserve it.
	marker := f.NewElementAccessExpression(f.NewIdentifier(name), nil, tx.runtimeMember("$direct"), ast.NodeFlagsNone)
	assign := f.NewBinaryExpression(nil, marker, nil, f.NewToken(ast.KindEqualsToken), f.NewIdentifier(name+"$direct"))
	marked := f.NewExpressionStatement(assign)

	if ast.HasSyntacticModifier(node, ast.ModifierFlagsDefault) {
		alias := f.NewExportSpecifier(false, f.NewIdentifier(name+"$direct"), f.NewIdentifier("default$direct"))
		export := f.NewExportDeclaration(nil, false, f.NewNamedExports(f.NewNodeList([]*ast.Node{alias})), nil, nil)
		return f.NewSyntaxList([]*ast.Node{direct, wrapper, marked, export})
	}
	return f.NewSyntaxList([]*ast.Node{direct, wrapper, marked})
}

// visitDirectReference writes the $direct variant in place of node: the
// callee of a call that runs inline, or a function value passed where a plain
// function is expected.
func (tx *lowering) visitDirectReference(node *ast.Node, ref *directReference) *ast.Node {
	f := tx.Factory()
	var call *ast.Node
	switch {
	case ast.IsTaggedTemplateExpression(node):
		template := node.AsTaggedTemplateExpression()
		call = f.UpdateTaggedTemplateExpression(template, tx.directExpression(template.Tag, ref), template.QuestionDotToken, nil, tx.Visitor().VisitNode(template.Template), node.Flags)
	case node.Kind == ast.KindCallExpression:
		callee := tx.directExpression(node.Expression(), ref)
		arguments := tx.Visitor().VisitNodes(node.ArgumentList())
		call = f.UpdateCallExpression(node.AsCallExpression(), callee, node.QuestionDotToken(), nil /*typeArguments*/, arguments, node.Flags)
	default:
		return tx.directExpression(node, ref)
	}
	if tx.commonJS && ref.specifier != nil {
		// (0, m_1.double$direct)(n): the variant is called without a receiver,
		// as TypeScript's CommonJS transform calls an imported function.
		tx.EmitContext().AddEmitFlags(call, printer.EFIndirectCall)
	}
	return call
}

// directExpression returns the expression that names ref's variant in place
// of node: double$direct, M.double$direct through a namespace import, or, in a
// file emitted as CommonJS, m_1.double$direct through a named or default
// import (importedVariant).
func (tx *lowering) directExpression(node *ast.Node, ref *directReference) *ast.Node {
	f := tx.Factory()
	var expression *ast.Node
	switch {
	case ref.access != nil:
		receiver := tx.Visitor().VisitNode(ref.access.Expression())
		expression = f.NewPropertyAccessExpression(receiver, nil /*questionDotToken*/, f.NewIdentifier(ref.name), ast.NodeFlagsNone)
	case ref.specifier != nil && tx.commonJS:
		expression = tx.importedVariant(ref.specifier)
	default:
		expression = f.NewIdentifier(ref.name)
	}
	tx.EmitContext().SetOriginal(expression, node)
	expression.Loc = node.Loc
	return expression
}

// importedVariant returns, in a file emitted as CommonJS, the expression that
// reads the $direct variant of binding, an import specifier or default import
// clause, from the module its import requires: m_1.double$direct, or
// m_1.default$direct. TypeScript's CommonJS transform finds what an identifier
// names through the checker, which knows nothing of the variants import an ES
// module gets (visitImportWithDirectVariants): it would rewrite double$direct
// to the import's own binding, m_1.double. This is the property it writes for
// that binding, with the variant's name.
func (tx *lowering) importedVariant(binding *ast.Node) *ast.Node {
	f := tx.Factory()
	module := f.NewGeneratedNameForNode(ast.FindAncestor(binding, ast.IsImportDeclaration))
	if ast.IsImportClause(binding) {
		return f.NewPropertyAccessExpression(module, nil /*questionDotToken*/, f.NewIdentifier("default$direct"), ast.NodeFlagsNone)
	}
	imported := binding.AsImportSpecifier().PropertyNameOrName()
	if ast.IsStringLiteral(imported) {
		return f.NewElementAccessExpression(module, nil /*questionDotToken*/, f.NewStringLiteral(imported.Text()+"$direct", ast.TokenFlagsNone), ast.NodeFlagsNone)
	}
	return f.NewPropertyAccessExpression(module, nil /*questionDotToken*/, f.NewIdentifier(imported.Text()+"$direct"), ast.NodeFlagsNone)
}

// visitImportWithDirectVariants keeps an import of plain functions and adds
// one that brings in their $direct variants:
//
//	import { double } from "./math.js"
//	import { double$direct } from "./math.js"
func (tx *lowering) visitImportWithDirectVariants(node *ast.Node) *ast.Node {
	f := tx.Factory()
	visited := tx.visitImportOrExport(node)
	var specifiers []*ast.Node
	for _, specifier := range tx.plan.directImports[node] {
		local := specifier.Name().Text()
		var propertyName *ast.Node
		if ast.IsImportClause(specifier) {
			propertyName = f.NewIdentifier("default$direct")
		} else if property := specifier.AsImportSpecifier().PropertyName; property != nil {
			imported := property.Text()
			if ast.IsStringLiteral(property) {
				propertyName = f.NewStringLiteral(imported+"$direct", ast.TokenFlagsNone)
			} else if imported != local {
				propertyName = f.NewIdentifier(imported + "$direct")
			}
		}
		specifiers = append(specifiers, f.NewImportSpecifier(false /*isTypeOnly*/, propertyName, f.NewIdentifier(local+"$direct")))
	}
	clause := f.NewImportClause(ast.KindUnknown, nil /*name*/, f.NewNamedImports(f.NewNodeList(specifiers)))
	specifier := f.NewStringLiteral(tx.outputSpecifier(node.ModuleSpecifier()).Text(), ast.TokenFlagsNone)
	variants := f.NewImportDeclaration(nil /*modifiers*/, clause, specifier, nil /*attributes*/)
	ast.SetParentInChildren(variants)
	return f.NewSyntaxList([]*ast.Node{visited, variants})
}

// Preserve public Effect wrappers and export their plain variants under
// matching names. Export-star declarations already forward both forms.
func (tx *lowering) visitExportWithDirectVariants(node *ast.Node) *ast.Node {
	f := tx.Factory()
	visited := tx.visitImportOrExport(node)
	if tx.commonJS && node.AsExportDeclaration().ModuleSpecifier == nil {
		return f.NewSyntaxList(append([]*ast.Node{visited}, tx.defineDirectExports(node)...))
	}
	var specifiers []*ast.Node
	for _, specifier := range tx.plan.directExports[node] {
		local := specifier.AsExportSpecifier().PropertyName
		if local == nil {
			local = specifier.Name()
		}
		name := func(node *ast.Node) *ast.Node {
			if ast.IsStringLiteral(node) {
				return f.NewStringLiteral(node.Text()+"$direct", ast.TokenFlagsNone)
			}
			return f.NewIdentifier(node.Text() + "$direct")
		}
		specifiers = append(specifiers, f.NewExportSpecifier(false /*isTypeOnly*/, name(local), name(specifier.Name())))
	}
	decl := visited.AsExportDeclaration()
	variants := f.NewExportDeclaration(nil /*modifiers*/, false /*isTypeOnly*/, f.NewNamedExports(f.NewNodeList(specifiers)), decl.ModuleSpecifier, decl.Attributes)
	ast.SetParentInChildren(variants)
	return f.NewSyntaxList([]*ast.Node{visited, variants})
}

// defineDirectExports returns, for a local export list in a file emitted as
// CommonJS, a statement per plain function that defines its variant on
// exports as a live binding, the form TypeScript writes for an exported
// import:
//
//	Object.defineProperty(exports, "alias$direct", { enumerable: true, get: () => double$direct })
//
// TypeScript's CommonJS transform finds a local export's value through the
// checker, which knows no $direct binding, so an export list that named the
// variants would leave them undefined. An imported function's variant is read
// from its module (importedVariant), so the import may stand anywhere.
func (tx *lowering) defineDirectExports(node *ast.Node) []*ast.Node {
	f := tx.Factory()
	var statements []*ast.Node
	for _, specifier := range tx.plan.directExports[node] {
		var value *ast.Node
		if binding := tx.plan.exportedImports[specifier]; binding != nil {
			value = tx.importedVariant(binding)
		} else {
			local := specifier.AsExportSpecifier().PropertyName
			if local == nil {
				local = specifier.Name()
			}
			value = f.NewIdentifier(local.Text() + "$direct")
		}
		getter := f.NewArrowFunction(nil /*modifiers*/, nil /*typeParameters*/, f.NewNodeList(nil), nil /*type*/, nil /*fullSignature*/, f.NewToken(ast.KindEqualsGreaterThanToken), value)
		descriptor := f.NewObjectLiteralExpression(f.NewNodeList([]*ast.Node{
			f.NewPropertyAssignment(nil /*modifiers*/, f.NewIdentifier("enumerable"), nil /*postfixToken*/, nil /*typeNode*/, f.NewKeywordExpression(ast.KindTrueKeyword)),
			f.NewPropertyAssignment(nil /*modifiers*/, f.NewIdentifier("get"), nil /*postfixToken*/, nil /*typeNode*/, getter),
		}), false /*multiLine*/)
		define := f.NewPropertyAccessExpression(f.NewIdentifier("Object"), nil /*questionDotToken*/, f.NewIdentifier("defineProperty"), ast.NodeFlagsNone)
		name := f.NewStringLiteral(specifier.Name().Text()+"$direct", ast.TokenFlagsNone)
		call := f.NewCallExpression(define, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{f.NewIdentifier("exports"), name, descriptor}), ast.NodeFlagsNone)
		statements = append(statements, f.NewExpressionStatement(call))
	}
	return statements
}

// visitExportDefaultWithDirectVariant keeps `export default double` and
// exports double's variant beside it, since a default import of the module
// calls default$direct:
//
//	export default double
//	export const default$direct = double$direct
//
// The variant is double$direct, an imported variant (visitImportWithDirectVariants,
// or importedVariant in CommonJS), or M.double$direct, and is bound when the
// default is.
func (tx *lowering) visitExportDefaultWithDirectVariant(node *ast.Node) *ast.Node {
	f := tx.Factory()
	visited := tx.Visitor().VisitEachChild(node)
	variant := tx.directExpression(node.AsExportAssignment().Expression, tx.plan.directDefaults[node])
	declaration := f.NewVariableDeclaration(f.NewIdentifier("default$direct"), nil /*exclamationToken*/, nil /*typeNode*/, variant)
	list := f.NewVariableDeclarationList(f.NewNodeList([]*ast.Node{declaration}), ast.NodeFlagsConst)
	export := f.NewVariableStatement(f.NewModifierList([]*ast.Node{f.NewModifier(ast.KindExportKeyword)}), list)
	return f.NewSyntaxList([]*ast.Node{visited, export})
}
