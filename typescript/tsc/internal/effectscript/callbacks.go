package effectscript

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// A JavaScript function such as Array.prototype.map calls its callback
// synchronously, on a stack frame the runtime cannot pause or capture, so a
// callback passed where a plain function is expected must compile to plain
// JavaScript (design D9, D10). A lambda there that performs an Effect
// operation, and an EffectScript function passed by name that can pause, are
// ES4301:
//
//	error ES4301: this callback can pause (Users.find), but Array.prototype.map calls it synchronously.
//	  Use Effect.forEach(ids, (id) => Users.find(id)) or a for...of loop.

func init() {
	fileChecks = append(fileChecks, checkCallbacks)
}

// pausingCallback is ES4301's text for a lambda, which names what pauses.
var pausingCallback = ES4301.With("this callback can pause ({0}), but {1} calls it synchronously.")

func checkCallbacks(c *checker.Checker, file *ast.SourceFile) {
	a := analyzerFor(c)
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		switch {
		case isLambda(node):
			if a.finalForm(node) == formPlain {
				if pause := a.firstPause(node); pause != nil {
					call, _ := ownerCall(node)
					d := report(c, node, pausingCallback, nodeText(file, pause.Expression()), calleeName(c, file, call))
					note(d, "Use {0} or a for...of loop.", forEachFix(file, call, node))
				}
			}
		case node.Kind == ast.KindIdentifier:
			if a.finalConversion(node) == convertRejected {
				call, _ := ownerCall(node)
				d := report(c, node, ES4301, node.Text(), calleeName(c, file, call))
				note(d, "Use {0} or a for...of loop.", forEachFix(file, call, node))
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// calleeName names what call calls, for a diagnostic: Array.prototype.map
// for a method an interface or class declares, else the callee as written.
func calleeName(c *checker.Checker, file *ast.SourceFile, call *ast.Node) string {
	if call == nil {
		return "its caller"
	}
	if signature := c.GetResolvedSignature(call); signature != nil {
		if d := signature.Declaration(); d != nil && (ast.IsMethodSignatureDeclaration(d) || ast.IsMethodDeclaration(d)) && d.Parent != nil {
			if owner := d.Parent.Name(); owner != nil && ast.IsIdentifier(owner) && ast.IsIdentifier(d.Name()) {
				return owner.Text() + ".prototype." + d.Name().Text()
			}
		}
	}
	return nodeText(file, call.Expression())
}

// forEachFix writes the Effect.forEach call that replaces call, a call such
// as ids.map(f) whose callback f can pause.
func forEachFix(file *ast.SourceFile, call *ast.Node, callback *ast.Node) string {
	if call != nil && ast.IsPropertyAccessExpression(call.Expression()) {
		return "Effect.forEach(" + nodeText(file, call.Expression().Expression()) + ", " + nodeText(file, callback) + ")"
	}
	return "Effect.forEach"
}

// nodeText returns the source text of node, without leading trivia.
func nodeText(file *ast.SourceFile, node *ast.Node) string {
	return file.Text()[scanner.SkipTrivia(file.Text(), node.Pos()):node.End()]
}
