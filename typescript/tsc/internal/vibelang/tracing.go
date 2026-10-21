package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// Tracing (design D34). With compilerOptions.vibelang.tracing set to
// "exported", each exported function compiles with an Effect.fn span named
// after it; with "all", so does every named function that compiles to
// frames, and a method's Effect runs inside an Effect.withSpan span:
//
//	export const greet = Effect.fn("greet")(function* greet(name) { … })
//
// A lambda without a name, such as a callback, gets no span.

// tracingPlan records the functions that compile with spans, and the spans'
// names.
type tracingPlan struct {
	spans map[*ast.Node]string // function -> the name of its span
}

func newTracingPlan() tracingPlan {
	return tracingPlan{spans: map[*ast.Node]string{}}
}

// planSpans records a span for each function in p.effects, and each lowered
// method, that the tracing mode of the project that owns file selects (D34,
// D297).
func (p *plan) planSpans(c *checker.Checker, file *ast.SourceFile) {
	mode := ""
	if options := optionsFor(c, file); options != nil && options.VibeLang != nil {
		mode = options.VibeLang.Tracing
	}
	if mode != "exported" && mode != "all" {
		return
	}
	for fn := range p.effects {
		name := spanName(fn)
		if name == "" || mode == "exported" && (fn.Kind == ast.KindMethodDeclaration || !isExportedFunction(c, file, fn)) {
			continue
		}
		p.spans[fn] = name
	}
}

// spanName returns the name of the span for fn: its source name, and for a
// class method, the class's name, a dot, and the method's name.
func spanName(fn *ast.Node) string {
	name := sourceName(fn)
	if name != "" && fn.Kind == ast.KindMethodDeclaration && ast.IsClassLike(fn.Parent) {
		if class := declarationName(fn.Parent); class != "" {
			return class + "." + name
		}
	}
	return name
}

// isExportedFunction reports whether fn is a function declaration, or a
// function a top-level const initializes, that file exports, by an export
// modifier or an export declaration.
func isExportedFunction(c *checker.Checker, file *ast.SourceFile, fn *ast.Node) bool {
	declaration := fn
	if fn.Kind != ast.KindFunctionDeclaration {
		declaration = fn.Parent
		if declaration == nil || declaration.Kind != ast.KindVariableDeclaration || declaration.Initializer() != fn {
			return false
		}
	}
	symbol := c.GetSymbolOfDeclaration(declaration)
	module := file.AsNode().Symbol()
	if symbol == nil || module == nil {
		return false
	}
	for _, export := range c.GetExportsOfModule(module) {
		target := export
		if target.Flags()&ast.SymbolFlagsAlias != 0 {
			target = c.GetAliasedSymbol(target)
		}
		if c.GetMergedSymbol(target) == c.GetMergedSymbol(symbol) {
			return true
		}
	}
	return false
}

// wrapFunction returns `Effect.fnUntraced(generator)` in place of node, or
// `Effect.fn("name")(generator)` when node has a span.
func (tx *lowering) wrapFunction(node *ast.Node, generator *ast.Node) *ast.Node {
	name, traced := tx.plan.spans[node]
	if !traced {
		return tx.wrapGenerator("fnUntraced", node, generator)
	}
	f := tx.Factory()
	fn := f.NewCallExpression(
		f.NewPropertyAccessExpression(tx.effectName(), nil /*questionDotToken*/, f.NewIdentifier("fn"), ast.NodeFlagsNone),
		nil, /*questionDotToken*/
		nil, /*typeArguments*/
		f.NewNodeList([]*ast.Node{f.NewStringLiteral(name, ast.TokenFlagsNone)}),
		ast.NodeFlagsNone,
	)
	lowered := f.NewCallExpression(fn, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{generator}), ast.NodeFlagsNone)
	tx.EmitContext().SetOriginal(lowered, node)
	lowered.Loc = node.Loc
	return lowered
}

// traceMethod returns `Effect.withSpan(effect, "Class.method")` when node, a
// lowered method, has a span, and effect otherwise.
func (tx *lowering) traceMethod(node *ast.Node, effect *ast.Node) *ast.Node {
	name, traced := tx.plan.spans[node]
	if !traced {
		return effect
	}
	f := tx.Factory()
	return f.NewCallExpression(
		f.NewPropertyAccessExpression(tx.effectName(), nil /*questionDotToken*/, f.NewIdentifier("withSpan"), ast.NodeFlagsNone),
		nil, /*questionDotToken*/
		nil, /*typeArguments*/
		f.NewNodeList([]*ast.Node{effect, f.NewStringLiteral(name, ast.TokenFlagsNone)}),
		ast.NodeFlagsNone,
	)
}
