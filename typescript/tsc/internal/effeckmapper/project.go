// Package effeckmapper gives stock TypeScript 7 a view of Effeck (.effeck)
// files. Project turns one file's text into virtual TypeScript plus a span
// map back to the original; Serve answers TypeScript's content mapper
// protocol (see internal/contentmapper) with that projection.
//
// Project is the only definition of what an .effeck file means to a host
// that is not the Effeck fork: tsc, the TypeScript 7 language server, and
// later a bundler or a Volar shim all call it. It is a pure function of the
// file's text, because TypeScript caches a transform by the file's content
// hash and never tells a mapper about other files.
//
// The lowering rules here are a walking skeleton:
//
//	error NotFound { id: string }   class NotFound extends $TaggedError("NotFound")<{ id: string }> {}
//	User!NotFound!Db                Effect.Effect<User, NotFound, Db>
//	function f(p) { body }          function f(p) { return $gen(function* () { body }) }
//	c(...)  in a function body      (yield* $(c(...)))
//	throw x in a function body      return yield* $fail(x)
//
// Everything else passes through as verbatim text. The names $gen, $,
// $fail, $TaggedError and Effect come from the prelude, prelude.d.ts in the
// effeck-ts-mapper package, which every projected file imports.
package effeckmapper

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/parser"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
	"github.com/microsoft/TypeScript/tsc/internal/spanmap"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

// PreludeModule is the module every projected file imports its helpers from.
const PreludeModule = "effeck-ts-mapper/prelude"

// preludeImport is the first line of every projected file. Importing per file,
// instead of declaring globals, keeps the helpers out of .ts files and lets
// TypeScript drop unused-import diagnostics in synthesized code.
const preludeImport = `import { $gen, $, $fail, $TaggedError } from "` + PreludeModule + `";` + "\n"

// effectPreludeImport is the first line of a projected file that does not
// bind the name Effect itself. Effect is a prelude name in Effeck (D4), and
// with it in scope TypeScript prints Effect types as Effect.Effect<A, E, R>,
// as the fork's declaration files do.
const effectPreludeImport = `import { $gen, $, $fail, $TaggedError, Effect } from "` + PreludeModule + `";` + "\n"

// The lowering's synthesized text. Each run of a call is wrapped in runOpen
// and runClose; a function body is wrapped in bodyOpen and bodyClose.
const (
	runOpen   = "(yield* $("
	runClose  = "))"
	bodyOpen  = " return $gen(function* () {"
	bodyClose = "})"
	failOpen  = "return yield* $fail("
	failClose = ")"
	// A contract names Effect's type through the prelude's Effect, or, in a
	// file that binds Effect to something of its own, through an import type.
	// TypeScript prints a written annotation as written, so this text is
	// what a hover on an annotated function shows.
	effectTypeOpen         = "Effect.Effect<"
	importedEffectTypeOpen = `import("effect").Effect.Effect<`
)

// Diagnostic codes the projector reports, under the source EK.
const (
	// CodeUnprojectedForm marks Effeck syntax the projector does not lower yet.
	CodeUnprojectedForm = 9001
)

// projectedFileName is the name the parser sees. The parser reads Effeck
// syntax only in files whose name ends in .effeck; the name itself never
// reaches the output, so the projection stays a function of the text alone.
var projectedFileName = tspath.RootedFilePathFromAbsolute("/projected.effeck")

// Projection is the virtual TypeScript for one .effeck file.
type Projection struct {
	// Text is the virtual TypeScript source.
	Text string
	// Segments map Text back to the original, ordered by virtual start. Text
	// outside every segment is synthesized.
	Segments []spanmap.Segment
	// Diagnostics are the projector's own errors, in original coordinates.
	Diagnostics []Diagnostic
}

// Diagnostic is an error the projector reports about the original text.
type Diagnostic struct {
	Start   int
	Length  int
	Code    int32
	Message string
}

// edit replaces the original text in [pos, end) with text. An edit with
// pos == end inserts text before the original character at pos.
type edit struct {
	pos  int
	end  int
	text string
}

