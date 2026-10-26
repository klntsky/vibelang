package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/astnav"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// Subpath exports are deliberately outside the root prelude (D146).
var effectNameImports = map[string]string{
	"TestClock": "import { TestClock } from \"effect/testing\"",
}

func init() { checker.RegisterDiagnosticsFilterHook(explainMissingEffectName) }

func explainMissingEffectName(c *checker.Checker, file *ast.SourceFile, all []*ast.Diagnostic) []*ast.Diagnostic {
	if !isVibeLangFile(file) {
		return all
	}
	for i, d := range all {
		if d.Source() != "" || d.Code() != 2304 {
			continue
		}
		name := astnav.GetTokenAtPosition(file, d.Pos())
		if name == nil || !ast.IsIdentifier(name) {
			continue
		}
		if statement := effectNameImports[name.Text()]; statement != "" {
			all[i] = note(d.Clone(), "{0} is an Effect subpath export; add {1}.", name.Text(), statement)
		}
	}
	return all
}
