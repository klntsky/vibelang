package effeckmapper

import (
	"strconv"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
)

//go:generate go run gen_builtin_contracts.go

// caughtBuiltinFailure recognizes an unshadowed shipped contract at a caught
// call (D54). Uncaught plain built-ins retain their direct calling convention.
func (p *projector) caughtBuiltinFailure(call *ast.Node) string {
	target := call.Expression()
	root, chain, ok := formsNameChain(target)
	if !ok || call.QuestionDotToken() != nil {
		return ""
	}
	if r, ok := p.forms.(*Resolver); ok && r.binding(root, root.Text()) != nil {
		return ""
	}
	key := root.Text()
	for _, part := range chain {
		key += "." + part
	}
	failure := builtinContracts[key]
	if failure == "" {
		return ""
	}
	for child, parent := call, call.Parent; parent != nil; child, parent = parent, parent.Parent {
		if ast.IsFunctionLike(parent) {
			break
		}
		if parent.Kind == ast.KindTryStatement && parent.AsTryStatement().TryBlock == child && parent.AsTryStatement().CatchClause != nil {
			return failure
		}
	}
	return ""
}

// builtinCallee adapts just the callee, so arguments run before entering the
// trusted boundary and their failures cannot be mistaken for built-in errors.
func (p *projector) builtinCallee(call *ast.Node) bool {
	failure := p.caughtBuiltinFailure(call)
	if failure == "" {
		return false
	}
	for _, argument := range call.Arguments() {
		if argument.Kind == ast.KindSpreadElement {
			p.diagnostics = append(p.diagnostics, Diagnostic{Start: p.tokenStart(argument), Length: argument.End() - p.tokenStart(argument), Code: CodeUnprojectedForm, Message: "The mapper cannot select a caught built-in's overload from spread arguments. Write the arguments explicitly or use an annotated trusted adapter."})
			return false
		}
	}
	if r, ok := p.forms.(*Resolver); ok && r.binding(call.Expression(), failure) != nil {
		p.diagnostics = append(p.diagnostics, Diagnostic{Start: p.tokenStart(call), Length: call.End() - p.tokenStart(call), Code: CodeUnprojectedForm, Message: "The shipped failure class " + failure + " is shadowed. Rename the binding or use an annotated trusted adapter."})
		return false
	}
	target := call.Expression()
	p.plan.markFields(editBuiltinCall, partWhole, p.tokenStart(target), target.End(), failure, p.text[p.tokenStart(target):target.End()], p.freshWord("$builtinArgs"), strconv.Itoa(len(call.Arguments())))
	return true
}

// renderBuiltinCall shares the shipped failure boundary between both renderings.
func renderBuiltinCall(e edit) string {
	return builtinCallText(e, true)
}

// builtinCallText captures each argument before the trusted thunk runs. Using
// an indexed parameter type retains the original call's arity, including a
// one-argument JSON.parse despite its two-argument ES2026 overload.
func builtinCallText(e edit, checking bool) string {
	failure, callee, base := e.fields[0], e.fields[1], e.fields[2]
	count, _ := strconv.Atoi(e.fields[3])
	var params, args []string
	for i := 0; i < count; i++ {
		name := base + strconv.Itoa(i)
		args = append(args, name)
		if checking {
			name += ": " + helperRef("$BuiltinArgs") + "<typeof " + callee + ">[" + strconv.Itoa(i) + "]"
		}
		params = append(params, name)
	}
	fn := base + "Fn"
	trusted := helperRef("$trusted")
	if checking {
		trusted += "<" + helperRef("$BuiltinResult") + "<typeof " + callee + ">, " + failure + ">"
	}
	fnParam := fn
	if checking {
		fnParam += ": typeof " + callee
	}
	return "((" + fnParam + ") => ((" + strings.Join(params, ", ") + ") => " + trusted + "(() => " + fn + "(" + strings.Join(args, ", ") + "), [" + failure + "])))(" + callee + ")"
}
