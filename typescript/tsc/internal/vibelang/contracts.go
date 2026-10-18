package vibelang

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// Contracts.
//
// The return type annotation of a .vibe function with a body is the function's
// contract, Effect<A, E, R>: the whole computation, as Promise<string> is the
// whole computation of an async function. The function is an Effect whatever
// its body runs. Its returns are checked against A, so a returned value that A
// does not accept is TypeScript's own error. An expected failure that can
// escape the body and that E does not accept is EK2001; a requirement that R
// does not accept is EK2002. A contract may be wider than the body, never
// narrower (Chapter 14).
//
// A channel written `infer Name` comes from the body (D57). The signature's
// return type holds what was inferred, so callers and declaration files see
// the full contract. A return annotation that leaves a channel out, such as
// Effect<User>, or that is not an Effect, such as number, is EK2003.

func init() {
	fileChecks = append(fileChecks, checkContracts)
}

// The channels of Effect<A, E, R>, in order.
const (
	channelA = iota
	channelE
	channelR
)

// A contract is the Effect<A, E, R> return annotation of a function that the
// lowering can turn into an Effect.
type contract struct {
	annotation *ast.Node   // the return type annotation
	reference  *ast.Node   // the reference to Effect whose type arguments are the channels; nil when the annotation names a whole Effect type, as a type alias does
	channels   []*ast.Node // reference's type arguments, A, E, R; fewer when some are left out
}

// inferred reports whether channel is written `infer Name`.
func (k *contract) inferred(channel int) bool {
	return channel < len(k.channels) && k.channels[channel].Kind == ast.KindInferType
}

func (k *contract) inferredAny() bool {
	return k.inferred(channelA) || k.inferred(channelE) || k.inferred(channelR)
}

// contractOf returns fn's contract, or nil when fn is not a function that the
// lowering can turn into an Effect, has no return type annotation, or is
// annotated with a type that is not an Effect.
func (a *analyzer) contractOf(fn *ast.Node) *contract {
	if k, ok := a.contracts[fn]; ok {
		return k
	}
	var k *contract
	if isLowerable(fn) && fn.Type() != nil {
		annotation := fn.Type()
		if t := a.checker.GetTypeFromTypeNode(annotation); isEffectType(a.checker, t) {
			k = &contract{annotation: annotation}
			// effect's own Effect<A, E, R>, or the prelude's alias of it, names
			// the channels; another alias names a whole Effect type.
			if annotation.Kind == ast.KindTypeReference && (t.Alias() == nil || t.Alias().Symbol().Name() == "Effect") {
				k.reference = annotation
				if arguments := annotation.TypeArgumentList(); arguments != nil {
					k.channels = arguments.Nodes
				}
			}
		}
	}
	a.contracts[fn] = k
	return k
}

// hasContract reports whether fn is a .vibe function whose return annotation
// is an Effect. Such a function is an Effect whatever its body runs.
func (a *analyzer) hasContract(fn *ast.Node) bool {
	return a.contractOf(fn) != nil
}

// hasInferredChannel reports whether node is a .vibe function whose return
// annotation writes a channel as `infer Name`. Its declaration file declares
// the return type the checker gives it, the full contract with the inferred
// channels filled in, instead of copying the annotation.
func hasInferredChannel(node *ast.Node) bool {
	if !isLowerable(node) || node.Type() == nil || node.Type().Kind != ast.KindTypeReference {
		return false
	}
	for _, argument := range node.Type().TypeArguments() {
		if argument.Kind == ast.KindInferType {
			return true
		}
	}
	return false
}

// unwrappedReturnType checks what a .vibe function with a contract returns
// against the contract's A, as TypeScript checks what an async function
// returns against the type its Promise wraps.
func unwrappedReturnType(c *checker.Checker, fn *ast.Node, t *checker.Type) *checker.Type {
	if !isLowerable(fn) || analyzerFor(c).contractOf(fn) == nil {
		return t
	}
	if success := successType(c, c.GetReturnTypeOfSignature(c.GetSignatureFromDeclaration(fn))); success != nil {
		return success
	}
	return t
}

