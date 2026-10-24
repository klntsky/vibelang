package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

func init() { fileChecks = append(fileChecks, checkEffectAvailable) }

// checkEffectAvailable reports EK1011 at each function of file that compiles
// to an Effect when no Effect type is in reach to type it, because effect is
// not installed where the program resolves it. Such a function's JavaScript
// imports effect, and without the Effect type its type and declaration were
// the plain ones, with no diagnostic. A function annotated with an Effect
// contract already reports the missing name (TS2503). A plain function keeps
// its plain behavior either way: only its Effect wrapper for TypeScript
// callers needs effect, and it is not reported.
func checkEffectAvailable(c *checker.Checker, file *ast.SourceFile) {
	a := analyzerFor(c)
	if a.programEffectInterface() != nil {
		return
	}
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if isLowerable(node) && node.Type() == nil && a.isEffect(node) && a.effectTypeOf(node, c.GetUnknownType()) == nil {
			d := report(c, functionNameNode(node), EK1011, functionDisplayName(node))
			note(d, "Install effect where this file can resolve it: the JavaScript VibeLang writes for {0} imports it.", functionDisplayName(node))
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// functionNameNode returns the name of fn, or of the variable whose
// initializer fn is, or fn itself.
func functionNameNode(fn *ast.Node) *ast.Node {
	if name := fn.Name(); name != nil {
		return name
	}
	if parent := fn.Parent; parent != nil && parent.Kind == ast.KindVariableDeclaration && parent.Name() != nil {
		return parent.Name()
	}
	return fn
}
