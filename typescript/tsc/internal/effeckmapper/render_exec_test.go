package effeckmapper

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/sourcemap"
	"gotest.tools/v3/assert"
)

// TestRenderExec checks each kind's default executable text, with helper
// references unresolved: the kinds whose text is the same in both
// renderings, and those whose run-time form differs.
func TestRenderExec(t *testing.T) {
	t.Parallel()
	tmp, run, s := helperRef("$t"), helperRef("$isRun"), helperRef("$s")
	render := func(kind editKind, arg string, inner string) string {
		var out strings.Builder
		for _, p := range []part{partOpen, partClose} {
			text, keep := renderExec(edit{kind: kind, part: p, arg: arg})
			assert.Assert(t, keep)
			out.WriteString(text)
			if p == partOpen {
				out.WriteString(inner)
			}
		}
		return out.String()
	}
	assert.Equal(t, render(editRun, "", "f(x)"), "(("+tmp+" = f(x)), "+run+"("+tmp+") ? (yield* "+tmp+") : "+tmp+")")
	assert.Equal(t, render(editServiceCall, "", "Users"), "(("+s+" = Users), "+helperRef("$isKey")+"("+s+") ? (yield* "+s+") : "+s+")")
	assert.Equal(t, render(editValueCall, "", "Config"), helperRef("$value")+"(Config)")
	assert.Equal(t, render(editFail, "void", "e"), "return (yield* "+helperRef("$fail")+"(e))")
	assert.Equal(t, render(editRecord, "return value", "x"), "return x")
	assert.Equal(t, render(editUsing, "", "lease()"), "(yield* "+helperRef("$acquire")+"("+helperRef("$scope")+", lease()))")
	assert.Equal(t, render(editEffectBody, "self", " body "), " return "+helperRef("$gen")+"({ self: this }, function* () { body })")
	text, keep := renderExec(edit{kind: editSyntax, text: "abstract class"})
	assert.Assert(t, keep && text == "abstract class")
	_, keep = renderExec(edit{kind: editWitness, text: ", g(x)"})
	assert.Assert(t, !keep)
}

