package ls_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/bundled"
	"github.com/microsoft/TypeScript/tsc/internal/compiler"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
	"github.com/microsoft/TypeScript/tsc/internal/effeckmapper"
	"github.com/microsoft/TypeScript/tsc/internal/ls"
	"github.com/microsoft/TypeScript/tsc/internal/ls/lsconv"
	"github.com/microsoft/TypeScript/tsc/internal/lsp/lsproto"
	"github.com/microsoft/TypeScript/tsc/internal/tsoptions"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
	"github.com/microsoft/TypeScript/tsc/internal/vfs"
	"github.com/microsoft/TypeScript/tsc/internal/vfs/vfstest"
)

// inferredActions exercises the public LS transaction and records JavaScript
// from the same program so an annotation cannot silently change lowering.
func inferredActions(t *testing.T, files map[string]string) ([]*lsproto.CodeAction, inferredEmit) {
	t.Helper()
	fs := inferredFileSystem(files)
	parsed, errors := tsoptions.GetParsedCommandLineOfConfigFile("/Users/x/src/tsconfig.json", &core.CompilerOptions{}, nil, fs, nil)
	if len(errors) != 0 {
		t.Fatalf("config: %v", errors)
	}
	p := compiler.NewProgram(compiler.ProgramOptions{Config: parsed, Host: compiler.NewCompilerHost(fs, bundled.LibPath(), nil, nil, nil)})
	converters := lsconv.NewConverters(lsproto.PositionEncodingKindUTF16, func(name tspath.RootedFilePath) *lsconv.LSPLineMap {
		return lsconv.ComputeLSPLineStarts(files[string(name)])
	})
	service := ls.NewLanguageService(nil, p, trustedFixHost{converters: converters}, "/Users/x/src/main.effeck")
	file := p.GetSourceFile("/Users/x/src/main.effeck")
	var actions []*lsproto.CodeAction
	// Compiler collections must stay free of the editor-only resolver, even
	// when the editor warning is configured as an error.
	for _, d := range append(p.GetSemanticDiagnostics(t.Context(), file), p.GetSuggestionDiagnostics(t.Context(), file)...) {
		if d.Code() == 2345 {
			t.Fatalf("callback form changed: %s", d.MessageText())
		}
		if d.Code() == 9100 {
			t.Fatal("EK9100 ran in the compiler")
		}
	}
	editor, err := service.ProvideDiagnostics(t.Context(), lsconv.FileNameToDocumentURI(file.FileName()))
	if err != nil {
		t.Fatal(err)
	}
	forkChoices := 0
	for _, diag := range editor.FullDocumentDiagnosticReport.Items {
		if diag.Code == nil || diag.Code.Integer == nil || *diag.Code.Integer != 9100 {
			continue
		}
		forkChoices++
		if diag.Message.String != nil && strings.HasPrefix(*diag.Message.String, "The mapper cannot decide this await element") && !strings.Contains(*diag.Message.String, "The fork infers the success type ") {
			t.Fatal("await choice lacks the inferred type")
		}
		if diag.Severity == nil || *diag.Severity != lsproto.DiagnosticSeverityWarning {
			t.Fatal("EK9100 is not an editor warning")
		}
		only := []lsproto.CodeActionKind{lsproto.CodeActionKindQuickFix}
		response, err := service.ProvideCodeActions(t.Context(), &lsproto.CodeActionParams{TextDocument: lsproto.TextDocumentIdentifier{Uri: lsconv.FileNameToDocumentURI(file.FileName())}, Range: diag.Range, Context: &lsproto.CodeActionContext{Diagnostics: []*lsproto.Diagnostic{diag}, Only: &only}})
		if err != nil {
			t.Fatal(err)
		}
		if response.CommandOrCodeActionArray != nil {
			for _, a := range *response.CommandOrCodeActionArray {
				if a.CodeAction != nil && a.CodeAction.Title == "Annotate as the fork infers" {
					actions = append(actions, a.CodeAction)
				}
			}
		}
	}
	projectorChoices := 0
	for _, d := range effeckmapper.Project(file.Text(), effeckmapper.FileContext{}).Diagnostics {
		if d.Code == 9100 {
			projectorChoices++
		}
	}
	if forkChoices != projectorChoices {
		t.Fatalf("fork choices=%d projector choices=%d", forkChoices, projectorChoices)
	}
	var emitted inferredEmit
	p.Emit(t.Context(), compiler.EmitOptions{WriteFile: func(name tspath.RootedFilePath, text string, _ *compiler.WriteFileData) error {
		if strings.HasSuffix(string(name), "main.js") {
			emitted.JS = text
		}
		if strings.HasSuffix(string(name), "main.d.ts") {
			emitted.DTS = text
		}
		return nil
	}})
	if emitted.JS == "" || emitted.DTS == "" {
		t.Fatal("no JavaScript emitted")
	}
	return actions, emitted
}

