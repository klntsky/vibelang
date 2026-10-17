package effeck

import (
	"context"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/ls"
	"github.com/microsoft/TypeScript/tsc/internal/lsp/lsproto"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// provide blocks (D130, D172).
//
//	provide UsersLive {
//	  return greet("42")
//	}
//
// runs its statements with the services UsersLive provides. The parser reads
// it as a Block flagged NodeFlagsEffeck whose first statement is the layer
// expression, so the checker treats the statements as code of the enclosing
// function: a return returns from it. The analyzer gives the block the
// errors of its statements and the layer, and the requirements of its
// statements that the layer does not provide plus the layer's own. The
// lowering runs the statements as a region closure under Effect.provide:
//
//	const completion = yield* Effect.provide(Effect.fnUntraced(function* f$provide() {
//	  return { kind: "return", value: yield* greet("42") };
//	})(), UsersLive);
//	if (completion.kind === "return") return completion.value;
//
// The keyword matches Effect.provide. The old spelling, with UsersLive { ... },
// still compiles and draws the warning EK1009, whose quick fix writes provide.

func init() {
	fileChecks = append(fileChecks, checkProvideBlocks, checkProvideSpelling)
	ls.RegisterCodeFixProvider(provideSpellingFix)
	ls.RegisterQuickInfoHook(providerQuickInfo)
}

// isProvideBlock reports whether node is the block of a provide statement.
func isProvideBlock(node *ast.Node) bool {
	return node.Kind == ast.KindBlock && node.Flags&ast.NodeFlagsEffeck != 0
}

// isProvideLayer reports whether node is the statement that holds the layer
// expression of a provide block.
func isProvideLayer(node *ast.Node) bool {
	return node.Kind == ast.KindExpressionStatement && node.Flags&ast.NodeFlagsEffeck != 0
}

// provideLayer returns the layer expression of block, a provide block.
func provideLayer(block *ast.Node) *ast.Node {
	return block.AsBlock().Statements.Nodes[0].Expression()
}

// hasProvideBlock reports whether fn's own body has a provide block, which makes it
// an Effect: the block runs Effect.provide.
func hasProvideBlock(fn *ast.Node) bool {
	if fn.Body() == nil {
		return false
	}
	var visit func(n *ast.Node) bool
	visit = func(n *ast.Node) bool {
		if isFunctionBoundary(n) {
			return false
		}
		return isProvideBlock(n) || n.ForEachChild(visit)
	}
	return fn.Body().ForEachChild(visit) || isProvideBlock(fn.Body())
}

// provideFlow collects what can escape a provide block: the errors of its
// statements and of building the layer, and the requirements of its
// statements that the layer does not provide, plus the layer's own.
func (a *analyzer) provideFlow(block *ast.Node) *effectInfo {
	c := a.checker
	layer := provideLayer(block)
	info := a.flowOf(layer)
	body := &effectInfo{}
	for _, statement := range block.AsBlock().Statements.Nodes[1:] {
		body.add(a.flowOf(statement))
	}
	if info.effect == nil {
		info.effect = body.effect
	}
	provided, errors, needs := providerChannelsOf(c, layer)
	info.errors = append(info.errors, body.errors...)
	info.errorSources = append(info.errorSources, body.errorSources...)
	for i, requirement := range body.requirements {
		for _, member := range unionMembers(requirement) {
			if provided == nil || !c.IsTypeAssignableTo(member, provided) {
				info.addRequirement(member, body.requirementSources[i])
			}
		}
	}
	if errors != nil && errors.Flags()&checker.TypeFlagsNever == 0 {
		info.addError(errors, layer)
	}
	if needs != nil && needs.Flags()&checker.TypeFlagsNever == 0 {
		info.addRequirement(needs, layer)
	}
	return info
}

// providerChannels returns what a provide block's layer expression provides,
// fails with, and needs. Like Effect.provide, it accepts a Layer, a Context,
// or an array of layers, whose channels join.
// providerChannelsOf returns what a provide block's provider expression
// provides, fails with, and needs. An array literal, such as the list that
// provide A, Clock = testClock { ... } becomes (D173), is read element by
// element: its own type merges the elements into one union, and because a
// layer is contravariant in what it provides, Layer<Names> | Layer<never>
// reduces to Layer<never>, which would provide nothing.
func providerChannelsOf(c *checker.Checker, layer *ast.Node) (provided, errors, needs *checker.Type) {
	list := ast.SkipParentheses(layer)
	if !ast.IsArrayLiteralExpression(list) || core.Some(list.AsArrayLiteralExpression().Elements.Nodes, ast.IsSpreadElement) {
		return providerChannels(c, c.GetTypeAtLocation(layer))
	}
	var outs, errs, ins []*checker.Type
	for _, element := range list.AsArrayLiteralExpression().Elements.Nodes {
		out, e, in := providerChannels(c, c.GetTypeAtLocation(element))
		if out == nil {
			return providerChannels(c, c.GetTypeAtLocation(layer))
		}
		outs = append(outs, out)
		if e != nil {
			errs = append(errs, e)
		}
		if in != nil {
			ins = append(ins, in)
		}
	}
	join := func(types []*checker.Type) *checker.Type {
		if len(types) == 0 {
			return nil
		}
		return c.GetUnionType(types)
	}
	return join(outs), join(errs), join(ins)
}

func providerChannels(c *checker.Checker, t *checker.Type) (provided, errors, needs *checker.Type) {
	var outs, errs, ins []*checker.Type
	var add func(t *checker.Type) bool
	add = func(t *checker.Type) bool {
		// Split a union first: its merged variance struct would read as one
		// layer that provides the intersection of the members' services.
		if t.Flags()&checker.TypeFlagsUnion != 0 {
			for _, member := range unionMembers(t) {
				if !add(member) {
					return false
				}
			}
			return true
		}
		if out, e, in := layerChannels(c, t); out != nil {
			outs, errs, ins = append(outs, out), append(errs, e), append(ins, in)
			return true
		}
		if services := contextServices(c, t); services != nil {
			outs = append(outs, services)
			return true
		}
		if checker.IsTupleType(t) {
			for _, element := range c.GetTypeArguments(t) {
				if !add(element) {
					return false
				}
			}
			return true
		}
		if c.IsArrayType(t) {
			return add(c.GetElementTypeOfArrayType(t))
		}
		return false
	}
	if !add(t) || len(outs) == 0 {
		return nil, nil, nil
	}
	join := func(types []*checker.Type) *checker.Type {
		var present []*checker.Type
		for _, t := range types {
			if t != nil {
				present = append(present, t)
			}
		}
		if len(present) == 0 {
			return nil
		}
		return c.GetUnionType(present)
	}
	return join(outs), join(errs), join(ins)
}

// contextServices returns the services a Context<Services> holds, or nil
// when t is not a Context. Its variance struct holds Services
// contravariantly.
func contextServices(c *checker.Checker, t *checker.Type) *checker.Type {
	for _, property := range c.GetPropertiesOfType(t) {
		services := c.GetTypeOfPropertyOfType(c.GetTypeOfSymbol(property), "_Services")
		if services == nil {
			continue
		}
		signatures := c.GetSignaturesOfType(services, checker.SignatureKindCall)
		if len(signatures) == 1 && len(signatures[0].Parameters()) == 1 {
			return c.GetTypeOfSymbol(signatures[0].Parameters()[0])
		}
	}
	return nil
}

// layerChannels returns what a Layer<ROut, E, RIn> provides, fails with, and
// needs, or nils when t is not a Layer. The Layer's variance struct holds
// ROut contravariantly, as a function's parameter, and E and RIn
// covariantly, as return types.
func layerChannels(c *checker.Checker, t *checker.Type) (provided, errors, needs *checker.Type) {
	for _, property := range c.GetPropertiesOfType(t) {
		variance := c.GetTypeOfSymbol(property)
		out := c.GetTypeOfPropertyOfType(variance, "_ROut")
		e := c.GetTypeOfPropertyOfType(variance, "_E")
		in := c.GetTypeOfPropertyOfType(variance, "_RIn")
		if out == nil || e == nil || in == nil {
			continue
		}
		parameter := func(f *checker.Type) *checker.Type {
			signatures := c.GetSignaturesOfType(f, checker.SignatureKindCall)
			if len(signatures) != 1 || len(signatures[0].Parameters()) != 1 {
				return nil
			}
			return c.GetTypeOfSymbol(signatures[0].Parameters()[0])
		}
		result := func(f *checker.Type) *checker.Type {
			signatures := c.GetSignaturesOfType(f, checker.SignatureKindCall)
			if len(signatures) != 1 {
				return nil
			}
			return c.GetReturnTypeOfSignature(signatures[0])
		}
		return parameter(out), result(e), result(in)
	}
	return nil, nil, nil
}

// visitProvideBlock lowers node, a provide block, to a region closure that runs
// under Effect.provide with the block's layer.
func (tx *lowering) visitProvideBlock(node *ast.Node) *ast.Node {
	f := tx.Factory()
	layer := tx.Visitor().VisitNode(provideLayer(node))
	r := &region{}
	statements := tx.regionStatements(r, node)
	run := tx.runNow(tx.regionFunction(r, "provide", nil, node, statements), node)
	provide := f.NewPropertyAccessExpression(tx.effectName(), nil /*questionDotToken*/, f.NewIdentifier("provide"), ast.NodeFlagsNone)
	run = f.NewCallExpression(provide, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{run, layer}), ast.NodeFlagsNone)
	return tx.runRegion(node, run, r)
}

