package vibelang

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/bundled"
	"github.com/microsoft/TypeScript/tsc/internal/compiler"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
	"github.com/microsoft/TypeScript/tsc/internal/tsoptions"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
	"github.com/microsoft/TypeScript/tsc/internal/vfs/osvfs"
)

// TestCompilerRejectedPrograms verifies every explicit error-code contract in
// the native corpus. Successful programs alone leave unsound-input guards
// untested: these cases exercise invalid contracts, missing providers, plain
// callbacks that pause, escaping scopes, handler misuse and boundary rules.
func TestCompilerRejectedPrograms(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	fixtures, err := filepath.Glob(filepath.Join(root, "typescript/vibelang/cases/*/tsconfig.json"))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, configPath := range fixtures {
		fixture := filepath.Dir(configPath)
		files, err := os.ReadDir(fixture)
		if err != nil {
			t.Fatal(err)
		}
		var expected []string
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".vibe") && !strings.HasSuffix(file.Name(), ".ts") || strings.HasSuffix(file.Name(), ".d.ts") {
				continue
			}
			text, err := os.ReadFile(filepath.Join(fixture, file.Name()))
			if err != nil {
				t.Fatal(err)
			}
			first, _, _ := strings.Cut(string(text), "\n")
			if header, ok := strings.CutPrefix(first, "// expect:"); ok {
				expected = append(expected, strings.Fields(header)...)
			}
		}
		if len(expected) == 0 {
			continue
		}
		count++
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			if pending, err := os.ReadFile(filepath.Join(fixture, "pending")); err == nil {
				t.Skipf("existing acceptance gap: %s", pending)
			}
			dir := copyCompilerFixture(t, fixture, root)
			fileSystem := bundled.WrapFS(osvfs.FS())
			config, errors := tsoptions.GetParsedCommandLineOfConfigFile(tspath.RootedFilePathFromAbsolute(filepath.ToSlash(filepath.Join(dir, "tsconfig.json"))), &core.CompilerOptions{}, nil, fileSystem, nil)
			if len(errors) != 0 {
				t.Fatalf("config: %v", errors)
			}
			program := compiler.NewProgram(compiler.ProgramOptions{Config: config, Host: compiler.NewCompilerHost(fileSystem, bundled.LibPath(), nil, nil, nil)})
			all := commandLineDiagnostics(t, config.Errors, program)
			var actual []string
			for _, diagnostic := range all {
				if diagnostic.Category() == diagnostics.CategoryError {
					line := compilerDiagnosticLine(t, dir, diagnostic)
					_, code, _ := strings.Cut(line, "error ")
					actual = append(actual, code)
				}
			}
			slices.Sort(actual)
			slices.Sort(expected)
			actual, expected = slices.Compact(actual), slices.Compact(expected)
			if !slices.Equal(actual, expected) {
				t.Fatalf("error codes: got %v, want %v\n%v", actual, expected, all)
			}
		})
	}
	if count == 0 {
		t.Fatal("no diagnostic contracts discovered")
	}
	t.Logf("checked %d rejected-program cases", count)
}
