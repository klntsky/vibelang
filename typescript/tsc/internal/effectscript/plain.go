package effectscript

import (
	"slices"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// A function declaration that performs no Effect operation and cannot fail
// compiles to plain JavaScript (design D10), and still has one calling
// convention: TypeScript sees (...) => Effect<A, never, never> (Chapter 15).
// It compiles to two functions:
//
//	function double$direct(n) { return n * 2 }                          // EffectScript callers use this
//	export function double(...args) { return Effect.sync(() => double$direct(...args)) }
//
// An EffectScript caller calls double$direct inline: double(4) has the type
// number, runs no Effect, and costs what a JavaScript call costs. Passed
// where a plain function is expected, double is double$direct (D9).
//
// Eligibility is a greatest fixpoint over the call graph (D56): a group of
// functions that call one another compiles to plain JavaScript when each
// would with the calls inside the group counted as plain, so pure recursion
// runs on the JavaScript stack.

type plainState uint8

const (
	plainUnknown   plainState = iota
	plainExploring            // in the group being decided: a call to it counts as plain
	plainYes
	plainNo
)

// isPlainCandidate reports whether D10 decides how fn compiles: a named .es
// function declaration with a body and no overloads.
func isPlainCandidate(fn *ast.Node) bool {
	if fn == nil || fn.Kind != ast.KindFunctionDeclaration || fn.Name() == nil || !isLowerable(fn) {
		return false
	}
	symbol := fn.Symbol()
	return symbol != nil && len(symbol.Declarations()) == 1
}

// isSplittable reports whether fn, when it compiles to plain JavaScript,
// compiles to a $direct variant and an Effect form: a plain candidate with no
// return type annotation or an Effect contract. A function annotated with a
// plain return type keeps that type, and its callers call it as it is.
func (a *analyzer) isSplittable(fn *ast.Node) bool {
	if !isPlainCandidate(fn) {
		return false
	}
	t := fn.Type()
	return t == nil || a.isEffectAnnotation(t)
}

// isEffectAnnotation reports whether the type node t names an Effect.
func (a *analyzer) isEffectAnnotation(t *ast.Node) bool {
	return isEffectType(a.checker, a.checker.GetTypeFromTypeNode(t))
}

// isPlainFunction reports whether fn, a plain candidate, compiles to plain
// JavaScript.
func (a *analyzer) isPlainFunction(fn *ast.Node) bool {
	if !isPlainCandidate(fn) {
		return false
	}
	switch a.plain[fn] {
	case plainYes, plainExploring:
		return true
	case plainNo:
		return false
	}
	type member struct {
		ok   bool
		deps []*ast.Node
	}
	group := map[*ast.Node]*member{}
	queue := []*ast.Node{fn}
	for len(queue) > 0 {
		d := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if _, seen := group[d]; seen || a.plain[d] == plainYes || a.plain[d] == plainNo {
			continue
		}
		a.plain[d] = plainExploring
		m := &member{ok: !hasEscapingThrow(d) && !declaresUsing(d) && !a.hasEscapingFailure(d)}
		group[d] = m
		forEachCallInBody(d, func(call *ast.Node) bool {
			if callee := a.candidateCallee(call); callee != nil {
				switch a.plain[callee] {
				case plainNo:
					m.ok = false
				case plainUnknown, plainExploring:
					m.deps = append(m.deps, callee)
					queue = append(queue, callee)
				}
				return false
			}
			if a.performs(call) {
				m.ok = false
			}
			return false
		})
	}
	for changed := true; changed; {
		changed = false
		for _, m := range group {
			if !m.ok {
				continue
			}
			for _, dep := range m.deps {
				if dm, ok := group[dep]; ok && !dm.ok || a.plain[dep] == plainNo {
					m.ok = false
					changed = true
					break
				}
			}
		}
	}
	for d, m := range group {
		if m.ok {
			a.plain[d] = plainYes
		} else {
			a.plain[d] = plainNo
		}
	}
	return a.plain[fn] == plainYes
}

// candidateCallee returns the plain candidate that call calls, or nil.
func (a *analyzer) candidateCallee(call *ast.Node) *ast.Node {
	signature := a.checker.GetResolvedSignature(call)
	if signature == nil {
		return nil
	}
	if fn := implementationOf(signature.Declaration()); isPlainCandidate(fn) {
		return fn
	}
	return nil
}

// declaresUsing reports whether fn's own body declares a using variable,
// which acquires an Effect resource, so fn compiles to frames (D54).
func declaresUsing(fn *ast.Node) bool {
	return firstUsing(fn) != nil
}

// firstUsing returns the first using declaration list in fn's own body, or
// nil.
func firstUsing(fn *ast.Node) *ast.Node {
	var found *ast.Node
	var visit func(n *ast.Node) bool
	visit = func(n *ast.Node) bool {
		if isFunctionBoundary(n) {
			return false
		}
		if n.Kind == ast.KindVariableDeclarationList && isUsingList(n) {
			found = n
			return true
		}
		return n.ForEachChild(visit)
	}
	visit(fn.Body())
	return found
}

// isUsingList reports whether list, a variable declaration list, declares
// using or await using variables. The block-scoped flags of await using are
// those of const and using together, so a test for either flag alone would
// match const.
func isUsingList(list *ast.Node) bool {
	kind := list.Flags & ast.NodeFlagsBlockScoped
	return kind == ast.NodeFlagsUsing || kind == ast.NodeFlagsAwaitUsing
}

// directReference describes how code in the callee expression's file reaches
// the $direct variant of a plain function.
type directReference struct {
	name      string    // the identifier to write in place of the reference, such as "double$direct"
	access    *ast.Node // for M.double through a namespace import, the access expression
	specifier *ast.Node // for a named import, the import specifier that must bring the variant in
}

// directReferenceOf returns how node, the callee of a call or a function
// value, reaches the $direct variant of fn, a plain function: by its own name
// in fn's file, by a named import of fn's .es module, or through a namespace
// import of it. It returns nil when the variant is out of reach, as through a
// default import, and the reference then uses fn's Effect form.
func (a *analyzer) directReferenceOf(node *ast.Node, fn *ast.Node) *directReference {
	switch {
	case ast.IsIdentifier(node):
		symbol := a.checker.GetResolvedSymbol(node)
		if symbol == nil {
			return nil
		}
		if symbol.Flags()&ast.SymbolFlagsAlias == 0 {
			// An exported declaration's name resolves to its local symbol.
			if slices.Contains(symbol.Declarations(), fn) {
				return &directReference{name: node.Text() + "$direct"}
			}
			return nil
		}
		for _, d := range symbol.Declarations() {
			if ast.IsImportSpecifier(d) && !d.AsImportSpecifier().IsTypeOnly && a.declares(a.checker.GetAliasedSymbol(symbol), fn) {
				return &directReference{name: node.Text() + "$direct", specifier: d}
			}
		}
	case ast.IsPropertyAccessExpression(node) && ast.IsIdentifier(node.Expression()) && ast.IsIdentifier(node.Name()):
		namespace := a.checker.GetResolvedSymbol(node.Expression())
		if namespace == nil || namespace.Flags()&ast.SymbolFlagsAlias == 0 || !slices.ContainsFunc(namespace.Declarations(), ast.IsNamespaceImport) {
			return nil
		}
		if a.declares(a.checker.GetSymbolAtLocation(node.Name()), fn) {
			return &directReference{name: node.Name().Text() + "$direct", access: node}
		}
	}
	return nil
}

// declares reports whether symbol, or the symbol it aliases, declares fn.
func (a *analyzer) declares(symbol *ast.Symbol, fn *ast.Node) bool {
	if symbol != nil && symbol.Flags()&ast.SymbolFlagsAlias != 0 {
		symbol = a.checker.GetAliasedSymbol(symbol)
	}
	return symbol != nil && slices.Contains(symbol.Declarations(), fn)
}

// directCallee returns how call, a call in an .es file that calls a plain
// function by name, reaches that function's $direct variant, or nil. Such a
// call runs inline wherever it is, at a module's top level too: it computes a
// value and runs no program, as Chapter 13's table says.
func (a *analyzer) directCallee(call *ast.Node) *directReference {
	if !isEffectScriptFile(ast.GetSourceFileOfNode(call)) {
		return nil
	}
	fn := esFunctionOf(a.checker, call.Expression())
	if fn == nil || !a.isSplittable(fn) || !a.isPlainFunction(fn) {
		return nil
	}
	return a.directReferenceOf(call.Expression(), fn)
}

// plainTypeOf returns the type of fn's $direct variant, given t, the type of
// its Effect form: the same parameters, returning the success type.
func (a *analyzer) plainTypeOf(t *checker.Type) *checker.Type {
	signatures := a.checker.GetSignaturesOfType(t, checker.SignatureKindCall)
	if len(signatures) != 1 {
		return t
	}
	success := successType(a.checker, a.checker.GetReturnTypeOfSignature(signatures[0]))
	if success == nil {
		return t
	}
	return a.checker.NewFunctionTypeWithReturnType(t.Symbol(), signatures[0], success)
}

// programEffectInterface returns the generic Effect interface that effect's
// Effect module declares, or nil when the program does not include effect.
// Unlike an Effect found in some type on the way, it does not depend on the
// order in which the checker checks things.
func (a *analyzer) programEffectInterface() *checker.Type {
	if a.programEffectSearched {
		return a.programEffect
	}
	a.programEffectSearched = true
	for _, file := range a.checker.Program().SourceFiles() {
		if !isEffectModuleFile(file, "Effect") || file.Symbol == nil {
			continue
		}
		for _, export := range a.checker.GetExportsOfModule(file.Symbol) {
			if export.Name() == "Effect" && export.Flags()&ast.SymbolFlagsInterface != 0 {
				a.programEffect = a.checker.GetDeclaredTypeOfSymbol(export)
				return a.programEffect
			}
		}
	}
	return nil
}

// plainPlan records what the lowering does with plain functions in one file.
type plainPlan struct {
	directs       map[*ast.Node]bool             // function declarations split into a $direct variant and an Effect wrapper
	directRefs    map[*ast.Node]*directReference // calls and function values that reach a $direct variant
	directImports map[*ast.Node][]*ast.Node      // import declarations, and the specifiers whose $direct variants they bring in
}

func newPlainPlan() plainPlan {
	return plainPlan{directs: map[*ast.Node]bool{}, directRefs: map[*ast.Node]*directReference{}, directImports: map[*ast.Node][]*ast.Node{}}
}

// planPlain records how the lowering treats node with respect to plain
// functions: a declaration to split, a call that runs inline, or a function
// value passed as its plain variant.
func (p *plainPlan) planPlain(a *analyzer, node *ast.Node) {
	if a.isSplittable(node) && a.isPlainFunction(node) {
		p.directs[node] = true
	}
	var ref *directReference
	switch {
	case node.Kind == ast.KindCallExpression:
		ref = a.directCallee(node)
	case a.finalConversion(node) == convertDirect:
		if fn := esFunctionOf(a.checker, node); fn != nil {
			ref = a.directReferenceOf(node, fn)
		}
	}
	if ref == nil {
		return
	}
	p.directRefs[node] = ref
	if ref.specifier != nil {
		declaration := ast.FindAncestor(ref.specifier, ast.IsImportDeclaration)
		if declaration != nil && !slices.Contains(p.directImports[declaration], ref.specifier) {
			p.directImports[declaration] = append(p.directImports[declaration], ref.specifier)
		}
	}
}
