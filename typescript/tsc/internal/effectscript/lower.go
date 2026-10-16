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
	runtime  *ast.IdentifierNode // local name for @effectscript/runtime, imported on first use
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
	case node.Kind == ast.KindCallExpression && tx.inEffect && tx.plan.values[node]:
		return tx.visitEffectValueCall(node)
	case node.Kind == ast.KindCallExpression && tx.inEffect && tx.plan.runsCall(node):
		return tx.visitRunningCall(node)
	case node.Kind == ast.KindPropertyAccessExpression && tx.inEffect && tx.plan.looksUp(node):
		return tx.visitServiceLookup(node)
	case node.Kind == ast.KindClassDeclaration && tx.plan.keys[node] != "":
		return tx.visitServiceClass(node)
	case node.Kind == ast.KindFunctionDeclaration && tx.plan.isEffect(node):
		return tx.visitEffectFunctionDeclaration(node)
	case tx.plan.isEffect(node):
		return tx.lowerFunction(node)
	case tx.plan.isProgram(node):
		return tx.lowerProgram(node)
	case isFunctionBoundary(node):
		saved := tx.inEffect
		tx.inEffect = false
		defer func() { tx.inEffect = saved }()
	}
	return tx.Visitor().VisitEachChild(node)
}

func (tx *lowering) visitSourceFile(file *ast.SourceFile) *ast.Node {
	visited := tx.Visitor().VisitEachChild(file.AsNode())
	var imports []*ast.Node
	if tx.effect != nil {
		imports = append(imports, tx.namespaceImport(tx.effect, "effect/Effect"))
	}
	if tx.runtime != nil {
		imports = append(imports, tx.namespaceImport(tx.runtime, "@effectscript/runtime"))
	}
	if len(imports) == 0 {
		return visited
	}
	statements := append(imports, visited.Statements()...)
	return tx.Factory().UpdateSourceFile(visited.AsSourceFile(), tx.Factory().NewNodeList(statements), visited.AsSourceFile().EndOfFileToken)
}

// namespaceImport returns `import * as name from "module"`.
func (tx *lowering) namespaceImport(name *ast.IdentifierNode, module string) *ast.Node {
	f := tx.Factory()
	declaration := f.NewImportDeclaration(
		nil, /*modifiers*/
		f.NewImportClause(ast.KindUnknown, nil /*name*/, f.NewNamespaceImport(name)),
		f.NewStringLiteral(module, ast.TokenFlagsNone),
		nil, /*attributes*/
	)
	ast.SetParentInChildren(declaration)
	return declaration
}

// effectName returns the local name bound to effect's Effect module.
func (tx *lowering) effectName() *ast.IdentifierNode {
	if tx.effect == nil {
		// "Effect" when the file does not use that name, else "Effect_1", ...
		tx.effect = tx.Factory().NewUniqueNameEx("Effect", printer.AutoGenerateOptions{Flags: printer.GeneratedIdentifierFlagsOptimistic | printer.GeneratedIdentifierFlagsFileLevel})
	}
	return tx.effect
}

// runtimeMember returns `runtime.name`, where runtime is the local name
// bound to @effectscript/runtime.
func (tx *lowering) runtimeMember(name string) *ast.Node {
	if tx.runtime == nil {
		tx.runtime = tx.Factory().NewUniqueNameEx("runtime", printer.AutoGenerateOptions{Flags: printer.GeneratedIdentifierFlagsOptimistic | printer.GeneratedIdentifierFlagsFileLevel})
	}
	return tx.Factory().NewPropertyAccessExpression(tx.runtime, nil /*questionDotToken*/, tx.Factory().NewIdentifier(name), ast.NodeFlagsNone)
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
	return tx.wrapGenerator("fnUntraced", node, tx.generatorOf(node))
}

// lowerProgram turns a lambda with no parameters that is a program into
// `Effect.gen(function* () { body })`, an Effect that runs the body each time
// it runs.
func (tx *lowering) lowerProgram(node *ast.Node) *ast.Node {
	return tx.wrapGenerator("gen", node, tx.generatorOf(node))
}

// generatorOf returns `function* (params) { body }` for node, with node's
// calls that run lowered to yield*.
func (tx *lowering) generatorOf(node *ast.Node) *ast.Node {
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
	return generator
}

// wrapGenerator returns `Effect.<method>(generator)` in place of node.
func (tx *lowering) wrapGenerator(method string, node *ast.Node, generator *ast.Node) *ast.Node {
	f := tx.Factory()
	callee := f.NewPropertyAccessExpression(tx.effectName(), nil /*questionDotToken*/, f.NewIdentifier(method), ast.NodeFlagsNone)
	lowered := f.NewCallExpression(callee, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{generator}), ast.NodeFlagsNone)
	tx.EmitContext().SetOriginal(lowered, node)
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
