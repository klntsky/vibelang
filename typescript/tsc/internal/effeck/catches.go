package effeck

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// Typed catch clauses (D131). The parser turns
//
//	try { ... } catch (e: NotFound) { A } catch (e) { B }
//
// into one catch clause whose block tests $isFailure<NotFound>($caught), a
// type guard from the runtime, so the checker narrows each branch. The
// lowering appends the _tag of each member of the clause's type to the
// test: $isFailure($caught, "NotFound").

func init() { fileChecks = append(fileChecks, checkCatchClauses) }

// isFailureTest reports whether node is the test the parser writes for a
// typed catch clause.
func isFailureTest(node *ast.Node) bool {
	if node.Kind != ast.KindCallExpression {
		return false
	}
	callee := node.Expression()
	return callee.Kind == ast.KindIdentifier && callee.Flags&ast.NodeFlagsReparsed != 0 && callee.Text() == "$isFailure"
}

// failureTags returns the literal _tag of each member of t, and the first
// member that has none.
func failureTags(c *checker.Checker, t *checker.Type) (tags []string, untagged *checker.Type) {
	for _, member := range unionMembers(t) {
		tag := c.GetTypeOfPropertyOfType(member, "_tag")
		if tag == nil || tag.Flags()&checker.TypeFlagsStringLiteral == 0 {
			return nil, member
		}
		tags = append(tags, tag.AsLiteralType().Value().(string))
	}
	return tags, nil
}

// visitFailureTest lowers a typed catch clause's test, appending the tags of
// its type.
func (tx *lowering) visitFailureTest(node *ast.Node) *ast.Node {
	f := tx.Factory()
	visited := tx.Visitor().VisitEachChild(node)
	arguments := append([]*ast.Node{}, visited.Arguments()...)
	for _, tag := range tx.plan.failureTags[node] {
		arguments = append(arguments, f.NewStringLiteral(tag, ast.TokenFlagsNone))
	}
	lowered := f.UpdateCallExpression(visited.AsCallExpression(), visited.Expression(), nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList(arguments), ast.NodeFlagsNone)
	return lowered
}

// checkCatchClauses reports EK2105 at a typed catch clause whose type has a
// member without a literal _tag, and EK2106 at a clause after an untyped one.
func checkCatchClauses(c *checker.Checker, file *ast.SourceFile) {
	var visit func(n *ast.Node) bool
	visit = func(n *ast.Node) bool {
		if isFailureTest(n) {
			typeNode := n.TypeArguments()[0]
			if _, untagged := failureTags(c, c.GetTypeFromTypeNode(typeNode)); untagged != nil {
				d := report(c, typeNode, EK2105, scanner.GetTextOfNode(typeNode), c.TypeToString(untagged))
				note(d, "Catch tagged errors, such as an error declaration or Schema.SchemaError, or test the value in an untyped catch clause.")
			}
		}
		if n.Kind == ast.KindIfStatement && n.Expression().Kind == ast.KindTrueKeyword && n.Expression().Flags&ast.NodeFlagsReparsed != 0 {
			for rest := n.AsIfStatement().ElseStatement; rest != nil; {
				if rest.Kind == ast.KindIfStatement {
					report(c, rest.Expression(), EK2106)
					rest = rest.AsIfStatement().ElseStatement
				} else {
					if rest.Flags&ast.NodeFlagsReparsed == 0 { // not the throw that ends a chain
						report(c, rest, EK2106)
					}
					rest = nil
				}
			}
		}
		return n.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}
