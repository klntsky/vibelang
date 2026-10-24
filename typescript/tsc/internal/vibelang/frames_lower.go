package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/transformers"
)

// super and arguments in generated generators.
//
// A generator that the lowering introduces is a function of its own, and
// JavaScript binds super and arguments afresh in every function that is not
// an arrow: super is a SyntaxError in a function* expression, and arguments
// would be the generator's own. An Effect method's body runs in such a
// generator, and so do an Effect arrow's body and the regions of try, catch
// and finally:
//
//	greet(name) {
//	  return Effect.gen({ self: this }, function* greet() {
//	    const b = yield* super.greet(name)   // SyntaxError
//	    return arguments.length              // the generator's: 0
//	  })
//	}
//
// So the function that binds them in the source declares them once at the
// top of its lowered body, and the code that the lowering moved into a
// generator reads those bindings:
//
//	greet(name) {
//	  const super_1 = (key) => super[key];
//	  const arguments_1 = arguments;
//	  return Effect.gen({ self: this }, function* greet() {
//	    const b = yield* super_1("greet").call(this, name)
//	    return arguments_1.length
//	  })
//	}
//
// The arrow sees the method's super. A generator that holds a super
// reference also gets the method's this (usesThis counts super), so a call
// keeps its receiver, and JavaScript's order: the method is looked up before
// the arguments are evaluated. An assignment, super.name = value, calls a
// second arrow, (key, value) => super[key] = value, and so does a compound
// one, super.count += 1. A compound assignment to super[key] or an update
// such as super.count++ would evaluate its key twice, so it keeps
// JavaScript's form, which fails where it runs.

// frameReferences records which source functions bind a super or arguments
// reference in their body, and which arguments identifiers are references.
type frameReferences struct {
	binders   map[*ast.Node]bool // functions and static blocks whose super or arguments their body refers to
	arguments map[*ast.Node]bool // identifiers that refer to their function's arguments
}

// findFrameReferences finds, in file, each super property and arguments
// reference and the function that binds it: the nearest enclosing function
// that is not an arrow, and for super a method, accessor, constructor or
// class static block.
func findFrameReferences(file *ast.SourceFile) frameReferences {
	refs := frameReferences{binders: map[*ast.Node]bool{}, arguments: map[*ast.Node]bool{}}
	var visit func(node *ast.Node, argumentsBinder, superBinder *ast.Node) bool
	visit = func(node *ast.Node, argumentsBinder, superBinder *ast.Node) bool {
		switch {
		case node.Kind == ast.KindSuperKeyword:
			if superBinder != nil && isSuperProperty(node.Parent) {
				refs.binders[superBinder] = true
			}
		case ast.IsIdentifier(node) && node.Text() == "arguments":
			if argumentsBinder != nil && !ast.IsRightSideOfQualifiedNameOrPropertyAccess(node) && !ast.IsDeclarationName(node) {
				refs.binders[argumentsBinder] = true
				refs.arguments[node] = true
			}
		case node.Kind == ast.KindArrowFunction:
			// An arrow sees its enclosing function's super and arguments.
		case node.Kind == ast.KindClassStaticBlockDeclaration:
			argumentsBinder, superBinder = nil, node
		case node.Kind == ast.KindPropertyDeclaration:
			argumentsBinder, superBinder = nil, nil // an initializer has no body to declare them in
		case ast.IsFunctionLike(node):
			argumentsBinder, superBinder = node, nil
			switch node.Kind {
			case ast.KindMethodDeclaration, ast.KindGetAccessor, ast.KindSetAccessor, ast.KindConstructor:
				superBinder = node
			}
		}
		return node.ForEachChild(func(child *ast.Node) bool { return visit(child, argumentsBinder, superBinder) })
	}
	file.AsNode().ForEachChild(func(child *ast.Node) bool { return visit(child, nil, nil) })
	return refs
}

// isSuperProperty reports whether node is super.name or super[key].
func isSuperProperty(node *ast.Node) bool {
	return node != nil && (ast.IsPropertyAccessExpression(node) || ast.IsElementAccessExpression(node)) && node.Expression().Kind == ast.KindSuperKeyword
}

// frameBody returns the body of fn, a function or class static block.
func frameBody(fn *ast.Node) *ast.Node {
	if fn.Kind == ast.KindClassStaticBlockDeclaration {
		return fn.AsClassStaticBlockDeclaration().Body
	}
	return fn.Body()
}

