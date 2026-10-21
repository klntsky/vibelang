package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/vibelangmapper"
)

// A generator with a JavaScript iterator return annotation (D271) compiles to
// the generator TypeScript would write. It follows D282 like any plain
// function (D296): its throw is a typed failure, so one that can escape it is
// a compile error, and its catch receives typed failures only, so an
// exception that its caller passes in with generator.throw() passes through.
//
//	function* steps(): Generator<number> {   // EK2003: can fail with Invalid
//	  throw new Invalid()
//	}
//
// Code that must speak a JavaScript protocol by throwing, such as a coroutine
// driven by generator.throw(), belongs in a module block (D283) or a .ts file.

// plainGeneratorFailure is EK2003's text for a generator with a JavaScript
// iterator annotation whose body can fail. {0} is the annotation, {1} the
// generator, and {2} the failures that can escape it.
var plainGeneratorFailure = EK2003.With("{0} is a JavaScript iterator annotation, but {1} can fail with {2}, and a plain generator never fails.")

func init() {
	fileChecks = append(fileChecks, checkPlainGenerators)
}

// isPlainGenerator reports whether fn is a generator function in a .vibe file,
// outside a hatch, whose return annotation is a JavaScript iterator type
// (D271).
func isPlainGenerator(fn *ast.Node) bool {
	if fn == nil || fn.Kind != ast.KindFunctionDeclaration && fn.Kind != ast.KindFunctionExpression || fn.Body() == nil {
		return false
	}
	if ast.GetFunctionFlags(fn)&ast.FunctionFlagsGenerator == 0 {
		return false
	}
	return isVibeLangFile(ast.GetSourceFileOfNode(fn)) && !inHatch(fn) && vibelangmapper.HasJavaScriptIteratorReturn(fn)
}

// inPlainGeneratorBody reports whether node is in the body of a plain
// generator, and not in a function nested there.
func inPlainGeneratorBody(node *ast.Node) bool {
	for n := node; n.Parent != nil; n = n.Parent {
		if isHatchArgument(n) {
			return false
		}
		if fn := n.Parent; isFunctionBoundary(fn) {
			return n == fn.Body() && isPlainGenerator(fn)
		}
	}
	return false
}

// checkPlainGenerators reports EK2003 at the annotation of each plain
// generator whose body can fail (D296).
func checkPlainGenerators(c *checker.Checker, file *ast.SourceFile) {
	a := analyzerFor(c)
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if isPlainGenerator(node) {
			// Its calls are TypeScript's and run nothing, so only its throws can fail it.
			var thrown []*checker.Type
			forEachEscapingThrow(node, func(throw *ast.Node) { thrown = append(thrown, a.thrownType(throw)) })
			if len(thrown) > 0 {
				d := report(c, node.Type(), plainGeneratorFailure, nodeText(file, node.Type()), functionDisplayName(node), c.TypeToString(c.GetUnionType(thrown)))
				note(d, "Remove the annotation to make it a Stream producer, whose throws fail the Stream.")
				note(d, "Or write a generator that speaks a JavaScript protocol by throwing in a module block or a .ts file.")
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}
