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
	case "error", "service", "control", "layer":
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
	case "service":
		return p.parseServiceDeclaration(pos, jsdoc, modifiers, false /*control*/)
	case "control":
		return p.parseServiceDeclaration(pos, jsdoc, modifiers, true /*control*/)
	case "layer":
		return p.parseLayerDeclaration(pos, jsdoc, modifiers)
	}
	return nil
}

// effeckSynthesized finishes node as synthesized at anchor, the range of the
// source token it stands for. The anchor must not be empty: TypeScript treats
// an empty node as missing, and resolves no name for it. Identifiers and
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

// effeckModifiers adds synthesized modifiers of the given kinds, anchored on
// a member's name, to modifiers, before the modifiers it already has. A kind
// it already has is not added again.
func (p *Parser) effeckModifiers(modifiers *ast.ModifierList, name *ast.Node, kinds ...ast.Kind) *ast.ModifierList {
	var added []*ast.Node
	for _, kind := range kinds {
		if modifiers != nil && core.Some(modifiers.Nodes, func(m *ast.Node) bool { return m.Kind == kind }) {
			continue
		}
		added = append(added, p.effeckSynthesized(p.factory.NewModifier(kind), name.Loc))
	}
	if len(added) == 0 {
		return modifiers
	}
	if modifiers == nil {
		return p.newModifierList(name.Loc, added)
	}
	return p.newModifierList(modifiers.Loc, append(added, modifiers.Nodes...))
}

// effeckAbstractMember adds abstract, and readonly for a property, to a
// member of a service or control body that has no body or initializer and is
// not static.
func (p *Parser) effeckAbstractMember(modifiers *ast.ModifierList, name *ast.Node, property bool) *ast.ModifierList {
	if !p.effeckAbstractMembers || modifiers != nil && core.Some(modifiers.Nodes, func(m *ast.Node) bool { return m.Kind == ast.KindStaticKeyword }) {
		return modifiers
	}
	if property {
		return p.effeckModifiers(modifiers, name, ast.KindAbstractKeyword, ast.KindReadonlyKeyword)
	}
	return p.effeckModifiers(modifiers, name, ast.KindAbstractKeyword)
}

