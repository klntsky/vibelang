package vibelangmapper

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// FormDiagnostics resolves editor form choices on an existing, immutable AST.
// It builds no lowering plan, renders no virtual file, and reads no package
// metadata, so an open-file suggestion need not pay for a full projection.
func FormDiagnostics(file *ast.SourceFile) []Diagnostic {
	p := &projector{file: file, text: file.Text(), forms: newFormResolver(file), names: newNames(file, file.Text()), services: map[string]*ast.Node{}}
	for _, statement := range file.Statements.Nodes {
		if isServiceClass(statement) {
			p.services[statement.Name().Text()] = statement
		}
	}
	// callArgument tries pipe routing before the ordinary lambda resolver,
	// including through transparent expression wrappers.
	pipeLambdas := map[*ast.Node]bool{}
	var pipeArgument func(*ast.Node)
	pipeArgument = func(n *ast.Node) {
		if p.programPipeInput(n) {
			if isLambda(n) && len(n.Parameters()) == 0 {
				pipeLambdas[n] = true
				return
			}
			if formsHoldsCall(n) || ast.IsIdentifier(n) || ast.IsPropertyAccessExpression(n) {
				return
			}
		}
		switch n.Kind {
		case ast.KindParenthesizedExpression, ast.KindAsExpression, ast.KindSatisfiesExpression, ast.KindNonNullExpression:
			pipeArgument(n.Expression())
		}
	}
	var visit func(*ast.Node, bool)
	visit = func(n *ast.Node, effect bool) {
		bodyEffect := effect
		if ast.IsPropertyAccessExpression(n) && p.sourceDataMethodAccess(n) {
			p.checkDataMethodAccess(n)
			call := n.Parent
			if call != nil && ast.IsCallExpression(call) && call.Expression() == n && !ast.IsOptionalChain(n) && !ast.IsOptionalChain(call) && p.dataReceiver(n.Expression(), 0) == "" && dataMethodAmbiguous[n.Name().Text()] && !p.nativeDataMember(n.Expression(), n.Name().Text(), true, 0) {
				if r := p.forms.(*Resolver); r.Callee(call).Canonical == "" {
					p.dataMethodDiagnostic(n, "D253 cannot select a module for this unproven data method: its candidate TypeIds can coexist.")
				}
			}
		}
		if ast.IsCallExpression(n) {
			if block := blockArgument(n); block != nil {
				p.choice(p.forms.Block(n), block)
			} else if arguments := n.Arguments(); len(arguments) > 1 {
				pipeArgument(arguments[0])
			}
		}
		if p.isPlainHatchFunction(n) {
			bodyEffect = false
		} else if isLambda(n) {
			if pipeLambdas[n] {
				bodyEffect = true
			} else {
				decision := p.forms.Lambda(n)
				p.choice(decision, n)
				bodyEffect = decision.Form != FormPlain
			}
		} else if isProducer(n) {
			bodyEffect = true
		} else if isProjectedFunction(n) {
			bodyEffect = contractOf(n.Type()) != contractPlain
		} else if ast.IsArrowFunction(n) && n.Parent != nil && ast.IsCallExpression(n.Parent) && blockArgument(n.Parent) == n {
			bodyEffect = p.forms.Block(n.Parent).Form != FormPlain
		} else if ast.IsMethodDeclaration(n) && ast.IsClassLike(n.Parent) {
			if p.names == nil {
				p.names = newNames(file, file.Text())
				p.services = map[string]*ast.Node{}
				for _, statement := range file.Statements.Nodes {
					if isServiceClass(statement) {
						p.services[statement.Name().Text()] = statement
					}
				}
			}
			bodyEffect = p.isEffectMethod(n)
		} else if ast.IsFunctionLike(n) && n.Body() != nil {
			bodyEffect = false
		}

		if helper := awaitOperationHelper(n); helper != "" {
			if !effect {
				p.awaitChoice(n, "An await operation must run in an Effect body; move it into an Effect function with an explicit contract.")
				return
			}
			p.awaitFormChoices(n.Arguments()[0], helper == "$awaitRace" || helper == "$awaitAny")
		}
		n.ForEachChild(func(child *ast.Node) bool {
			childEffect := effect
			if n.Body() != nil {
				childEffect = false
				if child == n.Body() {
					childEffect = bodyEffect
				}
			}
			visit(child, childEffect)
			return false
		})
	}
	file.AsNode().ForEachChild(func(n *ast.Node) bool { visit(n, false); return false })
	return p.diagnostics
}

