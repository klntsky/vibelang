package effeckmapper

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

// TestCatchDeclarationGoldens pins both clean-file parity regressions from
// decl-catch: an exhaustive switch must not invent a normal completion in
// rethrow, or propagate undefined into its caller's success record.
func TestCatchDeclarationGoldens(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	assert.NilError(t, err)
	scope := filepath.Join(root, "packages/ts-mapper/test/parity/out/work")
	stock := filepath.Join(root, "packages/ts-mapper/test/.cache/typescript-7.1.0-dev.20261017.1/node_modules/.bin/tsc")
	if _, err := os.Stat(stock); err != nil {
		t.Skip("stock TypeScript is installed by e2e.sh")
	}
	if _, err := os.Stat(filepath.Join(scope, "node_modules/effeck-runtime/dist")); err != nil {
		t.Skip("runtime is built by parity")
	}
	dir := t.TempDir()
	assert.NilError(t, os.Symlink(filepath.Join(scope, "node_modules"), filepath.Join(dir, "node_modules")))
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"effeck-ts-mapper","type":"module","exports":{"./prelude":"./prelude.d.ts"}}`), 0600))
	prelude, err := os.ReadFile(filepath.Join(root, "packages/ts-mapper/prelude.d.ts"))
	assert.NilError(t, err)
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "prelude.d.ts"), prelude, 0600))
	source, err := os.ReadFile(filepath.Join(root, "typescript/effeck/cases/decl-catch/main.effeck"))
	assert.NilError(t, err)
	projection := Project(string(source)+"\nexport { rethrow }", testContext)
	assert.Equal(t, len(projection.Diagnostics), 0, "%v", projection.Diagnostics)
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "main.ts"), []byte(projection.Text), 0600))
	cmd := exec.Command(stock, "--ignoreConfig", "--declaration", "--emitDeclarationOnly", "--skipLibCheck", "--strict", "--target", "esnext", "--module", "nodenext", "main.ts")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	assert.NilError(t, err, "%s", out)
	declarations, err := os.ReadFile(filepath.Join(dir, "main.d.ts"))
	assert.NilError(t, err)
	for _, golden := range []struct{ name, text string }{
		{"rethrow", `declare function rethrow(): Effect.Effect<"none" | "rethrown" | "wrongly caught", Broken | Offline, never>;`},
		{"main", `export declare function main(): Effect.Effect<{
    a: string;
    b: string;
    c: string;
    d: string;
    e: string;
    f: string;
    g: string;
}, Broken | Offline | Schema.SchemaError, never>;`},
	} {
		t.Run(golden.name, func(t *testing.T) {
			assert.Assert(t, strings.Contains(string(declarations), golden.text), "%s", declarations)
		})
	}
}
