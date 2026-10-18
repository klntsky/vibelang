package checker

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// isVibeLangPipe reports whether node is a pipe of a .vibe file, left |>
// body: a binary expression that the parser flags NodeFlagsVibeLang, with a
// BarToken, the | of |>, as its operator (parser/vibelang_pipes.go). Its body
// has the pipe's contextual type, and the pipe has the body's type; the
// vibelang package resolves the topic, %, in the body.
func isVibeLangPipe(node *ast.Node) bool {
	return node != nil && node.Kind == ast.KindBinaryExpression && node.Flags&ast.NodeFlagsVibeLang != 0 &&
		node.AsBinaryExpression().OperatorToken.Kind == ast.KindBarToken
}
