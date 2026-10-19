package vibelang

import "testing"

// A concise arrow's body may itself be a function boundary. Inspect that
// boundary before traversing its children, just as for a block body.
func TestReturnedArrowOwnsEffectStatements(t *testing.T) {
	for _, body := range []string{
		"provide Depth = 1 { return 1 }",
		"uninterruptible { return 1 }",
	} {
		t.Run(body, func(t *testing.T) {
			file := parseVibeLang(t, "const nested = () => () => { "+body+" }")
			outer := file.Statements.Nodes[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0].Initializer()
			if hasVibeLangStatement(outer) {
				t.Fatal("the returned arrow's statements belong to the inner function")
			}
			if !hasVibeLangStatement(outer.Body()) {
				t.Fatal("the inner function must retain its Effect statements")
			}
		})
	}
}
