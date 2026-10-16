package effectscript

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// expectsEffect reports whether fn, a method, is expected to return an
// Effect, so it compiles to an Effect even when its body runs nothing: the
// type its object literal's contextual type gives it, or the member it
// implements through its class's implements clause, is a function that
// returns an Effect. A provider's { find: (id) => ({ … }) } and a class that
// implements a service with plain-looking methods both need this.
func (a *analyzer) expectsEffect(fn *ast.Node) bool {
	if fn.Kind != ast.KindMethodDeclaration || ast.IsStatic(fn) {
		return false
	}
	var expected []*checker.Type
	switch parent := fn.Parent; {
	case ast.IsObjectLiteralExpression(parent):
		if t := a.checker.GetContextualTypeForObjectLiteralElement(fn, checker.ContextFlagsSignature); t != nil {
			expected = append(expected, t)
		}
	case ast.IsClassLike(parent):
		name := fn.Name()
		if !ast.IsIdentifier(name) && !ast.IsStringLiteral(name) {
			return false
		}
		for _, heritage := range ast.GetImplementsHeritageClauseElements(parent) {
			if member := a.checker.GetPropertyOfType(a.checker.GetTypeFromTypeNode(heritage), name.Text()); member != nil {
				expected = append(expected, a.checker.GetTypeOfSymbol(member))
			}
		}
	}
	return a.classify(expected).effectFn
}

// lowerMethod turns a method whose body runs Effects into a method that
// returns an Effect. The method keeps its name and parameters, so object
// literal methods and class methods stay methods, and its body runs as a
// generator with the method's this:
//
//	find(id) { return Effect.gen({ self: this }, function* () { … }) }
//
// A body that does not mention this omits the { self: this } argument.
func (tx *lowering) lowerMethod(node *ast.Node) *ast.Node {
	f := tx.Factory()
	ec := tx.EmitContext()
	method := node.AsMethodDeclaration()

	saved, savedFrame, savedPlain := tx.inEffect, tx.frame, tx.plain
	tx.inEffect, tx.plain = false, false // a parameter list cannot yield
	parameters := ec.VisitParameters(node.ParameterList(), tx.Visitor())
	tx.inEffect = true
	tx.frame = &frame{name: functionName(node), clause: tx.plan.resumesInTail(node)}
	body := ec.VisitFunctionBody(node.Body(), tx.Visitor())
	tx.inEffect, tx.frame, tx.plain = saved, savedFrame, savedPlain

	generator := tx.generatorExpression(node, sourceName(node), f.NewNodeList(nil), body)
	ec.SetOriginal(generator, node)
	arguments := []*ast.Node{generator}
	if usesThis(node.Body()) {
		self := f.NewPropertyAssignment(nil /*modifiers*/, f.NewIdentifier("self"), nil /*postfixToken*/, nil /*type*/, f.NewKeywordExpression(ast.KindThisKeyword))
		arguments = []*ast.Node{f.NewObjectLiteralExpression(f.NewNodeList([]*ast.Node{self}), false /*multiLine*/), generator}
	}
	gen := f.NewCallExpression(f.NewPropertyAccessExpression(tx.effectName(), nil /*questionDotToken*/, f.NewIdentifier("gen"), ast.NodeFlagsNone), nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList(arguments), ast.NodeFlagsNone)
	block := f.NewBlock(f.NewNodeList([]*ast.Node{f.NewReturnStatement(tx.traceMethod(node, gen))}), true /*multiLine*/)
	lowered := f.UpdateMethodDeclaration(method, node.Modifiers(), method.AsteriskToken, node.Name(), method.PostfixToken, method.TypeParameters, parameters, method.Type, method.FullSignature, block)
	ec.SetOriginal(lowered, node)
	return lowered
}

// usesThis reports whether body refers to this, looking into arrow
// functions, which share their enclosing function's this, but not into other
// functions or classes.
func usesThis(body *ast.Node) bool {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		switch {
		case node.Kind == ast.KindThisKeyword:
			return true
		case node.Kind == ast.KindArrowFunction:
		case isFunctionBoundary(node):
			return false
		}
		return node.ForEachChild(visit)
	}
	return visit(body)
}
