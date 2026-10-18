package parser

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// The boundary hatch try(…) (design D148) is a call in a .vibe file:
//
//	const parsed = try(() => JSON.parse(text))
//
// A try statement starts with a block, so `try (` is never one, and in
// TypeScript a try keyword followed by ( is a syntax error. The callee parses
// as an identifier named try, which the checker resolves to the prelude's
// hatch and the lowering replaces with Effect.try or Effect.tryPromise.

// isStartOfTryCall reports whether the current token, try, starts the call
// try(…) in a .vibe file.
func (p *Parser) isStartOfTryCall() bool {
	return p.token == ast.KindTryKeyword && p.isVibeLangFile() && p.lookAhead((*Parser).nextTokenIsOpenParen)
}
