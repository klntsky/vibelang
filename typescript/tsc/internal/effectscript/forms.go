package effectscript

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// A lambda, an arrow function or function expression in an .es file,
// compiles in one of three forms (design D56). Overload resolution decides
// which: while TypeScript checks a call's arguments against a candidate
// signature, each lambda argument takes the form that candidate's declared,
// uninstantiated parameter type asks for (D44), and the candidate that
// TypeScript's first-applicable rule selects decides the form the lambda
// compiles to.
//
//	Effect.retry(() => sync(store), policy)   // Effect<A, E, R> expected: a program
//	Effect.forEach(ids, (id) => find(id))      // (a) => Effect expected: an Effect function
//	ids.map((id) => id.trim())                 // (value) => U expected: a plain function
//
// The lambda's body is typed the same way in every form: a call that runs has
// its Effect's success type. Only the lambda's own type depends on the form,
// so each form is computed once from the same body, and a deferred recheck
// never reclassifies anything.
type form uint8

const (
	formNone    form = iota
	formPlain        // a plain JavaScript function: (x) => A
	formEffect       // an Effect function: (x) => Effect<A, E, R>
	formProgram      // a program, a function with no parameters: Effect<A, E, R>
)

// candidateKey identifies an overload candidate across its instantiations:
// the declaration that declares it, or the uninstantiated signature when it
// has none.
type candidateKey any

func keyOf(signature *checker.Signature) candidateKey {
	if d := signature.Declaration(); d != nil {
		return d
	}
	for signature.Target() != nil {
		signature = signature.Target()
	}
	return signature
}

// lambdaState is what the analyzer knows about one lambda.
type lambdaState struct {
	byCandidate map[candidateKey]form // the first form each candidate asked for
	first       form                  // the form the first candidate to check the lambda asked for
	free        form                  // the form outside every call's candidates
	deciding    bool                  // free is being decided; a recursive call sees plain
	types       [4]*checker.Type      // the lambda's type in each form
}

func (a *analyzer) lambda(fn *ast.Node) *lambdaState {
	s := a.lambdas[fn]
	if s == nil {
		s = &lambdaState{byCandidate: map[candidateKey]form{}}
		a.lambdas[fn] = s
	}
	return s
}

// isLambda reports whether fn is a function expression or arrow function in
// an .es file whose form the analyzer decides.
func isLambda(fn *ast.Node) bool {
	return (fn.Kind == ast.KindArrowFunction || fn.Kind == ast.KindFunctionExpression) && isLowerable(fn)
}

// callCandidate records that call's arguments are being checked against
// candidate, so a lambda among them takes that candidate's form.
func callCandidate(c *checker.Checker, call *ast.Node, candidate *checker.Signature) func() {
	if !isEffectScriptFile(ast.GetSourceFileOfNode(call)) {
		return func() {}
	}
	a := analyzerFor(c)
	a.current[call] = append(a.current[call], candidate)
	return func() {
		stack := a.current[call]
		if len(stack) <= 1 {
			delete(a.current, call)
		} else {
			a.current[call] = stack[:len(stack)-1]
		}
	}
}

// filterCandidate rejects a candidate that asked a lambda argument for the
// plain form when that lambda can pause: a plain function cannot run an
// Effect, so the candidate does not accept the lambda in the form it expects
// (D56), and overload resolution moves on to the next candidate.
func filterCandidate(c *checker.Checker, call *ast.Node, candidate *checker.Signature) bool {
	if !isEffectScriptFile(ast.GetSourceFileOfNode(call)) {
		return true
	}
	return !analyzerFor(c).rejected[call][keyOf(candidate)]
}

// functionExpressionType gives a lambda the type of its form at its position:
// the plain function TypeScript inferred, an Effect function, or the program's
// Effect.
func functionExpressionType(c *checker.Checker, fn *ast.Node, t *checker.Type) *checker.Type {
	if !isLambda(fn) {
		return t
	}
	a := analyzerFor(c)
	return a.typeInForm(fn, t, a.formHere(fn))
}

