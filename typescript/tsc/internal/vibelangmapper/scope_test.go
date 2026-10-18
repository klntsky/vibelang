package vibelangmapper

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

var testScope struct {
	sync.Once
	path string
	err  error
}

// TestMain owns the fallback scope and the data-method tests' runtime until
// every parallel test has finished.
func TestMain(m *testing.M) {
	code := m.Run()
	if testScope.path != "" {
		os.RemoveAll(testScope.path)
	}
	if currentRuntime.path != "" {
		os.RemoveAll(currentRuntime.path)
	}
	os.Exit(code)
}

// mapperTestScope supplies parity's package layout without requiring a parity
// run. One build serves all tests, including parallel runtime tests.
func mapperTestScope(t *testing.T, root string) string {
	t.Helper()
	scope := filepath.Join(root, "packages/ts-mapper/test/parity/out/work")
	if _, err := os.Stat(filepath.Join(scope, "node_modules/vibelang-runtime/dist/index.js")); err == nil {
		return scope
	}
	testScope.Do(func() { testScope.err = buildMapperTestScope(root) })
	if testScope.err != nil {
		t.Fatal(testScope.err)
	}
	return testScope.path
}

// buildMapperTestScope copies runtime sources before compiling, so a fresh
// checkout needs only the documented dependency installs and stock compiler.
func buildMapperTestScope(root string) error {
	scope, err := os.MkdirTemp("", "vibelang-mapper-scope-")
	if err != nil {
		return err
	}
	testScope.path = scope
	copyFile := func(from, to string) error {
		b, err := os.ReadFile(from)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(to), 0700); err != nil {
			return err
		}
		return os.WriteFile(to, b, 0600)
	}
	run := func(dir, program string, args ...string) error {
		cmd := exec.Command(program, args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("mapper test scope: %s: %w\n%s", program, err, out)
		}
		return nil
	}
	effectModules := os.Getenv("EFFECT_NODE_MODULES")
	if effectModules == "" {
		effectModules = filepath.Join(root, "experiments/node_modules")
		if _, err := os.Stat(filepath.Join(effectModules, "effect/package.json")); err != nil {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			effectModules = filepath.Join(home, "effectscript/experiments/node_modules")
		}
	}
	effect, err := filepath.EvalSymlinks(filepath.Join(effectModules, "effect"))
	if err != nil {
		return err
	}
	b, err := os.ReadFile(filepath.Join(effect, "package.json"))
	if err != nil {
		return err
	}
	var pkg struct{ Version string }
	if err := json.Unmarshal(b, &pkg); err != nil {
		return err
	}
	if pkg.Version != "4.0.5" {
		return fmt.Errorf("mapper tests require effect 4.0.5, found %s", pkg.Version)
	}
	for _, name := range []string{"package.json", "prelude.d.ts"} {
		if err := copyFile(filepath.Join(root, "packages/ts-mapper", name), filepath.Join(scope, name)); err != nil {
			return err
		}
	}
	for _, dir := range []string{"bin", "node_modules", "runtime-build/node_modules"} {
		if err := os.MkdirAll(filepath.Join(scope, dir), 0700); err != nil {
			return err
		}
	}
	for _, dir := range []string{"node_modules", "runtime-build/node_modules"} {
		if err := os.Symlink(effect, filepath.Join(scope, dir, "effect")); err != nil {
			return err
		}
	}
	for _, tool := range []string{"tsc", "vibelang-mapper"} {
		if err := run(filepath.Join(root, "typescript/tsc"), "go", "build", "-buildvcs=false", "-o", filepath.Join(scope, "bin", tool), "./cmd/"+tool); err != nil {
			return err
		}
	}
	runtime := filepath.Join(root, "packages/runtime")
	if err := filepath.WalkDir(filepath.Join(runtime, "src"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(runtime, path)
		if err != nil {
			return err
		}
		return copyFile(path, filepath.Join(scope, "runtime-build", rel))
	}); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(runtime, "tsconfig.json"), filepath.Join(scope, "runtime-build/tsconfig.json")); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(runtime, "package.json"), filepath.Join(scope, "node_modules/vibelang-runtime/package.json")); err != nil {
		return err
	}
	return run(scope, filepath.Join(scope, "bin/tsc"), "-p", filepath.Join(scope, "runtime-build/tsconfig.json"), "--outDir", filepath.Join(scope, "node_modules/vibelang-runtime/dist"))
}