// checkProvideBlocks reports EK1001 at a provide block that no function encloses:
// a module's top level only declares, and such a block would run a program.
func checkProvideBlocks(c *checker.Checker, file *ast.SourceFile) {
	var visit func(n *ast.Node) bool
	visit = func(n *ast.Node) bool {
		if isFunctionBoundary(n) {
			return false
		}
		if isProvideBlock(n) {
			report(c, n.AsBlock().Statements.Nodes[0], EK1001.With("This provide block runs a program, but a module's top level is not inside one."))
		}
		return n.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// checkProvideSpelling reports EK1009, a warning, at a provide block written
// with its old keyword, with (D172). The block compiles the same either way.
func checkProvideSpelling(c *checker.Checker, file *ast.SourceFile) {
	text := file.Text()
	var visit func(n *ast.Node) bool
	visit = func(n *ast.Node) bool {
		if isProvideBlock(n) {
			if start := scanner.SkipTrivia(text, n.Pos()); strings.HasPrefix(text[start:], "with") {
				reportAt(c, file, core.NewTextRange(start, start+len("with")), EK1009)
			}
		}
		return n.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// provideSpellingFix is EK1009's quick fix: it replaces with by provide.
var provideSpellingFix = &ls.CodeFixProvider{
	Source:     diagnosticSource,
	ErrorCodes: []int32{EK1009.Code},
	GetCodeActions: func(_ context.Context, fix *ls.CodeFixContext) ([]*ls.CodeAction, error) {
		text := fix.SourceFile.Text()
		if fix.Diagnostic == nil || fix.Span.End() > len(text) || text[fix.Span.Pos():fix.Span.End()] != "with" {
			return nil, nil
		}
		return []*ls.CodeAction{{
			Description: "Write provide instead of with",
			Changes:     []*lsproto.TextEdit{{Range: fix.Diagnostic.Range, NewText: "provide"}},
		}}, nil
	},
}

// providerCall returns the $provideService call that the parser wrote for a
// Key = value provider (D173) when node is part of its key, or nil.
func providerCall(node *ast.Node) *ast.Node {
	for n := node; n != nil; n = n.Parent {
		if !ast.IsCallExpression(n) {
			continue
		}
		callee := n.Expression()
		if !ast.IsIdentifier(callee) || callee.Text() != "$provideService" || callee.Flags&ast.NodeFlagsReparsed == 0 {
			return nil
		}
		args := n.Arguments()
		if len(args) != 3 || node.Pos() < args[0].Pos() || node.End() > args[0].End() {
			return nil
		}
		return n
	}
	return nil
}

// providerQuickInfo adds a line to a hover over the key of a Key = value
// provider naming the service the block receives. A module whose export of
// the same name is the key, such as Random, names that export: Random.Random.
func providerQuickInfo(c *checker.Checker, node *ast.Node) (string, string) {
	call := providerCall(node)
	if call == nil {
		return "", ""
	}
	args := call.Arguments()
	key := scanner.GetSourceTextOfNodeFromSourceFile(ast.GetSourceFileOfNode(call), args[0], false /*includeTrivia*/)
	name := args[2].Text()
	if c.GetPropertyOfType(c.GetTypeAtLocation(args[0]), name) != nil {
		key += "." + name
	}
	return "", "Provides the service `" + key + "` to this provide block's statements."
}
