package vibelangmapper

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// S5-1: only a whole return at an Effect contract delegates subtype data.
// Unannotated declarations have no Effect expectation. Lambda form resolution
// supplies the contextual expectation without consulting another file's types.
func (p *projector) programReturnBoundary(node *ast.Node) bool {
	if ast.IsParenthesizedExpression(node) || node.Flags&ast.NodeFlagsReparsed != 0 {
		return false
	}
	value := node
	for value.Parent != nil && ast.IsParenthesizedExpression(value.Parent) {
		value = value.Parent
	}
	parent := value.Parent
	if parent == nil {
		return false
	}
	var fn *ast.Node
	if parent.Kind == ast.KindArrowFunction && parent.Body() == value {
		fn = parent
	} else if parent.Kind == ast.KindReturnStatement && parent.Expression() == value {
		for fn = parent.Parent; fn != nil && !ast.IsFunctionLike(fn); fn = fn.Parent {
		}
	}
	if fn == nil || isProducer(fn) || fn.Flags&ast.NodeFlagsReparsed != 0 || !p.structuredReturnValue(node, map[*ast.Node]bool{}) {
		return false
	}
	// S5-1 runs structured returns; D269 factory bodies return programs cold.
	if p.forms.(*Resolver).isFactory(fn) {
		return false
	}
	if fn.Type() != nil {
		return contractOf(fn.Type()) == contractEffect || contractOf(fn.Type()) == contractInfer
	}
	if isLambda(fn) {
		d := p.forms.Lambda(fn)
		return d.Choice == nil && !d.Ambiguous && d.Provenance != FromFallback && d.Class&ClassPlainFn == 0 &&
			(d.Form == FormEffectFn || d.Form == FormProgram || d.Form == FormDual)
	}
	return false
}

// D24 constructor evidence follows immutable local bindings only. Unknown
// host calls, mutable values, exact Effect calls and plain expressions cannot
// establish a subtype's static type in a per-file projection.
func (p *projector) structuredReturnValue(node *ast.Node, seen map[*ast.Node]bool) bool {
	node = ast.SkipParentheses(node)
	if node == nil || seen[node] {
		return false
	}
	seen[node] = true
	if node.Kind == ast.KindCallExpression {
		return p.structuredCall(node)
	}
	if !ast.IsIdentifier(node) {
		return false
	}
	r := p.forms.(*Resolver)
	decl := r.binding(node, node.Text())
	if decl == nil || decl.Kind != ast.KindVariableDeclaration || decl.Type() != nil || decl.Parent == nil || decl.Parent.Flags&ast.NodeFlagsConst == 0 || decl.Initializer() == nil {
		return false
	}
	return p.structuredReturnValue(decl.Initializer(), seen)
}

func (p *projector) returnProgram(node *ast.Node, e env) {
	p.checkSubtypeReturnFailure(node)
	start := p.tokenStart(node)
	p.plan.mark(editRun, partOpen, start, start, "return")
	if node.Kind == ast.KindCallExpression {
		// Run the raw returned program once. Its success may itself be D24 data.
		p.moduleCall(node, e)
	} else {
		p.visitChildren(node, e)
	}
	p.plan.mark(editRun, partClose, node.End(), node.End(), "return")
}

// S5-1: a direct Exit.fail literal cannot satisfy a written never error
// channel. Nested regions and opaque host return rows need native flow/types.
func (p *projector) checkSubtypeReturnFailure(node *ast.Node) {
	if node.Kind != ast.KindCallExpression || p.forms.(*Resolver).Callee(node).Canonical != "effect/Exit#fail" || len(node.Arguments()) != 1 {
		return
	}
	value := node
	for value.Parent != nil && ast.IsParenthesizedExpression(value.Parent) {
		value = value.Parent
	}
	ret := value.Parent
	if ret == nil || ret.Kind != ast.KindReturnStatement || ret.Parent == nil || ret.Parent.Kind != ast.KindBlock {
		return
	}
	fn := ret.Parent.Parent
	if fn == nil || !ast.IsFunctionLike(fn) || fn.Body() != ret.Parent || fn.Type() == nil || fn.Name() == nil {
		return
	}
	args := fn.Type().TypeArguments()
	if len(args) != 3 || args[1].Kind != ast.KindNeverKeyword {
		return
	}
	failure := node.Arguments()[0]
	if failure.Kind != ast.KindStringLiteral {
		return
	}
	p.fiberDiagnostic(node, 2001, "string can escape "+fn.Name().Text()+", but its contract says E = never.")
}
