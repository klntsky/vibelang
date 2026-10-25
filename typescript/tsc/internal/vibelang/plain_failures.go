package vibelang

import (
	"context"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/ls"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// A plain return annotation (D67) makes a function plain JavaScript because
// it never fails and never needs a service (D282). `throw` is a typed failure
// everywhere outside a hatch, so a throw that can escape such a function is a
// compile error, as running an Effect in it is: both are EK2003.
//
//	const validate = (value: string): string => {   // EK2003: can fail with Invalid
//	  if (value === "") throw new Invalid({ value })
//	  return value
//	}
//
// The fixes are an Effect contract, `string!Invalid!`, or a hatch, whose
// TypeScript keeps JavaScript's throw. An exception that TypeScript code the
// function calls throws is not a typed failure: it propagates unchanged and
// becomes a defect where it reaches an effectful caller.

// plainContractFailure is EK2003's text for a function with a plain return
// annotation whose body can fail. {0} is the annotation, {1} the function, and
// {2} the failures that can escape it.
var plainContractFailure = EK2003.With("{0} is a plain return annotation, but {1} can fail with {2}, and a plain function never fails.")

func init() {
	ls.RegisterCodeFixProvider(plainContractFailureFix)
	fileChecks = append(fileChecks, checkPlainCodeThrows)
}

// plainFailures returns the expected failures that can escape the body of fn,
// a function with a plain return annotation, as one type, or nil when none
// can.
func (a *analyzer) plainFailures(fn *ast.Node) *checker.Type {
	if fn == nil || fn.Body() == nil || !isLowerable(fn) || !a.hasPlainContract(fn) || fn.Type() == nil {
		return nil
	}
	failures := a.checker.GetUnionType(a.flowOf(fn.Body()).errors)
	if failures.Flags()&checker.TypeFlagsNever != 0 {
		return nil
	}
	return failures
}

// reportFailingPlainContract reports EK2003 at the plain return annotation of
// fn when fn's body can fail, and reports whether it did.
func (a *analyzer) reportFailingPlainContract(fn *ast.Node) bool {
	failures := a.plainFailures(fn)
	if failures == nil {
		return false
	}
	c := a.checker
	annotation := fn.Type()
	written := scanner.GetTextOfNode(annotation)
	errors := c.TypeToString(failures)
	d := report(c, annotation, plainContractFailure, written, functionDisplayName(fn), errors)
	note(d, "Write {0} to make it an Effect function that fails with {1}.", bangText(written, errors), errors)
	note(d, "Or throw inside a hatch, whose TypeScript keeps JavaScript's throw: sync makes the exception a defect ({0}), and try makes it an UnknownError.", bangText(written, "never", "never"))
	return true
}

// plainContractFailureFix writes the Effect contract that EK2003's
// plainContractFailure offers: `: string` becomes `: string!Invalid!`.
var plainContractFailureFix = &ls.CodeFixProvider{
	Source:     diagnosticSource,
	ErrorCodes: []int32{EK2003.Code},
	GetCodeActions: func(ctx context.Context, fix *ls.CodeFixContext) ([]*ls.CodeAction, error) {
		annotation := reportedNode(fix.SourceFile, fix.Span, func(node *ast.Node) bool {
			return node.Parent != nil && isLowerable(node.Parent) && node.Parent.Type() == node
		})
		if annotation == nil {
			return nil, nil
		}
		c, done := fix.Program.GetTypeCheckerForFile(ctx, fix.SourceFile)
		defer done()
		failures := analyzerFor(c).plainFailures(annotation.Parent)
		if failures == nil {
			return nil, nil
		}
		contract := bangText(scanner.GetTextOfNode(annotation), c.TypeToString(failures))
		return []*ls.CodeAction{{
			Description: "Write " + contract,
			Changes:     replaceRange(fix, nodeStart(fix.SourceFile, annotation), annotation.End(), contract),
		}}, nil
	},
}

// failingCallback is EK4301's text for a lambda that can fail where a plain
// function is expected. {0} is the failures, {1} the callee.
var failingCallback = EK4301.With("this callback can fail with {0}, but {1} calls it synchronously, and a plain function never fails.")

// failingMethod is EK4301's text for a method that can fail and implements a
// plain-function member. {0} is the method, {1} the failures, {2} the member's
// owner.
var failingMethod = EK4301.With("{0} can fail with {1}, but {2} declares it as a plain function, which never fails.")

// escapingFailures returns the expected failures that can escape fn's body,
// as one type, or nil when none can.
func (a *analyzer) escapingFailures(fn *ast.Node) *checker.Type {
	if fn == nil || fn.Body() == nil {
		return nil
	}
	failures := a.checker.GetUnionType(a.flowOf(fn.Body()).errors)
	if failures.Flags()&checker.TypeFlagsNever != 0 {
		return nil
	}
	return failures
}

// reportFailingCallback reports EK4301 at fn, a lambda where a plain function
// is expected, as an argument of call or, when call is nil, through another
// contextual type, when it can fail, and reports whether it did.
func (a *analyzer) reportFailingCallback(file *ast.SourceFile, fn *ast.Node, call *ast.Node) bool {
	failures := a.escapingFailures(fn)
	if failures == nil {
		return false
	}
	c := a.checker
	d := report(c, fn, failingCallback, c.TypeToString(failures), calleeName(c, file, call))
	if call != nil {
		note(d, "Use {0} or a for...of loop.", forEachFix(c, file, call, fn))
	}
	return true
}

// reportFailingMethod reports EK4301 at the name of fn, a method that
// implements owner's plain-function member, when it can fail. A method with a
// plain return annotation has EK2003 instead.
func (a *analyzer) reportFailingMethod(file *ast.SourceFile, fn *ast.Node, owner string) {
	if fn.Type() != nil {
		return
	}
	if failures := a.escapingFailures(fn); failures != nil {
		report(a.checker, fn.Name(), failingMethod, nodeText(file, fn.Name()), a.checker.TypeToString(failures), owner)
	}
}

// reportFactoryFailures reports EK2115 at each throw that can end fn, a D269
// factory: its body runs as plain JavaScript when fn is called, and a plain
// function never fails (D282). The returned program fails instead.
func (a *analyzer) reportFactoryFailures(file *ast.SourceFile, fn *ast.Node, name string) {
	forEachEscapingThrow(fn, func(throw *ast.Node) {
		text := nodeText(file, throw)
		d := report(a.checker, throw, EK2115, name, "throw")
		note(d, "Fail when the program runs instead: return () => { {0} }.", text)
	})
}

// forEachEscapingThrow calls cb with each throw statement in fn's own body
// that no try block of fn with a catch clause encloses.
func forEachEscapingThrow(fn *ast.Node, cb func(throw *ast.Node)) {
	if fn.Body() != nil {
		forEachEscapingThrowIn(fn.Body(), cb)
	}
}

// forEachEscapingThrowIn calls cb with each throw statement in root, outside
// the functions and classes nested in it, that no try block in root with a
// catch clause encloses.
func forEachEscapingThrowIn(root *ast.Node, cb func(throw *ast.Node)) {
	var visit func(n *ast.Node, caught bool) bool
	visit = func(n *ast.Node, caught bool) bool {
		switch {
		case isFunctionBoundary(n):
			return false
		case n.Kind == ast.KindThrowStatement && !caught:
			cb(n)
			return false
		case n.Kind == ast.KindTryStatement:
			try := n.AsTryStatement()
			visit(try.TryBlock, caught || catches(n))
			if try.CatchClause != nil {
				visit(try.CatchClause, caught)
			}
			if try.FinallyBlock != nil {
				visit(try.FinallyBlock, caught)
			}
			return false
		}
		n.ForEachChild(func(child *ast.Node) bool { return visit(child, caught) })
		return false
	}
	visit(root, false /*caught*/)
}

// A constructor, an accessor and a class static block run as plain
// JavaScript wherever they stand, as a module's top level does: none of them
// is an Effect, so none of them can fail (D282). A throw is a typed failure
// everywhere outside a hatch or a module declaration, so one that can escape
// such code is a compile error, as it is in a function with a plain return
// annotation: EK2003, and EK1001 at a module's top level, where running a
// program is EK1001 too.
//
//	error Invalid { value: number }
//	class Port {
//	  constructor(readonly value: number) {
//	    if (value < 0) throw new Invalid({ value })   // EK2003
//	  }
//	}
//
// A throw that the code's own catch handles stays inside it.

// plainCodeFailure is EK2003's text for a throw that can escape a constructor,
// an accessor or a static block. {0} is what it throws, {1} names the code.
var plainCodeFailure = EK2003.With("This throw fails with {0}, but {1} runs as plain JavaScript, which never fails.")

// topLevelFailure is EK1001's text for a throw that can escape a module's top
// level.
var topLevelFailure = EK1001.With("This throw fails a program, but a module's top level is not inside one.")

// moduleThrowNote names the escape for code that must throw a JavaScript
// exception.
const moduleThrowNote = "Code that must throw a JavaScript exception, such as a test's throwing getter, belongs in a module declaration, whose TypeScript keeps JavaScript's throw (D283)."

// checkPlainCodeThrows reports each throw that can escape a module's top level
// (EK1001) or a constructor, accessor or static block (EK2003) in file.
func checkPlainCodeThrows(c *checker.Checker, file *ast.SourceFile) {
	a := analyzerFor(c)
	forEachEscapingThrowIn(file.AsNode(), func(throw *ast.Node) {
		d := report(c, throw, topLevelFailure)
		note(d, "Throw inside a function that a program runs, such as main.")
		note(d, moduleThrowNote)
	})
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if isHatchArgument(node) {
			return false // a hatch's function is TypeScript (D148)
		}
		body := node.Body()
		if node.Kind == ast.KindClassStaticBlockDeclaration {
			body = node.AsClassStaticBlockDeclaration().Body // a static block has no Body()
		}
		switch node.Kind {
		case ast.KindConstructor, ast.KindGetAccessor, ast.KindSetAccessor, ast.KindClassStaticBlockDeclaration:
			if body != nil {
				forEachEscapingThrowIn(body, func(throw *ast.Node) {
					failure := c.TypeToString(a.thrownType(throw))
					d := report(c, throw, plainCodeFailure, failure, plainCodeName(node))
					switch node.Kind {
					case ast.KindConstructor:
						note(d, "Check before constructing, in a function whose Effect contract names {0}.", failure)
					case ast.KindClassStaticBlockDeclaration:
						note(d, "Throw inside a function that a program runs, such as main.")
					default:
						note(d, "Write a method instead, whose Effect contract can name {0}.", failure)
					}
					note(d, moduleThrowNote)
				})
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// plainCodeName names node, a constructor, accessor or class static block, in
// a diagnostic: "the constructor of Port", "get value", "the static block of
// Port".
func plainCodeName(node *ast.Node) string {
	file := ast.GetSourceFileOfNode(node)
	switch node.Kind {
	case ast.KindGetAccessor:
		return "get " + nodeText(file, node.Name())
	case ast.KindSetAccessor:
		return "set " + nodeText(file, node.Name())
	}
	what := "the constructor"
	if node.Kind == ast.KindClassStaticBlockDeclaration {
		what = "the static block"
	}
	if class := node.Parent; class != nil && class.Name() != nil {
		return what + " of " + nodeText(file, class.Name())
	}
	return what + " of this class"
}

// inCatchingTry reports whether a try block with a catch clause in the
// function that holds node encloses node. In a plain function only there is
// a trusted call's listed exception a typed failure, which the catch receives
// (D54); elsewhere it propagates unchanged, as JavaScript's (D282).
func inCatchingTry(node *ast.Node) bool {
	for n := node; n.Parent != nil && !isFunctionBoundary(n.Parent); n = n.Parent {
		if parent := n.Parent; parent.Kind == ast.KindTryStatement && parent.AsTryStatement().TryBlock == n && catches(parent) {
			return true
		}
	}
	return false
}
