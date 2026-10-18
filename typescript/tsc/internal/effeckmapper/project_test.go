package effeckmapper

import (
	"errors"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
	"github.com/microsoft/TypeScript/tsc/internal/spanmap"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
	"gotest.tools/v3/assert"
)

// errNotFound is what a test file system returns for a missing file.
var errNotFound = errors.New("not found")

// testContext is the context of every golden: src/main.effeck in the package app.
var testContext = FileContext{FileName: "/app/src/main.effeck", ModuleKey: "app/src/main"}

// projectionCases are the lowering rules, one source and its virtual
// TypeScript each, without the prelude import line (TestImportLine).
var projectionCases = []struct {
	name    string
	source  string
	virtual string
}{
	{
		name:    "error with fields",
		source:  "error NotFound { id: string }\n",
		virtual: "class NotFound extends TaggedError(\"NotFound\")<{ readonly id: string }> {}\n",
	},
	{
		name:    "exported error without fields",
		source:  "export error Timeout\n",
		virtual: "export class Timeout extends TaggedError(\"Timeout\")<{}> {}\n",
	},
	{
		name:    "generic error",
		source:  "error Wrapped<T> { value: T }\n",
		virtual: "class Wrapped<T> extends TaggedError(\"Wrapped\")<{ readonly value: T }> {}\n",
	},
	{
		name:    "members are readonly unless mutable (D134)",
		source:  "interface Counter {\n  label: string\n  mutable count: number\n  [key: string]: unknown\n  tick(): void\n}\ntype P = { x: number; mutable y: number }\n",
		virtual: "interface Counter {\n  readonly label: string\n  count: number\n  readonly [key: string]: unknown\n  tick(): void\n}\ntype P = { readonly x: number; y: number }\n",
	},
	{
		name:    "a member named mutable stays",
		source:  "interface M { mutable: boolean }\n",
		virtual: "interface M { readonly mutable: boolean }\n",
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
		virtual: "function greet(id: string) { return $gen(function* () {\n  const user = (yield* $run(find(id)))\n  return user.name\n})}\n",
	},
	{
		name:    "nested and chained calls",
		source:  "function f(x: number) {\n  const y = f(g(x))\n  a(b)(c)\n  f().x\n}\n",
		virtual: "function f(x: number) { return $gen(function* () {\n  const y = (yield* $run(f((yield* $run(g(x))))))\n  ;(yield* $run((yield* $run(a(b)))(c)))\n  ;(yield* $run(f())).x\n})}\n",
	},
	{
		name:    "a run in an unbraced body takes no semicolon",
		source:  "function f(x: boolean) {\n  if (x) g(1)\n  else h(2)\n  while (x) g(3)\n  for (;;) h(4)\n}\n",
		virtual: "function f(x: boolean) { return $gen(function* () {\n  if (x) (yield* $run(g(1)))\n  else (yield* $run(h(2)))\n  while (x) (yield* $run(g(3)))\n  for (;;) (yield* $run(h(4)))\n})}\n",
	},
	{
		name:    "calls in a nested lambda stay",
		source:  "function f() {\n  return xs.map((x) => g(x))\n}\n",
		virtual: "function f() { return $gen(function* () {\n  return (yield* $run(xs.map((x) => g(x))))\n})}\n",
	},
	{
		name:    "a nested function declaration is its own Effect",
		source:  "function f() {\n  function inner() { return h(1) }\n  return inner\n}\n",
		virtual: "function f() { return $gen(function* () {\n  function inner() { return $gen(function* () { return (yield* $run(h(1))) })}\n  return inner\n})}\n",
	},
	{
		name:    "throw fails",
		source:  "function annotated(): User!NotFound! {\n  throw new NotFound({ id: f(1) })\n}\n",
		virtual: "function annotated(): Effect.Effect<User, NotFound, never> { return $gen(function* () {\n  return yield* $fail(new NotFound({ id: (yield* $run(f(1))) }))\n})}\n",
	},
	{
		name:    "a function that only throws keeps A void",
		source:  "function find(id: string) {\n  if (id === \"\") throw new Timeout()\n  throw new NotFound({ id })\n}\n",
		virtual: "function find(id: string) { return $gen(function* () {\n  if (id === \"\") return (yield* $fail(new Timeout())) as void\n  return (yield* $fail(new NotFound({ id }))) as void\n})}\n",
	},
	{
		name:    "an infer channel wraps the body in $contract (D57)",
		source:  "function find(id: string): string!infer E!Db {\n  return id\n}\n",
		virtual: "function find(id: string) { return $contract<string, $Infer, Db>()($gen(function* () {\n  return id\n}))}\n",
	},
	{
		name:    "omitted dependencies of a contract are inferred (D57)",
		source:  "function find(id: string): string!NotFound {\n  return id\n}\n",
		virtual: "function find(id: string) { return $contract<string, NotFound, $Infer>()($gen(function* () {\n  return id\n}))}\n",
	},
	{
		name:    "a plain return annotation keeps a plain function (L2)",
		source:  "function double(n: number): number {\n  return twice(n)\n}\n",
		virtual: "function double(n: number): number {\n  return twice(n)\n}\n",
	},
	{
		name:    "top-level calls and async functions stay; a generator function is a producer (D26)",
		source:  "const top = f()\nasync function a() { return f() }\nfunction* g() { yield f() }\n",
		virtual: "const top = f()\nasync function a() { return f() }\nfunction g() { return $produce(function* () { yield $emit((yield* $run($value(f)()))) })}\n",
	},
	{
		name:    "a service is an interface and a key (D45)",
		source:  "export service Users {\n  find(id: string): User!NotFound\n  region: string\n}\n",
		virtual: "export interface Users {\n  find(id: string): Effect.Effect<User, NotFound, never>\n  readonly region: string; readonly \"~effeck/service\"?: \"app/src/main#Users\" } export abstract class Users extends $service<Users>(\"app/src/main#Users\") {}\n",
	},
	{
		name:    "a control is a service whose base goes",
		source:  "control Approval { approve(request: string): boolean!! }\n",
		virtual: "interface Approval { approve(request: string): Effect.Effect<boolean, never, never>; readonly \"~effeck/service\"?: \"app/src/main#Approval\" } abstract class Approval extends $service<Approval>(\"app/src/main#Approval\") {}\n",
	},
	{
		name:    "a D1 abstract class keeps its static members in the key",
		source:  "abstract class Clock extends Control {\n  abstract now(): Effect<number>;\n  static readonly zero = 0\n}\n",
		virtual: "interface Clock {\n  now(): Effect<number>; readonly \"~effeck/service\"?: \"app/src/main#Clock\" } abstract class Clock extends $service<Clock>(\"app/src/main#Clock\") {\n  static readonly zero = 0\n}\n",
	},
	{
		name:    "a service without members is an abstract class",
		source:  "service Empty {}\n",
		virtual: "abstract class Empty {}\n",
	},
	{
		name:    "a service with a static default is a reference (D40)",
		source:  "abstract class Flags {\n  abstract readonly beta: boolean\n  static readonly default: Flags = { beta: true }\n}\n",
		virtual: "interface Flags {\n  readonly beta: boolean; readonly \"~effeck/service\"?: \"app/src/main#Flags\" } abstract class Flags extends $reference<Flags>(\"app/src/main#Flags\") {\n  static readonly default: Flags = { beta: true }\n}\n",
	},
	{
		name:    "methods of a class that implements a service are Effect functions (D92)",
		source:  "service Users { find(id: string): User!NotFound }\nclass Memory implements Users {\n  find(id: string) { return id }\n  static make() { return 1 }\n}\n",
		virtual: "interface Users { find(id: string): Effect.Effect<User, NotFound, never>; readonly \"~effeck/service\"?: \"app/src/main#Users\" } abstract class Users extends $service<Users>(\"app/src/main#Users\") {}\nclass Memory implements Users {\n  find(id: string) { return $gen(function* () { return id })}\n  static make() { return 1 }\n}\n",
	},
	{
		name:    "a method takes the form its body decides",
		source:  "class Cache {\n  get(k: string) { return load(k) }\n  refresh(k: string) {\n    const v = load(k)\n    return v\n  }\n}\n",
		virtual: "class Cache {\n  get(k: string) { return load(k) }\n  refresh(k: string) { return $gen(function* () {\n    const v = (yield* $run(load(k)))\n    return v\n  })}\n}\n",
	},
	{
		name:    "returns leave a try statement through its completion",
		source:  "function f(id: string) {\n  try {\n    return find(id)\n  } catch (e) {\n    return \"guest\"\n  }\n}\n",
		virtual: "function f(id: string) { return $gen(function* () {\n  const $c = yield* $catch($gen(function* () {\n    return $box((yield* $run(find(id))))\n  }), (e) => $gen(function* () {\n    return $box(\"guest\")\n  })); return $unboxed($c)\n})}\n",
	},
	{
		name:    "a region that no jump leaves dispatches nothing",
		source:  "function f() {\n  try {\n    g(1)\n  } catch {\n    g(2)\n  }\n}\n",
		virtual: "function f() { return $gen(function* () {\n  yield* $catch($gen(function* () {\n    ;(yield* $run(g(1)))\n  }), () => $gen(function* () {\n    ;(yield* $run(g(2)))\n  }))\n})}\n",
	},
	{
		name:    "a break that leaves a region is a jump the loop takes",
		source:  "function g(xs: number[]) {\n  for (const x of xs) {\n    try {\n      if (x) break\n      g(x)\n    } finally {\n      h(x)\n    }\n  }\n}\n",
		virtual: "function g(xs: number[]) { return $gen(function* () {\n  for (const x of (yield* $each(xs))) {\n    const $c = yield* $finally($gen(function* () {\n      if (x) return $jump(\"break:\")\n      ;(yield* $run(g(x)))\n    ; return $end }), $gen(function* () {\n      ;(yield* $run(h(x)))\n    ; return $end })); if ($jumped($c, \"break:\")) break\n  }\n})}\n",
	},
	{
		name:    "a jump that leaves two regions passes the completion on",
		source:  "function f() {\n  try {\n    try {\n      return 1\n    } finally {\n      g(1)\n    }\n  } catch {\n    return 2\n  }\n}\n",
		virtual: "function f() { return $gen(function* () {\n  const $c = yield* $catch($gen(function* () {\n    const $c_1 = yield* $finally($gen(function* () {\n      return $box(1)\n    }), $gen(function* () {\n      ;(yield* $run(g(1)))\n    ; return $end })); return $c_1\n  }), () => $gen(function* () {\n    return $box(2)\n  })); return $unboxed($c)\n})}\n",
	},
	{
		name:    "a provide block provides its layers to a region (D172, D173)",
		source:  "function main() {\n  provide AppLive, Clock = clock {\n    return greet(\"42\")\n  }\n}\n",
		virtual: "function main() { return $gen(function* () {\n  const $c = yield* Effect.provide([AppLive, $provideService(Clock, \"Clock\", clock)])($gen(function* () {\n    return $box((yield* $run(greet(\"42\"))))\n  })); return $unboxed($c)\n})}\n",
	},
	{
		name:    "a handle statement runs its block under handle (D132)",
		source:  "function k(version: string) {\n  try {\n    return release(version)\n  } handle Approval.approve(request) {\n    log(request)\n    resume(true)\n  }\n}\n",
		virtual: "function k(version: string) { return $gen(function* () {\n  const $c = yield* handle_1($gen(function* () {\n    return $box((yield* $run(release(version))))\n  }) , Approval, { approve: (request, $k) => $gen(function* () {\n    ;(yield* $run(log(request)))\n    return $tailResume($k, true)\n  }) }); return $unboxed($c)\n})}\n",
	},
	{
		name:    "a match expression in an Effect body runs as a program (D206)",
		source:  "function f(e: Failure) {\n  const label = match (e) {\n    when NotFound and { let id }: `no user ${id}`;\n    default: describe(e);\n  }\n  return label\n}\n",
		virtual: "function f(e: Failure) { return $gen(function* () {\n  const label = (yield* $run($gen(function* () { { if ($is(NotFound, e) && typeof e === \"object\" && e !== null && \"id\" in e) { let id = e.id; return `no user ${id}`; } } return (yield* $run(describe(e))); })))\n  return label\n})}\n",
	},
	{
		name:    "a returned match returns from its arms, exhaustively (D209)",
		source:  "function g(o: Option.Option<number>) {\n  return match (o) {\n    when Option.Some(let v): v;\n    when Option.None: 0;\n  }\n}\n",
		virtual: "function g(o: Option.Option<number>) { return $gen(function* () {\n  { { if (Option.isSome(o)) { let v = o.value; return v; } } { if (Option.isNone(o)) { return 0; } } return $noMatch(o); }\n})}\n",
	},
	{
		name:    "a match statement and an is test",
		source:  "function h(n: number) {\n  match (n) {\n    when 1: log(\"one\");\n    default: log(\"other\");\n  }\n  return n is 1 or 2\n}\n",
		virtual: "function h(n: number) { return $gen(function* () {\n  $match1: { { if (n === 1) { (yield* $run(log(\"one\"))); break $match1; } } (yield* $run(log(\"other\"))); break $match1; } \n  return (((n === 1) || (n === 2)))\n})}\n",
	},
	{
		name:    "using acquires its initializer by name in the block's Scope (D47, D14)",
		source:  "function report(path: string) {\n  using file = Files.open(path)\n  using t = timer(\"x\")\n  return file.name\n}\n",
		virtual: "function report(path: string) { return $gen(function* () {\n  const file = (yield* $acquire($use(Files).open(path)))\n  const t = (yield* $acquire(timer(\"x\")))\n  return file.name\n})}\n",
	},
	{
		name:    "interruption blocks and finally (exit) are regions (D178-D180)",
		source:  "function f() {\n  uninterruptible {\n    const conn = acquire()\n    try { restore { return use(conn) } } finally (exit) { release(conn, exit) }\n  }\n}\n",
		virtual: "function f() { return $gen(function* () {\n  const $c = yield* uninterruptible($gen(function* () {\n    const conn = (yield* $run($value(acquire)()))\n    const $c_1 = yield* $finallyExit($gen(function* () { const $c_2 = yield* restore($gen(function* () { return $box((yield* $run(use(conn)))) })); return $c_2 }), (exit) => $gen(function* () { ;(yield* $run(release(conn, exit))) ; return $end })); return $c_1\n  })); return $unboxed($c)\n})}\n",
	},
	{
		name:    "a tail region falls off its end for TypeScript's reachability",
		source:  "function describe(e: Failure) {\n  try {\n    return find(e)\n  } catch (error) {\n    switch (error._tag) {\n      case \"A\": return \"a\"\n      case \"B\": return \"b\"\n    }\n  }\n}\n",
		virtual: "function describe(e: Failure) { return $gen(function* () {\n  const $c = yield* $catch($gen(function* () {\n    return $box((yield* $run(find(e))))\n  }), (error) => $gen(function* () {\n    switch (error._tag) {\n      case \"A\": return $box(\"a\")\n      case \"B\": return $box(\"b\")\n    }\n  })); return $unboxed($c)\n})}\n",
	},
	{
		name:    "a producer is a Stream that emits its yields (D26)",
		source:  "function* lookups(keys: string[]) {\n  for (const key of keys) yield Store.get(key)\n  yield* more()\n}\nfunction total() {\n  let sum = 0\n  for (const n of lookups([\"a\"])) sum += n\n  return sum\n}\n",
		virtual: "function lookups(keys: string[]) { return $produce(function* () {\n  for (const key of (yield* $each(keys))) yield $emit((yield* $run($use(Store).get(key))))\n  yield* $delegate((yield* $run($value(more)())))\n})}\nfunction total() { return $gen(function* () {\n  let sum = 0\n  for (const n of (yield* $each((yield* $run(lookups([\"a\"])))))) sum += n\n  return sum\n})}\n",
	},
	{
		name:    "a layer without statements succeeds with its members",
		source:  "layer UsersLive: Users {\n  find(id) { return lookup(id) }\n  readonly region = \"eu\";\n}\n",
		virtual: "const UsersLive = Layer.succeed(Users, {\n  find(id) { return $gen(function* () { return (yield* $run(lookup(id))) })},\n  region: \"eu\"\n})\n",
	},
	{
		name:    "a layer with statements builds in an Effect",
		source:  "export layer UsersLive: Users {\n  const d = Directory()\n  find(id) { return d.lookup(id) }\n  readonly region = name()\n}\n",
		virtual: "export const UsersLive = Layer.effect(Users, $gen(function* () {\n  const d = (yield* $run($value(Directory)()))\n  return { find(id) { return $gen(function* () { return (yield* $run(d.lookup(id))) })},\n  region: (yield* $run($value(name)())) }\n}))\n",
	},
	{
		name:    "service member calls read the service (D41)",
		source:  "function f(id: string) {\n  return Users.find(id)\n}\n",
		virtual: "function f(id: string) { return $gen(function* () {\n  return (yield* $run($use(Users).find(id)))\n})}\n",
	},
	{
		name:    "effect modules and JavaScript globals are not keys",
		source:  "function f() {\n  Console.log(Math.max(1, 2))\n}\n",
		virtual: "function f() { return $gen(function* () {\n  ;(yield* $run(Console.log((yield* $run(Math.max(1, 2))))))\n})}\n",
	},
}

