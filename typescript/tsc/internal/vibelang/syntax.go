package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
	"github.com/microsoft/TypeScript/tsc/internal/vibelangmapper"
)

// The checks in this file report TypeScript syntax that VibeLang does not
// have. A call already waits, so async, for await, and await using are
// EK1003 (Chapter 10). The parser writes await outside an async function as
// $await (fibers.go), so an AwaitExpression here is inside an async function. A module's top level is not inside a program, so using
// there has no block to release it in: EK1004 (Chapter 6). D271 exempts
// explicit JavaScript iterator generators and their own body syntax.

var EK1015 = message(1015, errorCategory, "Calls run in VibeLang; remove yield*.")

func init() {
	registry[EK1015.Code] = EK1015
	fileChecks = append(fileChecks, checkSyntax, checkThrowExpressionPrecedence)
}

func checkSyntax(c *checker.Checker, file *ast.SourceFile) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if hatchOf(node) != "" {
			return forEachHatchVibeLang(node, visit) // async and await keep their JavaScript meaning in a hatch's TypeScript (D148)
		}
		if fn := ast.GetContainingFunction(node); fn != nil && !isFunctionBoundary(node) &&
			ast.GetFunctionFlags(fn)&ast.FunctionFlagsGenerator != 0 && vibelangmapper.HasJavaScriptIteratorReturn(fn) {
			// Await, for await and await using are JavaScript inside this
			// generator. Nested functions still own their VibeLang rules.
			return node.ForEachChild(visit)
		}
		switch node.Kind {
		case ast.KindFunctionDeclaration, ast.KindFunctionExpression, ast.KindArrowFunction, ast.KindMethodDeclaration:
			if ast.GetFunctionFlags(node)&ast.FunctionFlagsGenerator != 0 && vibelangmapper.HasJavaScriptIteratorReturn(node) {
				break // D271 keeps async function* as TypeScript writes it.
			}
			if modifiers := node.Modifiers(); modifiers != nil {
				for _, modifier := range modifiers.Nodes {
					if modifier.Kind == ast.KindAsyncKeyword {
						note(report(c, modifier, EK1003, "async"), "Remove async.")
					}
				}
			}
		case ast.KindYieldExpression:
			fn := ast.GetContainingFunction(node)
			if node.AsYieldExpression().AsteriskToken != nil && (fn == nil || ast.GetFunctionFlags(fn)&ast.FunctionFlagsGenerator == 0) {
				report(c, node, EK1015)
				cover(c, node)
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

// throwExpressionPrecedenceOperand recognizes D186's parser-generated
// (() => { throw operand })() by both its shape and its source keyword.
// A handwritten IIFE starts with '(' instead; do/match IIFEs have other
// keywords. Inspect the immediate parent so parentheses around the throw
// itself remain explicit, and right operands/conditional arms stay legal.
func throwExpressionPrecedenceOperand(file *ast.SourceFile, node *ast.Node) *ast.Node {
	if node.Kind != ast.KindCallExpression || node.Parent == nil || node.Pos() < 0 || node.End() > len(file.Text()) {
		return nil
	}
	parent := node.Parent
	switch parent.Kind {
	case ast.KindBinaryExpression:
		binary := parent.AsBinaryExpression()
		if binary.Left != node || binary.OperatorToken.Kind == ast.KindCommaToken {
			return nil
		}
	case ast.KindConditionalExpression:
		if parent.AsConditionalExpression().Condition != node {
			return nil
		}
	case ast.KindAsExpression, ast.KindSatisfiesExpression:
		if parent.Expression() != node {
			return nil
		}
	case ast.KindPropertyAccessExpression:
		// D272 normalizes throw x |> f(args)(%) to (throw x).pipe(f(args)).
		// Its generated 'pipe' name is anchored on '|', unlike a written member.
		if parent.Expression() != node || parent.Name().Text() != "pipe" {
			return nil
		}
		operator := scanner.GetRangeOfTokenAtPosition(file, parent.Name().Pos())
		if file.Text()[operator.Pos():operator.End()] != "|" {
			return nil
		}
	default:
		return nil
	}
	keyword := scanner.GetRangeOfTokenAtPosition(file, node.Pos())
	if file.Text()[keyword.Pos():keyword.End()] != "throw" {
		return nil
	}
	callee := node.Expression()
	if len(node.Arguments()) != 0 || callee.Kind != ast.KindParenthesizedExpression {
		return nil
	}
	arrow := callee.Expression()
	if arrow.Kind != ast.KindArrowFunction || arrow.Body() == nil || arrow.Body().Kind != ast.KindBlock {
		return nil
	}
	statements := arrow.Body().Statements()
	if len(statements) != 1 || statements[0].Kind != ast.KindThrowStatement {
		return nil
	}
	return statements[0].Expression()
}

// EK1010 checks parser-generated throw expressions even in a hatch's
// TypeScript callback: D148 keeps throw statements there, but the same
// unary expression grammar must not silently discard the remaining text.
// The existing checkSyntax keeps its keyword/using positions and hatch gate.
func checkThrowExpressionPrecedence(c *checker.Checker, file *ast.SourceFile) {
	var visit func(*ast.Node) bool
	visit = func(node *ast.Node) bool {
		if operand := throwExpressionPrecedenceOperand(file, node); operand != nil {
			report(c, node, EK1010, scanner.GetSourceTextOfNodeFromSourceFile(file, operand, false))
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}
