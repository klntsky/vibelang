package astnav

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

// Effeck (.effeck) forms parse into the TypeScript nodes they abbreviate
// (parser/effeck.go), and the nodes a form synthesizes carry
// NodeFlagsReparsed, so this package's walk skips them. Two things then leave
// a source token where getTokenAtPosition expects only trivia between a node's
// children:
//
//   - The word that names a form, such as error, match, when, join, or the all
//     of await.all, is an identifier with no node of its own.
//   - A desugared node can hold source children out of source order, or
//     outside its own range, so a binary search by position misses them:
//     for concurrent(8) (const id of ids) passes ids before 8, and its body
//     function starts after the binding id.
//
// TypeScript code has neither, so outside .effeck files an identifier in
// trivia still panics, as upstream.

func isEffeckSourceFile(sourceFile *ast.SourceFile) bool {
	return tspath.FileExtensionIs(string(sourceFile.FileName()), tspath.ExtensionEffeck)
}

// effeckSourceToken returns the source node that is exactly the token at
// [start, end), found below current or else below its nearest ancestor that
// holds it, or nil. A desugared node can range over a sibling's source child,
// as the body function of for concurrent ranges over its items, so the walk
// can stop below the node that holds the token. It visits every child in
// order rather than searching by position, and never descends into a
// synthesized node, so the node it returns has only source nodes above it.
func effeckSourceToken(sourceFile *ast.SourceFile, current *ast.Node, start int, end int) *ast.Node {
	var found *ast.Node
	var visit ast.Visitor
	visit = func(node *ast.Node) bool {
		if node.Flags&ast.NodeFlagsReparsed != 0 || node.End() < end ||
			scanner.GetTokenPosOfNode(node, sourceFile, false /*includeJSDoc*/) > start {
			return false
		}
		if node.End() == end && scanner.GetTokenPosOfNode(node, sourceFile, false /*includeJSDoc*/) == start && ast.IsTokenKind(node.Kind) {
			found = node
			return true
		}
		return node.ForEachChild(visit)
	}
	for scope := current; scope != nil && found == nil; scope = scope.Parent {
		scope.ForEachChild(visit)
	}
	return found
}

// effeckWordNode returns the node that stands for the word of an Effeck form
// at the scanner's current token, where no source node covers it. The word
// of a declaration that parses into a class, error, service, control, or the
// older layer, is that class's class keyword, and the word of one that parses
// into a const, schema or layer, is its const keyword, so the language
// service treats each as TypeScript treats those keywords: rename and go to
// definition move to the declared name. Any other word stands for the form
// itself, the node that contains it, which hovers describe.
func effeckWordNode(sourceFile *ast.SourceFile, current *ast.Node, s *scanner.Scanner, fullStart int, end int, flags ast.TokenFlags) *ast.Node {
	switch s.TokenValue() {
	case "error", "service", "control", "layer", "schema":
		if ast.IsClassDeclaration(current) {
			return sourceFile.GetOrCreateToken(ast.KindClassKeyword, fullStart, end, current, flags)
		}
		if ast.IsVariableStatement(current) {
			if list := current.AsVariableStatement().DeclarationList; list != nil && list.Flags&ast.NodeFlagsReparsed == 0 {
				return sourceFile.GetOrCreateToken(ast.KindConstKeyword, fullStart, end, list, flags)
			}
		}
	}
	return current
}
