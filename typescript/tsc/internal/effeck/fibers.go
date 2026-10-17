package effeck

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/ls"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// The fiber forms (D166-D170) parse into calls (parser/effeck_fibers.go):
// `fork x` is fork(x), `await x` is $await(x), select is a labeled block
// around $select(...), and `for concurrent` is $forConcurrent(...). The
// checker, the analyzer, and the lowering read them as those calls; this file
// reports what each form cannot mean and tells a hover what it does.

var (
	// EK4601: await before a call that already runs. {0} is the call.
	EK4601 = message(4601, errorCategory, "{0} already runs here, so there is nothing to await.")
	// EK4602: await on a promise. {0} is the operand.
	EK4602 = message(4602, errorCategory, "{0} is a promise, but await in Effeck code waits for a Fiber or a Deferred.")
	// EK4603: await on something that is not a Fiber, a Deferred, or an array or object of them. {0} is the operand's type.
	EK4603 = message(4603, errorCategory, "await waits for a Fiber, a Deferred, or an array or object of them, but this is {0}.")
	// EK4604: await inside a hatch body that is not async.
	EK4604 = message(4604, errorCategory, "await in a hatch body is JavaScript's await, which needs an async function.")
	// EK4605: a break that would leave a for concurrent loop.
	EK4605 = message(4605, errorCategory, "break cannot leave a for concurrent loop: each item runs in its own fiber.")
	// EK4606: a return in the body of a for concurrent loop.
	EK4606 = message(4606, errorCategory, "return cannot leave a for concurrent loop: each item runs in its own fiber.")
)

func init() {
	for _, m := range []*Message{EK4601, EK4602, EK4603, EK4604, EK4605, EK4606} {
		registry[m.Code] = m
	}
	fileChecks = append(fileChecks, checkFiberForms)
	ls.RegisterQuickInfoHook(fiberFormQuickInfo)
}

// fiberHelper returns the runtime helper that call, a call the parser wrote
// for a fiber form (its callee is synthesized), calls: "$await", "$select", or "$forConcurrent". It
// returns "" for any other node.
func fiberHelper(call *ast.Node) string {
	if call == nil || !ast.IsCallExpression(call) {
		return ""
	}
	callee := call.Expression()
	if callee.Kind != ast.KindIdentifier || callee.Flags&ast.NodeFlagsReparsed == 0 {
		return ""
	}
	switch name := callee.Text(); name {
	case "$await", "$select", "$forConcurrent":
		return name
	}
	return ""
}

