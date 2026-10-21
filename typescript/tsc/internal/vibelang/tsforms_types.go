package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// EK4833 (D287): an Effect type written with its type arguments, which the !
// form (D127) writes in fewer tokens:
//
//	Effect.Effect<User, NotFound, Db>   User!NotFound!Db
//	Effect<User>                        User!!
//
// Both Effect.Effect and the prelude's Effect<A, E, R> count. The parser
// expands the ! form into Effect.Effect<A, E, R>, so the rewrite is exact,
// with one rule about the dependencies: in the return annotation of a
// function with a body, a ! form that leaves them out infers them (D127), so
// there Effect<A, E, never> becomes A!E! and Effect<A, E, infer R> becomes
// A!E; elsewhere a left-out slot is never, so Effect<A, E> becomes A!E.
// Refused: a return annotation with a body that writes fewer than three
// arguments (EK2003 already asks for them), infer in another slot or outside
// such an annotation, and a reference the ! form cannot spell, such as one
// in JSDoc.

func init() {
	tsFormRules[EK4833.Code] = &tsFormRule{
		message:   EK4833,
		find:      findEffectTypes,
		plan:      effectTypeRewrite,
		diagnosed: ast.IsTypeReferenceNode,
	}
}

// isEffectTypeName reports whether reference, a type reference, names
// effect's Effect interface, as Effect.Effect or as the prelude's Effect type,
// in source the user wrote.
func isEffectTypeName(c *checker.Checker, reference *ast.Node) bool {
	name := reference.AsTypeReferenceNode().TypeName
	if name.Flags&ast.NodeFlagsReparsed != 0 || reference.Flags&ast.NodeFlagsJSDoc != 0 {
		return false // an A!E!R the parser expanded
	}
	if rightmostName(name).Text() != "Effect" {
		return false
	}
	// Effect.Effect, the prelude's Effect, or Effect imported from effect:
	// each names effect's own Effect interface.
	return isEffectType(c, c.GetTypeFromTypeNode(reference))
}

// findEffectTypes reports EK4833 at each Effect type of file that the ! form
// can write.
func findEffectTypes(c *checker.Checker, file *ast.SourceFile, report func(at *ast.Node, rewrite *formRewrite, args ...any)) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if ast.IsTypeReferenceNode(node) && isEffectTypeName(c, node) {
			if rewrite := effectTypeRewrite(c, file, node); rewrite.fixable() {
				report(node, rewrite, messageText(node, "This Effect type"), rewrite.description)
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// isBodyReturnAnnotation reports whether node is the return annotation of a
// function with a body, where a ! form without dependencies infers them.
func isBodyReturnAnnotation(node *ast.Node) bool {
	fn := node.Parent
	return fn != nil && ast.IsFunctionLike(fn) && fn.Type() == node && fn.Body() != nil
}

// effectTypeRewrite plans EK4833's fix for reference.
func effectTypeRewrite(c *checker.Checker, file *ast.SourceFile, reference *ast.Node) *formRewrite {
	if !isEffectTypeName(c, reference) {
		return nil
	}
	arguments := reference.TypeArguments()
	if len(arguments) == 0 || len(arguments) > 3 {
		return nil
	}
	bodyReturn := isBodyReturnAnnotation(reference)
	if bodyReturn && len(arguments) != 3 {
		return nil // EK2003 asks whether the missing channels are never or inferred
	}
	for i, argument := range arguments {
		if ast.IsInferTypeNode(argument) && !(bodyReturn && i == 2) {
			return nil
		}
	}
	slot := func(i int) string {
		if i >= len(arguments) || arguments[i].Kind == ast.KindNeverKeyword {
			return ""
		}
		return bangSlot(expressionText(file, arguments[i]), i > 0)
	}
	success, errors, dependencies := slot(0), slot(1), slot(2)
	if success == "" {
		success = "never"
	}
	var text string
	switch {
	case bodyReturn && ast.IsInferTypeNode(arguments[2]):
		text = success + "!" + errors
		if errors == "" {
			// A!! would promise no dependencies; write never for the errors.
			text = success + "!never"
		}
	case bodyReturn || dependencies != "":
		text = success + "!" + errors + "!" + dependencies
	default:
		text = success + "!" + errors
		if errors == "" {
			text = success + "!!"
		}
	}
	if needsBangParentheses(reference) {
		text = "(" + text + ")"
	}
	return &formRewrite{edits: []sourceEdit{{nodeStart(file, reference), reference.End(), text}}, description: text}
}

// needsBangParentheses reports whether an Effect type in the ! form needs
// parentheses where reference stands: as a member of a union or an
// intersection, which would join its first slot; before [] or an index; as
// the operand of keyof or readonly; in a conditional type; or in a slot of
// another ! form.
func needsBangParentheses(reference *ast.Node) bool {
	parent := reference.Parent
	if parent == nil {
		return false
	}
	switch parent.Kind {
	case ast.KindUnionType, ast.KindIntersectionType, ast.KindArrayType, ast.KindIndexedAccessType, ast.KindTypeOperator,
		ast.KindConditionalType, ast.KindOptionalType, ast.KindRestType:
		return true
	case ast.KindTypeReference:
		return parent.AsTypeReferenceNode().TypeName.Flags&ast.NodeFlagsReparsed != 0
	}
	return false
}