// emitCases are the executable rendering's rules: a source, text its
// JavaScript must contain, and text it must not.
var emitCases = []struct {
	name     string
	source   string
	contains []string
	absent   []string
}{

	{
		name:     "resume expressions hide normal and jump sentinels while retaining pending completion",
		source:   "control Ask { answer(): number!! } function f() { try { Ask.answer() } handle Ask.answer() { const value = resume(1); record(value) } }",
		contains: []string{"$returned($pending) ? $pending : undefined", "return $pending"},
	},

	{
		name:     "data methods and predicates call the module without prototypes",
		source:   "function f(o: Option.Option<number>) { if (o.isSome()) return o.getOrElse(() => 0); return 0 }",
		contains: []string{"Option.isSome(o)", "Option.getOrElse(o, () => 0)"},
		absent:   []string{"o.isSome()", "o.getOrElse("},
	},
	{
		name:     "acyclic entries forward defaults rest and destructuring once",
		source:   "function f(a = seed(), { x } = object(), [y] = array(), ...rest: number[]) { return unknown(a, x, y, rest) }",
		contains: []string{"function f$fast(a, $arg1, $arg2, ...rest)", "f$body(a, $arg1, $arg2, ...rest)", "function* f$body(a = seed(), { x } = object(), [y] = array(), ...rest)"},
		absent:   []string{"const f$fast", "$fast(f$body)"},
	},
	{
		name:     "a body that reads arguments keeps the counted entry, which forwards every argument",
		source:   "function f(a: number) { return unknown(arguments.length, () => arguments[1]) }",
		contains: []string{"const f$fast = $fast(f$body);"},
		absent:   []string{"function f$fast("},
	},
	{
		name:     "arguments of a nested function don't count",
		source:   "function f(a: number) { return unknown(function () { return arguments.length }) }",
		contains: []string{"function f$fast(a)"},
		absent:   []string{"const f$fast"},
	},
	{
		name:     "direct recursion counts eager depth",
		source:   "function f(n: number) { unknown(); return n ? f(n - 1) : 0 }",
		contains: []string{"const f$fast = $fast(f$body);", "f$fast(n - 1)"},
	},
	{
		name:     "mutual recursion counts only cycle members",
		source:   "function f(n: number) { return n ? g(n - 1) : unknown() } function g(n: number) { return f(n) } function outer(n: number) { return f(n) }",
		contains: []string{"const f$fast = $fast(f$body);", "const g$fast = $fast(g$body);", "function outer$fast(n)"},
		absent:   []string{"const outer$fast"},
	},
	{
		name:     "recursion through a lambda starts on the fiber",
		source:   "function f(n: number) { return unknown(() => f(n - 1)) }",
		contains: []string{"function f$fast(n)", "f$fast(n - 1)"},
		absent:   []string{"const f$fast"},
	},
	{
		name:     "export specifiers do not shadow a pure declaration",
		source:   "function f(n: number) { return n * 3 } export { f }; function g() { return f(2) }",
		contains: []string{"function f$direct(n)", "f[$direct] = f$direct;", "return f$direct(2);", "export { f };"},
		absent:   []string{"f$fast"},
	},
	{
		name:     "fiber forms in do bodies retain executable run boundaries",
		source:   "function f(a: Fiber.Fiber<number>) { return do { join a } } function g() { return do { for concurrent (const x of [1]) { log(x) } 1 } }",
		contains: []string{"$join(a)", "$forConcurrent([1], undefined, (x) => $gen(function* ()", "$isRun($t) ? (yield* $t) : $t"},
		absent:   []string{"for concurrent", "join a", "f$direct", "g$direct"},
	},
	{
		name:     "join runs a held fiber and keeps array and object shapes",
		source:   "function f(a: Fiber.Fiber<number>, b: Fiber.Fiber<string>) { const x = join a; const y = join [a, b]; return join { a, b } }",
		contains: []string{"$join(a)", "$join([a, b])", "$join({ a, b })", "$isRun($t) ? (yield* $t) : $t"},
		absent:   []string{"join a", "f$direct", "$asProgram(a)"},
	},
	{
		name:     "concurrent loops defer bodies and continue returns from one item",
		source:   "function f(xs: number[]) { for concurrent(2) (const x of xs) { if (!x) continue; log(x) } for concurrent (const x of xs) log(x) }",
		contains: []string{"$forConcurrent(xs, 2, (x) => $gen(function* ()", "$forConcurrent(xs, undefined, (x) => $gen(function* ()", "return;", "log(x)"},
		absent:   []string{"for concurrent", "continue", "f$direct"},
	},
	{
		name:     "all receives programs instead of completed strings",
		source:   "function f() { return all([Effect.succeed(1), Effect.succeed(2)]) }",
		contains: []string{"all([$gen(function* ()", "Effect.succeed(1)", "Effect.succeed(2)"},
		absent:   []string{"f$direct"},
	},
	{
		name:   "catch expressions run operands inside lazy generators",
		source: "function f() { return a() catch e => recover(e) }",
		contains: []string{
			`import { $catch, $done, $isRun, $settle } from "effeck-runtime";`,
			"$catch($gen(function* () { return (($t = $value(a)()), $isRun($t) ? (yield* $t) : $t); }), e => $gen(function* () { return (($t = recover(e)), $isRun($t) ? (yield* $t) : $t); }))",
		},
		absent: []string{"$catchExpression", "f$direct"},
	},
	{
		name:   "a catch fallback in a pipe keeps the evaluated topic",
		source: "declare function input(): string!!; declare function find(id: string): string!!; declare function backup(id: string): string!!; function f() { return input() |> find(%) catch backup(%) }",
		contains: []string{
			"const $topic = (($t = input()), $isRun($t) ? (yield* $t) : $t);",
			"$catch($gen(function* () { return (($t = find($topic)), $isRun($t) ? (yield* $t) : $t); }), () => $gen(function* () { return (($t = backup($topic)), $isRun($t) ? (yield* $t) : $t); }))",
		},
		absent: []string{"|>", "(%)", "f$direct"},
	},
	{
		name:   "await operations defer calls and do blocks while preserving a held Fiber",
		source: "function f(held: Fiber.Fiber<string>) {\n  return await.all [Effect.succeed(1), do { 2 }, held]\n}\n",
		contains: []string{
			`import { $awaitAll, $done, $isRun, $settle } from "effeck-runtime";`,
			"$awaitAll([$gen(function* () { return (($t = Effect.succeed(1)), $isRun($t) ? (yield* $t) : $t); })",
			"return 2;",
			"$asProgram(held)",
		},
		absent: []string{"await.all", "f$direct"},
	},
	{
		name:   "a pure function splits into f$direct and its Effect wrapper (D10)",
		source: "export function double(n: number) {\n  return n * 2\n}\n",
		contains: []string{
			"export function double(...$args) { return $sync(() => double$direct(...$args)); }",
			"double[$direct] = double$direct;",
			"function double$direct(n) {\n    return n * 2;\n}",
			`import { $direct } from "effeck-runtime";`,
			`import { sync as $sync } from "effect/Effect";`,
		},
		absent: []string{"export function double$direct", "$isRun"},
	},
	{
		name:   "a same-file pure call is direct, and pure groups recurse on the JavaScript stack",
		source: "function even(n: number) {\n  return n === 0 ? true : odd(n - 1)\n}\nfunction odd(n: number) {\n  return n === 0 ? false : even(n - 1)\n}\nfunction f(n: number) {\n  return even(n) && Math.max(n, 2) > 1\n}\n",
		contains: []string{
			"return n === 0 ? true : odd$direct(n - 1);",
			"return n === 0 ? false : even$direct(n - 1);",
			"return even$direct(n) && Math.max(n, 2) > 1;",
		},
		absent: []string{"$isRun", "$fast"},
	},
	{
		name:   "a function that runs an Effect gets a generator, a wrapper, and a fast entry",
		source: "function greet(id: string) {\n  const user = find(id)\n  return user.name\n}\nfunction twice(id: string) {\n  return greet(id) + greet(id)\n}\n",
		contains: []string{
			"function greet(...$args) { return (greet$effect ??= $fnUntraced(greet$body)).apply(this, $args); }",
			"function greet$fast(id) { const it = greet$body(id), s = it.next(); return s.done ? $done(s.value) : $settle(it, s); }",
			"function* greet$body(id) {",
			"const user = (($t = find(id)), $isRun($t) ? (yield* $t) : $t);",
			"return (($t = greet$fast(id)), $isRun($t) ? (yield* $t) : $t) + (($t = greet$fast(id)), $isRun($t) ? (yield* $t) : $t);",
			"var $t;",
		},
		absent: []string{"greet$direct"},
	},
	{
		name:     "a function in value position is the Effect wrapper (L2)",
		source:   "function double(n: number) {\n  return n * 2\n}\nexport function main() {\n  return [1, 2].map(double)\n}\n",
		contains: []string{"[1, 2].map(double)"},
		absent:   []string{"map(double$direct)"},
	},
	{
		name:   "a call of an imported function reads its $direct property",
		source: "import { shout } from \"./b.effeck\"\nfunction f(x: string) {\n  return shout(x)\n}\n",
		contains: []string{
			`import { shout } from "./b.js";`,
			"return (($t = (shout[$direct] ?? shout)(x)), shout[$direct] ? $t : ($isRun($t) ? (yield* $t) : $t));",
		},
	},
	{
		name:   "a plain return annotation keeps a plain function, whose calls do not run (L2)",
		source: "function len(s: string): number {\n  return s.trim().length\n}\nfunction f(s: string) {\n  return len(s)\n}\n",
		contains: []string{
			"function len(s) {\n    return s.trim().length;\n}",
			"return (($t = len(s)), $isRun($t) ? (yield* $t) : $t);",
		},
		absent: []string{"len$direct", "len$fast"},
	},
	{
		name:   "a service member call reads the service unless the key owns the member (D41, L8)",
		source: "service Users {\n  find(id: string): string!!\n}\nfunction f(id: string) {\n  return Users.find(id) + Users.pipe()\n}\n",
		contains: []string{
			`class Users extends $service("app/src/main#Users") {`,
			"(($s = Users), $isKey($s) ? (yield* $s) : $s).find(id)",
			"(($t = Users.pipe()), $isRun($t) ? (yield* $t) : $t)",
		},
	},
	{
		name:     "a member call on another key reads the service only when the key lacks the member",
		source:   "import { Sessions } from \"./sessions.effeck\"\nfunction f() {\n  return Sessions.get(\"t\")\n}\n",
		contains: []string{`(($s = Sessions), $isKey($s) && !("get" in $s) ? (yield* $s) : $s).get("t")`},
	},
	{
		name:     "a control registers its operations and their parameter counts",
		source:   "control Approval {\n  approve(request: string, level?: number): boolean!!\n}\n",
		contains: []string{`$control(Approval, ["approve"], [2]);`},
	},
	{
		name:     "a service with a static default passes it to $reference (D40)",
		source:   "abstract class Flags {\n  abstract readonly on: boolean\n  static readonly default: Flags = { on: false }\n}\n",
		contains: []string{`class Flags extends $reference("app/src/main#Flags", () => Flags.default) {`},
	},
	{
		name:   "regions keep the checking rendering's completions, defined at run time",
		source: "function f(xs: number[]) {\n  for (const x of xs) {\n    try {\n      if (x === 2) break\n      g(x)\n    } finally {\n      h()\n    }\n  }\n  return 1\n}\n",
		contains: []string{
			"const $end = {};",
			"class $Jump {\n    constructor(k) { this.k = k; }\n}",
			`const $c = yield* $finally($gen(function* () {`,
			`return $jump("break:");`,
			`if ($jumped($c, "break:"))`,
		},
	},
	{
		name:     "a value call runs a key or an Effect value, and calls a function (D90)",
		source:   "import { Users } from \"./users.effeck\"\nfunction f() {\n  return Users()\n}\n",
		contains: []string{"const $value = (v) => typeof v === \"function\" && !$isKey(v) ? v : () => $gen(function* () { return yield* v; });", "$value(Users)()"},
	},
	{
		name:     "throw fails the Effect (D46)",
		source:   "function f(n: number) {\n  if (n < 0) throw new Error(\"negative\")\n  return n\n}\n",
		contains: []string{`return yield* $fail(new Error("negative"));`},
		absent:   []string{"f$direct"},
	},
	{
		name:     "a function that holds a using gets a Scope of its own, and no fast entry",
		source:   "function f() {\n  using r = lease(1)\n  return r.id\n}\nfunction g() {\n  return f()\n}\n",
		contains: []string{"return $region(($scope) => $gen(function* () {", "const r = (yield* $acquire($scope, lease(1)));", "(($t = f()), $isRun($t) ? (yield* $t) : $t)"},
		absent:   []string{"f$fast", "f$body"},
	},
	{
		name:   "a producer runs under the runtime's Yield operation, and for...of takes a Stream's elements (D26)",
		source: "function* numbers(n: number) {\n  for (let i = 0; i < n; i++) yield i\n  yield* more(1)\n  yield\n}\nfunction sum() {\n  let s = 0\n  for (const x of numbers(3)) s += x\n  return s\n}\n",
		contains: []string{
			"return $produce(() => $producerGen(function* () {",
			"yield* $yield(i);",
			"yield* $yieldAll((($t = more(1)), $isRun($t) ? (yield* $t) : $t));",
			"yield* $yield(undefined);",
			"yield* $iterate((($t = numbers(3)), $isRun($t) ? (yield* $t) : $t),",
			"const x = $item;",
		},
	},
	{
		name:     "an entry module runs main when the host starts it (D135)",
		source:   "export function main() {\n  return 1\n}\n",
		contains: []string{"if (import.meta.main)\n    $runMain(main());"},
	},
	{
		name:     "a module that calls a runner itself gets no entry statement",
		source:   "export function main() {\n  return 1\n}\nrunPromise(main())\n",
		absent:   []string{"import.meta.main"},
		contains: []string{"runPromise(main())"},
	},
	{
		name:     "an Effect contract with an infer channel erases to the generator (D57)",
		source:   "class Gone extends TaggedError(\"Gone\")<{}> {}\nexport function f(n: number): number!infer E {\n  if (n < 0) throw new Gone()\n  return n\n}\n",
		contains: []string{"function* f$body(n) {", "return yield* $fail(new Gone());"},
		absent:   []string{"$Contract", "$Infer"},
	},
}

