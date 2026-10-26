package vibelang

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// EK4831 (D285): a Context.Service class, which a service declaration
// (D128, D223) writes without the key, the Self parameter, and the shape's
// separate type:
//
//	class Users extends Context.Service<Users, { find(id: string): Effect.Effect<User, NotFound> }>()("app/Users") {}
//	service Users { find(id: string): Effect<User, NotFound> }
//
// The fix applies when the shape is an object type written in place whose
// members a service declares: properties, which become value members, and
// methods whose return type is an Effect type, which become operations with
// that type, written Effect<…> as EK4833's fix writes it (D308), since a
// member without a body must write its contract (EK2101). The class must be named, exported or not, without
// type parameters, an implements clause, members of its own, or the make
// option. The declaration's key is derived from the package, the module,
// and the name (D93), so the written key string is gone: lookups through the
// class are unchanged, because providing and requiring both use the class,
// but the key appears in "Service not found" messages, and a Context.Service
// elsewhere with the same string would no longer share the slot. The note
// says what the key becomes. Effect's own deterministicKeys rule rewrites
// keys the same way. A class whose shape is a named type draws the warning
// without a fix: the declaration needs the members written out. A shape that
// is not an object type, such as Context.Service<Port, number>, draws
// nothing. D281's service Name: Type writes both, once the parser accepts it.

func init() {
	tsFormRules[EK4831.Code] = &tsFormRule{
		message: EK4831,
		find:    findServiceClasses,
		plan: func(c *checker.Checker, file *ast.SourceFile, at *ast.Node) *formRewrite {
			return serviceClassRewrite(c, file, at.Parent)
		},
		diagnosed: isClassName,
	}
}

// serviceClassOf returns the key argument and the shape type argument of
// class when it extends Context.Service<Self, Shape>()(key) with Self the
// class itself, or nil and nil.
func serviceClassOf(c *checker.Checker, class *ast.Node) (key *ast.Node, shape *ast.Node) {
	extends := ast.GetClassExtendsHeritageElement(class)
	if extends == nil || class.Name() == nil {
		return nil, nil
	}
	outer := extends.Expression()
	if !ast.IsCallExpression(outer) || len(outer.Arguments()) != 1 {
		return nil, nil
	}
	inner := outer.Expression()
	if !ast.IsCallExpression(inner) || len(inner.Arguments()) != 0 || !isEffectFunction(c, inner.Expression(), "Context", "Service") {
		return nil, nil
	}
	arguments := inner.TypeArguments()
	if len(arguments) != 2 || !ast.IsTypeReferenceNode(arguments[0]) || arguments[0].TypeArguments() != nil {
		return nil, nil
	}
	if self := arguments[0].AsTypeReferenceNode().TypeName; !ast.IsIdentifier(self) || self.Text() != class.Name().Text() {
		return nil, nil
	}
	return outer.Arguments()[0], arguments[1]
}