// hoistFrameReferences rewrites each super and arguments reference that
// binder binds and that its lowered form, lowered, holds in a generator the
// lowering introduced. The lowered function that stands for binder declares
// the bindings they read at the top of its body.
func (tx *lowering) hoistFrameReferences(binder *ast.Node, lowered *ast.Node) *ast.Node {
	if lowered == nil {
		return nil
	}
	ec := tx.EmitContext()
	f := tx.Factory()
	// The generator that lowers a function declaration or expression is
	// called with that function's this and arguments (Effect.fnUntraced and
	// the forwarding apply), so it binds what the function binds. An Effect
	// method's generator is called with neither.
	forwards := binder.Kind == ast.KindFunctionDeclaration || binder.Kind == ast.KindFunctionExpression
	var crossed bool                                     // visiting code inside an introduced generator
	var inFrame bool                                     // visiting the function that stands for binder
	var superName, superSetName, argumentsName *ast.Node // the bindings the current frame declares, once used
	read := func(key *ast.Node) *ast.Node {
		if superName == nil {
			superName = f.NewUniqueName("super")
		}
		return f.NewCallExpression(superName, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{key}), ast.NodeFlagsNone)
	}
	write := func(key, value *ast.Node) *ast.Node {
		if superSetName == nil {
			superSetName = f.NewUniqueName("superSet")
		}
		return f.NewCallExpression(superSetName, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{key, value}), ast.NodeFlagsNone)
	}
	visitor := ec.NewNodeVisitor(nil)
	visitor.Visit = func(node *ast.Node) *ast.Node {
		source := ec.ParseNode(node)
		switch {
		case crossed && isSuperAssignment(node):
			binary := node.AsBinaryExpression()
			operator := binary.OperatorToken.Kind
			right := visitor.VisitNode(binary.Right)
			var value *ast.Node
			switch {
			case operator == ast.KindEqualsToken:
				value = write(tx.superKey(visitor, binary.Left), right)
			case ast.IsLogicalOrCoalescingAssignmentOperator(operator):
				// super.x ??= v is super_1("x") ?? superSet_1("x", v).
				value = f.NewBinaryExpression(nil /*modifiers*/, read(tx.superKey(visitor, binary.Left)), nil /*typeNode*/, f.NewToken(transformers.GetNonAssignmentOperatorForCompoundAssignment(operator)), write(tx.superKey(visitor, binary.Left), right))
			default:
				// super.x += v is superSet_1("x", super_1("x") + v).
				current := f.NewBinaryExpression(nil /*modifiers*/, read(tx.superKey(visitor, binary.Left)), nil /*typeNode*/, f.NewToken(transformers.GetNonAssignmentOperatorForCompoundAssignment(operator)), right)
				value = write(tx.superKey(visitor, binary.Left), current)
			}
			ec.SetOriginal(value, node)
			value.Loc = node.Loc
			return value
		case crossed && ast.IsCallExpression(node) && isSuperProperty(node.Expression()):
			// super.m(a) is super_1("m").call(this, a): the lookup first, then the arguments.
			method := read(tx.superKey(visitor, node.Expression()))
			callee := f.NewPropertyAccessExpression(method, node.QuestionDotToken(), f.NewIdentifier("call"), ast.NodeFlagsNone)
			arguments := append([]*ast.Node{f.NewThisExpression()}, visitor.VisitNodes(node.ArgumentList()).Nodes...)
			call := f.NewCallExpression(callee, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList(arguments), ast.NodeFlagsNone)
			ec.SetOriginal(call, node)
			call.Loc = node.Loc
			return call
		case crossed && isSuperProperty(node) && !isSuperUpdateTarget(node):
			value := read(tx.superKey(visitor, node))
			ec.SetOriginal(value, node)
			value.Loc = node.Loc
			return value
		case crossed && ast.IsIdentifier(node) && tx.frames.arguments[source]:
			if argumentsName == nil {
				argumentsName = f.NewUniqueName("arguments")
			}
			return argumentsName
		case node.Kind == ast.KindArrowFunction:
			return visitor.VisitEachChild(node)
		case ast.IsClassLike(node) && source != nil:
			return node // its members bind their own super and arguments
		case !ast.IsFunctionLike(node) && node.Kind != ast.KindClassStaticBlockDeclaration:
			return visitor.VisitEachChild(node)
		case source == binder && node.Kind == binder.Kind && !inFrame:
			// The function that stands for binder, outermost: it declares the
			// bindings. A plain function declaration's $direct variant and its
			// wrapper are two such functions (plain_lower.go).
			crossed, inFrame, superName, superSetName, argumentsName = false, true, nil, nil, nil
			visited := tx.declareFrameBindings(visitor.VisitEachChild(node), superName, superSetName, argumentsName)
			inFrame = false
			return visited
		case source == binder && forwards:
			return visitor.VisitEachChild(node)
		case source != nil && source != binder && (ast.IsFunctionLike(source) && source.Kind != ast.KindArrowFunction || source.Kind == ast.KindClassStaticBlockDeclaration):
			return node // a function of the source binds its own
		}
		// A generator, or another function, that the lowering introduced.
		saved := crossed
		crossed = true
		visited := visitor.VisitEachChild(node)
		crossed = saved
		return visited
	}
	return visitor.VisitNode(lowered)
}

// isSuperAssignment reports whether node assigns to a super property in a
// form whose key is evaluated once when it is rewritten: super.name with any
// assignment operator, or super[key] = value.
func isSuperAssignment(node *ast.Node) bool {
	if !ast.IsBinaryExpression(node) {
		return false
	}
	binary := node.AsBinaryExpression()
	if !ast.IsAssignmentOperator(binary.OperatorToken.Kind) || !isSuperProperty(binary.Left) {
		return false
	}
	return binary.OperatorToken.Kind == ast.KindEqualsToken || ast.IsPropertyAccessExpression(binary.Left)
}

