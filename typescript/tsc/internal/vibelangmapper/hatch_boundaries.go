package vibelangmapper

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// isHatchCall recognizes the lexical prelude hatches, so a shadowed sync or
// callback keeps ordinary call semantics (D148).
func (p *projector) isHatchCall(node *ast.Node) bool {
	if !ast.IsCallExpression(node) || !ast.IsIdentifier(node.Expression()) {
		return false
	}
	name := node.Expression().Text()
	return formsHatchNames[name] && (name == "try" || p.forms.(*Resolver).binding(node.Expression(), name) == nil)
}

// hatchSpelling records one boundary spelling in the shared plan. The body
// remains source TypeScript; only its surrounding call differs at runtime.
func (p *projector) hatchSpelling(start, end int, checking, executable string) {
	p.plan.edits = append(p.plan.edits, edit{pos: start, end: end, kind: editHatch, text: checking, arg: executable})
}

// hatchCall keeps the first argument entirely TypeScript (D232), while the
// mapper and inputs run in their enclosing Effect body. Explicit async
// functions use Effect's promise boundary, which supplies the AbortSignal
// after bound inputs; value expressions are deferred until the hatch runs.
func (p *projector) hatchCall(call *ast.Node, e env) {
	start := p.tokenStart(call)
	if e.effect {
		if p.statementStarts[start] {
			p.plan.insert(start, ";")
			delete(p.statementStarts, start)
		}
		p.plan.mark(editRun, partOpen, start, start, "")
	}
	name := call.Expression().Text()
	args := call.Arguments()
	if name == "try" && len(args) == 1 && ast.IsObjectLiteralExpression(ast.SkipParentheses(args[0])) {
		for _, property := range ast.SkipParentheses(args[0]).AsObjectLiteralExpression().Properties.Nodes {
			if property.Name() != nil && ast.IsIdentifier(property.Name()) && property.Name().Text() == "try" {
				p.diagnostics = append(p.diagnostics, Diagnostic{Start: p.tokenStart(args[0]), Length: args[0].End() - p.tokenStart(args[0]), Code: 4404, Message: "The try hatch takes code, then an optional error mapper; write try(() => code, (error: unknown) => mappedError), rather than Effect's options object."})
				break
			}
		}
	}
	calleeEdit := -1
	if name == "try" {
		calleeEdit = len(p.plan.edits)
		checking := helperRef("$hatchTry")
		// A written arrow fixes the mapper position (D231). Parameter
		// arity or uses of the input cannot reopen the callable-input overload.
		if len(args) > 1 && ast.IsFunctionExpressionOrArrowFunction(ast.SkipParentheses(args[1])) {
			checking = helperRef("$hatchTryMapped")
		}
		p.hatchSpelling(start, call.Expression().End(), checking, helperRef("$hatchTry"))
	}
	if name == "try" && len(args) > 1 {
		second := ast.SkipParentheses(args[1])
		if !ast.IsFunctionExpressionOrArrowFunction(second) && !p.hatchValueInput(second, map[*ast.Node]bool{}) {
			// An opaque function input and an error mapper occupy the same
			// position. Only an annotation or an explicit arrow can decide.
			p.diagnostics = append(p.diagnostics, Diagnostic{Start: p.tokenStart(second), Length: second.End() - p.tokenStart(second), Code: CodeUnprojectedForm, Message: "The mapper cannot decide whether this argument maps a failure or supplies an input. For a mapper, write (error: unknown) => mapError(error). For a scalar input, annotate the binding with its non-callable type (for example, const text: string = value), or assert it at the call as value as string. For an opaque or callable input, write try((input) => code(input), (error: unknown) => new Cause.UnknownError(error), value)."})
		}
	}
	if len(args) > 0 && name != "callback" {
		first := ast.SkipParentheses(args[0])
		function := ast.IsFunctionExpressionOrArrowFunction(first)
		if !function && !p.hatchValueInput(first, map[*ast.Node]bool{}) {
			p.diagnostics = append(p.diagnostics, Diagnostic{Start: p.tokenStart(first), Length: first.End() - p.tokenStart(first), Code: CodeUnprojectedForm, Message: "The mapper cannot decide whether this hatch argument is a function or a value. Write an annotated arrow that calls the function or returns the value."})
		}
		if !function {
			// Both renderings see a thunk: even a value that happens to be callable
			// must be returned, rather than selected as the evaluate overload.
			p.plan.insert(p.tokenStart(args[0]), "() => (")
			p.plan.insert(args[0].End(), ")")
		}
		if function && ast.GetFunctionFlags(first)&ast.FunctionFlagsAsync != 0 {
			mapper := false
			if name == "try" && len(args) > 1 {
				second := ast.SkipParentheses(args[1])
				mapper = ast.IsFunctionExpressionOrArrowFunction(second)
			}
			fn, inputs := p.freshWord("$evaluate"), p.freshWord("$inputs")
			params := fn + ", ..." + inputs
			body := helperRef("$promise") + "(" + fn + ".bind(undefined, ..." + inputs + "))"
			if name == "try" {
				body = helperRef("$tryPromise") + "(" + fn + ".bind(undefined, ..." + inputs + "))"
				if mapper {
					mapping := p.freshWord("$mapError")
					params = fn + ", " + mapping + ", ..." + inputs
					body = helperRef("$tryPromise") + "({ try: " + fn + ".bind(undefined, ..." + inputs + "), catch: " + mapping + " })"
				}
			}
			executable := "((" + params + ") => " + body + ")"
			if calleeEdit >= 0 {
				p.plan.edits[calleeEdit].arg = executable
			} else {
				p.hatchSpelling(start, call.Expression().End(), name, executable)
			}
		}
	}
	if len(args) > 0 {
		p.visit(args[0], env{})
	}
	// Type arguments belong to the hatch's signature, not to its plain body.
	p.visitList(call.TypeArguments(), env{})
	for _, arg := range args[min(1, len(args)):] {
		p.visit(arg, e)
	}
	if e.effect {
		p.plan.mark(editRun, partClose, call.End(), call.End(), "")
	}
}