// TestEmit checks the executable rendering's rules on small sources.
func TestEmit(t *testing.T) {
	t.Parallel()
	for _, c := range emitCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			js, _, diagnostics := Emit(c.source, testContext)
			assert.Equal(t, len(diagnostics), 0, "diagnostics: %v\n%s", diagnostics, js)
			for _, want := range c.contains {
				assert.Assert(t, strings.Contains(js, want), "missing %q in\n%s", want, js)
			}
			for _, unwanted := range c.absent {
				assert.Assert(t, !strings.Contains(js, unwanted), "unexpected %q in\n%s", unwanted, js)
			}
		})
	}
}

// TestEmitCorpusDeclarations checks the controls and static-default service
// in the corpus, both before and after TypeScript erases their types.
func TestEmitCorpusDeclarations(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name          string
		registrations []string
		reference     string
	}{
		{
			name: "handle-tail-resume",
			registrations: []string{
				`$control(Approval, ["approve"], [1]);`,
				`$control(Notes, ["note"], [1]);`,
			},
		},
		{
			name:      "effeck-services-default",
			reference: "FeatureFlags",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(filepath.Join("../../../effeck/cases", c.name, "main.effeck"))
			assert.NilError(t, err)
			source := string(data)
			executable := Executable(source, testContext)
			js, _, diagnostics := Emit(source, testContext)
			assert.Equal(t, len(executable.Diagnostics), 0, "diagnostics: %v", executable.Diagnostics)
			assert.Equal(t, len(diagnostics), 0, "diagnostics: %v", diagnostics)
			for _, text := range []string{executable.Text, js} {
				assert.Equal(t, strings.Count(text, "$control("), len(c.registrations), "%s", text)
				for _, registration := range c.registrations {
					assert.Equal(t, strings.Count(text, registration), 1, "%s", text)
				}
			}
			if name := c.reference; name != "" {
				args := `("app/src/main#` + name + `", () => ` + name + `.default)`
				assert.Assert(t, strings.Contains(executable.Text, "$reference<"+name+">"+args), "%s", executable.Text)
				assert.Assert(t, strings.Contains(js, "$reference"+args), "%s", js)
			}
		})
	}
}

