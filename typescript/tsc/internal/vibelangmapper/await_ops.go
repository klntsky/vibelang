package vibelangmapper

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// awaitOperationHelper recognizes only the parser's D196 calls, so a
// source function named $awaitAll keeps ordinary call semantics.
func awaitOperationHelper(node *ast.Node) string {
	if node.Kind != ast.KindCallExpression || len(node.Arguments()) != 1 {
		return ""
	}
	callee := node.Expression()
	if !ast.IsIdentifier(callee) || callee.Flags&ast.NodeFlagsReparsed == 0 {
		return ""
	}
	switch callee.Text() {
	case "$awaitAll", "$awaitAllSettled", "$awaitRace", "$awaitAny":
		return callee.Text()
	}
	return ""
}

// awaitOperation lowers an await operation to its runtime helper, running
// the helper where the operation stands and passing each literal element
// by name. The existing run and program edits give both renderings the
// same fiber boundaries, including executable generators' temporaries.
func (p *projector) awaitOperation(call *ast.Node, e env) {
	if !e.effect {
		p.awaitChoice(call, "An await operation must run in an Effect body; move it into an Effect function with an explicit contract.")
		return
	}
	start := p.tokenStart(call)
	if p.statementStarts[start] {
		p.plan.insert(start, ";")
		delete(p.statementStarts, start)
	}
	operand := call.Arguments()[0]
	p.plan.mark(editRun, partOpen, start, start, "")
	p.plan.replace(start, p.tokenStart(operand), helperRef(awaitOperationHelper(call))+"(")
	p.awaitOperand(operand, e, awaitOperationHelper(call) == "$awaitRace" || awaitOperationHelper(call) == "$awaitAny")
	p.plan.insert(call.End(), ")")
	p.plan.mark(editRun, partClose, call.End(), call.End(), "")
}

// awaitOperand walks the operand's outer container only: calls in keys
// run at the operation, while calls in element programs run in fibers.
// Nonliteral containers already hold values; they must have a known form
// rather than silently treating an unknown collection of functions as data.
func (p *projector) awaitOperand(operand *ast.Node, e env, race bool) {
	switch operand.Kind {
	case ast.KindParenthesizedExpression, ast.KindAsExpression, ast.KindSatisfiesExpression, ast.KindNonNullExpression:
		p.awaitOperand(operand.Expression(), e, race)
		if operand.Type() != nil {
			p.visit(operand.Type(), env{})
		}
	case ast.KindArrayLiteralExpression:
		for _, element := range operand.Elements() {
			if element.Kind == ast.KindSpreadElement {
				p.awaitChoice(element, "Annotate the spread operand as a collection of Effect, Fiber, or Deferred values and pass that collection directly to the await operation.")
				p.visit(element, e)
			} else {
				p.awaitElement(element, e)
			}
		}
	case ast.KindObjectLiteralExpression:
		if race {
			p.awaitChoice(operand, "await.race and await.any require an iterable, not an object; use an array of programs.")
		}
		for _, property := range operand.Properties() {
			switch property.Kind {
			case ast.KindPropertyAssignment:
				p.visit(property.Name(), e)
				p.awaitElement(property.Initializer(), e)
			case ast.KindShorthandPropertyAssignment:
				name := property.Name()
				p.plan.insert(p.tokenStart(name), name.Text()+": ")
				p.awaitElement(name, e)
			default:
				p.awaitChoice(property, "Write an explicit property value with an Effect, Fiber, or Deferred annotation for this await operand.")
				p.visit(property, e)
			}
		}
	default:
		// D49 also converts collections of functions to programs. Until a
		// collection's elements can be classified, require explicit values
		// instead of applying a speculative conversion to the whole object.
		if !p.awaitKnownCollection(operand) {
			p.awaitChoice(operand, "Annotate the await operand as a collection of Effect, Fiber, or Deferred values, or write an array or object literal of programs.")
		}
		p.visit(operand, e)
	}
}

