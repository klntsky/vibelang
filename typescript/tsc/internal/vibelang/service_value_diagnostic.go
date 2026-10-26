package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/astnav"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

var EK4503 = message(4503, errorCategory, "{0} belongs to the service implementation; read it with {1}().{2}.")

func init() {
	registry[EK4503.Code] = EK4503
	checker.RegisterDiagnosticsFilterHook(explainServiceValue)
}

// Only replace a missing-member error. Static key members, and S.method()
// calls whose receiver hook already supplies the implementation, are valid.
func explainServiceValue(c *checker.Checker, file *ast.SourceFile, all []*ast.Diagnostic) []*ast.Diagnostic {
	if !isVibeLangFile(file) {
		return all
	}
	for i, d := range all {
		if d.Source() != "" || d.Code() != 2339 || diagnosticAlreadyExplained(c, file, d) {
			continue
		}
		name := astnav.GetTokenAtPosition(file, d.Pos())
		if name == nil || name.Parent == nil || !ast.IsPropertyAccessExpression(name.Parent) {
			continue
		}
		access := name.Parent
		if inHatch(access) {
			continue
		}
		_, shape := serviceOfKey(c, c.GetApparentType(c.GetTypeAtLocation(access.Expression())))
		if shape == nil || c.GetPropertyOfType(shape, name.Text()) == nil {
			continue
		}
		replacement := newDiagnosticAt(optionsFor(c, file), file, d.Loc(), EK4503,
			nodeText(file, access), nodeText(file, access.Expression()), name.Text())
		if isAtModuleTopLevel(access) {
			note(replacement, "Move the implementation lookup into an Effect function; a module's top level cannot run it.")
		}
		if len(c.GetSignaturesOfType(c.GetTypeOfPropertyOfType(shape, name.Text()), checker.SignatureKindCall)) != 0 {
			note(replacement, "To extract the implementation method, write {0}().{1}; to call it directly, write {0}.{1}(…).",
				nodeText(file, access.Expression()), name.Text())
		}
		all[i] = replacement
	}
	return all
}