// projector holds the state of one Project call.
type projector struct {
	file  *ast.SourceFile
	text  string
	edits []edit
	// statementStarts holds the start of each expression statement in an
	// Effect body, so a run that begins a statement can guard against ASI.
	statementStarts map[int]bool
	// statement is the top-level statement being walked; reported records
	// which statements already have an EK9001, so each gets at most one.
	statement   *ast.Node
	reported    map[*ast.Node]bool
	diagnostics []Diagnostic
	// effectTypeOpen opens a projected contract: effectTypeOpen, or
	// importedEffectTypeOpen when the file binds Effect itself.
	effectTypeOpen string
}

// Project returns the virtual TypeScript for the .effeck file whose text is
// text. It never fails: text it cannot lower passes through verbatim, and
// Effeck syntax it does not lower yet draws an EK9001 diagnostic.
func Project(text string) Projection {
	file := parser.ParseSourceFile(ast.SourceFileParseOptions{FileName: projectedFileName}, text, core.ScriptKindTS)
	p := &projector{
		file:            file,
		text:            text,
		statementStarts: map[int]bool{},
		reported:        map[*ast.Node]bool{},
		effectTypeOpen:  effectTypeOpen,
	}
	if bindsTopLevel(file, "Effect") {
		p.effectTypeOpen = importedEffectTypeOpen
		p.insert(0, preludeImport)
	} else {
		p.insert(0, effectPreludeImport)
	}
	for _, statement := range file.Statements.Nodes {
		p.statement = statement
		p.visit(statement, false /*inEffect*/)
	}
	return p.splice()
}

// visit walks node and records the edits that lower it. inEffect reports
// whether node is inside an Effect body, a projected function's generator,
// where calls run and throw fails.
func (p *projector) visit(node *ast.Node, inEffect bool) {
	switch {
	case isErrorDeclaration(node):
		p.errorDeclaration(node)
		return
	case isEffectType(node):
		p.effectType(node, inEffect)
		return
	case isProjectedFunction(node):
		p.functionDeclaration(node)
		return
	case node.Flags&ast.NodeFlagsReparsed != 0:
		p.unprojected(node)
		return
	}
	if inEffect {
		switch node.Kind {
		case ast.KindExpressionStatement:
			p.statementStarts[p.tokenStart(node)] = true
		case ast.KindCallExpression:
			if isRunnableCall(node) {
				p.runCall(node)
				return
			}
		case ast.KindThrowStatement:
			if node.Expression() != nil {
				p.throwStatement(node)
				return
			}
		}
		// A nested function or class body is not part of the generator, so
		// yield* is not valid there. Only nested function declarations are
		// projected, as Effects of their own.
		if ast.IsFunctionLike(node) || ast.IsClassLike(node) {
			inEffect = false
		}
	}
	node.ForEachChild(func(child *ast.Node) bool {
		p.visit(child, inEffect)
		return false
	})
}

// isErrorDeclaration reports whether node is an `error Name { ... }`
// declaration, which the parser reads as a class whose base is a synthesized
// TaggedError("Name") call.
func isErrorDeclaration(node *ast.Node) bool {
	if node.Kind != ast.KindClassDeclaration || node.Flags&ast.NodeFlagsReparsed != 0 {
		return false
	}
	base := errorBase(node)
	if base == nil {
		return false
	}
	call := base.Expression()
	if call.Kind != ast.KindCallExpression || call.Flags&ast.NodeFlagsReparsed == 0 {
		return false
	}
	callee := call.Expression()
	return callee.Kind == ast.KindIdentifier && callee.Text() == "TaggedError" && len(base.TypeArguments()) == 1
}

// errorBase returns the single extends element of class, or nil.
func errorBase(class *ast.Node) *ast.Node {
	clauses := class.AsClassDeclaration().HeritageClauses
	if clauses == nil || len(clauses.Nodes) != 1 {
		return nil
	}
	types := clauses.Nodes[0].AsHeritageClause().Types
	if types == nil || len(types.Nodes) != 1 {
		return nil
	}
	return types.Nodes[0]
}

