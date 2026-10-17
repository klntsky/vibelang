package parser

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

// Effeck (.effeck) files add short forms to TypeScript's syntax. Each form
// parses into the TypeScript nodes it abbreviates, so the binder, checker,
// emitter, and language service handle it exactly like the long form.
//
// A node that has no source text of its own is synthesized: it takes the
// location of the source token it stands for, NodeFlagsReparsed keeps the
// language service from treating it as source, and, for identifiers and
// string literals, NodeFlagsReparserTransformedLiteral makes the printer
// print the node's own text instead of the source text at its location.

// isEffeckFile reports whether the parser is reading an .effeck file.
func (p *Parser) isEffeckFile() bool {
	return tspath.FileExtensionIs(string(p.opts.FileName), tspath.ExtensionEffeck)
}

// isEffeckDeclarationWord reports whether text starts an Effeck declaration
// when an identifier follows it on the same line. Two identifiers in a row are
// a TypeScript syntax error, so these forms never change the meaning of code
// that parses today.
func isEffeckDeclarationWord(text string) bool {
	switch text {
	case "error":
		return true
	}
	return false
}

// scanStartOfEffeckDeclaration reports whether the current token starts an
// Effeck declaration. It moves the scanner; call it through lookAhead.
func (p *Parser) scanStartOfEffeckDeclaration() bool {
	return p.token == ast.KindIdentifier && isEffeckDeclarationWord(p.scanner.TokenValue()) && p.nextTokenIsIdentifierOnSameLine()
}

func (p *Parser) isStartOfEffeckDeclaration() bool {
	return p.isEffeckFile() && p.lookAhead((*Parser).scanStartOfEffeckDeclaration)
}

// parseEffeckDeclaration parses the Effeck declaration that starts at the
// current token, or returns nil when the token starts none.
func (p *Parser) parseEffeckDeclaration(pos int, jsdoc jsdocScannerInfo, modifiers *ast.ModifierList) *ast.Node {
	if !p.isStartOfEffeckDeclaration() {
		return nil
	}
	switch p.scanner.TokenValue() {
	case "error":
		return p.parseErrorDeclaration(pos, jsdoc, modifiers)
	}
	return nil
}

// effeckSynthesized finishes node as synthesized at anchor. Identifiers and
// string literals print their own text; a string literal anchored on an
// identifier prints that identifier's text in quotes.
func (p *Parser) effeckSynthesized(node *ast.Node, anchor core.TextRange) *ast.Node {
	node.Loc = anchor
	node.Flags |= p.contextFlags | ast.NodeFlagsReparsed
	if node.Kind == ast.KindIdentifier || node.Kind == ast.KindStringLiteral {
		node.Flags |= ast.NodeFlagsReparserTransformedLiteral
	}
	p.overrideParentInImmediateChildren(node)
	return node
}

// effeckSpanning finishes node, which has no source text of its own but holds
// source nodes, at loc, the range its source children cover, so the language
// service still descends into them.
func (p *Parser) effeckSpanning(node *ast.Node, loc core.TextRange) *ast.Node {
	node.Loc = loc
	node.Flags |= p.contextFlags
	p.overrideParentInImmediateChildren(node)
	return node
}

// effeckReadonly adds a synthesized readonly modifier to the modifiers of the
// property signature that starts at pos, unless it has one.
func (p *Parser) effeckReadonly(modifiers *ast.ModifierList, pos int) *ast.ModifierList {
	if modifiers != nil {
		for _, modifier := range modifiers.Nodes {
			if modifier.Kind == ast.KindReadonlyKeyword {
				return modifiers
			}
		}
	}
	anchor := core.NewTextRange(pos, pos)
	readonly := p.effeckSynthesized(p.factory.NewModifier(ast.KindReadonlyKeyword), anchor)
	if modifiers == nil {
		return p.newModifierList(anchor, []*ast.Node{readonly})
	}
	return p.newModifierList(modifiers.Loc, append([]*ast.Node{readonly}, modifiers.Nodes...))
}

// parseErrorDeclaration parses
//
//	error NotFound { id: string }
//
// as the class it abbreviates:
//
//	class NotFound extends TaggedError("NotFound")<{ readonly id: string }> {}
//
// The tag is the class name, every field is readonly, and the body is
// optional: `error Timeout` declares an error without fields.
func (p *Parser) parseErrorDeclaration(pos int, jsdoc jsdocScannerInfo, modifiers *ast.ModifierList) *ast.Node {
	p.nextToken() // error
	name := p.parseBindingIdentifier()
	typeParameters := p.parseTypeParameters()
	var fields *ast.Node
	if p.token == ast.KindOpenBraceToken {
		saveReadonly := p.effeckReadonlyMembers
		p.effeckReadonlyMembers = true
		fields = p.parseTypeLiteral()
		p.effeckReadonlyMembers = saveReadonly
	} else {
		empty := core.NewTextRange(name.End(), name.End())
		fields = p.effeckSynthesized(p.factory.NewTypeLiteralNode(p.newNodeList(empty, nil)), empty)
	}
	end := p.nodePos()
	tag := p.effeckSynthesized(p.factory.NewStringLiteral(name.Text(), ast.TokenFlagsNone), name.Loc)
	callee := p.effeckSynthesized(p.factory.NewIdentifier("TaggedError"), name.Loc)
	call := p.effeckSynthesized(p.factory.NewCallExpression(callee, nil, nil, p.newNodeList(name.Loc, []*ast.Node{tag}), ast.NodeFlagsNone), name.Loc)
	base := p.effeckSpanning(p.factory.NewExpressionWithTypeArguments(call, p.newNodeList(fields.Loc, []*ast.Node{fields})), fields.Loc)
	clause := p.effeckSpanning(p.factory.NewHeritageClause(ast.KindExtendsKeyword, p.newNodeList(fields.Loc, []*ast.Node{base})), fields.Loc)
	members := p.newNodeList(core.NewTextRange(end, end), nil)
	result := p.factory.NewClassDeclaration(modifiers, name, typeParameters, p.newNodeList(fields.Loc, []*ast.Node{clause}), members)
	p.finishNode(result, pos)
	p.withJSDoc(result, jsdoc)
	return result
}
