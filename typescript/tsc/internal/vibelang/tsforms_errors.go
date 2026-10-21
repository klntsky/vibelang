package vibelang

import (
	"slices"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// EK4830 (D284): an error class written the way Effect's TypeScript writes
// one, where an error declaration (D126) says the same:
//
//	class NotFound extends Data.TaggedError("NotFound")<{ readonly id: string }> {}
//	error NotFound { id: string }
//
// The fix applies when the class extends Data.TaggedError, or the prelude's
// TaggedError, with a string literal tag equal to its name, its fields are an
// object type written in place, and its body is empty. Otherwise a class that
// an error declaration could still replace, one with an empty body, draws the
// warning without a fix, and its note says what a declaration would change:
// a tag other than the name, or the _tag that Data.Error lacks. A
// Schema.TaggedError class draws EK4837 instead, also without a fix, since a
// declaration would drop its Schema; D279's schema error writes one, and a
// project whose errors are Schemas by design turns the code off until the
// parser accepts it. A class with members, a constructor, decorators, or
// another base draws nothing until the parser accepts D278's error
// declarations with class members.

func init() {
	tsFormRules[EK4830.Code] = &tsFormRule{
		message: EK4830,
		find:    findErrorClasses,
		plan: func(c *checker.Checker, file *ast.SourceFile, at *ast.Node) *formRewrite {
			return errorClassRewrite(c, file, at.Parent)
		},
		diagnosed: isClassName,
	}
}

// isClassName reports whether node is the name of a class declaration.
func isClassName(node *ast.Node) bool {
	return ast.IsIdentifier(node) && node.Parent != nil && ast.IsClassDeclaration(node.Parent) && node.Parent.Name() == node
}

// errorBase is what an error class extends.
type errorBase uint8

const (
	notErrorBase    errorBase = iota
	taggedErrorBase           // Data.TaggedError("tag") or TaggedError("tag")
	dataErrorBase             // Data.Error
	schemaErrorBase           // Schema.TaggedError<Self>()("tag", { … })
)

// errorClass is an error class's parts.
type errorClass struct {
	base     errorBase
	baseText string    // the base as written, for the message
	tag      *ast.Node // the tag argument, or nil
	fields   *ast.Node // the fields' type argument, or nil
}

// errorClassOf returns the parts of class when it extends an Effect error
// base, or base notErrorBase.
func errorClassOf(c *checker.Checker, file *ast.SourceFile, class *ast.Node) errorClass {
	extends := ast.GetClassExtendsHeritageElement(class)
	if extends == nil {
		return errorClass{}
	}
	expression := extends.Expression()
	result := errorClass{}
	switch {
	case ast.IsCallExpression(expression):
		callee := expression.Expression()
		if callee.Flags&ast.NodeFlagsReparsed != 0 {
			return errorClass{} // an error declaration (D126)
		}
		arguments := expression.Arguments()
		switch {
		case isEffectFunction(c, callee, "Data", "TaggedError") || isRuntimeExport(c, callee, "TaggedError"):
			if len(arguments) != 1 {
				return errorClass{}
			}
			result = errorClass{base: taggedErrorBase, tag: arguments[0]}
			if arguments := extends.TypeArguments(); len(arguments) == 1 {
				result.fields = arguments[0]
			} else if len(arguments) > 1 {
				return errorClass{}
			}
		case ast.IsCallExpression(callee) && isEffectFunction(c, callee.Expression(), "Schema", "TaggedError"):
			if len(arguments) < 1 {
				return errorClass{}
			}
			result = errorClass{base: schemaErrorBase, tag: arguments[0]}
		default:
			return errorClass{}
		}
		result.baseText = expressionText(file, callee)
		if result.base == schemaErrorBase {
			result.baseText = expressionText(file, callee.Expression())
		}
	case isEffectFunction(c, expression, "Data", "Error"):
		result = errorClass{base: dataErrorBase, baseText: expressionText(file, expression)}
		if arguments := extends.TypeArguments(); len(arguments) == 1 {
			result.fields = arguments[0]
		} else if len(arguments) > 1 {
			return errorClass{}
		}
	default:
		return errorClass{}
	}
	return result
}

// errorClassShape reports whether class could be an error declaration apart
// from its base: a named class declaration, exported or not, with no other
// modifiers, no implements clause, and an empty body.
func errorClassShape(file *ast.SourceFile, class *ast.Node) bool {
	if class.Name() == nil || len(ast.GetImplementsHeritageClauseElements(class)) != 0 {
		return false
	}
	for _, modifier := range class.ModifierNodes() {
		if modifier.Kind != ast.KindExportKeyword {
			return false
		}
	}
	if slices.ContainsFunc(class.Members(), func(m *ast.Node) bool { return !ast.IsSemicolonClassElement(m) }) {
		return false
	}
	extends := ast.GetClassExtendsHeritageElement(class)
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(file.Text()[extends.End():class.End()]), "{}")) == ""
}