// errorDeclaration lowers `error Name { fields }` to the class it
// abbreviates, `class Name extends $TaggedError("Name")<{ fields }> {}`.
// The fields stay verbatim. TaggedError's instance type is Readonly of them,
// which gives the readonly fields the Effeck form promises.
func (p *projector) errorDeclaration(node *ast.Node) {
	class := node.AsClassDeclaration()
	name := class.Name()
	// The name's leading trivia starts right after the error keyword.
	keywordEnd := name.Pos()
	keywordStart := keywordEnd - len("error")
	if keywordStart < 0 || p.text[keywordStart:keywordEnd] != "error" {
		p.unprojected(node)
		return
	}
	p.replace(keywordStart, keywordEnd, "class")
	head := name.End()
	if class.TypeParameters != nil {
		p.visitList(class.TypeParameters.Nodes, false /*inEffect*/)
		closing := scanner.SkipTrivia(p.text, class.TypeParameters.End())
		if closing >= len(p.text) || p.text[closing] != '>' {
			p.unprojected(node)
			return
		}
		head = closing + 1
	}
	base := "extends $TaggedError(" + strconv.Quote(name.Text()) + ")<"
	fields := errorBase(node).TypeArguments()[0]
	if fields.Flags&ast.NodeFlagsReparsed != 0 {
		// error Timeout: no fields were written.
		p.insert(head, " "+base+"{}> {}")
		return
	}
	p.insert(p.tokenStart(fields), base)
	p.visit(fields, false /*inEffect*/)
	p.insert(fields.End(), "> {}")
}

// isEffectType reports whether node is an Effect type written in the !
// form, which the parser reads as a reference to a synthesized
// Effect.Effect with three type arguments.
func isEffectType(node *ast.Node) bool {
	if node.Kind != ast.KindTypeReference || node.Flags&ast.NodeFlagsReparsed != 0 {
		return false
	}
	name := node.AsTypeReferenceNode().TypeName
	return name.Kind == ast.KindQualifiedName && name.Flags&ast.NodeFlagsReparsed != 0 && len(node.TypeArguments()) == 3
}

// effectType lowers `A!E!R` to `Effect.Effect<A, E, R>`. Each ! becomes a
// comma, and an empty or omitted slot becomes never. Dependencies omitted
// from a function's return annotation also become never; the fork infers
// them from the body (D57), which the skeleton does not do.
func (p *projector) effectType(node *ast.Node, inEffect bool) {
	slots := node.TypeArguments()
	result := slots[0]
	p.insert(p.tokenStart(result), p.effectTypeOpen)
	p.visit(result, inEffect)
	previous := result
	for _, slot := range slots[1:] {
		bang := scanner.SkipTrivia(p.text, previous.End())
		hasBang := bang < len(p.text) && p.text[bang] == '!'
		switch {
		case slot.Flags&ast.NodeFlagsReparsed == 0 && hasBang:
			// A written slot: User!NotFound.
			p.replace(bang, bang+1, ", ")
			p.visit(slot, inEffect)
		case slot.Flags&ast.NodeFlagsReparsed != 0 && hasBang && slot.Pos() == bang:
			// An empty slot stands on its own !: User!!Db.
			p.replace(bang, bang+1, ", never")
		case slot.Flags&ast.NodeFlagsReparsed != 0:
			// Omitted dependencies stand on the errors' last character: User!NotFound.
			p.insert(previous.End(), ", never")
		default:
			p.unprojected(slot)
		}
		previous = slot
	}
	p.insert(node.End(), ">")
}

// isProjectedFunction reports whether node is a function declaration the
// skeleton turns into an Effect: one with a braced body that is neither
// async nor a generator.
func isProjectedFunction(node *ast.Node) bool {
	if node.Kind != ast.KindFunctionDeclaration || node.Flags&ast.NodeFlagsReparsed != 0 {
		return false
	}
	fn := node.AsFunctionDeclaration()
	return fn.Body != nil && fn.AsteriskToken == nil && !ast.HasSyntacticModifier(node, ast.ModifierFlagsAsync)
}

