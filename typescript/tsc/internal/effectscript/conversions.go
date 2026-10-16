package effectscript

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// A function value, as opposed to a lambda written in place, is passed in the
// form its position expects (Chapter 13, "Function values"):
//
//	Effect.retry(flaky, policy)            // flaky: () => Effect<A, E, R>    -> Effect.suspend(flaky)
//	Effect.all(stores.map(makeSync))       // Array<() => Effect<A, E, R>>    -> $programs(...)
//	ids.map(findName)                      // findName can pause              -> ES4301
//
// Where the overload candidate under check declares an Effect, a function
// with no parameters that returns an Effect converts to a program (D7). Where
// it declares an iterable or a record of Effects, an array or record of such
// functions converts element by element (D49). Where it declares a plain
// function, an EffectScript function that can pause has no plain form, so the
// candidate rejects it, as it rejects such a lambda (D9, D56). As with
// lambdas, each candidate decides once, and the candidate overload resolution
// selects decides what the value compiles to.
type conversion uint8

const (
	convertNone     conversion = iota
	convertProgram             // Effect.suspend(value)
	convertPrograms            // $programs(value), an array or record of programs
	convertRejected            // a function that can pause where a plain function is expected
)

// valueState records, for one argument expression, the conversion each
// candidate asked for.
type valueState struct {
	byCandidate map[candidateKey]conversion
	types       map[candidateKey]*checker.Type
}

// expressionType gives an argument expression the type of the conversion the
// candidate under check asks for.
func expressionType(c *checker.Checker, node *ast.Node, t *checker.Type) *checker.Type {
	if !mayConvert(c, node, t) {
		return t
	}
	call, path := ownerCall(node)
	if call == nil || !isEffectScriptFile(ast.GetSourceFileOfNode(node)) {
		return t
	}
	a := analyzerFor(c)
	signature := a.candidateOf(call)
	if signature == nil {
		return t
	}
	key := keyOf(signature)
	s := a.conversions[node]
	if s == nil {
		s = &valueState{byCandidate: map[candidateKey]conversion{}, types: map[candidateKey]*checker.Type{}}
		a.conversions[node] = s
	}
	if conv, ok := s.byCandidate[key]; ok {
		if conv == convertProgram || conv == convertPrograms {
			return s.types[key]
		}
		return t
	}
	conv, converted := a.convert(node, t, a.classifyValuePosition(call, signature, path, node, t))
	s.byCandidate[key] = conv
	s.types[key] = converted
	if conv == convertRejected {
		a.reject(call, key)
	}
	return converted
}

// mayConvert is the cheap test that runs on every expression: node is an
// argument, an array element, or a property value, its type is a function or
// an object, and it is not a lambda, whose form forms.go decides.
func mayConvert(c *checker.Checker, node *ast.Node, t *checker.Type) bool {
	switch node.Kind {
	case ast.KindArrowFunction, ast.KindFunctionExpression, ast.KindParenthesizedExpression:
		return false
	}
	parent := node.Parent
	if parent == nil || t == nil || t.Flags()&(checker.TypeFlagsObject|checker.TypeFlagsUnion|checker.TypeFlagsIntersection) == 0 {
		return false
	}
	switch parent.Kind {
	case ast.KindCallExpression, ast.KindNewExpression:
		return parent.Expression() != node
	case ast.KindArrayLiteralExpression, ast.KindParenthesizedExpression:
		return true
	case ast.KindPropertyAssignment:
		return parent.Initializer() == node
	}
	return false
}

// valuePosition is what an argument position expects of a value.
type valuePosition struct {
	expectation             // the value itself
	elements    expectation // the elements or property values of a container
}

func (a *analyzer) classifyValuePosition(call *ast.Node, signature *checker.Signature, path []pathStep, node *ast.Node, t *checker.Type) valuePosition {
	expected := a.expectedAt(call, signature, path, node)
	position := valuePosition{expectation: a.classify(expected)}
	if !position.any() {
		elements := a.stepInto(expected, pathStep{element: true})
		elements = append(elements, a.stepInto(expected, pathStep{property: "\x00"})...) // string index signatures
		position.elements = a.classify(elements)
	}
	return position
}

