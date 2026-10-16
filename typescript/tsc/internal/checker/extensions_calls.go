package checker

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// MemberReceiverHook can replace receiver, the apparent type in which the
// checker is about to look up the property that node, a property access
// expression, names. It returns receiver when it has nothing to say.
type MemberReceiverHook func(c *Checker, node *ast.Node, receiver *Type) *Type

var memberReceiverHooks []MemberReceiverHook

// RegisterMemberReceiverHook adds hook to every checker. Call it only from
// the init function of an in-tree package that tsc/cmd/tsc imports;
// registration is not synchronized.
func RegisterMemberReceiverHook(hook MemberReceiverHook) {
	memberReceiverHooks = append(memberReceiverHooks, hook)
}

func (c *Checker) memberReceiverFromHooks(node *ast.Node, receiver *Type) *Type {
	for _, hook := range memberReceiverHooks {
		receiver = hook(c, node, receiver)
	}
	return receiver
}

// CallSignaturesHook can give signatures to calleeType, the type of the
// callee of call when that type has no call signatures of its own. It
// returns nil when it has nothing to say.
type CallSignaturesHook func(c *Checker, call *ast.Node, calleeType *Type) []*Signature

var callSignaturesHooks []CallSignaturesHook

// RegisterCallSignaturesHook adds hook to every checker. Call it only from
// the init function of an in-tree package that tsc/cmd/tsc imports;
// registration is not synchronized.
func RegisterCallSignaturesHook(hook CallSignaturesHook) {
	callSignaturesHooks = append(callSignaturesHooks, hook)
}

func (c *Checker) callSignaturesFromHooks(call *ast.Node, calleeType *Type) []*Signature {
	for _, hook := range callSignaturesHooks {
		if signatures := hook(c, call, calleeType); len(signatures) != 0 {
			return signatures
		}
	}
	return nil
}

// NewCallSignature returns the signature `() => returnType`.
func (c *Checker) NewCallSignature(returnType *Type) *Signature {
	return c.newSignature(SignatureFlagsNone, nil /*declaration*/, nil /*typeParameters*/, nil /*thisParameter*/, nil /*parameters*/, returnType, nil /*resolvedTypePredicate*/, 0 /*minArgumentCount*/)
}
