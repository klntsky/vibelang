package ls

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// QuickInfoHook returns a Markdown paragraph that a hover on node shows below
// the node's quick info, or "" to add nothing. c is the checker for node's
// file, held for the duration of the call.
type QuickInfoHook func(c *checker.Checker, node *ast.Node) string

var quickInfoHooks []QuickInfoHook

// RegisterQuickInfoHook adds hook to every hover. Call it only from the init
// function of an in-tree package that tsc/cmd/tsc imports; registration is
// not synchronized.
func RegisterQuickInfoHook(hook QuickInfoHook) {
	quickInfoHooks = append(quickInfoHooks, hook)
}

// withQuickInfoHooks puts the paragraphs the hooks return for node in front of
// documentation.
func withQuickInfoHooks(c *checker.Checker, node *ast.Node, documentation string) string {
	var paragraphs []string
	for _, hook := range quickInfoHooks {
		if text := hook(c, node); text != "" {
			paragraphs = append(paragraphs, text)
		}
	}
	if len(paragraphs) == 0 {
		return documentation
	}
	if documentation != "" {
		paragraphs = append(paragraphs, documentation)
	}
	return strings.Join(paragraphs, "\n\n")
}
