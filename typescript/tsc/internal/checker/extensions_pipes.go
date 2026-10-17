package checker

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// isEffeckPipe reports whether node is a pipe of an .effeck file, left |>
// body: a binary expression that the parser flags NodeFlagsEffeck, with a
// BarToken, the | of |>, as its operator (parser/effeck_pipes.go). Its body
// has the pipe's contextual type, and the pipe has the body's type; the
// effeck package resolves the topic, %, in the body.
func isEffeckPipe(node *ast.Node) bool {
	return node != nil && node.Kind == ast.KindBinaryExpression && node.Flags&ast.NodeFlagsEffeck != 0 &&
		node.AsBinaryExpression().OperatorToken.Kind == ast.KindBarToken
}
