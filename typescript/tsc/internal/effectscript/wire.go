package effectscript

import (
	"slices"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// wire (design D27) composes layers into one: wire(UsersLive, MailerLive,
// DatabaseLive, ConfigLive). The runtime's wire folds its arguments with
// Layer.provideMerge from the last to the first, so each argument may use
// what the arguments after it provide. The compiler reads each argument's
// Layer<Out, E, In> type and emits the arguments in an order where every
// consumer comes before its providers, keeping source order among
// independent arguments. It reports a duplicate provider (ES3202), a cycle
// (ES3203), and an argument whose type does not name its outputs (ES3204).
// An input that no argument provides stays in the result's In, as with
// Layer.provide, and is reported with the provider that needs it where the
// program's requirements must be empty, at a runner (ES3201).

// layerTypeID names the variance property of every Effect 4 Layer:
// `readonly [TypeId]: { _ROut: Contravariant<ROut>; _E; _RIn }`.
const layerTypeID = "~effect/Layer"

func init() { fileChecks = append(fileChecks, checkWires, checkRunners) }

// wireArgument is what one argument of a wire call provides and needs.
type wireArgument struct {
	node    *ast.Node
	outputs []*checker.Type // the services it provides
	inputs  []*checker.Type // the services it needs
	opaque  bool            // its type does not name its outputs (ES3204)
}

// wireGraph is the provider graph of one wire call.
type wireGraph struct {
	arguments  []wireArgument
	needs      [][]int // needs[i] lists the arguments that provide what argument i needs
	duplicates map[*checker.Type][]int
	cycle      []int // a cycle of arguments, the first repeated last, or nil
	order      []int // the emission order: consumers before providers
}

// isWireCall reports whether call calls the runtime's wire.
func isWireCall(c *checker.Checker, call *ast.Node) bool {
	return ast.IsCallExpression(call) && isRuntimeExport(c, call.Expression(), "wire")
}

// isRuntimeExport reports whether expr, an identifier, names the export name
// of @effectscript/runtime, through an import or the prelude.
func isRuntimeExport(c *checker.Checker, expr *ast.Node, name string) bool {
	if !ast.IsIdentifier(expr) || expr.Text() != name {
		return false
	}
	symbol := c.GetSymbolAtLocation(expr)
	if symbol != nil && symbol.Flags()&ast.SymbolFlagsAlias != 0 {
		symbol = c.GetAliasedSymbol(symbol)
	}
	if symbol == nil || symbol.ValueDeclaration() == nil {
		return false
	}
	return packageNameOf(c, ast.GetSourceFileOfNode(symbol.ValueDeclaration())) == "@effectscript/runtime"
}

// packageNameOf returns the name in the package.json nearest to file.
func packageNameOf(c *checker.Checker, file *ast.SourceFile) string {
	program := c.Program()
	directory := program.GetNearestAncestorDirectoryWithPackageJson(file.FileName().Directory())
	if directory == "" {
		return ""
	}
	if info := program.GetPackageJsonInfo(directory.ResolveFile("package.json")); info.Exists() {
		if name, ok := info.Contents.Name.GetValue(); ok {
			return name
		}
	}
	return ""
}

// layerParts reads Out and In from t, a Layer<Out, E, In>, as lists of
// services. opaque reports an Out of any or unknown, which names no
// service. ok is false when t is not a Layer.
func layerParts(c *checker.Checker, t *checker.Type) (outputs, inputs []*checker.Type, opaque, ok bool) {
	variance := c.GetTypeOfPropertyOfType(t, layerTypeID)
	if variance == nil {
		return nil, nil, false, false
	}
	out := c.GetTypeOfPropertyOfType(variance, "_ROut") // (_: ROut) => void
	in := c.GetTypeOfPropertyOfType(variance, "_RIn")   // (_: never) => RIn
	if out == nil || in == nil {
		return nil, nil, false, false
	}
	if out.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown) != 0 {
		return nil, nil, true, true // Layer.Any: { _ROut: any, _E: any, _RIn: any }
	}
	outSignatures := c.GetSignaturesOfType(out, checker.SignatureKindCall)
	inSignatures := c.GetSignaturesOfType(in, checker.SignatureKindCall)
	if len(outSignatures) != 1 || len(inSignatures) != 1 {
		return nil, nil, false, false
	}
	outType := c.GetTypeAtPosition(outSignatures[0], 0)
	if outType.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown) != 0 {
		return nil, unionMembers(c.GetReturnTypeOfSignature(inSignatures[0])), true, true
	}
	return unionMembers(outType), unionMembers(c.GetReturnTypeOfSignature(inSignatures[0])), false, true
}

// unionMembers returns the members of t, a union, or t alone; never has none.
func unionMembers(t *checker.Type) []*checker.Type {
	switch {
	case t.Flags()&checker.TypeFlagsNever != 0:
		return nil
	case t.Flags()&checker.TypeFlagsUnion != 0:
		return t.Types()
	}
	return []*checker.Type{t}
}

