package vibelang

import (
	"testing"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
)

func TestNodeTextRecoveredSpans(t *testing.T) {
	file := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: "/test.vibe", PathKey: "/test.vibe"}, "   operand", core.ScriptKindTS)
	tests := []struct {
		name       string
		start, end int
		want       string
	}{
		{"leading trivia", 0, 10, "operand"},
		{"missing operand", 0, 0, ""},
		{"trivia past end", 0, 2, ""},
		{"synthesized", -1, -1, ""},
		{"start past source", 11, 12, ""},
		{"end past source", 3, 11, ""},
		{"reversed", 5, 3, ""},
	}
	if got := nodeText(file, nil); got != "" {
		t.Fatalf("nil node: %q", got)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := ast.NodeFactory{}
			node := factory.NewIdentifier("operand")
			node.Loc = core.NewTextRange(tt.start, tt.end)
			if got := nodeText(file, node); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
