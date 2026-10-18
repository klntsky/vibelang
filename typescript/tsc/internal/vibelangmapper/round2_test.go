package vibelangmapper

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// D9 follows local object members to their calls, including reserved and quoted keys.
func TestRetryCalledObjectCallbacks(t *testing.T) {
	source := `export function scenario() {
 const handlers = {
 default: (n: number) => { sleep("1 millis"); return n + 1 },
 "not-an-identifier": (n: number) => { sleep("1 millis"); return n + 2 },
 arguments: (n: number) => { sleep("1 millis"); return n + 3 },
 yield: (n: number) => { sleep("1 millis"); return n + 4 }
 }; return [handlers.default(1), handlers["not-an-identifier"](1), handlers.arguments(1), handlers.yield(1)]
}`
	for _, projection := range []Projection{Project(source, testContext), Executable(source, testContext)} {
		if len(projection.Diagnostics) != 0 {
			t.Fatalf("%v\n%s", projection.Diagnostics, projection.Text)
		}
		if !strings.Contains(projection.Text, `default: (n: number) => { return $gen(function* () {`) {
			t.Fatal(projection.Text)
		}
	}
}

func TestRetryObjectCallbackShadowing(t *testing.T) {
	source := `function scenario() { const handlers = { default: () => { sleep("1 millis"); return 1 } }; { const handlers = { default: () => 2 }; handlers.default() }; return handlers }`
	projection := Project(source, testContext)
	if len(projection.Diagnostics) != 1 || projection.Diagnostics[0].Code != CodeFormChoice {
		t.Fatalf("%v\n%s", projection.Diagnostics, projection.Text)
	}
}

// D268 must leave known unbranded generic stages in their contextual position.
func TestD268PipeGenericStagesProjection(t *testing.T) {
	source, err := os.ReadFile("../../../vibelang/cases/pipe-generic-stages/main.vibe")
	if err != nil {
		t.Fatal(err)
	}
	for _, projection := range []Projection{Project(string(source), testContext), Executable(string(source), testContext)} {
		if len(projection.Diagnostics) != 0 {
			t.Fatal(projection.Diagnostics)
		}
		if strings.Contains(projection.Text, "$adapt(") {
			t.Fatal(projection.Text)
		}
		if !strings.Contains(projection.Text, "Match.exhaustive") {
			t.Fatal(projection.Text)
		}
	}
}

// The identity overload preserves generics; the describe-shaped pipeline keeps
// contextual inference for Match.exhaustive rather than becoming unknown.
func TestD268UnbrandedGenericTypes(t *testing.T) {
	dir, stock := stockBuiltinProject(t)
	source := `import { Match } from "effect"
const describe = pipe(Match.type<number | string>(), Match.when(Match.number, (n) => String(n)), Match.when(Match.string, (s) => s), Match.exhaustive)
export function main() { return { numbers: describe(1), strings: describe("a") } }`
	projection := Project(source, testContext)
	if len(projection.Diagnostics) != 0 {
		t.Fatal(projection.Diagnostics)
	}
	text := projection.Text + `
import { $adapt as adapt } from "vibelang-ts-mapper/prelude";
import type { Effect as Program } from "effect/Effect";
type Eq<A, B> = (<T>() => T extends A ? 1 : 2) extends (<T>() => T extends B ? 1 : 2) ? true : false;
const describeExact: Eq<typeof describe, (u: string | number) => string> = true;
const describeCheck: (u: string | number) => string = describe;
const resultCheck: Program<{ numbers: string; strings: string }> = main();
const identity = <T>(x: T): T => x;
const preserved: typeof identity = adapt(identity);
const inferred = adapt(identity)({ value: 1 });
const numeric: number = inferred.value;
const inferredExact: Eq<typeof inferred, { value: number }> = true;
const contextual: (value: number) => number = adapt((value) => value);
`
	if err := os.WriteFile(filepath.Join(dir, "main.ts"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(stock, "--ignoreConfig", "--noEmit", "--strict", "--skipLibCheck", "--target", "esnext", "--module", "nodenext", "main.ts")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s\n%s", err, out, text)
	}
}
