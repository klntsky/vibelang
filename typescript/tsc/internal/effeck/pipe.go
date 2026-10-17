package effeck

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// The built-in pipe takes the program first (design D8):
//
//	const user = pipe(
//	  () => fetchUser(id),
//	  Effect.retry({ times: 3 }),
//	  Effect.timeout("2 seconds")
//	)
//
// pipe's first parameter is a bare type parameter, which says nothing about
// form. So pipe looks ahead: when its first argument is a function with no
// parameters and its first stage takes an Effect, the first argument is a
// program. Applied to plain data, pipe is effect's ordinary pipe.

// isPipeProgram reports whether the argument at path is pipe's first
// argument, and pipe's first stage takes an Effect. Callers ask only about an
// argument that can be a program, a function with no required parameters.
//
// The stage itself is not checked here: pipe has not inferred the type the
// stage receives yet, and a generic stage such as Match.when(…) or
// Effect.catchTag(…) checked without it would keep a resolution that lost
// its inferences. The look-ahead reads the stage's callee instead.
func (a *analyzer) isPipeProgram(call *ast.Node, path []pathStep) bool {
	if len(path) != 1 || path[0].argument != 0 || len(call.Arguments()) < 2 || !a.isEffectPipe(call) {
		return false
	}
	for _, stage := range a.stageSignatures(call.Arguments()[1]) {
		if parameters := stage.Parameters(); len(parameters) > 0 && a.isExpectedEffect(a.checker.GetTypeOfSymbol(parameters[0])) {
			return true
		}
	}
	return false
}

// stageSignatures returns the signatures of the function that stage, a pipe
// argument, evaluates to: for a call such as Effect.retry(policy), the
// functions its callee's signatures of that arity return; for a function
// value such as Effect.orDie, its own signatures.
func (a *analyzer) stageSignatures(stage *ast.Node) []*checker.Signature {
	c := a.checker
	stage = ast.SkipParentheses(stage)
	switch {
	case ast.IsCallExpression(stage):
		count := len(stage.Arguments())
		var out []*checker.Signature
		for _, s := range c.GetSignaturesOfType(c.GetTypeAtLocation(stage.Expression()), checker.SignatureKindCall) {
			if count < s.MinArgumentCount() || count > len(s.Parameters()) && !s.HasRestParameter() {
				continue
			}
			out = append(out, c.GetSignaturesOfType(c.GetReturnTypeOfSignature(s), checker.SignatureKindCall)...)
		}
		return out
	case ast.IsIdentifier(stage) || ast.IsPropertyAccessExpression(stage):
		return c.GetSignaturesOfType(c.GetTypeAtLocation(stage), checker.SignatureKindCall)
	}
	return nil
}

// isEffectPipe reports whether call calls effect's pipe function, which the
// prelude also exports.
func (a *analyzer) isEffectPipe(call *ast.Node) bool {
	callee := call.Expression()
	if !ast.IsIdentifier(callee) || callee.Text() != "pipe" {
		return false
	}
	symbol := a.checker.GetSymbolAtLocation(callee)
	if symbol != nil && symbol.Flags()&ast.SymbolFlagsAlias != 0 {
		symbol = a.checker.GetAliasedSymbol(symbol)
	}
	if symbol == nil {
		return false
	}
	for _, declaration := range symbol.Declarations() {
		if isEffectModuleFile(ast.GetSourceFileOfNode(declaration), "Function") {
			return true
		}
	}
	return false
}
