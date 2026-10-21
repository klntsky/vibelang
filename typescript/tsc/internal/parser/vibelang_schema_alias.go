package parser

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// The alias form of a schema declaration (D280).
//
//	schema Code = "root_missing" | "read_failed"
//
// parses as the two declarations it abbreviates:
//
//	const Code = Schema.Literals(["root_missing", "read_failed"])
//	type Code = typeof Code.Type
//
// The type becomes its schema with D133's mapping (vibelangSchemaOf), so any
// type a struct field may have may follow the =: a union of literals or of
// other forms, an array, a tuple, a record, an object type, or the name of
// another schema. Any other type becomes Schema.Unknown flagged
// NodeFlagsVibeLang, which the checker reports as EK2109 (schemas.go). The
// variable declaration is flagged NodeFlagsVibeLang too, which is how the
// checker, the ESTree converter, and the projection tell the alias form from
// a const written out. The type alias goes to p.reparseList, as the struct
// form's interface does.

// VibeSyntaxSchemaAlias is the Name of the VibeSyntaxDeclaration recorded for
// the alias form: Head runs from schema through the name, and Open and Close
// are empty.
const VibeSyntaxSchemaAlias = "schema alias"

// scanSchemaAliasStart reports whether the current token, schema, starts the
// alias form: schema, a name, and =. It moves the scanner; call it through
// lookAhead.
func (p *Parser) scanSchemaAliasStart() bool {
	p.nextToken() // schema
	if !p.isIdentifier() {
		return false
	}
	p.nextToken()
	return p.token == ast.KindEqualsToken
}

// parseSchemaAliasDeclaration parses schema Name = Type.
func (p *Parser) parseSchemaAliasDeclaration(pos int, jsdoc jsdocScannerInfo, modifiers *ast.ModifierList) *ast.Node {
	keyword := p.scanner.TokenRange()
	p.nextToken() // schema
	name := p.parseBindingIdentifier()
	p.parseExpected(ast.KindEqualsToken)
	typeNode := p.parseType()
	p.parseSemicolon()
	p.recordVibeSyntax(VibeSyntax{
		Kind:  VibeSyntaxDeclaration,
		Name:  VibeSyntaxSchemaAlias,
		Head:  core.NewTextRange(keyword.Pos(), name.End()),
		Range: core.NewTextRange(scanner.SkipTrivia(p.sourceText, pos), p.nodePos()),
	})

	// const Code = <the type's schema>
	value := p.vibelangSchemaOf(typeNode)
	span := core.NewTextRange(name.Pos(), typeNode.End())
	declaration := p.vibelangSpanning(p.factory.NewVariableDeclaration(name, nil /*exclamationToken*/, nil /*typeNode*/, value), span)
	declaration.Flags |= ast.NodeFlagsVibeLang
	list := p.vibelangSpanning(p.factory.NewVariableDeclarationList(p.newNodeList(span, []*ast.Node{declaration}), ast.NodeFlagsConst), span)
	statement := p.factory.NewVariableStatement(modifiers, list)
	p.finishNode(statement, pos)
	p.withJSDoc(statement, jsdoc)

	// type Code = typeof Code.Type
	var aliasModifiers *ast.ModifierList
	if modifiers != nil {
		var copies []*ast.Node
		for _, modifier := range modifiers.Nodes {
			copies = append(copies, p.vibelangSynthesized(p.factory.NewModifier(modifier.Kind), keyword))
		}
		aliasModifiers = p.newModifierList(keyword, copies)
	}
	typeName := p.vibelangSynthesized(p.factory.NewIdentifier(name.Text()), name.Loc)
	left := p.vibelangSynthesized(p.factory.NewIdentifier(name.Text()), name.Loc)
	right := p.vibelangSynthesized(p.factory.NewIdentifier("Type"), name.Loc)
	query := p.vibelangSynthesized(p.factory.NewTypeQueryNode(p.vibelangSynthesized(p.factory.NewQualifiedName(left, right), name.Loc), nil /*typeArguments*/), name.Loc)
	alias := p.factory.NewTypeAliasDeclaration(aliasModifiers, typeName, nil /*typeParameters*/, query)
	p.vibelangSynthesized(alias, core.NewTextRange(pos, keyword.End()))
	p.reparseList = append(p.reparseList, alias)
	return statement
}

// IsSchemaAliasDeclaration reports whether node is the variable declaration
// of a schema alias, schema Name = Type (D280).
func IsSchemaAliasDeclaration(node *ast.Node) bool {
	return node != nil && node.Kind == ast.KindVariableDeclaration && node.Flags&ast.NodeFlagsVibeLang != 0
}