// awaitFormChoices checks only the outer container, reusing the projector's
// lexical classifiers without rendering its calls or traversing bodies twice.
func (p *projector) awaitFormChoices(operand *ast.Node, race bool) {
	switch operand.Kind {
	case ast.KindParenthesizedExpression, ast.KindAsExpression, ast.KindSatisfiesExpression, ast.KindNonNullExpression:
		p.awaitFormChoices(operand.Expression(), race)
	case ast.KindArrayLiteralExpression:
		for _, element := range operand.Elements() {
			if element.Kind == ast.KindSpreadElement {
				p.awaitChoice(element, "Annotate the spread operand as a collection of Effect, Fiber, or Deferred values and pass that collection directly to the await operation.")
			} else {
				p.awaitElementFormChoice(element)
			}
		}
	case ast.KindObjectLiteralExpression:
		if race {
			p.awaitChoice(operand, "await.race and await.any require an iterable, not an object; use an array of programs.")
		}
		for _, property := range operand.Properties() {
			switch property.Kind {
			case ast.KindPropertyAssignment:
				p.awaitElementFormChoice(property.Initializer())
			case ast.KindShorthandPropertyAssignment:
				p.awaitElementFormChoice(property.Name())
			default:
				p.awaitChoice(property, "Write an explicit property value with an Effect, Fiber, or Deferred annotation for this await operand.")
			}
		}
	default:
		if !p.awaitKnownCollection(operand) {
			p.awaitChoice(operand, "Annotate the await operand as a collection of Effect, Fiber, or Deferred values, or write an array or object literal of programs.")
		}
	}
}

// awaitElementFormChoice shares the exact await classifiers with emit; held
// values and call expressions still need annotation when text cannot decide.
func (p *projector) awaitElementFormChoice(element *ast.Node) {
	if isLambda(element) {
		return
	}
	switch element.Kind {
	case ast.KindParenthesizedExpression, ast.KindAsExpression, ast.KindSatisfiesExpression, ast.KindNonNullExpression:
		p.awaitElementFormChoice(element.Expression())
		return
	}
	if formsHoldsCall(element) {
		if _, known := p.awaitCalls(element); !known {
			p.awaitChoice(element, "The mapper cannot decide this await element's form from text. Bind it to a value with an explicit Effect, Fiber, or Deferred annotation, or write an explicitly annotated program lambda.")
		}
	} else if !p.awaitKnownValue(element, 0) {
		p.awaitChoice(element, "The mapper cannot decide this await element's form from text. Add an Effect, Fiber, or Deferred annotation, or an explicit parameterless Effect function annotation.")
	}
}

// sourceDataMethodAccess excludes synthesized pattern and handler scaffolding,
// which the projector lowers as a unit rather than as data-method expressions.
func (p *projector) sourceDataMethodAccess(n *ast.Node) bool {
	for parent := n; parent != nil; parent = parent.Parent {
		if parent.Kind == ast.KindTryStatement && parent.Flags&ast.NodeFlagsVibeLang != 0 {
			if groups, ok := handleClauses(parent); ok {
				for _, group := range groups {
					for _, clause := range group.clauses {
						if clause.operation == n {
							return false
						}
					}
				}
			}
		}
		if parent.Flags&ast.NodeFlagsReparsed != 0 || p.isMatchForm(parent) {
			return false
		}
	}
	return true
}
