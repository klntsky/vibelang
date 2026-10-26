package vibelang

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

// rewriteRetiredEffectTypes writes every retired ! form of source as EK1014's
// fix writes it, outermost first, from the parse alone.
func rewriteRetiredEffectTypes(t *testing.T, source string) string {
	t.Helper()
	name := tspath.RootedFilePath("/main.vibe")
	file := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: name, PathKey: tspath.CaseSensitive.PathKey(tspath.RootedPath(name))}, source, core.ScriptKindTS)
	var edits []sourceEdit
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if isRetiredEffectType(node) {
			span := retiredEffectTypeSpan(file, node)
			edits = append(edits, sourceEdit{span.Pos(), span.End(), retiredEffectTypeText(file, node, "Effect")})
			return false
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
	slices.SortFunc(edits, func(a, b sourceEdit) int { return a.start - b.start })
	return applyEdits(source, edits)
}

// D308: EK1014's fix writes the Effect type each retired form stood for.
func TestRetiredEffectTypeText(t *testing.T) {
	for _, test := range []struct{ name, source, want string }{
		{"every slot", "type T = User!NotFound!Db", "type T = Effect<User, NotFound, Db>"},
		{"empty dependencies", "type T = User!NotFound!", "type T = Effect<User, NotFound>"},
		{"empty errors", "type T = User!!Db", "type T = Effect<User, never, Db>"},
		{"empty slots", "type T = User!!", "type T = Effect<User>"},
		{"dependencies left out without a body", "type T = User!NotFound", "type T = Effect<User, NotFound>"},
		{"dependencies left out with a body", "function f(): User!NotFound { return g() }", "function f(): Effect<User, NotFound, infer R> { return g() }"},
		{"errors and dependencies left out with a body", "function f(): User! { return g() }", "function f(): Effect<User, never, infer R> { return g() }"},
		{"an arrow's body", "const f = (): User!NotFound => g()", "const f = (): Effect<User, NotFound, infer R> => g()"},
		{"a member without a body", "service S { find(id: string): User!NotFound }", "service S { find(id: string): Effect<User, NotFound> }"},
		{"a method with a body", "const s = { find(id: string): User!NotFound { return g(id) } }", "const s = { find(id: string): Effect<User, NotFound, infer R> { return g(id) } }"},
		{"a union in a slot", "type T = User!NotFound | Timeout!Db", "type T = Effect<User, NotFound | Timeout, Db>"},
		{"written never", "type T = User!never!never", "type T = Effect<User>"},
		{"a written infer", "function f(): User!infer E!Db { return g() }", "function f(): Effect<User, infer E, Db> { return g() }"},
		{"a function slot", "type T = (() => void)!E!R", "type T = Effect<() => void, E, R>"},
		{"an object slot", "type T = A!({ code: number })!", "type T = Effect<A, { code: number }>"},
		{"a parenthesized form", "type T = (number!!) | undefined", "type T = Effect<number> | undefined"},
		{"nested forms", "type T = (number!!)!NotFound!", "type T = Effect<Effect<number>, NotFound>"},
		{"a type argument", "type T = ReadonlyArray<number!!>", "type T = ReadonlyArray<Effect<number>>"},
		{"a generic data type is no Effect type", "data LoadState<A, E> { Idle; Loaded { value: A }; Failed { error: E } }", "data LoadState<A, E> { Idle; Loaded { value: A }; Failed { error: E } }"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := rewriteRetiredEffectTypes(t, test.source); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

// vibelang --fix rewrites retired-effect-types.fail into
// retired-effect-types-fixed, whose first line is its expected result where
// the failing case's is its expected codes.
func TestRetiredEffectTypesCaseFix(t *testing.T) {
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(root, "typescript/vibelang/cases/retired-effect-types.fail")
	result := fixCase(t, copyCompilerFixture(t, fixture, root))
	if len(result.Refused) != 0 {
		t.Fatalf("refused: %v", result.Refused)
	}
	if len(result.Files) != 1 {
		t.Fatalf("fixed %d files, want 1", len(result.Files))
	}
	var got string
	for _, fixed := range result.Files {
		got = fixed.Text
		if fixed.Counts[EK1014.Code] == 0 || len(fixed.Counts) != 1 {
			t.Errorf("fix counts %v, want EK1014 only", fixed.Counts)
		}
	}
	want, err := os.ReadFile(filepath.Join(root, "typescript/vibelang/cases/retired-effect-types-fixed/main.vibe"))
	if err != nil {
		t.Fatal(err)
	}
	_, gotRest, _ := strings.Cut(got, "\n")
	_, wantRest, _ := strings.Cut(string(want), "\n")
	if gotRest != wantRest {
		t.Errorf("--fix writes\n%s\nwant\n%s", gotRest, wantRest)
	}
}