// formHere decides fn's form for the check in progress: against the candidate
// the enclosing call is being checked against, else against the candidate
// that call selected, else from fn's contextual type and body. It records the
// first answer for each candidate.
func (a *analyzer) formHere(fn *ast.Node) form {
	s := a.lambda(fn)
	call, path := ownerCall(fn)
	if call != nil {
		if signature := a.candidateOf(call); signature != nil {
			key := keyOf(signature)
			if f, ok := s.byCandidate[key]; ok {
				return f
			}
			e := a.classify(a.expectedAt(call, signature, path, fn))
			if a.isPipeProgram(call, path) {
				e.program = true
			}
			f := a.decide(fn, e)
			s.byCandidate[key] = f
			if s.first == formNone {
				s.first = f
			}
			if f == formPlain && !a.canBePlain(fn) {
				a.reject(call, key)
			}
			return f
		}
	}
	return a.freeForm(fn)
}

// finalForm is the form fn compiles to: the form the candidate its call
// selected asked for, or the form it has outside any call. When no candidate
// accepted the call, TypeScript may report it against a signature no
// candidate was checked as, and fn keeps the form the first candidate asked
// for.
func (a *analyzer) finalForm(fn *ast.Node) form {
	if call, _ := ownerCall(fn); call != nil {
		if signature := a.candidateOf(call); signature != nil {
			s := a.lambda(fn)
			if f, ok := s.byCandidate[keyOf(signature)]; ok {
				return f
			}
			if s.first != formNone {
				return s.first
			}
		}
	}
	return a.freeForm(fn)
}

// firstPause returns the first call in fn's own body that performs an Effect
// operation, or nil when fn compiles to plain JavaScript.
func (a *analyzer) firstPause(fn *ast.Node) *ast.Node {
	var pause *ast.Node
	forEachCallInBody(fn, func(call *ast.Node) bool {
		if a.performs(call) {
			pause = call
			return true
		}
		return false
	})
	return pause
}

// freeForm is fn's form where no candidate decides it: from its contextual
// type, else from its body. While it is being decided, a call from fn's body
// back to fn counts as plain, so a recursive lambda that runs nothing else is
// plain (the greatest fixpoint, as D56 says for D10).
func (a *analyzer) freeForm(fn *ast.Node) form {
	s := a.lambda(fn)
	if s.free != formNone {
		return s.free
	}
	if s.deciding {
		return formPlain
	}
	s.deciding = true
	f := a.decide(fn, a.classify(a.contextualTypes(fn)))
	s.deciding = false
	s.free = f
	return f
}

// decide turns an expectation into a form for fn. Without one, fn is plain
// when D10 allows it and an Effect function otherwise. Where only an Effect is
// expected, fn is a program; with required parameters it cannot be one, so it
// stays an Effect function and TypeScript reports the mismatch. Where a
// function or an Effect is expected (D39), a function with parameters is a
// function, and one without is a program when it can pause.
func (a *analyzer) decide(fn *ast.Node, e expectation) form {
	function := formNone
	switch {
	case e.effectFn:
		function = formEffect
	case e.plainFn:
		function = formPlain
	}
	if e.program && requiredParameterCount(fn) == 0 && (function == formNone || !a.canBePlain(fn)) {
		return formProgram
	}
	if function != formNone {
		return function
	}
	if e.program {
		return formEffect
	}
	if a.canBePlain(fn) && !a.hasEscapingFailure(fn) {
		return formPlain
	}
	return formEffect
}

func (a *analyzer) reject(call *ast.Node, key candidateKey) {
	m := a.rejected[call]
	if m == nil {
		m = map[candidateKey]bool{}
		a.rejected[call] = m
	}
	m[key] = true
}

// candidateOf returns the candidate call's arguments are being checked
// against, else the signature overload resolution selected for call, which is
// an instantiation of the selected candidate.
func (a *analyzer) candidateOf(call *ast.Node) *checker.Signature {
	if stack := a.current[call]; len(stack) > 0 {
		return stack[len(stack)-1]
	}
	if a.resolving[call] {
		return nil
	}
	a.resolving[call] = true
	defer delete(a.resolving, call)
	return a.checker.GetResolvedSignature(call)
}

// canBePlain reports whether fn compiles to plain JavaScript (D10): its body
// runs no Effect and looks up no service, and no throw can end it.
func (a *analyzer) canBePlain(fn *ast.Node) bool {
	return !forEachCallInBody(fn, a.performs) && !hasEscapingThrow(fn)
}