// isRetiredCatchStatement recognizes the parser's synthetic catch chain;
// projecting its source spans as ordinary declarations would corrupt code.
func isRetiredCatchStatement(node *ast.Node) bool {
	if node.Kind != ast.KindTryStatement || node.Flags&ast.NodeFlagsVibeLang != 0 || node.AsTryStatement().CatchClause == nil {
		return false
	}
	v := node.AsTryStatement().CatchClause.AsCatchClause().VariableDeclaration
	return v != nil && (v.Type() != nil || v.Flags&ast.NodeFlagsReparsed != 0 || v.Name().Flags&ast.NodeFlagsReparsed != 0)
}

// retiredCatchStatement reports the approved L4 migration instead of
// assigning the fork's retired type-directed meaning to this clause.
func (p *projector) retiredCatchStatement(node *ast.Node) {
	clause := node.AsTryStatement().CatchClause
	p.diagnostics = append(p.diagnostics, Diagnostic{Start: p.tokenStart(clause), Length: len("catch"), Code: CodeUnprojectedForm, Message: "Typed catch clauses are deleted (L4, D216/D217). Catch the failure without an annotation and match on it; to recover a whole Cause, match on exit(program)."})
}

// hatchValueInput proves a non-callable input from its written type or
// initializer. An imported or unannotated opaque binding needs an annotation
// because try's second position is also the error mapper (D231).
func (p *projector) hatchValueInput(node *ast.Node, seen map[*ast.Node]bool) bool {
	if node == nil || seen[node] {
		return false
	}
	seen[node] = true
	node = ast.SkipParentheses(node)
	switch node.Kind {
	case ast.KindStringLiteral, ast.KindNumericLiteral, ast.KindTrueKeyword, ast.KindFalseKeyword, ast.KindNullKeyword,
		ast.KindNoSubstitutionTemplateLiteral, ast.KindTemplateExpression, ast.KindArrayLiteralExpression, ast.KindObjectLiteralExpression:
		return true
	case ast.KindAsExpression, ast.KindTypeAssertionExpression:
		if typ := node.Type(); typ != nil {
			return hatchScalarType(typ)
		}
	case ast.KindBinaryExpression:
		switch node.AsBinaryExpression().OperatorToken.Kind {
		case ast.KindPlusToken, ast.KindMinusToken, ast.KindAsteriskToken, ast.KindSlashToken, ast.KindPercentToken,
			ast.KindEqualsEqualsToken, ast.KindEqualsEqualsEqualsToken, ast.KindExclamationEqualsToken, ast.KindExclamationEqualsEqualsToken,
			ast.KindLessThanToken, ast.KindGreaterThanToken, ast.KindLessThanEqualsToken, ast.KindGreaterThanEqualsToken:
			return true
		}
	case ast.KindCallExpression:
		// These library operations return data, never an evaluate function.
		// Resolve their canonical export so aliases work and locals cannot
		// borrow the library's non-callable result guarantee.
		switch p.forms.(*Resolver).Callee(node).Canonical {
		case "effect/BigDecimal#fromNumberUnsafe", "effect/Schema#toJsonSchemaDocument",
			"effect/Schema#toCodecJson", "effect/Arbitrary#schema":
			return true
		}

		callee := node.Expression()
		if ast.IsIdentifier(callee) && (callee.Text() == "String" || callee.Text() == "Number" || callee.Text() == "Boolean") && p.forms.(*Resolver).binding(callee, callee.Text()) == nil {
			return true
		}
		if ast.IsPropertyAccessExpression(callee) && ast.IsIdentifier(callee.Expression()) {
			receiver := callee.Expression()
			if p.forms.(*Resolver).binding(receiver, receiver.Text()) == nil {
				if receiver.Text() == "Math" {
					switch callee.Name().Text() {
					case "max", "min", "abs", "ceil", "floor", "round", "trunc", "sqrt", "pow", "random", "sign", "sin", "cos", "tan", "log", "exp":
						return true
					}
				}
				if receiver.Text() == "Date" && callee.Name().Text() == "now" {
					return true
				}
				if receiver.Text() == "crypto" && callee.Name().Text() == "randomUUID" {
					return true
				}
			}
		}
	case ast.KindIdentifier:
		if declaration := p.forms.(*Resolver).binding(node, node.Text()); declaration != nil &&
			(declaration.Kind == ast.KindParameter || declaration.Kind == ast.KindVariableDeclaration) {
			if typ := declaration.Type(); typ != nil {
				switch typ.Kind {
				case ast.KindStringKeyword, ast.KindNumberKeyword, ast.KindBooleanKeyword, ast.KindBigIntKeyword, ast.KindLiteralType, ast.KindArrayType, ast.KindTupleType:
					return true
				}
			}
			return p.hatchValueInput(declaration.Initializer(), seen)
		}
	}
	return false
}

// isPlainHatchFunction keeps nested declarations plain as well as arrows;
// syntax extensions such as a pipe may still be projected in their bodies.
func (p *projector) isPlainHatchFunction(node *ast.Node) bool {
	if !ast.IsFunctionLike(node) {
		return false
	}
	_, hatch := p.forms.(*Resolver).inHatch(node)
	return hatch
}

// hatchScalarType recognizes annotations that exclude a callable value;
// arbitrary aliases and structural types require an explicit arrow.
func hatchScalarType(typ *ast.Node) bool {
	switch typ.Kind {
	case ast.KindStringKeyword, ast.KindNumberKeyword, ast.KindBooleanKeyword, ast.KindBigIntKeyword, ast.KindLiteralType, ast.KindArrayType, ast.KindTupleType:
		return true
	}
	return false
}
