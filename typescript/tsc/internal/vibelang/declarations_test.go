package vibelang

import (
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/bundled"
	"github.com/microsoft/TypeScript/tsc/internal/compiler"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
	"github.com/microsoft/TypeScript/tsc/internal/tsoptions"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
	"github.com/microsoft/TypeScript/tsc/internal/vfs"
	"github.com/microsoft/TypeScript/tsc/internal/vfs/vfstest"
)

// TestDeclarationImportsComputedKey checks that a .d.ts which writes a
// computed key the source took from the prelude or effect's root imports the
// key's name, and that a skipLibCheck=false consumer accepts the .d.ts. The
// checker prints such a key by its bare name, [customMatcher], because no
// scope it searches declares the key, and records no import for it.
func TestDeclarationImportsComputedKey(t *testing.T) {
	t.Parallel()
	even := `{ [customMatcher](subject: unknown) { return typeof subject === "number" && subject % 2 === 0 } }`
	for _, test := range []struct {
		name, source string
		want, avoid  []string
	}{
		{
			name:   "exported object",
			source: `export const Even = ` + even,
			want:   []string{`import type { customMatcher } from "vibelang-runtime";`, `[customMatcher](subject: unknown): boolean;`},
		},
		{
			// The parity harness compares unexported bindings through a type alias.
			name:   "unexported object a type names",
			source: `const Even = ` + even + `; export type Probe = typeof Even`,
			want:   []string{`import type { customMatcher } from "vibelang-runtime";`, `[customMatcher](subject: unknown): boolean;`},
		},
		{
			name:   "object inside another",
			source: `const Even = ` + even + `; export const matchers = { Even }`,
			want:   []string{`import type { customMatcher } from "vibelang-runtime";`, `[customMatcher](subject: unknown): boolean;`},
		},
		{
			// customMatcher comes from the runtime package, which linked
			// workspaces and the smoke and parity harnesses reach by symlink.
			name:   "symlinked runtime",
			source: `export const Even = ` + even,
			want:   []string{`import type { customMatcher } from "vibelang-runtime";`, `[customMatcher](subject: unknown): boolean;`},
		},
		{
			name:   "effect root key",
			source: `export const Shown = { [Inspectable.NodeInspectSymbol]() { return "shown" } }`,
			want:   []string{`import type { Inspectable } from "effect";`, `[Inspectable.NodeInspectSymbol](): string;`},
		},
		{
			name:   "beside a well-known symbol key",
			source: `export const Both = { [Symbol.toPrimitive]() { return 1 }, [customMatcher](subject: unknown) { return subject === 2 } }`,
			want:   []string{`import type { customMatcher } from "vibelang-runtime";`, `[Symbol.toPrimitive](): number;`, `[customMatcher](subject: unknown): subject is 2;`},
		},
		{
			name:   "class key",
			source: `export class Odd { static [customMatcher](subject: unknown) { return subject === 1 } }`,
			want:   []string{`import type { customMatcher } from "vibelang-runtime";`, `static [customMatcher](subject: unknown)`},
		},
		{
			name:   "imported key",
			source: `import { customMatcher } from "vibelang-runtime"; export const Even = ` + even,
			want:   []string{`import { customMatcher } from "vibelang-runtime";`},
			avoid:  []string{`import type { customMatcher }`},
		},
		{
			name:   "key the declaration file does not write",
			source: `const Even = ` + even + `; export const n = 1`,
			avoid:  []string{`customMatcher`},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			files := computedKeyFiles(strings.HasPrefix(test.name, "symlinked "))
			files["/Users/x/src/main.vibe"] = test.source
			dts := emitDeclarations(t, files)
			// A consumer that checks libraries must resolve every name the
			// declaration writes.
			files["/Users/x/src/main.d.ts"] = dts
			delete(files, "/Users/x/src/main.vibe")
			files["/Users/x/src/consumer.ts"] = `import * as value from "./main.js"; export { value }`
			files["/Users/x/src/tsconfig.json"] = `{"compilerOptions":{"strict":true,"skipLibCheck":false,"module":"NodeNext","noEmit":true},"files":["consumer.ts"]}`
			p := computedKeyProgram(t, files)
			for _, d := range p.GetSemanticDiagnostics(t.Context(), nil) {
				if d.Category() == diagnostics.CategoryError {
					t.Fatalf("declaration consumer: %s\n%s", d.String(), dts)
				}
			}
			for _, want := range test.want {
				if !strings.Contains(dts, want) {
					t.Fatalf("declaration lacks %q:\n%s", want, dts)
				}
			}
			for _, avoid := range test.avoid {
				if strings.Contains(dts, avoid) {
					t.Fatalf("declaration has %q:\n%s", avoid, dts)
				}
			}
		})
	}
}