// annotatedReturnType fills in each channel of fn's contract that is written
// `infer Name` with what fn's body gives it (D57): A from what the body
// returns, as TypeScript infers a return type, and E and R from what the body
// raises and needs.
func annotatedReturnType(c *checker.Checker, fn *ast.Node, t *checker.Type) *checker.Type {
	if !isLowerable(fn) {
		return t
	}
	a := analyzerFor(c)
	k := a.contractOf(fn)
	if k == nil || !k.inferredAny() {
		return t
	}
	effect := effectInterface(c, t)
	if effect == nil {
		return t
	}
	channels := []*checker.Type{successType(c, t), channelType(c, t, "_E"), channelType(c, t, "_R")}
	if k.inferred(channelA) {
		channels[channelA] = c.GetReturnTypeFromBody(fn)
	}
	if k.inferred(channelE) || k.inferred(channelR) {
		info := a.effectOf(fn)
		if info == nil {
			info = &effectInfo{} // fn's body is being collected
		}
		if k.inferred(channelE) {
			channels[channelE] = c.GetUnionType(info.errors)
		}
		if k.inferred(channelR) {
			channels[channelR] = c.GetUnionType(info.requirements)
		}
	}
	return c.NewTypeReference(effect, channels)
}

// inferChannel reports whether node, an infer type, is a channel of a
// contract, where `infer Name` asks for the channel to be inferred (D57). The
// name is not in scope anywhere, so it is never unused.
func inferChannel(c *checker.Checker, node *ast.Node) bool {
	reference := node.Parent
	if reference == nil || reference.Kind != ast.KindTypeReference || reference.Parent == nil {
		return false
	}
	fn := reference.Parent
	if fn.Type() != reference || !isLowerable(fn) {
		return false
	}
	k := analyzerFor(c).contractOf(fn)
	return k != nil && k.reference == reference
}

// calleeContract returns the E and R that a call contributes when its callee
// is a .vibe function with a contract, from the signature the call resolved
// to, so a generic contract is instantiated. ok is false when the contract
// infers a channel: the caller then reads the callee's body, which never asks
// for a return type that a call cycle may be computing.
func (a *analyzer) calleeContract(callee *ast.Node, sig *checker.Signature) (e, r *checker.Type, ok bool) {
	k := a.contractOf(callee)
	if k == nil || k.inferredAny() {
		return nil, nil, false
	}
	t := a.checker.GetReturnTypeOfSignature(sig)
	return channelType(a.checker, t, "_E"), channelType(a.checker, t, "_R"), true
}

