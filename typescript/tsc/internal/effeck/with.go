package effeck

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// with blocks (D130).
//
//	with UsersLive {
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
//	const completion = yield* Effect.provide(Effect.fnUntraced(function* f$with() {
//	  return { kind: "return", value: yield* greet("42") };
//	})(), UsersLive);
//	if (completion.kind === "return") return completion.value;

func init() { fileChecks = append(fileChecks, checkWithBlocks) }

// isWithBlock reports whether node is the block of a with statement.
func isWithBlock(node *ast.Node) bool {
	return node.Kind == ast.KindBlock && node.Flags&ast.NodeFlagsEffeck != 0
}

// isWithLayer reports whether node is the statement that holds the layer
// expression of a with block.
func isWithLayer(node *ast.Node) bool {
	return node.Kind == ast.KindExpressionStatement && node.Flags&ast.NodeFlagsEffeck != 0
}

// withLayer returns the layer expression of block, a with block.
func withLayer(block *ast.Node) *ast.Node {
	return block.AsBlock().Statements.Nodes[0].Expression()
}

// hasWithBlock reports whether fn's own body has a with block, which makes it
// an Effect: the block runs Effect.provide.
func hasWithBlock(fn *ast.Node) bool {
	if fn.Body() == nil {
		return false
	}
	var visit func(n *ast.Node) bool
	visit = func(n *ast.Node) bool {
		if isFunctionBoundary(n) {
			return false
		}
		return isWithBlock(n) || n.ForEachChild(visit)
	}
	return fn.Body().ForEachChild(visit) || isWithBlock(fn.Body())
}

// withFlow collects what can escape a with block: the errors of its
// statements and of building the layer, and the requirements of its
// statements that the layer does not provide, plus the layer's own.
func (a *analyzer) withFlow(block *ast.Node) *effectInfo {
	c := a.checker
	layer := withLayer(block)
	info := a.flowOf(layer)
	body := &effectInfo{}
	for _, statement := range block.AsBlock().Statements.Nodes[1:] {
		body.add(a.flowOf(statement))
	}
	if info.effect == nil {
		info.effect = body.effect
	}
	provided, errors, needs := providerChannels(c, c.GetTypeAtLocation(layer))
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

// providerChannels returns what a with block's layer expression provides,
// fails with, and needs. Like Effect.provide, it accepts a Layer, a Context,
// or an array of layers, whose channels join.
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

// visitWithBlock lowers node, a with block, to a region closure that runs
// under Effect.provide with the block's layer.
func (tx *lowering) visitWithBlock(node *ast.Node) *ast.Node {
	f := tx.Factory()
	layer := tx.Visitor().VisitNode(withLayer(node))
	r := &region{}
	statements := tx.regionStatements(r, node)
	run := tx.runNow(tx.regionFunction(r, "with", nil, node, statements), node)
	provide := f.NewPropertyAccessExpression(tx.effectName(), nil /*questionDotToken*/, f.NewIdentifier("provide"), ast.NodeFlagsNone)
	run = f.NewCallExpression(provide, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{run, layer}), ast.NodeFlagsNone)
	return tx.runRegion(node, run, r)
}

// checkWithBlocks reports EK1001 at a with block that no function encloses:
// a module's top level only declares, and such a block would run a program.
func checkWithBlocks(c *checker.Checker, file *ast.SourceFile) {
	var visit func(n *ast.Node) bool
	visit = func(n *ast.Node) bool {
		if isFunctionBoundary(n) {
			return false
		}
		if isWithBlock(n) {
			report(c, n.AsBlock().Statements.Nodes[0], EK1001.With("This with block runs a program, but a module's top level is not inside one."))
		}
		return n.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}
