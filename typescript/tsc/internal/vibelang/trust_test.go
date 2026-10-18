package vibelang

import (
	"fmt"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/bundled"
	"github.com/microsoft/TypeScript/tsc/internal/compiler"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
	"github.com/microsoft/TypeScript/tsc/internal/tsoptions"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
	"github.com/microsoft/TypeScript/tsc/internal/vfs/vfstest"
)

// TestTrustedMigrationWarnsAtChangedCalls keeps calls whose lowering does not
// use a failure contract quiet, while covering Effect and caught D54 calls.
func TestTrustedMigrationWarnsAtChangedCalls(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"/node_modules/effect/package.json": `{"name":"effect","types":"index.d.ts"}`,
		"/node_modules/effect/index.d.ts":   `export * as Effect from "./Effect"`,
		"/node_modules/effect/Effect.d.ts":  `export declare function sync<A>(thunk: () => A): A`,
		"/helpers.ts": `export class Bad extends Error {}
/** @vibelang trusted
 * @throws {Bad}
 */
export function parse(x: string): number { return Number(x) }
/** @vibelang trusted */
export const clean = (x: number) => x
export function plain() { return 1 }
export class Ports {
 /** @vibelang trusted
  * @throws {Bad}
  */
 offset(): number { return 1 }
}
`,
		"/main.vibe": `import { parse, clean, plain, Ports } from "./helpers"
import { Effect } from "effect"
const top = parse("1")
const callback = ["1"].map(text => parse(text))
export function boundary() { return Effect.sync(() => parse("boundary")) }
export function f() { return parse("2") + clean(3) }
export function g() { return JSON.parse("1") }
export function method(ports: Ports) { return ports.offset() }
export function mapped(values: ReadonlyArray<string>) {
  return values.map((v) => parse(v))
}
export function withoutThrows() { return clean(3) }
export function recovered(text: string): number {
  try { return parse(text) } catch { return 0 }
}
`,
		"/tsconfig.json": `{ "compilerOptions": { "strict": true }, "files": ["main.vibe", "helpers.ts"] }`,
	}
	for _, setting := range []struct {
		name     string
		category diagnostics.Category
	}{
		{"warning", diagnostics.CategoryWarning}, {"error", diagnostics.CategoryError},
	} {
		files["/tsconfig.json"] = fmt.Sprintf(`{"compilerOptions":{"strict":true,"vibelang":{"diagnostics":{"EK4406":%q}}},"files":["main.vibe","helpers.ts"]}`, setting.name)
		fs := bundled.WrapFS(vfstest.FromMap(files, tspath.CaseInsensitive))
		parsed, errors := tsoptions.GetParsedCommandLineOfConfigFile("/tsconfig.json", &core.CompilerOptions{}, nil, fs, nil)
		if len(errors) != 0 {
			t.Fatalf("tsconfig: %v", errors)
		}
		p := compiler.NewProgram(compiler.ProgramOptions{Config: parsed, Host: compiler.NewCompilerHost(fs, bundled.LibPath(), nil, nil, nil)})
		var positions []int
		for _, d := range p.GetSemanticDiagnostics(t.Context(), p.GetSourceFile("/main.vibe")) {
			if d.Code() == EK4406.Code {
				if d.Category() != setting.category {
					t.Errorf("migration category = %v", d.Category())
				}
				text := p.GetSourceFile("/main.vibe").Text()
				allowed := []int{
					strings.Index(text, `parse("2")`),
					strings.Index(text, "ports.offset()"),
					strings.Index(text, "parse(text) } catch"),
				}
				if d.Pos() != allowed[0] && d.Pos() != allowed[1] && d.Pos() != allowed[2] {
					t.Errorf("migration diagnostic for a call whose lowering does not use the tag at %d", d.Pos())
				}
				message := d.MessageText()
				if strings.Contains(text[d.Pos():d.End()], "offset") {
					if !strings.Contains(message, "Ports.offset") || !strings.Contains(message, "write an adapter") || !strings.Contains(message, "[Bad]") {
						t.Errorf("method migration message: %s", message)
					}
				} else if strings.Contains(message, "[Errors]") {
					t.Errorf("missing concrete contract: %s", message)
				}
				positions = append(positions, d.Pos())
			}
		}
		if len(positions) != 3 {
			t.Fatalf("migration warnings at %v; want only the Effect calls and caught D54 call whose tags affect emit", positions)
		}
	}

}

// D261 grants trust by package.json name, including TypeScript workspace
// sources reached through paths, and preserves the boundary without a match.
func TestTrustedWorkspacePackageNames(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, patterns string
		want           int
	}{
		{"scope glob", `["@smthrs/*"]`, 0},
		{"exact name", `["@smthrs/keys"]`, 0},
		{"another scope", `["@other/*"]`, 1},
		{"prefix is not a match", `["@smthrs/key"]`, 1},
		{"no trust", `[]`, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := map[string]string{
				"/workspace/keys/package.json": `{"name":"@smthrs/keys"}`,
				"/workspace/keys/index.ts":     `export { key } from "../../foreign"`,
				"/foreign.ts":                  `export function key() { return 42 }`,
				"/main.vibe":                   `import { key } from "@smthrs/keys"; export function use() { return key() }`,
				"/tsconfig.json":               fmt.Sprintf(`{"compilerOptions":{"strict":true,"paths":{"@smthrs/keys":["./workspace/keys/index.ts"]},"vibelang":{"trustedPackages":%s}},"files":["main.vibe"]}`, test.patterns),
			}
			fs := bundled.WrapFS(vfstest.FromMap(files, tspath.CaseSensitive))
			parsed, errors := tsoptions.GetParsedCommandLineOfConfigFile("/tsconfig.json", &core.CompilerOptions{}, nil, fs, nil)
			if len(errors) != 0 || len(parsed.Errors) != 0 {
				t.Fatalf("config: %v / %v", errors, parsed.Errors)
			}
			p := compiler.NewProgram(compiler.ProgramOptions{Config: parsed, Host: compiler.NewCompilerHost(fs, bundled.LibPath(), nil, nil, nil)})
			count := 0
			for _, d := range p.GetSemanticDiagnostics(t.Context(), p.GetSourceFile("/main.vibe")) {
				if d.Code() == EK4401.Code {
					count++
				} else if d.Category() == diagnostics.CategoryError {
					t.Errorf("unexpected diagnostic: %v", d)
				}
			}
			if count != test.want {
				t.Fatalf("EK4401 count = %d, want %d", count, test.want)
			}
		})
	}
}