// TestEmitScopeOwnership keeps a using's private Scope out of ordinary
// acquisitions, and keeps a lowered block valid as an if branch (D14).
func TestEmitScopeOwnership(t *testing.T) {
	t.Parallel()
	source := `function f() {
   const kept = acquire("kept")
   using log = acquire("log")
   if (kept) { using inner = acquire("inner") }
   return kept
 }`
	js, _, diagnostics := Emit(source, testContext)
	assert.Equal(t, len(diagnostics), 0, "%v\n%s", diagnostics, js)
	assert.Assert(t, strings.Contains(js, "$region(($scope) => $gen(function* () {"), js)
	assert.Assert(t, strings.Contains(js, "$acquire($scope, acquire(\"log\"))"), js)
	assert.Equal(t, strings.Count(js, "$region("), 2, js)
	assert.Assert(t, !strings.Contains(js, "$scopedBody"), js)
}

// TestEmitCorpusSyntax catches a lowered scope losing its statement braces
// or separator before else, both of which single-file transpilation recovers.
func TestEmitCorpusSyntax(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"scopes-using", "decl-handle"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("../../../effeck/cases", name, "main.effeck"))
			assert.NilError(t, err)
			js, _, diagnostics := Emit(string(data), testContext)
			for _, d := range diagnostics {
				assert.Assert(t, d.Code != CodeExecSyntax, "%v", diagnostics)
			}
			if node, err := exec.LookPath("node"); err == nil {
				cmd := exec.Command(node, "--check", "--input-type=module")
				cmd.Stdin = strings.NewReader(js)
				output, err := cmd.CombinedOutput()
				assert.NilError(t, err, "%s", output)
			}
		})
	}
}

