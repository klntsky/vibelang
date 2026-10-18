package effeckmapper

import (
	"bytes"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

// forms looks callee up in the embedded table and fails the test when the
// table has no answer.
func forms(t *testing.T, callee string, argc int) Forms {
	t.Helper()
	f, ok := FormsFor(callee, argc)
	if !ok {
		t.Fatalf("FormsFor(%q, %d): no answer", callee, argc)
	}
	return f
}

func TestFormsForEffectForEach(t *testing.T) {
	// forEach(self, f, options?) and forEach(f, options?): with two
	// arguments the data-first form ranks first and takes an Effect
	// function second; the data-last form disagrees there.
	f := forms(t, "effect/Effect#forEach", 2)
	if f.Callee != "effect/Effect#forEach" || len(f.Candidates) != 2 || f.Candidates[0].Min != 2 {
		t.Fatalf("candidates = %+v", f)
	}
	if f.Params[1].Top != ClassEffectFn || f.Agree[1] {
		t.Errorf("argument 1 = %v, agree %v; want E from the data-first form, candidates disagreeing", f.Params[1].Top, f.Agree[1])
	}
	// An import of the root, a D151 prelude name, and the prelude's Effect
	// all lead to the same key.
	for _, written := range []string{"effect#Effect.forEach", "prelude#forEach", "prelude#Effect.forEach", "effeck-runtime#Effect.forEach"} {
		if g := forms(t, written, 2); g.Callee != "effect/Effect#forEach" {
			t.Errorf("%s leads to %s", written, g.Callee)
		}
	}
}

func TestFormsForEffectRetry(t *testing.T) {
	// retry(self, policy): the self is a program in every data-first
	// overload; the policy is an options object, a Schedule, or a
	// function, so the overloads disagree on argument 1.
	f := forms(t, "effect/Effect#retry", 2)
	if len(f.Candidates) != 3 || !f.Candidates[0].DataFirst {
		t.Fatalf("candidates = %+v", f.Candidates)
	}
	if f.Params[0].Top != ClassProgram || !f.Agree[0] {
		t.Errorf("argument 0 = %v agree %v, want P agreed", f.Params[0].Top, f.Agree[0])
	}
	if f.Agree[1] || f.Agreed() {
		t.Error("the policy overloads should disagree on argument 1")
	}
	if c := f.Params[1].Props["while"]; c != ClassEffectFn|ClassPlainFn {
		t.Errorf("options.while = %v, want EF (a predicate or an Effect predicate)", c)
	}
}

func TestFormsForEffectTimeout(t *testing.T) {
	f := forms(t, "effect/Effect#timeout", 2)
	if !f.Agreed() || f.Params[0].Top != ClassProgram || !f.Candidates[0].DataFirst {
		t.Errorf("timeout(self, duration) = %+v", f)
	}
	g := forms(t, "effect/Effect#timeout", 1)
	if len(g.Candidates) != 1 || !g.Candidates[0].DataLast || g.Params[0].Top != 0 {
		t.Errorf("timeout(duration) = %+v, want the data-last form, a plain duration", g)
	}
}

func TestFormsForArrayPrototypeMapAbsent(t *testing.T) {
	// Array is JavaScript's global in an .effeck file (D146), and a method
	// of a value has no key: the table has nothing to say.
	for _, key := range []string{"prelude#Array.prototype.map", "prelude#Array.from", "Array.prototype.map"} {
		if f, ok := FormsFor(key, 1); ok {
			t.Errorf("FormsFor(%q) = %+v, want no answer", key, f)
		}
	}
	// effect's own Array.map is a different callee, and takes a plain callback.
	if f := forms(t, "effect/Array#map", 2); f.Params[1].Top != ClassPlainFn {
		t.Errorf("effect/Array#map callback = %v, want F", f.Params[1].Top)
	}
	r := resolve(t, `export function f(xs: number[]) { return xs.map((x) => x + 1) }`)
	d := r.lambda(t, "(x) =>")
	if d.Callee.Kind != CalleeNone || d.Provenance != FromFallback || d.Form != FormPlain {
		t.Errorf("xs.map's lambda = %+v, want no key, the fallback, plain", d)
	}
}

func TestFormsForLayerEffect(t *testing.T) {
	f := forms(t, "effect/Layer#effect", 2)
	if f.Params[1].Top != ClassProgram {
		t.Errorf("Layer.effect(key, program) argument 1 = %v, want P", f.Params[1].Top)
	}
	g := forms(t, "prelude#Layer.effect", 1)
	if !g.Candidates[0].DataLast {
		t.Error("Layer.effect(key) should be data-last: it returns a function of the program")
	}
}

func TestFormsForItEffect(t *testing.T) {
	f := forms(t, "@effect/vitest#it.effect", 2)
	if f.Params[1].Top != ClassEffectFn {
		t.Errorf("it.effect(name, self) argument 1 = %v, want E", f.Params[1].Top)
	}
}

func TestFormsForEffectAll(t *testing.T) {
	// Effect.all([a, b]) and Effect.all({ a, b }): elements and properties
	// are programs (D49); the argument itself is not.
	f := forms(t, "effect/Effect#all", 1)
	if p := f.Params[0]; p.Top != 0 || p.Elem != ClassProgram || p.AnyProp != ClassProgram {
		t.Errorf("all's argument = %+v, want elements and properties P", p)
	}
	if c := f.Params[0].At([]PathStep{{Element: true}}); c != ClassProgram {
		t.Errorf("At(element) = %v", c)
	}
	if c := f.Params[0].At([]PathStep{{Property: "users"}}); c != ClassProgram {
		t.Errorf("At(property) = %v", c)
	}
}

func TestFormsForEffectCatch(t *testing.T) {
	f := forms(t, "effect/Effect#catch", 2)
	if f.Params[0].Top != ClassProgram || f.Params[1].Top != ClassEffectFn || !f.Agreed() {
		t.Errorf("catch(self, f) = %+v, want P, E", f.Params)
	}
	g := forms(t, "effect/Effect#catch", 1)
	if g.Params[0].Top != ClassEffectFn || !g.Candidates[0].DataLast {
		t.Errorf("catch(f) = %+v, want the data-last form taking E", g)
	}
}

func TestFormsForEffectSync(t *testing.T) {
	if f := forms(t, "effect/Effect#sync", 1); f.Params[0].Top != ClassPlainFn {
		t.Errorf("sync(evaluate) = %v, want F", f.Params[0].Top)
	}
	r := resolve(t, `import { Effect } from "effect"
export function f() { return Effect.sync(() => Date.now()) }`)
	if d := r.lambda(t, "() => Date"); d.Form != FormPlain || d.Provenance != FromTable {
		t.Errorf("sync's lambda = %+v, want plain from the table", d)
	}
}

func TestBlockPlacementWithSpan(t *testing.T) {
	// D188: withSpan's data-first form takes the block as its first
	// argument; the block is a program.
	r := resolve(t, `export function f() {
  return withSpan("load") {
    return 1
  }
}`)
	d := r.block(t, `withSpan("load")`)
	if d.Placement != BlockPrepended || d.Form != FormProgram || d.Callee.Canonical != "effect/Effect#withSpan" {
		t.Errorf("withSpan block = %+v, want prepended program", d)
	}
	// Layer.effect(Key) { … }: the data-last form is applied to the block.
	r = resolve(t, `service Users { find(id: string): string!! }
export const UsersLive = Layer.effect(Users) {
  return { find(id) { return id } }
}`)
	if d := r.block(t, "Layer.effect(Users)"); d.Placement != BlockApplied || d.Form != FormProgram {
		t.Errorf("Layer.effect block = %+v, want applied program", d)
	}
	// A block after the pipe topic: % fills the data slot, so the block is
	// the data-first form's last argument.
	r = resolve(t, `import { Effect } from "effect"
export function f() {
  return Effect.succeed({}) |> Effect.bind(%, "value") {
    return 1
  }
}`)
	if d := r.block(t, `Effect.bind(%, "value")`); d.Placement != BlockAppended || d.Form != FormEffectFn {
		t.Errorf("bind block after the topic = %+v, want appended Effect function", d)
	}
}

func TestEmbeddedTableHeader(t *testing.T) {
	table, err := DefaultTable()
	if err != nil {
		t.Fatal(err)
	}
	h := table.Header
	if h.Notice != "generated; do not edit" || !strings.Contains(h.Command, "check-forms-table.sh") {
		t.Errorf("header = %+v", h)
	}
	pinned := map[string]Input{}
	for _, in := range h.Inputs {
		if _, dup := pinned[in.Package]; dup {
			t.Errorf("%s is pinned twice: the program loaded two copies", in.Package)
		}
		pinned[in.Package] = in
		if len(in.SHA256) != 64 || in.Files == 0 {
			t.Errorf("input %+v has no content hash", in)
		}
	}
	for pkg, version := range map[string]string{"effect": "4.0.5", "@effect/vitest": "4.0.5", "@effect/platform-node": "4.0.5", "@effect/platform-bun": "4.0.5", "effeck-runtime": "0.0.1"} {
		if pinned[pkg].Version != version {
			t.Errorf("%s pinned at %q, want %s", pkg, pinned[pkg].Version, version)
		}
	}
	if h.Keys != len(table.Keys()) || h.Aliases != len(table.Aliases()) {
		t.Errorf("header counts %d keys, %d aliases; table has %d, %d", h.Keys, h.Aliases, len(table.Keys()), len(table.Aliases()))
	}
}

// TestTableFormatRoundTrips checks that writing the parsed embedded table
// gives the embedded bytes: the writer is the format's only author.
func TestTableFormatRoundTrips(t *testing.T) {
	table, err := DefaultTable()
	if err != nil {
		t.Fatal(err)
	}
	entries := map[string][]Signature{}
	for _, k := range table.Keys() {
		entries[k] = table.Signatures(k)
	}
	var buf bytes.Buffer
	if err := WriteTable(&buf, table.Header, entries, table.Aliases()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), embeddedTable) {
		t.Error("rewriting the embedded table changes its bytes")
	}
}

