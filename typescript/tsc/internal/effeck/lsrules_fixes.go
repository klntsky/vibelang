package effeck

import (
	"context"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/ls"
	"github.com/microsoft/TypeScript/tsc/internal/lsp/lsproto"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// The quick fixes for the EK48xx rules (lsrules.go). Each finds the node
// that its diagnostic was reported at, the node whose error span is the
// diagnostic's, and rewrites the source around it.

// reportedNode returns the outermost node of file whose error span is span
// and that test accepts, or nil.
func reportedNode(file *ast.SourceFile, span core.TextRange, test func(node *ast.Node) bool) *ast.Node {
	var found *ast.Node
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if found != nil || node.End() < span.Pos() || node.Pos() > span.End() {
			return found != nil
		}
		if test(node) && scanner.GetErrorRangeForNode(file, node) == span {
			found = node
			return true
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
	return found
}

// replaceRange returns one text edit that replaces the span from start to end.
func replaceRange(fix *ls.CodeFixContext, start, end int, text string) []*lsproto.TextEdit {
	return []*lsproto.TextEdit{{Range: fix.LS.ToLSPRange(fix.SourceFile, core.NewTextRange(start, end)), NewText: text}}
}

// nodeStart returns where node's text starts, after its leading trivia.
func nodeStart(file *ast.SourceFile, node *ast.Node) int {
	return scanner.SkipTrivia(file.Text(), node.Pos())
}

// callEffectFix calls the Effect value that EK4801 or EK4802 reports, so
// that it runs: e becomes e(), and a + b becomes (a + b)().
var callEffectFix = &ls.CodeFixProvider{
	Source:     diagnosticSource,
	ErrorCodes: []int32{EK4801.Code, EK4802.Code},
	GetCodeActions: func(_ context.Context, fix *ls.CodeFixContext) ([]*ls.CodeAction, error) {
		value := reportedNode(fix.SourceFile, fix.Span, func(node *ast.Node) bool {
			parent := node.Parent
			switch {
			case ast.IsExpressionStatement(parent) && parent.Expression() == node:
				return inLowerableBody(parent)
			case ast.IsReturnStatement(parent):
				return true
			case ast.IsArrowFunction(parent):
				return parent.Body() == node
			}
			return false
		})
		if value == nil {
			return nil, nil
		}
		changes := replaceRange(fix, value.End(), value.End(), "()")
		if needsParentheses(value) {
			changes = append(replaceRange(fix, nodeStart(fix.SourceFile, value), nodeStart(fix.SourceFile, value), "("), replaceRange(fix, value.End(), value.End(), ")()")...)
		}
		return []*ls.CodeAction{{Description: "Call it to run it", Changes: changes}}, nil
	},
}

// mergeAllFix writes wire for the Layer.mergeAll that EK4807 reports.
var mergeAllFix = &ls.CodeFixProvider{
	Source:     diagnosticSource,
	ErrorCodes: []int32{EK4807.Code},
	GetCodeActions: func(_ context.Context, fix *ls.CodeFixContext) ([]*ls.CodeAction, error) {
		argument := reportedNode(fix.SourceFile, fix.Span, func(node *ast.Node) bool {
			return node.Parent != nil && ast.IsCallExpression(node.Parent) && node.Parent.Expression() != node
		})
		if argument == nil {
			return nil, nil
		}
		callee := argument.Parent.Expression()
		return []*ls.CodeAction{{
			Description: "Write wire for Layer.mergeAll",
			Changes:     replaceRange(fix, nodeStart(fix.SourceFile, callee), callee.End(), "wire"),
		}}, nil
	},
}

// nestedProvideFix merges the provide block that EK4808 reports into the
// block around it: provide A { provide B { … } } becomes provide A, B { … },
// or provide wire(A, B) { … } when B needs a service that A provides.
var nestedProvideFix = &ls.CodeFixProvider{
	Source:     diagnosticSource,
	ErrorCodes: []int32{EK4808.Code},
	GetCodeActions: func(ctx context.Context, fix *ls.CodeFixContext) ([]*ls.CodeAction, error) {
		file := fix.SourceFile
		var inner *ast.Node
		var visit func(node *ast.Node) bool
		visit = func(node *ast.Node) bool {
			if isProvideBlock(node) && nodeStart(file, node) == fix.Span.Pos() && isProvideBlock(node.Parent) {
				inner = node
				return true
			}
			return node.ForEachChild(visit)
		}
		file.AsNode().ForEachChild(visit)
		if inner == nil {
			return nil, nil
		}
		outer := inner.Parent
		outerLayer, innerLayer := provideLayer(outer), provideLayer(inner)
		c, done := fix.Program.GetTypeCheckerForFile(ctx, file)
		provided, _, _ := providerChannelsOf(c, outerLayer)
		_, _, needs := providerChannelsOf(c, innerLayer)
		dependent := false
		if provided != nil && needs != nil {
			for _, t := range needs.Distributed() {
				for _, p := range provided.Distributed() {
					dependent = dependent || t == p
				}
			}
		}
		done()
		outerText, innerText := scanner.GetTextOfNode(outerLayer), scanner.GetTextOfNode(innerLayer)
		providers := outerText + ", " + innerText
		if dependent {
			if len(providerList(outerLayer)) != 1 || len(providerList(innerLayer)) != 1 || isServiceProvider(outerLayer) || isServiceProvider(innerLayer) {
				return nil, nil // wire composes layers, not Key = value providers
			}
			providers = "wire(" + providers + ")"
		}
		text := file.Text()
		open := strings.IndexByte(text[innerLayer.End():], '{')
		if open < 0 {
			return nil, nil
		}
		body := text[innerLayer.End()+open+1 : inner.End()-1]
		body = strings.ReplaceAll(body, "\n"+lineIndent(text, nodeStart(file, inner)), "\n"+lineIndent(text, nodeStart(file, outer)))
		return []*ls.CodeAction{{
			Description: "Provide both layers in one block",
			Changes:     replaceRange(fix, nodeStart(file, outer), outer.End(), "provide "+providers+" {"+body+"}"),
		}}, nil
	},
}

// assertionFix removes the assertion that EK4810 reports.
var assertionFix = &ls.CodeFixProvider{
	Source:     diagnosticSource,
	ErrorCodes: []int32{EK4810.Code},
	GetCodeActions: func(_ context.Context, fix *ls.CodeFixContext) ([]*ls.CodeAction, error) {
		assertion := reportedNode(fix.SourceFile, fix.Span, func(node *ast.Node) bool {
			return ast.IsAsExpression(node) || ast.IsTypeAssertion(node)
		})
		if assertion == nil {
			return nil, nil
		}
		return []*ls.CodeAction{{
			Description: "Remove the assertion",
			Changes:     replaceRange(fix, nodeStart(fix.SourceFile, assertion), assertion.End(), scanner.GetTextOfNode(assertion.Expression())),
		}}, nil
	},
}

// outdatedAPIFix writes the Effect 4 name of the Effect 3 name that EK4813
// reports.
var outdatedAPIFix = &ls.CodeFixProvider{
	Source:     diagnosticSource,
	ErrorCodes: []int32{EK4813.Code},
	GetCodeActions: func(_ context.Context, fix *ls.CodeFixContext) ([]*ls.CodeAction, error) {
		return renameEffect3API(fix, reportedNode(fix.SourceFile, fix.Span, isEffect3Name)), nil
	},
}

// outdatedAPITypeScriptFix is outdatedAPIFix for a .ts file, where
// TypeScript's TS2339 reports an Effect 3 name of effect's Effect module.
var outdatedAPITypeScriptFix = &ls.CodeFixProvider{
	ErrorCodes: []int32{2339},
	GetCodeActions: func(ctx context.Context, fix *ls.CodeFixContext) ([]*ls.CodeAction, error) {
		name := reportedNode(fix.SourceFile, fix.Span, isEffect3Name)
		if name == nil {
			return nil, nil
		}
		c, done := fix.Program.GetTypeCheckerForFile(ctx, fix.SourceFile)
		defer done()
		if effectModuleOf(c, name.Parent.Expression()) != "Effect" {
			return nil, nil
		}
		return renameEffect3API(fix, name), nil
	},
}

// isEffect3Name reports whether node is the name of a property access that
// effect3API lists as renamed.
func isEffect3Name(node *ast.Node) bool {
	return ast.IsIdentifier(node) && node.Parent != nil && ast.IsPropertyAccessExpression(node.Parent) && node.Parent.Name() == node && effect3API[node.Text()].renamed != ""
}

// renameEffect3API returns the action that writes name's Effect 4 name.
func renameEffect3API(fix *ls.CodeFixContext, name *ast.Node) []*ls.CodeAction {
	if name == nil {
		return nil
	}
	renamed := effect3API[name.Text()].renamed
	module := scanner.GetTextOfNode(name.Parent.Expression())
	return []*ls.CodeAction{{
		Description: "Write " + module + "." + renamed,
		Changes:     replaceRange(fix, nodeStart(fix.SourceFile, name), name.End(), renamed),
	}}
}

// schemaIsFix writes Schema.is(S)(x) for the x instanceof S that EK4817
// reports.
var schemaIsFix = &ls.CodeFixProvider{
	Source:     diagnosticSource,
	ErrorCodes: []int32{EK4817.Code},
	GetCodeActions: func(_ context.Context, fix *ls.CodeFixContext) ([]*ls.CodeAction, error) {
		test := reportedNode(fix.SourceFile, fix.Span, func(node *ast.Node) bool {
			return ast.IsBinaryExpression(node) && node.AsBinaryExpression().OperatorToken.Kind == ast.KindInstanceOfKeyword
		})
		if test == nil {
			return nil, nil
		}
		b := test.AsBinaryExpression()
		text := "Schema.is(" + scanner.GetTextOfNode(b.Right) + ")(" + scanner.GetTextOfNode(b.Left) + ")"
		return []*ls.CodeAction{{Description: "Test with Schema.is", Changes: replaceRange(fix, nodeStart(fix.SourceFile, test), test.End(), text)}}, nil
	},
}