// effeckReadonly adds a synthesized readonly modifier, anchored on the
// property's name, to the modifiers of a property signature, unless it has one.
func (p *Parser) effeckReadonly(modifiers *ast.ModifierList, name *ast.Node) *ast.ModifierList {
	if modifiers != nil {
		for _, modifier := range modifiers.Nodes {
			if modifier.Kind == ast.KindReadonlyKeyword {
				return modifiers
			}
		}
	}
	readonly := p.effeckSynthesized(p.factory.NewModifier(ast.KindReadonlyKeyword), name.Loc)
	if modifiers == nil {
		return p.newModifierList(name.Loc, []*ast.Node{readonly})
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
		fields = p.effeckSynthesized(p.factory.NewTypeLiteralNode(p.newNodeList(name.Loc, nil)), name.Loc)
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

// Effect types.
//
// In an .effeck file a type followed by ! is an Effect type written
// Result!Error!Dependencies:
//
//	User!NotFound!Db   Effect.Effect<User, NotFound, Db>
//	User!NotFound!     Effect.Effect<User, NotFound, never>
//	User!!Db           Effect.Effect<User, never, Db>
//	User!!             Effect.Effect<User, never, never>
//
// A slot without a type is never. Dependencies left out entirely, as in
// User!NotFound, are never too, except in the return annotation of a function
// with a body, where the slot is `infer R` and the body supplies it (D57;
// parseReturnType calls effeckInferOmitted). Unions bind inside a slot:
// User!NotFound | Timeout!Db fails with NotFound | Timeout. A slot that starts
// with { is empty, because in a declaration the { starts the body; put an
// object type in a slot in parentheses. A slot's type starts on the line of
// its !, so a member that ends in `void!!` does not take the next member.
//
// The reference is Effect.Effect, which names Effect's type whether Effect is
// the prelude's or imported from "effect", so a declaration file reads
// Effect.Effect<A, E, R>.

// parseEffeckEffectType parses the slots that follow result when the current
// token is the ! that makes result an Effect type. Otherwise it returns result.
func (p *Parser) parseEffeckEffectType(result *ast.TypeNode) *ast.TypeNode {
	if p.token != ast.KindExclamationToken || p.hasPrecedingLineBreak() || !p.isEffeckFile() {
		return result
	}
	bang := p.scanner.TokenRange() // synthesized nodes stand on the first !
	errors := p.parseEffeckSlot()
	var dependencies *ast.Node
	omitted := p.token != ast.KindExclamationToken
	if omitted {
		// Stand on the last character of the errors, so the type arguments
		// stay in source order and the list does not seem to end in a comma.
		last := core.NewTextRange(errors.End()-1, errors.End())
		dependencies = p.effeckSynthesized(p.factory.NewKeywordTypeNode(ast.KindNeverKeyword), last)
	} else {
		dependencies = p.parseEffeckSlot()
	}
	span := core.NewTextRange(result.Pos(), dependencies.End())
	module := p.effeckSynthesized(p.factory.NewIdentifier("Effect"), bang)
	member := p.effeckSynthesized(p.factory.NewIdentifier("Effect"), bang)
	name := p.effeckSynthesized(p.factory.NewQualifiedName(module, member), bang)
	arguments := p.newNodeList(span, []*ast.Node{result, errors, dependencies})
	reference := p.effeckSpanning(p.factory.NewTypeReferenceNode(name, arguments), span)
	if omitted {
		if p.effeckOmittedDependencies == nil {
			p.effeckOmittedDependencies = map[*ast.Node]bool{}
		}
		p.effeckOmittedDependencies[reference] = true
	}
	return reference
}

// parseEffeckSlot parses the ! at the current token and the type after it, or
// returns a synthesized never when no type follows on the same line.
func (p *Parser) parseEffeckSlot() *ast.Node {
	bang := p.scanner.TokenRange()
	p.nextToken()
	if p.token == ast.KindExclamationToken || p.token == ast.KindOpenBraceToken || p.hasPrecedingLineBreak() || !p.isStartOfType(false /*inStartOfParameter*/) {
		return p.effeckSynthesized(p.factory.NewKeywordTypeNode(ast.KindNeverKeyword), bang)
	}
	return p.parseEffeckSlotType()
}

// parseEffeckSlotType parses a type as parseType does, but leaves a ! after it
// for the next slot.
func (p *Parser) parseEffeckSlotType() *ast.TypeNode {
	saveContextFlags := p.contextFlags
	p.setContextFlags(ast.NodeFlagsTypeExcludesFlags, false)
	var typeNode *ast.TypeNode
	if p.isStartOfFunctionTypeOrConstructorType() {
		typeNode = p.parseFunctionOrConstructorType()
	} else {
		pos := p.nodePos()
		typeNode = p.parseUnionTypeOrHigher()
		if !p.inDisallowConditionalTypesContext() && !p.hasPrecedingLineBreak() && p.parseOptional(ast.KindExtendsKeyword) {
			extendsType := p.doInContext(ast.NodeFlagsDisallowConditionalTypesContext, true, (*Parser).parseType)
			p.parseExpected(ast.KindQuestionToken)
			trueType := p.doInContext(ast.NodeFlagsDisallowConditionalTypesContext, false, (*Parser).parseType)
			p.parseExpected(ast.KindColonToken)
			falseType := p.doInContext(ast.NodeFlagsDisallowConditionalTypesContext, false, (*Parser).parseEffeckSlotType)
			typeNode = p.finishNode(p.factory.NewConditionalTypeNode(typeNode, extendsType, trueType, falseType), pos)
		}
	}
	p.contextFlags = saveContextFlags
	return typeNode
}

// effeckInferOmitted turns dependencies left out of returnType into `infer R`
// when returnType is the whole return annotation of a function whose body
// follows, so the body supplies them.
func (p *Parser) effeckInferOmitted(returnType *ast.TypeNode) {
	if returnType == nil || !p.effeckOmittedDependencies[returnType] {
		return
	}
	if p.token != ast.KindOpenBraceToken && p.token != ast.KindEqualsGreaterThanToken {
		return
	}
	arguments := returnType.TypeArgumentList()
	slot := arguments.Nodes[2]
	name := p.effeckSynthesized(p.factory.NewIdentifier("R"), slot.Loc)
	parameter := p.effeckSynthesized(p.factory.NewTypeParameterDeclaration(nil, name, nil, nil, nil), slot.Loc)
	infer := p.effeckSynthesized(p.factory.NewInferTypeNode(parameter), slot.Loc)
	infer.Parent = returnType
	arguments.Nodes[2] = infer
}

// parseServiceDeclaration parses
//
//	service Users { find(id: string): User!NotFound; readonly region: string }
//	control Approval { approve(request: string): boolean!! }
//
// as the abstract classes they abbreviate:
//
//	abstract class Users { abstract find(id: string): User!NotFound; abstract readonly region: string }
//	abstract class Approval extends Control { abstract approve(request: string): boolean!! }
//
// A member without a body or initializer that is not static becomes abstract,
// and a value member also readonly. Static members, such as a layer, and
// members with bodies stay as written. A member's contract is written in the
// ! form; without a ! a return type is not an Effect (EK2101).
func (p *Parser) parseServiceDeclaration(pos int, jsdoc jsdocScannerInfo, modifiers *ast.ModifierList, control bool) *ast.Node {
	word := p.scanner.TokenRange()
	p.nextToken() // service or control
	name := p.parseBindingIdentifier()
	typeParameters := p.parseTypeParameters()
	var heritage *ast.NodeList
	if control {
		base := p.effeckSynthesized(p.factory.NewIdentifier("Control"), word)
		expression := p.effeckSynthesized(p.factory.NewExpressionWithTypeArguments(base, nil), word)
		clause := p.effeckSynthesized(p.factory.NewHeritageClause(ast.KindExtendsKeyword, p.newNodeList(word, []*ast.Node{expression})), word)
		heritage = p.newNodeList(word, []*ast.Node{clause})
	}
	var members *ast.NodeList
	if p.parseExpected(ast.KindOpenBraceToken) {
		saveAbstract := p.effeckAbstractMembers
		p.effeckAbstractMembers = true
		members = p.parseList(PCClassMembers, (*Parser).parseClassElement)
		p.effeckAbstractMembers = saveAbstract
		p.parseExpected(ast.KindCloseBraceToken)
	} else {
		members = p.createMissingList()
	}
	abstract := p.effeckSynthesized(p.factory.NewModifier(ast.KindAbstractKeyword), word)
	if modifiers == nil {
		modifiers = p.newModifierList(word, []*ast.Node{abstract})
	} else {
		modifiers = p.newModifierList(core.NewTextRange(modifiers.Pos(), word.End()), append(append([]*ast.Node{}, modifiers.Nodes...), abstract))
	}
	result := p.factory.NewClassDeclaration(modifiers, name, typeParameters, heritage, members)
	p.finishNode(result, pos)
	p.withJSDoc(result, jsdoc)
	return result
}

// parseLayerDeclaration parses
//
//	layer UsersLive: Users {
//	  const directory = Directory()
//	  find(id) { ... }
//	  readonly region = "eu"
//	}
//
// as the layer it abbreviates:
//
//	const UsersLive = Layer.effect(Users, () => {
//	  const directory = Directory()
//	  return { find(id) { ... }, region: "eu" }
//	})
//
// The statements run once, when the layer builds, and the members form the
// implementation. Without statements it is Layer.succeed(Users, { ... }). A
// member is a method, name(params) { ... } or *name(params) { ... }, or a
// value, readonly name = expression. Statements come before the members;
// the checker reports one that follows them (EK2104).
func (p *Parser) parseLayerDeclaration(pos int, jsdoc jsdocScannerInfo, modifiers *ast.ModifierList) *ast.Node {
	word := p.scanner.TokenRange()
	p.nextToken() // layer
	name := p.parseBindingIdentifier()
	p.parseExpected(ast.KindColonToken)
	service := p.parseEffeckEntityExpression()
	bodyStart := p.nodePos()
	var statements, members []*ast.Node
	if p.parseExpected(ast.KindOpenBraceToken) {
		for p.token != ast.KindCloseBraceToken && p.token != ast.KindEndOfFile {
			switch {
			case p.lookAhead((*Parser).scanLayerValueStart):
				members = append(members, p.parseLayerValue())
			case p.lookAhead((*Parser).scanLayerMethodStart):
				memberPos := p.nodePos()
				memberJSDoc := p.jsdocScannerInfo()
				asterisk := p.parseOptionalToken(ast.KindAsteriskToken)
				memberName := p.parsePropertyName()
				members = append(members, p.parseMethodDeclaration(memberPos, memberJSDoc, nil /*modifiers*/, asterisk, memberName, nil /*questionToken*/, nil /*diagnosticMessage*/))
			default:
				statements = append(statements, p.parseStatement())
			}
		}
		p.parseExpected(ast.KindCloseBraceToken)
	}
	bodyEnd := p.nodePos()
	body := core.NewTextRange(bodyStart, bodyEnd)
	membersLoc := body
	if len(members) > 0 {
		membersLoc = core.NewTextRange(members[0].Pos(), members[len(members)-1].End())
	}
	implementation := p.effeckSpanning(p.factory.NewObjectLiteralExpression(p.newNodeList(membersLoc, members), true /*multiLine*/), membersLoc)
	method := "succeed"
	argument := implementation
	if len(statements) > 0 {
		method = "effect"
		returned := p.effeckSpanning(p.factory.NewReturnStatement(implementation), membersLoc)
		block := p.effeckSpanning(p.factory.NewBlock(p.newNodeList(body, append(statements, returned)), true /*multiLine*/), body)
		arrow := p.effeckSynthesized(p.factory.NewToken(ast.KindEqualsGreaterThanToken), word)
		argument = p.effeckSpanning(p.factory.NewArrowFunction(nil, nil, p.newNodeList(core.NewTextRange(bodyStart, bodyStart), nil), nil, nil, arrow, block), body)
	}
	layer := p.effeckSynthesized(p.factory.NewIdentifier("Layer"), word)
	constructor := p.effeckSynthesized(p.factory.NewIdentifier(method), word)
	callee := p.effeckSynthesized(p.factory.NewPropertyAccessExpression(layer, nil, constructor, ast.NodeFlagsNone), word)
	span := core.NewTextRange(service.Pos(), bodyEnd)
	call := p.effeckSpanning(p.factory.NewCallExpression(callee, nil, nil, p.newNodeList(span, []*ast.Node{service, argument}), ast.NodeFlagsNone), span)
	declarationSpan := core.NewTextRange(name.Pos(), bodyEnd)
	declaration := p.effeckSpanning(p.factory.NewVariableDeclaration(name, nil, nil, call), declarationSpan)
	list := p.effeckSpanning(p.factory.NewVariableDeclarationList(p.newNodeList(declarationSpan, []*ast.Node{declaration}), ast.NodeFlagsConst), declarationSpan)
	result := p.factory.NewVariableStatement(modifiers, list)
	p.finishNode(result, pos)
	p.withJSDoc(result, jsdoc)
	return result
}

// parseEffeckEntityExpression parses a name or a dotted name as an expression:
// Users or Accounts.Users.
func (p *Parser) parseEffeckEntityExpression() *ast.Node {
	pos := p.nodePos()
	expression := p.parseIdentifier()
	for p.parseOptional(ast.KindDotToken) {
		member := p.parseIdentifierName()
		expression = p.finishNode(p.factory.NewPropertyAccessExpression(expression, nil, member, ast.NodeFlagsNone), pos)
	}
	return expression
}

// scanLayerValueStart reports whether a layer value member starts here:
// readonly name =. It moves the scanner; call it through lookAhead.
func (p *Parser) scanLayerValueStart() bool {
	if p.token != ast.KindReadonlyKeyword {
		return false
	}
	p.nextToken()
	if !tokenIsIdentifierOrKeyword(p.token) {
		return false
	}
	p.nextToken()
	return p.token == ast.KindEqualsToken
}

// parseLayerValue parses readonly name = expression as the property name: expression.
func (p *Parser) parseLayerValue() *ast.Node {
	pos := p.nodePos()
	p.nextToken() // readonly
	name := p.parsePropertyName()
	p.parseExpected(ast.KindEqualsToken)
	initializer := p.parseAssignmentExpressionOrHigher()
	p.parseSemicolon()
	return p.finishNode(p.factory.NewPropertyAssignment(nil, name, nil, nil, initializer), pos)
}

// scanLayerMethodStart reports whether a layer method starts here: an
// optional *, a name, a parameter list, and then a return type or a { on the
// same line. A call statement cannot be followed by either, so the form
// cannot change the meaning of code that parses today. It moves the scanner;
// call it through lookAhead.
func (p *Parser) scanLayerMethodStart() bool {
	if p.token == ast.KindAsteriskToken {
		p.nextToken()
	}
	if !tokenIsIdentifierOrKeyword(p.token) {
		return false
	}
	p.nextToken()
	if p.token != ast.KindOpenParenToken {
		return false
	}
	for depth := 0; ; {
		switch p.token {
		case ast.KindOpenParenToken:
			depth++
		case ast.KindCloseParenToken:
			depth--
		case ast.KindEndOfFile:
			return false
		}
		p.nextToken()
		if depth == 0 {
			break
		}
	}
	return p.token == ast.KindColonToken || p.token == ast.KindOpenBraceToken && !p.hasPrecedingLineBreak()
}

// parseEffeckWithStatement parses
//
//	with UsersLive {
//	  greet("42")
//	}
//
// which runs its statements with the services the layer provides, as
// Effect.provide(() => { ... }, UsersLive) would, except that the statements
// are code of the enclosing function: a return returns from it, and a break
// or continue leaves its loops (D130). The block parses as a Block flagged
// NodeFlagsEffeck whose first statement holds the layer expression, so the
// checker reads both as ordinary code; the analyzer and the lowering
// recognize the flag. In an .effeck file, a module, JavaScript's with
// statement is a syntax error, so the form takes nothing away.
func (p *Parser) parseEffeckWithStatement() *ast.Node {
	pos := p.nodePos()
	p.parseExpected(ast.KindWithKeyword)
	layer := p.parseExpression()
	statement := p.effeckSpanning(p.factory.NewExpressionStatement(layer), layer.Loc)
	statement.Flags |= ast.NodeFlagsEffeck
	statements := []*ast.Node{statement}
	if p.parseExpected(ast.KindOpenBraceToken) {
		body := p.parseList(PCBlockStatements, (*Parser).parseStatement)
		statements = append(statements, body.Nodes...)
		p.parseExpected(ast.KindCloseBraceToken)
	}
	block := p.factory.NewBlock(p.newNodeList(core.NewTextRange(layer.Pos(), p.nodePos()), statements), true /*multiLine*/)
	p.finishNode(block, pos)
	block.Flags |= ast.NodeFlagsEffeck
	return block
}
