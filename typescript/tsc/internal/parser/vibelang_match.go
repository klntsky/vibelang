package parser

import (
	"strconv"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/diagnostics"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// Pattern matching (D206-D225) adopts the match expression and the is
// operator of the TC39 pattern matching proposal (stage 1). The parser writes
// them as TypeScript that the checker already narrows: typeof, in, ===,
// Array.isArray, and the guards Option.isSome, Exit.isFailure, Result.isSuccess,
// and the runtime's $is, with each binding a const. A match reads its subject
// once, into a const that its tests read (subjectPath):
//
//	match (e) {                              (() => {
//	  when NotFound and { let id }:            const $match1 = e
//	    `no user ${id}`;                       { if ($is(NotFound, $match1) && typeof $match1 === "object" && $match1 !== null && "id" in $match1) { const id = $match1.id; return `no user ${id}` } }
//	  when Timeout: "retry";                   { if ($is(Timeout, $match1)) { return "retry" } }
//	}                                          return $noMatch($match1)
//	                                         })()
//
// A match that a return statement returns, or that stands as a statement,
// is written without the function: return match (...) { ... } becomes a
// block whose arms return, and a match statement becomes a labeled block
// whose arms break out of it. Without a default arm, the arms end in a call of
// $noMatch, whose argument's narrowed type tells the checker which cases no
// arm handles (EK4701, match.go in package vibelang).
//
// A data variant's pattern (D307) tests the variant with $is, which the
// checker narrows and the lowering writes as the data type's own $is guard
// (data.go in package vibelang), then reads the fields it lists:
//
//	when Delivery.Sent { let tracking }:     if ($is(Delivery.Sent, d)) { const tracking = d.tracking; ... }
//
// The variant fixes the fields, so a field the pattern binds needs no in
// test, and an optional one the value leaves out binds undefined.
//
// x is P is a boolean expression. When P binds names, the if or while
// statement whose condition joins it with && is rewritten so the bindings are
// in scope in its body:
//
//	if (exit is Exit.Failure(let cause)) log(cause)
//	                                         { const $is1 = exit; if (Exit.isFailure($is1)) { const cause = $is1.cause; log(cause) } }
//
// A pattern form VibeLang does not support yet becomes a call of
// $patternError, which the checker reports (EK4702-EK4705).

// patternKind is the kind of a pattern as written.
type patternKind int

const (
	patternLiteral   patternKind = iota // a string, numeric, bigint, boolean, null, or template literal
	patternValue                        // a variable pattern: an identifier expression
	patternBinding                      // let, const, or var and a name
	patternArray                        // [items, ...rest]
	patternObject                       // { clauses, ...rest }
	patternExtractor                    // a variable pattern with an argument list: expr(items, ...rest)
	patternVariant                      // a variant and its fields: expr { clauses, ...rest } (D307)
	patternAnd                          // items joined by and
	patternOr                           // items joined by or
	patternNot                          // not items[0]
	patternGuard                        // if (expr)
	patternInvalid                      // a form VibeLang does not support yet: code, detail
)

// objectClause is one clause of an object pattern.
type objectClause struct {
	key      *ast.Node // an identifier name, a string or numeric literal, or a computed key's expression
	computed bool
	pattern  *pattern // nil when the clause only tests that the property exists
}

// pattern is a pattern as written.
type pattern struct {
	kind          patternKind
	anchor        core.TextRange // the pattern's first token, for the tokens written in its place
	expr          *ast.Node      // a literal, a variable pattern, a guard's expression, or an extractor's matcher
	bindFlags     ast.NodeFlags  // a binding's ast.NodeFlagsConst, ast.NodeFlagsLet, or ast.NodeFlagsNone for var
	name          *ast.Node      // a binding's name
	items         []*pattern
	hasRest       bool     // an array or extractor pattern ends in ...
	rest          *pattern // the pattern after ..., or nil
	clauses       []objectClause
	parenthesized bool
	code          int    // a $patternError code: the form, or 4705 for combinators sharing a level
	detail        string // the $patternError detail
}

// matchArm is one arm of a match expression; pattern is nil for the default arm.
type matchArm struct {
	anchor  core.TextRange
	pattern *pattern
	value   *ast.Node
	end     int
}

// matchInfo is a match expression as written, kept so a return statement or
// an expression statement can write it without the function.
type matchInfo struct {
	anchor  core.TextRange // the match keyword
	subject *ast.Node
	arms    []matchArm
	pos     int
}

// isInfo is an is expression whose pattern binds names, kept so the if or
// while statement whose condition holds it can bring the bindings into scope.
type isInfo struct {
	anchor  core.TextRange // the is keyword
	subject *ast.Node
	pattern *pattern
}

// matchStep is one step of a compiled pattern: a test or a declaration.
type matchStep struct {
	test *ast.Node
	decl *ast.Node
}

// matchPath writes one subject or part of a subject, as a new node each time.
type matchPath func() *ast.Node

// matchCompile carries the state of compiling one top-level pattern.
type matchCompile struct {
	anchor core.TextRange
	inline bool // inside or, not, or an is without an if or while: no declarations
}

// ---------------------------------------------------------------- parsing

// isStartOfVibeLangMatch reports whether the current token starts a match
// expression: in a .vibe file, match ( subject ) { on one line, then when,
// default, or }. The sequence is a TypeScript syntax error today.
func (p *Parser) isStartOfVibeLangMatch() bool {
	return p.token == ast.KindIdentifier && p.scanner.TokenValue() == "match" && p.isVibeLangFile() && p.lookAhead((*Parser).scanStartOfVibeLangMatch)
}

func (p *Parser) scanStartOfVibeLangMatch() bool {
	p.nextToken()
	if p.token != ast.KindOpenParenToken || p.hasPrecedingLineBreak() {
		return false
	}
	p.nextToken()
	p.parseExpressionAllowIn()
	if p.token != ast.KindCloseParenToken {
		return false
	}
	p.nextToken()
	if p.token != ast.KindOpenBraceToken || p.hasPrecedingLineBreak() {
		return false
	}
	p.nextToken()
	return p.isMatchWord("when") || p.token == ast.KindDefaultKeyword || p.token == ast.KindCloseBraceToken
}

// isMatchWord reports whether the current token is the identifier word.
func (p *Parser) isMatchWord(word string) bool {
	return p.token == ast.KindIdentifier && p.scanner.TokenValue() == word
}

// parseVibeLangMatch parses match (subject) { when pattern: value; ... default: value; }.
func (p *Parser) parseVibeLangMatch() *ast.Node {
	info := &matchInfo{pos: p.nodePos(), anchor: p.tokenRange()}
	p.nextToken() // match
	p.parseExpected(ast.KindOpenParenToken)
	info.subject = p.parseExpressionAllowIn()
	p.parseExpected(ast.KindCloseParenToken)
	form := VibeSyntax{Kind: VibeSyntaxMatch, Word: info.anchor, Left: p.sourceRange(info.subject)}
	if p.token == ast.KindOpenBraceToken {
		form.Open = p.tokenRange()
	}
	p.parseExpected(ast.KindOpenBraceToken)
	sawDefault := false
	for p.token != ast.KindCloseBraceToken && p.token != ast.KindEndOfFile {
		arm := matchArm{anchor: p.tokenRange()}
		armForm := VibeSyntaxArm{}
		switch {
		case p.token == ast.KindDefaultKeyword:
			if sawDefault {
				p.parseErrorAtCurrentToken(diagnostics.X_0_expected, "}")
			}
			sawDefault = true
			p.nextToken()
		case p.isMatchWord("when"):
			if sawDefault {
				p.parseErrorAtCurrentToken(diagnostics.X_0_expected, "}")
			}
			p.nextToken()
			arm.pattern = p.parseMatchPattern()
		default:
			p.parseErrorAtCurrentToken(diagnostics.X_0_expected, "when")
			p.nextToken()
			continue
		}
		armForm.Head = core.NewTextRange(arm.anchor.Pos(), p.nodePos())
		p.parseExpected(ast.KindColonToken)
		arm.value = p.parseAssignmentExpressionOrHigher()
		arm.end = p.nodePos()
		armForm.Value = p.sourceRange(arm.value)
		if p.token == ast.KindSemicolonToken {
			armForm.Semicolon = p.tokenRange()
		}
		p.parseSemicolon()
		info.arms = append(info.arms, arm)
		form.Arms = append(form.Arms, armForm)
	}
	if p.token == ast.KindCloseBraceToken {
		form.Close = p.tokenRange()
	}
	p.parseExpected(ast.KindCloseBraceToken)
	form.Range = core.NewTextRange(info.anchor.Pos(), p.nodePos())
	p.recordVibeSyntax(form)
	node := p.matchFunctionCall(info)
	if p.vibelangMatches == nil {
		p.vibelangMatches = map[*ast.Node]*matchInfo{}
	}
	p.vibelangMatches[node] = info
	return node
}

// combinatorWord returns "and" or "or" when the current token is that word.
func (p *Parser) combinatorWord() string {
	if p.isMatchWord("and") {
		return "and"
	}
	if p.isMatchWord("or") {
		return "or"
	}
	return ""
}

// parseMatchPattern parses a pattern: unary patterns joined by and, or by or.
// Combinators of different kinds cannot share a level (the proposal requires
// parentheses), which is EK4705.
func (p *Parser) parseMatchPattern() *pattern {
	first := p.parseMatchUnaryPattern()
	word := p.combinatorWord()
	if word == "" {
		return first
	}
	result := &pattern{kind: patternAnd, anchor: first.anchor, items: []*pattern{first}}
	if word == "or" {
		result.kind = patternOr
	}
	mixed := first.kind == patternNot && !first.parenthesized
	for w := p.combinatorWord(); w != ""; w = p.combinatorWord() {
		if w != word {
			mixed = true
		}
		p.nextToken()
		item := p.parseMatchUnaryPattern()
		if item.kind == patternNot && !item.parenthesized {
			mixed = true
		}
		result.items = append(result.items, item)
	}
	if mixed {
		result.code = 4705
	}
	return result
}

// parseMatchUnaryPattern parses not pattern, or a primary pattern.
func (p *Parser) parseMatchUnaryPattern() *pattern {
	if p.isMatchWord("not") && p.lookAhead((*Parser).nextTokenStartsPattern) {
		anchor := p.tokenRange()
		p.nextToken()
		return &pattern{kind: patternNot, anchor: anchor, items: []*pattern{p.parseMatchUnaryPattern()}}
	}
	return p.parseMatchPrimaryPattern()
}

// nextTokenStartsPattern reports whether the token after the current one, on
// the same line, can start a pattern.
func (p *Parser) nextTokenStartsPattern() bool {
	p.nextToken()
	return !p.hasPrecedingLineBreak() && p.tokenStartsPattern()
}

func (p *Parser) tokenStartsPattern() bool {
	switch p.token {
	case ast.KindIdentifier, ast.KindThisKeyword, ast.KindLetKeyword, ast.KindConstKeyword, ast.KindVarKeyword,
		ast.KindOpenBracketToken, ast.KindOpenBraceToken, ast.KindOpenParenToken, ast.KindIfKeyword,
		ast.KindStringLiteral, ast.KindNumericLiteral, ast.KindBigIntLiteral, ast.KindNoSubstitutionTemplateLiteral,
		ast.KindTrueKeyword, ast.KindFalseKeyword, ast.KindNullKeyword, ast.KindMinusToken, ast.KindPlusToken,
		ast.KindSlashToken, ast.KindSlashEqualsToken, ast.KindLessThanToken, ast.KindGreaterThanToken,
		ast.KindLessThanEqualsToken, ast.KindGreaterThanEqualsToken:
		return true
	}
	return false
}

// parseMatchPrimaryPattern parses a parenthesized pattern, a binding, an
// array or object pattern, a guard, a literal, or a variable or extractor
// pattern.
func (p *Parser) parseMatchPrimaryPattern() *pattern {
	anchor := p.tokenRange()
	switch p.token {
	case ast.KindOpenParenToken:
		p.nextToken()
		inner := p.parseMatchPattern()
		p.parseExpected(ast.KindCloseParenToken)
		inner.parenthesized = true
		return inner
	case ast.KindLetKeyword, ast.KindConstKeyword, ast.KindVarKeyword:
		flags := ast.NodeFlagsNone
		switch p.token {
		case ast.KindLetKeyword:
			flags = ast.NodeFlagsLet
		case ast.KindConstKeyword:
			flags = ast.NodeFlagsConst
		}
		p.nextToken()
		return &pattern{kind: patternBinding, anchor: anchor, bindFlags: flags, name: p.parseBindingIdentifier()}
	case ast.KindOpenBracketToken:
		result := &pattern{kind: patternArray, anchor: anchor}
		p.parseMatchListPattern(result, ast.KindCloseBracketToken)
		return result
	case ast.KindOpenBraceToken:
		return p.parseMatchObjectPattern(anchor)
	case ast.KindIfKeyword:
		p.nextToken()
		p.parseExpected(ast.KindOpenParenToken)
		expr := p.parseExpressionAllowIn()
		p.parseExpected(ast.KindCloseParenToken)
		return &pattern{kind: patternGuard, anchor: anchor, expr: expr}
	case ast.KindNoSubstitutionTemplateLiteral:
		if p.scanner.TokenFlags()&ast.TokenFlagsIsInvalid != 0 {
			p.reScanTemplateToken(false /*isTaggedTemplate*/)
		}
		return &pattern{kind: patternLiteral, anchor: anchor, expr: p.parseLiteralExpression()}
	case ast.KindStringLiteral, ast.KindNumericLiteral, ast.KindBigIntLiteral:
		return &pattern{kind: patternLiteral, anchor: anchor, expr: p.parseLiteralExpression()}
	case ast.KindTrueKeyword, ast.KindFalseKeyword, ast.KindNullKeyword:
		return &pattern{kind: patternLiteral, anchor: anchor, expr: p.parseKeywordExpression()}
	case ast.KindMinusToken, ast.KindPlusToken:
		pos := p.nodePos()
		operator := p.token
		p.nextToken()
		if p.token == ast.KindNumericLiteral || p.token == ast.KindBigIntLiteral {
			literal := p.parseLiteralExpression()
			return &pattern{kind: patternLiteral, anchor: anchor, expr: p.finishNode(p.factory.NewPrefixUnaryExpression(operator, literal), pos)}
		}
		value := p.parseMatchValueExpression()
		return &pattern{kind: patternValue, anchor: anchor, expr: p.finishNode(p.factory.NewPrefixUnaryExpression(operator, value), pos)}
	case ast.KindSlashToken, ast.KindSlashEqualsToken:
		p.reScanSlashToken()
		p.parseLiteralExpression()
		return &pattern{kind: patternInvalid, anchor: anchor, code: 4704, detail: "Regular expression"}
	case ast.KindLessThanToken, ast.KindGreaterThanToken, ast.KindLessThanEqualsToken, ast.KindGreaterThanEqualsToken:
		p.nextToken()
		p.parseUnaryExpressionOrHigher()
		return &pattern{kind: patternInvalid, anchor: anchor, code: 4704, detail: "Relational"}
	case ast.KindIdentifier, ast.KindThisKeyword:
		expr := p.parseMatchValueExpression()
		if p.token == ast.KindOpenParenToken && !p.hasPrecedingLineBreak() {
			result := &pattern{kind: patternExtractor, anchor: anchor, expr: expr}
			p.parseMatchListPattern(result, ast.KindCloseParenToken)
			return result
		}
		if p.token == ast.KindOpenBraceToken && !p.hasPrecedingLineBreak() {
			// Delivery.Sent { let tracking }: a variant and its fields (D307).
			// After any other matcher, a syntax error before, the checker
			// reports the field list (EK4706).
			result := p.parseMatchObjectPattern(p.tokenRange())
			result.kind, result.anchor, result.expr = patternVariant, anchor, expr
			return result
		}
		return &pattern{kind: patternValue, anchor: anchor, expr: expr}
	}
	p.parseErrorAtCurrentToken(diagnostics.X_0_expected, "pattern")
	return &pattern{kind: patternInvalid, anchor: anchor}
}

// parseMatchValueExpression parses the identifier expression of a variable
// or extractor pattern: a name, then .name and [expression] accesses.
func (p *Parser) parseMatchValueExpression() *ast.Node {
	pos := p.nodePos()
	var expr *ast.Node
	if p.token == ast.KindThisKeyword {
		expr = p.parseKeywordExpression()
	} else {
		expr = p.parseIdentifier()
	}
	for {
		switch {
		case p.token == ast.KindDotToken:
			p.nextToken()
			name := p.parseRightSideOfDot(true /*allowIdentifierNames*/, false /*allowPrivateIdentifiers*/, true /*allowUnicodeEscapeSequenceInIdentifierName*/)
			expr = p.finishNode(p.factory.NewPropertyAccessExpression(expr, nil, name, ast.NodeFlagsNone), pos)
		case p.token == ast.KindOpenBracketToken && !p.hasPrecedingLineBreak():
			p.nextToken()
			argument := p.parseExpressionAllowIn()
			p.parseExpected(ast.KindCloseBracketToken)
			expr = p.finishNode(p.factory.NewElementAccessExpression(expr, nil, argument, ast.NodeFlagsNone), pos)
		default:
			return expr
		}
	}
}

// parseMatchListPattern parses the items of an array pattern or of an
// extractor's argument list, between open and close, into result.
func (p *Parser) parseMatchListPattern(result *pattern, close ast.Kind) {
	p.nextToken() // [ or (
	for p.token != close && p.token != ast.KindEndOfFile {
		if p.token == ast.KindDotDotDotToken {
			p.nextToken()
			result.hasRest = true
			if p.token != close && p.token != ast.KindCommaToken {
				result.rest = p.parseMatchPattern()
			}
			p.parseOptional(ast.KindCommaToken)
			break
		}
		result.items = append(result.items, p.parseMatchPattern())
		if !p.parseOptional(ast.KindCommaToken) {
			break
		}
	}
	p.parseExpected(close)
}

// parseMatchObjectPattern parses { key, let name, key: pattern, [key]: pattern, ...pattern }.
func (p *Parser) parseMatchObjectPattern(anchor core.TextRange) *pattern {
	result := &pattern{kind: patternObject, anchor: anchor}
	p.nextToken() // {
	for p.token != ast.KindCloseBraceToken && p.token != ast.KindEndOfFile {
		switch {
		case p.token == ast.KindDotDotDotToken:
			p.nextToken()
			result.hasRest = true
			result.rest = p.parseMatchPattern()
		case p.token == ast.KindLetKeyword || p.token == ast.KindConstKeyword || p.token == ast.KindVarKeyword:
			binding := p.parseMatchPrimaryPattern()
			key := p.vibelangSynthesized(p.factory.NewIdentifier(binding.name.Text()), binding.name.Loc)
			result.clauses = append(result.clauses, objectClause{key: key, pattern: binding})
		case p.token == ast.KindOpenBracketToken:
			keyAnchor := p.tokenRange()
			p.nextToken()
			key := p.parseAssignmentExpressionOrHigher()
			p.parseExpected(ast.KindCloseBracketToken)
			p.parseExpected(ast.KindColonToken)
			sub := p.parseMatchPattern()
			if p.cloneIdentExpression(key) == nil && key.Kind != ast.KindStringLiteral && key.Kind != ast.KindNumericLiteral {
				result.items = append(result.items, &pattern{kind: patternInvalid, anchor: keyAnchor, code: 4704, detail: "A computed key other than a name or a literal"})
				continue
			}
			// A literal key reads the same as a written one, ["a"] as "a".
			result.clauses = append(result.clauses, objectClause{key: key, computed: !isLiteralKey(key), pattern: sub})
		default:
			key := p.parsePropertyName()
			clause := objectClause{key: key}
			if key.Kind == ast.KindComputedPropertyName {
				clause.key = key.Expression()
				clause.computed = !isLiteralKey(clause.key)
			}
			if p.parseOptional(ast.KindColonToken) {
				clause.pattern = p.parseMatchPattern()
			}
			result.clauses = append(result.clauses, clause)
		}
		if !p.parseOptional(ast.KindCommaToken) {
			break
		}
	}
	p.parseExpected(ast.KindCloseBraceToken)
	return result
}

// isVibeLangIs reports whether the current token is the is operator: in an
// .vibe file, the identifier is on the line of its subject, followed on the
// same line by a pattern. An expression followed by an identifier is a
// TypeScript syntax error today.
func (p *Parser) isVibeLangIs() bool {
	return p.token == ast.KindIsKeyword && !p.hasPrecedingLineBreak() && p.isVibeLangFile() && p.lookAhead((*Parser).nextTokenStartsPattern)
}

// parseVibeLangIs parses subject is pattern, after subject. Without bindings
// it is a boolean expression; with bindings it is a marker that an if or
// while condition rewrites (restructureVibeLangIs), and EK4703 anywhere else.
func (p *Parser) parseVibeLangIs(subject *ast.Node, pos int) *ast.Node {
	info := &isInfo{anchor: p.tokenRange(), subject: subject}
	p.nextToken() // is
	patternStart := p.scanner.TokenStart()
	info.pattern = p.parseMatchPattern()
	p.recordVibeSyntax(VibeSyntax{Kind: VibeSyntaxIs, Word: info.anchor, Right: core.NewTextRange(patternStart, p.nodePos()), Range: core.NewTextRange(info.anchor.Pos(), p.nodePos())})
	if !patternHasBindings(info.pattern) {
		return p.isExpression(info, pos)
	}
	marker := p.patternErrorCall(4703, "", info.anchor)
	marker = p.vibelangSpanning(marker, core.NewTextRange(pos, p.nodePos()))
	if p.vibelangIsBindings == nil {
		p.vibelangIsBindings = map[*ast.Node]*isInfo{}
	}
	p.vibelangIsBindings[marker] = info
	return marker
}

// patternHasBindings reports whether pat binds a name.
func patternHasBindings(pat *pattern) bool {
	if pat == nil {
		return false
	}
	switch pat.kind {
	case patternBinding:
		return true
	case patternObject, patternVariant:
		for _, clause := range pat.clauses {
			if patternHasBindings(clause.pattern) {
				return true
			}
		}
	}
	for _, item := range pat.items {
		if patternHasBindings(item) {
			return true
		}
	}
	return patternHasBindings(pat.rest)
}

// ---------------------------------------------------------------- writing

func (p *Parser) mIdent(name string, at core.TextRange) *ast.Node {
	return p.vibelangSynthesized(p.factory.NewIdentifier(name), at)
}

func (p *Parser) mToken(kind ast.Kind, at core.TextRange) *ast.Node {
	return p.vibelangSynthesized(p.factory.NewToken(kind), at)
}

func (p *Parser) mString(text string, at core.TextRange) *ast.Node {
	return p.vibelangSynthesized(p.factory.NewStringLiteral(text, ast.TokenFlagsNone), at)
}

func (p *Parser) mBinary(left *ast.Node, operator ast.Kind, right *ast.Node, at core.TextRange) *ast.Node {
	return p.vibelangSpanning(p.factory.NewBinaryExpression(nil, left, nil, p.mToken(operator, at), right), at)
}

func (p *Parser) mParens(expr *ast.Node, at core.TextRange) *ast.Node {
	return p.vibelangSpanning(p.factory.NewParenthesizedExpression(expr), at)
}

func (p *Parser) mCall(callee *ast.Node, args []*ast.Node, at core.TextRange) *ast.Node {
	return p.vibelangSpanning(p.factory.NewCallExpression(callee, nil, nil, p.newNodeList(at, args), ast.NodeFlagsNone), at)
}

func (p *Parser) mProperty(expr *ast.Node, name string, at core.TextRange) *ast.Node {
	return p.vibelangSpanning(p.factory.NewPropertyAccessExpression(expr, nil, p.mIdent(name, at), ast.NodeFlagsNone), at)
}

func (p *Parser) mElement(expr *ast.Node, argument *ast.Node, at core.TextRange) *ast.Node {
	return p.vibelangSpanning(p.factory.NewElementAccessExpression(expr, nil, argument, ast.NodeFlagsNone), at)
}

func (p *Parser) mNumber(n int, at core.TextRange) *ast.Node {
	return p.vibelangSynthesized(p.factory.NewNumericLiteral(strconv.Itoa(n), ast.TokenFlagsNone), at)
}

func (p *Parser) mKeyword(kind ast.Kind, at core.TextRange) *ast.Node {
	return p.vibelangSynthesized(p.factory.NewKeywordExpression(kind), at)
}

func (p *Parser) mBlock(statements []*ast.Node, at core.TextRange) *ast.Node {
	return p.vibelangSpanning(p.factory.NewBlock(p.newNodeList(at, statements), true /*multiLine*/), at)
}

func (p *Parser) mConst(name *ast.Node, flags ast.NodeFlags, init *ast.Node, at core.TextRange) *ast.Node {
	declaration := p.vibelangSpanning(p.factory.NewVariableDeclaration(name, nil, nil, init), at)
	list := p.vibelangSpanning(p.factory.NewVariableDeclarationList(p.newNodeList(at, []*ast.Node{declaration}), flags), at)
	return p.vibelangSpanning(p.factory.NewVariableStatement(nil, list), at)
}

// patternErrorCall writes $patternError(code) or $patternError(code, detail),
// which the checker reports at at.
func (p *Parser) patternErrorCall(code int, detail string, at core.TextRange) *ast.Node {
	args := []*ast.Node{p.mNumber(code, at)}
	if detail != "" {
		args = append(args, p.mString(detail, at))
	}
	return p.mCall(p.mIdent("$patternError", at), args, at)
}

// cloneIdentExpression writes a new copy of expr, an identifier expression:
// a name or this, followed by property and element accesses whose arguments
// are literals. It returns nil for any other expression.
func (p *Parser) cloneIdentExpression(expr *ast.Node) *ast.Node {
	switch expr.Kind {
	case ast.KindIdentifier:
		return p.mIdent(expr.Text(), expr.Loc)
	case ast.KindThisKeyword:
		start := scanner.SkipTrivia(p.sourceText, expr.Pos())
		return p.mKeyword(ast.KindThisKeyword, core.NewTextRange(start, start+len("this")))
	case ast.KindPropertyAccessExpression:
		base := p.cloneIdentExpression(expr.Expression())
		name := expr.Name()
		if base == nil || name.Kind != ast.KindIdentifier {
			return nil
		}
		return p.vibelangSpanning(p.factory.NewPropertyAccessExpression(base, nil, p.mIdent(name.Text(), name.Loc), ast.NodeFlagsNone), expr.Loc)
	case ast.KindElementAccessExpression:
		base := p.cloneIdentExpression(expr.Expression())
		argument := expr.AsElementAccessExpression().ArgumentExpression
		if base == nil {
			return nil
		}
		var copied *ast.Node
		switch argument.Kind {
		case ast.KindStringLiteral:
			copied = p.mString(argument.Text(), argument.Loc)
		case ast.KindNumericLiteral:
			copied = p.vibelangSynthesized(p.factory.NewNumericLiteral(argument.Text(), ast.TokenFlagsNone), argument.Loc)
		default:
			return nil
		}
		return p.mElement(base, copied, expr.Loc)
	}
	return nil
}

// subjectPath holds a match or is subject in a const that reads it once,
// before any pattern runs, as JavaScript's switch reads its discriminant
// once (D214). It returns the path of the const and its declaration, to
// write first. The declaration starts at the anchor, so the references
// written at the anchor do not come before it for TypeScript's
// use-before-declaration check, and it stands outside every arm, so a
// binding that shadows the subject's name cannot put the read in its
// temporal dead zone. The checker narrows the subject through the const,
// as TypeScript narrows a switch's discriminant in its clauses
// (match_subject.go in package vibelang).
func (p *Parser) subjectPath(subject *ast.Node, anchor core.TextRange, prefix string) (matchPath, *ast.Node) {
	p.vibelangMatchTemps++
	name := prefix + strconv.Itoa(p.vibelangMatchTemps)
	decl := p.mConst(p.mIdent(name, anchor), ast.NodeFlagsConst, subject, core.NewTextRange(min(anchor.Pos(), subject.Pos()), subject.End()))
	return func() *ast.Node { return p.mIdent(name, anchor) }, decl
}

// propertyPath returns the path of the property key of base's subject.
func (p *Parser) propertyPath(base matchPath, clause objectClause) matchPath {
	key := clause.key
	return func() *ast.Node {
		switch {
		case !clause.computed && key.Kind == ast.KindIdentifier:
			return p.mProperty(base(), key.Text(), key.Loc)
		case !clause.computed && key.Kind == ast.KindStringLiteral:
			return p.mElement(base(), p.mString(key.Text(), key.Loc), key.Loc)
		case !clause.computed && key.Kind == ast.KindNumericLiteral:
			return p.mElement(base(), p.vibelangSynthesized(p.factory.NewNumericLiteral(key.Text(), ast.TokenFlagsNone), key.Loc), key.Loc)
		}
		return p.mElement(base(), p.cloneIdentExpression(key), key.Loc)
	}
}

func isLiteralKey(key *ast.Node) bool {
	return key.Kind == ast.KindStringLiteral || key.Kind == ast.KindNumericLiteral
}

// propertyKeyTest writes "key" in path(), or key in path() for a computed key.
func (p *Parser) propertyKeyTest(path matchPath, clause objectClause, at core.TextRange) *ast.Node {
	key := clause.key
	var left *ast.Node
	switch {
	case !clause.computed && (key.Kind == ast.KindIdentifier || key.Kind == ast.KindStringLiteral):
		left = p.mString(key.Text(), key.Loc)
	case !clause.computed && key.Kind == ast.KindNumericLiteral:
		left = p.vibelangSynthesized(p.factory.NewNumericLiteral(key.Text(), ast.TokenFlagsNone), key.Loc)
	default:
		left = p.cloneIdentExpression(key)
	}
	return p.mBinary(left, ast.KindInKeyword, path(), at)
}

// knownMatcher is a dotted name the compiler matches with an Effect guard:
// test is the guard's module and name, and values the properties the
// extractor yields.
type knownMatcher struct {
	module, guard string
	values        []string
	typeofName    string // for the classes of primitives: the typeof result
}

var knownMatchers = map[string]knownMatcher{
	"Option.Some":    {module: "Option", guard: "isSome", values: []string{"value"}},
	"Option.None":    {module: "Option", guard: "isNone"},
	"Exit.Success":   {module: "Exit", guard: "isSuccess", values: []string{"value"}},
	"Exit.Failure":   {module: "Exit", guard: "isFailure", values: []string{"cause"}},
	"Result.Success": {module: "Result", guard: "isSuccess", values: []string{"success"}},
	"Result.Failure": {module: "Result", guard: "isFailure", values: []string{"failure"}},
	"String":         {typeofName: "string"},
	"Number":         {typeofName: "number"},
	"Boolean":        {typeofName: "boolean"},
	"BigInt":         {typeofName: "bigint"},
	"Symbol":         {typeofName: "symbol"},
}

// dottedName returns the text of expr when it is a name or a name.name, or "".
func dottedName(expr *ast.Node) string {
	switch expr.Kind {
	case ast.KindIdentifier:
		return expr.Text()
	case ast.KindPropertyAccessExpression:
		if base := expr.Expression(); base.Kind == ast.KindIdentifier && expr.Name().Kind == ast.KindIdentifier {
			return base.Text() + "." + expr.Name().Text()
		}
	}
	return ""
}

// knownTest writes the test of a known matcher against path.
func (p *Parser) knownTest(known knownMatcher, path matchPath, at core.TextRange) *ast.Node {
	if known.typeofName != "" {
		typeOf := p.vibelangSpanning(p.factory.NewTypeOfExpression(path()), at)
		return p.mBinary(typeOf, ast.KindEqualsEqualsEqualsToken, p.mString(known.typeofName, at), at)
	}
	guard := p.mProperty(p.mIdent(known.module, at), known.guard, at)
	return p.mCall(guard, []*ast.Node{path()}, at)
}

// matcherTest writes $is(matcher, subject) for pat, a variable or variant
// pattern. The call spans the matcher, so that the language service finds
// a dotted matcher's names in it.
func (p *Parser) matcherTest(pat *pattern, path matchPath) *ast.Node {
	at := pat.anchor
	span := core.NewTextRange(at.Pos(), max(at.End(), pat.expr.End()))
	return p.mCall(p.mIdent("$is", at), []*ast.Node{pat.expr, path()}, span)
}

// compilePattern appends the steps that match pat against path to steps.
func (p *Parser) compilePattern(pat *pattern, path matchPath, steps *[]matchStep, ctx *matchCompile) {
	at := pat.anchor
	test := func(t *ast.Node) { *steps = append(*steps, matchStep{test: t}) }
	if pat.code == 4705 {
		// Combinators sharing a level have no meaning to compile; EK4705 reports them.
		test(p.patternErrorCall(4705, "", at))
		return
	}
	switch pat.kind {
	case patternInvalid:
		if pat.code != 0 {
			test(p.patternErrorCall(pat.code, pat.detail, at))
		}
	case patternLiteral:
		test(p.mBinary(path(), ast.KindEqualsEqualsEqualsToken, pat.expr, at))
	case patternValue:
		if pat.expr.Kind == ast.KindIdentifier && pat.expr.Text() == "undefined" {
			test(p.mBinary(path(), ast.KindEqualsEqualsEqualsToken, pat.expr, at))
		} else if known, ok := knownMatchers[dottedName(pat.expr)]; ok {
			test(p.knownTest(known, path, at))
		} else {
			test(p.matcherTest(pat, path))
		}
	case patternBinding:
		if ctx.inline {
			test(p.patternErrorCall(4702, "", at))
			return
		}
		*steps = append(*steps, matchStep{decl: p.mConst(pat.name, pat.bindFlags, path(), pat.name.Loc)})
	case patternGuard:
		test(p.mParens(pat.expr, at))
	case patternArray:
		test(p.mCall(p.mProperty(p.mIdent("Array", at), "isArray", at), []*ast.Node{path()}, at))
		p.compileList(pat, path, len(pat.items), func(i int) matchPath {
			return func() *ast.Node { return p.mElement(path(), p.mNumber(i, at), at) }
		}, func(n int) *ast.Node {
			return p.mCall(p.mProperty(path(), "slice", at), []*ast.Node{p.mNumber(n, at)}, at)
		}, func() *ast.Node { return p.mProperty(path(), "length", at) }, steps, ctx)
	case patternObject, patternVariant:
		var names []func() *ast.Node
		if pat.kind == patternVariant {
			// The variant's test; its tag is never part of a rest.
			test(p.matcherTest(pat, path))
			names = append(names, func() *ast.Node { return p.mString("_tag", at) })
		} else {
			typeOf := p.vibelangSpanning(p.factory.NewTypeOfExpression(path()), at)
			test(p.mBinary(typeOf, ast.KindEqualsEqualsEqualsToken, p.mString("object", at), at))
			test(p.mBinary(path(), ast.KindExclamationEqualsEqualsToken, p.mKeyword(ast.KindNullKeyword, at), at))
		}
		for _, clause := range pat.clauses {
			if pat.kind == patternObject || clause.pattern == nil {
				test(p.propertyKeyTest(path, clause, at))
			}
			key := clause.key
			if clause.computed {
				names = append(names, func() *ast.Node { return p.cloneIdentExpression(key) })
			} else {
				names = append(names, func() *ast.Node { return p.mString(key.Text(), key.Loc) })
			}
			if clause.pattern != nil {
				p.compilePattern(clause.pattern, p.propertyPath(path, clause), steps, ctx)
			}
		}
		for _, item := range pat.items {
			p.compilePattern(item, path, steps, ctx) // a clause VibeLang does not support (patternInvalid)
		}
		if pat.rest != nil {
			restPath := func() *ast.Node {
				var copies []*ast.Node
				for _, name := range names {
					copies = append(copies, name())
				}
				keys := p.vibelangSpanning(p.factory.NewArrayLiteralExpression(p.newNodeList(at, copies), false), at)
				return p.mCall(p.mIdent("$rest", at), []*ast.Node{path(), keys}, at)
			}
			p.compilePattern(pat.rest, restPath, steps, ctx)
		}
	case patternExtractor:
		if known, ok := knownMatchers[dottedName(pat.expr)]; ok {
			test(p.knownTest(known, path, at))
			values := known.values
			if known.typeofName != "" {
				values = []string{""}
			}
			if len(pat.items) > len(values) || (!pat.hasRest && len(pat.items) != len(values)) {
				test(p.mKeyword(ast.KindFalseKeyword, at))
				return
			}
			element := func(i int) matchPath {
				if values[i] == "" {
					return path
				}
				name := values[i]
				return func() *ast.Node { return p.mProperty(path(), name, at) }
			}
			for i, item := range pat.items {
				p.compilePattern(item, element(i), steps, ctx)
			}
			if pat.rest != nil {
				restPath := func() *ast.Node {
					var elements []*ast.Node
					for i := len(pat.items); i < len(values); i++ {
						elements = append(elements, element(i)())
					}
					return p.vibelangSpanning(p.factory.NewArrayLiteralExpression(p.newNodeList(at, elements), false), at)
				}
				p.compilePattern(pat.rest, restPath, steps, ctx)
			}
			return
		}
		// A matcher the compiler does not know: $extract runs its custom matcher.
		matcher := pat.expr
		var extracted matchPath
		if ctx.inline {
			first := true
			extracted = func() *ast.Node {
				m := matcher
				if !first {
					if copied := p.cloneIdentExpression(matcher); copied != nil {
						m = copied
					}
				}
				first = false
				return p.mCall(p.mIdent("$extract", at), []*ast.Node{m, path()}, at)
			}
		} else {
			p.vibelangMatchTemps++
			name := "$extracted" + strconv.Itoa(p.vibelangMatchTemps)
			*steps = append(*steps, matchStep{decl: p.mConst(p.mIdent(name, at), ast.NodeFlagsConst, p.mCall(p.mIdent("$extract", at), []*ast.Node{matcher, path()}, at), at)})
			extracted = func() *ast.Node { return p.mIdent(name, at) }
		}
		test(p.mBinary(extracted(), ast.KindExclamationEqualsEqualsToken, p.mKeyword(ast.KindFalseKeyword, at), at))
		p.compileList(pat, extracted, len(pat.items), func(i int) matchPath {
			return func() *ast.Node { return p.mElement(extracted(), p.mNumber(i, at), at) }
		}, func(n int) *ast.Node {
			return p.mCall(p.mProperty(extracted(), "slice", at), []*ast.Node{p.mNumber(n, at)}, at)
		}, func() *ast.Node { return p.mProperty(extracted(), "length", at) }, steps, ctx)
	case patternAnd:
		for _, item := range pat.items {
			p.compilePattern(item, path, steps, ctx)
		}
	case patternOr:
		var alternatives []*ast.Node
		for _, item := range pat.items {
			alternatives = append(alternatives, p.mParens(p.inlinePattern(item, path, ctx), at))
		}
		test(p.mParens(p.joinTests(alternatives, ast.KindBarBarToken, at), at))
	case patternNot:
		inner := p.mParens(p.inlinePattern(pat.items[0], path, ctx), at)
		test(p.vibelangSpanning(p.factory.NewPrefixUnaryExpression(ast.KindExclamationToken, inner), at))
	}
}

// compileList appends the steps of an array pattern or argument list: the
// length test, each item against its element, and the rest pattern against
// the remaining elements.
func (p *Parser) compileList(pat *pattern, path matchPath, n int, element func(int) matchPath, remaining func(int) *ast.Node, length func() *ast.Node, steps *[]matchStep, ctx *matchCompile) {
	at := pat.anchor
	operator := ast.KindEqualsEqualsEqualsToken
	if pat.hasRest {
		operator = ast.KindGreaterThanEqualsToken
	}
	*steps = append(*steps, matchStep{test: p.mBinary(length(), operator, p.mNumber(n, at), at)})
	for i, item := range pat.items {
		p.compilePattern(item, element(i), steps, ctx)
	}
	if pat.rest != nil {
		p.compilePattern(pat.rest, func() *ast.Node { return remaining(n) }, steps, ctx)
	}
}

// inlinePattern writes pat as one boolean expression, for or, not, and an
// is outside an if or while condition. A binding there is EK4702.
func (p *Parser) inlinePattern(pat *pattern, path matchPath, ctx *matchCompile) *ast.Node {
	inner := &matchCompile{anchor: ctx.anchor, inline: true}
	var steps []matchStep
	p.compilePattern(pat, path, &steps, inner)
	var tests []*ast.Node
	for _, step := range steps {
		if step.test != nil {
			tests = append(tests, step.test)
		}
	}
	if len(tests) == 0 {
		return p.mKeyword(ast.KindTrueKeyword, pat.anchor)
	}
	return p.joinTests(tests, ast.KindAmpersandAmpersandToken, pat.anchor)
}

// joinTests joins tests with operator, left to right.
func (p *Parser) joinTests(tests []*ast.Node, operator ast.Kind, at core.TextRange) *ast.Node {
	result := tests[0]
	for _, t := range tests[1:] {
		result = p.mBinary(result, operator, t, at)
	}
	return result
}

// emitSteps writes steps as statements: each run of tests becomes an if
// whose block holds the rest, each declaration a statement, and success
// runs when every test passed.
func (p *Parser) emitSteps(steps []matchStep, success []*ast.Node, at core.TextRange) []*ast.Node {
	var out []*ast.Node
	i := 0
	for i < len(steps) && steps[i].decl != nil {
		out = append(out, steps[i].decl)
		i++
	}
	if i == len(steps) {
		return append(out, success...)
	}
	var tests []*ast.Node
	for i < len(steps) && steps[i].test != nil {
		tests = append(tests, steps[i].test)
		i++
	}
	inner := p.emitSteps(steps[i:], success, at)
	// The if and its block span the statements they hold, such as an arm's
	// return of its value, so that a position in the value is inside them
	// and the language service can find the value's tokens there.
	span := at
	for _, statement := range inner {
		if statement.End() > span.End() {
			span = core.NewTextRange(at.Pos(), statement.End())
		}
	}
	then := p.mBlock(inner, span)
	return append(out, p.vibelangSpanning(p.factory.NewIfStatement(p.joinTests(tests, ast.KindAmpersandAmpersandToken, at), then, nil), span))
}

// isExpression writes subject is pattern, a pattern without bindings, as a
// boolean expression. A pattern that reads its subject once tests an
// identifier expression in place, so the checker narrows it. Any other
// subject, and one that the pattern reads more than once or not at all, is
// held by the parameter of a function called on it, so it is evaluated
// once; the checker narrows an identifier expression through the parameter
// (match_subject.go in package vibelang).
func (p *Parser) isExpression(info *isInfo, pos int) *ast.Node {
	ctx := &matchCompile{anchor: info.anchor, inline: true}
	inner := ast.SkipParentheses(info.subject)
	if p.cloneIdentExpression(inner) != nil {
		// Count the reads with copies first, so that a pattern which reads
		// the subject more than once leaves the subject node to the call
		// below.
		reads := 0
		p.inlinePattern(info.pattern, func() *ast.Node { reads++; return p.cloneIdentExpression(inner) }, ctx)
		if reads == 1 {
			return p.finishNode(p.mParens(p.inlinePattern(info.pattern, func() *ast.Node { return inner }, ctx), info.anchor), pos)
		}
	}
	p.vibelangMatchTemps++
	name := "$is" + strconv.Itoa(p.vibelangMatchTemps)
	path := func() *ast.Node { return p.mIdent(name, info.anchor) }
	body := p.inlinePattern(info.pattern, path, ctx)
	parameter := p.vibelangSpanning(p.factory.NewParameterDeclaration(nil, nil, p.mIdent(name, info.anchor), nil, nil, nil), info.anchor)
	arrow := p.vibelangProgramArrow([]*ast.Node{parameter}, body, info.anchor)
	callee := p.mParens(arrow, info.anchor)
	call := p.vibelangSpanning(p.factory.NewCallExpression(callee, nil, nil, p.newNodeList(info.subject.Loc, []*ast.Node{info.subject}), ast.NodeFlagsNone), core.NewTextRange(pos, p.nodePos()))
	return p.finishNode(call, pos)
}

// matchStatements writes the arms of a match: the subject's const, then a
// block per arm whose tests guard success(value), then the default arm or
// the fallthrough for an unmatched subject.
func (p *Parser) matchStatements(info *matchInfo, success func(value *ast.Node, at core.TextRange) []*ast.Node, unmatched func(subject *ast.Node, at core.TextRange) *ast.Node) []*ast.Node {
	path, decl := p.subjectPath(info.subject, info.anchor, "$match")
	out := []*ast.Node{decl}
	defaulted := false
	for _, arm := range info.arms {
		span := core.NewTextRange(arm.anchor.Pos(), arm.end)
		if arm.pattern == nil {
			out = append(out, success(arm.value, span)...)
			defaulted = true
			continue
		}
		var steps []matchStep
		p.compilePattern(arm.pattern, path, &steps, &matchCompile{anchor: arm.anchor})
		out = append(out, p.mBlock(p.emitSteps(steps, success(arm.value, span), arm.anchor), span))
	}
	if !defaulted {
		noMatch := p.mCall(p.mIdent("$noMatch", info.anchor), []*ast.Node{path()}, info.anchor)
		out = append(out, unmatched(noMatch, info.anchor))
	}
	return out
}

// matchFunctionCall writes a match expression as a function called where it
// stands, whose arms return their values.
func (p *Parser) matchFunctionCall(info *matchInfo) *ast.Node {
	statements := p.matchStatements(info, func(value *ast.Node, at core.TextRange) []*ast.Node {
		return []*ast.Node{p.vibelangSpanning(p.factory.NewReturnStatement(value), at)}
	}, func(noMatch *ast.Node, at core.TextRange) *ast.Node {
		return p.vibelangSpanning(p.factory.NewReturnStatement(noMatch), at)
	})
	span := core.NewTextRange(info.anchor.End(), p.nodePos())
	body := p.mBlock(statements, span)
	arrow := p.vibelangProgramArrow(nil, body, info.anchor)
	callee := p.vibelangSpanning(p.factory.NewParenthesizedExpression(arrow), span)
	call := p.factory.NewCallExpression(callee, nil, nil, p.newNodeList(core.NewTextRange(p.nodePos(), p.nodePos()), nil), ast.NodeFlagsNone)
	return p.finishNode(call, info.pos)
}

// matchReturnStatement writes return match (...) { ... } as a block whose
// arms return their values.
func (p *Parser) matchReturnStatement(info *matchInfo, pos int) *ast.Node {
	statements := p.matchStatements(info, func(value *ast.Node, at core.TextRange) []*ast.Node {
		return []*ast.Node{p.vibelangSpanning(p.factory.NewReturnStatement(value), at)}
	}, func(noMatch *ast.Node, at core.TextRange) *ast.Node {
		return p.vibelangSpanning(p.factory.NewReturnStatement(noMatch), at)
	})
	return p.finishNode(p.factory.NewBlock(p.newNodeList(core.NewTextRange(pos, p.nodePos()), statements), true /*multiLine*/), pos)
}

// matchStatement writes a match that stands as a statement as a labeled
// block whose arms evaluate their values and leave it.
func (p *Parser) matchStatement(info *matchInfo, pos int) *ast.Node {
	p.vibelangMatchTemps++
	label := "$match" + strconv.Itoa(p.vibelangMatchTemps)
	statements := p.matchStatements(info, func(value *ast.Node, at core.TextRange) []*ast.Node {
		leave := p.vibelangSpanning(p.factory.NewBreakStatement(p.mIdent(label, info.anchor)), info.anchor)
		return []*ast.Node{p.vibelangSpanning(p.factory.NewExpressionStatement(value), at), leave}
	}, func(noMatch *ast.Node, at core.TextRange) *ast.Node {
		return p.vibelangSpanning(p.factory.NewExpressionStatement(noMatch), at)
	})
	block := p.finishNode(p.factory.NewBlock(p.newNodeList(core.NewTextRange(info.anchor.End(), p.nodePos()), statements), true /*multiLine*/), info.anchor.End())
	return p.finishNode(p.factory.NewLabeledStatement(p.mIdent(label, info.anchor), block), pos)
}

// conjuncts returns the operands of the && chain expr, outermost first.
func conjuncts(expr *ast.Node) []*ast.Node {
	inner := ast.SkipParentheses(expr)
	if ast.IsBinaryExpression(inner) && inner.AsBinaryExpression().OperatorToken.Kind == ast.KindAmpersandAmpersandToken {
		b := inner.AsBinaryExpression()
		return append(conjuncts(b.Left), conjuncts(b.Right)...)
	}
	return []*ast.Node{inner}
}

// conditionSteps returns the steps of condition when an is in its && chain
// binds names, or nil.
func (p *Parser) conditionSteps(condition *ast.Node) []matchStep {
	parts := conjuncts(condition)
	found := false
	for _, part := range parts {
		if p.vibelangIsBindings[part] != nil {
			found = true
		}
	}
	if !found {
		return nil
	}
	var steps []matchStep
	for _, part := range parts {
		info := p.vibelangIsBindings[part]
		if info == nil {
			steps = append(steps, matchStep{test: part})
			continue
		}
		path, decl := p.subjectPath(info.subject, info.anchor, "$is")
		steps = append(steps, matchStep{decl: decl})
		p.compilePattern(info.pattern, path, &steps, &matchCompile{anchor: info.anchor})
	}
	return steps
}

// endsInJump reports whether statement ends with return, throw, break, or
// continue, so nothing written after it runs.
func endsInJump(statement *ast.Node) bool {
	if statement.Kind == ast.KindBlock {
		statements := statement.AsBlock().Statements.Nodes
		if len(statements) == 0 {
			return false
		}
		statement = statements[len(statements)-1]
	}
	switch statement.Kind {
	case ast.KindReturnStatement, ast.KindThrowStatement, ast.KindBreakStatement, ast.KindContinueStatement:
		return true
	}
	return false
}

// restructureVibeLangIf rewrites if (... && x is P && ...) S else E, when P
// binds names, so the bindings are in scope in S:
//
//	$isN: { if (...) { const name = ...; if (...) { S; break $isN } } E }
//
// It returns nil when no is in the condition binds names.
func (p *Parser) restructureVibeLangIf(condition, then, otherwise *ast.Node, pos int) *ast.Node {
	steps := p.conditionSteps(condition)
	if steps == nil {
		return nil
	}
	anchor := p.keywordRangeAt(pos, "if")
	if otherwise == nil {
		return p.finishNode(p.factory.NewBlock(p.newNodeList(core.NewTextRange(pos, p.nodePos()), p.emitSteps(steps, []*ast.Node{then}, anchor)), true /*multiLine*/), pos)
	}
	p.vibelangMatchTemps++
	label := "$is" + strconv.Itoa(p.vibelangMatchTemps)
	success := []*ast.Node{then}
	if !endsInJump(then) {
		success = append(success, p.vibelangSpanning(p.factory.NewBreakStatement(p.mIdent(label, anchor)), anchor))
	}
	statements := append(p.emitSteps(steps, success, anchor), otherwise)
	block := p.finishNode(p.factory.NewBlock(p.newNodeList(core.NewTextRange(pos, p.nodePos()), statements), true /*multiLine*/), pos)
	return p.finishNode(p.factory.NewLabeledStatement(p.mIdent(label, anchor), block), pos)
}

// restructureVibeLangWhile rewrites while (... && x is P && ...) S, when P
// binds names, so the bindings are in scope in S and the condition runs
// before each iteration:
//
//	while (true) { if (...) { const name = ...; if (...) { S; continue } } break }
//
// It returns nil when no is in the condition binds names.
func (p *Parser) restructureVibeLangWhile(condition, body *ast.Node, pos int) *ast.Node {
	steps := p.conditionSteps(condition)
	if steps == nil {
		return nil
	}
	anchor := p.keywordRangeAt(pos, "while")
	success := []*ast.Node{body}
	if !endsInJump(body) {
		success = append(success, p.vibelangSpanning(p.factory.NewContinueStatement(nil), anchor))
	}
	statements := append(p.emitSteps(steps, success, anchor), p.vibelangSpanning(p.factory.NewBreakStatement(nil), anchor))
	loopBody := p.finishNode(p.factory.NewBlock(p.newNodeList(core.NewTextRange(pos, p.nodePos()), statements), true /*multiLine*/), pos)
	return p.finishNode(p.factory.NewWhileStatement(p.mKeyword(ast.KindTrueKeyword, anchor), loopBody), pos)
}

// keywordRangeAt returns the range of keyword, which starts the statement at pos.
func (p *Parser) keywordRangeAt(pos int, keyword string) core.TextRange {
	start := scanner.SkipTrivia(p.sourceText, pos)
	return core.NewTextRange(start, min(start+len(keyword), len(p.sourceText)))
}