func TestResolverAliasAndShadowing(t *testing.T) {
	r := resolve(t, `import { Effect as E } from "effect"
import * as Arr from "effect/Array"
export function a() { return E.catch(() => E.succeed(1), (e) => e) }
export function b(names: ReadonlyArray<string>) {
  const forEach = Arr.map
  return forEach(names, (n) => n.length)
}
export function c(names: ReadonlyArray<string>) { return forEach(names, (n) => n.length) }`)
	if d := r.lambda(t, "() => E.succeed"); d.Callee.Canonical != "effect/Effect#catch" || d.Form != FormProgram {
		t.Errorf("E.catch's first lambda = %+v, want a program through the alias", d)
	}
	if d := r.lambda(t, "(e) => e"); d.Form != FormEffectFn {
		t.Errorf("E.catch's handler = %+v, want an Effect function", d)
	}
	shadowed := r.lambdas(t, "(n) => n.length")
	if len(shadowed) != 2 {
		t.Fatalf("found %d lambdas", len(shadowed))
	}
	if d := shadowed[0]; d.Callee.Canonical != "effect/Array#map" || d.Form != FormPlain {
		t.Errorf("the local forEach (Arr.map) lambda = %+v, want plain", d)
	}
	if d := shadowed[1]; d.Callee.Canonical != "effect/Effect#forEach" || d.Form != FormEffectFn {
		t.Errorf("the prelude forEach lambda = %+v, want an Effect function", d)
	}
}