// virtualBody returns projection's text without its first line, the import line.
func virtualBody(t *testing.T, projection Projection) string {
	t.Helper()
	line, rest, ok := strings.Cut(projection.Text, "\n")
	assert.Assert(t, ok && strings.HasPrefix(line, "import {"), "first line: %q", line)
	return rest
}

// TestProjection checks each lowering rule's virtual text, that its span map
// passes the host's validation, and that the virtual text parses as
// TypeScript without syntax errors.
func TestProjection(t *testing.T) {
	t.Parallel()
	for _, c := range projectionCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			projection := Project(c.source, testContext)
			assert.Equal(t, virtualBody(t, projection), c.virtual)
			assert.Assert(t, len(projection.Diagnostics) == 0, "diagnostics: %v", projection.Diagnostics)
			assertValidMap(t, projection, c.source)
			assertParses(t, projection.Text)
		})
	}
}

// TestImportLine checks the first line: the prelude names the source uses
// and does not declare, the helpers under names no word of the file
// spells, and the sentinel that fails at line 1 when the prelude cannot
// resolve effect or effeck-runtime.
func TestImportLine(t *testing.T) {
	t.Parallel()
	source := "const Option = 1\nfunction f() {\n  const $run = 2\n  Console.log(Option)\n}\nconst L = Layer.empty\n"
	projection := Project(source, testContext)
	line, _, _ := strings.Cut(projection.Text, "\n")
	assert.Equal(t, line, `import { type $PreludeLoaded, $gen, $run as $run_1, Console, Layer } from "effeck-ts-mapper/prelude"; true satisfies $PreludeLoaded;`)
}

