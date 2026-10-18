package effeckmapper

import (
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
	"github.com/microsoft/TypeScript/tsc/internal/spanmap"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
	"gotest.tools/v3/assert"
)

// projectionCases are the skeleton's lowering rules, one source and its
// virtual TypeScript each, without the prelude import line.
var projectionCases = []struct {
	name    string
	source  string
	virtual string
}{
	{
		name:    "error with fields",
		source:  "error NotFound { id: string }\n",
		virtual: "class NotFound extends $TaggedError(\"NotFound\")<{ id: string }> {}\n",
	},
	{
		name:    "exported error without fields",
		source:  "export error Timeout\n",
		virtual: "export class Timeout extends $TaggedError(\"Timeout\")<{}> {}\n",
	},
	{
		name:    "generic error",
		source:  "error Wrapped<T> { value: T }\n",
		virtual: "class Wrapped<T> extends $TaggedError(\"Wrapped\")<{ value: T }> {}\n",
	},
	{
		name:    "contract with every slot",
		source:  "type A = User!NotFound!Db\n",
		virtual: "type A = Effect.Effect<User, NotFound, Db>\n",
	},
	{
		name:    "contract without dependencies",
		source:  "type B = User!NotFound\n",
		virtual: "type B = Effect.Effect<User, NotFound, never>\n",
	},
	{
		name:    "contract with empty slots",
		source:  "type C = User!!Db\ntype D = User!!\ntype E = User!NotFound!\n",
		virtual: "type C = Effect.Effect<User, never, Db>\ntype D = Effect.Effect<User, never, never>\ntype E = Effect.Effect<User, NotFound, never>\n",
	},
	{
		name:    "calls run in a function body",
		source:  "function greet(id: string) {\n  const user = find(id)\n  return user.name\n}\n",
		virtual: "function greet(id: string) { return $gen(function* () {\n  const user = (yield* $(find(id)))\n  return user.name\n})}\n",
	},
	{
		name:    "nested and chained calls",
		source:  "function f(x: number) {\n  const y = f(g(x))\n  a(b)(c)\n  f().x\n}\n",
		virtual: "function f(x: number) { return $gen(function* () {\n  const y = (yield* $(f((yield* $(g(x))))))\n  ;(yield* $((yield* $(a(b)))(c)))\n  ;(yield* $(f())).x\n})}\n",
	},
	{
		name:    "calls in a nested lambda stay",
		source:  "function f() {\n  return xs.map((x) => g(x))\n}\n",
		virtual: "function f() { return $gen(function* () {\n  return (yield* $(xs.map((x) => g(x))))\n})}\n",
	},
	{
		name:    "a nested function declaration is its own Effect",
		source:  "function f() {\n  function inner() { return h() }\n  return inner\n}\n",
		virtual: "function f() { return $gen(function* () {\n  function inner() { return $gen(function* () { return (yield* $(h())) })}\n  return inner\n})}\n",
	},
	{
		name:    "throw fails",
		source:  "function annotated(): User!NotFound {\n  throw new NotFound({ id: f() })\n}\n",
		virtual: "function annotated(): Effect.Effect<User, NotFound, never> { return $gen(function* () {\n  return yield* $fail(new NotFound({ id: (yield* $(f())) }))\n})}\n",
	},
	{
		name:    "top-level calls, async and generator functions stay",
		source:  "const top = f()\nasync function a() { return f() }\nfunction* g() { yield f() }\n",
		virtual: "const top = f()\nasync function a() { return f() }\nfunction* g() { yield f() }\n",
	},
}

// TestProjection checks each lowering rule's virtual text, that its span map
// passes the host's validation, and that the virtual text parses as
// TypeScript without syntax errors.
func TestProjection(t *testing.T) {
	t.Parallel()
	for _, c := range projectionCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			projection := Project(c.source)
			assert.Equal(t, projection.Text, effectPreludeImport+c.virtual)
			assert.Assert(t, len(projection.Diagnostics) == 0, "diagnostics: %v", projection.Diagnostics)
			assertValidMap(t, projection, c.source)
			assertParses(t, projection.Text)
		})
	}
}

// TestVerbatimSegmentsCoverSourceText checks that the identifiers a user
// hovers map back to their own text, so TypeScript's answers at virtual
// positions land on the original ones.
func TestVerbatimSegmentsCoverSourceText(t *testing.T) {
	t.Parallel()
	source := "error EmptyName { name: string }\n\nexport function greet(name: string): string!EmptyName {\n  const loud = shout(name)\n  return loud\n}\n"
	projection := Project(source)
	m := spanmap.New(projection.Segments)
	for _, word := range []string{"EmptyName", "greet", "loud", "shout", "string"} {
		virtual := strings.Index(projection.Text, word)
		original, fidelity := m.VirtualToOriginalSpan(core.NewTextRange(virtual, virtual+len(word)))
		assert.Assert(t, fidelity.IsExact(), "%s maps with fidelity %v", word, fidelity)
		assert.Equal(t, source[original.Pos():original.End()], word)
	}
}

// TestFileThatBindsEffect checks that a file with its own Effect does not
// import the prelude's, and that its contracts name Effect's type directly.
func TestFileThatBindsEffect(t *testing.T) {
	t.Parallel()
	source := "import { Effect } from \"effect\"\ntype A = User!!\n"
	projection := Project(source)
	assert.Equal(t, projection.Text, preludeImport+"import { Effect } from \"effect\"\ntype A = import(\"effect\").Effect.Effect<User, never, never>\n")
	assertValidMap(t, projection, source)
}

// TestUnprojectedForm checks that Effeck syntax without a lowering passes
// through unchanged and draws one EK9001 at the form.
func TestUnprojectedForm(t *testing.T) {
	t.Parallel()
	source := "service Users { find(id: string): User!NotFound }\n"
	projection := Project(source)
	assert.Equal(t, projection.Text, effectPreludeImport+"service Users { find(id: string): Effect.Effect<User, NotFound, never> }\n")
	assert.Equal(t, len(projection.Diagnostics), 1)
	d := projection.Diagnostics[0]
	assert.Equal(t, d.Code, int32(CodeUnprojectedForm))
	assert.Equal(t, source[d.Start:d.Start+d.Length], "service")
	assertValidMap(t, projection, source)
}

// TestProjectionIsDeterministic checks that equal text projects to equal
// output, which TypeScript's content-hash cache relies on.
func TestProjectionIsDeterministic(t *testing.T) {
	t.Parallel()
	for _, c := range projectionCases {
		first, second := Project(c.source), Project(c.source)
		assert.DeepEqual(t, first, second)
	}
}

// assertValidMap fails unless projection's span map passes the validation
// TypeScript applies to every transform result.
func assertValidMap(t *testing.T, projection Projection, source string) {
	t.Helper()
	if problem := spanmap.New(projection.Segments).Validate(projection.Text, source); problem != nil {
		t.Fatalf("invalid span map: %v", problem)
	}
}

// assertParses fails if text, read as TypeScript, has a syntax error.
func assertParses(t *testing.T, text string) {
	t.Helper()
	file := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: tspath.RootedFilePathFromAbsolute("/virtual.ts")}, text, core.ScriptKindTS)
	for _, d := range file.Diagnostics() {
		t.Errorf("syntax error at %d: %s\n%s", d.Pos(), d.String(), text)
	}
}