// convert decides what value of type t converts to at a position.
func (a *analyzer) convert(node *ast.Node, t *checker.Type, position valuePosition) (conversion, *checker.Type) {
	switch {
	case position.program && !(position.effectFn || position.plainFn):
		if effect := a.programOf(t); effect != nil {
			return convertProgram, effect
		}
	case position.program:
		// A function or an Effect (D39): a function with no parameters that
		// returns an Effect can pause, so it converts to a program.
		if effect := a.programOf(t); effect != nil {
			return convertProgram, effect
		}
	case position.plainFn && !position.effectFn:
		if fn := esFunctionOf(a.checker, node); fn != nil && a.isEffect(fn) {
			return convertRejected, t
		}
	case position.elements.program:
		if converted := a.programsOf(t); converted != nil {
			return convertPrograms, converted
		}
	}
	return convertNone, t
}

// programOf returns the Effect that t, a function with no required parameters
// that returns an Effect, converts to, or nil.
func (a *analyzer) programOf(t *checker.Type) *checker.Type {
	signatures := a.checker.GetSignaturesOfType(t, checker.SignatureKindCall)
	if len(signatures) != 1 || signatures[0].MinArgumentCount() != 0 || len(signatures[0].TypeParameters()) != 0 {
		return nil
	}
	if returned := a.checker.GetReturnTypeOfSignature(signatures[0]); isEffectType(a.checker, returned) {
		return returned
	}
	return nil
}

// programsOf returns the array or record of Effects that t, an array or
// record of programs, converts to, or nil.
func (a *analyzer) programsOf(t *checker.Type) *checker.Type {
	if a.checker.GetSignaturesOfType(t, checker.SignatureKindCall) != nil {
		return nil
	}
	if elements := a.elementTypes(t, 0); len(elements) > 0 && a.checker.IsArrayLikeType(t) {
		var effects []*checker.Type
		for _, element := range elements {
			effect := a.programOf(element)
			if effect == nil {
				return nil
			}
			effects = append(effects, effect)
		}
		return a.checker.CreateArrayType(a.checker.GetUnionType(effects), false)
	}
	ok := true
	converted := a.checker.MapObjectType(t, func(value *checker.Type) *checker.Type {
		if effect := a.programOf(value); effect != nil {
			return effect
		}
		ok = false
		return value
	})
	if !ok || len(a.checker.GetPropertiesOfType(t)) == 0 && len(a.checker.GetIndexInfosOfType(t)) == 0 {
		return nil
	}
	return converted
}

// finalConversion is what node compiles to: the conversion the candidate its
// call selected asked for.
func (a *analyzer) finalConversion(node *ast.Node) conversion {
	s := a.conversions[node]
	if s == nil {
		return convertNone
	}
	call, _ := ownerCall(node)
	if call == nil {
		return convertNone
	}
	if signature := a.candidateOf(call); signature != nil {
		return s.byCandidate[keyOf(signature)]
	}
	return convertNone
}

// esFunctionOf returns the .es function that node, an identifier, names: a
// function declaration, or a variable initialized with a lambda.
func esFunctionOf(c *checker.Checker, node *ast.Node) *ast.Node {
	if !ast.IsIdentifier(node) {
		return nil
	}
	symbol := c.GetResolvedSymbol(node)
	if symbol == nil {
		return nil
	}
	if symbol.Flags()&ast.SymbolFlagsAlias != 0 {
		symbol = c.GetAliasedSymbol(symbol)
	}
	declaration := symbol.ValueDeclaration()
	if declaration == nil {
		return nil
	}
	if ast.IsVariableDeclaration(declaration) && declaration.Initializer() != nil {
		declaration = ast.SkipParentheses(declaration.Initializer())
	}
	if isLowerable(declaration) {
		return implementationOf(declaration)
	}
	return nil
}
