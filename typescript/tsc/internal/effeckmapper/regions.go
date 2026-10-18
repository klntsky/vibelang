package effeckmapper

import (
	"slices"
	"strconv"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// Regions (rule 6: completion records across regions). A region is a block
// of an Effect function that runs as an Effect of its own, so that a runtime
// helper can wrap it: the blocks of a try statement ($catch, $finally), a
// provide block (Effect.provide), and the body and clauses of a handle
// statement (handle). As the fork lowers it (internal/effeck/regions.go), a
// region is a generator, and a return, break, or continue that leaves it
// returns a completion record instead:
//
//	try { return find(id) } catch (e) { return guest }
//	const $c = yield* $catch($gen(function* () { return { kind: "return" as const, value: (yield* $run(find(id))) } }),
//	  (e) => $gen(function* () { return { kind: "return" as const, value: guest } })); return $c.value
//
// The generator that runs the region dispatches on the record: it returns,
// breaks, or continues itself, or, when it is a region's generator too and
// the jump leaves it as well, it returns the record to its own runner. A
// block that can end normally ends with $end when some jump leaves its
// region; a region that no jump leaves has no record to dispatch. The
// checking rendering returns a region's values boxed (prelude.d.ts, $box),
// so they keep the types TypeScript infers for the source's returns.

// A jump is where a return, break, or continue statement goes.
type jump struct {
	kind  ast.Kind // ast.KindReturnStatement, ast.KindBreakStatement, or ast.KindContinueStatement
	label string   // the target label; "" for return and for an unlabeled break or continue
}

// region collects the jumps that leave a region while the walk lowers its blocks.
type region struct {
	jumps []jump
	// tail reports that the region's statement ends its Effect function's
	// own body (isTailRegion).
	tail bool
}

// isTailRegion reports whether node, a region statement, is the last
// statement of an Effect function's own body. After it the function ends,
// so the region's blocks may fall off their ends: the generator's return
// type then holds undefined exactly where TypeScript finds the end
// reachable, an exhaustive switch included, and the dispatch returns the
// completion unconditionally, as the function's own end would.
func isTailRegion(node *ast.Node, e env) bool {
	if e.frame == nil || e.frame.region != nil || len(e.frame.targets) != 0 {
		return false
	}
	block := node.Parent
	if block == nil || block.Kind != ast.KindBlock || block.Parent == nil || !ast.IsFunctionLike(block.Parent) {
		return false
	}
	statements := block.Statements()
	return len(statements) > 0 && statements[len(statements)-1] == node
}

func (r *region) add(j jump) {
	if !slices.Contains(r.jumps, j) {
		r.jumps = append(r.jumps, j)
	}
}

// A frame is one generator the projection writes: an Effect function's own
// body, or a region's block.
type frame struct {
	region  *region  // the region whose block this is; nil for an Effect function's body
	targets []target // the statements a break or continue can jump to, innermost last
	// expression reports that the generator's body is one expression, a
	// lambda's, which no block holds.
	expression bool
}

// A target is a statement that break or continue can jump to: a loop, a
// switch, or a labeled statement.
type target struct {
	label string // the label of a labeled statement; "" for a loop or a switch
	loop  bool   // continue can jump to it
}

// newEffectEnv returns the env of a new Effect body: a generator of its own.
func newEffectEnv() env {
	return env{effect: true, frame: &frame{}}
}

// regionEnv returns the env of one block of region r, inside the code of e.
func regionEnv(r *region, e env) env {
	return env{effect: true, frame: &frame{region: r}, continuation: e.continuation, voidFail: e.voidFail, producer: e.producer}
}

// lands reports whether j lands in f, so that f can jump itself instead of
// returning a completion record.
func (f *frame) lands(j jump) bool {
	if j.kind == ast.KindReturnStatement {
		return f.region == nil
	}
	for i := len(f.targets) - 1; i >= 0; i-- {
		t := f.targets[i]
		switch {
		case j.label != "":
			if t.label == j.label {
				return true
			}
		case t.label == "" && (t.loop || j.kind == ast.KindBreakStatement):
			return true
		}
	}
	return false
}

// isJumpTarget reports whether node is a statement that break or continue
// can jump to.
func isJumpTarget(node *ast.Node) bool {
	return node.Kind == ast.KindLabeledStatement || node.Kind == ast.KindSwitchStatement || ast.IsIterationStatement(node, false /*lookInLabeledStatements*/)
}

// jumpTarget walks node, a loop, switch, or labeled statement in an Effect
// body, with node open as a jump target of e's frame.
func (p *projector) jumpTarget(node *ast.Node, e env) {
	t := target{loop: ast.IsIterationStatement(node, false /*lookInLabeledStatements*/)}
	if node.Kind == ast.KindLabeledStatement {
		statement := node.AsLabeledStatement()
		t = target{label: statement.Label.Text(), loop: ast.IsIterationStatement(statement.Statement, true /*lookInLabeledStatements*/)}
	}
	f := e.frame
	f.targets = append(f.targets, t)
	p.visitChildren(node, e)
	f.targets = f.targets[:len(f.targets)-1]
}

// jumpKind is the kind a jump's completion takes: "break:label" or
// "continue:label".
func jumpKind(j jump) string {
	if j.kind == ast.KindBreakStatement {
		return "break:" + j.label
	}
	return "continue:" + j.label
}

// returnStatement records a return in a region's block as a jump that
// leaves the region. In the checking rendering the region's generator
// returns the value boxed, `return $box(x)`, which the dispatch narrows
// back to the value (prelude.d.ts); a return without a value stays. The
// executable rendering will return the record { kind: "return", value }.
func (p *projector) returnStatement(node *ast.Node, e env) {
	e.frame.region.add(jump{kind: ast.KindReturnStatement})
	start := p.tokenStart(node)
	expression := node.Expression()
	if expression == nil {
		p.plan.mark(editRecord, partOpen, start, start, "return")
		p.plan.mark(editRecord, partClose, node.End(), node.End(), "return")
		return
	}
	from := start
	if p.isCompletionReturn(node) {
		// A do expression's completion value: no return keyword to replace.
		from = p.tokenStart(expression)
	}
	p.plan.mark(editRecord, partOpen, from, p.tokenStart(expression), "return value")
	p.visit(expression, e)
	p.plan.mark(editRecord, partClose, expression.End(), expression.End(), "return value")
}

// jumpStatement lowers a break or continue that leaves the region of e's
// frame to the completion of that jump, `return $jump("break:label")`; one
// that lands in the frame stays.
func (p *projector) jumpStatement(node *ast.Node, e env) {
	j := jump{kind: node.Kind}
	if label := node.Label(); label != nil {
		j.label = label.Text()
	}
	if e.frame.lands(j) || e.frame.region == nil {
		return
	}
	e.frame.region.add(j)
	p.plan.markText(editRecord, partOpen, p.tokenStart(node), node.End(), "return "+helperRef("$jump")+"("+strconv.Quote(jumpKind(j))+")")
}

// openRegion returns the text that starts a region statement whose record
// is held in name, or "" when no jump leaves r: `const $c = yield* `.
func openRegion(r *region, name string) string {
	if len(r.jumps) == 0 {
		return "yield* "
	}
	return "const " + name + " = yield* "
}

// dispatch returns the statements that act on the completion in name
// after a region statement in e's frame: each jump that lands in the frame
// is taken, and each that does not is passed on, as the frame's own
// completion. When the statement cannot end normally, the last one is
// unconditional.
func (p *projector) dispatch(r *region, name string, normal bool, e env) string {
	if r.tail {
		normal = false
	}
	var out []string
	for i, j := range r.jumps {
		var action string
		switch {
		case !e.frame.lands(j):
			// Passed on to the enclosing region. A narrowed return value
			// is boxed again, so the enclosing generator does not widen it.
			e.frame.region.add(j)
			action = "return " + name
			if j.kind == ast.KindReturnStatement && (normal || i < len(r.jumps)-1) {
				action = "return " + helperRef("$box") + "(" + name + ")"
			}
		case j.kind == ast.KindReturnStatement:
			action = "return " + name
			if !normal && i == len(r.jumps)-1 {
				action = "return " + helperRef("$unboxed") + "(" + name + ")"
			}
		case j.kind == ast.KindBreakStatement:
			action = strings.TrimSpace("break " + j.label)
		default:
			action = strings.TrimSpace("continue " + j.label)
		}
		if !normal && i == len(r.jumps)-1 {
			out = append(out, action)
			continue
		}
		condition := helperRef("$returned") + "(" + name + ")"
		if j.kind != ast.KindReturnStatement {
			condition = helperRef("$jumped") + "(" + name + ", " + strconv.Quote(jumpKind(j)) + ")"
		}
		out = append(out, "if ("+condition+") "+action)
	}
	if len(out) == 0 {
		return ""
	}
	return "; " + strings.Join(out, "; ")
}

// regionBlock walks block, a braced block of region r, in a frame of its
// own, and returns whether it can end normally. When r ends up with jumps,
// a block that can end normally returns $end.
func (p *projector) regionBlock(block *ast.Node, statements []*ast.Node, r *region, e env) (normal bool, closeNormal func()) {
	p.visitList(statements, regionEnv(r, e))
	normal = canCompleteNormally(statements)
	closing := block.End() - 1
	return normal, func() {
		if normal && len(r.jumps) > 0 && !r.tail && closing >= 0 && p.text[closing] == '}' {
			p.plan.markText(editRecord, partClose, closing, closing, "; return "+helperRef("$end")+" ")
		}
	}
}

// freshCompletion returns a name for the completion record of a region
// statement that no word of the file and no other region uses.
func (p *projector) freshCompletion() string {
	for {
		p.completions++
		name := "$c"
		if p.completions > 1 {
			name += "_" + strconv.Itoa(p.completions-1)
		}
		if !p.names.words[name] {
			return name
		}
	}
}

// generatorOpen is the program edit text that opens a region block's
// generator, with this passed in when the block refers to it.
func generatorOpen(block *ast.Node) string {
	if usesThis(block) {
		return helperRef("$gen") + "({ self: this }, function* () "
	}
	return helperRef("$gen") + "(function* () "
}

// tryStatement lowers a try statement in an Effect body to its region:
// `$catch(try, (e) => catch)` for a catch clause and `$finally(try, finally)`
// for a finally block, each block a generator. A typed catch clause, the
// retired form (L4), and a clause typed Cause (D176) are not lowered yet.
func (p *projector) tryStatement(node *ast.Node, e env) {
	statement := node.AsTryStatement()
	tryBlock, catch, finally := statement.TryBlock, statement.CatchClause, statement.FinallyBlock
	if catch != nil {
		if variable := catch.AsCatchClause().VariableDeclaration; variable != nil &&
			(variable.Type() != nil || variable.Flags&ast.NodeFlagsReparsed != 0 || variable.Name().Flags&ast.NodeFlagsReparsed != 0) {
			p.unprojected(variable)
			p.visitChildren(node, e)
			return
		}
	}
	r := &region{tail: isTailRegion(node, e)}
	name := p.freshCompletion()
	tryNormal, closeTry := p.regionBlock(tryBlock, tryBlock.Statements(), r, e)
	normal := tryNormal
	var closeCatch, closeFinally func()
	var catchBlock *ast.Node
	if catch != nil {
		catchBlock = catch.AsCatchClause().Block
		catchNormal, closer := p.regionBlock(catchBlock, catchBlock.Statements(), r, e)
		closeCatch = closer
		normal = tryNormal || catchNormal
	}
	exit := (*ast.Node)(nil)
	if finally != nil {
		// The finalizer's value is dropped; a jump out of it is a record too.
		statements := finally.Statements()
		if exit = exitBinding(finally); exit != nil {
			statements = statements[1:]
		}
		finallyNormal, closer := p.regionBlock(finally, statements, r, e)
		closeFinally = closer
		normal = normal && finallyNormal
	}
	closeTry()
	if closeCatch != nil {
		closeCatch()
	}
	if closeFinally != nil {
		closeFinally()
	}

	open := openRegion(r, name)
	if finally != nil {
		if exit != nil {
			open += helperRef("$finallyExit") + "("
		} else {
			open += helperRef("$finally") + "("
		}
	}
	if catch != nil {
		open += helperRef("$catch") + "("
	}
	p.plan.markText(editRegion, partOpen, p.tokenStart(node), p.tokenStart(tryBlock), open+generatorOpen(tryBlock))
	after := tryBlock.End()
	if catch != nil {
		clause := catch.AsCatchClause()
		if variable := clause.VariableDeclaration; variable != nil {
			p.plan.markText(editRegion, partMiddle, after, p.tokenStart(variable), "), (")
			p.visit(variable, env{})
			p.plan.markText(editRegion, partMiddle, variable.End(), p.tokenStart(catchBlock), ") => "+generatorOpen(catchBlock))
		} else {
			p.plan.markText(editRegion, partMiddle, after, p.tokenStart(catchBlock), "), () => "+generatorOpen(catchBlock))
		}
		after = catchBlock.End()
		p.plan.markText(editRegion, partMiddle, after, after, "))")
	} else {
		p.plan.markText(editRegion, partMiddle, after, after, ")")
	}
	if finally != nil && exit != nil {
		// finally (exit) { ... }: the finalizer receives the try's Exit.
		brace := p.braceAfter(exit.End())
		p.plan.markText(editRegion, partMiddle, after, p.tokenStart(exit), ", (")
		p.visit(exit, env{})
		p.plan.markText(editRegion, partMiddle, exit.End(), brace, ") => "+generatorOpen(finally))
		after = finally.End()
		p.plan.markText(editRegion, partMiddle, after, after, "))")
	} else if finally != nil {
		p.plan.markText(editRegion, partMiddle, after, p.tokenStart(finally), ", "+generatorOpen(finally))
		after = finally.End()
		p.plan.markText(editRegion, partMiddle, after, after, "))")
	}
	p.plan.markText(editDispatch, partClose, after, after, p.dispatch(r, name, normal, e))
}

// isProvideBlock reports whether node is a provide block (D130, D172): a
// block flagged NodeFlagsEffeck whose first statement, flagged too, holds
// the providers.
func isProvideBlock(node *ast.Node) bool {
	if node.Kind != ast.KindBlock || node.Flags&ast.NodeFlagsEffeck == 0 {
		return false
	}
	statements := node.Statements()
	return len(statements) > 0 && statements[0].Kind == ast.KindExpressionStatement && statements[0].Flags&ast.NodeFlagsEffeck != 0
}

// provideBlock lowers `provide Layers { statements }` in an Effect body to
// its region, `Effect.provide(Layers)(program)`, which keeps the source order:
// the providers come first. Several providers become an array, and a
// provider written Key = value the layer $provideService(Key, "Key", value)
// that provides that one service (D173, D175).
func (p *projector) provideBlock(node *ast.Node, e env) {
	statements := node.Statements()
	providers := statements[0].Expression()
	items := []*ast.Node{providers}
	array := false
	if providers.Kind == ast.KindArrayLiteralExpression && providers.Flags&ast.NodeFlagsReparsed == 0 && !p.isWrittenArray(providers) {
		items = providers.Elements()
		array = true
	} else if isProvideService(providers) {
		array = true
	}
	if len(items) == 0 {
		p.unprojected(node)
		return
	}
	first, last := items[0], items[len(items)-1]
	brace := p.braceAfter(p.lastTokenEnd(last))
	closing := node.End() - 1
	if brace < 0 || closing <= brace || p.text[closing] != '}' {
		p.unprojected(node)
		return
	}
	r := &region{tail: isTailRegion(node, e)}
	name := p.freshCompletion()
	body := statements[1:]
	p.visitList(body, regionEnv(r, e))
	normal := canCompleteNormally(body)
	if normal && len(r.jumps) > 0 && !r.tail {
		p.plan.markText(editRecord, partClose, closing, closing, "; return "+helperRef("$end")+" ")
	}

	open := openRegion(r, name) + p.names.prelude("Effect") + ".provide("
	if array {
		open += "["
	}
	p.plan.markText(editRegion, partOpen, p.tokenStart(node), p.tokenStart(first), open)
	for i, item := range items {
		p.provider(item)
		if i < len(items)-1 {
			continue
		}
	}
	middle := ")("
	if array {
		middle = "])("
	}
	p.plan.markText(editRegion, partMiddle, p.lastTokenEnd(last), brace, middle+generatorOpen(node))
	p.plan.markText(editRegion, partClose, node.End(), node.End(), "))")
	p.plan.markText(editDispatch, partClose, node.End(), node.End(), p.dispatch(r, name, normal, e))
}

// isWrittenArray reports whether array, an array literal of providers, is
// written in the source as one, rather than made by the parser from
// providers separated by commas.
func (p *projector) isWrittenArray(array *ast.Node) bool {
	start := p.tokenStart(array)
	return start < len(p.text) && p.text[start] == '['
}

// isProvideService reports whether node is the parser's
// $provideService(Key, "Key", value) for a provider written Key = value.
func isProvideService(node *ast.Node) bool {
	return node.Kind == ast.KindCallExpression && node.Expression().Kind == ast.KindIdentifier &&
		node.Expression().Flags&ast.NodeFlagsReparsed != 0 && node.Expression().Text() == "$provideService"
}

// provider lowers one provider of a provide block: Key = value becomes
// $provideService(Key, "Key", value); any other provider stays.
func (p *projector) provider(item *ast.Node) {
	if !isProvideService(item) {
		p.visit(item, env{})
		return
	}
	arguments := item.Arguments()
	key, label, value := arguments[0], arguments[1], arguments[2]
	p.plan.insert(p.tokenStart(key), helperRef("$provideService")+"(")
	p.visit(key, env{})
	p.plan.replace(key.End(), p.tokenStart(value), ", "+strconv.Quote(label.Text())+", ")
	p.visit(value, env{})
	p.plan.insert(value.End(), ")")
}

// lastTokenEnd returns the end of node's last token.
func (p *projector) lastTokenEnd(node *ast.Node) int {
	return node.End()
}

// braceAfter returns the position of the first { after pos, skipping
// trivia and the closing parenthesis of provide(...) { ... }, or -1.
func (p *projector) braceAfter(pos int) int {
	for i := scanner.SkipTrivia(p.text, pos); i < len(p.text); i = scanner.SkipTrivia(p.text, i+1) {
		switch p.text[i] {
		case '{':
			return i
		case ')':
			continue
		}
		return -1
	}
	return -1
}

// canCompleteNormally reports whether a statement list can end by falling
// off its end, read from syntax: a return, throw, break, or continue does
// not, nor an if whose branches both do not. It errs toward true.
func canCompleteNormally(statements []*ast.Node) bool {
	for _, statement := range statements {
		if !completes(statement) {
			return false
		}
	}
	return true
}

// completes reports whether statement can end normally (canCompleteNormally).
func completes(statement *ast.Node) bool {
	switch statement.Kind {
	case ast.KindReturnStatement, ast.KindThrowStatement, ast.KindBreakStatement, ast.KindContinueStatement:
		return false
	case ast.KindBlock:
		return canCompleteNormally(statement.Statements())
	case ast.KindIfStatement:
		s := statement.AsIfStatement()
		return s.ElseStatement == nil || completes(s.ThenStatement) || completes(s.ElseStatement)
	case ast.KindTryStatement:
		s := statement.AsTryStatement()
		normal := completes(s.TryBlock)
		if s.CatchClause != nil {
			normal = normal || completes(s.CatchClause.AsCatchClause().Block)
		}
		return normal && (s.FinallyBlock == nil || completes(s.FinallyBlock))
	case ast.KindLabeledStatement:
		return true
	}
	return true
}

// A handleGroup is the clauses of a handle statement for one control, in
// source order.
type handleGroup struct {
	control string // the control's text, as the operations name it
	clauses []handleClause
}

// A handleClause is one clause of a handle statement as the parser reads
// it: its block, whose first statement binds the continuation and the
// parameters, and the operation it answers.
type handleClause struct {
	block      *ast.Node
	binding    *ast.Node
	operation  *ast.Node // Control.operation
	parameters []*ast.Node
	body       []*ast.Node // the clause's own statements
}

// handleClauses reads the clauses of a handle statement from the if chain
// the parser writes in its catch block (internal/parser/effeck.go), grouped
// by control. ok is false when clauses of one control do not stand together.
func handleClauses(node *ast.Node) (groups []handleGroup, ok bool) {
	catch := node.AsTryStatement().CatchClause
	if catch == nil {
		return nil, false
	}
	statements := catch.AsCatchClause().Block.Statements()
	if len(statements) != 1 {
		return nil, false
	}
	for link := statements[0]; link != nil; {
		if link.Kind != ast.KindIfStatement {
			return nil, false
		}
		s := link.AsIfStatement()
		block := s.ThenStatement
		inner := block.Statements()
		if len(inner) == 0 || inner[0].Kind != ast.KindVariableStatement || inner[0].Flags&ast.NodeFlagsEffeck == 0 {
			return nil, false
		}
		declaration := inner[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes[0]
		call := declaration.Initializer()
		if call == nil || call.Kind != ast.KindCallExpression || len(call.Arguments()) != 1 {
			return nil, false
		}
		operation := call.Arguments()[0]
		if operation.Kind != ast.KindPropertyAccessExpression {
			return nil, false
		}
		elements := declaration.Name().Elements()
		clause := handleClause{block: block, binding: inner[0], operation: operation, parameters: elements[1:], body: inner[1:]}
		control := operation.Expression()
		key := control.Text()
		if control.Kind != ast.KindIdentifier {
			key = scannerText(control)
		}
		if n := len(groups); n > 0 && groups[n-1].control == key {
			groups[n-1].clauses = append(groups[n-1].clauses, clause)
		} else {
			for _, g := range groups {
				if g.control == key {
					return nil, false
				}
			}
			groups = append(groups, handleGroup{control: key, clauses: []handleClause{clause}})
		}
		link = s.ElseStatement
	}
	return groups, len(groups) > 0
}

// scannerText returns the source text of node, or "" for a node the parser
// synthesized.
func scannerText(node *ast.Node) string {
	file := ast.GetSourceFileOfNode(node)
	if file == nil || node.Flags&ast.NodeFlagsReparsed != 0 {
		return ""
	}
	text := file.Text()
	return text[scanner.SkipTrivia(text, node.Pos()):node.End()]
}

// handleStatement lowers a handle statement (D132) in an Effect body to the
// runtime's handle, one call per control, the first control innermost:
//
//	try { B } handle Approval.approve(request) { C }
//	const $c = yield* handle($gen(function* () { B }), Approval, { approve: (request, $k) => $gen(function* () { C }) })
//
// The block and the clauses are blocks of one region: a return, break, or
// continue in either leaves the statement through a completion record. In a
// clause, the parser passes the continuation to resume, resumeWith, and
// abandon, and a resume in tail position returns the runtime's tail-resume
// request (D18).
func (p *projector) handleStatement(node *ast.Node, e env) {
	groups, ok := handleClauses(node)
	body := node.AsTryStatement().TryBlock
	if !ok || p.tokenStart(body) >= len(p.text) || p.text[p.tokenStart(body)] != '{' {
		p.unprojected(node)
		return
	}
	r := &region{tail: isTailRegion(node, e)}
	name := p.freshCompletion()
	k := p.freshWord("$k")
	var closers []func()
	normal, closer := p.regionBlock(body, body.Statements(), r, e)
	closers = append(closers, closer)
	type clauseBraces struct{ open, close int }
	braces := map[*ast.Node]clauseBraces{}
	for _, group := range groups {
		for _, clause := range group.clauses {
			open := p.clauseBodyBrace(clause)
			closing := clause.block.End() - 1
			if open < 0 || p.text[closing] != '}' {
				p.unprojected(node)
				return
			}
			braces[clause.block] = clauseBraces{open, closing}
			ce := regionEnv(r, e)
			ce.continuation = k
			p.visitList(clause.body, ce)
			clauseNormal := canCompleteNormally(clause.body)
			normal = normal || clauseNormal
			closers = append(closers, func() {
				if clauseNormal && len(r.jumps) > 0 && !r.tail {
					p.plan.markText(editRecord, partClose, closing, closing, "; return "+helperRef("$end")+" ")
				}
			})
		}
	}
	for _, closer := range closers {
		closer()
	}
	open := openRegion(r, name) + strings.Repeat(helperRef("handle")+"(", len(groups))
	p.plan.markText(editRegion, partOpen, p.tokenStart(node), p.tokenStart(body), open+generatorOpen(body))
	p.plan.markText(editRegion, partMiddle, body.End(), body.End(), ")")
	for _, group := range groups {
		for i, clause := range group.clauses {
			handleKeyword := p.tokenStart(clause.block)
			operation := clause.operation
			control, member := operation.Expression(), operation.Name()
			if i == 0 {
				// , Approval, { approve: (request, $k) => ...
				p.plan.markText(editRegion, partMiddle, handleKeyword, p.tokenStart(control), ", ")
				p.visit(control, env{})
				p.plan.markText(editRegion, partMiddle, control.End(), p.tokenStart(member), ", { ")
			} else {
				p.plan.markText(editRegion, partMiddle, handleKeyword, p.tokenStart(member), ", ")
			}
			b := braces[clause.block]
			var params []*ast.Node
			for _, parameter := range clause.parameters {
				if parameter.Kind == ast.KindBindingElement && parameter.Flags&ast.NodeFlagsReparsed == 0 {
					params = append(params, parameter)
				}
			}
			if len(params) == 0 {
				p.plan.markText(editRegion, partMiddle, member.End(), b.open, ": ("+k+") => "+generatorOpen(clause.block))
			} else {
				p.plan.markText(editRegion, partMiddle, member.End(), p.tokenStart(params[0]), ": (")
				for _, parameter := range params {
					p.visit(parameter, env{})
				}
				p.plan.markText(editRegion, partMiddle, params[len(params)-1].End(), b.open, ", "+k+") => "+generatorOpen(clause.block))
			}
			separator := ")"
			if i == len(group.clauses)-1 {
				separator = ") })"
			}
			p.plan.markText(editRegion, partMiddle, b.close+1, b.close+1, separator)
		}
	}
	end := node.End()
	p.plan.markText(editDispatch, partClose, end, end, p.dispatch(r, name, normal, e))
}

// clauseBodyBrace returns the position of the { that opens a handle
// clause's own block, after its parameter list, or -1.
func (p *projector) clauseBodyBrace(clause handleClause) int {
	pos := clause.operation.End()
	if n := len(clause.parameters); n > 0 && clause.parameters[n-1].Flags&ast.NodeFlagsReparsed == 0 {
		pos = clause.parameters[n-1].End()
	}
	depth := 0
	for i := scanner.SkipTrivia(p.text, pos); i < len(p.text); i = scanner.SkipTrivia(p.text, i+1) {
		switch p.text[i] {
		case '(':
			depth++
		case ')':
			depth--
		case '{':
			if depth <= 0 {
				return i
			}
		default:
			if depth <= 0 {
				return -1
			}
		}
	}
	return -1
}

// freshWord returns name, or name with a numeric suffix, that no word of
// the file spells, for a synthesized local such as a clause's continuation.
func (p *projector) freshWord(name string) string {
	if !p.names.words[name] {
		return name
	}
	for i := 1; ; i++ {
		if candidate := name + "_" + strconv.Itoa(i); !p.names.words[candidate] {
			return candidate
		}
	}
}

// isContinuationArgument reports whether node is the continuation the parser
// passes to resume, resumeWith, or abandon in a handle clause.
func isContinuationArgument(node *ast.Node) bool {
	return node.Kind == ast.KindIdentifier && node.Flags&ast.NodeFlagsReparsed != 0 && node.Text() == "$k"
}

// resumeCall lowers the arguments of resume, resumeWith, or abandon in a
// handle clause: the parser's continuation becomes the clause's, and resume()
// passes undefined, as the parser writes it.
func (p *projector) continuationArguments(call *ast.Node, e env) {
	arguments := call.Arguments()
	open := scanner.SkipTrivia(p.text, call.Expression().End())
	if call.TypeArgumentList() != nil {
		open = scanner.SkipTrivia(p.text, call.TypeArgumentList().End()+1)
	}
	var written []*ast.Node
	for _, argument := range arguments[1:] {
		if argument.Flags&ast.NodeFlagsReparsed == 0 {
			written = append(written, argument)
		}
	}
	text := e.continuation
	switch {
	case len(written) > 0:
		text += ", "
	case len(arguments) > 1:
		text += ", undefined"
	}
	p.plan.insert(open+1, text)
	for _, argument := range written {
		p.visit(argument, e)
	}
}

// isTailResume reports whether node is the return the parser writes for a
// resume or resumeWith in tail position of a handle clause.
func isTailResume(node *ast.Node) bool {
	if node.Kind != ast.KindReturnStatement || node.Flags&ast.NodeFlagsEffeck == 0 || node.Expression() == nil {
		return false
	}
	call := node.Expression()
	return call.Kind == ast.KindCallExpression && len(call.Arguments()) > 0 && isContinuationArgument(call.Arguments()[0])
}

// tailResume lowers a tail resume to the tail-resume request it returns:
// `resume(v)` becomes `return $tailResume($k, v)`.
func (p *projector) tailResume(node *ast.Node, e env) {
	call := node.Expression()
	callee := call.Expression()
	helper := "$tailResume"
	if callee.Text() == "resumeWith" {
		helper = "$tailResumeWith"
	}
	p.plan.insert(p.tokenStart(node), "return ")
	p.plan.replace(p.tokenStart(callee), callee.End(), helperRef(helper))
	p.continuationArguments(call, e)
}

// isUsingDeclaration reports whether node declares with using: in an
// Effect body, an acquisition or a Disposable the block releases (D47).
func isUsingDeclaration(node *ast.Node) bool {
	if node.Kind != ast.KindVariableStatement {
		return false
	}
	list := node.AsVariableStatement().DeclarationList
	return list.Flags&ast.NodeFlagsUsing != 0 && list.Flags&ast.NodeFlagsAwaitUsing != ast.NodeFlagsAwaitUsing
}

// usingDeclaration lowers `using x = acquire()` in an Effect body to
// `const x = (yield* $acquire(acquire()))`. The initializer passes by name:
// its own call does not run, so $acquire receives the acquisition, and the
// block's Scope leaves its R (D14). A Disposable is acquired as itself.
func (p *projector) usingDeclaration(node *ast.Node, e env) {
	list := node.AsVariableStatement().DeclarationList
	start := p.tokenStart(list)
	if !strings.HasPrefix(p.text[start:], "using") {
		p.unprojected(node)
		return
	}
	p.plan.replace(start, start+len("using"), "const")
	for _, declaration := range list.AsVariableDeclarationList().Declarations.Nodes {
		p.visit(declaration.Name(), env{})
		if t := declaration.Type(); t != nil {
			p.visit(t, env{})
		}
		init := declaration.Initializer()
		if init == nil {
			continue
		}
		p.plan.mark(editUsing, partOpen, p.tokenStart(init), p.tokenStart(init), "")
		p.byName(init, e)
		p.plan.mark(editUsing, partClose, init.End(), init.End(), "")
	}
}

// byName walks expr, an argument passed by name: a call at its top does not
// run, though its callee and arguments are lowered as in any call.
func (p *projector) byName(expr *ast.Node, e env) {
	if expr.Kind == ast.KindCallExpression && isRunnableCall(expr) {
		p.callee(expr, e)
		p.callArguments(expr, e)
		return
	}
	p.visit(expr, e)
}

// maskWord returns the interruption block's word, uninterruptible, restore,
// or interruptible, when node is such a block: a block whose first statement
// calls the parser's marker, such as $uninterruptibleBlock(). It returns "".
func maskWord(node *ast.Node) string {
	if node.Kind != ast.KindBlock {
		return ""
	}
	statements := node.Statements()
	if len(statements) == 0 || statements[0].Kind != ast.KindExpressionStatement {
		return ""
	}
	call := statements[0].Expression()
	if call.Kind != ast.KindCallExpression || call.Flags&ast.NodeFlagsReparsed == 0 || !ast.IsIdentifier(call.Expression()) {
		return ""
	}
	switch call.Expression().Text() {
	case "$uninterruptibleBlock":
		return "uninterruptible"
	case "$restoreBlock":
		return "restore"
	case "$interruptibleBlock":
		return "interruptible"
	}
	return ""
}

// maskBlock lowers an interruption block (D179, D180) in an Effect body to
// its region under the prelude's function of the same name:
// `uninterruptible { body }` runs as `uninterruptible(program)`.
func (p *projector) maskBlock(node *ast.Node, word string, e env) {
	start := p.tokenStart(node)
	brace := scanner.SkipTrivia(p.text, start+len(word))
	closing := node.End() - 1
	if !strings.HasPrefix(p.text[start:], word) || brace >= len(p.text) || p.text[brace] != '{' || p.text[closing] != '}' {
		p.unprojected(node)
		return
	}
	r := &region{tail: isTailRegion(node, e)}
	name := p.freshCompletion()
	body := node.Statements()[1:]
	p.visitList(body, regionEnv(r, e))
	normal := canCompleteNormally(body)
	if normal && len(r.jumps) > 0 && !r.tail {
		p.plan.markText(editRecord, partClose, closing, closing, "; return "+helperRef("$end")+" ")
	}
	p.plan.markText(editRegion, partOpen, start, brace, openRegion(r, name)+p.names.prelude(word)+"("+generatorOpen(node))
	p.plan.markText(editRegion, partClose, node.End(), node.End(), "))")
	p.plan.markText(editDispatch, partClose, node.End(), node.End(), p.dispatch(r, name, normal, e))
}

// exitBinding returns the name a `finally (exit)` block binds (D178): the
// parser writes `const exit = $exit()` as the block's first statement.
func exitBinding(finally *ast.Node) *ast.Node {
	statements := finally.Statements()
	if len(statements) == 0 || statements[0].Kind != ast.KindVariableStatement {
		return nil
	}
	declarations := statements[0].AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations.Nodes
	if len(declarations) != 1 {
		return nil
	}
	init := declarations[0].Initializer()
	if init == nil || init.Kind != ast.KindCallExpression || init.Flags&ast.NodeFlagsReparsed == 0 ||
		!ast.IsIdentifier(init.Expression()) || init.Expression().Text() != "$exit" {
		return nil
	}
	return declarations[0].Name()
}