// TestEmitIsDeterministic checks that equal text emits equal output.
func TestEmitIsDeterministic(t *testing.T) {
	t.Parallel()
	for _, c := range emitCases {
		js1, map1, _ := Emit(c.source, testContext)
		js2, map2, _ := Emit(c.source, testContext)
		assert.Equal(t, js1, js2)
		assert.Equal(t, map1, map2)
	}
}

// TestEmitSourceMap checks that the source map takes a verbatim name in
// the JavaScript back to its original line and column, through the
// transpiler's map and the plan's segments, and names the original file.
func TestEmitSourceMap(t *testing.T) {
	t.Parallel()
	source := "// one\nfunction greet(id: string) {\n  const user = find(id)\n  return user.name\n}\n"
	js, sourceMap, _ := Emit(source, testContext)
	var raw sourcemap.RawSourceMap
	assert.NilError(t, json.Unmarshal([]byte(sourceMap), &raw))
	assert.DeepEqual(t, raw.Sources, []string{"main.effeck"})
	assert.Equal(t, raw.File, "main.js")
	assert.Equal(t, *raw.SourcesContent[0], source)
	at := strings.Index(js, "find(id)")
	assert.Assert(t, at > 0)
	lines := core.ComputeECMALineStarts(js)
	line := 0
	for line+1 < len(lines) && int(lines[line+1]) <= at {
		line++
	}
	column := at - int(lines[line])
	found := false
	for m := range sourcemap.DecodeMappings(raw.Mappings).Values() {
		if m.GeneratedLine == line && int(m.GeneratedCharacter) == column {
			found = true
			// find(id) is on line 3 (index 2), at column 15 (index 15).
			assert.Equal(t, m.SourceLine, 2)
			assert.Equal(t, int(m.SourceCharacter), 15)
		}
	}
	assert.Assert(t, found, "no mapping at %d:%d in %s", line, column, raw.Mappings)
}

