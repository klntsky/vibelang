package vibelang

import (
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
	"github.com/microsoft/TypeScript/tsc/internal/ls/lsconv"
	"github.com/microsoft/TypeScript/tsc/internal/lsp/lsproto"
)

// D33 and D240 promise mechanical fixes that leave usable source. Request the
// public LSP action, compare all edits, and type-check the resulting document.
func TestEditorQuickFixGoldens(t *testing.T) {
	for _, test := range []struct {
		name, source, title, want string
		code                      int32
	}{
		{"run value", "function main(job: Effect.Effect<number>) { job }", "Call it to run it", "function main(job: Effect.Effect<number>) { job() }", 4801},
		{"run conditional", "function main(flag: boolean, a: Effect.Effect<number>, b: Effect.Effect<number>) { flag ? a : b }", "Call it to run it", "function main(flag: boolean, a: Effect.Effect<number>, b: Effect.Effect<number>) { (flag ? a : b)() }", 4801},
		{"return value", "function main(job: Effect.Effect<number>) { Effect.succeed(0); return job }", "Call it to run it", "function main(job: Effect.Effect<number>) { Effect.succeed(0); return job() }", 4802},
		{"return conditional", "function main(flag: boolean, a: Effect.Effect<number>, b: Effect.Effect<number>) { Effect.succeed(0); return flag ? a : b }", "Call it to run it", "function main(flag: boolean, a: Effect.Effect<number>, b: Effect.Effect<number>) { Effect.succeed(0); return (flag ? a : b)() }", 4802},
		{"return conditional arm", "function main(flag: boolean, job: Effect.Effect<number>) { return flag ? job : Effect.succeed(1) }", "Call it to run it", "function main(flag: boolean, job: Effect.Effect<number>) { return flag ? job() : Effect.succeed(1) }", 4802},
		{"return value as unknown", "function main(job: Effect.Effect<number>): unknown!! { return job }", "Call it to run it", "function main(job: Effect.Effect<number>): unknown!! { return job() }", 4802},
		{"return program by name as unknown", "function main(job: () => number!!): unknown!! { return job }", "Call it to run it", "function main(job: () => number!!): unknown!! { return job() }", 4802},
		{"return coalesced operand", "function main(n: number | undefined, job: Effect.Effect<number>) { Effect.succeed(0); return n ?? job }", "Call it to run it", "function main(n: number | undefined, job: Effect.Effect<number>) { Effect.succeed(0); return n ?? job() }", 4802},
		{"retain failure contract", `function main(job: Effect.Effect<number, string>) { const narrow = job as Effect.Effect<number>; return narrow() }`, "Remove the assertion", `function main(job: Effect.Effect<number, string>) { const narrow = job; return narrow() }`, 4810},
		{"Effect 4 name", `function main() { return Effect.catchAll(Effect.fail("bad"), () => Effect.succeed(1)) }`, "Write Effect.catch", `function main() { return Effect.catch(Effect.fail("bad"), () => Effect.succeed(1)) }`, 4813},
		{"empty pipe", "// @vibelang-diagnostics EK4824:warning\nexport const value = pipe(1)", "Remove the pipe", "// @vibelang-diagnostics EK4824:warning\nexport const value = 1", 4824},
		{"arrow expression", "// @vibelang-diagnostics EK4823:warning\nexport const add = (x: number) => { return x + 1 }", "Write the value as the body", "// @vibelang-diagnostics EK4823:warning\nexport const add = (x: number) => x + 1", 4823},
		{"arrow object", "// @vibelang-diagnostics EK4823:warning\nexport const box = (x: number) => { return { value: x } }", "Write the value as the body", "// @vibelang-diagnostics EK4823:warning\nexport const box = (x: number) => ({ value: x })", 4823},
		{"tagged schema", `// @vibelang-diagnostics EK4821:warning
export const Person = Schema.Struct({ _tag: Schema.Literal("Person"), name: Schema.String })`, "Write Schema.TaggedStruct", `// @vibelang-diagnostics EK4821:warning
export const Person = Schema.TaggedStruct("Person", { name: Schema.String })`, 4821},
	} {
		t.Run(test.name, func(t *testing.T) {
			const prefix = "// café 😀\n"
			program, file, service, converters := editorProgram(t, prefix+test.source)
			var diagnostic *ast.Diagnostic
			for _, d := range program.GetSemanticDiagnostics(t.Context(), file) {
				if d.Code() == test.code {
					diagnostic = d
					break
				}
			}
			if diagnostic == nil {
				t.Fatalf("missing EK%d in %s", test.code, test.source)
			}
			d := lsconv.DiagnosticToLSPPush(t.Context(), converters, diagnostic)
			only := []lsproto.CodeActionKind{lsproto.CodeActionKindQuickFix}
			response, err := service.ProvideCodeActions(t.Context(), &lsproto.CodeActionParams{
				TextDocument: lsproto.TextDocumentIdentifier{Uri: lsconv.FileNameToDocumentURI(file.FileName())},
				Range:        d.Range, Context: &lsproto.CodeActionContext{Diagnostics: []*lsproto.Diagnostic{d}, Only: &only},
			})
			if err != nil {
				t.Fatal(err)
			}
			var chosen *lsproto.CodeAction
			if response.CommandOrCodeActionArray != nil {
				for _, action := range *response.CommandOrCodeActionArray {
					if action.CodeAction != nil && action.CodeAction.Title == test.title {
						chosen = action.CodeAction
					}
				}
			}
			if chosen == nil || chosen.Edit == nil || chosen.Edit.Changes == nil {
				t.Fatalf("missing quick fix %q", test.title)
			}
			changes := *chosen.Edit.Changes
			if len(changes) != 1 {
				t.Fatalf("fix changed unrelated documents: %v", changes)
			}
			edits := changes[lsconv.FileNameToDocumentURI(file.FileName())]
			got := applyEditorEdits(t, file, converters, edits)
			if got != prefix+test.want {
				t.Fatalf("edited source differs:\ngot: %s\nwant: %s", got, prefix+test.want)
			}
			checked, edited, _, _ := editorProgram(t, got)
			all := append(checked.GetSyntacticDiagnostics(t.Context(), edited), checked.GetSemanticDiagnostics(t.Context(), edited)...)
			for _, after := range all {
				if after.Category() == diagnostics.CategoryError || after.Code() == test.code {
					t.Errorf("quick fix left a diagnostic: %s", after.String())
				}
			}
			if strings.Contains(got, "()()") {
				t.Fatal("quick fix calls the result twice")
			}
		})
	}
}

