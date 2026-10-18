package vibelangmapper

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// isFiberHelper recognizes parser-generated calls without claiming written
// calls whose names happen to match the fiber helpers.
func isFiberHelper(node *ast.Node, name string, arity int) bool {
	return ast.IsCallExpression(node) && len(node.Arguments()) == arity &&
		ast.IsIdentifier(node.Expression()) && node.Expression().Flags&ast.NodeFlagsReparsed != 0 && node.Expression().Text() == name
}

// join runs the runtime's shape-preserving join at the source position.
// Validation is deliberately textual: unknown values need an annotation,
// since $join's never-valued fallback cannot diagnose an invalid operand.
func (p *projector) join(call *ast.Node, e env) {
	if _, hatch := p.forms.(*Resolver).inHatch(call); hatch {
		p.fiberDiagnostic(call, 4612, "join is VibeLang syntax, but a hatch body is plain TypeScript. Move the join out of the hatch.")
		return
	}
	if !e.effect {
		p.awaitChoice(call, "A join must run in an Effect body; move it into an Effect function with an explicit contract.")
		return
	}
	operand := call.Arguments()[0]
	valid, known := p.joinValue(operand, 0, 0)
	if !known {
		p.awaitChoice(operand, "The mapper cannot determine whether this join operand holds fibers. Add an explicit Fiber annotation, or an array or object annotation with Fiber elements.")
	} else if !valid {
		p.fiberDiagnostic(operand, 4611, "join takes a Fiber or a one-level array or object of fibers. Annotate the operand with Fiber elements; nest joins for deeper collections.")
	}
	start := p.tokenStart(call)
	if p.statementStarts[start] {
		p.plan.insert(start, ";")
		delete(p.statementStarts, start)
	}
	p.plan.mark(editRun, partOpen, start, start, "")
	p.plan.replace(start, p.tokenStart(operand), helperRef("$join")+"(")
	p.visit(operand, e)
	p.plan.insert(call.End(), ")")
	p.plan.mark(editRun, partClose, call.End(), call.End(), "")
}

// fiberDiagnostic reports a fiber rule at its source span in both renderings.
func (p *projector) fiberDiagnostic(at *ast.Node, code int32, message string) {
	start := p.tokenStart(at)
	p.diagnostics = append(p.diagnostics, Diagnostic{Start: start, Length: max(0, at.End()-start), Code: code, Message: message})
}

// joinValue follows lexical initializers and annotations, allowing containers
// only at the outer level (D228). Recursion through aliases is bounded.
func (p *projector) joinValue(n *ast.Node, level, depth int) (valid, known bool) {
	if n == nil || depth > 16 {
		return false, false
	}
	r := p.forms.(*Resolver)
	n = ast.SkipParentheses(n)
	switch n.Kind {
	case ast.KindAsExpression, ast.KindSatisfiesExpression:
		return p.joinType(n.Type(), level, depth+1)
	case ast.KindNonNullExpression:
		return p.joinValue(n.Expression(), level, depth+1)
	case ast.KindIdentifier:
		if d := r.binding(n, n.Text()); d != nil {
			if d.Type() != nil {
				return p.joinType(d.Type(), level, depth+1)
			}
			if ast.IsIdentifier(d.Name()) && d.Name().Text() == n.Text() {
				return p.joinValue(d.Initializer(), level, depth+1)
			}
		}
		return false, false
	case ast.KindArrayLiteralExpression, ast.KindObjectLiteralExpression:
		if level > 0 {
			return false, true
		}
		var elements []*ast.Node
		if n.Kind == ast.KindObjectLiteralExpression {
			elements = n.Properties()
		} else {
			elements = n.Elements()
		}
		valid, known = n.Kind != ast.KindObjectLiteralExpression || len(elements) > 0, true
		for _, element := range elements {
			v := element
			if element.Kind == ast.KindPropertyAssignment {
				v = element.Initializer()
			}
			if element.Kind == ast.KindShorthandPropertyAssignment {
				v = element.Name()
			}
			if element.Kind == ast.KindSpreadElement || element.Kind == ast.KindSpreadAssignment {
				return false, false
			}
			ok, certain := p.joinValue(v, level+1, depth+1)
			valid, known = valid && ok, known && certain
		}
		return
	case ast.KindCallExpression:
		if r.receiverModule(n, 0) == "effect/Fiber" {
			return true, true
		}
		if ast.IsIdentifier(n.Expression()) {
			name := n.Expression().Text()
			if r.binding(n.Expression(), name) == nil && (name == "fork" || name == "detach" || name == "forkIn") {
				return true, true
			}
			if d := r.binding(n.Expression(), name); d != nil && ast.IsFunctionLike(d) && d.Type() != nil {
				t := formsUnwrapType(d.Type())
				if r.isEffectTypeNode(t, 0) {
					if len(t.TypeArguments()) == 0 {
						return false, false
					}
					t = t.TypeArguments()[0]
				}
				return p.joinType(t, level, depth+1)
			}
		}
		return false, false
	}
	if ast.IsLiteralKind(n.Kind) || n.Kind == ast.KindTrueKeyword || n.Kind == ast.KindFalseKeyword || n.Kind == ast.KindNullKeyword || isLambda(n) {
		return false, true
	}
	return false, false
}

// joinType validates written Fiber leaves, unions, and one outer container;
// any, unknown, unresolved names, and generic shapes require source evidence.
func (p *projector) joinType(t *ast.Node, level, depth int) (valid, known bool) {
	t = formsUnwrapType(t)
	if t == nil || depth > 16 {
		return false, false
	}
	r := p.forms.(*Resolver)
	if r.typeModuleOf(t) == "effect/Fiber" {
		return true, true
	}
	if t.Kind == ast.KindAnyKeyword || t.Kind == ast.KindUnknownKeyword {
		return false, false
	}
	if t.Kind == ast.KindUnionType {
		valid, known = true, true
		for _, member := range t.AsUnionTypeNode().Types.Nodes {
			v, k := p.joinType(member, level, depth+1)
			valid, known = valid && v, known && k
		}
		return
	}
	if t.Kind == ast.KindTypeReference {
		if alias := r.localType(t); alias != nil {
			if alias.Kind == ast.KindTypeAliasDeclaration {
				return p.joinType(alias.Type(), level, depth+1)
			}
			t = alias
		}
	}
	if level == 0 {
		if elem := formsElementType(t); elem != nil {
			return p.joinType(elem, 1, depth+1)
		}
		if t.Kind == ast.KindTupleType {
			valid, known = true, true
			for _, elem := range t.AsTupleTypeNode().Elements.Nodes {
				v, k := p.joinType(elem, 1, depth+1)
				valid, known = valid && v, known && k
			}
			return
		}
		if t.Kind == ast.KindTypeLiteral || t.Kind == ast.KindInterfaceDeclaration {
			valid, known = len(t.Members()) > 0, true
			for _, member := range t.Members() {
				v, k := p.joinType(member.Type(), 1, depth+1)
				valid, known = valid && v, known && k
			}
			return
		}
	}
	if t.Kind == ast.KindTypeReference && r.typeModuleOf(t) == "" {
		return false, false
	}
	return false, true
}