// findServiceClasses reports EK4831 at each Context.Service class of file
// that a service declaration could write.
func findServiceClasses(c *checker.Checker, file *ast.SourceFile, report func(at *ast.Node, rewrite *formRewrite, args ...any)) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if ast.IsClassDeclaration(node) && !inHatch(node) {
			if rewrite := serviceClassRewrite(c, file, node); rewrite != nil {
				report(node.Name(), rewrite, node.Name().Text())
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// serviceClassRewrite plans EK4831's fix for class. It returns nil when class
// is not a Context.Service class that a service declaration could write, and
// a plan without edits when the declaration needs a change by hand.
func serviceClassRewrite(c *checker.Checker, file *ast.SourceFile, class *ast.Node) *formRewrite {
	key, shape := serviceClassOf(c, class)
	if shape == nil || class.TypeParameters() != nil || len(ast.GetImplementsHeritageClauseElements(class)) != 0 {
		return nil
	}
	for _, modifier := range class.ModifierNodes() {
		if modifier.Kind != ast.KindExportKeyword {
			return nil
		}
	}
	statics := false
	for _, member := range class.Members() {
		switch {
		case ast.IsSemicolonClassElement(member):
		case ast.IsStatic(member):
			statics = true
		default:
			return nil // an instance member: no service declares one
		}
	}
	shapeType := c.GetTypeFromTypeNode(shape)
	if shapeType == nil || shapeType.Flags()&checker.TypeFlagsObject == 0 || len(c.GetSignaturesOfType(shapeType, checker.SignatureKindCall)) != 0 || len(c.GetPropertiesOfType(shapeType)) == 0 {
		return nil // a service declares members
	}
	name := class.Name().Text()
	keyNote := ""
	if ast.IsStringLiteralLike(key) {
		keyNote = " Its key would become \"" + servicesFor(c).key(class) + "\" instead of \"" + key.Text() + "\"."
	}
	if !ast.IsTypeLiteralNode(shape) {
		return &formRewrite{why: "No quick fix: its members come from " + expressionText(file, shape) + "; write them in the declaration." + keyNote}
	}
	if statics {
		return &formRewrite{why: "No quick fix: move its static members into the declaration." + keyNote}
	}
	body, why := serviceBodyText(c, file, shape)
	if why != "" {
		return &formRewrite{why: "No quick fix: " + why + "." + keyNote}
	}
	text := file.Text()
	start := nodeStart(file, class)
	if modifiers := class.Modifiers(); modifiers != nil && len(modifiers.Nodes) > 0 {
		start = scanner.SkipTrivia(text, modifiers.End())
	}
	if !strings.HasPrefix(text[start:], "class") {
		return nil
	}
	declaration := "service " + name + " " + body
	rewrite := withImportRemovals(file, &formRewrite{
		edits:       []sourceEdit{{start, class.End(), declaration}},
		description: shortText(declaration, "service "+name+" { … }"),
	})
	rewrite.why = strings.TrimSpace(keyNote)
	return rewrite
}

// serviceBodyText returns literal, a Context.Service shape, as a service
// declaration's body, or the reason it cannot be one.
func serviceBodyText(c *checker.Checker, file *ast.SourceFile, literal *ast.Node) (string, string) {
	text := file.Text()
	start := nodeStart(file, literal)
	var edits []sourceEdit
	for _, member := range literal.AsTypeLiteralNode().Members.Nodes {
		name := member.Name()
		if name == nil || !ast.IsIdentifier(name) && !ast.IsStringLiteral(name) {
			return "", "a service declares named members only"
		}
		if postfix := member.PostfixToken(); postfix != nil && postfix.Kind == ast.KindQuestionToken {
			return "", name.Text() + " is optional, which a service member cannot be"
		}
		switch {
		case ast.IsPropertySignatureDeclaration(member):
			if member.Type() == nil {
				return "", name.Text() + " has no type"
			}
			for _, modifier := range member.ModifierNodes() {
				if modifier.Kind == ast.KindReadonlyKeyword && modifier.Flags&ast.NodeFlagsReparsed == 0 {
					edits = append(edits, sourceEdit{nodeStart(file, modifier), scanner.SkipTrivia(text, modifier.End()), ""})
				}
			}
		case ast.IsMethodSignatureDeclaration(member):
			returnType := member.Type()
			if returnType == nil {
				return "", name.Text() + " has no return type"
			}
			if !isEffectType(c, c.GetTypeFromTypeNode(returnType)) {
				return "", name.Text() + " returns " + expressionText(file, returnType) + ", but an operation of a service returns an Effect type"
			}
			if contract := effectTypeRewrite(c, file, returnType); ast.IsTypeReferenceNode(returnType) && contract.fixable() {
				edits = append(edits, contract.edits...)
			}
		default:
			return "", "a service declares properties and methods only"
		}
		if end := member.End(); end > 0 && text[end-1] == ',' {
			// A class body separates members with semicolons or lines.
			rest := text[end:]
			if newline := strings.IndexAny(rest, "\r\n}"); newline >= 0 && strings.TrimSpace(rest[:newline]) == "" && rest[newline] != '}' {
				edits = append(edits, sourceEdit{end - 1, end, ""})
			} else {
				edits = append(edits, sourceEdit{end - 1, end, ";"})
			}
		}
	}
	out := text[start:literal.End()]
	for i := len(edits) - 1; i >= 0; i-- {
		e := edits[i]
		out = out[:e.start-start] + e.text + out[e.end-start:]
	}
	return out, ""
}