// TestEmitReportsUnparsedText checks that text of the rendering that does
// not parse, here a syntax error of the source, reaches the caller as a
// CodeExecSyntax diagnostic in original coordinates, and that Emit still
// returns JavaScript.
func TestEmitReportsUnparsedText(t *testing.T) {
	t.Parallel()
	source := "function f(x: number) {\n  return x +\n}\n"
	js, _, diagnostics := Emit(source, testContext)
	assert.Assert(t, js != "")
	assert.Assert(t, len(diagnostics) > 0)
	for _, d := range diagnostics {
		assert.Equal(t, d.Code, int32(CodeExecSyntax))
		assert.Assert(t, d.Start >= 0 && d.Start+d.Length <= len(source), "diagnostic %v out of range", d)
		assert.Assert(t, strings.Contains(d.Message, "TS1109: Expression expected."), d.Message)
	}
}

// TestEmitScopedContract keeps the contract's closing function brace
// outside the private Scope and optional tracing wrapper (D14, D34, D57).
func TestEmitScopedContract(t *testing.T) {
	t.Parallel()
	ctx := testContext
	ctx.Tracing = "all"
	js, _, diagnostics := Emit("function f(): string!infer E { using r = lease(1); return r.name }", ctx)
	assert.Equal(t, len(diagnostics), 0, "%v\n%s", diagnostics, js)
	assert.Assert(t, strings.Contains(js, "$withSpan($region(($scope) => $gen(function* () {"), js)
	assert.Assert(t, !strings.Contains(js, "f$fast"), js)
}