// functionDeclaration lowers `function f(p) { body }` to
// `function f(p) { return $gen(function* () { body }) }`, so stock
// TypeScript infers f's Effect<A, E, R> from the generator. The signature is
// walked outside the Effect body; the body's statements inside it.
func (p *projector) functionDeclaration(node *ast.Node) {
	body := node.AsFunctionDeclaration().Body
	open := p.tokenStart(body)
	closing := body.End() - 1
	if open >= len(p.text) || p.text[open] != '{' || closing <= open || p.text[closing] != '}' {
		// An unterminated body; TypeScript reports the syntax error itself.
		return
	}
	node.ForEachChild(func(child *ast.Node) bool {
		if child != body {
			p.visit(child, false /*inEffect*/)
		}
		return false
	})
	p.insert(open+1, bodyOpen)
	p.visitList(body.Statements(), true /*inEffect*/)
	p.insert(closing, bodyClose)
}

// isRunnableCall reports whether call runs where it stands. Synthesized
// calls belong to Effeck forms the skeleton does not lower, and import()
// and super() are not calls of a function.
func isRunnableCall(call *ast.Node) bool {
	if call.Flags&ast.NodeFlagsReparsed != 0 {
		return false
	}
	callee := call.Expression().Kind
	return callee != ast.KindImportKeyword && callee != ast.KindSuperKeyword
}

// runCall lowers a call c in an Effect body to `(yield* $(c))`: $ lifts a
// value that is not an Effect to one, so every call runs and has its success
// type. A run that begins an expression statement starts with a semicolon,
// because Effeck code omits semicolons and a line that starts with ( would
// otherwise continue the previous one.
func (p *projector) runCall(call *ast.Node) {
	start := p.tokenStart(call)
	open := runOpen
	if p.statementStarts[start] {
		open = ";" + open
		delete(p.statementStarts, start)
	}
	p.insert(start, open)
	call.ForEachChild(func(child *ast.Node) bool {
		p.visit(child, true /*inEffect*/)
		return false
	})
	p.insert(call.End(), runClose)
}

// throwStatement lowers `throw x` in an Effect body to
// `return yield* $fail(x)`, which adds x's type to the function's errors.
func (p *projector) throwStatement(node *ast.Node) {
	start := p.tokenStart(node)
	if p.text[start:min(start+len("throw"), len(p.text))] != "throw" {
		p.unprojected(node)
		return
	}
	expression := node.Expression()
	// Replace the space after throw too: `$fail(x)`, not `$fail( x)`.
	p.replace(start, p.tokenStart(expression), failOpen)
	p.visit(expression, true /*inEffect*/)
	p.insert(expression.End(), failClose)
}

// unprojected records that node, a node the parser synthesized for an Effeck
// form, has no lowering here. Its source passes through unchanged, so stock
// TypeScript will usually report a syntax error there too; EK9001 says why.
// Synthesized readonly modifiers (D134) are dropped silently: the virtual
// members are mutable, which loosens types but reports nothing false.
func (p *projector) unprojected(node *ast.Node) {
	if node.Kind == ast.KindReadonlyKeyword || p.reported[p.statement] {
		return
	}
	p.reported[p.statement] = true
	start := scanner.SkipTrivia(p.text, node.Pos())
	end := max(node.End(), start)
	p.diagnostics = append(p.diagnostics, Diagnostic{
		Start:   start,
		Length:  end - start,
		Code:    CodeUnprojectedForm,
		Message: "The TypeScript 7 mapper does not lower this Effeck form yet; TypeScript sees its source text unchanged.",
	})
}

// bindsTopLevel reports whether file declares or imports name at its top
// level, so a prelude import of name would collide with it.
func bindsTopLevel(file *ast.SourceFile, name string) bool {
	named := func(node *ast.Node) bool {
		return node != nil && node.Kind == ast.KindIdentifier && node.Text() == name
	}
	for _, statement := range file.Statements.Nodes {
		switch statement.Kind {
		case ast.KindImportDeclaration:
			clause := statement.AsImportDeclaration().ImportClause
			if clause == nil {
				continue
			}
			if named(clause.Name()) {
				return true
			}
			bindings := clause.AsImportClause().NamedBindings
			switch {
			case bindings == nil:
			case bindings.Kind == ast.KindNamespaceImport:
				if named(bindings.Name()) {
					return true
				}
			case bindings.Kind == ast.KindNamedImports:
				for _, element := range bindings.AsNamedImports().Elements.Nodes {
					if named(element.Name()) {
						return true
					}
				}
			}
		case ast.KindImportEqualsDeclaration:
			if named(statement.Name()) {
				return true
			}
		case ast.KindVariableStatement:
			for _, declaration := range statement.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes {
				if named(declaration.Name()) {
					return true
				}
			}
		case ast.KindFunctionDeclaration, ast.KindClassDeclaration, ast.KindInterfaceDeclaration,
			ast.KindTypeAliasDeclaration, ast.KindEnumDeclaration, ast.KindModuleDeclaration:
			if named(statement.Name()) {
				return true
			}
		}
	}
	return false
}