// awaitElement passes a lambda as a program, defers an expression that
// runs calls, and preserves held values. $asProgram converts a named
// parameterless Effect function without restarting a Fiber or Deferred.
func (p *projector) awaitElement(element *ast.Node, e env) {
	if isLambda(element) {
		p.lambda(element, e)
		return
	}
	if element.Kind == ast.KindParenthesizedExpression || element.Kind == ast.KindAsExpression ||
		element.Kind == ast.KindSatisfiesExpression || element.Kind == ast.KindNonNullExpression {
		p.awaitElement(element.Expression(), e)
		if element.Type() != nil {
			p.visit(element.Type(), env{})
		}
		return
	}
	if formsHoldsCall(element) {
		pauses, known := p.awaitCalls(element)
		if !known {
			p.awaitChoice(element, "The mapper cannot decide this await element's form from text. Bind it to a value with an explicit Effect, Fiber, or Deferred annotation, or write an explicitly annotated program lambda.")
		}
		if pauses {
			arg := "expr " + selfArg(element)
			start := p.tokenStart(element)
			p.plan.mark(editProgram, partOpen, start, start, arg)
			p.visit(element, expressionEnv())
			p.plan.mark(editProgram, partClose, element.End(), element.End(), arg)
			return
		}
		p.visit(element, e)
		return
	}
	if !p.awaitKnownValue(element, 0) {
		p.awaitChoice(element, "The mapper cannot decide this await element's form from text. Add an Effect, Fiber, or Deferred annotation, or an explicit parameterless Effect function annotation.")
	}
	if !p.programValue(element, Decision{Class: ClassProgram}, e) {
		p.visit(element, e)
	}
}

// awaitCalls checks whether evaluating an expression runs an Effect and
// whether every call's form is known. A do/match or catch expression and
// nested await operations run by construction; function bodies run later.
func (p *projector) awaitCalls(expression *ast.Node) (pauses, known bool) {
	known = true
	r := p.forms.(*Resolver)
	var visit func(*ast.Node) bool
	visit = func(node *ast.Node) bool {
		if formsBoundary(node) {
			return false
		}
		if ast.IsCallExpression(node) {
			if isCatchExpression(node) || p.isMatchForm(node) || awaitOperationHelper(node) != "" || r.returnsEffect(node) {
				pauses = true
				return false
			}
			c := r.Callee(node)
			if c.Kind == CalleeLocal {
				if root, chain, ok := formsNameChain(node.Expression()); ok && len(chain) == 0 {
					if d := r.binding(root, root.Text()); d != nil && d.Kind == ast.KindVariableDeclaration {
						if init := d.Initializer(); init != nil && isLambda(ast.SkipParentheses(init)) {
							decision := p.forms.Lambda(ast.SkipParentheses(init))
							known = known && decision.Choice == nil && !decision.Ambiguous
							pauses = pauses || decision.Form != FormPlain
							return node.ForEachChild(visit)
						}
						if t := formsUnwrapType(d.Type()); t != nil && t.Kind == ast.KindFunctionType {
							pauses = pauses || r.isEffectTypeNode(t.Type(), 0)
							return node.ForEachChild(visit)
						}
					}
				}
			}
			sigs, _ := r.signaturesOf(c)
			candidates := formsCandidates(sigs, len(node.Arguments()))
			if len(candidates) == 0 {
				// Parameters with written function types have no Callee key.
				callee := node.Expression()
				if ast.IsIdentifier(callee) {
					if d := r.binding(callee, callee.Text()); d != nil && d.Type() != nil {
						t := formsUnwrapType(d.Type())
						if t.Kind == ast.KindFunctionType {
							pauses = pauses || r.isEffectTypeNode(t.Type(), 0)
							return node.ForEachChild(visit)
						}
					}
				}
				known = false
			} else {
				for _, sig := range candidates {
					isEffect := sig.Result == "effect/Effect"
					if isEffect != (candidates[0].Result == "effect/Effect") {
						known = false
					}
					pauses = pauses || isEffect
				}
			}
		}
		return node.ForEachChild(visit)
	}
	visit(expression)
	return
}