// computedKeyFiles is a package with effect and vibelang-runtime installed, or
// linked from outside every ancestor node_modules when symlinked is true.
func computedKeyFiles(symlinked bool) map[string]string {
	effect := "/Users/x/src/node_modules/effect/"
	runtime := "/Users/x/src/node_modules/vibelang-runtime/"
	if symlinked {
		effect = "/external/experiments/node_modules/effect/"
		runtime = "/external/runtime/"
	}
	return map[string]string{
		"/Users/x/src/package.json":  `{"type":"module"}`,
		"/Users/x/src/tsconfig.json": `{"compilerOptions":{"strict":true,"target":"ESNext","module":"NodeNext","declaration":true},"files":["main.vibe"]}`,
		effect + "package.json":      `{"name":"effect","type":"module","exports":{".":{"types":"./index.d.ts"},"./Effect":{"types":"./Effect.d.ts"},"./Inspectable":{"types":"./Inspectable.d.ts"}}}`,
		effect + "index.d.ts":        `export * as Effect from "./Effect.js"; export * as Inspectable from "./Inspectable.js"`,
		effect + "Effect.d.ts":       `export interface Effect<A, E = never, R = never> { readonly "~effect/Effect": { readonly _A: (_: never) => A; readonly _E: (_: never) => E; readonly _R: (_: never) => R } }`,
		effect + "Inspectable.d.ts":  `export declare const NodeInspectSymbol: unique symbol`,
		runtime + "package.json":     `{"name":"vibelang-runtime","type":"module","exports":{".":{"types":"./index.d.ts"}}}`,
		runtime + "index.d.ts":       `export * as Effect from "effect/Effect"; export * from "./match.js"`,
		runtime + "match.d.ts":       `export declare const customMatcher: unique symbol`,
	}
}

// computedKeyProgram builds the program of /Users/x/src/tsconfig.json, linking
// the packages into node_modules when they live outside it.
func computedKeyProgram(t *testing.T, files map[string]string) *compiler.Program {
	t.Helper()
	entries := make(map[string]any, len(files)+3)
	for name, text := range files {
		entries[name] = text
	}
	if _, linked := files["/external/runtime/index.d.ts"]; linked {
		entries["/Users/x/src/node_modules/effect"] = vfstest.Symlink("/external/experiments/node_modules/effect")
		entries["/Users/x/src/node_modules/vibelang-runtime"] = vfstest.Symlink("/external/runtime")
		entries["/external/runtime/node_modules/effect"] = vfstest.Symlink("/external/experiments/node_modules/effect")
	}
	var fs vfs.FS = bundled.WrapFS(vfstest.FromMap(entries, tspath.CaseInsensitive))
	parsed, errors := tsoptions.GetParsedCommandLineOfConfigFile("/Users/x/src/tsconfig.json", &core.CompilerOptions{}, nil, fs, nil)
	if len(errors) != 0 {
		t.Fatalf("config: %v", errors)
	}
	return compiler.NewProgram(compiler.ProgramOptions{Config: parsed, Host: compiler.NewCompilerHost(fs, bundled.LibPath(), nil, nil, nil)})
}

// emitDeclarations returns the main.d.ts that the program of files emits,
// failing on any error diagnostic.
func emitDeclarations(t *testing.T, files map[string]string) string {
	t.Helper()
	p := computedKeyProgram(t, files)
	for _, d := range p.GetSemanticDiagnostics(t.Context(), nil) {
		if d.Category() == diagnostics.CategoryError {
			t.Fatalf("source: %s", d.String())
		}
	}
	var dts string
	result := p.Emit(t.Context(), compiler.EmitOptions{WriteFile: func(name tspath.RootedFilePath, text string, _ *compiler.WriteFileData) error {
		if strings.HasSuffix(string(name), "main.d.ts") {
			dts = text
		}
		return nil
	}})
	for _, d := range result.Diagnostics {
		if d.Category() == diagnostics.CategoryError {
			t.Fatalf("emit: %s", d.String())
		}
	}
	if dts == "" {
		t.Fatal("no main.d.ts emitted")
	}
	return dts
}