func checkFiberForms(c *checker.Checker, file *ast.SourceFile) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		switch fiberHelper(node) {
		case "$await":
			checkAwait(c, file, node)
		case "$forConcurrent":
			if args := node.Arguments(); len(args) == 3 {
				checkConcurrentBody(c, args[2].Body())
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// checkAwait reports an await whose operand it cannot wait for.
func checkAwait(c *checker.Checker, file *ast.SourceFile, call *ast.Node) {
	keyword := scanner.GetRangeOfTokenAtPosition(file, call.Pos())
	if inHatch(call) {
		note(reportAt(c, file, keyword, EK4604), "Make the function async, or move the await out of the hatch.")
		cover(c, call)
		return
	}
	args := call.Arguments()
	if len(args) != 1 {
		return
	}
	operand := args[0]
	if inner := ast.SkipParentheses(operand); ast.IsCallExpression(inner) && callReturnsEffect(c, inner) {
		note(reportAt(c, file, keyword, EK4601, nodeText(file, operand)), "Remove await: the call runs and waits. To run it concurrently, fork it and await the fiber.")
		cover(c, call)
		return
	}
	t := c.GetTypeAtLocation(operand)
	if t == nil {
		return
	}
	if c.GetPromisedTypeOfPromise(t) != nil {
		note(reportAt(c, file, keyword, EK4602, nodeText(file, operand)), "Wrap it in a hatch: try(async () => …) or sync(async () => …).")
		cover(c, call)
		return
	}
	if !isAwaitable(c, t, 0) {
		d := reportAt(c, file, keyword, EK4603, c.TypeToString(t))
		if isEffectType(c, t) {
			note(d, "A program runs when you call it; fork it to run it concurrently, then await the fiber.")
		}
		cover(c, call)
	}
}

// callReturnsEffect reports whether call's signature returns an Effect, so
// in Effeck code the call runs where it stands.
func callReturnsEffect(c *checker.Checker, call *ast.Node) bool {
	signature := c.GetResolvedSignature(call)
	return signature != nil && isEffectType(c, c.GetReturnTypeOfSignature(signature))
}

// isAwaitable reports whether await can wait for a value of type t: a Fiber,
// a Deferred, or (at depth 0) an array, tuple, or object whose elements all
// are Fibers or Deferreds.
func isAwaitable(c *checker.Checker, t *checker.Type, depth int) bool {
	if t.Flags()&checker.TypeFlagsUnion != 0 {
		for _, member := range t.Types() {
			if !isAwaitable(c, member, depth) {
				return false
			}
		}
		return true
	}
	if c.GetPropertyOfType(t, "~effect/Fiber") != nil || c.GetPropertyOfType(t, "~effect/Deferred") != nil {
		return true
	}
	if depth > 0 || t.Flags()&checker.TypeFlagsObject == 0 || isEffectType(c, t) || c.GetPromisedTypeOfPromise(t) != nil || len(c.GetSignaturesOfType(t, checker.SignatureKindCall)) != 0 {
		return false
	}
	if checker.IsTupleType(t) {
		for _, element := range c.GetTypeArguments(t) {
			if !isAwaitable(c, element, depth+1) {
				return false
			}
		}
		return true
	}
	if c.IsArrayType(t) {
		return isAwaitable(c, c.GetElementTypeOfArrayType(t), depth+1)
	}
	for _, property := range c.GetPropertiesOfType(t) {
		if !isAwaitable(c, c.GetTypeOfSymbol(property), depth+1) {
			return false
		}
	}
	return true
}

// checkConcurrentBody reports a break that would leave a for concurrent loop
// (the parser flags it) and a return the body writes; the parser's own
// returns, which stand for a continue, are synthesized.
func checkConcurrentBody(c *checker.Checker, body *ast.Node) {
	if body == nil {
		return
	}
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		switch {
		case node.Kind == ast.KindBreakStatement && node.Flags&ast.NodeFlagsEffeck != 0:
			note(report(c, node, EK4605), "Use continue to end this item, or throw to stop every item.")
			cover(c, node)
			return false
		case node.Kind == ast.KindReturnStatement && node.Flags&ast.NodeFlagsReparsed == 0:
			note(report(c, node, EK4606), "Use continue to end this item.")
			return false
		case isFunctionBoundary(node):
			return false
		}
		return node.ForEachChild(visit)
	}
	body.ForEachChild(visit)
}

// fiberFormQuickInfo tells a hover on the keyword of a fiber form what the
// form does. The parser anchors the helper's callee on the keyword, so a
// hover there finds the callee, or the call itself.
func fiberFormQuickInfo(c *checker.Checker, node *ast.Node) (string, string) {
	call := node
	if call != nil && call.Parent != nil && fiberHelper(call.Parent) != "" && call.Parent.Expression() == call {
		call = call.Parent
	}
	switch fiberHelper(call) {
	case "$await":
		value := c.GetTypeAtLocation(call)
		paragraph := "Waits for the operand and returns `" + c.TypeToStringEx(value, call, hoverTypeFlags, nil) + "`."
		if signature := c.GetResolvedSignature(call); signature != nil {
			if errors := channelType(c, c.GetReturnTypeOfSignature(signature), "_E"); errors != nil && errors.Flags()&checker.TypeFlagsNever == 0 {
				paragraph += " Its failures propagate: `" + c.TypeToStringEx(errors, call, hoverTypeFlags, nil) + "`."
			}
		}
		return "await", paragraph
	case "$select":
		return "select", "Starts each case in its own fiber; the first to finish wins, its statements run, and the other cases are interrupted. A failure propagates."
	case "$forConcurrent":
		return "for concurrent", "Runs the body for each item in its own fiber and joins them all before the loop ends; a failure interrupts the rest."
	}
	return "", ""
}