// typeInForm returns fn's type in form f, given t, the type TypeScript
// inferred for it: a function that returns its body's success value A.
func (a *analyzer) typeInForm(fn *ast.Node, t *checker.Type, f form) *checker.Type {
	if f == formPlain || f == formNone || fn.Type() != nil {
		return t
	}
	s := a.lambda(fn)
	if cached := s.types[f]; cached != nil {
		return cached
	}
	signatures := a.checker.GetSignaturesOfType(t, checker.SignatureKindCall)
	if len(signatures) != 1 {
		return t
	}
	effect := a.effectTypeOf(fn, a.checker.GetReturnTypeOfSignature(signatures[0]))
	if effect == nil {
		return t
	}
	result := effect
	if f == formEffect {
		result = a.checker.NewFunctionTypeWithReturnType(t.Symbol(), signatures[0], effect)
	}
	s.types[f] = result
	return result
}

// effectTypeOf returns Effect<A, E, R> for fn's body, given its success type
// A, or nil when no Effect interface is in reach.
func (a *analyzer) effectTypeOf(fn *ast.Node, success *checker.Type) *checker.Type {
	info := a.effectOf(fn)
	if info == nil {
		info = &effectInfo{} // fn's body is being collected
	}
	effect := a.effectInterfaceFor(fn, info)
	if effect == nil {
		effect = a.programEffectInterface()
	}
	if effect == nil {
		effect = a.effectInterface
	}
	if effect == nil {
		return nil
	}
	errors, requirements := info.errors, info.requirements
	return a.checker.NewTypeReference(effect, []*checker.Type{success, a.checker.GetUnionType(errors), a.checker.GetUnionType(requirements)})
}

// ownerCall returns the call or new expression that fn is an argument of,
// directly or as an element of an array literal or the value of an object
// literal property there, with the path from the argument down to fn.
func ownerCall(fn *ast.Node) (*ast.Node, []pathStep) {
	var path []pathStep
	node := fn
	for parent := node.Parent; parent != nil; node, parent = parent, parent.Parent {
		switch parent.Kind {
		case ast.KindParenthesizedExpression, ast.KindObjectLiteralExpression:
			continue
		case ast.KindArrayLiteralExpression:
			path = append(path, pathStep{element: true})
			continue
		case ast.KindPropertyAssignment:
			name := parent.Name()
			if parent.Initializer() != node || !(ast.IsIdentifier(name) || ast.IsStringLiteral(name) || ast.IsNumericLiteral(name)) {
				return nil, nil
			}
			path = append(path, pathStep{property: name.Text()})
			continue
		case ast.KindCallExpression, ast.KindNewExpression:
			for i, arg := range parent.Arguments() {
				if arg == node {
					reverse(path)
					return parent, append([]pathStep{{argument: i}}, path...)
				}
			}
		}
		return nil, nil
	}
	return nil, nil
}

// pathStep is one step from a call's argument list down to a lambda: the
// argument at an index, an array element, or an object property.
type pathStep struct {
	argument int
	element  bool
	property string
}

func reverse(path []pathStep) {
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
}

func requiredParameterCount(fn *ast.Node) int {
	n := 0
	for _, p := range fn.Parameters() {
		if p.Initializer() != nil || p.QuestionToken() != nil || p.AsParameterDeclaration().DotDotDotToken != nil {
			break
		}
		n++
	}
	return n
}

// contextualReturnType makes what an .es function returns contextually typed
// by the success type of the Effect expected of it. Its body returns the
// success value, because the calls in it run: an Effect function expected as
// (a) => Effect<B, E, R> returns a B. A program expected as Effect<A, E, R>
// has no contextual signature, so its body is typed by A, as the object a
// Layer.effect program returns is typed by the service's shape.
func contextualReturnType(c *checker.Checker, fn *ast.Node, t *checker.Type) *checker.Type {
	if !isLowerable(fn) {
		return t
	}
	if t == nil {
		if !isLambda(fn) || requiredParameterCount(fn) != 0 {
			return nil
		}
		if contextual := c.GetContextualType(fn, checker.ContextFlagsSignature); contextual != nil && len(c.GetSignaturesOfType(contextual, checker.SignatureKindCall)) == 0 {
			return successType(c, contextual)
		}
		return nil
	}
	if success := successType(c, t); success != nil {
		return success
	}
	return t
}
