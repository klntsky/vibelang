package vibelang

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// EK4836 (D290): combinators that wrap a program, which VibeLang writes as a
// block around the statements:
//
//	return Effect.withSpan(load(id), "load")          return withSpan("load") { return load(id) }
//	return Effect.provide(main(), AppLive)            provide AppLive { return main() }
//	return Effect.provideService(roll(), Random, r)   provide Random = r { return roll() }
//
// Effect.withSpan(self, name) becomes the block param call withSpan(name)
// { return self } (D188, D151) wherever it stands in a VibeLang function body;
// the block's return gives the program's value, so the span covers the same
// program. Effect.provide(self, layers) and Effect.provideService(self, key,
// value) become a provide block (D130, D172, D173) where a block can stand:
// as a return, whose return inside the block returns from the function, and
// as an expression statement. A layer array becomes the block's comma list.
// The program runs inside the block as the call ran it: a call runs where it
// stands, and an Effect value is called. Refused: options arguments, the
// data-last forms inside pipe, a provide or provideService anywhere a block
// cannot stand, a key that is not a name or a dotted name, a file where a
// local withSpan shadows the prelude's, and anything outside a VibeLang
// function body.

func init() {
	tsFormRules[EK4836.Code] = &tsFormRule{
		message:   EK4836,
		find:      findBlockForms,
		plan:      blockFormRewrite,
		diagnosed: ast.IsCallExpression,
	}
}

// blockFormOf returns the combinator call calls in its data-first form, or "".
func blockFormOf(c *checker.Checker, call *ast.Node) string {
	if !ast.IsCallExpression(call) || call.TypeArguments() != nil || !inLowerableBody(call) || inHatch(call) || ast.IsOptionalChain(call) {
		return ""
	}
	for _, argument := range call.Arguments() {
		if ast.IsSpreadElement(argument) {
			return ""
		}
	}
	if !isEffectFunction(c, call.Expression(), "Effect", "withSpan", "provide", "provideService") {
		return ""
	}
	_, name := effectMember(c, call.Expression())
	switch {
	case name == "withSpan" && len(call.Arguments()) == 2, name == "provide" && len(call.Arguments()) == 2, name == "provideService" && len(call.Arguments()) == 3:
		return name
	}
	return ""
}

// findBlockForms reports EK4836 at each such call of file that a block form
// writes.
func findBlockForms(c *checker.Checker, file *ast.SourceFile, report func(at *ast.Node, rewrite *formRewrite, args ...any)) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if name := blockFormOf(c, node); name != "" {
			if rewrite := blockFormRewrite(c, file, node); rewrite.fixable() {
				form := "withSpan(name) { … }"
				switch name {
				case "provide":
					form = "provide Layer { … }"
				case "provideService":
					form = "provide Key = value { … }"
				}
				report(node, rewrite, "Effect."+name, form)
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// blockFormRewrite plans EK4836's fix for call.
func blockFormRewrite(c *checker.Checker, file *ast.SourceFile, call *ast.Node) *formRewrite {
	name := blockFormOf(c, call)
	if name == "" {
		return nil
	}
	arguments := call.Arguments()
	program, runs := runningText(c, file, arguments[0])
	if !runs {
		return nil
	}
	text := file.Text()
	start, end := nodeStart(file, call), call.End()
	indent := lineIndent(text, start)
	var result string
	switch name {
	case "withSpan":
		if shadowed(call, "withSpan") {
			return nil
		}
		head := "withSpan(" + expressionText(file, arguments[1]) + ")"
		if ast.IsExpressionStatement(call.Parent) {
			result = head + blockText(program, indent, false)
		} else {
			result = head + blockText(program, indent, true)
		}
	case "provide", "provideService":
		statement := call.Parent
		returns := ast.IsReturnStatement(statement)
		if !returns && !ast.IsExpressionStatement(statement) {
			return nil
		}
		if !analyzerFor(c).runs(call) {
			return nil
		}
		var providers []string
		if name == "provide" {
			layers := arguments[1]
			elements := []*ast.Node{layers}
			if ast.IsArrayLiteralExpression(layers) {
				elements = layers.Elements()
			}
			if len(elements) == 0 {
				return nil
			}
			for _, element := range elements {
				if ast.IsSpreadElement(element) || ast.IsOmittedExpression(element) {
					return nil
				}
				providers = append(providers, providerText(file, element))
			}
		} else {
			key := arguments[1]
			if !ast.IsEntityNameExpression(key) {
				return nil
			}
			providers = []string{expressionText(file, key) + " = " + providerText(file, arguments[2])}
		}
		start, end = nodeStart(file, statement), statement.End()
		result = "provide " + strings.Join(providers, ", ") + blockText(program, indent, returns)
	}
	return withImportRemovals(file, &formRewrite{edits: []sourceEdit{{start, end, result}}, description: shortText(result, "the block form")})
}

// blockText returns a block that runs program, returning its value when
// returns is set: on one line when program is, else on lines of its own at
// indent's next level.
func blockText(program, indent string, returns bool) string {
	statement := program
	if returns {
		statement = "return " + program
	}
	if !strings.ContainsAny(program, "\r\n") && len(statement) < 60 {
		return " { " + statement + " }"
	}
	return " {\n" + indent + "  " + statement + "\n" + indent + "}"
}

// providerText returns node's text as one provider of a provide block, in
// parentheses when a comma or a conditional would split it.
func providerText(file *ast.SourceFile, node *ast.Node) string {
	text := expressionText(file, node)
	if isLowPrecedence(node) {
		return "(" + text + ")"
	}
	return text
}
