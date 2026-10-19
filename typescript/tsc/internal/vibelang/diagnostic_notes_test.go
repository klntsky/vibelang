package vibelang

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// Diagnostic.String prints only the headline; advice lives in its chain.
func diagnosticWithNotes(d *ast.Diagnostic) string {
	text := d.String()
	for _, child := range d.MessageChain() {
		text += "\n" + diagnosticWithNotes(child)
	}
	return text
}
