package effectscript

import (
	"slices"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// This file computes what code can fail with and what it needs, through try
// statements. A catch handles every expected failure of its try block (D13,
// D28), so a try statement with a catch passes on only what its catch block
// itself raises; requirements are never handled by a catch. A finally block
// adds its own failures and requirements (D15). E of a function is what can
// escape its body.

// flowOf collects what running node adds to E and R. node is a statement,
// block, or expression in the body of one function; the functions and classes
// nested in it do not run as part of it.
func (a *analyzer) flowOf(node *ast.Node) *effectInfo {
	info := &effectInfo{}
	var visit func(n *ast.Node) bool
	visit = func(n *ast.Node) bool {
		switch {
		case isFunctionBoundary(n):
		case n.Kind == ast.KindTryStatement:
			info.add(a.tryFlow(n))
		case n.Kind == ast.KindCallExpression:
			n.ForEachChild(visit)
			a.addServiceLookup(info, n)
			a.addCall(info, n)
		default:
			n.ForEachChild(visit)
		}
		return false
	}
	visit(node)
	return info
}

// tryFlow collects what can escape a try statement.
func (a *analyzer) tryFlow(node *ast.Node) *effectInfo {
	try := node.AsTryStatement()
	body := a.tryBlockFlow(try.TryBlock)
	escapes := &effectInfo{requirements: slices.Clone(body.requirements), effect: body.effect}
	if try.CatchClause != nil {
		escapes.add(a.flowOf(try.CatchClause.AsCatchClause().Block))
	} else {
		escapes.errors = append(escapes.errors, body.errors...)
	}
	if try.FinallyBlock != nil {
		escapes.add(a.flowOf(try.FinallyBlock))
	}
	return escapes
}

// tryBlockFlow is flowOf for the try block of a try statement, remembered:
// its errors are also the type of the catch clause's variable.
func (a *analyzer) tryBlockFlow(block *ast.Node) *effectInfo {
	if info, ok := a.tryBlocks[block]; ok {
		if info == nil {
			return &effectInfo{} // being collected; see catchVariableType
		}
		return info
	}
	a.tryBlocks[block] = nil
	info := a.flowOf(block)
	a.tryBlocks[block] = info
	return info
}

// addCall adds what call contributes to info when the call runs an Effect.
// It reads the channels of a TypeScript callee from its Effect return type and
// recurses into an .es callee instead of asking for that callee's return type,
// which may be the one the checker is inferring right now. A call cycle adds
// nothing, so E and R are incomplete on cycles until the checker solves them
// as a fixed point.
func (a *analyzer) addCall(info *effectInfo, call *ast.Node) {
	if !a.runs(call) {
		return
	}
	sig := a.checker.GetResolvedSignature(call)
	if callee := implementationOf(sig.Declaration()); callee != nil && isLowerable(callee) {
		if calleeInfo := a.effectOf(callee); calleeInfo != nil {
			info.add(calleeInfo)
		}
		return
	}
	t := a.checker.GetReturnTypeOfSignature(sig)
	if e := channelType(a.checker, t, "_E"); e != nil {
		info.errors = append(info.errors, e)
	}
	if r := channelType(a.checker, t, "_R"); r != nil {
		info.requirements = append(info.requirements, r)
	}
	if info.effect == nil {
		info.effect = effectInterface(a.checker, t)
	}
}

// catchVariableType types the variable of a catch clause in a function that
// the lowering can turn into an Effect: it is a union of the expected failures
// the try block can raise, and never when it raises none. A catch elsewhere,
// such as at a module's top level, keeps TypeScript's meaning.
func catchVariableType(c *checker.Checker, declaration *ast.Node) *checker.Type {
	clause := declaration.Parent
	if clause == nil || clause.Kind != ast.KindCatchClause || !inLowerableBody(clause.Parent) {
		return nil
	}
	info := analyzerFor(c).tryBlockFlow(clause.Parent.AsTryStatement().TryBlock)
	return c.GetUnionType(info.errors)
}
