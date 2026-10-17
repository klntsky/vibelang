package parser

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
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
	case "error", "service", "control", "layer", "schema":
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
	case "schema":
		return p.parseSchemaDeclaration(pos, jsdoc, modifiers)
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
	if node.Kind == ast.KindIdentifier || node.Kind == ast.KindStringLiteral || node.Kind == ast.KindNumericLiteral {
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

// Readonly members (D134). In an .effeck file a property signature or an
// index signature of an interface or an object type is readonly unless it is
// written with mutable:
//
//	interface Counter { label: string; mutable count: number }
//
// reads as `interface Counter { readonly label: string; count: number }`.
// mutable is a modifier only when a member name follows it on the same line,
// so a member named mutable still parses as before.

// parseEffeckMutable consumes mutable at the start of a type member and
// reports whether it did.
func (p *Parser) parseEffeckMutable() bool {
	if p.token != ast.KindIdentifier || p.scanner.TokenValue() != "mutable" || !p.lookAhead((*Parser).nextTokenStartsMemberNameOnSameLine) {
		return false
	}
	p.nextToken()
	return true
}

// nextTokenStartsMemberNameOnSameLine reports whether the next token, on the
// same line, starts a member's name or an index signature.
func (p *Parser) nextTokenStartsMemberNameOnSameLine() bool {
	p.nextToken()
	return !p.hasPrecedingLineBreak() && (p.isLiteralPropertyName() || p.token == ast.KindOpenBracketToken)
}

// effeckReadonly adds a synthesized readonly modifier, anchored on anchor, a
// member's name or its first token, to modifiers, unless they have one.
func (p *Parser) effeckReadonly(modifiers *ast.ModifierList, anchor core.TextRange) *ast.ModifierList {
	if modifiers != nil {
		for _, modifier := range modifiers.Nodes {
			if modifier.Kind == ast.KindReadonlyKeyword {
				return modifiers
			}
		}
	}
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
		fields = p.parseTypeLiteral() // readonly unless written mutable (D134)
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

// isStartOfEffeckProvide reports whether the current token starts a provide
// block: the word provide followed, on the same line, by a layer expression
// and an opening brace (D172). provide followed by anything else stays an
// identifier, so a local function or variable named provide keeps its
// meaning: provide(x) or provide[i] = v parse as before.
func (p *Parser) isStartOfEffeckProvide() bool {
	return p.token == ast.KindIdentifier && p.scanner.TokenValue() == "provide" && p.isEffeckFile() && p.lookAhead((*Parser).scanStartOfEffeckProvide)
}

func (p *Parser) scanStartOfEffeckProvide() bool {
	p.nextToken()
	if p.hasPrecedingLineBreak() || p.token == ast.KindOpenBraceToken {
		return false
	}
	if p.token == ast.KindOpenParenToken && p.lookAhead((*Parser).scanEffeckProvideArguments) {
		return true
	}
	if !p.isStartOfExpression() {
		return false
	}
	saveNoBlockParams := p.effeckNoBlockParams
	p.effeckNoBlockParams = true // the { after the providers opens the block (effeck_blocks.go)
	p.parseExpression()
	p.effeckNoBlockParams = saveNoBlockParams
	return p.token == ast.KindOpenBraceToken && !p.hasPrecedingLineBreak()
}

// scanEffeckProvideArguments reports whether the current token, (, opens the
// call shape of a provide block, provide(A, Key = value) { ... }: a provider
// list, a closing parenthesis, and { on the same line. The list is read as an
// expression, not as the parameters of an arrow function, which (a, b = {})
// followed by { would otherwise look like. It moves the scanner; call it
// through lookAhead.
func (p *Parser) scanEffeckProvideArguments() bool {
	p.nextToken()
	if p.token == ast.KindCloseParenToken {
		return false
	}
	p.parseExpression()
	if p.token != ast.KindCloseParenToken {
		return false
	}
	p.nextToken()
	return p.token == ast.KindOpenBraceToken && !p.hasPrecedingLineBreak()
}

// parseEffeckProvideStatement parses
//
//	provide UsersLive {
//	  greet("42")
//	}
//
// which runs its statements with the services the layer provides, as
// Effect.provide(() => { ... }, UsersLive) would, except that the statements
// are code of the enclosing function: a return returns from it, and a break
// or continue leaves its loops (D130, D172). The block parses as a Block
// flagged NodeFlagsEffeck whose first statement holds the layer expression,
// so the checker reads both as ordinary code; the analyzer and the lowering
// recognize the flag. The old spelling, with UsersLive { ... }, parses the
// same way and draws the warning EK1009; in an .effeck file, a module,
// JavaScript's with statement is a syntax error, so it took nothing away.
func (p *Parser) parseEffeckProvideStatement() *ast.Node {
	pos := p.nodePos()
	p.nextToken() // provide, or the old spelling with
	var providers *ast.Node
	if p.token == ast.KindOpenParenToken && p.lookAhead((*Parser).scanEffeckProvideArguments) {
		// provide(A, Key = value) { ... } means provide A, Key = value { ... },
		// as fork(x) means fork x.
		p.nextToken()
		providers = p.parseExpression()
		p.parseExpected(ast.KindCloseParenToken)
	} else {
		saveNoBlockParams := p.effeckNoBlockParams
		p.effeckNoBlockParams = true // the { after the providers opens the block (effeck_blocks.go)
		providers = p.parseExpression()
		p.effeckNoBlockParams = saveNoBlockParams
	}
	layer := p.effeckProviders(providers)
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

// effeckProviders rewrites the provider list of a provide block (D173).
// Providers separated by commas, which parse as a comma expression, become an
// array of layers, and a provider written Key = value becomes
// $provideService(Key, value, "Key"), a layer that provides that one service:
//
//	provide AppLive, Clock = testClock { ... }
//
// provides [AppLive, $provideService(Clock, testClock, "Clock")]. The name
// lets the runtime use a module's eponymous key, so Clock, the effect module,
// stands for Clock.Clock. An array literal provider adds its elements. A
// single provider that is not Key = value is returned unchanged.
func (p *Parser) effeckProviders(expression *ast.Node) *ast.Node {
	var items []*ast.Node
	var collect func(node *ast.Node)
	collect = func(node *ast.Node) {
		if ast.IsBinaryExpression(node) && node.AsBinaryExpression().OperatorToken.Kind == ast.KindCommaToken {
			collect(node.AsBinaryExpression().Left)
			collect(node.AsBinaryExpression().Right)
			return
		}
		items = append(items, node)
	}
	list := expression
	for ast.IsParenthesizedExpression(list) {
		list = list.Expression() // provide(A, Key = value) { ... } reads as provide A, Key = value { ... }
	}
	collect(list)
	services := false
	for i, item := range items {
		if !ast.IsBinaryExpression(item) || item.AsBinaryExpression().OperatorToken.Kind != ast.KindEqualsToken {
			continue
		}
		key, value := item.AsBinaryExpression().Left, item.AsBinaryExpression().Right
		name := key
		if ast.IsPropertyAccessExpression(name) {
			name = name.Name()
		}
		if !ast.IsIdentifier(name) {
			continue
		}
		services = true
		callee := p.effeckSynthesized(p.factory.NewIdentifier("$provideService"), key.Loc)
		label := p.effeckSynthesized(p.factory.NewStringLiteral(name.Text(), ast.TokenFlagsNone), key.Loc)
		args := p.newNodeList(item.Loc, []*ast.Node{key, value, label})
		items[i] = p.effeckSpanning(p.factory.NewCallExpression(callee, nil /*questionDotToken*/, nil /*typeArguments*/, args, ast.NodeFlagsNone), item.Loc)
	}
	if len(items) == 1 && !services {
		return expression
	}
	var elements []*ast.Node
	for _, item := range items {
		if ast.IsArrayLiteralExpression(item) {
			elements = append(elements, item.AsArrayLiteralExpression().Elements.Nodes...)
			continue
		}
		elements = append(elements, item)
	}
	if len(elements) == 1 {
		return elements[0]
	}
	return p.effeckSpanning(p.factory.NewArrayLiteralExpression(p.newNodeList(expression.Loc, elements), false /*multiLine*/), expression.Loc)
}

// parseEffeckCatchClauses parses the catch clauses that follow first, a parsed
// catch clause, when a catch clause has a type or several follow one try:
//
//	try { ... } catch (e: NotFound) { A } catch (e: Timeout | Offline) { B } catch (e) { C }
//
// parses as one catch clause whose block tests each type in turn:
//
//	catch ($caught) {
//	  if ($isFailure<NotFound>($caught)) { const e = $caught; A }
//	  else if ($isFailure<Timeout | Offline>($caught)) { const e = $caught; B }
//	  else { const e = $caught; C }
//	}
//
// $isFailure is a type guard, so each branch sees its clause's type and the
// branches after it see what is left; the lowering passes it the _tag of each
// type (D131). Without a final untyped clause, the chain ends in
// `throw $caught`, so a failure no clause names propagates with its type. A
// failure raised inside a clause is not tested by the clauses after it. One
// untyped clause stays as written.
func (p *Parser) parseEffeckCatchClauses(first *ast.Node) *ast.Node {
	clauses := []*ast.Node{first}
	for p.token == ast.KindCatchKeyword {
		clauses = append(clauses, p.parseCatchClause())
	}
	if len(clauses) == 1 && !isTypedCatchClause(first) {
		return first
	}
	anchor := clauseAnchor(first)
	causeMode := false
	for _, clause := range clauses {
		causeMode = causeMode || isCauseCatchClause(clause)
	}
	chain := p.effeckCatchChain(clauses, causeMode)
	span := core.NewTextRange(first.Pos(), clauses[len(clauses)-1].End())
	if !causeMode {
		caught := p.effeckSynthesized(p.factory.NewIdentifier(effeckCaughtName), anchor)
		variable := p.effeckSynthesized(p.factory.NewVariableDeclaration(caught, nil, nil, nil), anchor)
		block := p.effeckSpanning(p.factory.NewBlock(p.newNodeList(span, []*ast.Node{chain}), true /*multiLine*/), span)
		return p.effeckSpanning(p.factory.NewCatchClause(variable, block), span)
	}
	// A clause typed Cause catches every reason (D176). The catch variable is
	// the Cause, and the clauses before it test the expected failure it holds:
	//
	//	catch ($cause) {
	//	  const $caught = $failureOf($cause)
	//	  if ($isFailure<NotFound>($caught)) { const e = $caught; A }
	//	  else { const cause = $cause; C }
	//	}
	causeName := p.effeckSynthesized(p.factory.NewIdentifier(effeckCauseName), anchor)
	variable := p.effeckSynthesized(p.factory.NewVariableDeclaration(causeName, nil, nil, nil), anchor)
	failureOf := p.effeckSynthesized(p.factory.NewIdentifier(effeckFailureOf), anchor)
	argument := p.effeckSynthesized(p.factory.NewIdentifier(effeckCauseName), anchor)
	call := p.effeckSynthesized(p.factory.NewCallExpression(failureOf, nil, nil, p.newNodeList(anchor, []*ast.Node{argument}), ast.NodeFlagsNone), anchor)
	caught := p.effeckSynthesized(p.factory.NewIdentifier(effeckCaughtName), anchor)
	declaration := p.effeckSynthesized(p.factory.NewVariableDeclaration(caught, nil, nil, call), anchor)
	list := p.effeckSynthesized(p.factory.NewVariableDeclarationList(p.newNodeList(anchor, []*ast.Node{declaration}), ast.NodeFlagsConst), anchor)
	statement := p.effeckSynthesized(p.factory.NewVariableStatement(nil, list), anchor)
	block := p.effeckSpanning(p.factory.NewBlock(p.newNodeList(span, []*ast.Node{statement, chain}), true /*multiLine*/), span)
	return p.effeckSpanning(p.factory.NewCatchClause(variable, block), span)
}

// effeckCaughtName is the variable of the catch clause that tests typed clauses.
const effeckCaughtName = "$caught"

// isTypedCatchClause reports whether clause's variable has a type annotation.
func isTypedCatchClause(clause *ast.Node) bool {
	variable := clause.AsCatchClause().VariableDeclaration
	return variable != nil && variable.Type() != nil
}

// clauseAnchor returns a source range with width that stands for clause: its
// variable's name, or, without a variable, its block.
func clauseAnchor(clause *ast.Node) core.TextRange {
	if variable := clause.AsCatchClause().VariableDeclaration; variable != nil {
		return variable.Name().Loc
	}
	return clause.AsCatchClause().Block.Loc
}

// effeckCatchChain returns the statement that runs the first of clauses
// whose type the caught failure has. In cause mode a clause typed Cause
// catches every reason, and an untyped clause catches only an expected
// failure (D176).
func (p *Parser) effeckCatchChain(clauses []*ast.Node, causeMode bool) *ast.Node {
	clause := clauses[0]
	catch := clause.AsCatchClause()
	span := core.NewTextRange(clause.Pos(), clauses[len(clauses)-1].End())
	if causeMode && isCauseCatchClause(clause) {
		body := p.effeckClauseBodyOf(clause, effeckCauseName)
		if len(clauses) == 1 {
			return body
		}
		// The clauses after a Cause clause cannot run (EK2106).
		always := p.effeckSynthesized(p.factory.NewKeywordExpression(ast.KindTrueKeyword), clauseAnchor(clause))
		return p.effeckSpanning(p.factory.NewIfStatement(always, body, p.effeckCatchChain(clauses[1:], causeMode)), span)
	}
	body := p.effeckClauseBody(clause)
	if !isTypedCatchClause(clause) && causeMode {
		// Before a Cause clause, an untyped clause catches an expected failure.
		anchor := clauseAnchor(clause)
		caught := p.effeckSynthesized(p.factory.NewIdentifier(effeckCaughtName), anchor)
		none := p.effeckSynthesized(p.factory.NewIdentifier(effeckNoFailure), anchor)
		test := p.effeckSynthesized(p.factory.NewBinaryExpression(nil /*modifiers*/, caught, nil /*typeNode*/, p.effeckSynthesized(p.factory.NewToken(ast.KindExclamationEqualsEqualsToken), anchor), none), anchor)
		var rest *ast.Node
		if len(clauses) > 1 {
			rest = p.effeckCatchChain(clauses[1:], causeMode)
		}
		return p.effeckSpanning(p.factory.NewIfStatement(test, body, rest), span)
	}
	if !isTypedCatchClause(clause) {
		if len(clauses) == 1 {
			return body
		}
		// An untyped clause catches everything, so the clauses after it cannot
		// run (EK2106). They stay in the tree, after a test that always holds,
		// so the checker still reads them.
		always := p.effeckSynthesized(p.factory.NewKeywordExpression(ast.KindTrueKeyword), clauseAnchor(clause))
		return p.effeckSpanning(p.factory.NewIfStatement(always, body, p.effeckCatchChain(clauses[1:], causeMode)), span)
	}
	anchor := clauseAnchor(clause)
	typeNode := catch.VariableDeclaration.Type()
	guard := p.effeckSynthesized(p.factory.NewIdentifier("$isFailure"), anchor)
	caught := p.effeckSynthesized(p.factory.NewIdentifier(effeckCaughtName), anchor)
	test := p.effeckSpanning(p.factory.NewCallExpression(guard, nil, p.newNodeList(typeNode.Loc, []*ast.Node{typeNode}), p.newNodeList(anchor, []*ast.Node{caught}), ast.NodeFlagsNone), typeNode.Loc)
	var rest *ast.Node
	if len(clauses) > 1 {
		rest = p.effeckCatchChain(clauses[1:], causeMode)
	} else {
		last := p.effeckSynthesized(p.factory.NewIdentifier(effeckCaughtName), anchor)
		rest = p.effeckSynthesized(p.factory.NewThrowStatement(last), anchor)
	}
	return p.effeckSpanning(p.factory.NewIfStatement(test, body, rest), span)
}

// effeckClauseBody returns clause's block with `const <variable> = $caught`
// in front when the clause names a variable.
func (p *Parser) effeckClauseBody(clause *ast.Node) *ast.Node {
	return p.effeckClauseBodyOf(clause, effeckCaughtName)
}

// effeckClauseBodyOf returns clause's block with `const <variable> = <source>`
// in front when the clause names a variable.
func (p *Parser) effeckClauseBodyOf(clause *ast.Node, source string) *ast.Node {
	catch := clause.AsCatchClause()
	statements := catch.Block.AsBlock().Statements.Nodes
	start := catch.Block.Pos()
	if variable := catch.VariableDeclaration; variable != nil {
		name := variable.Name()
		start = name.Pos()
		caught := p.effeckSynthesized(p.factory.NewIdentifier(source), name.Loc)
		declaration := p.effeckSpanning(p.factory.NewVariableDeclaration(name, nil, nil, caught), name.Loc)
		list := p.effeckSpanning(p.factory.NewVariableDeclarationList(p.newNodeList(name.Loc, []*ast.Node{declaration}), ast.NodeFlagsConst), name.Loc)
		statement := p.effeckSpanning(p.factory.NewVariableStatement(nil, list), name.Loc)
		statements = append([]*ast.Node{statement}, statements...)
	}
	span := core.NewTextRange(start, catch.Block.End())
	return p.effeckSpanning(p.factory.NewBlock(p.newNodeList(span, statements), true /*multiLine*/), span)
}

// isStartOfHandleClause reports whether the current token, after a try block
// or a handle clause, starts a handle clause: `handle` and then a name on the
// same line. In TypeScript only catch or finally may follow a try block, so
// the form takes nothing away.
func (p *Parser) isStartOfHandleClause() bool {
	return p.token == ast.KindIdentifier && p.scanner.TokenValue() == "handle" && p.lookAhead((*Parser).nextTokenIsIdentifierOnSameLine)
}

// The names the parser gives a handle statement's synthesized variables: the
// catch variable that stands for "an operation was performed", the
// continuation of a clause, and the function whose result types a clause's
// bindings.
const (
	effeckPerformedName    = "$performed"
	effeckContinuationName = "$k"
	effeckClauseName       = "$clause"
)

// parseEffeckHandleStatement parses the handle clauses that follow tryBlock,
// and the catch clauses and finally block that may follow them:
//
//	try { BODY } handle Approval.approve(request) { CLAUSE } catch (e) { C } finally { F }
//
// as the TypeScript the checker reads in its place:
//
//	try {
//	  try { BODY } catch ($performed) {
//	    if ($performed) { const [$k, request] = $clause(Approval.approve); CLAUSE }
//	  }
//	} catch (e) { C } finally { F }
//
// The inner try statement, flagged NodeFlagsEffeck, is the handle statement
// (D132). Its catch block stands for the clauses: TypeScript lets a catch
// block start at any point of its try block, as a clause starts at any
// operation the body performs, and the if chain makes each clause reachable
// on its own. $clause types a clause's parameters as the operation's, and
// $k, the continuation, so that its resume returns never. In a clause,
// resume(value), resumeWith(program), and abandon() pass $k, and a resume in
// tail position becomes `return resume($k, value)`: the clause ends there, as
// the rest of the try block decides. A return in the body or a clause
// returns from the enclosing function. The analyzer and the lowering read
// the statement as a call of handle (handles.go); catch and finally wrap the
// whole statement, so they also see failures raised in a clause.
func (p *Parser) parseEffeckHandleStatement(pos int, jsdoc jsdocScannerInfo, tryBlock *ast.Node) *ast.Node {
	var clauses []effeckHandleClause
	for p.isStartOfHandleClause() {
		clauses = append(clauses, p.parseHandleClause())
	}
	var chain *ast.Node
	end := clauses[len(clauses)-1].block.End()
	for i := len(clauses) - 1; i >= 0; i-- {
		clause := clauses[i]
		test := p.effeckSynthesized(p.factory.NewIdentifier(effeckPerformedName), clause.anchor)
		chain = p.effeckSpanning(p.factory.NewIfStatement(test, clause.block, chain), core.NewTextRange(clause.block.Pos(), end))
	}
	anchor := clauses[0].anchor
	name := p.effeckSynthesized(p.factory.NewIdentifier(effeckPerformedName), anchor)
	variable := p.effeckSynthesized(p.factory.NewVariableDeclaration(name, nil /*exclamationToken*/, nil /*typeNode*/, nil /*initializer*/), anchor)
	block := p.effeckSpanning(p.factory.NewBlock(p.newNodeList(chain.Loc, []*ast.Node{chain}), true /*multiLine*/), chain.Loc)
	catch := p.effeckSpanning(p.factory.NewCatchClause(variable, block), chain.Loc)
	handle := p.finishNode(p.factory.NewTryStatement(tryBlock, catch, nil /*finallyBlock*/), pos)
	handle.Flags |= ast.NodeFlagsEffeck
	if p.token != ast.KindCatchKeyword && p.token != ast.KindFinallyKeyword {
		p.withJSDoc(handle, jsdoc)
		return handle
	}
	var catchClause *ast.Node
	if p.token == ast.KindCatchKeyword {
		catchClause = p.parseEffeckCatchClauses(p.parseCatchClause())
	}
	var finallyBlock *ast.Node
	if p.parseOptional(ast.KindFinallyKeyword) {
		finallyBlock = p.parseBlock(false /*ignoreMissingOpenBrace*/, nil)
	}
	body := p.effeckSpanning(p.factory.NewBlock(p.newNodeList(handle.Loc, []*ast.Node{handle}), true /*multiLine*/), handle.Loc)
	result := p.finishNode(p.factory.NewTryStatement(body, catchClause, finallyBlock), pos)
	p.withJSDoc(result, jsdoc)
	return result
}

// An effeckHandleClause is one parsed handle clause: the range of its handle
// word, which its synthesized nodes stand on, and the block the checker
// reads, whose first statement binds the clause's parameters.
type effeckHandleClause struct {
	anchor core.TextRange
	block  *ast.Node
}

// parseHandleClause parses `handle Approval.approve(request) { ... }`. Its
// parameters are binding elements without types, which come from the
// operation.
func (p *Parser) parseHandleClause() effeckHandleClause {
	anchor := p.scanner.TokenRange()
	p.nextToken() // handle
	operation := p.parseHandleOperation()
	open := p.scanner.TokenRange()
	p.parseExpected(ast.KindOpenParenToken)
	parameters := p.parseDelimitedList(PCParameters, (*Parser).parseArrayBindingElement)
	p.parseExpected(ast.KindCloseParenToken)
	header := core.NewTextRange(open.Pos(), p.nodePos())
	body := p.parseBlock(false /*ignoreMissingOpenBrace*/, nil)
	// The continuation comes first, so a rest parameter stays last.
	var k *ast.Node
	if p.effeckClauseResumes(body, open) {
		k = p.effeckSynthesized(p.factory.NewIdentifier(effeckContinuationName), open)
	}
	continuation := p.effeckSynthesized(p.factory.NewBindingElement(nil /*dotDotDotToken*/, nil /*propertyName*/, k, nil /*initializer*/), open)
	elements := p.newNodeList(core.NewTextRange(open.Pos(), max(parameters.End(), open.End())), append([]*ast.Node{continuation}, parameters.Nodes...))
	pattern := p.effeckSpanning(p.factory.NewBindingPattern(ast.KindArrayBindingPattern, elements), header)
	callee := p.effeckSynthesized(p.factory.NewIdentifier(effeckClauseName), anchor)
	call := p.effeckSpanning(p.factory.NewCallExpression(callee, nil /*questionDotToken*/, nil /*typeArguments*/, p.newNodeList(operation.Loc, []*ast.Node{operation}), ast.NodeFlagsNone), core.NewTextRange(anchor.Pos(), operation.End()))
	span := core.NewTextRange(anchor.Pos(), header.End())
	declaration := p.effeckSpanning(p.factory.NewVariableDeclaration(pattern, nil /*exclamationToken*/, nil /*typeNode*/, call), span)
	list := p.effeckSpanning(p.factory.NewVariableDeclarationList(p.newNodeList(span, []*ast.Node{declaration}), ast.NodeFlagsConst), span)
	binding := p.effeckSpanning(p.factory.NewVariableStatement(nil /*modifiers*/, list), span)
	binding.Flags |= ast.NodeFlagsEffeck
	statements := append([]*ast.Node{binding}, body.AsBlock().Statements.Nodes...)
	blockSpan := core.NewTextRange(anchor.Pos(), body.End())
	block := p.effeckSpanning(p.factory.NewBlock(p.newNodeList(blockSpan, statements), true /*multiLine*/), blockSpan)
	return effeckHandleClause{anchor: anchor, block: block}
}

// parseHandleOperation parses the operation a handle clause answers, a
// control's name and the operation's, Approval.approve, as a property access.
func (p *Parser) parseHandleOperation() *ast.Node {
	pos := p.nodePos()
	expression := p.parseIdentifier()
	for p.token == ast.KindDotToken || expression.Kind == ast.KindIdentifier {
		p.parseExpected(ast.KindDotToken)
		name := p.parseRightSideOfDot(true /*allowIdentifierNames*/, false /*allowPrivateIdentifiers*/, true /*allowUnicodeEscapeSequenceInIdentifierName*/)
		expression = p.finishNode(p.factory.NewPropertyAccessExpression(expression, nil /*questionDotToken*/, name, ast.NodeFlagsNone), pos)
	}
	return expression
}

// effeckClauseResumes passes the clause's continuation, $k standing on
// anchor, to each resume(value), resume(), resumeWith(program), and abandon()
// in the clause's block, outside nested functions, and turns a resume or resumeWith
// statement in tail position into `return resume($k, value)`. Calls with the
// continuation already passed, such as those of a nested handle statement's
// clauses, keep their arguments. It reports whether the clause uses the
// continuation.
func (p *Parser) effeckClauseResumes(block *ast.Node, anchor core.TextRange) bool {
	used := false
	resumes := map[*ast.Node]bool{}
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if ast.IsFunctionLike(node) || ast.IsClassLike(node) || node.Kind == ast.KindClassStaticBlockDeclaration {
			return false
		}
		node.ForEachChild(visit)
		if node.Kind != ast.KindCallExpression || !ast.IsIdentifier(node.Expression()) || node.QuestionDotToken() != nil {
			return false
		}
		call := node.AsCallExpression()
		arguments := call.Arguments
		switch node.Expression().Text() {
		case "resume":
			// resume() continues an operation whose result is void.
			if len(arguments.Nodes) > 1 {
				return false
			}
			resumes[node] = true
		case "resumeWith":
			if len(arguments.Nodes) != 1 {
				return false
			}
			resumes[node] = true
		case "abandon":
			if len(arguments.Nodes) != 0 {
				return false
			}
		default:
			return false
		}
		callee := node.Expression()
		k := p.effeckSynthesized(p.factory.NewIdentifier(effeckContinuationName), callee.Loc)
		passed := append([]*ast.Node{k}, arguments.Nodes...)
		loc := k.Loc
		if len(arguments.Nodes) != 0 {
			loc = core.NewTextRange(k.Pos(), arguments.End())
		} else if callee.Text() == "resume" {
			passed = append(passed, p.effeckSynthesized(p.factory.NewIdentifier("undefined"), callee.Loc))
		}
		for _, argument := range passed {
			argument.Parent = node
		}
		call.Arguments = p.newNodeList(loc, passed)
		used = true
		return false
	}
	block.ForEachChild(visit)
	// A tail resume ends the clause: it returns what the resumed body does.
	var tail func(statement *ast.Node) *ast.Node
	tail = func(statement *ast.Node) *ast.Node {
		switch statement.Kind {
		case ast.KindExpressionStatement:
			if resumes[ast.SkipParentheses(statement.Expression())] {
				result := p.effeckSpanning(p.factory.NewReturnStatement(statement.Expression()), statement.Loc)
				result.Flags |= ast.NodeFlagsEffeck
				return result
			}
		case ast.KindIfStatement:
			s := statement.AsIfStatement()
			s.ThenStatement = tail(s.ThenStatement)
			s.ThenStatement.Parent = statement
			if s.ElseStatement != nil {
				s.ElseStatement = tail(s.ElseStatement)
				s.ElseStatement.Parent = statement
			}
		case ast.KindBlock:
			if statements := statement.AsBlock().Statements.Nodes; len(statements) != 0 {
				statements[len(statements)-1] = tail(statements[len(statements)-1])
				statements[len(statements)-1].Parent = statement
			}
		}
		return statement
	}
	tail(block)
	return used
}

// parseSchemaDeclaration parses
//
//	schema User { id: string; nickname?: string; role: "admin" | "member"; tags: ReadonlyArray<string>; home: Address }
//
// as the two declarations it abbreviates (D133):
//
//	const User = Schema.Struct({
//	  id: Schema.String,
//	  nickname: Schema.optional(Schema.String),
//	  role: Schema.Literals(["admin", "member"]),
//	  tags: Schema.Array(Schema.String),
//	  home: Address
//	})
//	interface User extends Schema.Schema.Type<typeof User> {}
//
// A field written mutable gets Schema.mutableKey (D134).
//
// A field's type becomes the schema of its values: a primitive, a literal, a
// union, an array, a tuple, a record, an object type, or the name of another
// schema, which the struct refers to as a value. Any other type becomes
// Schema.Unknown flagged NodeFlagsEffeck, which the checker reports as EK2109
// (schemas.go). The struct call is flagged NodeFlagsEffeck too. The interface
// goes to p.reparseList, so the statement list holds it before the constant.
func (p *Parser) parseSchemaDeclaration(pos int, jsdoc jsdocScannerInfo, modifiers *ast.ModifierList) *ast.Node {
	keyword := p.scanner.TokenRange()
	p.nextToken() // schema
	name := p.parseBindingIdentifier()
	var members []*ast.Node
	body := name.Loc
	if p.token == ast.KindOpenBraceToken {
		literal := p.parseTypeLiteral()
		members = literal.AsTypeLiteralNode().Members.Nodes
		body = literal.Loc
	}
	fields := p.effeckSchemaFields(members, body)
	statement := p.factory.NewVariableStatement(modifiers, p.effeckSpanning(p.factory.NewVariableDeclarationList(
		p.newNodeList(core.NewTextRange(name.Pos(), fields.End()), []*ast.Node{
			p.effeckSpanning(p.factory.NewVariableDeclaration(name, nil /*exclamationToken*/, nil /*typeNode*/, fields), core.NewTextRange(name.Pos(), fields.End())),
		}), ast.NodeFlagsConst), core.NewTextRange(name.Pos(), fields.End())))
	p.finishNode(statement, pos)
	p.withJSDoc(statement, jsdoc)

	// interface User extends Schema.Schema.Type<typeof User> {}
	var interfaceModifiers *ast.ModifierList
	if modifiers != nil {
		var copies []*ast.Node
		for _, modifier := range modifiers.Nodes {
			copies = append(copies, p.effeckSynthesized(p.factory.NewModifier(modifier.Kind), keyword))
		}
		interfaceModifiers = p.newModifierList(keyword, copies)
	}
	typeName := p.effeckSynthesized(p.factory.NewIdentifier(name.Text()), name.Loc)
	query := p.effeckSynthesized(p.factory.NewTypeQueryNode(p.effeckSynthesized(p.factory.NewIdentifier(name.Text()), name.Loc), nil /*typeArguments*/), name.Loc)
	helper := p.effeckSchemaMember(p.effeckSchemaMember(nil, "Schema", keyword), "Type", keyword)
	base := p.effeckSynthesized(p.factory.NewExpressionWithTypeArguments(helper, p.newNodeList(name.Loc, []*ast.Node{query})), keyword)
	clause := p.effeckSynthesized(p.factory.NewHeritageClause(ast.KindExtendsKeyword, p.newNodeList(keyword, []*ast.Node{base})), keyword)
	declaration := p.factory.NewInterfaceDeclaration(interfaceModifiers, typeName, nil /*typeParameters*/, p.newNodeList(keyword, []*ast.Node{clause}), p.newNodeList(keyword, nil))
	p.effeckSynthesized(declaration, core.NewTextRange(pos, keyword.End()))
	p.reparseList = append(p.reparseList, declaration)
	return statement
}

// effeckSchemaMember makes Schema.<name>, or <object>.<name> when object is
// not nil, synthesized at anchor.
func (p *Parser) effeckSchemaMember(object *ast.Node, name string, anchor core.TextRange) *ast.Node {
	if object == nil {
		object = p.effeckSynthesized(p.factory.NewIdentifier("Schema"), anchor)
	}
	member := p.effeckSynthesized(p.factory.NewIdentifier(name), anchor)
	return p.effeckSynthesized(p.factory.NewPropertyAccessExpression(object, nil /*questionDotToken*/, member, ast.NodeFlagsNone), anchor)
}

// effeckSchemaCall makes Schema.<name>(arguments...), whose callee stands on
// anchor and which spans anchor and its arguments.
func (p *Parser) effeckSchemaCall(name string, anchor core.TextRange, arguments ...*ast.Node) *ast.Node {
	span := anchor
	if len(arguments) != 0 {
		span = core.NewTextRange(min(anchor.Pos(), arguments[0].Pos()), max(anchor.End(), arguments[len(arguments)-1].End()))
	}
	list := p.newNodeList(span, arguments)
	return p.effeckSpanning(p.factory.NewCallExpression(p.effeckSchemaMember(nil, name, anchor), nil /*questionDotToken*/, nil /*typeArguments*/, list, ast.NodeFlagsNone), span)
}

// effeckSchemaFields makes Schema.Struct({ ... }) of members, the members of
// a schema declaration's body or of an object type in a field, flagged
// NodeFlagsEffeck. body is the range of the braces.
func (p *Parser) effeckSchemaFields(members []*ast.Node, body core.TextRange) *ast.Node {
	var properties []*ast.Node
	for _, member := range members {
		name := member.Name()
		if name == nil || !ast.IsPropertySignatureDeclaration(member) || member.Type() == nil {
			// A method, an index signature, or a property without a type.
			if name == nil {
				name = p.effeckSynthesized(p.factory.NewStringLiteral("", ast.TokenFlagsNone), member.Loc)
			}
			properties = append(properties, p.effeckSpanning(p.factory.NewPropertyAssignment(nil /*modifiers*/, name, nil /*postfixToken*/, nil /*typeNode*/, p.effeckUnsupportedSchema(member.Loc)), member.Loc))
			continue
		}
		value := p.effeckSchemaOf(member.Type())
		if member.PostfixToken() != nil && member.PostfixToken().Kind == ast.KindQuestionToken {
			value = p.effeckSchemaCall("optional", member.PostfixToken().Loc, value)
		}
		if !ast.HasSyntacticModifier(member, ast.ModifierFlagsReadonly) {
			// Written mutable (D134): the field's key is mutable.
			value = p.effeckSchemaCall("mutableKey", name.Loc, value)
		}
		properties = append(properties, p.effeckSpanning(p.factory.NewPropertyAssignment(nil /*modifiers*/, name, nil /*postfixToken*/, nil /*typeNode*/, value), core.NewTextRange(name.Pos(), value.End())))
	}
	object := p.effeckSpanning(p.factory.NewObjectLiteralExpression(p.newNodeList(body, properties), true /*multiLine*/), body)
	brace := scanner.SkipTrivia(p.sourceText, body.Pos())
	call := p.effeckSchemaCall("Struct", core.NewTextRange(brace, min(brace+1, body.End())), object)
	call.Flags |= ast.NodeFlagsEffeck
	return call
}

// effeckUnsupportedSchema makes the Schema.Unknown that stands for a field
// type with no schema form, flagged NodeFlagsEffeck so the checker reports it.
func (p *Parser) effeckUnsupportedSchema(anchor core.TextRange) *ast.Node {
	unknown := p.effeckSchemaMember(nil, "Unknown", anchor)
	unknown.Flags |= ast.NodeFlagsEffeck
	return unknown
}

// effeckSchemaOf returns the schema expression of typeNode, a field's type.
func (p *Parser) effeckSchemaOf(typeNode *ast.Node) *ast.Node {
	switch typeNode.Kind {
	case ast.KindParenthesizedType:
		return p.effeckSchemaOf(typeNode.AsParenthesizedTypeNode().Type)
	case ast.KindStringKeyword, ast.KindNumberKeyword, ast.KindBooleanKeyword, ast.KindBigIntKeyword, ast.KindSymbolKeyword,
		ast.KindUndefinedKeyword, ast.KindUnknownKeyword, ast.KindAnyKeyword, ast.KindNeverKeyword, ast.KindObjectKeyword, ast.KindVoidKeyword:
		return p.effeckSchemaMember(nil, effeckSchemaKeywords[typeNode.Kind], typeNode.Loc)
	case ast.KindLiteralType:
		literal := typeNode.AsLiteralTypeNode().Literal
		if literal.Kind == ast.KindNullKeyword {
			return p.effeckSchemaMember(nil, "Null", typeNode.Loc)
		}
		return p.effeckSchemaCall("Literal", typeNode.Loc, literal)
	case ast.KindUnionType:
		members := typeNode.AsUnionTypeNode().Types.Nodes
		if core.Every(members, isEffeckSchemaLiteral) {
			literals := make([]*ast.Node, len(members))
			for i, member := range members {
				literals[i] = member.AsLiteralTypeNode().Literal
			}
			array := p.effeckSpanning(p.factory.NewArrayLiteralExpression(p.newNodeList(typeNode.Loc, literals), false /*multiLine*/), typeNode.Loc)
			return p.effeckSchemaCall("Literals", typeNode.Loc, array)
		}
		schemas := make([]*ast.Node, len(members))
		for i, member := range members {
			schemas[i] = p.effeckSchemaOf(member)
		}
		array := p.effeckSpanning(p.factory.NewArrayLiteralExpression(p.newNodeList(typeNode.Loc, schemas), false /*multiLine*/), typeNode.Loc)
		return p.effeckSchemaCall("Union", typeNode.Loc, array)
	case ast.KindArrayType:
		return p.effeckSchemaCall("Array", typeNode.Loc, p.effeckSchemaOf(typeNode.AsArrayTypeNode().ElementType))
	case ast.KindTypeOperator:
		if operator := typeNode.AsTypeOperatorNode(); operator.Operator == ast.KindReadonlyKeyword &&
			(operator.Type.Kind == ast.KindArrayType || operator.Type.Kind == ast.KindTupleType) {
			return p.effeckSchemaOf(operator.Type)
		}
	case ast.KindTupleType:
		elements := typeNode.AsTupleTypeNode().Elements.Nodes
		schemas := make([]*ast.Node, len(elements))
		for i, element := range elements {
			if element.Kind == ast.KindNamedTupleMember || element.Kind == ast.KindOptionalType || element.Kind == ast.KindRestType {
				return p.effeckUnsupportedSchema(typeNode.Loc)
			}
			schemas[i] = p.effeckSchemaOf(element)
		}
		array := p.effeckSpanning(p.factory.NewArrayLiteralExpression(p.newNodeList(typeNode.Loc, schemas), false /*multiLine*/), typeNode.Loc)
		return p.effeckSchemaCall("Tuple", typeNode.Loc, array)
	case ast.KindTypeLiteral:
		return p.effeckSchemaFields(typeNode.AsTypeLiteralNode().Members.Nodes, typeNode.Loc)
	case ast.KindTypeReference:
		reference := typeNode.AsTypeReferenceNode()
		var arguments []*ast.Node
		if reference.TypeArguments != nil {
			arguments = reference.TypeArguments.Nodes
		}
		name := reference.TypeName
		switch {
		case len(arguments) == 0:
			return p.effeckEntityExpression(name)
		case ast.IsIdentifier(name) && (name.Text() == "Array" || name.Text() == "ReadonlyArray") && len(arguments) == 1:
			return p.effeckSchemaCall("Array", name.Loc, p.effeckSchemaOf(arguments[0]))
		case ast.IsIdentifier(name) && name.Text() == "Record" && len(arguments) == 2:
			return p.effeckSchemaCall("Record", name.Loc, p.effeckSchemaOf(arguments[0]), p.effeckSchemaOf(arguments[1]))
		}
	}
	return p.effeckUnsupportedSchema(typeNode.Loc)
}

// effeckSchemaKeywords names the schema of each keyword type.
var effeckSchemaKeywords = map[ast.Kind]string{
	ast.KindStringKeyword:    "String",
	ast.KindNumberKeyword:    "Number",
	ast.KindBooleanKeyword:   "Boolean",
	ast.KindBigIntKeyword:    "BigInt",
	ast.KindSymbolKeyword:    "Symbol",
	ast.KindUndefinedKeyword: "Undefined",
	ast.KindUnknownKeyword:   "Unknown",
	ast.KindAnyKeyword:       "Any",
	ast.KindNeverKeyword:     "Never",
	ast.KindObjectKeyword:    "ObjectKeyword",
	ast.KindVoidKeyword:      "Void",
}

// isEffeckSchemaLiteral reports whether typeNode is a literal type other than
// null: a string, number, bigint, or boolean literal.
func isEffeckSchemaLiteral(typeNode *ast.Node) bool {
	return typeNode.Kind == ast.KindLiteralType && typeNode.AsLiteralTypeNode().Literal.Kind != ast.KindNullKeyword
}

// effeckEntityExpression turns name, an entity name such as Address or
// models.Address, into the expression that names the same value, reusing its
// identifiers.
func (p *Parser) effeckEntityExpression(name *ast.Node) *ast.Node {
	if name.Kind != ast.KindQualifiedName {
		return name
	}
	qualified := name.AsQualifiedName()
	return p.effeckSpanning(p.factory.NewPropertyAccessExpression(p.effeckEntityExpression(qualified.Left), nil /*questionDotToken*/, qualified.Right, ast.NodeFlagsNone), name.Loc)
}
