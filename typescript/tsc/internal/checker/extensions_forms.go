package checker

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// This file holds the hooks that let an in-tree language extension decide
// what a function expression is while overload resolution checks it against
// each candidate signature.

// CallCandidateHook is told that the arguments of call, a call or new
// expression, are about to be checked against candidate, during type argument
// inference or the applicability check of overload resolution. It returns a
// function that the checker calls when that check is done; nested and
// repeated checks of the same call nest their calls.
type CallCandidateHook func(c *Checker, call *ast.Node, candidate *Signature) (done func())

var callCandidateHooks []CallCandidateHook

// RegisterCallCandidateHook adds hook to every checker. Call it only from the
// init function of an in-tree package that tsc/cmd/tsc imports; registration
// is not synchronized.
func RegisterCallCandidateHook(hook CallCandidateHook) {
	callCandidateHooks = append(callCandidateHooks, hook)
}

func (c *Checker) enterCallCandidate(call *ast.Node, candidate *Signature) func() {
	switch len(callCandidateHooks) {
	case 0:
		return func() {}
	case 1:
		return callCandidateHooks[0](c, call, candidate)
	}
	dones := make([]func(), 0, len(callCandidateHooks))
	for _, hook := range callCandidateHooks {
		dones = append(dones, hook(c, call, candidate))
	}
	return func() {
		for i := len(dones) - 1; i >= 0; i-- {
			dones[i]()
		}
	}
}

// CandidateFilterHook can reject candidate for call after every argument
// type-checked against it. Overload resolution then treats candidate as if an
// argument did not match and tries the next one. It returns true to accept.
type CandidateFilterHook func(c *Checker, call *ast.Node, candidate *Signature) bool

var candidateFilterHooks []CandidateFilterHook

// RegisterCandidateFilterHook adds hook to every checker. Call it only from
// the init function of an in-tree package that tsc/cmd/tsc imports;
// registration is not synchronized.
func RegisterCandidateFilterHook(hook CandidateFilterHook) {
	candidateFilterHooks = append(candidateFilterHooks, hook)
}

func (c *Checker) acceptCandidateByHooks(call *ast.Node, candidate *Signature) bool {
	for _, hook := range candidateFilterHooks {
		if !hook(c, call, candidate) {
			return false
		}
	}
	return true
}
