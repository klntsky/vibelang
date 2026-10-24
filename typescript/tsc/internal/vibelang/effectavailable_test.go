package vibelang

import (
	"slices"
	"testing"

	"gotest.tools/v3/assert"
)

// TestMissingEffectReportsEffectFunctions compiles a program in which neither
// effect nor vibelang-runtime resolves. Each function that compiles to an
// Effect by what it does, a throw or a producer, is EK1011; before, each was
// declared plain while its JavaScript imported effect and returned an Effect.
// A plain function, with or without a plain annotation, keeps its plain
// behavior and is not reported.
func TestMissingEffectReportsEffectFunctions(t *testing.T) {
	t.Parallel()
	source := `export const make = (name: string) => {
  if (name === "") throw new TypeError("empty")
  return { name }
}
export function greet(name: string) {
  return "hello " + name
}
export const count = function* () {
  yield 1
}
export function shout(name: string): string {
  return name.toUpperCase()
}
`
	p := computedKeyProgram(t, map[string]string{
		"/Users/x/src/package.json":  `{"type":"module"}`,
		"/Users/x/src/tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ESNext","module":"NodeNext","declaration":true,"types":[]},"files":["main.vibe"]}`,
		"/Users/x/src/main.vibe":     source,
	})
	var reported []string
	for _, d := range p.GetSemanticDiagnostics(t.Context(), nil) {
		if d.Code() == EK1011.Code {
			reported = append(reported, source[d.Pos():d.End()])
		}
	}
	slices.Sort(reported)
	assert.DeepEqual(t, reported, []string{"count", "make"})
}
