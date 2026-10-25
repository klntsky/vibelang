package vibelang

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/astnav"
	"github.com/microsoft/TypeScript/tsc/internal/bundled"
	"github.com/microsoft/TypeScript/tsc/internal/compiler"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
	"github.com/microsoft/TypeScript/tsc/internal/ls"
	"github.com/microsoft/TypeScript/tsc/internal/ls/lsconv"
	"github.com/microsoft/TypeScript/tsc/internal/ls/lsutil"
	"github.com/microsoft/TypeScript/tsc/internal/lsp/lsproto"
	"github.com/microsoft/TypeScript/tsc/internal/tsoptions"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
	"github.com/microsoft/TypeScript/tsc/internal/vfs/osvfs"
)

type editorTestHost struct {
	ls.Host
	converters *lsconv.Converters
}

func (h editorTestHost) Converters() *lsconv.Converters { return h.converters }
func (h editorTestHost) GetPreferences(string) lsutil.UserPreferences {
	return lsutil.UserPreferences{}
}

func editorProgram(t *testing.T, source string) (*compiler.Program, *ast.SourceFile, *ls.LanguageService, *lsconv.Converters) {
	t.Helper()
	dir := t.TempDir()
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "node_modules"), 0755); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{"effect": "packages/runtime/node_modules/effect", "vibelang-runtime": "packages/runtime"} {
		if err := os.Symlink(filepath.Join(root, target), filepath.Join(dir, "node_modules", name)); err != nil {
			t.Fatal(err)
		}
	}
	for name, text := range map[string]string{
		"package.json":  `{"type":"module"}`,
		"tsconfig.json": `{"compilerOptions":{"target":"ES2022","module":"NodeNext","strict":true,"skipLibCheck":true,"noEmit":true},"files":["main.vibe"]}`,
		"main.vibe":     source,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	fs := bundled.WrapFS(osvfs.FS())
	config, errors := tsoptions.GetParsedCommandLineOfConfigFile(tspath.RootedFilePathFromAbsolute(filepath.ToSlash(filepath.Join(dir, "tsconfig.json"))), &core.CompilerOptions{}, nil, fs, nil)
	if len(errors) != 0 || len(config.Errors) != 0 {
		t.Fatalf("config: %v %v", errors, config.Errors)
	}
	program := compiler.NewProgram(compiler.ProgramOptions{Config: config, Host: compiler.NewCompilerHost(fs, bundled.LibPath(), nil, nil, nil)})
	file := program.GetSourceFile(tspath.RootedFilePathFromAbsolute(filepath.ToSlash(filepath.Join(dir, "main.vibe"))))
	converters := lsconv.NewConverters(lsproto.PositionEncodingKindUTF16, func(name tspath.RootedFilePath) *lsconv.LSPLineMap {
		text, ok := fs.ReadFile(name)
		if !ok {
			t.Fatalf("missing editor document %s", name)
		}
		return lsconv.ComputeLSPLineStarts(text)
	})
	service := ls.NewLanguageService(nil, program, editorTestHost{converters: converters}, string(file.FileName()))
	return program, file, service, converters
}

func applyEditorEdits(t *testing.T, file *ast.SourceFile, converters *lsconv.Converters, edits []*lsproto.TextEdit) string {
	t.Helper()
	edits = slices.Clone(edits)
	slices.SortFunc(edits, func(a, b *lsproto.TextEdit) int {
		return converters.FromLSPRangeToOriginal(file, b.Range).Pos() - converters.FromLSPRangeToOriginal(file, a.Range).Pos()
	})
	text := file.Text()
	previous := len(text)
	for _, edit := range edits {
		span := converters.FromLSPRangeToOriginal(file, edit.Range)
		if span.Pos() < 0 || span.End() > previous || span.Pos() > span.End() {
			t.Fatalf("invalid or overlapping editor edit: %+v", edit)
		}
		text = text[:span.Pos()] + edit.NewText + text[span.End():]
		previous = span.Pos()
	}
	return text
}