func TestResolverSameFileAnnotations(t *testing.T) {
	// D12: a callback parameter written without a contract accepts any
	// contract, so a lambda passed there is an Effect function; an
	// unannotated parameter says nothing.
	r := resolve(t, `function apply(f: (x: number) => number, g) { return 0 }
function run(p: Effect<number>) { return 0 }
export function main() {
  apply((x) => x + 1, (y) => y)
  run(() => 1)
}`)
	if d := r.lambda(t, "(x) => x + 1"); d.Provenance != FromAnnotation || d.Form != FormEffectFn {
		t.Errorf("written callback = %+v, want an Effect function from the annotation (D12)", d)
	}
	if d := r.lambda(t, "(y) => y"); d.Provenance != FromFallback || d.Form != FormPlain {
		t.Errorf("unannotated parameter = %+v, want the fallback", d)
	}
	if d := r.lambda(t, "() => 1"); d.Provenance != FromAnnotation || d.Form != FormProgram {
		t.Errorf("Effect<number> parameter = %+v, want a program", d)
	}
}

func TestResolverServiceContext(t *testing.T) {
	r := resolve(t, `import { Context } from "effect"
import { Mailer } from "./mailer.effeck"
service Users { find(id: string): string!! }
class Greeter extends Context.Service<Greeter, { readonly greet: (name: string) => Effect<string> }>()("Greeter") {}
export const UsersTest = Layer.succeed(Users, { find: (id) => id })
export const GreeterLive = Layer.succeed(Greeter, { greet: (name) => name })
export const MailerTest = Layer.succeed(Mailer, { send: (to) => to })
export const RandomFixed = Effect.provideService(Random.next(), Random.Random, { nextIntUnsafe: () => 4 })`)
	for _, c := range []struct{ text, note string }{
		{"(id) => id", "member Users.find"},
		{"(name) => name", "member Greeter.greet"},
		{"(to) => to", "provider of a service from ./mailer.effeck"},
	} {
		if d := r.lambda(t, c.text); d.Provenance != FromService || d.Form != FormEffectFn || d.Note != c.note {
			t.Errorf("%s = %+v, want an Effect function (%s)", c.text, d, c.note)
		}
	}
	// effect's own Random has plain members: no service context.
	if d := r.lambda(t, "() => 4"); d.Provenance != FromFallback || d.Form != FormPlain {
		t.Errorf("Random.Random member = %+v, want the fallback", d)
	}
}

