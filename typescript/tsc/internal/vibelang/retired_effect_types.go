package vibelang

import (
	"context"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/ls"
)

// The ! form of an Effect type, Result!Error!Dependencies, is retired (D308,
// which removes D127's form). An Effect type is written Effect<A, E, R>, as
// in TypeScript. The parser still reads the ! form, as the Effect type it
// stood for, so a file that has one still checks; each one is EK1014, whose
// quick fix writes that Effect type, and vibelang --fix rewrites a whole
// project:
//
//	User!NotFound!Db               Effect<User, NotFound, Db>
//	User!NotFound!                 Effect<User, NotFound>
//	User!!Db                       Effect<User, never, Db>
//	User!!                         Effect<User>
//	User!NotFound, with a body     Effect<User, NotFound, infer R>
//	User!NotFound, anywhere else   Effect<User, NotFound>
//
// "With a body" is the return annotation of a function whose body follows,
// where the ! form inferred the dependencies it left out. Parentheses that
// the ! form needed, around a slot or around the whole form, go away, since
// Effect<…> needs none. In a file whose import binds Effect to the Effect
// namespace alone, as import { Effect } from "effect" does, the fix writes
// Effect.Effect<…>, the type the parser read (effectTypeName).

// EK1014: an Effect type written in the retired ! form. {0} is the form as written, {1} the Effect type it stands for.
var EK1014 = message(1014, errorCategory, "{0} is the retired ! form of an Effect type; write {1}.")

func init() {
	registry[EK1014.Code] = EK1014
	fileChecks = append(fileChecks, checkRetiredEffectTypes)
	ls.RegisterCodeFixProvider(retiredEffectTypeFix)
}

// isRetiredEffectType reports whether node is an Effect type written in the !
// form, which the parser reads as Effect.Effect<A, E, R> with a name it
// synthesized around the slots the source wrote. Other forms synthesize
// qualified names too, such as a data declaration's Data.TaggedEnum, but
// the whole reference with them.
func isRetiredEffectType(node *ast.Node) bool {
	if !ast.IsTypeReferenceNode(node) || node.Flags&ast.NodeFlagsReparsed != 0 || len(node.TypeArguments()) != 3 {
		return false
	}
	name := node.AsTypeReferenceNode().TypeName
	if !ast.IsQualifiedName(name) || name.Flags&ast.NodeFlagsReparsed == 0 {
		return false
	}
	qualified := name.AsQualifiedName()
	return ast.IsIdentifier(qualified.Left) && qualified.Left.Text() == "Effect" && qualified.Right.Text() == "Effect"
}

// checkRetiredEffectTypes reports EK1014 at each Effect type of file written
// in the ! form, nested ones included.
func checkRetiredEffectTypes(c *checker.Checker, file *ast.SourceFile) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if isRetiredEffectType(node) {
			written := file.Text()[nodeStart(file, node):node.End()]
			d := report(c, node, EK1014, shortText(written, "This type"), retiredEffectTypeText(file, node, effectTypeName(c, node)))
			note(d, "vibelang -p tsconfig.json --fix rewrites every one in the project.")
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// retiredEffectTypeText writes reference, an Effect type in the ! form, as
// the Effect type it stands for, under name. The parser wrote a slot left
// empty as a synthesized never, and dependencies left out of a function's
// contract as a synthesized infer R (parser/vibelang.go).
func retiredEffectTypeText(file *ast.SourceFile, reference *ast.Node, name string) string {
	arguments := reference.TypeArguments()
	channels := make([]string, len(arguments))
	for i, argument := range arguments {
		switch {
		case argument.Flags&ast.NodeFlagsReparsed == 0:
			channels[i] = retiredSlotText(file, argument, name)
		case ast.IsInferTypeNode(argument):
			channels[i] = "infer R"
		default:
			channels[i] = "never"
		}
	}
	return namedEffectText(name, channels...)
}

// retiredSlotText returns the text of slot, a type the ! form held, without
// the parentheses the form needed around it, and with each ! form inside it
// written as an Effect type.
func retiredSlotText(file *ast.SourceFile, slot *ast.Node, name string) string {
	for ast.IsParenthesizedTypeNode(slot) {
		slot = slot.AsParenthesizedTypeNode().Type
	}
	if isRetiredEffectType(slot) {
		return retiredEffectTypeText(file, slot, name)
	}
	start, text := nodeStart(file, slot), file.Text()
	var out strings.Builder
	at := start
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if isRetiredEffectType(node) {
			span := retiredEffectTypeSpan(file, node)
			if span.Pos() >= at && span.End() <= slot.End() {
				out.WriteString(text[at:span.Pos()])
				out.WriteString(retiredEffectTypeText(file, node, name))
				at = span.End()
			}
			return false
		}
		return node.ForEachChild(visit)
	}
	slot.ForEachChild(visit)
	out.WriteString(text[at:slot.End()])
	return out.String()
}

// retiredEffectTypeSpan returns the text that EK1014's fix replaces for
// reference: the ! form, with the parentheses around it that hold nothing
// else.
func retiredEffectTypeSpan(file *ast.SourceFile, reference *ast.Node) core.TextRange {
	node := reference
	for node.Parent != nil && ast.IsParenthesizedTypeNode(node.Parent) {
		node = node.Parent
	}
	return core.NewTextRange(nodeStart(file, node), node.End())
}

// retiredEffectTypeFix is EK1014's quick fix: it writes the Effect type that
// a ! form stands for.
var retiredEffectTypeFix = &ls.CodeFixProvider{
	Source:     diagnosticSource,
	ErrorCodes: []int32{EK1014.Code},
	GetCodeActions: func(ctx context.Context, fix *ls.CodeFixContext) ([]*ls.CodeAction, error) {
		reference := reportedNode(fix.SourceFile, fix.Span, isRetiredEffectType)
		if reference == nil {
			return nil, nil
		}
		c, done := fix.Program.GetTypeCheckerForFile(ctx, fix.SourceFile)
		defer done()
		text := retiredEffectTypeText(fix.SourceFile, reference, effectTypeName(c, reference))
		span := retiredEffectTypeSpan(fix.SourceFile, reference)
		return []*ls.CodeAction{{
			Description: "Write " + text,
			Changes:     replaceRange(fix, span.Pos(), span.End(), text),
		}}, nil
	},
}