// graphOfWire builds the provider graph of call, a wire call.
func graphOfWire(c *checker.Checker, call *ast.Node) *wireGraph {
	g := &wireGraph{duplicates: map[*checker.Type][]int{}}
	for _, argument := range call.Arguments() {
		outputs, inputs, opaque, _ := layerParts(c, c.GetTypeAtLocation(argument))
		g.arguments = append(g.arguments, wireArgument{node: argument, outputs: outputs, inputs: inputs, opaque: opaque})
	}
	providers := map[*checker.Type][]int{}
	for i, a := range g.arguments {
		for _, output := range a.outputs {
			providers[output] = append(providers[output], i)
		}
	}
	for output, indexes := range providers {
		if len(indexes) > 1 {
			g.duplicates[output] = indexes
		}
	}
	g.needs = make([][]int, len(g.arguments))
	for i, a := range g.arguments {
		for _, input := range a.inputs {
			for _, j := range providers[input] {
				if j != i && !slices.Contains(g.needs[i], j) {
					g.needs[i] = append(g.needs[i], j)
				}
			}
		}
	}
	g.cycle = findCycle(g.needs)
	g.order = consumersFirst(g.needs)
	return g
}

// findCycle returns a cycle in the graph needs describes, the first node
// repeated at the end, or nil.
func findCycle(needs [][]int) []int {
	const (
		unvisited = iota
		visiting
		done
	)
	state := make([]int, len(needs))
	var path, cycle []int
	var visit func(i int) bool
	visit = func(i int) bool {
		switch state[i] {
		case visiting:
			start := slices.Index(path, i)
			cycle = append(slices.Clone(path[start:]), i)
			return true
		case done:
			return false
		}
		state[i] = visiting
		path = append(path, i)
		for _, j := range needs[i] {
			if visit(j) {
				return true
			}
		}
		path = path[:len(path)-1]
		state[i] = done
		return false
	}
	for i := range needs {
		if visit(i) {
			return cycle
		}
	}
	return nil
}

// consumersFirst returns a topological order of the graph needs describes in
// which every node comes before the nodes it needs, choosing the lowest
// index among the nodes that are ready. With a cycle it returns source order.
func consumersFirst(needs [][]int) []int {
	waiting := make([]int, len(needs)) // how many consumers of each node are not yet placed
	for _, js := range needs {
		for _, j := range js {
			waiting[j]++
		}
	}
	var order []int
	placed := make([]bool, len(needs))
	for len(order) < len(needs) {
		next := -1
		for i := range needs {
			if !placed[i] && waiting[i] == 0 {
				next = i
				break
			}
		}
		if next < 0 {
			order = order[:0]
			for i := range needs {
				order = append(order, i)
			}
			return order
		}
		placed[next] = true
		order = append(order, next)
		for _, j := range needs[next] {
			waiting[j]--
		}
	}
	return order
}

