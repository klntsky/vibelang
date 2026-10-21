package vibelang

import (
	"slices"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// A match without a default arm ends in $noMatch(subject) (match.go). When
// D209's proof covers the match, its subject's declared type is a finite
// union and nothing of it is left after the last arm, so the last arm's tests
// cannot fail and $noMatch cannot run. The lowering then emits the last arm
// without its tests and drops $noMatch (D294):
//
//	return match (phase) {                 { if (phase === "draft") { return "Draft" } }
//	  when "draft": "Draft";               { { return "Published" } }
//	  when "published": "Published";
//	}
//
// The arm's declarations, its bindings and any extracted values, still run,
// so its names are bound as before. A match over any other type keeps the
// tests and $noMatch, which throws the proposal's TypeError at run time.

// exhaustivePlan records what the lowering leaves out of the matches D209
// proves exhaustive in one file.
type exhaustivePlan struct {
	unreachable map[*ast.Node]bool // statements that call $noMatch, dropped
	untested    map[*ast.Node]bool // if statements of a last arm, emitted as their blocks
	kept        bool               // some $noMatch call stays, so the file imports $noMatch
}

func newExhaustivePlan() exhaustivePlan {
	return exhaustivePlan{unreachable: map[*ast.Node]bool{}, untested: map[*ast.Node]bool{}}
}

// planExhaustiveMatch records node, a $noMatch call, and the tests of the
// last arm before it when D209 proves the match exhaustive.
func (p *exhaustivePlan) planExhaustiveMatch(c *checker.Checker, node *ast.Node) {
	if patternHelper(node) != "$noMatch" {
		return
	}
	statement := node.Parent
	if !provenExhaustive(c, node) || statement == nil || statement.Parent == nil || statement.Parent.Kind != ast.KindBlock ||
		statement.Kind != ast.KindReturnStatement && statement.Kind != ast.KindExpressionStatement {
		p.kept = true
		return
	}
	p.unreachable[statement] = true
	// The parser writes each arm as a block whose tests are a chain of if
	// statements, each the last statement of the block before it, around
	// the arm's success statements (emitSteps, vibelang_match.go).
	statements := statement.Parent.Statements()
	i := slices.Index(statements, statement)
	if i < 1 || statements[i-1].Kind != ast.KindBlock {
		return // no arm, or only the subject's declaration
	}
	for arm := statements[i-1]; arm.Kind == ast.KindBlock; {
		body := arm.Statements()
		if len(body) == 0 {
			return
		}
		test := body[len(body)-1]
		if test.Kind != ast.KindIfStatement || test.AsIfStatement().ElseStatement != nil {
			return
		}
		p.untested[test] = true
		arm = test.AsIfStatement().ThenStatement
	}
}

// withoutUnusedNoMatch removes $noMatch from names, a file's prelude uses,
// when every $noMatch call of the file is dropped.
func (p *exhaustivePlan) withoutUnusedNoMatch(names []string) []string {
	if p.kept || len(p.unreachable) == 0 {
		return names
	}
	return slices.DeleteFunc(names, func(name string) bool { return name == "$noMatch" })
}

// visitUntestedArm emits test, an if statement of a last arm that cannot
// fail, as its block.
func (tx *lowering) visitUntestedArm(test *ast.Node) *ast.Node {
	return tx.Visitor().VisitNode(test.AsIfStatement().ThenStatement)
}
