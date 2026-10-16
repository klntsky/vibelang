package ls

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// QuickInfoHook can change a hover on node. A text that is not "" replaces
// the quick info line, such as `function add(a: number, b: number): number`.
// A paragraph that is not "" is Markdown that the hover shows below it,
// before the documentation. c is the checker for node's file, held for the
// duration of the call.
type QuickInfoHook func(c *checker.Checker, node *ast.Node) (text string, paragraph string)

var quickInfoHooks []QuickInfoHook

// RegisterQuickInfoHook adds hook to every hover. Call it only from the init
// function of an in-tree package that tsc/cmd/tsc imports; registration is
// not synchronized.
func RegisterQuickInfoHook(hook QuickInfoHook) {
	quickInfoHooks = append(quickInfoHooks, hook)
}

// withQuickInfoHooks applies the hooks to a hover on node, whose quick info
// line and documentation TypeScript computed.
func withQuickInfoHooks(c *checker.Checker, node *ast.Node, quickInfo string, documentation string) (string, string) {
	var paragraphs []string
	for _, hook := range quickInfoHooks {
		text, paragraph := hook(c, node)
		if text != "" {
			quickInfo = text
		}
		if paragraph != "" {
			paragraphs = append(paragraphs, paragraph)
		}
	}
	if len(paragraphs) == 0 {
		return quickInfo, documentation
	}
	if documentation != "" {
		paragraphs = append(paragraphs, documentation)
	}
	return quickInfo, strings.Join(paragraphs, "\n\n")
}
