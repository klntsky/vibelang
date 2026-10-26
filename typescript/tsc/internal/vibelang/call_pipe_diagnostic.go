package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/astnav"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// EK4502 explains a missing pipe on the success value of a call. A real
// pipe member on a returned object keeps its ordinary meaning.
var EK4502 = message(4502, errorCategory, "{0} already ran and returned its success value, which has no pipe method.")

func init() {
	registry[EK4502.Code] = EK4502
	checker.RegisterDiagnosticsFilterHook(explainCallPipe)
}

func missingCallPipe(c *checker.Checker, node *ast.Node) *ast.Node {
	if !ast.IsPropertyAccessExpression(node) || node.Name().Text() != "pipe" || !inLowerableBody(node) || inHatch(node) {
		return nil
	}
	receiver := ast.SkipParentheses(node.Expression())
	if !ast.IsCallExpression(receiver) {
		return nil
	}
	t := c.GetTypeAtLocation(receiver)
	if t == nil || t.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown) != 0 || c.GetPropertyOfType(c.GetApparentType(t), "pipe") != nil {
		return nil
	}
	return receiver
}

func explainCallPipe(c *checker.Checker, file *ast.SourceFile, all []*ast.Diagnostic) []*ast.Diagnostic {
	if !isVibeLangFile(file) {
		return all
	}
	for i, d := range all {
		if d.Source() != "" || d.Code() != 2339 || diagnosticAlreadyExplained(c, file, d) {
			continue
		}
		node := astnav.GetTokenAtPosition(file, d.Pos())
		if node != nil && node.Parent != nil {
			if call := missingCallPipe(c, node.Parent); call != nil {
				replacement := newDiagnosticAt(optionsFor(c, file), file, d.Loc(), EK4502, nodeText(file, call))
				note(replacement, "Write {0} |> retry(%, policy), or pass the call by name: retry({0}, policy).", nodeText(file, call))
				all[i] = replacement
			}
		}
	}
	return all
}

// fileChecks may already explain an error with an EK diagnostic and cover
// its TypeScript follow-ons. Do not turn a covered follow-on into a fresh EK
// error before filterCoveredDiagnostics gets to remove it.
func diagnosticAlreadyExplained(c *checker.Checker, file *ast.SourceFile, d *ast.Diagnostic) bool {
	for _, covered := range diagnosticsStateOf(c).covered[file] {
		if covered.Pos() <= d.Pos() && d.End() <= covered.End() {
			return true
		}
	}
	return false
}
