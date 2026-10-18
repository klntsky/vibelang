package vibelang

import (
	"fmt"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/bundled"
	"github.com/microsoft/TypeScript/tsc/internal/compiler"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/tsoptions"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
	"github.com/microsoft/TypeScript/tsc/internal/vfs/vfstest"
)

// TestPlainFunctionStopsAtItsOwnBody checks that deciding whether a function
// compiles to plain JavaScript does not explore its callees when its own body
// already rules it out. Each unannotated function of a chain throws outside a try and
// calls the previous one; asking about the last must decide only the last.
// Exploring the callees anyway made the language server's first hover in a
// module analyze every function the call graph reached.
func TestPlainFunctionStopsAtItsOwnBody(t *testing.T) {
	t.Parallel()
	const count = 50
	var source strings.Builder
	for i := range count {
		result := "x"
		if i > 0 {
			result = fmt.Sprintf("f%d(x - 1)", i-1)
		}
		fmt.Fprintf(&source, "export function f%d(x: number) {\n  if (x < 0) throw new Error(\"negative\")\n  return %s\n}\n", i, result)
	}
	fs := bundled.WrapFS(vfstest.FromMap(map[string]string{
		"/chain.vibe":    source.String(),
		"/tsconfig.json": `{ "compilerOptions": { "strict": true }, "files": ["chain.vibe"] }`,
	}, tspath.CaseInsensitive))
	parsed, errors := tsoptions.GetParsedCommandLineOfConfigFile("/tsconfig.json", &core.CompilerOptions{}, nil, fs, nil)
	if len(errors) != 0 {
		t.Fatalf("tsconfig: %v", errors)
	}
	p := compiler.NewProgram(compiler.ProgramOptions{Config: parsed, Host: compiler.NewCompilerHost(fs, bundled.LibPath(), nil, nil, nil)})
	p.BindSourceFiles()
	c, done := p.GetTypeChecker(t.Context())
	defer done()
	last := p.GetSourceFile("/chain.vibe").Statements.Nodes[count-1]
	a := analyzerFor(c)
	if a.isPlainFunction(last) {
		t.Fatalf("f%d throws outside a try, so it cannot compile to plain JavaScript", count-1)
	}
	if len(a.plain) != 1 {
		t.Errorf("deciding f%d decided %d functions; its own throw rules it out, so only it needs a verdict", count-1, len(a.plain))
	}
}

// D263 includes plain contracts supplied by an interface or object context.
// Throws and explicitly plain callback parameters must not infer an Effect.
func TestInferredPlainMethodContract(t *testing.T) {
	t.Parallel()
	fs := bundled.WrapFS(vfstest.FromMap(map[string]string{
		"/main.vibe": `interface Plain { call(build: (payload: string) => string): string }
export class Impl implements Plain {
  call(build: (payload: string) => string) {
    if (!build) throw new Error("empty")
    return build("x")
  }
}
export const object: Plain = {
  call(build) {
    if (!build) throw new Error("empty")
    return build("x")
  }
}
`,
		"/tsconfig.json": `{ "compilerOptions": { "strict": true }, "files": ["main.vibe"] }`,
	}, tspath.CaseInsensitive))
	parsed, errors := tsoptions.GetParsedCommandLineOfConfigFile("/tsconfig.json", &core.CompilerOptions{}, nil, fs, nil)
	if len(errors) != 0 {
		t.Fatalf("tsconfig: %v", errors)
	}
	p := compiler.NewProgram(compiler.ProgramOptions{Config: parsed, Host: compiler.NewCompilerHost(fs, bundled.LibPath(), nil, nil, nil)})
	for _, d := range p.GetSemanticDiagnostics(t.Context(), p.GetSourceFile("/main.vibe")) {
		t.Errorf("plain contract diagnostic: %s", d.String())
	}
}
