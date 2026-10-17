package effeck

import (
	"context"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/ls"
	"github.com/microsoft/TypeScript/tsc/internal/lsp/lsproto"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// The current spelling of services, controls, and layers (D220, D221).
//
//	service class Users { find(id: string): User!NotFound }
//	control class Approval { approve(request: string): boolean!! }
//	const UsersLive = Layer.effect(Users) { const seen = new Set<string>(); return { find(id) { ... } } }
//
// The older spellings still compile and draw warnings with quick fixes:
// service Users { ... } and control Approval { ... } without class (EK2120),
// and layer UsersLive: Users { ... } (EK2121), whose fix rewrites it into the
// Layer.effect block or Layer.succeed call it abbreviates.

var (
	// EK2120: a service or control declaration written without class (D220). {0} is the word. A warning.
	EK2120 = message(2120, warningCategory, "{0} is a modifier of a class declaration: write {0} class.")
	// EK2121: a layer declaration (D221). A warning.
	EK2121 = message(2121, warningCategory, "layer declarations are the old spelling of a layer: write Layer.effect(Service) { ... } or Layer.succeed(Service, { ... }).")
)

func init() {
	registry[EK2120.Code] = EK2120
	registry[EK2121.Code] = EK2121
	fileChecks = append(fileChecks, checkDeclarationSpelling)
	ls.RegisterCodeFixProvider(serviceClassFix)
	ls.RegisterCodeFixProvider(layerDeclarationFix)
}

// declarationWordRange returns the range of the word service or control that
// declared class, from the abstract modifier the parser anchored on it, and
// false when class was not declared with that word.
func declarationWordRange(class *ast.Node) (core.TextRange, bool) {
	for _, modifier := range class.ModifierNodes() {
		if modifier.Kind == ast.KindAbstractKeyword && modifier.Flags&ast.NodeFlagsReparsed != 0 {
			return modifier.Loc, true
		}
	}
	return core.TextRange{}, false
}

// layerDeclarationCall returns the Layer.effect or Layer.succeed call the
// parser wrote for statement when statement is a layer declaration, and nil
// otherwise. The parser anchors the synthesized Layer on the word layer.
func layerDeclarationCall(statement *ast.Node) *ast.Node {
	if !ast.IsVariableStatement(statement) {
		return nil
	}
	declarations := statement.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
	if len(declarations) != 1 || declarations[0].Initializer() == nil || !ast.IsCallExpression(declarations[0].Initializer()) {
		return nil
	}
	call := declarations[0].Initializer()
	callee := call.Expression()
	if callee.Flags&ast.NodeFlagsReparsed == 0 || !ast.IsPropertyAccessExpression(callee) {
		return nil
	}
	if name := callee.Name().Text(); name != "effect" && name != "succeed" {
		return nil
	}
	if layer := callee.Expression(); !ast.IsIdentifier(layer) || layer.Text() != "Layer" {
		return nil
	}
	return call
}

// checkDeclarationSpelling reports EK2120 at a service or control declared
// without class, and EK2121 at a layer declaration.
func checkDeclarationSpelling(c *checker.Checker, file *ast.SourceFile) {
	text := file.Text()
	var visit func(n *ast.Node) bool
	visit = func(n *ast.Node) bool {
		if ast.IsClassDeclaration(n) {
			if word, ok := declarationWordRange(n); ok {
				after := scanner.SkipTrivia(text, word.End())
				if !strings.HasPrefix(text[after:], "class") {
					reportAt(c, file, word, EK2120, text[word.Pos():word.End()])
				}
			}
		}
		if call := layerDeclarationCall(n); call != nil {
			reportAt(c, file, call.Expression().Expression().Loc, EK2121)
		}
		return n.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// serviceClassFix is EK2120's quick fix: it writes class after the word.
var serviceClassFix = &ls.CodeFixProvider{
	Source:     diagnosticSource,
	ErrorCodes: []int32{EK2120.Code},
	GetCodeActions: func(_ context.Context, fix *ls.CodeFixContext) ([]*ls.CodeAction, error) {
		text := fix.SourceFile.Text()
		if fix.Diagnostic == nil || fix.Span.End() > len(text) {
			return nil, nil
		}
		word := text[fix.Span.Pos():fix.Span.End()]
		if word != "service" && word != "control" {
			return nil, nil
		}
		end := fix.Diagnostic.Range.End
		return []*ls.CodeAction{{
			Description: "Write " + word + " class",
			Changes:     []*lsproto.TextEdit{{Range: lsproto.Range{Start: end, End: end}, NewText: " class"}},
		}}, nil
	},
}

// layerDeclarationFix is EK2121's quick fix: it rewrites the layer declaration
// whose word layer the diagnostic marks into the call it abbreviates.
var layerDeclarationFix = &ls.CodeFixProvider{
	Source:     diagnosticSource,
	ErrorCodes: []int32{EK2121.Code},
	GetCodeActions: func(_ context.Context, fix *ls.CodeFixContext) ([]*ls.CodeAction, error) {
		file := fix.SourceFile
		var statement *ast.Node
		var find func(n *ast.Node) bool
		find = func(n *ast.Node) bool {
			if call := layerDeclarationCall(n); call != nil && call.Expression().Expression().Pos() == fix.Span.Pos() {
				statement = n
				return true
			}
			return n.ForEachChild(find)
		}
		file.AsNode().ForEachChild(find)
		if statement == nil {
			return nil, nil
		}
		replacement, ok := layerDeclarationText(file, statement)
		if !ok {
			return nil, nil
		}
		start := scanner.SkipTrivia(file.Text(), statement.Pos())
		return []*ls.CodeAction{{
			Description: "Rewrite the layer declaration as a Layer call",
			Changes: []*lsproto.TextEdit{{
				Range:   lsproto.Range{Start: lspPosition(file, start), End: lspPosition(file, statement.End())},
				NewText: replacement,
			}},
		}}, nil
	},
}

// lspPosition converts an offset in file to an LSP position.
func lspPosition(file *ast.SourceFile, pos int) lsproto.Position {
	line, character := scanner.GetECMALineAndUTF16CharacterOfPosition(file, pos)
	return lsproto.Position{Line: uint32(line), Character: uint32(character)}
}

// layerDeclarationText writes statement, a layer declaration, as the call it
// abbreviates:
//
//	export const UsersLive = Layer.effect(Users) {
//	  const seen = new Set<string>()
//	  return {
//	    find(id) { ... },
//	    region: "eu"
//	  }
//	}
//
// or, without setup statements, export const DirectoryLive = Layer.succeed(Directory, { ... }).
func layerDeclarationText(file *ast.SourceFile, statement *ast.Node) (string, bool) {
	text := file.Text()
	call := layerDeclarationCall(statement)
	args := call.Arguments()
	if len(args) != 2 {
		return "", false
	}
	start := scanner.SkipTrivia(text, statement.Pos())
	word := call.Expression().Expression().Loc
	prefix := text[start:word.Pos()] // modifiers such as export, with their spacing
	indent := lineIndent(text, start)
	declaration := statement.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0]
	name := sourceText(file, declaration.Name())
	service := sourceText(file, args[0])

	var statements []*ast.Node
	object := args[1]
	if ast.IsArrowFunction(args[1]) {
		body := args[1].Body().AsBlock().Statements.Nodes
		if len(body) == 0 || !ast.IsReturnStatement(body[len(body)-1]) || body[len(body)-1].Expression() == nil {
			return "", false
		}
		statements = body[:len(body)-1]
		object = body[len(body)-1].Expression()
	}
	if !ast.IsObjectLiteralExpression(object) {
		return "", false
	}
	var members []string
	for _, member := range object.AsObjectLiteralExpression().Properties.Nodes {
		switch {
		case ast.IsPropertyAssignment(member):
			members = append(members, sourceText(file, member.Name())+": "+sourceText(file, member.Initializer()))
		default:
			members = append(members, sourceText(file, member))
		}
	}

	var b strings.Builder
	b.WriteString(prefix)
	b.WriteString("const " + name + " = ")
	if len(statements) == 0 {
		b.WriteString("Layer.succeed(" + service + ", {\n")
		writeMembers(&b, members, indent+"  ", "")
		b.WriteString(indent + "})")
		return b.String(), true
	}
	b.WriteString("Layer.effect(" + service + ") {\n")
	for _, s := range statements {
		b.WriteString(indent + "  " + sourceText(file, s) + "\n")
	}
	b.WriteString(indent + "  return {\n")
	writeMembers(&b, members, indent+"    ", "  ")
	b.WriteString(indent + "  }\n")
	b.WriteString(indent + "}")
	return b.String(), true
}

// writeMembers writes members, one per line at indent and separated by
// commas, adding extra to the indentation of their continuation lines.
func writeMembers(b *strings.Builder, members []string, indent string, extra string) {
	for i, member := range members {
		member = strings.ReplaceAll(member, "\n", "\n"+extra)
		b.WriteString(indent + member)
		if i < len(members)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
}

// sourceText returns node's text in file, without leading trivia.
func sourceText(file *ast.SourceFile, node *ast.Node) string {
	return scanner.GetSourceTextOfNodeFromSourceFile(file, node, false /*includeTrivia*/)
}

// lineIndent returns the whitespace that starts the line holding pos.
func lineIndent(text string, pos int) string {
	lineStart := strings.LastIndexByte(text[:pos], '\n') + 1
	end := lineStart
	for end < len(text) && (text[end] == ' ' || text[end] == '\t') {
		end++
	}
	return text[lineStart:end]
}