// TestSynthesizedNamesAvoidDeclarations checks that synthesized text names
// a prelude module through an alias when the file declares that name.
func TestSynthesizedNamesAvoidDeclarations(t *testing.T) {
	t.Parallel()
	source := "import { Layer } from \"./mine\"\ntype A = User!!\nfunction g(Effect: number) { return Effect }\nlayer L: S { readonly x = 1 }\n"
	projection := Project(source, testContext)
	line, rest, _ := strings.Cut(projection.Text, "\n")
	assert.Assert(t, strings.Contains(line, "Effect as Effect_1") && strings.Contains(line, "Layer as Layer_1"), line)
	assert.Assert(t, strings.Contains(rest, "type A = Effect_1.Effect<User, never, never>"), rest)
	assert.Assert(t, strings.Contains(rest, "const L = Layer_1.succeed(S, { x: 1 })"), rest)
	assertParses(t, projection.Text)
}

// TestImportedEffectNamespaceKeepsThePreludeType checks that Effect<...> as
// a type means the prelude's type in a file that imports effect's Effect
// module, which has no type (D4), as the fork resolves it.
func TestImportedEffectNamespaceKeepsThePreludeType(t *testing.T) {
	t.Parallel()
	source := "import { Effect } from \"effect\"\ntype A = Effect<string, never, never>\nconst x = Effect.void\nfunction f<Scope>(s: Scope) {}\n"
	projection := Project(source, testContext)
	line, rest, _ := strings.Cut(projection.Text, "\n")
	assert.Assert(t, strings.Contains(line, "Effect as Effect_1"), line)
	assert.Equal(t, rest, "import { Effect } from \"effect\"\ntype A = Effect_1<string, never, never>\nconst x = Effect.void\nfunction f<Scope>(s: Scope) { return $gen(function* () {})}\n")
}

