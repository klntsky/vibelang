package effectscript

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
)

// Debugging (design D36). A compiled function is a generator function named
// after its source, so a stack trace and a debugger's call stack show the
// EffectScript function by name:
//
//	export const greet = Effect.fnUntraced(function* greet(name) { … })
//
// A named function expression binds its name inside its own body, which
// would capture the body's references to the function itself: fact(n - 1)
// would call the generator, not the Effect function. When the body refers to
// the name, the generator is a method of an object literal instead, which
// has the name and binds nothing:
//
//	const fact = Effect.fnUntraced({ *fact(n) { … yield* fact(n - 1) … } }.fact)

// generatorExpression returns the generator for original, a function the
// lowering turns into an Effect, named name: `function* name(…) { … }`,
// the method form when body refers to name, or an anonymous generator when
// name is "". A function expression's own name is its binding in the source,
// so it stays one.
func (tx *lowering) generatorExpression(original *ast.Node, name string, parameters *ast.NodeList, body *ast.Node) *ast.Node {
	f := tx.Factory()
	ownName := original.Kind == ast.KindFunctionExpression && original.Name() != nil
	if name == "" || ownName || !refersTo(original.Body(), name) {
		var id *ast.Node
		if name != "" {
			id = f.NewIdentifier(name)
		}
		return f.NewFunctionExpression(nil /*modifiers*/, f.NewToken(ast.KindAsteriskToken), id, nil /*typeParameters*/, parameters, nil /*type*/, nil /*fullSignature*/, body)
	}
	method := f.NewMethodDeclaration(nil /*modifiers*/, f.NewToken(ast.KindAsteriskToken), f.NewIdentifier(name), nil /*postfixToken*/, nil /*typeParameters*/, parameters, nil /*type*/, nil /*fullSignature*/, body)
	object := f.NewObjectLiteralExpression(f.NewNodeList([]*ast.Node{method}), false /*multiLine*/)
	return f.NewPropertyAccessExpression(object, nil /*questionDotToken*/, f.NewIdentifier(name), ast.NodeFlagsNone)
}

// refersTo reports whether node contains an identifier named name, other
// than a property name after a dot. It may answer true for a name that a
// nested scope declares, which only costs the method form.
func refersTo(node *ast.Node, name string) bool {
	if node == nil {
		return false
	}
	if ast.IsIdentifier(node) && node.Text() == name && !ast.IsRightSideOfQualifiedNameOrPropertyAccess(node) {
		return true
	}
	return node.ForEachChild(func(child *ast.Node) bool { return refersTo(child, name) })
}

// sourceName returns the name the source gives fn: its own name, the name of
// the variable or property it initializes, or "".
func sourceName(fn *ast.Node) string {
	if name := functionName(fn); name != "" {
		return name
	}
	if parent := fn.Parent; parent != nil && parent.Kind == ast.KindPropertyAssignment && parent.Initializer() == fn {
		return declarationName(parent)
	}
	return ""
}
