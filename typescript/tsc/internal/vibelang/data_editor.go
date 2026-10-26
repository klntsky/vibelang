package vibelang

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/ls"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// The editor's view of a data declaration (D307). TypeScript sees the type
// alias and the constructors object the parser writes: a hover on Delivery
// prints the union of the variants' object types, a hover on Delivery.Sent
// the constructor's function type, and go to definition on Delivery.Sent
// finds nothing, since Data.TaggedEnum.Constructor makes each variant's
// constructor with a mapped type. In a .vibe file a hover shows the
// declaration as written,
//
//	data Delivery {
//	  Waiting
//	  Sent { tracking: string }
//	}
//
// or the variant, (variant) Delivery.Sent { tracking: string }, and go to
// definition on a variant goes to it in the declaration. Completions after
// Delivery. are TypeScript's: the variants, $is and $match.

func init() {
	ls.RegisterQuickInfoHook(dataQuickInfo)
	ls.RegisterDefinitionHook(dataVariantDefinition)
}

// dataVariantDefinition goes from Name.Variant to the variant in Name's data
// declaration.
func dataVariantDefinition(c *checker.Checker, node *ast.Node) []*ast.Node {
	if !ast.IsIdentifier(node) || node.Parent == nil || !ast.IsPropertyAccessExpression(node.Parent) || node.Parent.Name() != node {
		return nil
	}
	alias := dataDeclarationOf(c, node.Parent.Expression())
	if variant := dataVariantNamed(alias, node.Text()); variant != nil {
		return []*ast.Node{variant}
	}
	return nil
}

// dataQuickInfo shows a data type, or one of its variants, as its
// declaration writes it, in a hover in a .vibe file.
func dataQuickInfo(c *checker.Checker, node *ast.Node) (string, string) {
	if !ast.IsIdentifier(node) || node.Parent == nil || !isVibeLangFile(ast.GetSourceFileOfNode(node)) {
		return "", ""
	}
	parent := node.Parent
	switch {
	case parent.Kind == ast.KindPropertySignature && parent.Name() == node && parent.Parent != nil && parser.IsDataDeclaration(dataAliasOfVariants(parent.Parent)):
		// The variant's name in its declaration.
		alias := dataAliasOfVariants(parent.Parent)
		return "(variant) " + alias.Name().Text() + "." + variantText(parent), ""
	case ast.IsPropertyAccessExpression(parent) && parent.Name() == node:
		if alias := dataDeclarationOf(c, parent.Expression()); alias != nil {
			if variant := dataVariantNamed(alias, node.Text()); variant != nil {
				return "(variant) " + scanner.GetTextOfNode(parent.Expression()) + "." + variantText(variant), ""
			}
		}
		return "", ""
	}
	if alias := dataDeclarationOf(c, node); alias != nil {
		return dataDeclarationText(alias), ""
	}
	return "", ""
}

// dataAliasOfVariants returns the data declaration's type alias whose type
// literal of variants is literal, or nil.
func dataAliasOfVariants(literal *ast.Node) *ast.Node {
	if literal.Kind != ast.KindTypeLiteral || literal.Parent == nil || literal.Parent.Kind != ast.KindTypeReference || literal.Parent.Parent == nil {
		return nil
	}
	if alias := literal.Parent.Parent; parser.IsDataDeclaration(alias) {
		return alias
	}
	return nil
}

// dataDeclarationText writes alias, a data declaration's type alias, as the
// declaration: data Name<T> { Variant; Variant { field: Type } }, one
// variant to a line.
func dataDeclarationText(alias *ast.Node) string {
	var b strings.Builder
	b.WriteString("data ")
	b.WriteString(alias.Name().Text())
	if list := alias.TypeParameterList(); list != nil {
		parameters := make([]string, len(list.Nodes))
		for i, parameter := range list.Nodes {
			parameters[i] = collapseSpaces(scanner.GetTextOfNode(parameter))
		}
		b.WriteString("<" + strings.Join(parameters, ", ") + ">")
	}
	b.WriteString(" {")
	for _, variant := range parser.DataVariants(alias) {
		b.WriteString("\n  " + variantText(variant))
	}
	b.WriteString("\n}")
	return b.String()
}

// variantText writes variant, a data declaration's variant, as its
// declaration does: Sent { tracking: string }, or Waiting.
func variantText(variant *ast.Node) string {
	text := variant.Name().Text()
	fields := variant.Type()
	if fields == nil || fields.Kind != ast.KindTypeLiteral || len(fields.Members()) == 0 {
		return text
	}
	parts := make([]string, 0, len(fields.Members()))
	for _, field := range fields.Members() {
		part := strings.TrimRight(collapseSpaces(scanner.GetTextOfNode(field)), ";, ")
		parts = append(parts, part)
	}
	return text + " { " + strings.Join(parts, "; ") + " }"
}

// collapseSpaces writes each run of white space in text as one space.
func collapseSpaces(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