// TestEmitLoopRegions checks that iteration pulls rather than collecting,
// and that loop callbacks propagate returns and labeled exits (D26).
func TestEmitLoopRegions(t *testing.T) {
	t.Parallel()
	source := `function f(xs: number[]) {
 outer: for (const x of xs) {
   for (const y of xs) { if (y > x) continue outer; if (y === 3) return y }
   if (x === 4) break
 }
 return 0
 }`
	js, _, diagnostics := Emit(source, testContext)
	assert.Equal(t, len(diagnostics), 0, "%v\n%s", diagnostics, js)
	assert.Equal(t, strings.Count(js, "yield* $iterate("), 2, js)
	assert.Assert(t, strings.Contains(js, `return $jump("continue:outer")`), js)
	assert.Assert(t, strings.Contains(js, `return $jump("break:")`), js)
	assert.Assert(t, !strings.Contains(js, "$runCollect"), js)
}

// TestEmitProducerCleanup makes native generator finalizers part of the
// producer Effect's exit, including interruption after an early pull.
func TestEmitProducerCleanup(t *testing.T) {
	t.Parallel()
	source := `function* values() { try { yield 1; yield 2 } finally { release() } }`
	js, _, diagnostics := Emit(source, testContext)
	assert.Equal(t, len(diagnostics), 0, "%v\n%s", diagnostics, js)
	assert.Assert(t, strings.Contains(js, "$produce(() => $producerGen(function* () {"), js)
	assert.Assert(t, strings.Contains(js, "$onExit($gen(() => it)"), js)
	assert.Assert(t, strings.Contains(js, "it.return(undefined)"), js)
}

// TestEmitTracing keeps named functions, variable-bound arrows, and class
// methods in their D34 spans; calls must not bypass them through fast entries.
func TestEmitTracing(t *testing.T) {
	t.Parallel()
	source := `function inner() { return Effect.currentSpan().name }
 const viaArrow = () => Effect.currentSpan().name
 service Greeter { greet(): string!! }
 class Hello implements Greeter { greet() { return Effect.currentSpan().name } }
 export function main() { return [inner(), viaArrow(), new Hello().greet()] }`
	for _, mode := range []string{"all", "exported"} {
		t.Run(mode, func(t *testing.T) {
			ctx := testContext
			ctx.Tracing = mode
			js, _, diagnostics := Emit(source, ctx)
			for _, d := range diagnostics {
				assert.Assert(t, d.Code != CodeExecSyntax, "%v\n%s", diagnostics, js)
			}
			assert.Assert(t, strings.Contains(js, `}), "main")`), js)
			for _, name := range []string{"inner", "viaArrow", "Hello.greet"} {
				assert.Equal(t, strings.Contains(js, `}), "`+name+`")`), mode == "all", js)
			}
			assert.Assert(t, !strings.Contains(js, "main$fast"), js)
			if mode == "all" {
				assert.Assert(t, strings.Contains(js, "(($t = inner()), $isRun($t) ? (yield* $t) : $t)"), js)
				assert.Assert(t, !strings.Contains(js, "inner$fast"), js)
			}
		})
	}
}

// TestEmitClauseContinuation keeps the continuation last when bindings
// omit operation parameters, and propagates a resumed return after cleanup.
func TestEmitClauseContinuation(t *testing.T) {
	t.Parallel()
	source := `control Log { write(message: string, level?: string): void!! }
 function f() {
  try { Log.write("hi"); return 7 }
  handle Log.write(message) { resume(); record(message) }
 }
 function g() {
  try { Log.write("hi"); return 8 }
  handle Log.write(message) { using r = lease(); resume() }
 }`
	js, _, diagnostics := Emit(source, testContext)
	assert.Equal(t, len(diagnostics), 0, "%v\n%s", diagnostics, js)
	assert.Assert(t, strings.Contains(js, "write: (message, ...$clauseArgs) =>"), js)
	assert.Assert(t, strings.Contains(js, "const $k = $clauseArgs[$clauseArgs.length - 1]"), js)
	assert.Assert(t, strings.Contains(js, "return $pending"), js)
	assert.Assert(t, strings.Contains(js, "yield* $resume($k, undefined)"), js)
}