func TestResolverByName(t *testing.T) {
	r := resolve(t, `import { Effect } from "effect"
declare function fetchUser(): Effect<number>
export function f() {
  const a = Effect.retry(fetchUser(), { times: 2 })
  const b = fetchUser() |> Effect.timeout(%, "1 second")
  const c = Effect.succeed(fetchUser())
}`)
	if d := r.argument(t, "fetchUser(), { times"); !d.ByName || d.Provenance != FromTable {
		t.Errorf("retry's self = %+v, want by name", d)
	}
	if d := r.argument(t, `%, "1 second"`); !d.ByName {
		t.Errorf("the topic at timeout's self = %+v, want by name", d)
	}
	if d := r.argument(t, "fetchUser())\n"); d.ByName {
		t.Errorf("succeed's value = %+v, want it to run", d)
	}
}

// resolved is a parsed .effeck file and its resolver.
type resolved struct {
	*Resolver
	text string
	file *ast.SourceFile
}

func resolve(t *testing.T, text string) resolved {
	t.Helper()
	table, err := DefaultTable()
	if err != nil {
		t.Fatal(err)
	}
	file := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: tspath.RootedFilePathFromAbsolute("/test.effeck")}, text, core.ScriptKindTS)
	return resolved{Resolver: NewResolver(file, table), text: text, file: file}
}

// find returns the nodes of kind accepted whose text starts with prefix.
func (r resolved) find(prefix string, accept func(*ast.Node) bool) []*ast.Node {
	var out []*ast.Node
	var visit func(n *ast.Node) bool
	visit = func(n *ast.Node) bool {
		if accept(n) {
			start := n.Pos()
			for start < len(r.text) && (r.text[start] == ' ' || r.text[start] == '\n') {
				start++
			}
			if strings.HasPrefix(r.text[start:], prefix) {
				out = append(out, n)
			}
		}
		n.ForEachChild(visit)
		return false
	}
	r.file.AsNode().ForEachChild(visit)
	return out
}

func isLambdaNode(n *ast.Node) bool {
	return n.Kind == ast.KindArrowFunction || n.Kind == ast.KindFunctionExpression || n.Kind == ast.KindMethodDeclaration
}

