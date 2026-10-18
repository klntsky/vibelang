package checker

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// CallSignaturesFilterHook can change the signatures that call, a call
// expression, resolves against, after the checker has found its callee's own.
// It returns signatures unchanged when it has nothing to say. VibeLang's block
// params (D188) use it to offer `f(args) { block }` the forms of f's
// signatures that take the block where an Effect dual takes its program.
type CallSignaturesFilterHook func(c *Checker, call *ast.Node, signatures []*Signature) []*Signature

var callSignaturesFilterHooks []CallSignaturesFilterHook

// RegisterCallSignaturesFilterHook adds hook to every checker. Call it only
// from the init function of an in-tree package that tsc/cmd/tsc imports;
// registration is not synchronized.
func RegisterCallSignaturesFilterHook(hook CallSignaturesFilterHook) {
	callSignaturesFilterHooks = append(callSignaturesFilterHooks, hook)
}

func (c *Checker) filterCallSignaturesByHooks(call *ast.Node, signatures []*Signature) []*Signature {
	for _, hook := range callSignaturesFilterHooks {
		signatures = hook(c, call, signatures)
	}
	return signatures
}

// RotateSignature returns sig taking its parameters 1 through n and then its
// first parameter: the data-first form `(self, a, b) => R` rotated at 2 is
// `(a, b, self) => R`, so a call that passes self last resolves against it
// with the same type parameters and inference. Parameters after n must be
// optional; it returns nil when sig has fewer than n+1 parameters or needs
// more than n+1 arguments.
func (c *Checker) RotateSignature(sig *Signature, n int) *Signature {
	if n+1 > len(sig.parameters) || int(sig.minArgumentCount) > n+1 || sig.HasRestParameter() && n+1 >= len(sig.parameters) {
		return nil
	}
	parameters := append(append([]*ast.Symbol{}, sig.parameters[1:n+1]...), sig.parameters[0])
	return c.newSignature(SignatureFlagsNone, sig.declaration, sig.TypeParameters(), sig.ThisParameter(), parameters, c.getReturnTypeOfSignature(sig), nil /*resolvedTypePredicate*/, n+1)
}

// FlattenSignature returns the signature of applying sig to n arguments and
// then the function it returns, whose signature is inner, to one more: the
// data-last form `(a) => (self) => R` flattened at 1 is `(a, self) => R`, with
// the type parameters of both, so the call infers them together.
func (c *Checker) FlattenSignature(sig *Signature, n int, inner *Signature) *Signature {
	if n > len(sig.parameters) || len(inner.parameters) == 0 {
		return nil
	}
	typeParameters := append(append([]*Type{}, sig.TypeParameters()...), inner.TypeParameters()...)
	parameters := append(append([]*ast.Symbol{}, sig.parameters[:n]...), inner.parameters[0])
	return c.newSignature(SignatureFlagsNone, sig.declaration, typeParameters, sig.ThisParameter(), parameters, c.getReturnTypeOfSignature(inner), nil /*resolvedTypePredicate*/, n+1)
}
