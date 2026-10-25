package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// Block params (D188, https://github.com/samuelgoto/proposal-block-params):
// in a .vibe file, a block on the same line after a call or a name is a
// program passed to it. The parser writes `f(args) { block }` as
// f(args, () => { block }), one call, so the checker infers every type
// parameter from the arguments and the block together. Besides the callee's
// own signatures, which append the block, the call may resolve against two
// forms of an Effect dual's: the data-first signature with its program, its
// first parameter, moved last (withSpan(self, name) taking (name, self)), and
// the data-last signature flattened (withSpan(name)(self) taking
// (name, self)). The lowering moves the block where the chosen form takes it.

func init() {
	checker.RegisterCallSignaturesFilterHook(blockParamSignatures)
}

// blockParamForm is where a block param call passes its block.
type blockParamForm uint8

const (
	blockAppended  blockParamForm = iota // f(args, block): the callee's own signature
	blockPrepended                       // f(block, args): a data-first Effect dual
	blockApplied                         // f(args)(block): a data-last Effect dual
)

// isBlockParamCall reports whether call passes a block param: its last
// argument is an arrow function with no parameters and a block body, written
// as a bare block, so its text starts with { rather than (.
func isBlockParamCall(call *ast.Node) bool {
	if !ast.IsCallExpression(call) {
		return false
	}
	arguments := call.Arguments()
	if len(arguments) == 0 {
		return false
	}
	arrow := arguments[len(arguments)-1]
	if arrow.Kind != ast.KindArrowFunction || len(arrow.Parameters()) != 0 || arrow.Body() == nil || arrow.Body().Kind != ast.KindBlock {
		return false
	}
	file := ast.GetSourceFileOfNode(call)
	if !isVibeLangFile(file) {
		return false
	}
	text := file.Text()
	start := scanner.SkipTrivia(text, arrow.Pos())
	return start < len(text) && text[start] == '{'
}

type blockParamsKey struct{}

// blockParamForms maps each signature blockParamSignatures made for c to the
// form it passes the block in.
func blockParamForms(c *checker.Checker) map[*checker.Signature]blockParamForm {
	return c.ExtensionState(blockParamsKey{}, func() any { return map[*checker.Signature]blockParamForm{} }).(map[*checker.Signature]blockParamForm)
}

// blockParamSignatures gives a block param call, f(args, block), each of f's
// signatures in the forms that can take the block: rotated when its first
// parameter is an Effect (a data-first dual), flattened when it returns a
// function whose first parameter is an Effect (a data-last dual), and as it
// is, which appends the block.
func blockParamSignatures(c *checker.Checker, call *ast.Node, signatures []*checker.Signature) []*checker.Signature {
	if len(signatures) == 0 || !isBlockParamCall(call) {
		return signatures
	}
	n := len(call.Arguments()) - 1
	forms := blockParamForms(c)
	result := make([]*checker.Signature, 0, len(signatures)*2)
	for _, signature := range signatures {
		if n > 0 && takesEffectFirst(c, signature) {
			if rotated := c.RotateSignature(signature, n); rotated != nil {
				forms[rotated] = blockPrepended
				result = append(result, rotated)
			}
		}
		if acceptsArguments(signature, n) {
			for _, inner := range c.GetSignaturesOfType(c.GetReturnTypeOfSignature(signature), checker.SignatureKindCall) {
				if !takesEffectFirst(c, inner) || takesBlockDirectly(c, signatures, n, inner) {
					continue
				}
				if flattened := c.FlattenSignature(signature, n, inner); flattened != nil {
					forms[flattened] = blockApplied
					result = append(result, flattened)
				}
			}
		}
		result = append(result, signature)
	}
	return result
}

// takesBlockDirectly reports whether one of signatures takes the block as its
// own parameter n in the role inner takes it, an Effect parameter of the same
// name, as Layer.effect(service, effect) does beside its data-last form
// Layer.effect(service)(effect). The call is then that signature's own, and
// flattening the data-last form would infer the outer type parameters from
// the block too: Layer.effect(S) { ... } would widen S, the service's shape,
// to the object the block returns, where the direct form's NoInfer<S> keeps it.
func takesBlockDirectly(c *checker.Checker, signatures []*checker.Signature, n int, inner *checker.Signature) bool {
	name := inner.Parameters()[0].Name()
	for _, signature := range signatures {
		parameters := signature.Parameters()
		if n < len(parameters) && acceptsArguments(signature, n+1) && parameters[n].Name() == name && isEffectType(c, c.GetTypeOfSymbol(parameters[n])) {
			return true
		}
	}
	return false
}

func acceptsArguments(signature *checker.Signature, n int) bool {
	return n >= signature.MinArgumentCount() && (signature.HasRestParameter() || n <= len(signature.Parameters()))
}

// takesEffectFirst reports whether signature's first parameter is an Effect.
func takesEffectFirst(c *checker.Checker, signature *checker.Signature) bool {
	parameters := signature.Parameters()
	return len(parameters) != 0 && isEffectType(c, c.GetTypeOfSymbol(parameters[0]))
}

// blockParamFormOf returns the form in which the block param call call
// passes its block, from the signature it resolved to.
func blockParamFormOf(c *checker.Checker, call *ast.Node) blockParamForm {
	if !isBlockParamCall(call) {
		return blockAppended
	}
	forms := blockParamForms(c)
	for signature := c.GetResolvedSignature(call); signature != nil; signature = signature.Target() {
		if form, ok := forms[signature]; ok {
			return form
		}
	}
	return blockAppended
}

// visitBlockParamCall lowers f(args, block) where the block goes elsewhere:
// f(block, args) for a data-first dual, f(args)(block) for a data-last one. A
// call that runs keeps its yield*.
func (tx *lowering) visitBlockParamCall(node *ast.Node, form blockParamForm) *ast.Node {
	f := tx.Factory()
	callee := tx.Visitor().VisitNode(node.Expression())
	var arguments []*ast.Node
	for _, argument := range node.Arguments() {
		arguments = append(arguments, tx.Visitor().VisitNode(argument))
	}
	block, args := arguments[len(arguments)-1], arguments[:len(arguments)-1]
	var lowered *ast.Node
	if form == blockPrepended {
		lowered = f.NewCallExpression(callee, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList(append([]*ast.Node{block}, args...)), ast.NodeFlagsNone)
	} else {
		head := f.NewCallExpression(callee, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList(args), ast.NodeFlagsNone)
		lowered = f.NewCallExpression(head, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{block}), ast.NodeFlagsNone)
	}
	tx.EmitContext().SetOriginal(lowered, node)
	if tx.inEffect && tx.plan.runsCall(node) {
		run := f.NewYieldExpression(f.NewToken(ast.KindAsteriskToken), lowered)
		tx.EmitContext().SetOriginal(run, node)
		return run
	}
	return lowered
}
