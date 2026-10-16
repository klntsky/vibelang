package effectscript

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// servicePlan records what the lowering does with services in one file.
type servicePlan struct {
	keys map[*ast.Node]string // service classes, by declaration, and their keys
}

func newServicePlan() servicePlan {
	return servicePlan{keys: map[*ast.Node]string{}}
}

// planServiceClass records the key of node when it declares a service.
func (p *servicePlan) planServiceClass(c *checker.Checker, node *ast.Node) {
	if classKindOf(node) != ordinaryClass {
		p.keys[node] = servicesFor(c).key(node)
	}
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
// transform.
func (tx *lowering) visitServiceClass(node *ast.Node) *ast.Node {
	f := tx.Factory()
	key := tx.plan.keys[node]

	saved := tx.inEffect
	tx.inEffect = false
	members := tx.Visitor().VisitNodes(node.MemberList())
	tx.inEffect = saved

	base := f.NewCallExpression(tx.runtimeMember("$service"), nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{f.NewStringLiteral(key, ast.TokenFlagsNone)}), ast.NodeFlagsNone)
	extends := f.NewHeritageClause(ast.KindExtendsKeyword, f.NewNodeList([]*ast.Node{f.NewExpressionWithTypeArguments(base, nil /*typeArguments*/)}))
	class := f.UpdateClassDeclaration(node.AsClassDeclaration(), node.Modifiers(), node.Name(), nil /*typeParameters*/, f.NewNodeList([]*ast.Node{extends}), members)
	tx.EmitContext().SetOriginal(class, node)
	return class
}