// TestVerbatimSegmentsCoverSourceText checks that the identifiers a user
// hovers map back to their own text, so TypeScript's answers at virtual
// positions land on the original ones.
func TestVerbatimSegmentsCoverSourceText(t *testing.T) {
	t.Parallel()
	source := "error EmptyName { name: string }\n\nexport function greet(name: string): string!EmptyName!! {\n  const loud = shout(name)\n  return loud\n}\n"
	projection := Project(source, testContext)
	m := spanmap.New(projection.Segments)
	for _, word := range []string{"EmptyName", "greet", "loud", "shout", "string"} {
		virtual := strings.Index(virtualBody(t, projection), word) + strings.Index(projection.Text, "\n") + 1
		original, fidelity := m.VirtualToOriginalSpan(core.NewTextRange(virtual, virtual+len(word)))
		assert.Assert(t, fidelity.IsExact(), "%s maps with fidelity %v", word, fidelity)
		assert.Equal(t, source[original.Pos():original.End()], word)
	}
}

// TestUnprojectedForm checks that Effeck syntax without a lowering passes
// through unchanged and draws one EK9001 at the form.
func TestUnprojectedForm(t *testing.T) {
	t.Parallel()
	source := "function f() {\n  return await.all [g(1), g(2)]\n}\n"
	projection := Project(source, testContext)
	assert.Equal(t, len(projection.Diagnostics), 1)
	d := projection.Diagnostics[0]
	assert.Equal(t, d.Code, int32(CodeUnprojectedForm))
	assertValidMap(t, projection, source)
}