// D245 promises usable source rewrites. Compare the complete edited source,
// with real UTF-16 editor coordinates, then check the result with the compiler.
func TestEditorRefactorGoldens(t *testing.T) {
	const helpers = "function f(x: number): number { return x * 2 }\nfunction g(x: number): number { return x + 1 }\n"
	for _, test := range []struct{ name, source, action, want string }{
		{"interface to schema", "export interface /*cursor*/Person { readonly name: string }", "Convert to a schema declaration", "export schema Person { readonly name: string }"},
		{"type to schema", "export type /*cursor*/Person = { name: string }", "Convert to a schema declaration", "export schema Person { name: string }"},
		{"inferred contract", "export function /*cursor*/double(x: number) { return x * 2 }", "Write the inferred contract", "export function double(x: number): number!! { return x * 2 }"},
		{"inferred effect contract", "export function /*cursor*/load() { return Effect.succeed(1) }", "Write the inferred contract", "export function load(): number!! { return Effect.succeed(1) }"},
		{"remove contract", "export function /*cursor*/double(x: number): number!! { return x * 2 }", "Remove the contract", "export function double(x: number) { return x * 2 }"},
		{"nested calls", helpers + "export const value = /*cursor*/f(g(1))", "Convert to a pipe", helpers + "export const value = 1 |> g(%) |> f(%)"},
		{"pipe to calls", helpers + "export const value = 1 /*cursor*/|> g(%) |> f(%)", "Convert the pipe to calls", helpers + "export const value = f(g(1))"},
		{"pipe precedence", "export const value = (1 + 2) /*cursor*/|> % * 3", "Convert the pipe to calls", "export const value = (1 + 2) * 3"},
	} {
		t.Run(test.name, func(t *testing.T) {
			const prefix = "const café = '😀'; "
			pos := len(prefix) + strings.Index(test.source, "/*cursor*/")
			source := prefix + strings.ReplaceAll(test.source, "/*cursor*/", "")
			program, file, service, converters := editorProgram(t, source)
			actions := vibelangRefactors(t.Context(), &ls.RefactorContext{SourceFile: file, Span: core.NewTextRange(pos, pos), Program: program, LS: service})
			var chosen *ls.CodeAction
			for _, action := range actions {
				if action.Description == test.action {
					chosen = action
				}
			}
			if chosen == nil {
				t.Fatalf("missing refactor %q: %+v", test.action, actions)
			}
			got := applyEditorEdits(t, file, converters, chosen.Changes)
			if got != prefix+test.want {
				t.Fatalf("edited source differs:\ngot: %s\nwant: %s", got, prefix+test.want)
			}
			checked, edited, _, _ := editorProgram(t, got)
			all := append(checked.GetSyntacticDiagnostics(t.Context(), edited), checked.GetSemanticDiagnostics(t.Context(), edited)...)
			for _, d := range all {
				if d.Category() == diagnostics.CategoryError {
					t.Fatalf("refactor produced a compiler error: %s", d.String())
				}
			}
		})
	}
}

func TestEditorRefactorsDeclineUnsupportedRewrites(t *testing.T) {
	for _, test := range []struct{ source, action string }{
		{"interface /*cursor*/Box<T> { value: T }", "Convert to a schema declaration"},
		{"interface Base { value: number }; interface /*cursor*/Child extends Base {}", "Convert to a schema declaration"},
		{"const value = 1 /*cursor*/|> % + %", "Convert the pipe to calls"},
		{"function /*cursor*/plain(): number { return 1 }", "Remove the contract"},
		{"async function /*cursor*/nested() { return await Promise.resolve(await Promise.resolve(1)) }", "Convert to a VibeLang function"},
	} {
		pos := strings.Index(test.source, "/*cursor*/")
		source := strings.ReplaceAll(test.source, "/*cursor*/", "")
		program, file, service, _ := editorProgram(t, source)
		for _, action := range vibelangRefactors(t.Context(), &ls.RefactorContext{SourceFile: file, Span: core.NewTextRange(pos, pos), Program: program, LS: service}) {
			if action.Description == test.action {
				t.Errorf("unsupported refactor %q offered for %s", test.action, source)
			}
		}
	}
}