// inferredEmit records both public contracts and execution for fix comparisons.
type inferredEmit struct{ JS, DTS string }

// normalizeEffectHelper ignores only the local generated Effect namespace
// binding, which a newly written bang contract can reserve in the parser.
func normalizeEffectHelper(js string) string {
	re := regexp.MustCompile(`import \* as (\w+) from "effect/Effect";`)
	match := re.FindStringSubmatchIndex(js)
	if match == nil {
		return js
	}
	name := js[match[2]:match[3]]
	js = js[:match[2]] + "EffectHelper" + js[match[3]:]
	return regexp.MustCompile(`\b`+name+`\.`).ReplaceAllString(js, "EffectHelper.")
}

// TestInferredFormFix covers source shapes from the documentation census and
// checks both the projector's ambiguity and the fork's executable behavior.
func TestInferredFormFix(t *testing.T) {
	for _, test := range []struct {
		name, source string
		choices      int
	}{
		{"named method contract", `export class C { f(): ReadonlyArray<string>!! { return ["x"] } }`, 0},
		{"program argument", `import { takesProgram } from "./host"; export function run() { return takesProgram(() => { return work(1) }) }`, 1},
		{"program block", `import { takesProgram } from "./host"; export function run() { return takesProgram { return work(1) } }`, 1},
		{"cross-file Effect callback", `import { takesFn } from "./host"; export function run() { return takesFn((x: number) => { const n=work(x); return n }) }`, 1},
		{"symlinked named function", `export function fn(x: number) { const y=work(x); return y }`, 0},
		{"symlinked exported arrow", `export const fn = (x: number) => { const y=work(x); return y }`, 1},
		{"symlinked property", `export const obj = { f: (x: number) => { work(x); return x } }`, 1},
		{"exported arrow", `export const fn = (x: number) => { const y=work(x); return y }`, 1},
		{"block", `declare const host: any; host { work(1); return 1 }`, 1},
		{"failure and requirement", `interface Bad { bad: true }; interface Env { env: true }; declare function fail(): number!Bad!Env; export const obj = { f: () => { const x = fail(); return x } }`, 1},
		{"object method", `export const obj = { f(x: number) { work(x); return x } }`, 1},
		{"function expression", `declare const host: any; host(function(x: number) { work(x); return x })`, 1},
		{"property", `export const obj = { f: (x: number) => { work(x); return x } }`, 1},
		{"untyped argument", `declare const host: any; host((x: number) => { work(x); return x })`, 1},
		{"unparenthesized", `declare const host: any; host(x => { work(x); return x })`, 1},
		{"plain", `import { host } from "./host"; host((x: number) => Math.abs(x))`, 1},
		{"await call element", `import { get } from "./host"; export function main() { return await.all [get(1)] }`, 1},
		{"await collection", `export function main() { return await.all [() => work(1), () => work(2)] }`, 0},
		{"hatch mapper", `export function main() { return sync(() => 1, (error: unknown) => { Math.abs(1); return new Error() }) }`, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := map[string]string{
				"/Users/x/src/tsconfig.json":                            `{"compilerOptions":{"strict":true,"target":"ESNext","declaration":true},"files":["main.effeck","host.ts"]}`,
				"/Users/x/src/host.ts":                                  `import type { Effect } from "effect/Effect"; export declare const host: any; export declare function work(x: number): Effect<number,never,never>; export declare function get(x: number): Effect<number,never,never>; export declare function takesProgram(p: Effect<number,never,never>): Effect<number,never,never>; export declare function takesFn(f:(x:number)=>Effect<number,never,never>): Effect<number,never,never>`,
				"/Users/x/src/main.effeck":                              `import { work } from "./host"; ` + test.source,
				"/Users/x/src/node_modules/effeck-runtime/package.json": `{"name":"effeck-runtime","types":"index.d.ts"}`,
				"/Users/x/src/node_modules/effeck-runtime/index.d.ts":   `export * as Effect from "effect/Effect"; import type { Effect } from "effect/Effect"; export declare function $awaitAll<A,E,R>(xs: readonly Effect<A,E,R>[]): Effect<A[],E,R>; export declare function sync<A>(f:()=>A, mapper?: (error: unknown)=>unknown): A`,
				"/Users/x/src/node_modules/effect/package.json":         `{"name":"effect","types":"index.d.ts"}`,
				"/Users/x/src/node_modules/effect/index.d.ts":           `export * as Effect from "./Effect"`,
				"/Users/x/src/node_modules/effect/Effect.d.ts":          `export interface Effect<A,E=never,R=never> { readonly "~effect/Effect": { readonly _A: (_:never)=>A; readonly _E: (_:never)=>E; readonly _R: (_:never)=>R } }`,
			}
			if strings.HasPrefix(test.name, "symlinked ") {
				// The package's real path is outside every ancestor node_modules,
				// as in linked workspaces and the smoke/docs harnesses.
				for name, text := range files {
					if strings.HasPrefix(name, "/Users/x/src/node_modules/effect/") {
						delete(files, name)
						files[strings.Replace(name, "/Users/x/src/node_modules/effect/", "/external/experiments/node_modules/effect/", 1)] = text
					}
				}
				files["/external/experiments/node_modules/effect/package.json"] = `{"name":"effect","type":"module","exports":{".":{"types":"./index.d.ts"},"./Effect":{"types":"./dist/Effect.d.ts"}}}`
				files["/external/experiments/node_modules/effect/dist/Effect.d.ts"] = files["/external/experiments/node_modules/effect/Effect.d.ts"]
				delete(files, "/external/experiments/node_modules/effect/Effect.d.ts")
				files["/external/experiments/node_modules/effect/index.d.ts"] = `export * as Effect from "./dist/Effect.js"`
				files["/Users/x/src/host.ts"] = `export { work } from "effeck-runtime"`
				for name, text := range files {
					if strings.HasPrefix(name, "/Users/x/src/node_modules/effeck-runtime/") {
						delete(files, name)
						if strings.HasSuffix(name, "/index.d.ts") {
							text += `; export declare function work(x: number): Effect<number,never,never>`
						}
						files[strings.Replace(name, "/Users/x/src/node_modules/effeck-runtime/", "/external/runtime/", 1)] = text
					}
				}
				files["/Users/x/src/package.json"] = `{"type":"module"}`
				files["/Users/x/src/tsconfig.json"] = `{"compilerOptions":{"strict":true,"target":"ESNext","module":"NodeNext","declaration":true},"files":["main.effeck","host.ts"]}`
				files["/Users/x/src/main.effeck"] = strings.ReplaceAll(files["/Users/x/src/main.effeck"], `"./host"`, `"./host.js"`)
			}
			source := files["/Users/x/src/main.effeck"]
			count := func(text string) int {
				n := 0
				for _, d := range effeckmapper.Project(text, effeckmapper.FileContext{}).Diagnostics {
					if d.Code == 9100 {
						n++
					}
				}
				return n
			}
			if n := count(source); n != test.choices {
				t.Fatalf("projector choices=%d want %d", n, test.choices)
			}
			initialActions, before := inferredActions(t, files)
			if test.name == "named method contract" && !strings.Contains(before.DTS, "ReadonlyArray<string>") {
				t.Fatalf("named contract spelling changed: %s", before.DTS)
			}
			if test.name == "await call element" || strings.HasPrefix(test.name, "program ") {
				if len(initialActions) != 0 {
					t.Fatal("await expression received a fix that changes emit")
				}
				return
			}
			for step := 0; step < test.choices; step++ {
				actions, _ := inferredActions(t, files)
				if len(actions) == 0 {
					t.Fatalf("no fix at step %d: %s", step, files["/Users/x/src/main.effeck"])
				}
				applyTrustedAction(t, files, actions[0])
			}
			fixed := files["/Users/x/src/main.effeck"]
			if test.name == "failure and requirement" && !strings.Contains(fixed, ": number!Bad!Env") {
				t.Fatalf("lost inferred channels: %s", fixed)
			}
			if n := count(fixed); n != 0 {
				t.Fatalf("%d projector choices remain: %s", n, fixed)
			}
			actions, after := inferredActions(t, files)
			if len(actions) != 0 {
				t.Fatal("fork still reports choices")
			}
			if normalizeEffectHelper(before.JS) != normalizeEffectHelper(after.JS) {
				t.Fatalf("JavaScript changed: before %s after %s", before.JS, after.JS)
			}
			if strings.HasPrefix(test.name, "symlinked ") && !strings.Contains(after.DTS, `import("effect/Effect"`) {
				t.Fatalf("declaration exposes the symlink target: %s", after.DTS)
			}
			if before.DTS != after.DTS {
				t.Fatalf("declaration changed: before %s after %s", before.DTS, after.DTS)
			}
			// Declarations are consumed with skipLibCheck=false, so a raw
			// prelude namespace cannot silently escape into a public API.
			consumer := map[string]string{}
			for k, v := range files {
				consumer[k] = v
			}
			consumer["/Users/x/src/emitted.d.ts"] = after.DTS
			consumer["/Users/x/src/consumer.ts"] = `import * as value from "./emitted"; export {value}`
			consumer["/Users/x/src/tsconfig.json"] = `{"compilerOptions":{"strict":true,"skipLibCheck":false},"files":["consumer.ts"]}`
			if strings.HasPrefix(test.name, "symlinked ") {
				consumer["/Users/x/src/consumer.ts"] = `import * as value from "./emitted.js"; export {value}`
				consumer["/Users/x/src/tsconfig.json"] = `{"compilerOptions":{"strict":true,"skipLibCheck":false,"module":"NodeNext"},"files":["consumer.ts"]}`
			}
			fs := inferredFileSystem(consumer)
			config, errs := tsoptions.GetParsedCommandLineOfConfigFile("/Users/x/src/tsconfig.json", &core.CompilerOptions{}, nil, fs, nil)
			if len(errs) > 0 {
				t.Fatal(errs)
			}
			p := compiler.NewProgram(compiler.ProgramOptions{Config: config, Host: compiler.NewCompilerHost(fs, bundled.LibPath(), nil, nil, nil)})
			for _, d := range p.GetSemanticDiagnostics(t.Context(), nil) {
				if d.Category() == diagnostics.CategoryError {
					t.Fatalf("declaration consumer: %d %s", d.Code(), d.MessageText())
				}
			}
		})
	}
}

// inferredFileSystem resolves Effect through its external symlink when present,
// so the same LS transaction and declaration consumer cover linked packages.
func inferredFileSystem(files map[string]string) vfs.FS {
	entries := make(map[string]any, len(files)+1)
	for name, text := range files {
		entries[name] = text
	}
	if _, linked := files["/external/experiments/node_modules/effect/package.json"]; linked {
		entries["/Users/x/src/node_modules/effect"] = vfstest.Symlink("/external/experiments/node_modules/effect")
		entries["/Users/x/src/node_modules/effeck-runtime"] = vfstest.Symlink("/external/runtime")
		entries["/external/runtime/node_modules/effect"] = vfstest.Symlink("/external/experiments/node_modules/effect")
	}
	return bundled.WrapFS(vfstest.FromMap(entries, tspath.CaseInsensitive))
}
