package effeckmapper

import "github.com/microsoft/TypeScript/tsc/internal/ast"

// isPipeExpression recognizes the parser's D154 binary node. The body can
// hold a catch expression (D237), whose thunks must see the same topic.
func isPipeExpression(node *ast.Node) bool {
	return node.Kind == ast.KindBinaryExpression && node.Flags&ast.NodeFlagsEffeck != 0 && node.AsBinaryExpression().OperatorToken.Kind == ast.KindBarToken
}

// pipeExpression evaluates the input once, then runs the body with a lexical
// topic. A program gives nested catch thunks and lambdas access to that value
// without sharing a mutable temporary between concurrent executions.
func (p *projector) pipeExpression(node *ast.Node, e env) {
	if !p.pipeValueTopics(node) {
		return
	}
	if !e.effect {
		p.awaitChoice(node, "A pipe must run in an Effect body; move it into an Effect function with an explicit contract.")
		return
	}
	if p.pipeTopics == nil {
		p.pipeTopics = map[*ast.Node]string{}
	}
	topic := p.freshWord("$topic")
	p.names.words[topic] = true
	p.pipeTopics[node] = topic
	start := p.tokenStart(node)
	if p.statementStarts[start] {
		p.plan.insert(start, ";")
		delete(p.statementStarts, start)
	}
	p.plan.mark(editRun, partOpen, start, start, "")
	p.plan.mark(editProgram, partOpen, start, start, selfArg(node))
	p.plan.insert(start, "{ const "+topic+" = ")
	binary := node.AsBinaryExpression()
	p.visit(binary.Left, expressionEnv())
	p.plan.replace(binary.Left.End(), p.tokenStart(binary.Right), "; return ")
	p.visit(binary.Right, expressionEnv())
	p.plan.insert(node.End(), " }")
	p.plan.mark(editProgram, partClose, node.End(), node.End(), "")
	p.plan.mark(editRun, partClose, node.End(), node.End(), "")
}

// pipeValueTopics limits this lowering to value topics. D155 program topics
// need a different evaluation boundary, and an opaque callee may expect one;
// either case draws a diagnostic rather than silently evaluating too early.
func (p *projector) pipeValueTopics(pipe *ast.Node) bool {
	r := p.forms.(*Resolver)
	ok := true
	topics := 0
	var visit func(*ast.Node) bool
	visit = func(node *ast.Node) bool {
		if isPipeExpression(node) {
			// Topics in a nested pipe's input still belong to this pipe.
			return visit(node.AsBinaryExpression().Left)
		}
		if formsIsTopic(node) {
			topics++
			d := p.forms.Argument(node)
			if d.ByName {
				p.awaitChoice(node, "The mapper does not lower a pipe's program topic yet. Bind the input to an explicitly annotated value before the pipe, or pass an explicit program lambda to the combinator.")
				ok = false
			} else if d.Call != nil {
				sigs, _ := r.signaturesOf(d.Callee)
				if d.Ambiguous || len(formsCandidates(sigs, len(d.Call.Arguments()))) == 0 {
					p.awaitChoice(node, "The mapper cannot decide whether this pipe topic is a value or a program. Annotate the callee's parameter, or bind the input to an explicitly annotated value before the pipe.")
					ok = false
				}
			}
			return false
		}
		return node.ForEachChild(visit)
	}
	visit(pipe.AsBinaryExpression().Right)
	if topics == 0 {
		p.diagnostics = append(p.diagnostics, Diagnostic{
			Start: p.tokenStart(pipe.AsBinaryExpression().OperatorToken), Length: len("|>"), Code: 1006,
			Message: "A pipe body must use its topic (%); the input value would otherwise be discarded.",
		})
		return false
	}
	return ok
}

// pipeTopic resolves % through the syntax tree, so a topic in a nested
// pipe's input still belongs to the outer pipe and catch thunks keep scope.
func (p *projector) pipeTopic(node *ast.Node) {
	for n := node; n.Parent != nil; n = n.Parent {
		if isPipeExpression(n.Parent) && n.Parent.AsBinaryExpression().Right == n {
			if topic := p.pipeTopics[n.Parent]; topic != "" {
				p.plan.replace(p.tokenStart(node), node.End(), topic)
				return
			}
		}
	}
	p.unprojected(node)
}