func (r resolved) lambdas(t *testing.T, prefix string) []Decision {
	t.Helper()
	var out []Decision
	for _, n := range r.find(prefix, isLambdaNode) {
		out = append(out, r.Lambda(n))
	}
	return out
}

func (r resolved) lambda(t *testing.T, prefix string) Decision {
	t.Helper()
	ds := r.lambdas(t, prefix)
	if len(ds) != 1 {
		t.Fatalf("found %d lambdas starting %q", len(ds), prefix)
	}
	return ds[0]
}

func (r resolved) block(t *testing.T, prefix string) Decision {
	t.Helper()
	calls := r.find(prefix, func(n *ast.Node) bool { return ast.IsCallExpression(n) && formsIsBlockCall(r.text, n) })
	if len(calls) != 1 {
		t.Fatalf("found %d block calls starting %q", len(calls), prefix)
	}
	return r.Block(calls[0])
}

func (r resolved) argument(t *testing.T, prefix string) Decision {
	t.Helper()
	args := r.find(prefix, func(n *ast.Node) bool {
		call, _, _ := formsOwnerCall(n)
		return call != nil && !isLambdaNode(n)
	})
	if len(args) == 0 {
		t.Fatalf("found no argument starting %q", prefix)
	}
	return r.Argument(args[0])
}

func TestResolverLambdaReturnAnnotation(t *testing.T) {
	// Where the call gives no class, a lambda's own return annotation
	// decides: a plain type keeps it plain, a contract makes it an Effect
	// function. Without one, the call outside tail position decides.
	r := resolve(t, `function double(x: number): number { return x * 2 }
export function f(xs: number[]) {
  const a = xs.map((x): number => double(x) + 1)
  const b = xs.map((x): number!! => double(x) + 1)
  const c = xs.map((x) => double(x) + 1)
}`)
	for _, c := range []struct {
		text string
		form Form
		prov Provenance
	}{
		{"(x): number =>", FormPlain, FromAnnotation},
		{"(x): number!! =>", FormEffectFn, FromAnnotation},
		{"(x) => double", FormEffectFn, FromFallback},
	} {
		if d := r.lambda(t, c.text); d.Form != c.form || d.Provenance != c.prov {
			t.Errorf("%s = %v from %v, want %v from %v", c.text, d.Form, d.Provenance, c.form, c.prov)
		}
	}
}

func TestResolverSyntaxContexts(t *testing.T) {
	r := resolve(t, `interface LineFile { readLine(): Effect<string | undefined, never, never> }
service Banner {
  text(title: string): string!!
  static make(): Banner!! {
    return { text: (title) => title }
  }
}
class Runner {
  run(name: string, program: () => string!!) { return name }
}
export function f() {
  const tuple = Effect.all([() => 12] as const)
  const file: LineFile = { readLine: () => undefined }
  const made = 3 |> (() => % + 1)
  const runner = new Runner()
  return runner.run("job") {
    return "done"
  }
}`)
	// as const keeps the array an argument of Effect.all: its elements are programs.
	if d := r.lambda(t, "() => 12"); d.Form != FormProgram || d.Provenance != FromTable {
		t.Errorf("element under as const = %+v, want a program from the table", d)
	}
	// A member of an object typed by a written interface or service.
	if d := r.lambda(t, "() => undefined"); d.Form != FormEffectFn || d.Provenance != FromAnnotation {
		t.Errorf("LineFile.readLine = %+v, want an Effect function from the annotation", d)
	}
	if d := r.lambda(t, "(title) => title"); d.Form != FormEffectFn || d.Provenance != FromAnnotation {
		t.Errorf("Banner.text returned by make() = %+v, want an Effect function", d)
	}
	// The topic is a value, not an Effeck statement.
	if d := r.lambda(t, "() => % + 1"); d.Form != FormPlain {
		t.Errorf("lambda over the topic = %+v, want plain", d)
	}
	// A method of a class this file declares, through new.
	if d := r.block(t, `runner.run("job")`); d.Form != FormEffectFn || d.Placement != BlockAppended || d.Provenance != FromAnnotation {
		t.Errorf("runner.run block = %+v, want an appended Effect function", d)
	}
}