// checkContracts reports, for each function in file that the lowering can
// turn into an Effect: EK2003 for a return annotation that is not a contract
// or leaves a channel out, EK2001 for an expected failure that escapes the
// contract's E, and EK2002 for a requirement that escapes its R.
func checkContracts(c *checker.Checker, file *ast.SourceFile) {
	a := analyzerFor(c)
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if isLowerable(node) && node.Type() != nil {
			a.checkContract(file, node)
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

func (a *analyzer) checkContract(file *ast.SourceFile, fn *ast.Node) {
	c := a.checker
	annotation := fn.Type()
	written := scanner.GetTextOfNode(annotation)
	k := a.contractOf(fn)
	switch {
	case k == nil:
		// A plain annotation keeps its type on a function that compiles to
		// plain JavaScript (D67). On one whose body runs an Effect it is not
		// what the function returns. A method that implements a plain-function
		// member, such as Equal.symbol, is checked against that member (D55).
		if fn.Kind != ast.KindMethodDeclaration && (!a.canBePlain(fn) || a.hasEscapingFailure(fn)) {
			d := report(c, annotation, EK2003.With("{0} is a plain return annotation, but {1} runs Effects, so it returns an Effect."), written, functionDisplayName(fn))
			note(d, "Write {0} to check that it cannot fail and needs nothing, or {1} to name its errors and infer its dependencies.", bangText(written, "never", "never"), bangText(written, "Errors"))
			note(d, "Or remove the annotation to infer the whole contract.")
		}
		return
	case k.reference != nil && len(k.channels) < 3:
		if len(k.channels) == 0 {
			return // TypeScript reports the missing type arguments
		}
		channels := channelTexts(k)
		d := report(c, annotation, EK2003, written)
		note(d, "Write {0} to check the missing channels as never.", bangText(channels...))
		if len(channels) == 2 {
			note(d, "Or write {0} to infer the dependencies from the body.", strings.TrimSuffix(bangText(channels...), "!"))
		} else {
			note(d, "Or remove the annotation to infer them from the body.")
		}
		return
	}
	info := a.effectOf(fn)
	if info == nil {
		return
	}
	contractType := c.GetReturnTypeOfSignature(c.GetSignatureFromDeclaration(fn))
	name := functionDisplayName(fn)
	if !k.inferred(channelE) {
		declared := channelType(c, contractType, "_E")
		for _, escape := range escapes(c, info.errors, info.errorSources, declared) {
			d := report(c, escape.source, EK2001, c.TypeToString(escape.t), name, "its contract says E = "+c.TypeToString(declared))
			note(d, "Catch it inside {0}, or change the contract to {1}.", name, contractText(c, contractType, channelE, escape.t))
		}
	}
	if !k.inferred(channelR) {
		declared := channelType(c, contractType, "_R")
		for _, escape := range escapes(c, info.requirements, info.requirementSources, declared) {
			d := report(c, escape.source, EK2002, name, c.TypeToString(escape.t), c.TypeToString(declared))
			note(d, "Provide {0} inside {1}, or add it: {2}.", c.TypeToString(escape.t), name, contractText(c, contractType, channelR, escape.t))
		}
	}
}

// An escape is what one call or throw adds to a channel that the contract's
// channel does not accept.
type escape struct {
	t      *checker.Type
	source *ast.Node
}

// escapes returns, for each source, the members of what it adds that
// declared does not accept, joined.
func escapes(c *checker.Checker, types []*checker.Type, sources []*ast.Node, declared *checker.Type) []escape {
	if declared == nil {
		return nil
	}
	var order []*ast.Node
	bySource := map[*ast.Node][]*checker.Type{}
	for i, t := range types {
		for _, member := range unionMembers(t) {
			if c.IsTypeAssignableTo(member, declared) {
				continue
			}
			source := sources[i]
			if _, seen := bySource[source]; !seen {
				order = append(order, source)
			}
			bySource[source] = append(bySource[source], member)
		}
	}
	result := make([]escape, 0, len(order))
	for _, source := range order {
		result = append(result, escape{t: c.GetUnionType(bySource[source]), source: source})
	}
	return result
}

// contractText writes contract, an Effect<A, E, R>, with extra joined to the
// given channel, for a fix that widens the contract. It writes the ! form.
func contractText(c *checker.Checker, contract *checker.Type, channel int, extra *checker.Type) string {
	channels := []*checker.Type{successType(c, contract), channelType(c, contract, "_E"), channelType(c, contract, "_R")}
	channels[channel] = c.GetUnionType([]*checker.Type{channels[channel], extra})
	texts := make([]string, len(channels))
	for i, t := range channels {
		texts[i] = c.TypeToString(t)
	}
	return bangText(texts...)
}

// channelTexts returns the source text of each channel k writes.
func channelTexts(k *contract) []string {
	texts := make([]string, len(k.channels))
	for i, channel := range k.channels {
		texts[i] = scanner.GetTextOfNode(channel)
	}
	return texts
}

// functionDisplayName names fn in a diagnostic: its name, the name of the
// variable it initializes, or "this function".
func functionDisplayName(fn *ast.Node) string {
	if name := functionName(fn); name != "" {
		return name
	}
	return "this function"
}
