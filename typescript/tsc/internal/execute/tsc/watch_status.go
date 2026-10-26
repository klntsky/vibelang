package tsc

import (
	"fmt"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
)

// WatchStatus counts only errors as errors, as the normal compilation does.
func WatchStatus(items []*ast.Diagnostic) *ast.Diagnostic {
	errors, warnings := 0, 0
	for _, item := range items {
		switch item.Category() {
		case diagnostics.CategoryError:
			errors++
		case diagnostics.CategoryWarning:
			warnings++
		}
	}
	if warnings == 0 {
		if errors == 1 {
			return ast.NewCompilerDiagnostic(diagnostics.Found_1_error_Watching_for_file_changes)
		}
		return ast.NewCompilerDiagnostic(diagnostics.Found_0_errors_Watching_for_file_changes, errors)
	}
	plural := func(n int, word string) string {
		if n != 1 {
			word += "s"
		}
		return fmt.Sprintf("%d %s", n, word)
	}
	text := fmt.Sprintf("Found %s and %s. Watching for file changes.", plural(errors, "error"), plural(warnings, "warning"))
	return ast.NewDiagnosticFromText(nil, core.TextRange{}, 6194, diagnostics.CategoryMessage, text, nil, nil, false, false)
}
