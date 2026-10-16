package effectscript

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// servicePlan records what the lowering does with services in one file.
type servicePlan struct {
	lookups    map[*ast.Node]bool   // calls S.member(…) whose callee looks the service up
	values     map[*ast.Node]bool   // calls p() that run p, an Effect value
	keys       map[*ast.Node]string // service and control classes, by declaration, and their keys
	wireOrders map[*ast.Node][]int  // wire calls whose arguments the lowering reorders
}

func newServicePlan() servicePlan {
	return servicePlan{lookups: map[*ast.Node]bool{}, values: map[*ast.Node]bool{}, keys: map[*ast.Node]string{}, wireOrders: map[*ast.Node][]int{}}
}

// planServiceCall records how call, in the body of a function the lowering
// turns into an Effect, uses services.
func (p *servicePlan) planServiceCall(c *checker.Checker, call *ast.Node) {
	if isServiceLookup(c, call) {
		p.lookups[call] = true
	}
	if isEffectValueCall(c, call) {
		p.values[call] = true
	}
}

// planServiceNode records the key of node when it declares a service or a
// control operation, and the order of its arguments when it calls wire.
func (p *servicePlan) planServiceNode(c *checker.Checker, node *ast.Node) {
	if classKindOf(node) != ordinaryClass {
		p.keys[node] = servicesFor(c).key(node)
	}
	if ast.IsCallExpression(node) {
		p.planWire(c, node)
	}
}

// looksUp reports whether node, a property access, is the callee of a call
// that looks a service up.
func (p *servicePlan) looksUp(node *ast.Node) bool {
	return node.Parent != nil && p.lookups[node.Parent] && node.Parent.Expression() == node
}

// addServiceLookup adds the service that call looks up to info's
// requirements.
func (a *analyzer) addServiceLookup(info *effectInfo, call *ast.Node) {
	identifier := serviceLookup(a.checker, call)
	if identifier == nil {
		return
	}
	info.requirements = append(info.requirements, identifier)
	if info.effect == nil {
		info.effect = effectInterface(a.checker, a.checker.GetApparentType(a.checker.GetTypeAtLocation(call.Expression().Expression())))
	}
}

// visitServiceLookup turns the callee `Users.find` of a service call into
// `(yield* Users).find`, which reads the member from the implementation in
// the current Context.
func (tx *lowering) visitServiceLookup(node *ast.Node) *ast.Node {
	f := tx.Factory()
	receiver := tx.Visitor().VisitNode(node.Expression())
	lookup := f.NewParenthesizedExpression(f.NewYieldExpression(f.NewToken(ast.KindAsteriskToken), receiver))
	tx.EmitContext().SetOriginal(lookup, node.Expression())
	access := f.UpdatePropertyAccessExpression(node.AsPropertyAccessExpression(), lookup, nil /*questionDotToken*/, node.Name(), ast.NodeFlagsNone)
	return access
}

// visitEffectValueCall turns `p()`, where p is an Effect value, into
// `yield* p`.
func (tx *lowering) visitEffectValueCall(node *ast.Node) *ast.Node {
	f := tx.Factory()
	callee := tx.Visitor().VisitNode(node.Expression())
	run := f.NewYieldExpression(f.NewToken(ast.KindAsteriskToken), callee)
	tx.EmitContext().SetOriginal(run, node)
	return run
}

// visitServiceClass turns a service class into a class that extends the
// runtime's key class for its key (design D45):
//
//	abstract class Users { abstract find(id: string): … }
//
// becomes
//
//	class Users extends runtime.$service("app/users#Users") {}
//
// The type-only abstract members are erased later by the TypeScript
// transform. A service with `static readonly default` extends
// runtime.$reference(key, () => Flags.default) instead, a Context.Reference
// that needs no provider (D40). A control operation also registers one
// kernel operation per abstract method, after the class:
// runtime.$control(Approval, ["approve"]).
func (tx *lowering) visitServiceClass(node *ast.Node) *ast.Node {
	f := tx.Factory()
	ec := tx.EmitContext()
	key := tx.plan.keys[node]

	saved := tx.inEffect
	tx.inEffect = false
	members := tx.Visitor().VisitNodes(node.MemberList())
	tx.inEffect = saved

	helper, arguments := "$service", []*ast.Node{f.NewStringLiteral(key, ast.TokenFlagsNone)}
	if hasStaticDefault(node) {
		// runtime.$reference(key, () => Flags.default)
		helper = "$reference"
		read := f.NewPropertyAccessExpression(f.NewIdentifier(node.Name().Text()), nil /*questionDotToken*/, f.NewIdentifier("default"), ast.NodeFlagsNone)
		arguments = append(arguments, f.NewArrowFunction(nil /*modifiers*/, nil /*typeParameters*/, f.NewNodeList(nil), nil /*type*/, nil /*fullSignature*/, f.NewToken(ast.KindEqualsGreaterThanToken), read))
	}
	base := f.NewCallExpression(tx.runtimeMember(helper), nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList(arguments), ast.NodeFlagsNone)
	extends := f.NewHeritageClause(ast.KindExtendsKeyword, f.NewNodeList([]*ast.Node{f.NewExpressionWithTypeArguments(base, nil /*typeArguments*/)}))
	class := f.UpdateClassDeclaration(node.AsClassDeclaration(), node.Modifiers(), node.Name(), nil /*typeParameters*/, f.NewNodeList([]*ast.Node{extends}), members)
	ec.SetOriginal(class, node)
	if classKindOf(node) != controlClass {
		return class
	}

	var operations []*ast.Node
	for _, member := range node.Members() {
		if ast.IsMethodDeclaration(member) && !ast.IsStatic(member) && ast.IsIdentifier(member.Name()) {
			operations = append(operations, f.NewStringLiteral(member.Name().Text(), ast.TokenFlagsNone))
		}
	}
	register := f.NewCallExpression(tx.runtimeMember("$control"), nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{
		f.NewIdentifier(node.Name().Text()),
		f.NewArrayLiteralExpression(f.NewNodeList(operations), false /*multiLine*/),
	}), ast.NodeFlagsNone)
	statement := f.NewExpressionStatement(register)
	ec.SetOriginal(statement, node)
	return f.NewSyntaxList([]*ast.Node{class, statement})
}
