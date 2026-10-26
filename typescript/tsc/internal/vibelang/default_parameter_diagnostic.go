package vibelang

import (
	"slices"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/astnav"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

func init() { checker.RegisterDiagnosticsFilterHook(explainInferredDefault) }

// D282 keeps defaults as JavaScript. An unannotated parameter may legitimately
// hold a cold program, so explain it only when TypeScript reports its inferred
// Effect value being used as another type. Explicit Effect defaults stay valid.
func explainInferredDefault(c *checker.Checker, file *ast.SourceFile, all []*ast.Diagnostic) []*ast.Diagnostic {
	if !isVibeLangFile(file) {
		return all
	}
	seen := map[*ast.Node]bool{}
	var added []*ast.Diagnostic
	for _, diagnostic := range all {
		if diagnostic.Source() != "" || !slices.Contains([]int32{2322, 2345, 2362, 2363, 2365}, diagnostic.Code()) {
			continue
		}
		token := astnav.GetTokenAtPosition(file, diagnostic.Pos())
		parameter := inferredDefaultAt(c, token)
		if parameter == nil || seen[parameter] || ast.GetSourceFileOfNode(parameter) != file {
			continue
		}
		var program *ast.Node
		a := analyzerFor(c)
		var visit func(*ast.Node) bool
		visit = func(n *ast.Node) bool {
			if isFunctionBoundary(n) {
				return false
			}
			if ast.IsCallExpression(n) && !a.passedAsProgram(n) {
				if runs, _ := runsAtTopLevel(c, a, n); runs {
					program = n
					return true
				}
			}
			return n.ForEachChild(visit)
		}
		visit(parameter.Initializer())
		if program == nil {
			continue
		}
		seen[parameter] = true
		d := newDiagnosticAt(optionsFor(c, file), file, scanner.GetErrorRangeForNode(file, program),
			EK1001.With("{0} runs a program, but a default parameter value is plain JavaScript, so the call does not run."),
			nodeText(file, program))
		note(d, "Make {0} optional and compute the default in the body: {0} ?? {1}.",
			nodeText(file, parameter.Name()), nodeText(file, program))
		added = append(added, d)
	}
	return append(all, added...)
}

func inferredDefaultAt(c *checker.Checker, token *ast.Node) *ast.Node {
	if token == nil {
		return nil
	}
	eligible := func(symbol *ast.Symbol) *ast.Node {
		if symbol == nil {
			return nil
		}
		declaration := symbol.ValueDeclaration()
		if declaration != nil && ast.IsParameterDeclaration(declaration) && declaration.Type() == nil && declaration.Initializer() != nil {
			return declaration
		}
		return nil
	}
	if parameter := eligible(c.GetSymbolAtLocation(token)); parameter != nil {
		return parameter
	}
	return nil
}
