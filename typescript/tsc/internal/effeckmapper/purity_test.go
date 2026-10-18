package effeckmapper

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// purityOf returns the purity of source's functions by name, as
// "pure", "plain" (looks plain, not provably pure), or "effect".
func purityOf(t *testing.T, source string) map[string]string {
	t.Helper()
	got := map[string]string{}
	for _, f := range Project(source, testContext).Purity {
		assert.Assert(t, !f.ProvablyPure || f.PlainLooking, "%s is pure but not plain-looking", f.Name)
		assert.Equal(t, source[f.NameStart:f.NameEnd], f.Name)
		assert.Assert(t, strings.HasPrefix(source[f.Start:f.End], "function") || strings.HasPrefix(source[f.Start:f.End], "export"))
		switch {
		case f.ProvablyPure:
			got[f.Name] = "pure"
		case f.PlainLooking:
			got[f.Name] = "plain"
		default:
			got[f.Name] = "effect"
		}
	}
	return got
}

// TestPurityPlainFunctions checks the shapes of the plain-functions case:
// a pure helper, one that calls a method of a value, one that calls a
// plain-annotated function, and functions that run Effects; annotated plain
// functions are plain already and not listed.
func TestPurityPlainFunctions(t *testing.T) {
	t.Parallel()
	source := `import { Effect } from "effect"
const order: string[] = []
function double(n: number) {
  return n * 2
}
function traced(label: string, n: number) {
  order.push(label)
  return n
}
function sumTo(n: number): number {
  return n === 0 ? 0 : n + sumTo(n - 1)
}
function isEven(n: number): boolean {
  return n === 0 ? true : isOdd(n - 1)
}
function isOdd(n: number): boolean {
  return n === 0 ? false : isEven(n - 1)
}
function parity(n: number) {
  return isEven(n) ? "even" : "odd"
}
function area(w: number, h: number) {
  const scaled = traced("w", double(w)) * traced("h", double(h))
  Effect.sleep("1 millis")
  return scaled
}
function maps(xs: ReadonlyArray<number>) {
  const doubled = xs.map(double)
  const viaEffect = Effect.forEach(xs, double)
  return { doubled, viaEffect }
}
export function main() {
  return { sum: sumTo(1000), parity: parity(10), area: area(2, 3), order, maps: maps([1, 2]) }
}
`
	assert.DeepEqual(t, purityOf(t, source), map[string]string{
		"double": "pure",
		"traced": "plain",
		"parity": "plain",
		"area":   "effect",
		"maps":   "effect",
		"main":   "effect",
	})
}

// TestPurityCh13Plain checks the shapes of ch13-plain: pure recursion under
// an Effect contract, a member of a service the file declares, a method of
// a value, and a function in value position.
func TestPurityCh13Plain(t *testing.T) {
	t.Parallel()
	source := `abstract class Settings {
  abstract scale(): Effect<number, never, never>
}
const order: string[] = []
function double(n: number) {
  return n * 2
}
function area(w: number, h: number) {
  return double(w) * double(h) * Settings.scale()
}
function tracedDouble(n: number) {
  order.push(` + "`double ${n}`" + `)
  return n * 2
}
function tracedArea(w: number, h: number) {
  return tracedDouble(w) * tracedDouble(h) * Settings.scale()
}
function sumTo(n: number): Effect<number, never, never> {
  return n === 0 ? 0 : n + sumTo(n - 1)
}
function gcd(a: number, b: number): Effect<number, never, never> {
  return b === 0 ? a : gcd(b, a % b)
}
function doubleAll(xs: ReadonlyArray<number>) {
  return xs.map(double)
}
export function main() {
  return Effect.provide(() => doubleAll([1, 2]), Layer.empty)
}
`
	assert.DeepEqual(t, purityOf(t, source), map[string]string{
		"double":       "pure",
		"area":         "effect",
		"tracedDouble": "plain",
		"tracedArea":   "effect",
		"sumTo":        "pure",
		"gcd":          "pure",
		"doubleAll":    "plain",
		"main":         "effect",
	})
}

// TestPurityNeverOnEffects checks that code that names an Effect never
// looks plain: a throw, a try, a using, a yield-free fork imported from the
// runtime, a value call of a key from another file, a member of another
// file's key, and a call of a function that is not plain itself. A for...of
// and a call of another file's function are of unknown purity.
func TestPurityNeverOnEffects(t *testing.T) {
	t.Parallel()
	source := `import { fork } from "effeck-runtime"
import { Users } from "./users.effeck"
import { helper } from "./helper.effeck"
function thrower(n: number) {
  if (n < 0) throw new Error("negative")
  return n
}
function tries(n: number) {
  try { return helper(n) } catch { return 0 }
}
function leases() {
  using r = lease(1)
  return r.id
}
function forks() {
  fork(() => helper(1))
  return 1
}
function reads() {
  return Users()
}
function finds(id: string) {
  return Users.find(id)
}
function callsThrower(n: number) {
  return thrower(n) + 1
}
function loops(xs: ReadonlyArray<number>) {
  let s = 0
  for (const x of xs) s += x
  return s
}
function delegates(n: number) {
  return helper(n) * 2
}
`
	assert.DeepEqual(t, purityOf(t, source), map[string]string{
		"thrower":      "effect",
		"tries":        "effect",
		"leases":       "effect",
		"forks":        "effect",
		"reads":        "effect",
		"finds":        "effect",
		"callsThrower": "effect",
		"loops":        "plain",
		"delegates":    "plain",
	})
}

// TestEmitPuritySplits checks the D10 output on the corpus's plain-function
// shapes: pure functions get a direct body and property; plain-looking
// calls of methods and plain-annotated functions still use Effect wrappers.
func TestEmitPuritySplits(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name   string
		file   string
		pure   []string
		others []string
	}{
		{
			name: "plain-functions", file: "main.effeck",
			pure:   []string{"double"},
			others: []string{"traced", "parity", "area", "maps", "main", "sumTo", "isEven", "isOdd"},
		},
		{
			name: "ch13-plain", file: "ch13-plain.effeck",
			pure:   []string{"double", "sumTo", "gcd"},
			others: []string{"area", "tracedDouble", "tracedArea", "doubleAll", "scenario", "main"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(filepath.Join("../../../effeck/cases", c.name, c.file))
			assert.NilError(t, err)
			js, _, diagnostics := Emit(string(data), testContext)
			assert.Equal(t, len(diagnostics), 0, "diagnostics: %v", diagnostics)
			for _, name := range c.pure {
				assert.Assert(t, strings.Contains(js, "function "+name+"$direct("), "missing direct body for %s in\n%s", name, js)
				// The corpus's comments can reserve $direct, so its import may have an alias.
				assignment := regexp.MustCompile(regexp.QuoteMeta(name) + `\[\$direct(?:_\d+)?\] = ` + regexp.QuoteMeta(name) + `\$direct;`)
				assert.Assert(t, assignment.MatchString(js), "missing direct property for %s in\n%s", name, js)
			}
			for _, name := range c.others {
				assert.Assert(t, !strings.Contains(js, name+"$direct"), "unexpected direct entry for %s in\n%s", name, js)
				assert.Assert(t, !strings.Contains(js, name+"[$direct"), "unexpected direct property for %s in\n%s", name, js)
			}
		})
	}
}