// TestEditorQuickFixSkipsUncallableUnion checks that EK4802's call-it fix is
// not offered for a returned union with a member that is no program: calling
// `chosen` where it may be undefined would not check.
func TestEditorQuickFixSkipsUncallableUnion(t *testing.T) {
	source := "export function main(job: Effect.Effect<number> | undefined): unknown!! { const chosen = job; return chosen }"
	program, file, service, converters := editorProgram(t, source)
	var diagnostic *ast.Diagnostic
	for _, d := range program.GetSemanticDiagnostics(t.Context(), file) {
		if d.Code() == 4802 {
			diagnostic = d
			break
		}
	}
	if diagnostic == nil {
		t.Fatalf("missing EK4802 in %s", source)
	}
	d := lsconv.DiagnosticToLSPPush(t.Context(), converters, diagnostic)
	only := []lsproto.CodeActionKind{lsproto.CodeActionKindQuickFix}
	response, err := service.ProvideCodeActions(t.Context(), &lsproto.CodeActionParams{
		TextDocument: lsproto.TextDocumentIdentifier{Uri: lsconv.FileNameToDocumentURI(file.FileName())},
		Range:        d.Range, Context: &lsproto.CodeActionContext{Diagnostics: []*lsproto.Diagnostic{d}, Only: &only},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.CommandOrCodeActionArray != nil {
		for _, action := range *response.CommandOrCodeActionArray {
			if action.CodeAction != nil && action.CodeAction.Title == "Call it to run it" {
				t.Fatal("offered Call it to run it for a union with undefined")
			}
		}
	}
}
