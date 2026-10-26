package parser_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
)

func TestVibeLangBlockDoDiagnostic(t *testing.T) {
	for _, keyword := range []string{"fork", "uninterruptible", "interruptible", "restore"} {
		t.Run(keyword, func(t *testing.T) {
			text := fmt.Sprintf("function f() { %s do { log(\"x\") } }", keyword)
			file := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: "/test.vibe", PathKey: "/test.vibe"}, text, core.ScriptKindTS)
			items := file.Diagnostics()
			if len(items) != 1 || items[0].Source() != "EK" || items[0].Code() != 1003 {
				t.Fatalf("expected one EK1003, got %v", items)
			}
			d := items[0]
			if !strings.Contains(d.MessageText(), keyword+" { ... }") || text[d.Pos():d.End()] != "do" {
				t.Fatalf("wrong message or range: %v", d)
			}
		})
	}
	for _, text := range []string{
		"function f() { const fiber = fork { log(\"x\") } }",
		"function f() { uninterruptible { restore { log(\"x\") } } }",
		"function f() { do { work() } while (false) }",
		"module js { export function f() { do { work() } while (false) } }",
	} {
		file := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: "/test.vibe", PathKey: "/test.vibe"}, text, core.ScriptKindTS)
		if len(file.Diagnostics()) != 0 {
			t.Fatalf("valid code rejected: %v\n%s", file.Diagnostics(), text)
		}
	}
}
