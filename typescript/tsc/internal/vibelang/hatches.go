package vibelang

import (
	"context"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/astnav"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/ls"
	"github.com/microsoft/TypeScript/tsc/internal/lsp/lsproto"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// The boundary hatches (design D148) run plain TypeScript inside VibeLang:
//
//	const id = sync(crypto.randomUUID())                       // Effect.sync
//	const parsed = try((text) => JSON.parse(text), text)       // Effect.try: a throw is UnknownError
//	const body = try(async (url, signal) => fetch(url, { signal }), (error) => new Offline({ error }), url)
//	const tick = callback<number>((resume) => {                // Effect.callback
//	  const timer = setTimeout(() => resume(1), 10)
//	  return () => clearTimeout(timer)
//	})
//
// A hatch's first argument is TypeScript, not VibeLang: a call there does not
// run, so calling a .vibe function gives an inert Effect value for
// runPromise or runFork; throw, async, and await keep their JavaScript
// meaning; platform APIs are allowed (no EK4401, EK4402, or EK4403); and a
// runner may start a program (no EK1005). That TypeScript is self-contained
// (D229): it may use its own parameters and locals, globals, imports, and the
// file's error declarations, but no other binding of the .vibe file
// (EK4405). Values reach it as inputs, the arguments after the function, which
// fill its first parameters (D230); try's error mapper stays the second
// argument (D231). The inputs and the mapper are VibeLang (D232).
//
// A hatch is a call whose callee is the prelude's sync, try, or callback: the
// name, with no declaration in any scope around the call that shadows it. try
// is a keyword, so nothing can shadow it.

// hatchNames are the prelude's hatches.
var hatchNames = map[string]bool{"sync": true, "try": true, "callback": true}

// hatchOf returns the name of the hatch that call calls, or "".
func hatchOf(call *ast.Node) string {
	if call == nil || !ast.IsCallExpression(call) {
		return ""
	}
	callee := call.Expression()
	if !ast.IsIdentifier(callee) || !hatchNames[callee.Text()] {
		return ""
	}
	name := callee.Text()
	if !isVibeLangFile(ast.GetSourceFileOfNode(call)) || name != "try" && shadowed(call, name) {
		return ""
	}
	return name
}

// shadowed reports whether a scope around node declares name: a block, a
// function, or the module, whose locals the binder recorded.
func shadowed(node *ast.Node, name string) bool {
	for n := node; n != nil; n = n.Parent {
		if locals := n.Locals(); locals != nil {
			if _, ok := locals[name]; ok {
				return true
			}
		}
	}
	return false
}

// isHatchArgument reports whether node is a hatch's TypeScript: its first
// argument. The inputs after it, and try's error mapper, are VibeLang (D232).
func isHatchArgument(node *ast.Node) bool {
	parent := node.Parent
	if parent == nil || !ast.IsCallExpression(parent) || parent.Expression() == node || hatchOf(parent) == "" {
		return false
	}
	arguments := parent.Arguments()
	return len(arguments) > 0 && arguments[0] == node
}

// forEachHatchVibeLang calls visit with the arguments of call, a hatch, that are
// VibeLang: its error mapper and inputs, every argument after the first (D232).
// It returns true when visit does, as ForEachChild would.
func forEachHatchVibeLang(call *ast.Node, visit func(node *ast.Node) bool) bool {
	arguments := call.Arguments()
	for i := 1; i < len(arguments); i++ {
		if visit(arguments[i]) {
			return true
		}
	}
	return false
}

// inHatch reports whether node is a hatch's first argument or inside one,
// where TypeScript, not VibeLang, gives the code its meaning.
func inHatch(node *ast.Node) bool {
	for n := node; n != nil && n.Parent != nil; n = n.Parent {
		if isHatchArgument(n) {
			return true
		}
	}
	return false
}

func init() {
	fileChecks = append(fileChecks, checkHatches)
	ls.RegisterCodeFixProvider(hatchInputFix)
	ls.RegisterQuickInfoHook(hatchQuickInfo)
}

// checkHatches reports EK4404 at try({ try: …, catch: … }): Effect's options
// object is a value to the hatch, which would succeed with the object itself.
// It reports EK4405 at each reference in a hatch's TypeScript to a binding
// that the .vibe file declares outside it (D229).
func checkHatches(c *checker.Checker, file *ast.SourceFile) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if name := hatchOf(node); name != "" {
			if name == "try" {
				if arguments := node.Arguments(); len(arguments) == 1 && ast.IsObjectLiteralExpression(arguments[0]) && hasProperty(arguments[0], "try") {
					d := report(c, arguments[0], EK4404, "try")
					note(d, "Write try(code, (error) => …), as in try((text) => parse(text), (error) => new BadInput({ error }), text).")
					cover(c, arguments[0]) // TypeScript's errors inside the object restate EK4404
					return node.ForEachChild(visit)
				}
			}
			for _, reference := range outsideReferences(c, hatchIsland(node)) {
				d := report(c, reference, EK4405, reference.Text(), name)
				note(d, "Add a parameter for it and pass the value after the function, as in {0}(({1}) => …, {1}).", name, reference.Text())
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// hatchIsland returns the TypeScript of call, a hatch: its first argument, or
// nil.
func hatchIsland(call *ast.Node) *ast.Node {
	if arguments := call.Arguments(); len(arguments) > 0 {
		return arguments[0]
	}
	return nil
}

// outsideReferences returns the references in island, TypeScript, to bindings
// that its file declares outside island, in source order.
func outsideReferences(c *checker.Checker, island *ast.Node) []*ast.Node {
	if island == nil {
		return nil
	}
	var found []*ast.Node
	var visit func(n *ast.Node) bool
	visit = func(n *ast.Node) bool {
		switch {
		case n.Kind == ast.KindTypeQuery:
			// `typeof value` here names a type, not a runtime capture (D229).
			// IsPartOfTypeNode deliberately treats its queried name as a value.
			return false
		case n.Kind == ast.KindShorthandPropertyAssignment:
			if declaredOutside(c.GetShorthandAssignmentValueSymbol(n), island) {
				found = append(found, n.Name())
			}
			return false
		case n.Kind == ast.KindIdentifier && isHatchValueReference(n):
			if declaredOutside(c.GetSymbolAtLocation(n), island) {
				found = append(found, n)
			}
		}
		return n.ForEachChild(visit)
	}
	visit(island)
	return found
}

// outsideNames returns the names of the bindings that outsideReferences finds
// in island, each once, in source order.
func outsideNames(c *checker.Checker, island *ast.Node) []string {
	var names []string
	seen := map[string]bool{}
	for _, reference := range outsideReferences(c, island) {
		if name := reference.Text(); !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}

// hatchCall writes code, a plain expression, as a call of hatch that passes
// the bindings code reads from its file as inputs (D229, D230), for a note
// that suggests the hatch: try(parse(text)) reads text, so it is
// try((text) => parse(text), text). A mapper that is not "" goes second.
func hatchCall(c *checker.Checker, file *ast.SourceFile, hatch string, code *ast.Node, mapper string) string {
	names := strings.Join(outsideNames(c, code), ", ")
	text := nodeText(file, code)
	if names != "" {
		text = "(" + names + ") => " + text
	}
	if mapper != "" {
		text += ", " + mapper
	}
	if names != "" {
		text += ", " + names
	}
	return hatch + "(" + text + ")"
}

// isHatchValueReference reports whether id, an identifier, reads a value: not a
// declaration's name, a type, or a member name after a dot.
func isHatchValueReference(id *ast.Node) bool {
	return id.Parent != nil && !ast.IsDeclarationName(id) && !ast.IsPartOfTypeNode(id) && !ast.IsRightSideOfQualifiedNameOrPropertyAccess(id)
}

// declaredOutside reports whether symbol is a value that island's file
// declares outside island. Imports, globals, prelude names, and the file's
// error declarations are not: an error declaration is a plain class, and
// failing with one is what a hatch most often needs from its file (D229). An
// .vibe file is always a module, so another one's values arrive by import.
func declaredOutside(symbol *ast.Symbol, island *ast.Node) bool {
	if symbol == nil || symbol.Flags()&ast.SymbolFlagsAlias != 0 || symbol.Flags()&ast.SymbolFlagsValue == 0 {
		return false
	}
	declaration := symbol.ValueDeclaration()
	if declaration == nil {
		if len(symbol.Declarations()) == 0 {
			return false
		}
		declaration = symbol.Declarations()[0]
	}
	if ast.GetSourceFileOfNode(declaration) != ast.GetSourceFileOfNode(island) || isTaggedErrorClass(declaration) {
		return false
	}
	for n := declaration; n != nil; n = n.Parent {
		if n == island {
			return false
		}
	}
	return true
}

// isTaggedErrorClass reports whether declaration is a class that extends
// TaggedError(…), as an error declaration does.
func isTaggedErrorClass(declaration *ast.Node) bool {
	if !ast.IsClassDeclaration(declaration) && !ast.IsClassExpression(declaration) {
		return false
	}
	heritage := ast.GetClassExtendsHeritageElement(declaration)
	if heritage == nil || !ast.IsCallExpression(heritage.Expression()) {
		return false
	}
	callee := heritage.Expression().Expression()
	switch {
	case ast.IsIdentifier(callee):
		return callee.Text() == "TaggedError"
	case ast.IsPropertyAccessExpression(callee):
		return callee.Name().Text() == "TaggedError"
	}
	return false
}

// hasProperty reports whether object, an object literal, has a property
// named name.
func hasProperty(object *ast.Node, name string) bool {
	for _, property := range object.AsObjectLiteralExpression().Properties.Nodes {
		if n := property.Name(); n != nil && ast.IsIdentifier(n) && n.Text() == name {
			return true
		}
	}
	return false
}

// hatchForm is what the lowering emits for a sync or try hatch.
type hatchForm struct {
	effect string // "sync", "promise", "try", or "tryPromise"
	value  bool   // sync(expression), try(expression): wrap it in () =>
	mapper bool   // try's second argument maps a thrown error (D231)
}

// hatchFormOf returns the form of call, a sync or try hatch. The overload the
// checker selected tells a function from a value, by the name the runtime
// gives its first parameter (evaluate or value), and whether try has an error
// mapper (a second parameter named mapError). The function itself decides
// between sync and async (D232): an async function, or one that returns a
// promise, is Effect.promise or Effect.tryPromise.
func hatchFormOf(c *checker.Checker, call *ast.Node) (hatchForm, bool) {
	name := hatchOf(call)
	if name != "sync" && name != "try" {
		return hatchForm{}, false
	}
	signature := c.GetResolvedSignature(call)
	if signature == nil || signature.Declaration() == nil || len(call.Arguments()) == 0 {
		return hatchForm{}, false
	}
	parameters := signature.Declaration().Parameters()
	if len(parameters) == 0 || !ast.IsIdentifier(parameters[0].Name()) {
		return hatchForm{}, false
	}
	var form hatchForm
	switch parameters[0].Name().Text() {
	case "evaluate":
	case "value":
		form.value = true
	default:
		return hatchForm{}, false
	}
	form.mapper = len(parameters) > 1 && ast.IsIdentifier(parameters[1].Name()) && parameters[1].Name().Text() == "mapError"
	form.effect = name
	if isAsyncHatch(c, call.Arguments()[0], form.value) {
		form.effect = map[string]string{"sync": "promise", "try": "tryPromise"}[name]
	}
	return form, true
}

// isAsyncHatch reports whether argument, a hatch's first argument, is
// asynchronous: an async function, a function that returns a promise, or, for
// a value, a promise.
func isAsyncHatch(c *checker.Checker, argument *ast.Node, value bool) bool {
	argument = ast.SkipParentheses(argument)
	if value {
		return isPromiseLike(c, c.GetTypeAtLocation(argument))
	}
	if ast.IsFunctionExpressionOrArrowFunction(argument) && ast.GetFunctionFlags(argument)&ast.FunctionFlagsAsync != 0 {
		return true
	}
	for _, signature := range c.GetSignaturesOfType(c.GetTypeAtLocation(argument), checker.SignatureKindCall) {
		if isPromiseLike(c, c.GetReturnTypeOfSignature(signature)) {
			return true
		}
	}
	return false
}

// isPromiseLike reports whether t is a promise: it has a then. any and unknown
// are not, so a function returning any, such as JSON.parse, stays synchronous.
func isPromiseLike(c *checker.Checker, t *checker.Type) bool {
	return t != nil && t.Flags()&checker.TypeFlagsAnyOrUnknown == 0 && c.GetPropertyOfType(t, "then") != nil
}

// visitHatchCall lowers a sync or try hatch to Effect's function:
//
//	sync(() => f())                    Effect.sync(() => f())
//	sync(fetch)                        Effect.promise(fetch) when fetch returns a promise
//	sync((n) => n * 2, count)          Effect.sync(((n) => n * 2).bind(undefined, count))
//	try((s) => parse(s), wrap, text)   Effect.try({ try: ((s) => parse(s)).bind(undefined, text), catch: wrap })
//
// The first argument stays TypeScript: its calls do not run. The inputs and
// the mapper are VibeLang, lowered where the call stands, so a call among the
// inputs runs before the hatch starts. bind fills the function's first
// parameters with the inputs; Effect then passes the signal, if any, after
// them. The call itself returns an Effect, so it runs as yield* where calls
// run.
func (tx *lowering) visitHatchCall(node *ast.Node) *ast.Node {
	form := tx.plan.hatches[node]
	f := tx.Factory()
	arguments := node.Arguments()
	saved, savedPlain := tx.inEffect, tx.plain
	tx.inEffect, tx.plain = false, false
	body := tx.Visitor().VisitNode(arguments[0])
	tx.inEffect, tx.plain = saved, savedPlain
	rest := arguments[1:]
	var mapper *ast.Node
	if form.mapper && len(rest) > 0 {
		mapper = tx.Visitor().VisitNode(rest[0])
		rest = rest[1:]
	}
	inputs := []*ast.Node{f.NewIdentifier("undefined")}
	for _, input := range rest {
		inputs = append(inputs, tx.Visitor().VisitNode(input))
	}
	switch {
	case form.value:
		body = f.NewArrowFunction(nil /*modifiers*/, nil /*typeParameters*/, f.NewNodeList(nil), nil /*returnType*/, nil /*fullSignature*/, f.NewToken(ast.KindEqualsGreaterThanToken), body)
	case len(inputs) > 1:
		bind := f.NewPropertyAccessExpression(f.NewParenthesizedExpression(body), nil /*questionDotToken*/, f.NewIdentifier("bind"), ast.NodeFlagsNone)
		body = f.NewCallExpression(bind, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList(inputs), ast.NodeFlagsNone)
	}
	effectArguments := []*ast.Node{body}
	if mapper != nil {
		options := f.NewObjectLiteralExpression(f.NewNodeList([]*ast.Node{
			f.NewPropertyAssignment(nil /*modifiers*/, f.NewIdentifier("try"), nil /*postfixToken*/, nil /*typeNode*/, body),
			f.NewPropertyAssignment(nil /*modifiers*/, f.NewIdentifier("catch"), nil /*postfixToken*/, nil /*typeNode*/, mapper),
		}), false /*multiLine*/)
		effectArguments = []*ast.Node{options}
	}
	callee := f.NewPropertyAccessExpression(tx.effectName(), nil /*questionDotToken*/, f.NewIdentifier(form.effect), ast.NodeFlagsNone)
	lowered := f.NewCallExpression(callee, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList(effectArguments), ast.NodeFlagsNone)
	tx.EmitContext().SetOriginal(lowered, node)
	lowered.Loc = node.Loc
	if tx.inEffect && tx.plan.runsCall(node) {
		run := f.NewYieldExpression(f.NewToken(ast.KindAsteriskToken), lowered)
		tx.EmitContext().SetOriginal(run, node)
		return run
	}
	return lowered
}

// hatchQuickInfo describes a hatch on a hover of its name.
func hatchQuickInfo(c *checker.Checker, node *ast.Node) (string, string) {
	call := node.Parent
	if call == nil || !ast.IsCallExpression(call) || call.Expression() != node {
		return "", ""
	}
	name := hatchOf(call)
	if name == "" {
		return "", ""
	}
	paragraph := "An escape hatch: its first argument is self-contained TypeScript (D229)."
	if inputs := hatchInputs(c, call); len(inputs) > 0 {
		var names []string
		for _, input := range inputs {
			names = append(names, "`"+scanner.GetSourceTextOfNodeFromSourceFile(ast.GetSourceFileOfNode(call), input, false /*includeTrivia*/)+"`")
		}
		paragraph += " Inputs, which fill its first parameters: " + strings.Join(names, ", ") + "."
	}
	return "", paragraph
}

// hatchInputs returns the inputs of call, a hatch: the arguments after its
// function and, for try, after its error mapper.
func hatchInputs(c *checker.Checker, call *ast.Node) []*ast.Node {
	arguments := call.Arguments()
	if len(arguments) < 2 {
		return nil
	}
	rest := arguments[1:]
	if form, ok := hatchFormOf(c, call); ok && form.mapper {
		rest = rest[1:]
	}
	return rest
}

// hatchInputFix is EK4405's quick fix: it makes every binding the hatch's
// TypeScript reads from outside an input. It adds a parameter for each after
// the parameters that take the existing inputs, and passes each value after
// the existing inputs.
var hatchInputFix = &ls.CodeFixProvider{
	Source:     diagnosticSource,
	ErrorCodes: []int32{EK4405.Code},
	GetCodeActions: func(ctx context.Context, fix *ls.CodeFixContext) ([]*ls.CodeAction, error) {
		file := fix.SourceFile
		var call *ast.Node
		for n := astnav.GetTouchingPropertyName(file, fix.Span.Pos()); n != nil; n = n.Parent {
			if isHatchArgument(n) {
				call = n.Parent
				break
			}
		}
		if call == nil {
			return nil, nil
		}
		c, done := fix.Program.GetTypeCheckerForFile(ctx, file)
		defer done()
		names := outsideNames(c, hatchIsland(call))
		if len(names) == 0 {
			return nil, nil
		}
		list := strings.Join(names, ", ")
		edits := hatchInputEdits(c, fix, file, call, list)
		arguments := call.Arguments()
		edits = append(edits, edit(fix, file, arguments[len(arguments)-1].End(), ", "+list))
		description := "Pass " + list + " as an input"
		if len(names) > 1 {
			description += "s"
		}
		return []*ls.CodeAction{{Description: description, Changes: edits}}, nil
	},
}

// hatchInputEdits adds parameters named list to the hatch's TypeScript, or,
// for an expression, makes it a function of them.
func hatchInputEdits(c *checker.Checker, fix *ls.CodeFixContext, file *ast.SourceFile, call *ast.Node, list string) []*lsproto.TextEdit {
	rangeOf := func(start, end int) lsproto.Range { return fix.LS.RangeOf(file, core.NewTextRange(start, end)) }
	island := call.Arguments()[0]
	text := file.Text()
	start := scanner.SkipTrivia(text, island.Pos())
	function := ast.SkipParentheses(island)
	if !ast.IsFunctionExpressionOrArrowFunction(function) {
		replacement := "(" + list + ") => " + text[start:island.End()]
		if len(c.GetSignaturesOfType(c.GetTypeAtLocation(island), checker.SignatureKindCall)) > 0 {
			replacement += "()" // a function value: call it
		}
		return []*lsproto.TextEdit{{NewText: replacement, Range: rangeOf(start, island.End())}}
	}
	parameters := function.Parameters()
	existing := len(hatchInputs(c, call))
	if len(parameters) > 0 && !parenthesized(text, parameters[0].Pos()) {
		// x => …: a lone parameter without parentheses.
		first := scanner.SkipTrivia(text, parameters[0].Pos())
		inner := list + ", " + text[first:parameters[0].End()]
		if existing > 0 {
			inner = text[first:parameters[0].End()] + ", " + list
		}
		return []*lsproto.TextEdit{{NewText: "(" + inner + ")", Range: rangeOf(first, parameters[0].End())}}
	}
	switch {
	case existing < len(parameters):
		at := scanner.SkipTrivia(text, parameters[existing].Pos())
		return []*lsproto.TextEdit{{NewText: list + ", ", Range: rangeOf(at, at)}}
	case len(parameters) > 0:
		at := parameters[len(parameters)-1].End()
		return []*lsproto.TextEdit{{NewText: ", " + list, Range: rangeOf(at, at)}}
	default:
		open := strings.IndexByte(text[function.Pos():function.End()], '(')
		if open < 0 {
			return nil
		}
		at := function.Pos() + open + 1
		return []*lsproto.TextEdit{{NewText: list, Range: rangeOf(at, at)}}
	}
}

// parenthesized reports whether the parameter at pos follows an open paren.
func parenthesized(text string, pos int) bool {
	for i := pos - 1; i >= 0; i-- {
		switch text[i] {
		case ' ', '\t', '\n', '\r':
			continue
		case '(':
			return true
		}
		return false
	}
	return false
}

// edit inserts text at pos.
func edit(fix *ls.CodeFixContext, file *ast.SourceFile, pos int, text string) *lsproto.TextEdit {
	return &lsproto.TextEdit{NewText: text, Range: fix.LS.RangeOf(file, core.NewTextRange(pos, pos))}
}
