package vibelangmapper

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"strings"
)

// regionVar lowers a function-scoped var through a generated region. Its
// initializer remains at the source site; a checker-only generator infers
// its success type without running or adding its errors a second time.
func (p *projector) regionVar(node *ast.Node, e env) {
	list := node.AsVariableDeclarationList()
	fn := node.Parent
	for fn != nil && !ast.IsFunctionLike(fn) {
		fn = fn.Parent
	}
	if fn == nil || fn.Body() == nil {
		p.unprojected(node)
		return
	}
	for _, d := range list.Declarations.Nodes {
		if !ast.IsIdentifier(d.Name()) || d.Initializer() == nil || !p.canHoistRegionType(d, fn) {
			p.diagnostics = append(p.diagnostics, Diagnostic{Start: p.tokenStart(d), Length: d.End() - p.tokenStart(d), Code: CodeUnprojectedForm, Message: "Annotate this region variable and declare it before the region; its binding cannot be inferred from a single initializer."})
			return
		}
	}
	p.plan.replace(p.tokenStart(node), p.tokenStart(node)+len("var"), "")
	for _, d := range list.Declarations.Nodes {
		init := d.Initializer()
		if d.Type() != nil {
			p.plan.replace(d.Name().End(), d.Type().End(), "")
		}
		before := len(p.plan.edits)
		p.visit(init, e)
		value, ok := p.checkingFragment(init, p.plan.edits[before:])
		if !ok {
			p.unprojected(d)
			continue
		}
		name := d.Name().Text()
		typ := ""
		if d.Type() != nil {
			beforeType := len(p.plan.edits)
			p.visit(d.Type(), env{})
			text, ok := p.checkingFragment(d.Type(), p.plan.edits[beforeType:])
			p.plan.edits = p.plan.edits[:beforeType]
			if !ok {
				p.unprojected(d)
				continue
			}
			typ = ": " + text
		}
		check := "var " + name + typ + " = " + helperRef("$varType") + "(" + helperRef("$gen") + "(" + selfPrefix(init) + "function* () { return " + strings.TrimSpace(value) + " })); "
		p.plan.markFields(editRegionVar, partWhole, p.tokenStart(fn.Body())+1, p.tokenStart(fn.Body())+1, check, "var "+name+"; ")
	}
}

// checkingFragment renders a source subtree with its own edits so a type
// witness can retain the exact lowering without executing it twice.
func (p *projector) checkingFragment(node *ast.Node, edits []edit) (string, bool) {
	start := p.tokenStart(node)
	var fragment plan
	for _, ed := range edits {
		ed.pos -= start
		ed.end -= start
		fragment.edits = append(fragment.edits, ed)
	}
	text, _, _, ok := fragment.render(checking{names: p.names}, p.text[start:node.End()], func() string { return "" })
	return text, ok
}

// canHoistRegionType rejects a type witness that would cross a lexical
// binding or a repeated var declaration. An explicit declaration outside
// the region is needed in those cases; the projector cannot infer there.
func (p *projector) canHoistRegionType(declaration, fn *ast.Node) bool {
	name := declaration.Name().Text()
	declarations := 0
	hidden := map[string]bool{}
	var scan func(*ast.Node)
	scan = func(n *ast.Node) {
		local := false
		for parent := n.Parent; parent != nil && parent != fn.Body(); parent = parent.Parent {
			if parent.Kind == ast.KindBlock || parent.Kind == ast.KindCatchClause || ast.IsIterationStatement(parent, false) {
				local = true
				break
			}
		}
		var bind func(*ast.Node)
		bind = func(binding *ast.Node) {
			if binding == nil {
				return
			}
			if ast.IsIdentifier(binding) {
				hidden[binding.Text()] = true
				return
			}
			binding.ForEachChild(func(child *ast.Node) bool { bind(child); return false })
		}
		if n.Kind == ast.KindVariableDeclaration {
			if ast.IsIdentifier(n.Name()) && n.Name().Text() == name {
				declarations++
			}
			if n.Parent.Kind == ast.KindCatchClause || local && n.Parent.Kind == ast.KindVariableDeclarationList && n.Parent.Flags&ast.NodeFlagsBlockScoped != 0 {
				bind(n.Name())
			}
		}
		if n != fn.Body() && (ast.IsFunctionLike(n) || ast.IsClassLike(n)) {
			if local {
				bind(n.Name())
			}
			return
		}
		if local && (n.Kind == ast.KindTypeAliasDeclaration || n.Kind == ast.KindInterfaceDeclaration) {
			bind(n.Name())
		}
		n.ForEachChild(func(child *ast.Node) bool { scan(child); return false })
	}
	scan(fn.Body())
	if declarations != 1 {
		return false
	}
	for _, param := range fn.Parameters() {
		if ast.IsIdentifier(param.Name()) && param.Name().Text() == name {
			return false
		}
	}
	valid := true
	var check func(*ast.Node)
	check = func(n *ast.Node) {
		if ast.IsIdentifier(n) && hidden[n.Text()] {
			valid = false
		}
		n.ForEachChild(func(child *ast.Node) bool { check(child); return false })
	}
	check(declaration.Initializer())
	if declaration.Type() != nil {
		check(declaration.Type())
	}
	return valid
}