// isSuperUpdateTarget reports whether node, a super property, is the operand
// of ++ or -- or the target of a compound assignment that isSuperAssignment
// leaves alone: rewriting it as a read would not assign.
func isSuperUpdateTarget(node *ast.Node) bool {
	parent := node.Parent
	switch {
	case parent == nil:
		return false
	case ast.IsPrefixUnaryExpression(parent):
		operator := parent.AsPrefixUnaryExpression().Operator
		return operator == ast.KindPlusPlusToken || operator == ast.KindMinusMinusToken
	case ast.IsPostfixUnaryExpression(parent):
		return true
	case ast.IsBinaryExpression(parent):
		return parent.AsBinaryExpression().Left == node && ast.IsAssignmentOperator(parent.AsBinaryExpression().OperatorToken.Kind)
	}
	return false
}

// superKey returns the key that the super property node reads, "name" for
// super.name and the visited key for super[key].
func (tx *lowering) superKey(visitor *ast.NodeVisitor, node *ast.Node) *ast.Node {
	if ast.IsPropertyAccessExpression(node) {
		return tx.Factory().NewStringLiteral(node.Name().Text(), ast.TokenFlagsNone)
	}
	return visitor.VisitNode(node.AsElementAccessExpression().ArgumentExpression)
}

// declareFrameBindings returns fn, a lowered function or static block, with
// `const super_1 = (key) => super[key]`, `const superSet_1 = (key, value) =>
// super[key] = value` and `const arguments_1 = arguments` at the top of its
// body, after its directives, for the names that are not nil.
func (tx *lowering) declareFrameBindings(fn *ast.Node, superName, superSetName, argumentsName *ast.Node) *ast.Node {
	body := frameBody(fn)
	if superName == nil && superSetName == nil && argumentsName == nil || body == nil || !ast.IsBlock(body) {
		return fn
	}
	f := tx.Factory()
	ec := tx.EmitContext()
	var declarations []*ast.Node
	declare := func(name, value *ast.Node) {
		declarations = append(declarations, f.NewVariableStatement(nil /*modifiers*/, f.NewVariableDeclarationList(f.NewNodeList([]*ast.Node{
			f.NewVariableDeclaration(name, nil /*exclamationToken*/, nil /*typeNode*/, value),
		}), ast.NodeFlagsConst)))
	}
	if superName != nil {
		key := f.NewIdentifier("key")
		parameter := f.NewParameterDeclaration(nil /*modifiers*/, nil /*dotDotDotToken*/, key, nil /*questionToken*/, nil /*type*/, nil /*initializer*/)
		read := f.NewElementAccessExpression(f.NewKeywordExpression(ast.KindSuperKeyword), nil /*questionDotToken*/, f.NewIdentifier("key"), ast.NodeFlagsNone)
		declare(superName, f.NewArrowFunction(nil /*modifiers*/, nil /*typeParameters*/, f.NewNodeList([]*ast.Node{parameter}), nil /*type*/, nil /*fullSignature*/, f.NewToken(ast.KindEqualsGreaterThanToken), read))
	}
	if superSetName != nil {
		key := f.NewParameterDeclaration(nil /*modifiers*/, nil /*dotDotDotToken*/, f.NewIdentifier("key"), nil /*questionToken*/, nil /*type*/, nil /*initializer*/)
		value := f.NewParameterDeclaration(nil /*modifiers*/, nil /*dotDotDotToken*/, f.NewIdentifier("value"), nil /*questionToken*/, nil /*type*/, nil /*initializer*/)
		target := f.NewElementAccessExpression(f.NewKeywordExpression(ast.KindSuperKeyword), nil /*questionDotToken*/, f.NewIdentifier("key"), ast.NodeFlagsNone)
		assign := f.NewBinaryExpression(nil /*modifiers*/, target, nil /*typeNode*/, f.NewToken(ast.KindEqualsToken), f.NewIdentifier("value"))
		declare(superSetName, f.NewArrowFunction(nil /*modifiers*/, nil /*typeParameters*/, f.NewNodeList([]*ast.Node{key, value}), nil /*type*/, nil /*fullSignature*/, f.NewToken(ast.KindEqualsGreaterThanToken), assign))
	}
	if argumentsName != nil {
		declare(argumentsName, f.NewIdentifier("arguments"))
	}
	statements := body.Statements()
	directives := 0
	for directives < len(statements) && ast.IsPrologueDirective(statements[directives]) {
		directives++
	}
	all := append(append(append([]*ast.Node{}, statements[:directives]...), declarations...), statements[directives:]...)
	block := f.UpdateBlock(body.AsBlock(), f.NewNodeList(all), body.AsBlock().MultiLine)
	replace := ec.NewNodeVisitor(nil)
	replace.Visit = func(node *ast.Node) *ast.Node {
		if node == body {
			return block
		}
		return node
	}
	return replace.VisitEachChild(fn)
}
