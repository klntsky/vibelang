package effectscript

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/printer"
)

// effectTypeID names the variance property that every Effect 4 Effect has:
// `readonly [TypeId]: Variance<A, E, R>` with `TypeId = "~effect/Effect"`.
const effectTypeID = "~effect/Effect"

// plan records what the lowering does to one .es file.
type plan struct {
	effects  map[*ast.Node]bool       // functions lowered to Effect.fnUntraced generators
	programs map[*ast.Node]bool       // lambdas lowered to Effect.gen programs (design D7)
	runs     map[*ast.Node]bool       // calls in those functions emitted as yield*
	converts map[*ast.Node]conversion // function values converted where they are passed
	prelude  []string                 // prelude names the file uses as values (prelude.go)
	servicePlan
	plainPlan
	trustedPlan
	tracingPlan
}

func (p *plan) isEffect(fn *ast.Node) bool             { return p.effects[fn] }
func (p *plan) isProgram(fn *ast.Node) bool            { return p.programs[fn] }
func (p *plan) runsCall(call *ast.Node) bool           { return p.runs[call] }
func (p *plan) conversionOf(node *ast.Node) conversion { return p.converts[node] }

// planFile asks the checker which functions in file are Effects and which of
// their calls run.
func planFile(resolver printer.EmitResolver, file *ast.SourceFile) *plan {
	p := &plan{effects: map[*ast.Node]bool{}, programs: map[*ast.Node]bool{}, runs: map[*ast.Node]bool{}, converts: map[*ast.Node]conversion{}, servicePlan: newServicePlan(), plainPlan: newPlainPlan(), trustedPlan: newTrustedPlan(), tracingPlan: newTracingPlan()}
	r, ok := resolver.(*checker.EmitResolver)
	if !ok {
		return p
	}
	r.WithChecker(func(c *checker.Checker) {
		p.prelude = preludeUses(c, file)
		a := analyzerFor(c)
		var visit func(node *ast.Node) bool
		visit = func(node *ast.Node) bool {
			if isLowerable(node) && a.isEffect(node) {
				if isLambda(node) && a.finalForm(node) == formProgram {
					p.programs[node] = true
				} else {
					p.effects[node] = true
				}
				forEachCallInBody(node, func(call *ast.Node) bool {
					if a.runs(call) {
						p.runs[call] = true
					}
					p.planServiceCall(c, call)
					return false
				})
			}
			if conversion := a.finalConversion(node); conversion == convertProgram || conversion == convertPrograms {
				p.converts[node] = conversion
			}
			p.planPlain(a, node)
			p.planServiceNode(c, node)
			node.ForEachChild(visit)
			return false
		}
		file.AsNode().ForEachChild(visit)
		p.planTrustedCalls(c, a, file)
		p.planSpans(c, file)
	})
	return p
}

// forEachCallInBody calls cb for each call that fn's own body evaluates,
// skipping nested functions and classes, until cb returns true. It reports
// whether cb returned true.
func forEachCallInBody(fn *ast.Node, cb func(call *ast.Node) bool) bool {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if isFunctionBoundary(node) {
			return false
		}
		if node.Kind == ast.KindCallExpression && cb(node) {
			return true
		}
		return node.ForEachChild(visit)
	}
	return visit(fn.Body())
}

type fnState uint8

const (
	stateUnknown fnState = iota
	stateVisiting
	statePure
	stateEffect
)

// analyzer decides which .es functions are Effects. A function is an Effect
// when its body runs a call, a call whose signature returns an Effect or a
// call to an .es function that is itself an Effect, or when a throw can end
// it. A function on a call cycle is an Effect, which is always correct and
// only slower than solving the cycle. Every other function compiles to plain
// JavaScript (design D10).
type analyzer struct {
	checker   *checker.Checker
	states    map[*ast.Node]fnState
	effects   map[*ast.Node]*effectInfo // see effectOf
	tryBlocks map[*ast.Node]*effectInfo // see tryBlockFlow

	// Lambda forms (forms.go).
	lambdas         map[*ast.Node]*lambdaState
	current         map[*ast.Node][]*checker.Signature  // call -> candidates its arguments are being checked against
	rejected        map[*ast.Node]map[candidateKey]bool // call -> candidates a lambda argument rejected
	resolving       map[*ast.Node]bool                  // calls whose signature candidateOf is resolving
	effectInterface *checker.Type                       // the generic Effect interface, once seen

	conversions map[*ast.Node]*valueState // argument expressions that convert (conversions.go)

	// Plain JavaScript functions (plain.go).
	plain                 map[*ast.Node]plainState // function declarations that compile to plain JavaScript
	programEffect         *checker.Type            // effect's Effect interface, found in the program's files
	programEffectSearched bool

	callbacks map[*ast.Node]*callbackContracts // functions with uncontracted callback parameters (callbackcontracts.go)
}

