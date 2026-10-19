package vibelang

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/bundled"
	"github.com/microsoft/TypeScript/tsc/internal/compiler"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
	"github.com/microsoft/TypeScript/tsc/internal/tsoptions"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
	"github.com/microsoft/TypeScript/tsc/internal/vfs/osvfs"
)

// TestCompilerGoldenCases brings the native acceptance corpus's diagnostic
// spans and emitted JS/declaration contracts into go test's coverage. The
// external smoke harness remains responsible for running their programs.
// This uses the real compiler pipeline and pinned packages, not stub Effect
// types or a subprocess invisible to the coverage profiler (D24, D31, D34).
func TestCompilerGoldenCases(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	cases := filepath.Join(root, "typescript/vibelang/cases")
	dirs, err := os.ReadDir(cases)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range dirs {
		if !entry.IsDir() {
			continue
		}
		fixture := filepath.Join(cases, entry.Name())
		goldens, err := filepath.Glob(filepath.Join(fixture, "expected", "*"))
		if err != nil {
			t.Fatal(err)
		}
		goldens = slices.DeleteFunc(goldens, func(name string) bool {
			return filepath.Base(name) != "diagnostics.txt" && !strings.HasSuffix(name, ".js") && !strings.HasSuffix(name, ".d.ts")
		})
		if len(goldens) == 0 {
			continue
		}
		count++
		t.Run(entry.Name(), func(t *testing.T) {
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
			all := slices.Clone(config.Errors)
			all = append(all, program.GetSyntacticDiagnostics(t.Context(), nil)...)
			all = append(all, program.GetGlobalDiagnostics(t.Context())...)
			all = append(all, program.GetSemanticDiagnostics(t.Context(), nil)...)
			all = compiler.SortAndDeduplicateDiagnostics(all)
			wantDiagnostics, hasDiagnosticGolden := "", false
			needsEmit := false
			for _, golden := range goldens {
				if filepath.Base(golden) == "diagnostics.txt" {
					text, err := os.ReadFile(golden)
					if err != nil {
						t.Fatal(err)
					}
					wantDiagnostics, hasDiagnosticGolden = strings.TrimSpace(string(text)), true
				} else {
					needsEmit = true
				}
			}
			if hasDiagnosticGolden {
				var lines []string
				for _, d := range all {
					lines = append(lines, compilerDiagnosticLine(t, dir, d))
				}
				if got := strings.Join(lines, "\n"); got != wantDiagnostics {
					t.Fatalf("diagnostic golden differs:\ngot:\n%s\nwant:\n%s", got, wantDiagnostics)
				}
			} else {
				for _, d := range all {
					if d.Category() == diagnostics.CategoryError {
						t.Fatalf("unexpected error: %s", d.String())
					}
				}
			}
			if !needsEmit {
				return
			}
			outputs := map[string]string{}
			var outputMutex sync.Mutex
			result := program.Emit(t.Context(), compiler.EmitOptions{WriteFile: func(name tspath.RootedFilePath, text string, _ *compiler.WriteFileData) error {
				rel, err := filepath.Rel(filepath.Join(dir, "out"), string(name))
				if err != nil {
					return err
				}
				outputMutex.Lock()
				outputs[filepath.ToSlash(rel)] = text
				outputMutex.Unlock()
				return nil
			}})
			if result.EmitSkipped {
				t.Fatal("emit was skipped")
			}
			for _, d := range result.Diagnostics {
				if d.Category() == diagnostics.CategoryError {
					t.Fatalf("emit error: %s", d.String())
				}
			}
			for _, golden := range goldens {
				if filepath.Base(golden) == "diagnostics.txt" {
					continue
				}
				want, err := os.ReadFile(golden)
				if err != nil {
					t.Fatal(err)
				}
				if got, ok := outputs[filepath.Base(golden)]; !ok || got != string(want) {
					t.Errorf("%s differs:\ngot:\n%s\nwant:\n%s", filepath.Base(golden), got, want)
				}
			}
		})
	}
	if count == 0 {
		t.Fatal("no compiler goldens discovered")
	}
	t.Logf("checked %d native compiler golden cases", count)
}

func copyCompilerFixture(t *testing.T, fixture, root string) string {
	t.Helper()
	// Unnamed fixture packages derive their nominal service key from the
	// directory name, so preserve it instead of using TempDir's "001".
	temp := t.TempDir()
	// Preserve the acceptance corpus's inherited ESM package scope. Individual
	// fixtures can override it with their own package.json.
	if err := os.WriteFile(filepath.Join(temp, "package.json"), []byte(`{"type":"module"}`), 0600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(temp, filepath.Base(fixture))
	err := filepath.WalkDir(fixture, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(fixture, name)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if rel != "." && (entry.Name() == "node_modules" || entry.Name() == "out" || entry.Name() == "expected" || strings.HasPrefix(entry.Name(), ".")) {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dir, rel), 0755)
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, rel), data, 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "node_modules"), 0755); err != nil {
		t.Fatal(err)
	}
	// Use the runtime's own Effect installation so declarations do not name
	// a second copy through vibelang-runtime/node_modules/effect.
	for pkg, target := range map[string]string{"effect": "packages/runtime/node_modules/effect", "vibelang-runtime": "packages/runtime"} {
		if err := os.Symlink(filepath.Join(root, filepath.FromSlash(target)), filepath.Join(dir, "node_modules", pkg)); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func compilerDiagnosticLine(t *testing.T, dir string, d *ast.Diagnostic) string {
	t.Helper()
	source := d.Source()
	if source == "" {
		source = "TS"
	}
	message := fmt.Sprintf("%s %s%d", d.Category().Name(), source, d.Code())
	if d.File() == nil {
		return message
	}
	file, err := filepath.Rel(dir, string(d.File().FileName()))
	if err != nil {
		t.Fatal(err)
	}
	line, column := scanner.GetECMALineAndUTF16CharacterOfPosition(d.File(), d.Pos())
	return fmt.Sprintf("%s(%d,%d): %s", filepath.ToSlash(file), line+1, column+1, message)
}
