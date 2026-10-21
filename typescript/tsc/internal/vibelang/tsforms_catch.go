package vibelang

import (
	"slices"
	"strconv"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// EK4835 (D289): Effect's catch combinators in a VibeLang function, which a
// catch expression (D236) and a match (D206, D216) write:
//
//	Effect.catchTag(find(id), "NotFound", (e) => `no user ${e.id}`)
//	find(id) catch (e) => match (e) { when NotFound: `no user ${e.id}`; default: throw e; }
//
//	Effect.catch(find(id), (e) => describe(e))
//	find(id) catch (e) => describe(e)
//
// Effect.catchTag(self, "Tag", handler) and Effect.catchTags(self, { Tag:
// handler, … }) become one arm per tag and a default that rethrows. An arm
// tests the failure's _tag with an object pattern, { _tag: "Tag" }, which is
// what catchTag tests; when the failures of that tag are the instances of one
// error class the file names, whose declaration fixes the tag (an error
// declaration, or a class extending TaggedError or Schema.TaggedError with
// that tag), and no other failure extends it, the arm names the class
// instead, as idiomatic VibeLang does. Effect.catch(self, handler) becomes
// the catch expression with the handler. The left side runs inside the catch
// as the program argument did (D147); an Effect value is called to run it.
// A handler must be an arrow function with at most one plain parameter and an
// expression body, or a block that only returns one; the catch expression's
// variable takes its parameter's name. A handler whose value is an Effect
// value is called, since catchTag runs what its handler returns. Every
// failure in the program's E must have a literal _tag for catchTag and
// catchTags, so that the match removes exactly the failures catchTag would.
// One difference is VibeLang's by design (D13, D236): a cause that mixes an
// expected failure with a defect or an interruption passes through a catch
// expression, while Effect's combinators recover from the failure and drop
// the rest. Refused: the data-last forms inside pipe, type arguments, a tag
// that is not a string literal, and anything outside a VibeLang function body.

// catchHandlerMessage is EK4835's text for Effect.catch, whose handler the
// catch expression keeps.
var catchHandlerMessage = EK4835.With("{0} takes a handler; write a catch expression: x catch (e) => handler.")

func init() {
	tsFormRules[EK4835.Code] = &tsFormRule{
		message:   EK4835,
		find:      findCatchCombinators,
		plan:      catchCombinatorRewrite,
		diagnosed: ast.IsCallExpression,
	}
}

// catchCombinatorOf returns the name of the Effect catch combinator that call
// calls in its data-first form, or "".
func catchCombinatorOf(c *checker.Checker, call *ast.Node) string {
	if !ast.IsCallExpression(call) || call.TypeArguments() != nil || !inLowerableBody(call) || inHatch(call) {
		return ""
	}
	for _, argument := range call.Arguments() {
		if ast.IsSpreadElement(argument) {
			return ""
		}
	}
	if !isEffectFunction(c, call.Expression(), "Effect", "catchTag", "catchTags", "catch") {
		return ""
	}
	_, name := effectMember(c, call.Expression())
	switch {
	case name == "catchTag" && len(call.Arguments()) == 3, name == "catchTags" && len(call.Arguments()) == 2, name == "catch" && len(call.Arguments()) == 2:
		return name
	}
	return ""
}

// findCatchCombinators reports EK4835 at each catch combinator of file that a
// catch expression writes.
func findCatchCombinators(c *checker.Checker, file *ast.SourceFile, report func(at *ast.Node, rewrite *formRewrite, args ...any)) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if name := catchCombinatorOf(c, node); name != "" {
			if rewrite := catchCombinatorRewrite(c, file, node); rewrite.fixable() {
				report(node, rewrite, "Effect."+name)
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// catchHandler is a handler's parameter name, or "", and its value.
type catchHandler struct {
	parameter string
	value     *ast.Node
}

// catchHandlerOf returns handler's parts when it is an arrow function with
// at most one identifier parameter and a body that is one value.
func catchHandlerOf(handler *ast.Node) (catchHandler, bool) {
	handler = ast.SkipParentheses(handler)
	if !ast.IsArrowFunction(handler) || ast.HasSyntacticModifier(handler, ast.ModifierFlagsAsync) || handler.TypeParameters() != nil || handler.Type() != nil {
		return catchHandler{}, false
	}
	parameters := handler.Parameters()
	result := catchHandler{}
	switch len(parameters) {
	case 0:
	case 1:
		p := parameters[0]
		if !ast.IsIdentifier(p.Name()) || p.Type() != nil || p.Initializer() != nil || p.AsParameterDeclaration().DotDotDotToken != nil {
			return catchHandler{}, false
		}
		result.parameter = p.Name().Text()
	default:
		return catchHandler{}, false
	}
	body := handler.Body()
	if ast.IsBlock(body) {
		statements := body.Statements()
		if len(statements) != 1 || !ast.IsReturnStatement(statements[0]) || statements[0].Expression() == nil {
			return catchHandler{}, false
		}
		body = statements[0].Expression()
	}
	result.value = body
	return result, true
}

// catchCombinatorRewrite plans EK4835's fix for call.
func catchCombinatorRewrite(c *checker.Checker, file *ast.SourceFile, call *ast.Node) *formRewrite {
	name := catchCombinatorOf(c, call)
	if name == "" {
		return nil
	}
	arguments := call.Arguments()
	self := arguments[0]
	left, runs := runningText(c, file, self)
	if !runs {
		return nil
	}
	if needsOperandParentheses(self) && isInvocation(ast.SkipParentheses(self)) || isHelperCall(self) || isLowPrecedence(self) {
		left = "(" + left + ")"
	}
	var handlers []catchHandler
	var tags []string
	switch name {
	case "catch":
		handler, ok := catchHandlerOf(arguments[1])
		if !ok {
			return nil
		}
		handlers = append(handlers, handler)
	case "catchTag":
		if !ast.IsStringLiteralLike(arguments[1]) {
			return nil
		}
		handler, ok := catchHandlerOf(arguments[2])
		if !ok {
			return nil
		}
		tags, handlers = append(tags, arguments[1].Text()), append(handlers, handler)
	case "catchTags":
		if !ast.IsObjectLiteralExpression(arguments[1]) {
			return nil
		}
		for _, property := range arguments[1].AsObjectLiteralExpression().Properties.Nodes {
			if !ast.IsPropertyAssignment(property) || !ast.IsIdentifier(property.Name()) && !ast.IsStringLiteral(property.Name()) {
				return nil
			}
			handler, ok := catchHandlerOf(property.Initializer())
			if !ok {
				return nil
			}
			tags, handlers = append(tags, property.Name().Text()), append(handlers, handler)
		}
		if len(tags) == 0 {
			return nil
		}
	}
	// The catch variable: the handlers' parameter, or a name no handler reads.
	variable := ""
	for _, h := range handlers {
		switch {
		case h.parameter == "":
		case variable == "":
			variable = h.parameter
		case variable != h.parameter:
			return nil
		}
	}
	if variable == "" {
		variable = freshCatchName(handlers)
	}
	values := make([]string, len(handlers))
	for i, h := range handlers {
		value, _ := runningText(c, file, h.value)
		if strings.HasPrefix(value, "{") || isCommaExpression(h.value) {
			value = "(" + value + ")"
		}
		values[i] = value
	}
	var text string
	if name == "catch" {
		if handlers[0].parameter == "" {
			// A handler that ignores the failure is a fallback (D236).
			fallback := values[0]
			if isLowPrecedence(handlers[0].value) && !strings.HasPrefix(fallback, "(") {
				fallback = "(" + fallback + ")"
			}
			text = left + " catch " + fallback
		} else {
			text = left + " catch (" + variable + ") => " + values[0]
		}
	} else {
		failures := catchFailures(c, call)
		if failures == nil {
			return nil
		}
		var arms []string
		for i, tag := range tags {
			pattern, ok := catchPattern(c, call, failures, tag)
			if !ok {
				return nil
			}
			arms = append(arms, "when "+pattern+": "+values[i]+";")
		}
		arms = append(arms, "default: throw "+variable+";")
		text = left + " catch (" + variable + ") => match (" + variable + ") { " + strings.Join(arms, " ") + " }"
		if len(text) > 100 || strings.ContainsAny(expressionText(file, call), "\r\n") {
			// One arm per line, as a match is written when it does not fit one.
			indent := lineIndent(file.Text(), nodeStart(file, call))
			text = left + " catch (" + variable + ") => match (" + variable + ") {\n" + indent + "  " + strings.Join(arms, "\n"+indent+"  ") + "\n" + indent + "}"
		}
	}
	if catchNeedsParentheses(call) {
		text = "(" + text + ")"
	}
	rewrite := withImportRemovals(file, &formRewrite{
		edits:       []sourceEdit{{nodeStart(file, call), call.End(), text}},
		description: shortText(text, "a catch expression"),
	})
	if name == "catch" {
		rewrite.message = catchHandlerMessage
	}
	return rewrite
}

// freshCatchName returns a name for the catch variable that no handler's
// value contains.
func freshCatchName(handlers []catchHandler) string {
	for i := 1; ; i++ {
		name := "e"
		if i > 1 {
			name = "e" + strconv.Itoa(i)
		}
		if !slices.ContainsFunc(handlers, func(h catchHandler) bool { return containsIdentifier(h.value, name) }) {
			return name
		}
	}
}

// containsIdentifier reports whether node holds an identifier written name.
func containsIdentifier(node *ast.Node, name string) bool {
	if ast.IsIdentifier(node) && node.Text() == name {
		return true
	}
	return node.ForEachChild(func(child *ast.Node) bool { return containsIdentifier(child, name) })
}

// catchFailures returns the expected failures of the program call, a catch
// combinator, catches, one type per member, or nil when one has no literal
// _tag. They are the E of its first parameter as the call instantiates it.
func catchFailures(c *checker.Checker, call *ast.Node) []*checker.Type {
	signature := c.GetResolvedSignature(call)
	if signature == nil || len(signature.Parameters()) == 0 {
		return nil
	}
	t := c.GetTypeAtPosition(signature, 0)
	if !isEffectType(c, t) {
		return nil
	}
	errors := channelType(c, t, "_E")
	if errors == nil {
		return nil
	}
	var failures []*checker.Type
	for _, member := range errors.Distributed() {
		if member.Flags()&checker.TypeFlagsNever != 0 {
			continue
		}
		tag := c.GetTypeOfPropertyOfType(member, "_tag")
		if tag == nil || !tag.IsStringLiteral() {
			return nil
		}
		failures = append(failures, member)
	}
	return failures
}

// catchPattern returns the match pattern that tests a failure for tag: the
// name of the one error class whose instances carry it, or { _tag: "tag" }.
func catchPattern(c *checker.Checker, at *ast.Node, failures []*checker.Type, tag string) (string, bool) {
	var tagged, others []*checker.Type
	for _, failure := range failures {
		if literal := c.GetTypeOfPropertyOfType(failure, "_tag"); literal != nil && literal.IsStringLiteral() && literal.AsLiteralType().Value() == tag {
			tagged = append(tagged, failure)
		} else {
			others = append(others, failure)
		}
	}
	if len(tagged) == 0 {
		return "", false // catchTag of a tag the program cannot fail with
	}
	object := "{ _tag: " + strconv.Quote(tag) + " }"
	symbol := tagged[0].Symbol()
	if symbol == nil || slices.ContainsFunc(tagged[1:], func(t *checker.Type) bool { return t.Symbol() != symbol }) {
		return object, true
	}
	class := symbol.ValueDeclaration()
	if class == nil || !ast.IsClassDeclaration(class) || class.Name() == nil || declaredErrorTag(c, class) != tag {
		return object, true
	}
	for _, other := range others {
		if derivesFrom(c, other, symbol) {
			return object, true
		}
	}
	name := class.Name().Text()
	resolved := c.ResolveName(name, at, ast.SymbolFlagsValue, false)
	if resolved != nil && resolved.Flags()&ast.SymbolFlagsAlias != 0 {
		resolved = c.GetAliasedSymbol(resolved)
	}
	if resolved != symbol {
		return object, true
	}
	return name, true
}

// declaredErrorTag returns the tag that class's declaration gives every
// instance: an error declaration's name, or the literal tag of the
// TaggedError or Schema.TaggedError it extends; or "".
func declaredErrorTag(c *checker.Checker, class *ast.Node) string {
	extends := ast.GetClassExtendsHeritageElement(class)
	if extends == nil || !ast.IsCallExpression(extends.Expression()) {
		return ""
	}
	call := extends.Expression()
	if call.Expression().Flags&ast.NodeFlagsReparsed != 0 && ast.IsIdentifier(call.Expression()) && call.Expression().Text() == "TaggedError" {
		return class.Name().Text() // an error declaration
	}
	parts := errorClassOf(c, ast.GetSourceFileOfNode(class), class)
	if (parts.base == taggedErrorBase || parts.base == schemaErrorBase) && parts.tag != nil && ast.IsStringLiteralLike(parts.tag) {
		return parts.tag.Text()
	}
	return ""
}

// derivesFrom reports whether t is an instance of a class that extends the
// class base declares.
func derivesFrom(c *checker.Checker, t *checker.Type, base *ast.Symbol) bool {
	seen := map[*checker.Type]bool{}
	var walk func(t *checker.Type) bool
	walk = func(t *checker.Type) bool {
		if t == nil || seen[t] || t.Flags()&checker.TypeFlagsObject == 0 {
			return false
		}
		seen[t] = true
		if t.Symbol() == base {
			return true
		}
		for _, b := range c.GetBaseTypes(t) {
			if walk(b) {
				return true
			}
		}
		return false
	}
	return walk(t)
}

// catchNeedsParentheses reports whether a catch expression that replaces call
// needs parentheses: anywhere but a whole expression of its own, since catch
// binds looser than every binary operator and its handler extends as far as
// an arrow function's body does (D237).
func catchNeedsParentheses(call *ast.Node) bool {
	parent := call.Parent
	if isSynthesizedPosition(parent) {
		return true // the left side of another catch expression, or another form the parser wrote
	}
	switch {
	case ast.IsVariableDeclaration(parent) && parent.Initializer() == call,
		ast.IsReturnStatement(parent), ast.IsExpressionStatement(parent),
		ast.IsCallExpression(parent) && parent.Expression() != call && !isHelperCall(parent),
		ast.IsPropertyAssignment(parent) && parent.Initializer() == call,
		ast.IsArrayLiteralExpression(parent), ast.IsTemplateSpan(parent), ast.IsParenthesizedExpression(parent),
		ast.IsArrowFunction(parent) && parent.Body() == call:
		return false
	}
	return true
}
