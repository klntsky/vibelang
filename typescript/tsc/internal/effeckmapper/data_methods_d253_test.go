package effeckmapper

import (
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// TestD253RejectsAmbiguousMethods prevents runtime TypeIds from choosing
// between coexisting modules; explicit module and native annotations resolve it.
func TestD253RejectsAmbiguousMethods(t *testing.T) {
	for _, name := range []string{"asVoid", "isFailure", "isSuccess", "map", "mapBoth", "mapError", "match"} {
		source := `import { value } from "./opaque.js"; function f() { return value.` + name + `() }`
		p := Project(source, testContext)
		assert.Equal(t, len(p.Diagnostics), 1, "%s", p.Text)
		assert.Assert(t, strings.Contains(p.Diagnostics[0].Message, "D253") && strings.Contains(p.Diagnostics[0].Message, "annotation"), "%v", p.Diagnostics)
		executable := Executable(source, testContext)
		assert.Assert(t, !strings.Contains(executable.Text, `$method(`), "%s", executable.Text)
	}
	for _, source := range []string{
		`interface Native { map(f: (n: number) => number): number[]; size: number }; function f(x: Native) { const m = x.map; return x.map((n) => n + x.size) }`,
		`function f() { return { first: "a", last: "b" } |> %.first + %.last }`,
		`function f(s: string) { return s.match(/x/) }`,
		`function f() { return "text".match(/x/) }`,
		`function f() { return [1].map((n) => n + 1) }`,
		`import { value } from "./opaque.js"; function f() { return (value as Option.Option<number>).map((n) => n + 1) }`,
		`function f(xs: readonly number[]) { return xs.map((n) => n + 1) }`,
		`function f(x: Effect.Effect<number>) { return x.map((n) => n + 1) }`,
		`function f(x: Exit.Exit<number>) { return x.map((n) => n + 1) }`,
		`import { value } from "./opaque.js"; function f() { return Effect.map(value, (n: number) => n + 1) }`,
	} {
		p := Project(source, testContext)
		assert.Equal(t, len(p.Diagnostics), 0, "%v", p.Diagnostics)
	}
}

// TestD253RejectsUnprovenAccess retains the fork's optional and extracted
// method errors even though the checking augmentation exposes these members.
func TestD253RejectsUnprovenAccess(t *testing.T) {
	for _, source := range []string{
		`import { maybe } from "./opaque.js"; const x = maybe?.isSome()`,
		`import { local } from "./opaque.js"; const x = local(1)?.map((n: number) => n)`,
		`import { maybe } from "./opaque.js"; export const m = maybe.map`,
		`import { maybe } from "./opaque.js"; export const m = maybe.isSome`,
		`import { maybe } from "./opaque.js"; export const m = maybe.isSome?.()`,
	} {
		p := Project(source, testContext)
		assert.Equal(t, len(p.Diagnostics), 1, "%s", p.Text)
		assert.Assert(t, strings.Contains(p.Diagnostics[0].Message, "optional") || strings.Contains(p.Diagnostics[0].Message, "uncalled"), "%v", p.Diagnostics)
	}
	p := Project(`function f(xs: readonly number[] | undefined) { return xs?.map }`, testContext)
	assert.Equal(t, len(p.Diagnostics), 0, "%v", p.Diagnostics)
}
