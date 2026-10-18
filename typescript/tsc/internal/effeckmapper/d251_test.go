package effeckmapper

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// TestD251StaticDataBoundary checks opaque declarations with stock TypeScript:
// identical runtime Exits must be distinguished by their declared types.
func TestD251StaticDataBoundary(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	assert.NilError(t, err)
	scope := filepath.Join(root, "packages/ts-mapper/test/parity/out/work")
	stock := filepath.Join(root, "packages/ts-mapper/test/.cache/typescript-7.1.0-dev.20261010.1/node_modules/.bin/tsc")
	if _, err := os.Stat(stock); err != nil {
		t.Fatal("stock TypeScript is installed by e2e")
	}
	dir := t.TempDir()
	assert.NilError(t, os.Symlink(filepath.Join(scope, "node_modules"), filepath.Join(dir, "node_modules")))
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"effeck-ts-mapper","type":"module","exports":{"./prelude":"./prelude.d.ts"}}`), 0600))
	prelude, err := os.ReadFile(filepath.Join(root, "packages/ts-mapper/prelude.d.ts"))
	assert.NilError(t, err)
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "prelude.d.ts"), prelude, 0600))
	helper := `import { Effect, Exit } from "effect";
export function data(): Exit.Exit<number> { return Exit.succeed(42) }
export function program(): Effect.Effect<number> { return Exit.succeed(42) }`
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "helper.ts"), []byte(helper), 0600))
	source := `import { data, program } from "./helper.js";
export function bad() { return data() }
export function good() { return program() }
export function kept() { return sync((): Exit.Exit<number> => data()) }
export function resumed() { return Effect.callback<number>((resume): void => { resume(Exit.succeed(9)) }) }
export function generic<T>(x: T) { return x }`
	projection := Project(source, testContext)
	assert.Equal(t, len(projection.Diagnostics), 0, "%v", projection.Diagnostics)
	assert.Assert(t, strings.Contains(projection.Text, "$run($value(data)())"), "%s", projection.Text)
	assert.Assert(t, strings.Contains(projection.Text, "resume(Exit.succeed(9))"), "%s", projection.Text)
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "main.ts"), []byte(projection.Text+`
import { $run as runProbe, $gen as genProbe } from "./prelude.js"; export function genericRun<T>(x: T) { return runProbe(x) }
export function optional<T>(x: T | undefined) { return runProbe(x) }
import { Config, Context, Cause } from "effect";
declare const config: Config.Config<number>;
declare const key: Context.Key<{}, {}>;
declare const error: Cause.YieldableError;
declare const mixed: Exit.Exit<number> | number;
function configProbe() { return genProbe(function* () { return yield* runProbe(config) }) }
function keyProbe() { return genProbe(function* () { return yield* runProbe(key) }) }
function errorProbe() { return genProbe(function* () { return yield* runProbe(error) }) }
function mixedProbe() { return genProbe(function* () { return yield* runProbe(mixed) }) }`), 0600))
	cmd := exec.Command(stock, "--ignoreConfig", "--declaration", "--emitDeclarationOnly", "--skipLibCheck", "--strict", "--target", "esnext", "--module", "nodenext", "main.ts")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	assert.Assert(t, err != nil, "%s", out)
	assert.Assert(t, strings.Contains(string(out), "sync((): Exit.Exit<A, E>"), "%s", out)
	assert.Equal(t, strings.Count(string(out), "error TS"), 5, "%s", out)
	declarations, err := os.ReadFile(filepath.Join(dir, "main.d.ts"))
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(string(declarations), "generic<T>(x: T): Effect.Effect<T, never, never>"), "%s", declarations)
	assert.Assert(t, strings.Contains(string(declarations), "genericRun<T>(x: T): Effect.Effect<T, never, never>"), "%s", declarations)
	assert.Assert(t, strings.Contains(string(declarations), "optional<T>(x: T | undefined): Effect.Effect<T | undefined, never, never>"), "%s", declarations)
	js, _, ds := Emit(strings.Replace(source, "export function bad() { return data() }", "", 1), testContext)
	assert.Equal(t, len(ds), 0, "%v", ds)
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "main.mjs"), []byte(js+`
console.log(JSON.stringify([await Effect.runPromise(good()), (await Effect.runPromise(kept())).value, await Effect.runPromise(resumed())]))`), 0600))
	// The helper's annotations erase to the same constructor at runtime.
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "helper.js"), []byte(`import { Exit } from "effect"; export const data = () => Exit.succeed(42); export const program = data;`), 0600))
	cmd = exec.Command("node", "--no-warnings", "main.mjs")
	cmd.Dir = dir
	out, err = cmd.CombinedOutput()
	assert.NilError(t, err, "%s", out)
	assert.Equal(t, strings.TrimSpace(string(out)), "[42,42,9]")
}

// TestD251ProjectionGolden keeps the static boundary in checking output and
// preserves the explicitly plain D43 registration body without run edits.
func TestD251ProjectionGolden(t *testing.T) {
	source := `import { data } from "./helper.js"; function f() { return data() } function g() { return Effect.callback<number>((resume): void => { resume(Exit.succeed(9)) }) }`
	projection := Project(source, testContext)
	assert.Equal(t, len(projection.Diagnostics), 0)
	assert.Equal(t, virtualBody(t, projection), `import { data } from "./helper.js"; function f() { return $gen(function* () { return (yield* $run($value(data)())) })} function g() { return $gen(function* () { return (yield* $run(Effect.callback<number>((resume): void => { resume(Exit.succeed(9)) }))) })}`)
}