// visitList walks nodes in order.
func (p *projector) visitList(nodes []*ast.Node, inEffect bool) {
	for _, node := range nodes {
		p.visit(node, inEffect)
	}
}

// tokenStart returns the position of node's first token, after its leading
// trivia.
func (p *projector) tokenStart(node *ast.Node) int {
	return scanner.GetTokenPosOfNode(node, p.file, false /*includeJSDoc*/)
}

// insert records text to insert before the original character at pos.
func (p *projector) insert(pos int, text string) {
	p.edits = append(p.edits, edit{pos: pos, end: pos, text: text})
}

// replace records text to replace the original text in [pos, end).
func (p *projector) replace(pos int, end int, text string) {
	p.edits = append(p.edits, edit{pos: pos, end: end, text: text})
}

// splice applies the recorded edits to the original text. Edits are sorted
// by position, stably, so edits at one position apply in the order the walk
// recorded them: an outer run opens before an inner one and closes after it.
// Each stretch of original text between edits is copied verbatim and gets a
// Verbatim segment with every language feature; inserted text is
// synthesized and gets none.
func (p *projector) splice() Projection {
	edits := slices.Clone(p.edits)
	slices.SortStableFunc(edits, func(a, b edit) int { return cmp.Compare(a.pos, b.pos) })
	var out strings.Builder
	var segments []spanmap.Segment
	cursor := 0
	for _, e := range edits {
		if e.pos < cursor {
			// Overlapping edits are a projector bug; pass the file through
			// unchanged rather than send TypeScript a broken map.
			return p.verbatim(Diagnostic{
				Start:   e.pos,
				Length:  0,
				Code:    CodeUnprojectedForm,
				Message: "The TypeScript 7 mapper produced overlapping edits here; the file is passed through unchanged.",
			})
		}
		segments = appendVerbatim(segments, &out, p.text, cursor, e.pos)
		out.WriteString(e.text)
		cursor = e.end
	}
	segments = appendVerbatim(segments, &out, p.text, cursor, len(p.text))
	return Projection{Text: out.String(), Segments: segments, Diagnostics: p.diagnostics}
}

// verbatim returns the identity projection of the original text, with
// diagnostic and the diagnostics recorded so far.
func (p *projector) verbatim(diagnostic Diagnostic) Projection {
	var out strings.Builder
	segments := appendVerbatim(nil, &out, p.text, 0, len(p.text))
	return Projection{Text: out.String(), Segments: segments, Diagnostics: append(p.diagnostics, diagnostic)}
}

// appendVerbatim copies original[from:to] to out and appends its Verbatim
// segment, merged into the previous segment when the two are contiguous in
// both texts.
func appendVerbatim(segments []spanmap.Segment, out *strings.Builder, original string, from int, to int) []spanmap.Segment {
	if to <= from {
		return segments
	}
	virtualStart := core.TextPos(out.Len())
	out.WriteString(original[from:to])
	if n := len(segments); n > 0 && segments[n-1].VirtualEnd == virtualStart && segments[n-1].OriginalEnd == core.TextPos(from) {
		segments[n-1].VirtualEnd = core.TextPos(out.Len())
		segments[n-1].OriginalEnd = core.TextPos(to)
		return segments
	}
	return append(segments, spanmap.Segment{
		VirtualStart:  virtualStart,
		VirtualEnd:    core.TextPos(out.Len()),
		OriginalStart: core.TextPos(from),
		OriginalEnd:   core.TextPos(to),
		Kind:          spanmap.KindVerbatim,
		Features:      spanmap.FeatureAll,
	})
}
