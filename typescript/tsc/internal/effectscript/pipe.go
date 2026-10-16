package effectscript

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
// argument, and pipe's first stage takes an Effect.
func (a *analyzer) isPipeProgram(call *ast.Node, path []pathStep) bool {
	if len(path) != 1 || path[0].argument != 0 || len(call.Arguments()) < 2 || !a.isEffectPipe(call) {
		return false
	}
	stage := a.checker.GetTypeAtLocation(call.Arguments()[1])
	for _, signature := range a.checker.GetSignaturesOfType(stage, checker.SignatureKindCall) {
		if parameters := signature.Parameters(); len(parameters) > 0 && a.isEffectLike(a.checker.GetTypeOfSymbol(parameters[0])) {
			return true
		}
	}
	return false
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
