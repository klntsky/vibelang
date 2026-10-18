package effeck

import (
	"context"
	"strconv"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/ls"
	"github.com/microsoft/TypeScript/tsc/internal/ls/lsconv"
	"github.com/microsoft/TypeScript/tsc/internal/lsp/lsproto"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

// The quick fixes for the EK48xx rules (lsrules.go). Each finds the node
// that its diagnostic was reported at, the node whose error span is the
// diagnostic's, and rewrites the source around it.

// reportedNode returns the outermost node of file whose error span is span
// and that test accepts, or nil.
func reportedNode(file *ast.SourceFile, span core.TextRange, test func(node *ast.Node) bool) *ast.Node {
	var found *ast.Node
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if found != nil || node.End() < span.Pos() || node.Pos() > span.End() {
			return found != nil
		}
		if test(node) && scanner.GetErrorRangeForNode(file, node) == span {
			found = node
			return true
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
	return found
}

// replaceRange returns one text edit that replaces the span from start to end.
func replaceRange(fix *ls.CodeFixContext, start, end int, text string) []*lsproto.TextEdit {
	return []*lsproto.TextEdit{{Range: fix.LS.ToLSPRange(fix.SourceFile, core.NewTextRange(start, end)), NewText: text}}
}

// nodeStart returns where node's text starts, after its leading trivia.
func nodeStart(file *ast.SourceFile, node *ast.Node) int {
	return scanner.SkipTrivia(file.Text(), node.Pos())
}

// callEffectFix calls the Effect value that EK4801 or EK4802 reports, so
// that it runs: e becomes e(), and a + b becomes (a + b)().
var callEffectFix = &ls.CodeFixProvider{
	Source:     diagnosticSource,
	ErrorCodes: []int32{EK4801.Code, EK4802.Code},
	GetCodeActions: func(_ context.Context, fix *ls.CodeFixContext) ([]*ls.CodeAction, error) {
		value := reportedNode(fix.SourceFile, fix.Span, func(node *ast.Node) bool {
			parent := node.Parent
			switch {
			case ast.IsExpressionStatement(parent) && parent.Expression() == node:
				return inLowerableBody(parent)
			case ast.IsReturnStatement(parent):
				return true
			case ast.IsArrowFunction(parent):
				return parent.Body() == node
			}
			return false
		})
		if value == nil {
			return nil, nil
		}
		changes := replaceRange(fix, value.End(), value.End(), "()")
		if needsParentheses(value) {
			changes = append(replaceRange(fix, nodeStart(fix.SourceFile, value), nodeStart(fix.SourceFile, value), "("), replaceRange(fix, value.End(), value.End(), ")()")...)
		}
		return []*ls.CodeAction{{Description: "Call it to run it", Changes: changes}}, nil
	},
}

// mergeAllFix writes wire for the Layer.mergeAll that EK4807 reports.
var mergeAllFix = &ls.CodeFixProvider{
	Source:     diagnosticSource,
	ErrorCodes: []int32{EK4807.Code},
	GetCodeActions: func(_ context.Context, fix *ls.CodeFixContext) ([]*ls.CodeAction, error) {
		argument := reportedNode(fix.SourceFile, fix.Span, func(node *ast.Node) bool {
			return node.Parent != nil && ast.IsCallExpression(node.Parent) && node.Parent.Expression() != node
		})
		if argument == nil {
			return nil, nil
		}
		callee := argument.Parent.Expression()
		return []*ls.CodeAction{{
			Description: "Write wire for Layer.mergeAll",
			Changes:     replaceRange(fix, nodeStart(fix.SourceFile, callee), callee.End(), "wire"),
		}}, nil
	},
}

// nestedProvideFix merges the provide block that EK4808 reports into the
// block around it: provide A { provide B { … } } becomes provide A, B { … },
// or provide wire(A, B) { … } when B needs a service that A provides.
var nestedProvideFix = &ls.CodeFixProvider{
	Source:     diagnosticSource,
	ErrorCodes: []int32{EK4808.Code},
	GetCodeActions: func(ctx context.Context, fix *ls.CodeFixContext) ([]*ls.CodeAction, error) {
		file := fix.SourceFile
		var inner *ast.Node
		var visit func(node *ast.Node) bool
		visit = func(node *ast.Node) bool {
			if isProvideBlock(node) && nodeStart(file, node) == fix.Span.Pos() && isProvideBlock(node.Parent) {
				inner = node
				return true
			}
			return node.ForEachChild(visit)
		}
		file.AsNode().ForEachChild(visit)
		if inner == nil {
			return nil, nil
		}
		outer := inner.Parent
		outerLayer, innerLayer := provideLayer(outer), provideLayer(inner)
		c, done := fix.Program.GetTypeCheckerForFile(ctx, file)
		provided, _, _ := providerChannelsOf(c, outerLayer)
		_, _, needs := providerChannelsOf(c, innerLayer)
		dependent := false
		if provided != nil && needs != nil {
			for _, t := range needs.Distributed() {
				for _, p := range provided.Distributed() {
					dependent = dependent || t == p
				}
			}
		}
		done()
		outerText, innerText := scanner.GetTextOfNode(outerLayer), scanner.GetTextOfNode(innerLayer)
		providers := outerText + ", " + innerText
		if dependent {
			if len(providerList(outerLayer)) != 1 || len(providerList(innerLayer)) != 1 || isServiceProvider(outerLayer) || isServiceProvider(innerLayer) {
				return nil, nil // wire composes layers, not Key = value providers
			}
			providers = "wire(" + providers + ")"
		}
		text := file.Text()
		open := strings.IndexByte(text[innerLayer.End():], '{')
		if open < 0 {
			return nil, nil
		}
		body := text[innerLayer.End()+open+1 : inner.End()-1]
		body = strings.ReplaceAll(body, "\n"+lineIndent(text, nodeStart(file, inner)), "\n"+lineIndent(text, nodeStart(file, outer)))
		return []*ls.CodeAction{{
			Description: "Provide both layers in one block",
			Changes:     replaceRange(fix, nodeStart(file, outer), outer.End(), "provide "+providers+" {"+body+"}"),
		}}, nil
	},
}

// assertionFix removes the assertion that EK4810 reports.
var assertionFix = &ls.CodeFixProvider{
	Source:     diagnosticSource,
	ErrorCodes: []int32{EK4810.Code},
	GetCodeActions: func(_ context.Context, fix *ls.CodeFixContext) ([]*ls.CodeAction, error) {
		assertion := reportedNode(fix.SourceFile, fix.Span, func(node *ast.Node) bool {
			return ast.IsAsExpression(node) || ast.IsTypeAssertion(node)
		})
		if assertion == nil {
			return nil, nil
		}
		return []*ls.CodeAction{{
			Description: "Remove the assertion",
			Changes:     replaceRange(fix, nodeStart(fix.SourceFile, assertion), assertion.End(), scanner.GetTextOfNode(assertion.Expression())),
		}}, nil
	},
}

// outdatedAPIFix writes the Effect 4 name of the Effect 3 name that EK4813
// reports.
var outdatedAPIFix = &ls.CodeFixProvider{
	Source:     diagnosticSource,
	ErrorCodes: []int32{EK4813.Code},
	GetCodeActions: func(_ context.Context, fix *ls.CodeFixContext) ([]*ls.CodeAction, error) {
		return renameEffect3API(fix, reportedNode(fix.SourceFile, fix.Span, isEffect3Name)), nil
	},
}

// outdatedAPITypeScriptFix is outdatedAPIFix for a .ts file, where
// TypeScript's TS2339 reports an Effect 3 name of effect's Effect module.
var outdatedAPITypeScriptFix = &ls.CodeFixProvider{
	ErrorCodes: []int32{2339},
	GetCodeActions: func(ctx context.Context, fix *ls.CodeFixContext) ([]*ls.CodeAction, error) {
		name := reportedNode(fix.SourceFile, fix.Span, isEffect3Name)
		if name == nil {
			return nil, nil
		}
		c, done := fix.Program.GetTypeCheckerForFile(ctx, fix.SourceFile)
		defer done()
		if effectModuleOf(c, name.Parent.Expression()) != "Effect" {
			return nil, nil
		}
		return renameEffect3API(fix, name), nil
	},
}

// isEffect3Name reports whether node is the name of a property access that
// effect3API lists as renamed.
func isEffect3Name(node *ast.Node) bool {
	return ast.IsIdentifier(node) && node.Parent != nil && ast.IsPropertyAccessExpression(node.Parent) && node.Parent.Name() == node && effect3API[node.Text()].renamed != ""
}

// renameEffect3API returns the action that writes name's Effect 4 name.
func renameEffect3API(fix *ls.CodeFixContext, name *ast.Node) []*ls.CodeAction {
	if name == nil {
		return nil
	}
	renamed := effect3API[name.Text()].renamed
	module := scanner.GetTextOfNode(name.Parent.Expression())
	return []*ls.CodeAction{{
		Description: "Write " + module + "." + renamed,
		Changes:     replaceRange(fix, nodeStart(fix.SourceFile, name), name.End(), renamed),
	}}
}

// schemaIsFix writes Schema.is(S)(x) for the x instanceof S that EK4817
// reports.
var schemaIsFix = &ls.CodeFixProvider{
	Source:     diagnosticSource,
	ErrorCodes: []int32{EK4817.Code},
	GetCodeActions: func(_ context.Context, fix *ls.CodeFixContext) ([]*ls.CodeAction, error) {
		test := reportedNode(fix.SourceFile, fix.Span, func(node *ast.Node) bool {
			return ast.IsBinaryExpression(node) && node.AsBinaryExpression().OperatorToken.Kind == ast.KindInstanceOfKeyword
		})
		if test == nil {
			return nil, nil
		}
		b := test.AsBinaryExpression()
		text := "Schema.is(" + scanner.GetTextOfNode(b.Right) + ")(" + scanner.GetTextOfNode(b.Left) + ")"
		return []*ls.CodeAction{{Description: "Test with Schema.is", Changes: replaceRange(fix, nodeStart(fix.SourceFile, test), test.End(), text)}}, nil
	},
}

// trustedValueFix moves D23 metadata into a value beside the implementation,
// retaining the original function for TypeScript callers (D249).
var trustedValueFix = &ls.CodeFixProvider{
	Source:     diagnosticSource,
	ErrorCodes: []int32{EK4406.Code},
	GetCodeActions: func(ctx context.Context, fix *ls.CodeFixContext) ([]*ls.CodeAction, error) {
		callee := reportedNode(fix.SourceFile, fix.Span, func(n *ast.Node) bool {
			return ast.IsIdentifier(n) && n.Parent != nil && ast.IsCallExpression(n.Parent) && n.Parent.Expression() == n
		})
		if callee == nil {
			return nil, nil
		}
		c, done := fix.Program.GetTypeCheckerForFile(ctx, fix.SourceFile)
		defer done()
		plan := trustedValueEdits(c, fix.SourceFile, callee.Parent)
		if plan == nil {
			return nil, nil
		}
		edits := func(file *ast.SourceFile, replacements []trustedValueEdit) []*lsproto.TextEdit {
			var result []*lsproto.TextEdit
			for _, edit := range replacements {
				result = append(result, &lsproto.TextEdit{Range: fix.LS.ToLSPRange(file, core.NewTextRange(edit.start, edit.end)), NewText: edit.text})
			}
			return result
		}
		action := &ls.CodeAction{Description: "Export and call " + plan.name, Changes: edits(fix.SourceFile, plan.caller)}
		if len(plan.declaration) != 0 {
			action.AdditionalChanges = []ls.FileTextChanges{{URI: lsconv.FileNameToDocumentURI(tspath.RootedFilePath(plan.file.FileName())), Changes: edits(plan.file, plan.declaration)}}
		}
		return []*ls.CodeAction{action}, nil
	},
}

// trustedValueEdit is a replacement in one of the two migration documents.
type trustedValueEdit struct {
	start, end int
	text       string
}

// trustedValuePlan keeps document ranges separate until the LSP conversion.
type trustedValuePlan struct {
	file                *ast.SourceFile
	name                string
	declaration, caller []trustedValueEdit
}

// trustedValueEdits plans a value contract for a named imported function.
// Methods and declaration-only packages need a hand-written adapter: guessing
// a receiver or editing a package's declarations would change their meaning.
func trustedValueEdits(c *checker.Checker, file *ast.SourceFile, call *ast.Node) *trustedValuePlan {
	callee := call.Expression()
	if !ast.IsIdentifier(callee) {
		return nil
	}
	t := callTrust(c, call)
	if t.kind != trustTagged || t.declaration == nil || t.unreachable != "" || t.promise {
		return nil
	}
	decl := t.declaration
	source := ast.GetSourceFileOfNode(decl)
	if source == nil || source == file || source.IsDeclarationFile || strings.Contains(string(source.FileName()), "/node_modules/") {
		return nil
	}
	var statement *ast.Node
	switch {
	case ast.IsFunctionDeclaration(decl):
		statement = decl
		if sym := decl.Symbol(); sym != nil {
			for _, overload := range sym.Declarations() {
				if ast.IsFunctionDeclaration(overload) && overload.End() > statement.End() {
					statement = overload
				}
			}
		}
	case ast.IsVariableDeclaration(decl):
		statement = ast.FindAncestorKind(decl, ast.KindVariableStatement)
	default:
		return nil
	}
	if statement == nil || statement.Parent != source.AsNode() || decl.Name() == nil || !ast.IsIdentifier(decl.Name()) {
		return nil
	}
	// Only a direct import gives the caller an unambiguous module to import
	// the new export from, including when it aliases the original function.
	sym := c.GetSymbolAtLocation(callee)
	if sym == nil || sym.Flags()&ast.SymbolFlagsAlias == 0 {
		return nil
	}
	var imported *ast.Node
	for _, d := range sym.Declarations() {
		if ast.IsImportSpecifier(d) {
			imported = ast.FindAncestorKind(d, ast.KindImportDeclaration)
			break
		}
	}
	if imported == nil {
		return nil
	}

	// A barrel can export the original function without exporting the new value.
	// Only edit a declaration reached through its own module.
	module := c.ResolveExternalModuleName(imported.ModuleSpecifier(), nil)
	direct := false
	if module != nil {
		for _, d := range module.Declarations() {
			direct = direct || ast.GetSourceFileOfNode(d) == source
		}
	}
	if !direct {
		return nil
	}
	fresh := func(base string, scope *ast.SourceFile) string {
		name := base
		for i := 2; c.ResolveName(name, scope.AsNode(), ast.SymbolFlagsValue|ast.SymbolFlagsType|ast.SymbolFlagsAlias, false) != nil; i++ {
			name = base + strconv.Itoa(i)
		}
		return name
	}
	plan := &trustedValuePlan{file: source}
	stem := decl.Name().Text() + "Trusted"
	// Reuse only a wrapper with this implementation and these exact classes;
	// a similarly named user value must not silently change the contract.
	for _, stmt := range source.Statements.Nodes {
		if !ast.IsVariableStatement(stmt) || !ast.HasSyntacticModifier(stmt, ast.ModifierFlagsExport) {
			continue
		}
		for _, v := range stmt.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
			if !ast.IsIdentifier(v.Name()) || !strings.HasPrefix(v.Name().Text(), stem) {
				continue
			}
			init := v.Initializer()
			if init == nil || !ast.IsCallExpression(init) || len(init.Arguments()) != 2 {
				continue
			}
			helper := trustedImportName(source, "trusted")
			args := init.Arguments()
			if helper == "" || !ast.IsIdentifier(init.Expression()) || init.Expression().Text() != helper || !ast.IsIdentifier(args[0]) || args[0].Text() != decl.Name().Text() || !ast.IsArrayLiteralExpression(args[1]) {
				continue
			}
			classes := args[1].Elements()
			if len(classes) != len(t.failures) {
				continue
			}
			same := true
			for i, f := range t.failures {
				same = same && ast.IsIdentifier(classes[i]) && classes[i].Text() == f.name
			}
			if same {
				plan.name = v.Name().Text()
				break
			}
		}
		if plan.name != "" {
			break
		}
	}
	if plan.name == "" {
		plan.name = fresh(stem, source)
		helper := trustedImportName(source, "trusted")
		if helper == "" {
			helper = fresh("trusted", source)
			binding := "trusted"
			if helper != binding {
				binding += " as " + helper
			}
			plan.declaration = append(plan.declaration, trustedImportEdit(source, "effeck-runtime", binding))
		}
		var classes []string
		for _, f := range t.failures {
			classes = append(classes, f.name)
		}
		end := statement.End()
		plan.declaration = append(plan.declaration, trustedValueEdit{end, end, "\nexport const " + plan.name + " = " + helper + "(" + decl.Name().Text() + ", [" + strings.Join(classes, ", ") + "]);\n"})
	}
	local := ""
	for _, stmt := range file.Statements.Nodes {
		if !ast.IsImportDeclaration(stmt) || stmt.ModuleSpecifier().Text() != imported.ModuleSpecifier().Text() {
			continue
		}
		local = namedImportLocal(stmt, plan.name)
		if local != "" {
			break
		}
	}
	if local == "" {
		local = fresh(plan.name, file)
		binding := plan.name
		if local != binding {
			binding += " as " + local
		}
		plan.caller = append(plan.caller, trustedImportEdit(file, imported.ModuleSpecifier().Text(), binding))
	}
	plan.caller = append(plan.caller, trustedValueEdit{nodeStart(file, callee), callee.End(), local})
	return plan
}

// namedImportLocal finds a value import's local spelling so fixes preserve aliases.
func namedImportLocal(statement *ast.Node, name string) string {
	clause := statement.ImportClause()
	if clause == nil || clause.AsImportClause().IsTypeOnly() {
		return ""
	}
	bindings := clause.AsImportClause().NamedBindings
	if bindings == nil || !ast.IsNamedImports(bindings) {
		return ""
	}
	for _, spec := range bindings.Elements() {
		if spec.AsImportSpecifier().IsTypeOnly {
			continue
		}
		original := spec.PropertyName()
		if original == nil {
			original = spec.Name()
		}
		if original.Text() == name {
			return spec.Name().Text()
		}
	}
	return ""
}

// trustedImportName finds the existing runtime import to avoid duplicate helpers.
func trustedImportName(file *ast.SourceFile, name string) string {
	for _, stmt := range file.Statements.Nodes {
		if ast.IsImportDeclaration(stmt) && stmt.ModuleSpecifier().Text() == "effeck-runtime" {
			if local := namedImportLocal(stmt, name); local != "" {
				return local
			}
		}
	}
	return ""
}

// trustedImportEdit extends a named import or adds one while preserving comments.
func trustedImportEdit(file *ast.SourceFile, module, binding string) trustedValueEdit {
	pos := 0
	if strings.HasPrefix(file.Text(), "#!") {
		if e := strings.IndexByte(file.Text(), '\n'); e >= 0 {
			pos = e + 1
		}
	}
	for _, stmt := range file.Statements.Nodes {
		if !ast.IsImportDeclaration(stmt) {
			continue
		}
		pos = stmt.End()
		if stmt.ModuleSpecifier().Text() != module {
			continue
		}
		clause := stmt.ImportClause()
		if clause == nil || clause.AsImportClause().IsTypeOnly() {
			continue
		}
		bindings := clause.AsImportClause().NamedBindings
		if bindings == nil || !ast.IsNamedImports(bindings) {
			continue
		}
		elements := bindings.Elements()
		at := bindings.End() - 1
		text := binding
		if len(elements) != 0 {
			at = elements[len(elements)-1].End()
			text = ", " + binding
		}
		return trustedValueEdit{at, at, text}
	}
	return trustedValueEdit{pos, pos, "\nimport { " + binding + " } from " + strconv.Quote(module) + ";\n"}
}