// TestFormChoice checks that a lambda whose form the rules cannot decide
// draws EK9100 at the lambda, with the resolver's message, and still lowers
// the default's guess.
func TestFormChoice(t *testing.T) {
	t.Parallel()
	source := "function f(xs: Items) {\n  xs.forEach((x) => {\n    log(x)\n    g(x)\n  })\n}\n"
	projection := Project(source, testContext)
	assert.Equal(t, len(projection.Diagnostics), 1)
	d := projection.Diagnostics[0]
	assert.Equal(t, d.Code, int32(CodeFormChoice))
	assert.Equal(t, source[d.Start:d.Start+d.Length], "(x) => {\n    log(x)\n    g(x)\n  }")
	assert.Assert(t, strings.HasPrefix(d.Message, "Cannot tell whether the lambda at argument 1 of xs.forEach is a plain function or an Effect function"), d.Message)
	assert.Assert(t, strings.Contains(projection.Text, "(x) => { return $gen(function* () {"), projection.Text)
}

// TestProjectionIsDeterministic checks that equal text projects to equal
// output, which TypeScript's content-hash cache relies on.
func TestProjectionIsDeterministic(t *testing.T) {
	t.Parallel()
	for _, c := range projectionCases {
		first, second := Project(c.source, testContext), Project(c.source, testContext)
		assert.DeepEqual(t, first, second)
	}
}

