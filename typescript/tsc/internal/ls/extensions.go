package ls

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/lsp/lsproto"
)

// RangeOf returns the LSP range of textRange in file, for the edits of a code
// fix that an in-tree package registers.
func (l *LanguageService) RangeOf(file *ast.SourceFile, textRange core.TextRange) lsproto.Range {
	r, _ := l.converters.ToLSPRange(file, textRange)
	return r
}

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

// RegisterCodeFixProvider adds provider to the quick fixes. A provider whose
// Source is not "" fixes diagnostics from that source, such as "EK". Call it
// only from the init function of an in-tree package that tsc/cmd/tsc imports;
// registration is not synchronized.
func RegisterCodeFixProvider(provider *CodeFixProvider) {
	codeFixProviders = append(codeFixProviders, provider)
}

// QuickInfoTextHook rewrites the quick info line of a hover on node after
// the QuickInfoHooks ran, for example to write its types another way.
type QuickInfoTextHook func(c *checker.Checker, node *ast.Node, text string) string

var quickInfoTextHooks []QuickInfoTextHook

// RegisterQuickInfoTextHook adds hook to every hover. Call it only from the
// init function of an in-tree package that tsc/cmd/tsc imports; registration
// is not synchronized.
func RegisterQuickInfoTextHook(hook QuickInfoTextHook) {
	quickInfoTextHooks = append(quickInfoTextHooks, hook)
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
	for _, hook := range quickInfoTextHooks {
		quickInfo = hook(c, node, quickInfo)
	}
	if len(paragraphs) == 0 {
		return quickInfo, documentation
	}
	if documentation != "" {
		paragraphs = append(paragraphs, documentation)
	}
	return quickInfo, strings.Join(paragraphs, "\n\n")
}

// CompletionScopeHook lists symbols that resolve at location although no
// scope enclosing location declares them, such as names that a checker name
// resolution hook supplies. Global completions offer them after the scopes'
// own symbols, which shadow them by name, and rank them as the file's own
// names.
type CompletionScopeHook func(c *checker.Checker, location *ast.Node, meaning ast.SymbolFlags) []*ast.Symbol

var completionScopeHooks []CompletionScopeHook

// RegisterCompletionScopeHook adds hook to every completion request. Call it
// only from the init function of an in-tree package that tsc/cmd/tsc
// imports; registration is not synchronized.
func RegisterCompletionScopeHook(hook CompletionScopeHook) {
	completionScopeHooks = append(completionScopeHooks, hook)
}

func completionScopeSymbolsFromHooks(c *checker.Checker, location *ast.Node, meaning ast.SymbolFlags) []*ast.Symbol {
	var symbols []*ast.Symbol
	for _, hook := range completionScopeHooks {
		symbols = append(symbols, hook(c, location, meaning)...)
	}
	return symbols
}

// MemberCompletionHook can replace t, the type whose members completions
// list after `left.`, for example to list the members that a property
// access on left reads in a language built on the checker. It returns t
// when it has nothing to say.
type MemberCompletionHook func(c *checker.Checker, left *ast.Node, t *checker.Type) *checker.Type

var memberCompletionHooks []MemberCompletionHook

// RegisterMemberCompletionHook adds hook to every completion request. Call it
// only from the init function of an in-tree package that tsc/cmd/tsc
// imports; registration is not synchronized.
func RegisterMemberCompletionHook(hook MemberCompletionHook) {
	memberCompletionHooks = append(memberCompletionHooks, hook)
}

func applyMemberCompletionHooks(c *checker.Checker, left *ast.Node, t *checker.Type) *checker.Type {
	for _, hook := range memberCompletionHooks {
		t = hook(c, left, t)
	}
	return t
}

// ToLSPRange converts textRange in file to an LSP range, for a quick fix
// whose edit covers more than its diagnostic's range.
func (l *LanguageService) ToLSPRange(file *ast.SourceFile, textRange core.TextRange) lsproto.Range {
	rng, _ := l.converters.ToLSPRange(file, textRange)
	return rng
}
