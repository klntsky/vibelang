package effeckmapper

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// dataReceiver names a D152 receiver only when its annotation, initializer,
// lexical pipe topic, or a pinned declaration determines the module. Following
// bindings preserves shadows and never chooses a module from a method name.
func (p *projector) dataReceiver(expr *ast.Node, depth int) string {
	if expr == nil || depth > 24 {
		return ""
	}
	r, ok := p.forms.(*Resolver)
	if !ok {
		return ""
	}
	expr = ast.SkipParentheses(expr)
	if formsIsTopic(expr) {
		for child := expr; child.Parent != nil; child = child.Parent {
			if isPipeExpression(child.Parent) && child.Parent.AsBinaryExpression().Right == child {
				return p.dataReceiver(child.Parent.AsBinaryExpression().Left, depth+1)
			}
		}
		return ""
	}
	if ast.IsIdentifier(expr) {
		d := r.binding(expr, expr.Text())
		if d != nil && d.Kind == ast.KindVariableDeclaration && d.Type() == nil {
			return p.dataReceiver(d.Initializer(), depth+1)
		}
	}
	if ast.IsCallExpression(expr) {
		c := r.Callee(expr)
		if c.Canonical == "effeck-runtime#fork" || c.Canonical == "effeck-runtime#forkIn" {
			return "effect/Fiber"
		}
		if c.Kind == CalleeLocal && ast.IsIdentifier(expr.Expression()) {
			d := r.binding(expr.Expression(), expr.Expression().Text())
			if d != nil && d.Kind == ast.KindFunctionDeclaration && d.AsFunctionDeclaration().AsteriskToken != nil {
				return "effect/Stream"
			}
		}
		access := ast.SkipParentheses(expr.Expression())
		if ast.IsPropertyAccessExpression(access) {
			module := p.dataReceiver(access.Expression(), depth+1)
			if module != "" {
				if result := p.dataMethodResult(module, access.Name().Text(), expr.Arguments()); result != "" {
					return result
				}
			}
		}
	}
	return r.receiverModule(expr, 0)
}

// dataMethod lowers a syntax-identified D152 call to its module function.
// Checking and execution share this edit, so methods never need prototypes
// at run time. The table's own-member entries take precedence over self-first
// functions, just as the fork's missing-property hook does.
func (p *projector) dataMethod(call *ast.Node, e env) bool {
	access := call.Expression()
	if !ast.IsPropertyAccessExpression(access) || access.QuestionDotToken() != nil || call.QuestionDotToken() != nil {
		return false
	}
	module := p.dataReceiver(access.Expression(), 0)
	name := access.Name().Text()
	if !dataMethods[module][name] || dataOwnMembers[module][name] {
		return false
	}
	if call.TypeArgumentList() != nil {
		p.diagnostics = append(p.diagnostics, Diagnostic{Start: p.tokenStart(call), Length: call.End() - p.tokenStart(call), Code: CodeUnprojectedForm, Message: "Write this data-method call as an annotated module function call; explicit method type arguments cannot be transferred without an annotation."})
		return false
	}
	receiver := access.Expression()
	// A predicate stays a direct call: wrapping its boolean in $run would
	// discard TypeScript's narrowing of the original receiver.
	guard := dataGuards[module+"#"+name]
	start := p.tokenStart(call)
	if p.statementStarts[start] {
		p.plan.insert(start, ";")
		delete(p.statementStarts, start)
	}
	if e.effect && !guard {
		p.plan.mark(editRun, partOpen, start, start, "")
	}
	namespace := strings.TrimPrefix(module, "effect/")
	p.plan.insert(start, p.names.prelude(namespace)+"."+name+"(")
	p.visit(receiver, e)
	end := scanner.SkipTrivia(p.text, access.End()) + 1
	comma := ""
	if len(call.Arguments()) > 0 {
		comma = ", "
	}
	p.plan.replace(receiver.End(), end, comma)
	p.callArguments(call, e)
	if e.effect && !guard {
		p.plan.mark(editRun, partClose, call.End(), call.End(), "")
	}
	return true
}

// predicateCall preserves narrowing and contextual inference in pinned
// predicates and explicitly curried plain calls.
// Resolving its canonical import first excludes local functions with the
// same spelling, whose calls may return Effects instead of booleans.
func (p *projector) predicateCall(call *ast.Node, e env) bool {
	r, ok := p.forms.(*Resolver)
	if !ok {
		return false
	}
	key := r.Callee(call).Canonical
	plain := dataGuards[key] || p.plainArrayPipeStage(call, key) || p.plainMatchPipeStage(call, key)
	if !plain && dataLastMethods[key] {
		candidates := formsCandidates(r.table.Signatures(key), len(call.Arguments()))
		plain = len(candidates) > 0
		for _, sig := range candidates {
			if sig.Self || sig.Result == "effect/Effect" {
				plain = false
			}
		}
	}
	if !plain {
		return false
	}
	p.callee(call, e)
	p.callArguments(call, e)
	return true
}

// dataMethodResult retains return-module metadata while removing self from
// arity, and follows pipe only through declarations explicitly curried on self.
func (p *projector) dataMethodResult(module, name string, args []*ast.Node) string {
	r := p.forms.(*Resolver)
	if name == "pipe" {
		result := module
		for _, arg := range args {
			if !ast.IsCallExpression(arg) {
				return ""
			}
			key := r.Callee(arg).Canonical
			home, member, ok := strings.Cut(key, "#")
			if !ok || !dataLastMethods[key] {
				return ""
			}
			result = p.dataMethodResult(home, member, arg.Arguments())
			if result == "" {
				return ""
			}
		}
		return result
	}
	result := ""
	for _, m := range r.table.methodReceivers(name) {
		if m.receiver != module {
			continue
		}
		for _, sig := range m.sigs {
			argc := len(args)
			if m.self {
				argc++
			}
			if !sig.Accepts(argc) {
				continue
			}
			next := sig.Result
			if next == "effect/Effect" {
				next = sig.Runs
			}
			if next == "" || result != "" && next != result {
				return ""
			}
			result = next
		}
	}
	return result
}