type analyzerKey struct{}

// analyzerFor returns the analyzer that remembers c's answers. It lives on c,
// so it is freed with c. A checker is used by one goroutine at a time, so its
// analyzer needs no lock of its own.
func analyzerFor(c *checker.Checker) *analyzer {
	return c.ExtensionState(analyzerKey{}, func() any {
		return &analyzer{
			checker:   c,
			states:    map[*ast.Node]fnState{},
			effects:   map[*ast.Node]*effectInfo{},
			tryBlocks: map[*ast.Node]*effectInfo{},
			lambdas:   map[*ast.Node]*lambdaState{},
			current:   map[*ast.Node][]*checker.Signature{},
			rejected:  map[*ast.Node]map[candidateKey]bool{},
			resolving: map[*ast.Node]bool{},

			conversions: map[*ast.Node]*valueState{},
			plain:       map[*ast.Node]plainState{},
			callbacks:   map[*ast.Node]*callbackContracts{},
		}
	}).(*analyzer)
}

func (a *analyzer) isEffect(fn *ast.Node) bool {
	if isLambda(fn) {
		return a.finalForm(fn) != formPlain
	}
	if isPlainCandidate(fn) {
		return !a.isPlainFunction(fn)
	}
	switch a.states[fn] {
	case stateVisiting, stateEffect:
		return true
	case statePure:
		return false
	}
	a.states[fn] = stateVisiting
	effect := forEachCallInBody(fn, a.performs) || a.expectsEffect(fn) || hasEscapingThrow(fn) || a.hasEscapingFailure(fn)
	a.states[fn] = core.IfElse(effect, stateEffect, statePure)
	return effect
}

// performs reports whether call performs an Effect operation: it runs an
// Effect, or it looks up a service.
func (a *analyzer) performs(call *ast.Node) bool {
	return a.runs(call) || isServiceLookup(a.checker, call)
}

// runs reports whether call runs an Effect, which the lowering spells yield*.
func (a *analyzer) runs(call *ast.Node) bool {
	sig := a.checker.GetResolvedSignature(call)
	if sig == nil {
		return false
	}
	if fn := implementationOf(sig.Declaration()); fn != nil && isLowerable(fn) {
		if a.isPlainFunction(fn) {
			// A plain function's call runs inline, through its $direct
			// variant when it has one, and runs its Effect form only where
			// that variant is out of reach.
			return a.isSplittable(fn) && a.directReferenceOf(call.Expression(), fn) == nil
		}
		return a.isEffect(fn)
	}
	return isEffectType(a.checker, a.checker.GetReturnTypeOfSignature(sig))
}

// implementationOf returns the function body that decl resolves to, which
// differs from decl when decl is an overload signature.
func implementationOf(decl *ast.Node) *ast.Node {
	if decl == nil || decl.Kind != ast.KindFunctionDeclaration || decl.Body() != nil {
		return decl
	}
	if symbol := decl.Symbol(); symbol != nil {
		for _, d := range symbol.Declarations() {
			if d.Kind == ast.KindFunctionDeclaration && d.Body() != nil {
				return d
			}
		}
	}
	return decl
}

// isEffectType reports whether t is effect's own Effect interface,
// Effect<A, E, R>. Types that extend it, such as Config, Exit, Context keys,
// and TaggedError instances, are data: a call that returns one does not run
// it (design D24).
func isEffectType(c *checker.Checker, t *checker.Type) bool {
	if t == nil || t.Flags()&checker.TypeFlagsObject == 0 || t.Symbol() == nil {
		return false
	}
	return effectInterfaceSymbol(c, t) == t.Symbol()
}

// effectInterfaceSymbol returns the symbol of the interface named Effect that
// declares t's `~effect/Effect` property, or nil if t has no such property.
func effectInterfaceSymbol(c *checker.Checker, t *checker.Type) *ast.Symbol {
	property := c.GetPropertyOfType(t, effectTypeID)
	if property == nil {
		return nil
	}
	for _, decl := range property.Declarations() {
		if parent := decl.Parent; parent != nil && ast.IsInterfaceDeclaration(parent) && parent.Name().Text() == "Effect" {
			return c.GetSymbolOfDeclaration(parent)
		}
	}
	return nil
}
