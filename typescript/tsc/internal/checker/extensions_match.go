package checker

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// isVibeLangClassPattern reports whether call is the test the parser writes
// for a class pattern of a .vibe file, `when NotFound:` or `e is NotFound`
// (parser/vibelang_match.go): a call of the runtime's $is whose matcher is a
// class. $is tests a class with instanceof, or with its custom matcher, which
// for an error class is instanceof too (D213, D215, D278). The call narrows
// as instanceof does, by derivation rather than by shape, so two classes with
// the same name and fields, or a family's base and an unrelated error with
// the same fields, stay distinct, and D209's proof sees the cases a class
// pattern leaves (vibelang/match.go).
func (c *Checker) isVibeLangClassPattern(call *ast.Node) bool {
	callee := call.Expression()
	if !ast.IsIdentifier(callee) || callee.Flags&ast.NodeFlagsReparsed == 0 || callee.Text() != "$is" {
		return false
	}
	args := call.Arguments()
	return len(args) == 2 && len(c.getSignaturesOfType(c.getTypeOfExpression(args[0]), SignatureKindConstruct)) != 0
}
