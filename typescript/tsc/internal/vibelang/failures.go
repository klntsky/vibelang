package vibelang

import (
	"slices"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// This file computes what code can fail with and what it needs. A throw adds
// the static type of the thrown value to E, widened as TypeScript widens a
// return type, so `throw "bad"` adds string. A catch handles every expected
// failure of its try block (D13, D28), so a try statement with a catch passes
// on only what its catch block itself raises; requirements are never handled
// by a catch. A finally block adds its own failures and requirements (D15). E
// of a function is what can escape its body.

// flowOf collects what running node adds to E and R. node is a statement,
// block, or expression in the body of one function; the functions and classes
// nested in it, and the expressions it passes by name (D147), do not run as
// part of it.
func (a *analyzer) flowOf(node *ast.Node) *effectInfo {
	info := &effectInfo{}
	var visit func(n *ast.Node) bool
	visit = func(n *ast.Node) bool {
		switch {
		case isFunctionBoundary(n):
		case n != node && a.isByName(n):
			// Passed as a program (D147): it runs where the callee runs it.
		case isHatchArgument(n):
			// TypeScript inside a boundary hatch (D148): nothing there runs.
		case isHandleStatement(n):
			info.add(a.handleFlow(n)) // try { ... } handle Control.op(...) { ... } (handles.go)
		case n.Kind == ast.KindTryStatement:
			info.add(a.tryFlow(n))
		case isProvideBlock(n):
			info.add(a.provideFlow(n)) // provide Layer { ... } (provide.go)
		case n.Kind == ast.KindThrowStatement:
			n.ForEachChild(visit)
			info.addError(a.thrownType(n), n.Expression()) // a typed failure everywhere outside a hatch (D282)
		case n.Kind == ast.KindCallExpression && a.getsScope(n):
			n.ForEachChild(visit)
			a.addScoped(info, n) // a fork or acquisition in a Scope region (scopes.go)
		case ast.IsTaggedTemplateExpression(n):
			n.ForEachChild(visit)
			a.addCall(info, n)
		case n.Kind == ast.KindCallExpression:
			n.ForEachChild(visit)
			a.addServiceLookup(info, n)
			a.addCall(info, n)
			a.addFailures(info, n)
		case n.Kind == ast.KindForOfStatement:
			n.ForEachChild(visit)
			a.addStreamLoop(info, n)
		default:
			n.ForEachChild(visit)
		}
		if program := a.returnedProgram(n); program != nil {
			info.addError(channelType(a.checker, program, "_E"), n)
			info.addRequirement(channelType(a.checker, program, "_R"), n)
			if info.effect == nil {
				info.effect = effectInterface(a.checker, program)
			}
		}
		return false
	}
	visit(node)
	return info
}

// thrownType returns the expected failure that throw adds to E.
func (a *analyzer) thrownType(throw *ast.Node) *checker.Type {
	c := a.checker
	t := c.GetWidenedType(c.GetWidenedLiteralType(c.GetTypeOfExpressionBeforeWidening(throw.Expression())))
	if e, ok := causeErrorOf(c, t); ok {
		return e // throw cause: every reason it holds (D177, primitives.go)
	}
	return t
}

// hasEscapingThrow reports whether a throw statement in fn's own body can end
// fn: a throw that no try block of fn with a catch clause encloses.
func hasEscapingThrow(fn *ast.Node) bool {
	var visit func(n *ast.Node, caught bool) bool
	visit = func(n *ast.Node, caught bool) bool {
		switch {
		case isFunctionBoundary(n):
			return false
		case n.Kind == ast.KindThrowStatement && !caught:
			return true
		case n.Kind == ast.KindTryStatement:
			try := n.AsTryStatement()
			return visit(try.TryBlock, caught || catches(n)) ||
				try.CatchClause != nil && visit(try.CatchClause, caught) ||
				try.FinallyBlock != nil && visit(try.FinallyBlock, caught)
		}
		return n.ForEachChild(func(child *ast.Node) bool { return visit(child, caught) })
	}
	return visit(fn.Body(), false /*caught*/)
}

// tryFlow collects what can escape a try statement.
func (a *analyzer) tryFlow(node *ast.Node) *effectInfo {
	try := node.AsTryStatement()
	body := a.tryBlockFlow(try.TryBlock)
	escapes := &effectInfo{requirements: slices.Clone(body.requirements), requirementSources: slices.Clone(body.requirementSources), effect: body.effect}
	if try.CatchClause != nil {
		escapes.add(a.flowOf(try.CatchClause.AsCatchClause().Block))
	} else {
		escapes.errors = append(escapes.errors, body.errors...)
		escapes.errorSources = append(escapes.errorSources, body.errorSources...)
	}
	if try.FinallyBlock != nil {
		escapes.add(a.flowOf(try.FinallyBlock))
	}
	return escapes
}

// tryBlockFlow is flowOf for the try block of a try statement, remembered:
// its errors are also the type of the catch clause's variable. Inside a call
// cycle that is being solved, it is read again in each round.
func (a *analyzer) tryBlockFlow(block *ast.Node) *effectInfo {
	if info, ok := a.tryBlocks[block]; ok {
		if info == nil {
			return &effectInfo{} // being collected; see catchVariableType
		}
		return info
	}
	if fn := ownerFunction(block); fn != nil {
		if _, solving := a.approx[fn]; solving {
			return a.flowOf(block)
		}
	}
	a.tryBlocks[block] = nil
	info := a.flowOf(block)
	a.tryBlocks[block] = info
	return info
}

// addCall adds what call contributes to info when the call runs an Effect.
// It reads the channels of a TypeScript callee from its Effect return type and
// recurses into a .vibe callee instead of asking for that callee's return type,
// which may be the one the checker is inferring right now. A call cycle adds
// nothing, so E and R are incomplete on cycles until the checker solves them
// as a fixed point.
func (a *analyzer) addCall(info *effectInfo, call *ast.Node) {
	if !a.runs(call) {
		return
	}
	sig := a.checker.GetResolvedSignature(call)
	// A call to a generic .vibe function, such as one whose callback accepts any
	// contract (D12), adds the callee's E and R as this call instantiates
	// them, so it reads its instantiated return type; the callee's own body
	// knows only its type parameters. A callee with a contract adds what the
	// contract says.
	if callee := implementationOf(sig.Declaration()); callee != nil && isLowerable(callee) && sig.Target() == nil && !a.isFactory(callee) && (a.isSplittable(callee) || a.isEffect(callee)) {
		if e, r, ok := a.calleeContract(callee, call, sig); ok {
			info.addError(e, call)
			info.addRequirement(r, call)
		} else if calleeInfo := a.effectOf(callee); calleeInfo != nil {
			info.addFrom(calleeInfo, call)
		}
		return
	}
	// D269 declared factories and S3-3 plain factories read returned channels.
	// A plain factory returns an existing program: its E/R belong to that
	// program's type, not to the JavaScript body that constructs it.
	t := runningCallResult(a.checker, call, sig)
	if e := channelType(a.checker, t, "_E"); e != nil {
		info.addError(e, call)
	}
	if r := channelType(a.checker, t, "_R"); r != nil {
		info.addRequirement(r, call)
	}
	if info.effect == nil {
		info.effect = effectInterface(a.checker, t)
	}
}

// catchVariableType types the variable of a catch clause in a function that
// runs Effects: it is a union of the expected failures the try block can
// raise, and never when it raises none. A plain function catches every
// JavaScript exception as unknown (D260).
func catchVariableType(c *checker.Checker, declaration *ast.Node) *checker.Type {
	if t := causeVariableType(c, declaration); t != nil {
		return t // catch (cause: Cause) (primitives.go)
	}
	clause := declaration.Parent
	if clause == nil || clause.Kind != ast.KindCatchClause || !inLowerableBody(clause.Parent) && !inPlainGeneratorBody(clause.Parent) || isHandleStatement(clause.Parent) {
		return nil
	}
	info := analyzerFor(c).tryBlockFlow(clause.Parent.AsTryStatement().TryBlock)
	return c.GetUnionType(info.errors)
}

func init() {
	fileChecks = append(fileChecks, checkDeadCatches)
}

// blockRunsEffects tests operations, rather than E/R or effectInfo.effect:
// a call through a written contract and a Stream loop can run Effects without
// supplying an Effect interface to flowOf. Nested functions and lazy program
// arguments do not run as part of this block.
func (a *analyzer) blockRunsEffects(block *ast.Node) bool {
	var visit func(*ast.Node) bool
	visit = func(n *ast.Node) bool {
		if isFunctionBoundary(n) || isHatchArgument(n) || n != block && a.isByName(n) {
			return false
		}
		if a.returnedProgram(n) != nil || isInvocation(n) && a.performs(n) ||
			isHandleStatement(n) ||
			isProvideBlock(n) || maskWord(n) != "" ||
			n.Kind == ast.KindVariableDeclarationList && isUsingList(n) ||
			isStreamLoop(a.checker, n) {
			return true
		}
		return n.ForEachChild(visit)
	}
	return visit(block)
}

// deadCatch is EK2004's text for a catch whose try block runs no Effect and
// fails with no expected error (D282).
var deadCatch = EK2004.With("this catch can never run: nothing in its try block fails with an expected error, and a JavaScript exception passes through it as a defect.")

// checkDeadCatches reports EK2004 when a try block's E is never (D260, D282):
// the catch receives typed failures only, so defects, JavaScript exceptions
// included, pass through it. The warning covers TypeScript's errors about the
// never binding.
func checkDeadCatches(c *checker.Checker, file *ast.SourceFile) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if node.Kind == ast.KindTryStatement && catches(node) && !isCauseCatch(node) && (inLowerableBody(node) || inPlainGeneratorBody(node)) {
			try := node.AsTryStatement()
			a := analyzerFor(c)
			info := a.tryBlockFlow(try.TryBlock)
			if c.GetUnionType(info.errors).Flags()&checker.TypeFlagsNever != 0 {
				// A catch receives typed failures only, in a plain function too
				// (D282): a JavaScript exception is a defect that passes through.
				m := deadCatch
				if a.blockRunsEffects(try.TryBlock) {
					m = EK2004
				}
				d := reportAt(c, file, scanner.GetRangeOfTokenAtPosition(file, try.CatchClause.Pos()), m)
				if d != nil {
					note(d, "Use a form that can fail, such as Option.getOrElse or Schema.decodeUnknownEffect, or bound the call with try.")
					cover(c, try.CatchClause)
				}
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}
