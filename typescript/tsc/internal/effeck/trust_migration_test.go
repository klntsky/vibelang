package effeck

import (
	"slices"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/bundled"
	"github.com/microsoft/TypeScript/tsc/internal/compiler"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/tsoptions"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
	"github.com/microsoft/TypeScript/tsc/internal/vfs/vfstest"
)

// TestTrustedValueEdits checks both documents of the migration, retaining
// the TypeScript implementation and its rest parameters for existing callers.
func TestTrustedValueEdits(t *testing.T) {
	t.Parallel()
	const helper = `export class Bad extends Error {}
/** @effeck trusted
 * @throws {Bad}
 */
export function parse(x: string, ...rest: string[]): number { return x.length + rest.length }
`
	const caller = `import { parse as read } from "./helpers"
export function main() { return read("x", "y") }
`
	fs := bundled.WrapFS(vfstest.FromMap(map[string]string{
		"/helpers.ts":    helper,
		"/main.effeck":   caller,
		"/tsconfig.json": `{"compilerOptions":{"strict":true},"files":["helpers.ts","main.effeck"]}`,
	}, tspath.CaseInsensitive))
	parsed, errors := tsoptions.GetParsedCommandLineOfConfigFile("/tsconfig.json", &core.CompilerOptions{}, nil, fs, nil)
	if len(errors) != 0 {
		t.Fatalf("config: %v", errors)
	}
	p := compiler.NewProgram(compiler.ProgramOptions{Config: parsed, Host: compiler.NewCompilerHost(fs, bundled.LibPath(), nil, nil, nil)})
	p.BindSourceFiles()
	file := p.GetSourceFile("/main.effeck")
	var call *ast.Node
	var visit func(*ast.Node) bool
	visit = func(n *ast.Node) bool {
		if ast.IsCallExpression(n) && ast.IsIdentifier(n.Expression()) && n.Expression().Text() == "read" {
			call = n
			return true
		}
		return n.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
	c, done := p.GetTypeChecker(t.Context())
	defer done()
	plan := trustedValueEdits(c, file, call)
	if plan == nil {
		t.Fatal("no migration for imported tagged function")
	}
	apply := func(text string, edits []trustedValueEdit) string {
		edits = slices.Clone(edits)
		slices.SortFunc(edits, func(a, b trustedValueEdit) int { return b.start - a.start })
		for _, edit := range edits {
			text = text[:edit.start] + edit.text + text[edit.end:]
		}
		return text
	}
	wantHelper := "\nimport { trusted } from \"effeck-runtime\";\n" + strings.TrimSuffix(helper, "\n") + "\nexport const parseTrusted = trusted(parse, [Bad]);\n\n"
	if got := apply(helper, plan.declaration); got != wantHelper {
		t.Errorf("helper:\n%s\nwant:\n%s", got, wantHelper)
	}
	wantCaller := "import { parse as read, parseTrusted } from \"./helpers\"\nexport function main() { return parseTrusted(\"x\", \"y\") }\n"
	if got := apply(caller, plan.caller); got != wantCaller {
		t.Errorf("caller:\n%s\nwant:\n%s", got, wantCaller)
	}
}
