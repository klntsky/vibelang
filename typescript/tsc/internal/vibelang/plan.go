package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/printer"
)

// effectTypeID names the variance property that every Effect 4 Effect has:
// `readonly [TypeId]: Variance<A, E, R>` with `TypeId = "~effect/Effect"`.
const effectTypeID = "~effect/Effect"

// plan records what the lowering does to one .vibe file.
type plan struct {
	effects      map[*ast.Node]bool       // functions lowered to Effect.fnUntraced generators
	programs     map[*ast.Node]bool       // lambdas lowered to Effect.gen programs (design D7)
	runs         map[*ast.Node]bool       // calls in those functions emitted as yield*
	heldPrograms map[*ast.Node]bool       // held values in by-name result positions
	returns      map[*ast.Node]bool       // returned Effect subtypes executed at an Effect return boundary
	converts     map[*ast.Node]conversion // function values converted where they are passed
	prelude      []string                 // prelude names the file uses as values (prelude.go)
	servicePlan
	plainPlan
	trustedPlan
	tracingPlan
	scopePlan
	handlerPlan
	producerPlan
	hatches     map[*ast.Node]hatchForm      // sync and try hatches, lowered to Effect's functions (hatches.go)
	failureTags map[*ast.Node][]string       // the tags of each typed catch clause's test (catches.go)
	causeThrows map[*ast.Node]bool           // throw statements whose value is a Cause (primitives.go)
	methods     map[*ast.Node]methodCall     // method calls on Effect data types (datamethods.go)
	blockForms  map[*ast.Node]blockParamForm // block param calls that move their block (blockparams.go)

	pipes         map[*ast.Node]pipeMode    // how each pipe compiles (pipes_lower.go)
	topicClosures map[*ast.Node][]*ast.Node // functions that read the topic of these pipes

	entry *entryPlan // how an entry module runs its main (entry.go), or nil
}

func (p *plan) isEffect(fn *ast.Node) bool             { return p.effects[fn] }
func (p *plan) isProgram(fn *ast.Node) bool            { return p.programs[fn] }
func (p *plan) runsCall(call *ast.Node) bool           { return p.runs[call] }
func (p *plan) conversionOf(node *ast.Node) conversion { return p.converts[node] }