func TestEditorHoverExplainsCallBehavior(t *testing.T) {
	for _, test := range []struct{ name, source, signature, note string }{
		{"plain function", "function double(x: number) { return x * 2 }; function main() { return /*cursor*/double(2) }", "function double(x: number): number", "Plain JavaScript: VibeLang code calls it directly."},
		{"running effect", "function main() { return Effect./*cursor*/succeed(1) }", "", "Runs here and returns `number`."},
		{"deferred effect", "function main() { return Effect.all([Effect./*cursor*/succeed(1)]) }", "", "Passed by name to `Effect.all` as a program:"},
		{"hatch body", "function main() { return sync(() => Effect./*cursor*/succeed(1)) }", "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			pos := strings.Index(test.source, "/*cursor*/")
			source := strings.ReplaceAll(test.source, "/*cursor*/", "")
			program, file, _, _ := editorProgram(t, source)
			program.GetSemanticDiagnostics(t.Context(), file)
			c, done := program.GetTypeCheckerForFile(t.Context(), file)
			defer done()
			node := astnav.GetTouchingPropertyName(file, pos)
			signature, note := plainFunctionQuickInfo(c, node)
			_, running := runningCallQuickInfo(c, node)
			note += running
			if signature != test.signature || (test.note == "" && note != "") || !strings.Contains(note, test.note) {
				t.Fatalf("hover: %q / %q; want %q / %q", signature, note, test.signature, test.note)
			}
		})
	}
}

// A hover on an argument passed by name, before its call is checked,
// resolves the call first, so the call keeps the signature a full check
// infers: after the hover on the first topic, Effect.timeout's program, the
// second topic is the timed greeting's value, a string, not unknown.
func TestEditorHoverOnByNameArgumentBeforeCheck(t *testing.T) {
	source := "function greeting() { return \"hi\" }\nexport function piped() {\n  return greeting() |> Effect.timeout(%, \"1 second\") |> `${%}!`\n}\n"
	_, file, service, _ := editorProgram(t, source)
	uri := lsconv.FileNameToDocumentURI(file.FileName())
	line := strings.Split(source, "\n")[2]
	first := hoverText(t, service, uri, lsproto.Position{Line: 2, Character: uint32(strings.Index(line, "(%,") + 1)})
	second := hoverText(t, service, uri, lsproto.Position{Line: 2, Character: uint32(strings.Index(line, "${%}") + 2)})
	if !strings.Contains(first, "(topic) %: string!!") || !strings.HasPrefix(second, "(topic) %: stringThe value") {
		t.Fatalf("hovers %q, then %q; want the program string!!, then the value string", first, second)
	}
}

func TestEditorDirectiveCompletions(t *testing.T) {
	for _, source := range []string{"// @vibe", "// @vibelang-diagnostics ", "// @vibelang-diagnostics-next-line EK2004:off EK44"} {
		program, file, _, _ := editorProgram(t, source)
		c, done := program.GetTypeCheckerForFile(t.Context(), file)
		items := vibelangTextCompletions(c, file, len(source))
		done()
		labels := map[string]bool{}
		for _, item := range items {
			labels[item.Label] = true
			if item.Start < 0 || item.Start > len(source) || (item.End != 0 && item.End < item.Start) {
				t.Fatalf("invalid completion replacement: %+v", item)
			}
		}
		if source == "// @vibe" {
			if len(items) != 2 || !labels["@vibelang-diagnostics"] || !labels["@vibelang-diagnostics-next-line"] {
				t.Fatalf("missing directive completions: %+v", items)
			}
			continue
		}
		for _, code := range core.VibeLangAdjustableCodes {
			for _, category := range []string{"error", "warning", "off"} {
				if !labels[code+":"+category] {
					t.Errorf("missing completion %s:%s", code, category)
				}
			}
		}
		if !labels["*:off"] || labels["EK2001:off"] || labels["EK4813:off"] {
			t.Fatalf("completions must preserve fixed diagnostic categories: %v", labels)
		}
	}
}