// TestExecutableRendering checks that the executable rendering drops
// checker-only witnesses and runs a call by a run-time test (L3,
// render_exec.go).
func TestExecutableRendering(t *testing.T) {
	t.Parallel()
	text, keep := executable{}.text(edit{kind: editWitness, text: ", g(x)"})
	assert.Assert(t, !keep && text == "")
	open, _ := executable{}.text(edit{kind: editRun, part: partOpen})
	close, _ := executable{}.text(edit{kind: editRun, part: partClose})
	tmp := helperRef("$t")
	assert.Equal(t, open+"c"+close, "(("+tmp+" = c), "+helperRef("$isRun")+"("+tmp+") ? (yield* "+tmp+") : "+tmp+")")
}

// TestModuleKey checks the fork's key rule: the nearest package.json's name,
// or its directory's, and the module path within it.
func TestModuleKey(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"/repo/package.json":            `{"name": "@acme/app"}`,
		"/repo/cases/ch01/package.json": `{"private": true}`,
	}
	read := func(name string) ([]byte, error) {
		if text, ok := files[name]; ok {
			return []byte(text), nil
		}
		return nil, errNotFound
	}
	assert.Equal(t, moduleKey("/repo/src/users.effeck", read), "@acme/app/src/users")
	assert.Equal(t, moduleKey("/repo/cases/ch01/main.effeck", read), "ch01/main")
	assert.Equal(t, moduleKey("/elsewhere/main.effeck", read), "main")
}

// TestVersionMismatch checks that a project whose installed effect or
// @effect packages differ from the forms table's pinned versions gets one
// message naming both versions of each, and that agreeing or absent
// packages get none.
func TestVersionMismatch(t *testing.T) {
	t.Parallel()
	header := Header{Inputs: []Input{{Package: "effect", Version: "4.0.5"}, {Package: "@effect/vitest", Version: "4.0.5"}, {Package: "vite", Version: "8.3.4"}}}
	files := map[string]string{
		"/repo/node_modules/effect/package.json":         `{"version": "4.1.0"}`,
		"/repo/node_modules/@effect/vitest/package.json": `{"version": "4.0.5"}`,
		"/repo/node_modules/vite/package.json":           `{"version": "9.0.0"}`,
		"/ok/node_modules/effect/package.json":           `{"version": "4.0.5"}`,
	}
	read := func(name string) ([]byte, error) {
		if text, ok := files[name]; ok {
			return []byte(text), nil
		}
		return nil, errNotFound
	}
	message := versionMismatch(header, "/repo/app", read)
	assert.Assert(t, strings.Contains(message, "effect 4.1.0 (the table pins 4.0.5)"), message)
	assert.Assert(t, !strings.Contains(message, "vitest") && !strings.Contains(message, "vite "), message)
	assert.Equal(t, versionMismatch(header, "/ok", read), "")
	assert.Equal(t, versionMismatch(header, "/none", read), "")
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
