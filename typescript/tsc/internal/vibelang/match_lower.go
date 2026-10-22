package vibelang

import (
	"slices"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// A match without a default arm ends in $noMatch(subject) (match.go). When
// D209's proof covers the match, its subject's declared type is a finite
// union and nothing of it is left after the last arm, so no value of that
// type reaches $noMatch, and the lowering drops it (D294). It drops the last
// arm's tests too when the type proves them, as it proves a literal, typeof,
// in, Array.isArray, length or _tag test:
//
//	return match (phase) {                 { if (phase === "draft") { return "Draft" } }
//	  when "draft": "Draft";               { { return "Published" } }
//	  when "published": "Published";
//	}
//
// A class or a custom matcher, which $is and $extract call, decides at run
// time what the type does not prove: a structural object of a class type is
// no instance, and a custom matcher may refuse an instance. Such a value
// fails an arm that D209 counts as handling it and can reach the last arm.
// When an arm tests a class or a custom matcher, the last arm keeps its
// tests, each written as a check that ends the match as $noMatch does when it
// fails:
//
//	return match (e) {                     { if ($is(NotFound, e)) { return "missing" } }
//	  when NotFound: "missing";            { { runtime.$assertMatch($is(Timeout, e), e); return "retry" } }
//	  when Timeout: "retry";
//	}
//
// The check is a call, not a branch, so tests that reach every arm still
// cover every branch. The arm's declarations, its bindings and any extracted
// values, still run, so its names are bound as before. A match over any other
// type keeps the tests and $noMatch, which throws the proposal's TypeError at
// run time.

// exhaustivePlan records what the lowering leaves out of the matches D209
// proves exhaustive in one file.
type exhaustivePlan struct {
	unreachable map[*ast.Node]bool      // statements that call $noMatch, dropped
	untested    map[*ast.Node]bool      // if statements of a last arm, emitted as their blocks
	checked     map[*ast.Node]*ast.Node // if statements of a last arm, emitted as a check against the subject and their blocks
	kept        bool                    // some $noMatch call stays, so the file imports $noMatch
}

func newExhaustivePlan() exhaustivePlan {
	return exhaustivePlan{unreachable: map[*ast.Node]bool{}, untested: map[*ast.Node]bool{}, checked: map[*ast.Node]*ast.Node{}}
}

// planExhaustiveMatch records node, a $noMatch call, and the tests of the
// last arm before it when D209 proves the match exhaustive: untested, or
// checked when an arm tests a class or a custom matcher.
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
	checked := slices.ContainsFunc(statements[:i], testsByMatcher)
	subject := node.Arguments()[0]
	for arm := statements[i-1]; arm.Kind == ast.KindBlock; {
		body := arm.Statements()
		if len(body) == 0 {
			return
		}
		test := body[len(body)-1]
		if test.Kind != ast.KindIfStatement || test.AsIfStatement().ElseStatement != nil {
			return
		}
		if checked {
			p.checked[test] = subject
		} else {
			p.untested[test] = true
		}
		arm = test.AsIfStatement().ThenStatement
	}
}

// testsByMatcher reports whether statement, an arm of a match, tests the
// subject with a class or a custom matcher: whether a test in its chain of if
// statements, or a declaration between them, calls $is or $extract. What the
// arm runs once it matches doesn't count, nor does the subject's const.
func testsByMatcher(statement *ast.Node) bool {
	if statement.Kind != ast.KindBlock {
		return false
	}
	for _, s := range statement.Statements() {
		switch {
		case s.Kind == ast.KindVariableStatement && callsMatcher(s):
			return true
		case s.Kind == ast.KindIfStatement && (callsMatcher(s.AsIfStatement().Expression) || testsByMatcher(s.AsIfStatement().ThenStatement)):
			return true
		}
	}
	return false
}

// callsMatcher reports whether node calls $is or $extract, the runtime's
// tests of a class or a custom matcher, as the parser writes them.
func callsMatcher(node *ast.Node) bool {
	if ast.IsCallExpression(node) {
		callee := node.Expression()
		if callee.Kind == ast.KindIdentifier && callee.Flags&ast.NodeFlagsReparsed != 0 && (callee.Text() == "$is" || callee.Text() == "$extract") {
			return true
		}
	}
	return node.ForEachChild(callsMatcher)
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

// visitCheckedArm emits test, an if statement of a last arm whose tests stay,
// as its block led by runtime.$assertMatch(test, subject), which ends the
// match as $noMatch does when the test fails (D294).
func (tx *lowering) visitCheckedArm(test *ast.Node, subject *ast.Node) *ast.Node {
	f := tx.Factory()
	statement := test.AsIfStatement()
	check := f.NewExpressionStatement(tx.runtimeCall("$assertMatch", tx.Visitor().VisitNode(statement.Expression), f.DeepCloneNode(tx.Visitor().VisitNode(subject))))
	tx.EmitContext().SetOriginal(check, test)
	check.Loc = statement.Expression.Loc
	statements := []*ast.Node{check}
	if then := tx.Visitor().VisitNode(statement.ThenStatement); then != nil && then.Kind == ast.KindBlock {
		statements = append(statements, then.Statements()...)
	} else if then != nil {
		statements = append(statements, then)
	}
	block := f.NewBlock(f.NewNodeList(statements), true /*multiLine*/)
	tx.EmitContext().SetOriginal(block, statement.ThenStatement)
	block.Loc = statement.ThenStatement.Loc
	return block
}
