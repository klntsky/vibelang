package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
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
// hoisted function declaration.
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
	ec.SetOriginal(direct, node)

	// function double(...args) { return Effect.sync(() => double$direct(...args)) }
	args := f.NewIdentifier("args")
	rest := f.NewParameterDeclaration(nil /*modifiers*/, f.NewToken(ast.KindDotDotDotToken), args, nil /*questionToken*/, nil /*type*/, nil /*initializer*/)
	call := f.NewCallExpression(f.NewIdentifier(name+"$direct"), nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{f.NewSpreadElement(args)}), ast.NodeFlagsNone)
	thunk := f.NewArrowFunction(nil /*modifiers*/, nil /*typeParameters*/, f.NewNodeList(nil), nil /*type*/, nil /*fullSignature*/, f.NewToken(ast.KindEqualsGreaterThanToken), call)
	sync := f.NewCallExpression(f.NewPropertyAccessExpression(tx.effectName(), nil /*questionDotToken*/, f.NewIdentifier("sync"), ast.NodeFlagsNone), nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{thunk}), ast.NodeFlagsNone)
	wrapperBody := f.NewBlock(f.NewNodeList([]*ast.Node{f.NewReturnStatement(sync)}), false /*multiLine*/)
	wrapper := f.UpdateFunctionDeclaration(node.AsFunctionDeclaration(), node.Modifiers(), nil /*asteriskToken*/, node.Name(), nil /*typeParameters*/, f.NewNodeList([]*ast.Node{rest}), nil /*type*/, nil /*fullSignature*/, wrapperBody)
	ec.SetOriginal(wrapper, node)
	return f.NewSyntaxList([]*ast.Node{direct, wrapper})
}

// visitDirectReference writes the $direct variant in place of node: the
// callee of a call that runs inline, or a function value passed where a plain
// function is expected.
func (tx *lowering) visitDirectReference(node *ast.Node, ref *directReference) *ast.Node {
	f := tx.Factory()
	if node.Kind != ast.KindCallExpression {
		return tx.directExpression(node, ref)
	}
	callee := tx.directExpression(node.Expression(), ref)
	arguments := tx.Visitor().VisitNodes(node.ArgumentList())
	return f.UpdateCallExpression(node.AsCallExpression(), callee, node.QuestionDotToken(), nil /*typeArguments*/, arguments, node.Flags)
}

// directExpression returns the expression that names ref's variant in place
// of node: double$direct, or M.double$direct through a namespace import.
func (tx *lowering) directExpression(node *ast.Node, ref *directReference) *ast.Node {
	f := tx.Factory()
	var expression *ast.Node
	if ref.access != nil {
		receiver := tx.Visitor().VisitNode(ref.access.Expression())
		expression = f.NewPropertyAccessExpression(receiver, nil /*questionDotToken*/, f.NewIdentifier(ref.name), ast.NodeFlagsNone)
	} else {
		expression = f.NewIdentifier(ref.name)
	}
	tx.EmitContext().SetOriginal(expression, node)
	expression.Loc = node.Loc
	return expression
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
		imported := local
		if property := specifier.AsImportSpecifier().PropertyName; property != nil {
			imported = property.Text()
		}
		var propertyName *ast.Node
		if imported != local {
			propertyName = f.NewIdentifier(imported + "$direct")
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
