package format

import (
	"context"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/ls/lsutil"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
)

// This golden is also the dprint output for the same source with indentWidth 2.
func TestFormatVibeLangMatch(t *testing.T) {
	source := "error Timeout\nexport function h(e: Timeout) {\nreturn match (e) {\nwhen Timeout: \"same\";\n}\n}\n"
	want := "error Timeout\nexport function h(e: Timeout) {\n  return match (e) {\n    when Timeout: \"same\";\n  }\n}\n"
	options := lsutil.GetDefaultFormatCodeSettings()
	options.IndentSize = 2
	options.TabSize = 2
	options.ConvertTabsToSpaces = core.TSTrue
	ctx := WithFormatCodeSettings(context.Background(), options, "\n")
	file := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: "/match.vibe", PathKey: "/match.vibe"}, source, core.ScriptKindTS)
	got := core.ApplyBulkEdits(source, FormatDocument(ctx, file))
	if got != want {
		t.Fatalf("formatting differs\nwant:\n%s\ngot:\n%s", want, got)
	}
	file = parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: "/match.vibe", PathKey: "/match.vibe"}, got, core.ScriptKindTS)
	if changes := FormatDocument(ctx, file); len(changes) != 0 {
		t.Fatalf("formatting is not idempotent: %v", changes)
	}
}
