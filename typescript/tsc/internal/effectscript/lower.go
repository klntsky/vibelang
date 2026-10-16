package effectscript

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/printer"
	"github.com/microsoft/TypeScript/tsc/internal/transformers"
)

// lowering rewrites one .es file so it runs on Effect 4. It runs before the
// TypeScript transforms, which then erase types and convert modules as usual.
// A function that the plan marks as an Effect becomes an Effect.fnUntraced
// generator, and each call in it that the plan marks as running becomes a
// yield*:
//
//	export function greet(name: string) {
//	  Console.log(`hello, ${name}`)
//	  return name.length
//	}
//
// becomes
//
//	import * as Effect from "effect/Effect"
//	export const greet = Effect.fnUntraced(function* (name: string) {
//	  yield* Console.log(`hello, ${name}`)
//	  return name.length
//	})
type lowering struct {
	transformers.Transformer
	plan     *plan
	effect   *ast.IdentifierNode // local name for effect's Effect module, imported on first use
	inEffect bool                // visiting code that runs inside a lowered generator
}

func newLowering(opts *transformers.TransformOptions, file *ast.SourceFile) *transformers.Transformer {
	if !isEffectScriptFile(file) {
		return nil
	}
	tx := &lowering{plan: planFile(opts.EmitResolver, file)}
	return tx.NewTransformer(tx.visit, opts.Context)
}

func (tx *lowering) visit(node *ast.Node) *ast.Node {
	switch {
	case node.Kind == ast.KindSourceFile:
		return tx.visitSourceFile(node.AsSourceFile())
	case node.Kind == ast.KindCallExpression && tx.inEffect && tx.plan.runsCall(node):
		return tx.visitRunningCall(node)
	case node.Kind == ast.KindFunctionDeclaration && tx.plan.isEffect(node):
		return tx.visitEffectFunctionDeclaration(node)
	case tx.plan.isEffect(node):
		return tx.lowerFunction(node)
	case isFunctionBoundary(node):
		saved := tx.inEffect
		tx.inEffect = false
		defer func() { tx.inEffect = saved }()
	}
	return tx.Visitor().VisitEachChild(node)
}

func (tx *lowering) visitSourceFile(file *ast.SourceFile) *ast.Node {
	visited := tx.Visitor().VisitEachChild(file.AsNode())
	if tx.effect == nil {
		return visited
	}
	f := tx.Factory()
	// import * as Effect from "effect/Effect"
	importEffect := f.NewImportDeclaration(
		nil, /*modifiers*/
		f.NewImportClause(ast.KindUnknown, nil /*name*/, f.NewNamespaceImport(tx.effect)),
		f.NewStringLiteral("effect/Effect", ast.TokenFlagsNone),
		nil, /*attributes*/
	)
	ast.SetParentInChildren(importEffect)
	statements := append([]*ast.Node{importEffect}, visited.Statements()...)
	return f.UpdateSourceFile(visited.AsSourceFile(), f.NewNodeList(statements), visited.AsSourceFile().EndOfFileToken)
}

// effectName returns the local name bound to effect's Effect module.
func (tx *lowering) effectName() *ast.IdentifierNode {
	if tx.effect == nil {
		// "Effect" when the file does not use that name, else "Effect_1", ...
		tx.effect = tx.Factory().NewUniqueNameEx("Effect", printer.AutoGenerateOptions{Flags: printer.GeneratedIdentifierFlagsOptimistic | printer.GeneratedIdentifierFlagsFileLevel})
	}
	return tx.effect
}

// visitRunningCall turns `f(x)` into `yield* f(x)`.
func (tx *lowering) visitRunningCall(node *ast.Node) *ast.Node {
	call := tx.Visitor().VisitEachChild(node)
	run := tx.Factory().NewYieldExpression(tx.Factory().NewToken(ast.KindAsteriskToken), call)
	tx.EmitContext().SetOriginal(run, node)
	return run
}

// lowerFunction turns a function expression or arrow function into
// `Effect.fnUntraced(function* (params) { body })`.
func (tx *lowering) lowerFunction(node *ast.Node) *ast.Node {
	f := tx.Factory()
	ec := tx.EmitContext()

	saved := tx.inEffect
	tx.inEffect = false // a generator's parameter list cannot yield
	parameters := ec.VisitParameters(node.ParameterList(), tx.Visitor())
	tx.inEffect = true
	body := ec.VisitFunctionBody(node.Body(), tx.Visitor())
	tx.inEffect = saved
	if !ast.IsBlock(body) {
		body = ec.ConvertToFunctionBlock(body, false /*multiLine*/)
	}

	var name *ast.Node
	if node.Kind == ast.KindFunctionExpression {
		name = node.Name()
	}
	generator := f.NewFunctionExpression(nil /*modifiers*/, f.NewToken(ast.KindAsteriskToken), name, nil /*typeParameters*/, parameters, nil /*type*/, nil /*fullSignature*/, body)
	ec.SetOriginal(generator, node)

	fnUntraced := f.NewPropertyAccessExpression(tx.effectName(), nil /*questionDotToken*/, f.NewIdentifier("fnUntraced"), ast.NodeFlagsNone)
	lowered := f.NewCallExpression(fnUntraced, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{generator}), ast.NodeFlagsNone)
	ec.SetOriginal(lowered, node)
	lowered.Loc = node.Loc
	return lowered
}

// visitEffectFunctionDeclaration turns `function f() {}` into
// `const f = Effect.fnUntraced(function* () {})`, keeping `export` and
// `export default`. The const is not hoisted like the function was.
func (tx *lowering) visitEffectFunctionDeclaration(node *ast.Node) *ast.Node {
	f := tx.Factory()
	ec := tx.EmitContext()
	lowered := tx.lowerFunction(node)
	exported := ast.HasSyntacticModifier(node, ast.ModifierFlagsExport)
	isDefault := ast.HasSyntacticModifier(node, ast.ModifierFlagsDefault)

	name := node.Name()
	if name == nil {
		// export default function () {}
		statement := f.NewExportAssignment(nil /*modifiers*/, false /*isExportEquals*/, nil /*type*/, lowered)
		ec.SetOriginal(statement, node)
		statement.Loc = node.Loc
		return statement
	}

	var modifiers *ast.ModifierList
	if exported && !isDefault {
		modifiers = f.NewModifierList([]*ast.Node{f.NewModifier(ast.KindExportKeyword)})
	}
	declaration := f.NewVariableDeclaration(name, nil /*exclamationToken*/, nil /*type*/, lowered)
	statement := f.NewVariableStatement(modifiers, f.NewVariableDeclarationList(f.NewNodeList([]*ast.Node{declaration}), ast.NodeFlagsConst))
	ec.SetOriginal(statement, node)
	statement.Loc = node.Loc
	if exported && isDefault {
		exportDefault := f.NewExportAssignment(nil /*modifiers*/, false /*isExportEquals*/, nil /*type*/, f.NewIdentifier(name.Text()))
		return f.NewSyntaxList([]*ast.Node{statement, exportDefault})
	}
	return statement
}