// planFile asks the checker which functions in file are Effects and which of
// their calls run.
func planFile(resolver printer.EmitResolver, file *ast.SourceFile) *plan {
	p := &plan{effects: map[*ast.Node]bool{}, programs: map[*ast.Node]bool{}, runs: map[*ast.Node]bool{}, heldPrograms: map[*ast.Node]bool{}, converts: map[*ast.Node]conversion{}, servicePlan: newServicePlan(), plainPlan: newPlainPlan(), trustedPlan: newTrustedPlan(), tracingPlan: newTracingPlan(), scopePlan: newScopePlan()}
	p.returns = map[*ast.Node]bool{}
	p.handlerPlan = newHandlerPlan()
	p.hatches = map[*ast.Node]hatchForm{}
	p.failureTags = map[*ast.Node][]string{}
	p.causeThrows = map[*ast.Node]bool{}
	p.methods = map[*ast.Node]methodCall{}
	p.blockForms = map[*ast.Node]blockParamForm{}
	p.pipes = map[*ast.Node]pipeMode{}
	p.topicClosures = map[*ast.Node][]*ast.Node{}
	p.producerPlan = newProducerPlan()
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
				p.planClause(c, node)
				p.planProducer(c, node)
				forEachCallInBody(node, func(call *ast.Node) bool {
					if a.runs(call) {
						p.runs[call] = true
					}
					if ast.IsCallExpression(call) {
						p.planServiceCall(c, call)
						p.planResume(c, call)
					}
					return false
				})
				p.planScopes(a, node)
			}
			if a.returnedProgram(node) != nil {
				p.returns[node] = true
			}
			if conversion := a.finalConversion(node); conversion == convertProgram || conversion == convertSyncProgram || conversion == convertPrograms || conversion == convertByName {
				p.converts[node] = conversion
				if conversion == convertByName {
					byNameResults(byNameSubject(node), func(result *ast.Node) {
						p.heldPrograms[result] = a.heldPrograms[result]
					})
				}
			}
			if form, ok := hatchFormOf(c, node); ok {
				p.hatches[node] = form
			}
			if node.Kind == ast.KindThrowStatement && inLowerableBody(node) {
				if _, ok := causeErrorOf(c, c.GetTypeAtLocation(node.Expression())); ok {
					p.causeThrows[node] = true
				}
			}
			if isFailureTest(node) {
				p.failureTags[node], _ = failureTags(c, c.GetTypeFromTypeNode(node.TypeArguments()[0]))
			}
			if ast.IsCallExpression(node) && ast.IsPropertyAccessExpression(node.Expression()) {
				if call, ok := methodCallOf(c, node.Expression()); ok {
					p.methods[node] = call
				}
			}
			if isPipe(node) {
				p.planPipe(a, node)
			}
			if form := blockParamFormOf(c, node); form != blockAppended {
				p.blockForms[node] = form
			}
			p.planPlain(a, node)
			p.planServiceNode(c, node)
			node.ForEachChild(visit)
			return false
		}
		file.AsNode().ForEachChild(visit)
		p.planTrustedCalls(c, a, file)
		p.entry = planEntry(c, file)
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
		if isInvocation(node) && cb(node) {
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

// analyzer decides which .vibe functions are Effects. A function is an Effect
// when its body runs a call, a call whose signature returns an Effect or a
// call to a .vibe function that is itself an Effect, or when a throw can end
// it. A function on a call cycle is an Effect, which is always correct and
// only slower than solving the cycle. Every other function compiles to plain
// JavaScript (design D10).
type analyzer struct {
	checker   *checker.Checker
	states    map[*ast.Node]fnState
	effects   map[*ast.Node]*effectInfo // see effectOf
	approx    map[*ast.Node]*effectInfo // the E and R found so far of a call cycle being solved (recursion.go)
	recursive map[*ast.Node]bool        // functions on a call cycle
	tryBlocks map[*ast.Node]*effectInfo // see tryBlockFlow
	contracts map[*ast.Node]*contract   // see contractOf

	// Lambda forms (forms.go).
	lambdas         map[*ast.Node]*lambdaState
	current         map[*ast.Node][]*checker.Signature  // call -> candidates its arguments are being checked against
	rejected        map[*ast.Node]map[candidateKey]bool // call -> candidates a lambda argument rejected
	resolving       map[*ast.Node]bool                  // calls whose signature candidateOf is resolving
	effectInterface *checker.Type                       // the generic Effect interface, once seen

	conversions      map[*ast.Node]*valueState   // argument expressions that convert (conversions.go)
	heldPrograms     map[*ast.Node]bool          // by-name result values before argument conversion
	returnedPrograms map[*ast.Node]*checker.Type // structured programs at Effect return boundaries
	valueTyping      map[*ast.Node]bool          // by-name arguments whose own type is being read (valueTypeOf)

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
			approx:    map[*ast.Node]*effectInfo{},
			recursive: map[*ast.Node]bool{},
			tryBlocks: map[*ast.Node]*effectInfo{},
			contracts: map[*ast.Node]*contract{},
			lambdas:   map[*ast.Node]*lambdaState{},
			current:   map[*ast.Node][]*checker.Signature{},
			rejected:  map[*ast.Node]map[candidateKey]bool{},
			resolving: map[*ast.Node]bool{},

			conversions:      map[*ast.Node]*valueState{},
			heldPrograms:     map[*ast.Node]bool{},
			returnedPrograms: map[*ast.Node]*checker.Type{},
			valueTyping:      map[*ast.Node]bool{},
			plain:            map[*ast.Node]plainState{},
			callbacks:        map[*ast.Node]*callbackContracts{},
		}
	}).(*analyzer)
}

func (a *analyzer) isEffect(fn *ast.Node) bool {
	if isProducer(fn) {
		return true // its body is an Effect body; calling it returns a Stream
	}
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
	effect := forEachCallInBody(fn, a.performs) || (a.expectsEffect(fn) || hasEscapingThrow(fn)) && !a.hasPlainContract(fn) || a.hasEscapingFailure(fn) || a.hasContract(fn) || declaresUsing(fn) || hasVibeLangStatement(fn) || a.hasStreamLoop(fn)
	a.states[fn] = core.IfElse(effect, stateEffect, statePure)
	return effect
}

// performs reports whether call performs an Effect operation: it runs an
// Effect, or it looks up a service.
func (a *analyzer) performs(call *ast.Node) bool {
	return a.runs(call) || ast.IsCallExpression(call) && !holdsFieldProgram(a.checker, call) && isServiceLookup(a.checker, call)
}

// runs reports whether call runs an Effect, which the lowering spells yield*.
func (a *analyzer) runs(call *ast.Node) bool {
	if inHatch(call) || holdsFieldProgram(a.checker, call) {
		return false // hatches and contextual program fields keep the Effect value
	}
	sig := a.checker.GetResolvedSignature(call)
	if sig == nil {
		return false
	}
	if fn := implementationOf(sig.Declaration()); fn != nil && isLowerable(fn) {
		if isProducer(fn) {
			return false // a call constructs the producer's Stream and runs none of it
		}
		if a.isPlainFunction(fn) && a.isSplittable(fn) {
			// A plain function's call runs inline, through its $direct
			// variant when it has one, and runs its Effect form only where
			// that variant is out of reach.
			return a.directReferenceOf(ast.GetInvokedExpression(call), fn) == nil
		}
		if a.isEffect(fn) {
			return true // body inference may still be computing the signature's type
		}
		// A plain factory can return a cold Effect stored in a local. The
		// callee's form does not decide whether that returned program runs.
	}
	return callRunsResult(a.checker, call, a.checker.GetReturnTypeOfSignature(sig))
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
