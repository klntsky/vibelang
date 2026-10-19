package vibelang

import (
	"strings"
	"testing"
)

func TestBoundaryInlineCallbackDiagnostic(t *testing.T) {
	p, file, _, _ := editorProgram(t, `error Boom { cause: unknown }
export const inline = <A>(body: () => A): Effect.Effect<A, Boom, never> =>
 Effect.try({ try: () => body(), catch: (cause) => new Boom({ cause }) })`)
	found := false
	for _, d := range p.GetSemanticDiagnostics(t.Context(), file) {
		if d.Code() != EK4301.Code {
			continue
		}
		found = true
		for _, want := range []string{"body", "Effect.try", "named function type", "type Body<A> = () => A"} {
			if !strings.Contains(diagnosticWithNotes(d), want) {
				t.Errorf("missing %q in %s", want, diagnosticWithNotes(d))
			}
		}
	}
	if !found {
		t.Fatal("missing callback diagnostic")
	}
}