// checkWires reports ES3202, ES3203, and ES3204 for the wire calls in file.
func checkWires(c *checker.Checker, file *ast.SourceFile) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if isWireCall(c, node) {
			checkWire(c, node)
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

func checkWire(c *checker.Checker, call *ast.Node) {
	g := graphOfWire(c, call)
	for _, a := range g.arguments {
		if a.opaque {
			note(report(c, a.node, ES3204, scanner.GetTextOfNode(a.node)), "Compose it with Layer.provide instead.")
		}
	}
	reported := map[int]bool{}
	for i, a := range g.arguments {
		for _, output := range a.outputs {
			indexes := g.duplicates[output]
			if len(indexes) < 2 || indexes[len(indexes)-1] != i || reported[i] {
				continue
			}
			reported[i] = true
			names := make([]string, len(indexes))
			for k, index := range indexes {
				names[k] = scanner.GetTextOfNode(g.arguments[index].node)
			}
			report(c, a.node, ES3202, c.TypeToString(output), strings.Join(names, " and "))
		}
	}
	if g.cycle != nil {
		var steps []string
		for k, i := range g.cycle {
			steps = append(steps, scanner.GetTextOfNode(g.arguments[i].node))
			if k+1 < len(g.cycle) {
				if service := neededFrom(g, i, g.cycle[k+1]); service != nil {
					steps = append(steps, c.TypeToString(service))
				}
			}
		}
		report(c, call, ES3203, strings.Join(steps, " → "))
	}
}

// neededFrom returns a service that argument i needs and argument j provides.
func neededFrom(g *wireGraph, i, j int) *checker.Type {
	for _, input := range g.arguments[i].inputs {
		if slices.Contains(g.arguments[j].outputs, input) {
			return input
		}
	}
	return nil
}

// planWire records the order in which the lowering emits call's arguments,
// when call is a wire call whose arguments are not already in that order.
func (p *servicePlan) planWire(c *checker.Checker, call *ast.Node) {
	if !isWireCall(c, call) {
		return
	}
	g := graphOfWire(c, call)
	if g.cycle == nil && !slices.IsSorted(g.order) {
		p.wireOrders[call] = g.order
	}
}

// visitWireCall emits a wire call with its arguments in dependency order.
func (tx *lowering) visitWireCall(node *ast.Node) *ast.Node {
	order := tx.plan.wireOrders[node]
	visited := tx.Visitor().VisitNodes(node.ArgumentList())
	arguments := make([]*ast.Node, len(order))
	for k, i := range order {
		arguments[k] = visited.Nodes[i]
	}
	f := tx.Factory()
	return f.UpdateCallExpression(node.AsCallExpression(), tx.Visitor().VisitNode(node.Expression()), nil /*questionDotToken*/, node.TypeArgumentList(), f.NewNodeList(arguments), ast.NodeFlagsNone)
}

// checkRunners reports ES3201 at a runner whose program still needs a
// service that a wire call in the program leaves unprovided: the dependency
// path names the provider that needs it.
func checkRunners(c *checker.Checker, file *ast.SourceFile) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if ast.IsCallExpression(node) {
			switch {
			case isRootRunner(c, node) && len(node.Arguments()) > 0:
				checkProgram(c, node.Arguments()[0], nil /*accepted*/, false /*test*/)
			case isTestRegistration(c, node):
				checkTest(c, node)
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// checkTest checks the program of a test that it.effect registers against
// what the test provides: Scope, and the services of an enclosing layer(…),
// which the type of its parameter says.
func checkTest(c *checker.Checker, call *ast.Node) {
	signature := c.GetResolvedSignature(call)
	if signature == nil {
		return
	}
	for i, program := range call.Arguments() {
		if accepted := requirementsOf(c, c.GetTypeAtPosition(signature, i)); accepted != nil {
			checkProgram(c, program, accepted, true /*test*/)
			return
		}
	}
}

// checkProgram reports ES3201 for the services that program, the program a
// runner receives, needs and accepted does not include: a root runner such
// as Effect.runPromise or NodeRuntime.runMain starts with an empty Context,
// so accepted is nil. TypeScript would report that the program's type does
// not fit the parameter; ES3201 names the services instead, and the wire
// arguments that need them.
func checkProgram(c *checker.Checker, program *ast.Node, accepted *checker.Type, test bool) {
	needed := requirementsOf(c, c.GetTypeAtLocation(program))
	if needed == nil {
		return
	}
	var names, paths []string
	for _, service := range unionMembers(needed) {
		if accepted != nil && c.IsTypeAssignableTo(service, accepted) {
			continue
		}
		names = append(names, c.TypeToString(service))
		for _, needer := range servicesFor(c).wireNeeders(service) {
			paths = append(paths, scanner.GetTextOfNode(needer)+" needs "+c.TypeToString(service))
		}
	}
	if len(names) == 0 {
		return
	}
	d := report(c, program, ES3201, strings.Join(names, ", "))
	for _, path := range paths {
		note(d, "{0}, and no argument of its wire provides it.", path)
	}
	switch {
	case len(paths) > 0:
		note(d, "Add a provider of {0} to wire.", strings.Join(names, ", "))
	case test:
		note(d, "Provide it to the test with Effect.provide(() => …, layer), or register the test inside layer(…).")
	default:
		note(d, "Provide it to the program with Effect.provide(() => …, layer).")
	}
	coverArgument(c, program)
}

// requirementsOf returns R of t, an Effect or a function that returns one,
// or nil for any other type.
func requirementsOf(c *checker.Checker, t *checker.Type) *checker.Type {
	if r := varianceChannel(c, t, "_R"); r != nil {
		return r
	}
	if signatures := c.GetSignaturesOfType(t, checker.SignatureKindCall); len(signatures) == 1 {
		return varianceChannel(c, c.GetReturnTypeOfSignature(signatures[0]), "_R")
	}
	return nil
}

// wireNeeders returns the wire arguments, anywhere in the program, that need
// service while no argument of their wire call provides it.
func (s *services) wireNeeders(service *checker.Type) []*ast.Node {
	if s.unprovided == nil {
		s.unprovided = map[*checker.Type][]*ast.Node{}
		for _, file := range s.checker.Program().SourceFiles() {
			if !isEffectScriptFile(file) {
				continue
			}
			var visit func(node *ast.Node) bool
			visit = func(node *ast.Node) bool {
				if isWireCall(s.checker, node) {
					g := graphOfWire(s.checker, node)
					for i, a := range g.arguments {
						for _, input := range a.inputs {
							if !providedBy(g, input, i) {
								s.unprovided[input] = append(s.unprovided[input], a.node)
							}
						}
					}
				}
				return node.ForEachChild(visit)
			}
			file.AsNode().ForEachChild(visit)
		}
	}
	return s.unprovided[service]
}

// providedBy reports whether an argument of g other than i provides service.
func providedBy(g *wireGraph, service *checker.Type, i int) bool {
	for j, a := range g.arguments {
		if j != i && slices.Contains(a.outputs, service) {
			return true
		}
	}
	return false
}