// awaitKnownValue follows lexical bindings and written annotations only,
// so an imported value or an unconstrained parameter cannot silently take
// the function-to-program conversion's fallback form.
func (p *projector) awaitKnownValue(value *ast.Node, depth int) bool {
	if depth > 8 {
		return false
	}
	r := p.forms.(*Resolver)
	value = ast.SkipParentheses(value)
	if ast.IsIdentifier(value) {
		if d := r.binding(value, value.Text()); d != nil {
			if t := d.Type(); t != nil {
				for i := 0; i < 8; i++ {
					t = formsUnwrapType(t)
					if t.Kind == ast.KindTypeReference {
						if alias := r.localType(t); alias != nil && alias.Kind == ast.KindTypeAliasDeclaration {
							t = alias.Type()
							continue
						}
					}
					return t.Kind != ast.KindAnyKeyword && t.Kind != ast.KindUnknownKeyword && t.Kind != ast.KindUnionType
				}
				return false
			}
			if d.Kind == ast.KindFunctionDeclaration {
				return true
			}
			if d.Kind == ast.KindVariableDeclaration || d.Kind == ast.KindParameter {
				if init := d.Initializer(); init != nil {
					return p.awaitKnownValue(init, depth+1)
				}
			}
		}
		return r.receiverModule(value, 0) != ""
	}
	if isLambda(value) {
		return !p.forms.Lambda(value).Ambiguous
	}
	if ast.IsCallExpression(value) {
		_, known := p.awaitCalls(value)
		return known
	}
	if ast.IsPropertyAccessExpression(value) {
		return r.receiverModule(value, 0) != ""
	}
	return ast.IsLiteralKind(value.Kind)
}

// awaitKnownCollection accepts written collections whose elements already
// are runtime await values. Collections of functions need D49 conversion,
// so they draw a diagnostic until that conversion is planned explicitly.
func (p *projector) awaitKnownCollection(value *ast.Node) bool {
	if !ast.IsIdentifier(value) {
		return false
	}
	r := p.forms.(*Resolver)
	d := r.binding(value, value.Text())
	if d == nil || d.Type() == nil {
		return false
	}
	var valid func(*ast.Node, int) bool
	valid = func(t *ast.Node, depth int) bool {
		t = formsUnwrapType(t)
		if t == nil || depth > 8 || t.Kind == ast.KindAnyKeyword || t.Kind == ast.KindUnknownKeyword || t.Kind == ast.KindFunctionType {
			return false
		}
		if r.isEffectTypeNode(t, 0) || r.typeModuleOf(t) == "effect/Fiber" || r.typeModuleOf(t) == "effect/Deferred" {
			return true
		}
		if elem := formsElementType(t); elem != nil && elem != t {
			return valid(elem, depth+1)
		}
		if t.Kind == ast.KindTypeReference {
			name := t.AsTypeReferenceNode().TypeName
			if ast.IsIdentifier(name) && name.Text() == "Record" && len(t.TypeArguments()) == 2 && r.binding(name, name.Text()) == nil {
				return valid(t.TypeArguments()[1], depth+1)
			}
			if alias := r.localType(t); alias != nil {
				if alias.Kind == ast.KindTypeAliasDeclaration {
					return valid(alias.Type(), depth+1)
				}
				return valid(alias, depth+1)
			}
		}
		if t.Kind == ast.KindTypeLiteral || t.Kind == ast.KindInterfaceDeclaration {
			for _, member := range t.Members() {
				if member.Kind != ast.KindPropertySignature && member.Kind != ast.KindIndexSignature || !valid(member.Type(), depth+1) {
					return false
				}
			}
			return true
		}
		return false
	}
	return valid(d.Type(), 0)
}

// awaitChoice asks for source evidence whenever a text-only await lowering
// cannot decide a form, preserving the mapper's no-silent-wrong rule.
func (p *projector) awaitChoice(at *ast.Node, message string) {
	start := p.tokenStart(at)
	p.diagnostics = append(p.diagnostics, Diagnostic{Start: start, Length: max(0, at.End()-start), Code: CodeFormChoice, Message: message})
}
