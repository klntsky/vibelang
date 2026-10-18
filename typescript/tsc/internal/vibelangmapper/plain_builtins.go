package vibelangmapper

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// plainBuiltinFunction proves the D260 direct path from syntax: the body
// has a native catch, its explicit throws and shipped failures are caught
// locally, and every call stays synchronous. Unknown calls retain the Effect
// entry rather than guessing. JavaScript property access can throw too.
func (p *projector) plainBuiltinFunction(fn *ast.Node) bool {
	if !isProjectedFunction(fn) || fn.Name() == nil || fn.Parent != p.file.AsNode() || fn.Type() != nil || p.purityDeclarations()[fn.Name().Text()] != 1 {
		return false
	}
	found, safe := false, true
	var visit func(*ast.Node, bool)
	visit = func(n *ast.Node, caught bool) {
		if n == nil || !safe {
			return
		}
		if n != fn.Body() && (ast.IsFunctionLike(n) || ast.IsClassLike(n)) {
			safe = false
			return
		}
		if n.Flags&(ast.NodeFlagsVibeLang|ast.NodeFlagsReparsed) != 0 {
			safe = false
			return
		}
		switch n.Kind {
		case ast.KindTryStatement:
			stmt := n.AsTryStatement()
			if stmt.CatchClause != nil {
				found = true
				v := stmt.CatchClause.AsCatchClause().VariableDeclaration
				if v != nil && (!ast.IsIdentifier(v.Name()) || v.Type() != nil) {
					safe = false
					return
				}
			}
			visit(stmt.TryBlock, caught || stmt.CatchClause != nil)
			visit(stmt.CatchClause, caught)
			visit(stmt.FinallyBlock, caught)
			return
		case ast.KindThrowStatement:
			if !caught {
				safe = false
				return
			}
		case ast.KindCallExpression:
			if p.builtinFailure(n) != "" {
				found = true
				safe = caught
			} else {
				safe = p.isPureGlobal(n)
			}
			if !safe {
				return
			}
		case ast.KindThisKeyword, ast.KindSuperKeyword, ast.KindAwaitExpression, ast.KindYieldExpression, ast.KindForOfStatement, ast.KindTaggedTemplateExpression:
			safe = false
			return
		case ast.KindVariableDeclarationList:
			if n.Flags&ast.NodeFlagsUsing != 0 {
				safe = false
				return
			}
		case ast.KindIdentifier:
			if n.Text() == "arguments" {
				safe = false
				return
			}
		}
		n.ForEachChild(func(child *ast.Node) bool { visit(child, caught); return !safe })
	}
	visit(fn.Body(), false)
	return found && safe
}

// preparePlainBuiltinBody records the direct body's native try/catch edits
// beside the checking Effect plan. Both use the same shipped call contract.
func (p *projector) preparePlainBuiltinBody(fn *ast.Node) {
	if !p.plainBuiltinFunction(fn) {
		return
	}
	q := *p
	q.plan = plan{}
	q.plainBuiltin = true
	q.visitList(fn.Body().Statements(), env{})
	if p.plainBuiltinBodies == nil {
		p.plainBuiltinBodies = map[*ast.Node][]edit{}
	}
	p.plainBuiltinBodies[fn] = append([]edit{}, q.plan.edits...) // non-nil even when the native body needs no edits
}

// directBuiltinEdits substitutes native bodies only for execution. Retaining
// their outer Effect-body markers lets the usual direct-entry split handle
// wrappers, calls, and source mapping without a second declaration renderer.
func (p *projector) directBuiltinEdits() []edit {
	var result []edit
	for _, e := range p.plan.edits {
		replace := false
		for fn := range p.plainBuiltinBodies {
			lo, hi := p.tokenStart(fn.Body())+1, fn.Body().End()-1
			if e.pos >= lo && e.pos <= hi && !(e.kind == editEffectBody && (e.pos == lo && e.part == partOpen || e.pos == hi && e.part == partClose)) {
				replace = true
				break
			}
		}
		if !replace {
			result = append(result, e)
		}
	}
	for _, edits := range p.plainBuiltinBodies {
		result = append(result, edits...)
	}
	return result
}

// plainBuiltinCall selects the proven synchronous entry only in a plain
// callback; its declaration and ordinary Effect calls keep their Effect type.
func (p *projector) plainBuiltinCall(call *ast.Node) bool {
	if !ast.IsIdentifier(call.Expression()) || call.QuestionDotToken() != nil {
		return false
	}
	r, ok := p.forms.(*Resolver)
	if !ok {
		return false
	}
	target := call.Expression()
	fn := r.binding(target, target.Text())
	if fn == nil || !p.plainBuiltinFunction(fn) {
		return false
	}
	p.plan.mark(editPlainCall, partOpen, p.tokenStart(target), p.tokenStart(target), "")
	p.plan.mark(editPlainCall, partClose, target.End(), target.End(), "")
	return true
}