// findErrorClasses reports EK4830 at each error class of file that an error
// declaration could replace.
func findErrorClasses(c *checker.Checker, file *ast.SourceFile, report func(at *ast.Node, rewrite *formRewrite, args ...any)) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if ast.IsClassDeclaration(node) && !inHatch(node) {
			parts := errorClassOf(c, file, node)
			if parts.base != notErrorBase && errorClassShape(file, node) {
				rewrite := errorClassRewrite(c, file, node)
				if rewrite != nil && rewrite.message == EK4837 {
					report(node.Name(), rewrite, node.Name().Text())
				} else {
					report(node.Name(), rewrite, node.Name().Text(), parts.baseText)
				}
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// errorClassRewrite plans EK4830's fix for class.
func errorClassRewrite(c *checker.Checker, file *ast.SourceFile, class *ast.Node) *formRewrite {
	parts := errorClassOf(c, file, class)
	name := class.Name().Text()
	tag, literal := "", false
	if parts.tag != nil && ast.IsStringLiteralLike(parts.tag) {
		tag, literal = parts.tag.Text(), true
	}
	silence := " Keep the class if something depends on that, and silence this line with // @vibelang-diagnostics-next-line EK4830:off."
	switch {
	case parts.base == schemaErrorBase:
		return &formRewrite{message: EK4837, why: "No quick fix: an error declaration is not a Schema, so decoding, encoding, and the checks at construction would change" + core.IfElse(literal && tag != name, ", and its tag would be "+name+", not \""+tag+"\"", "") + ". Where the Schema is needed, keep the class; a project whose errors are Schemas by design turns EK4837 off."}
	case parts.base == dataErrorBase:
		return &formRewrite{why: "No quick fix: an error declaration adds a _tag, \"" + name + "\", which Data.Error has none of." + silence}
	case !literal:
		return &formRewrite{why: "No quick fix: the tag is not a string literal, and an error declaration's tag is its name."}
	case tag != name:
		return &formRewrite{why: "No quick fix: an error declaration's tag is its name, so the tag would become \"" + name + "\" instead of \"" + tag + "\"." + silence}
	case parts.fields != nil && !ast.IsTypeLiteralNode(parts.fields):
		return &formRewrite{why: "No quick fix: the fields come from " + expressionText(file, parts.fields) + "; write them in the declaration."}
	}
	text := file.Text()
	start := nodeStart(file, class)
	if modifiers := class.Modifiers(); modifiers != nil && len(modifiers.Nodes) > 0 {
		start = scanner.SkipTrivia(text, modifiers.End())
	}
	if !strings.HasPrefix(text[start:], "class") {
		return nil
	}
	declaration := "error " + name
	if class.TypeParameters() != nil {
		extendsKeyword := strings.Index(text[class.Name().End():], "extends")
		if extendsKeyword < 0 {
			return nil
		}
		declaration += strings.TrimSpace(text[class.Name().End() : class.Name().End()+extendsKeyword])
	}
	if parts.fields != nil {
		if fields := errorFieldsText(file, parts.fields); strings.Trim(fields, "{} \t\r\n") != "" {
			declaration += " " + fields
		}
	}
	return withImportRemovals(file, &formRewrite{
		edits:       []sourceEdit{{start, class.End(), declaration}},
		description: shortText(declaration, "error "+name+" { … }"),
	})
}

// errorFieldsText returns literal, an object type, as an error declaration's
// fields: its text without the readonly modifiers, since every field of an
// error is readonly (D134).
func errorFieldsText(file *ast.SourceFile, literal *ast.Node) string {
	text := file.Text()
	start := nodeStart(file, literal)
	var removals []sourceEdit
	for _, member := range literal.AsTypeLiteralNode().Members.Nodes {
		for _, modifier := range member.ModifierNodes() {
			if modifier.Kind == ast.KindReadonlyKeyword && modifier.Flags&ast.NodeFlagsReparsed == 0 {
				from := nodeStart(file, modifier)
				removals = append(removals, sourceEdit{from, scanner.SkipTrivia(text, modifier.End()), ""})
			}
		}
	}
	out := text[start:literal.End()]
	for i := len(removals) - 1; i >= 0; i-- {
		r := removals[i]
		out = out[:r.start-start] + out[r.end-start:]
	}
	return out
}
