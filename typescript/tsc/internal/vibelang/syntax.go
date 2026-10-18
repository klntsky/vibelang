package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// The checks in this file report TypeScript syntax that VibeLang does not
// have. A call already waits, so async, for await, and await using are
// EK1003 (Chapter 10). The parser writes await outside an async function as
// $await (fibers.go), so an AwaitExpression here is inside an async function. A module's top level is not inside a program, so using
// there has no block to release it in: EK1004 (Chapter 6).

func init() {
	fileChecks = append(fileChecks, checkSyntax)
}

func checkSyntax(c *checker.Checker, file *ast.SourceFile) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if hatchOf(node) != "" {
			return forEachHatchVibeLang(node, visit) // async and await keep their JavaScript meaning in a hatch's TypeScript (D148)
		}
		switch node.Kind {
		case ast.KindFunctionDeclaration, ast.KindFunctionExpression, ast.KindArrowFunction, ast.KindMethodDeclaration:
			if modifiers := node.Modifiers(); modifiers != nil {
				for _, modifier := range modifiers.Nodes {
					if modifier.Kind == ast.KindAsyncKeyword {
						note(report(c, modifier, EK1003, "async"), "Remove async.")
					}
				}
			}
		case ast.KindAwaitExpression:
			// TypeScript's own errors, such as await outside an async function,
			// restate EK1003.
			note(reportAt(c, file, scanner.GetRangeOfTokenAtPosition(file, node.Pos()), EK1003, "await"), "Remove await: the call runs and waits.")
			cover(c, node)
		case ast.KindForOfStatement:
			if await := node.AsForInOrOfStatement().AwaitModifier; await != nil {
				note(report(c, await, EK1003, "for await"), "Remove await: for...of over a producer or a Stream already waits.")
				cover(c, await)
			}
		case ast.KindVariableDeclarationList:
			switch node.Flags & ast.NodeFlagsBlockScoped {
			case ast.NodeFlagsAwaitUsing:
				note(reportAt(c, file, scanner.GetRangeOfTokenAtPosition(file, node.Pos()), EK1003, "await using"), "Write using: it already waits for the acquisition and the release.")
				cover(c, node)
				if isAtModuleTopLevel(node) {
					reportUsingAtTopLevel(c, file, node)
				}
			case ast.NodeFlagsUsing:
				if isAtModuleTopLevel(node) {
					reportUsingAtTopLevel(c, file, node)
				}
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// reportUsingAtTopLevel reports EK1004 at the using keyword of list.
func reportUsingAtTopLevel(c *checker.Checker, file *ast.SourceFile, list *ast.Node) {
	pos := list.Pos()
	if list.Flags&ast.NodeFlagsBlockScoped == ast.NodeFlagsAwaitUsing {
		pos = scanner.GetRangeOfTokenAtPosition(file, pos).End() // skip await
	}
	note(reportAt(c, file, scanner.GetRangeOfTokenAtPosition(file, pos), EK1004), "Move it into a function.")
}

// isAtModuleTopLevel reports whether node runs as part of its module's top
// level: no function encloses it. A class static block runs when the module
// defines the class, so it counts as the top level.
func isAtModuleTopLevel(node *ast.Node) bool {
	for n := node.Parent; n != nil; n = n.Parent {
		if ast.IsFunctionLikeDeclaration(n) {
			return false
		}
	}
	return true
}
