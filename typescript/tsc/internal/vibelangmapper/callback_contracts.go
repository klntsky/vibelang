package vibelangmapper

import (
	"strconv"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
)

// callbackParameters projects D12/D68's contract-less callback parameters
// together with their enclosing declaration's inferred E and R parameters.
// Only direct function types qualify, matching the fork's callbackcontracts.go.
func (p *projector) callbackParameters(fn *ast.Node) {
	if isLambda(fn) {
		if call, _, _ := formsOwnerCall(fn); call != nil {
			return
		}
	}
	var extra []string
	taken := map[string]bool{}
	for scope := fn; scope != nil; scope = scope.Parent {
		if !ast.IsFunctionLike(scope) && !ast.IsClassLike(scope) && !ast.IsInterfaceDeclaration(scope) && !ast.IsTypeAliasDeclaration(scope) {
			continue
		}
		for _, tp := range scope.TypeParameters() {
			taken[tp.Name().Text()] = true
		}
	}
	fresh := func(base string) string {
		name := base
		for i := 2; taken[name]; i++ {
			name = base + "_" + strconv.Itoa(i)
		}
		taken[name] = true
		return name
	}
	for _, parameter := range fn.Parameters() {
		ft := parameter.Type()
		if ft == nil || ft.Kind != ast.KindFunctionType || ft.Type() == nil || isEffectType(ft.Type()) {
			continue
		}
		retType := formsUnwrapType(ft.Type())
		if retType.Kind == ast.KindTypeReference && ast.IsIdentifier(retType.AsTypeReferenceNode().TypeName) && retType.AsTypeReferenceNode().TypeName.Text() == "Effect" && p.names.shadowsPreludeType("Effect") {
			continue
		}
		if r, ok := p.forms.(*Resolver); ok && r.isEffectTypeNode(ft.Type(), 0) {
			continue
		}
		suffix := ""
		if len(extra) > 0 {
			suffix = strconv.Itoa(len(extra)/2 + 1)
		}
		e, r := fresh("E"+suffix), fresh("R"+suffix)
		extra = append(extra, e, r)
		ret := ft.Type()
		p.plan.witness(p.tokenStart(ret), p.names.prelude("Effect")+".Effect<")
		p.plan.witness(ret.End(), ", "+e+", "+r+">")
	}
	if len(extra) == 0 {
		return
	}
	text := strings.Join(extra, ", ")
	if tps := fn.TypeParameters(); len(tps) > 0 {
		p.plan.witness(tps[len(tps)-1].End(), ", "+text)
	} else if fn.Name() != nil {
		p.plan.witness(fn.Name().End(), "<"+text+">")
	} else {
		// An anonymous function's generic list precedes its parameter list.
		at := p.tokenStart(fn)
		if fn.Kind == ast.KindFunctionExpression {
			at += len("function")
		}
		p.plan.witness(at, "<"+text+">")
	}
}
