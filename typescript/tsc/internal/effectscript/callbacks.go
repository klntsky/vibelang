package effectscript

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// A JavaScript function such as Array.prototype.map calls its callback
// synchronously, on a stack frame the runtime cannot pause or capture, so a
// callback passed where a plain function is expected must compile to plain
// JavaScript (design D9, D10). A lambda there that performs an Effect
// operation, and an EffectScript function passed by name that can pause, are
// ES4301:
//
//	error ES4301: this callback can pause (Users.find), but Array.prototype.map calls it synchronously.
//	  Use Effect.forEach(ids, (id) => Users.find(id)) or a for...of loop.

func init() {
	fileChecks = append(fileChecks, checkCallbacks)
}

// pausingCallback is ES4301's text for a lambda, which names what pauses.
var pausingCallback = ES4301.With("this callback can pause ({0}), but {1} calls it synchronously.")

// pausingMethod is ES4301's text for a method that implements a plain
// function member, such as Equal.Equal's [Equal.symbol] (D55).
var pausingMethod = ES4301.With("{0} can pause ({1}), but {2} declares it as a plain function, which its callers call synchronously.")

func checkCallbacks(c *checker.Checker, file *ast.SourceFile) {
	a := analyzerFor(c)
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		switch {
		case isLambda(node):
			if a.finalForm(node) == formPlain {
				if pause := a.firstPause(node); pause != nil {
					call, _ := ownerCall(node)
					d := report(c, node, pausingCallback, nodeText(file, pause.Expression()), calleeName(c, file, call))
					note(d, "Use {0} or a for...of loop.", forEachFix(c, file, call, node))
				}
			}
		case node.Kind == ast.KindMethodDeclaration && isLowerable(node):
			if owner := a.plainMemberOwner(node); owner != "" {
				if pause := a.firstPause(node); pause != nil {
					report(c, node.Name(), pausingMethod, nodeText(file, node.Name()), nodeText(file, pause.Expression()), owner)
				}
			}
		case node.Kind == ast.KindIdentifier:
			if a.finalConversion(node) == convertRejected {
				call, _ := ownerCall(node)
				d := report(c, node, ES4301, node.Text(), calleeName(c, file, call))
				note(d, "Use {0} or a for...of loop.", forEachFix(c, file, call, node))
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// plainMemberOwner returns the name of the type whose member method
// implements when that member is a plain function, such as Equal.Equal for
// [Equal.symbol] (D55), and "" otherwise: the method then compiles to a plain
// method, so it must not pause. The member comes from the interfaces that
// method's class implements, or from the contextual type of method's object
// literal.
func (a *analyzer) plainMemberOwner(method *ast.Node) string {
	c := a.checker
	symbol := c.GetSymbolOfDeclaration(method)
	if symbol == nil || ast.IsStatic(method) {
		return ""
	}
	var owners []*ast.Node
	var expected []*checker.Type
	switch parent := method.Parent; {
	case ast.IsObjectLiteralExpression(parent):
		if t := c.GetContextualTypeForObjectLiteralElement(method, checker.ContextFlagsSignature); t != nil {
			expected = append(expected, t)
		}
	case ast.IsClassLike(parent):
		for _, heritage := range ast.GetImplementsHeritageClauseElements(parent) {
			if member := c.GetPropertyOfType(c.GetTypeFromTypeNode(heritage), symbol.Name()); member != nil {
				expected = append(expected, c.GetTypeOfSymbol(member))
				owners = append(owners, heritage)
			}
		}
	}
	e := a.classify(expected)
	if !e.plainFn || e.effectFn || e.program {
		return ""
	}
	if len(owners) > 0 {
		return nodeText(ast.GetSourceFileOfNode(method), owners[0])
	}
	return "its contextual type"
}

// calleeName names what call calls, for a diagnostic: Array.prototype.map
// for a method an interface or class declares, called on an instance, else
// the callee as written, such as Array.from or Order.mapInput.
func calleeName(c *checker.Checker, file *ast.SourceFile, call *ast.Node) string {
	if call == nil {
		return "its caller"
	}
	if signature := c.GetResolvedSignature(call); signature != nil && !isStaticReceiver(c, receiverOf(call)) {
		if d := signature.Declaration(); d != nil && (ast.IsMethodSignatureDeclaration(d) || ast.IsMethodDeclaration(d)) && d.Parent != nil {
			if owner := d.Parent.Name(); owner != nil && ast.IsIdentifier(owner) && ast.IsIdentifier(d.Name()) {
				return owner.Text() + ".prototype." + d.Name().Text()
			}
		}
	}
	return nodeText(file, call.Expression())
}

// forEachFix writes the Effect.forEach call that replaces call, a call such
// as ids.map(f) whose callback f can pause, when the object call's callee
// belongs to is iterable, and else names Effect.forEach.
func forEachFix(c *checker.Checker, file *ast.SourceFile, call *ast.Node, callback *ast.Node) string {
	if receiver := receiverOf(call); receiver != nil && c.IteratedType(c.GetTypeAtLocation(receiver)) != nil {
		return "Effect.forEach(" + nodeText(file, receiver) + ", " + nodeText(file, callback) + ")"
	}
	return "Effect.forEach"
}

// receiverOf returns the object that call's callee belongs to, ids in
// ids.map(f), or nil.
func receiverOf(call *ast.Node) *ast.Node {
	if call != nil && ast.IsPropertyAccessExpression(call.Expression()) {
		return call.Expression().Expression()
	}
	return nil
}

// isStaticReceiver reports whether receiver names a namespace, a class, an
// enum, or a value that a declaration file declares, such as Array in
// Array.from, rather than an instance whose methods its prototype holds.
func isStaticReceiver(c *checker.Checker, receiver *ast.Node) bool {
	if receiver == nil || !ast.IsEntityNameExpression(receiver) {
		return false
	}
	symbol := c.GetSymbolAtLocation(receiver)
	if symbol != nil && symbol.Flags()&ast.SymbolFlagsAlias != 0 {
		symbol = c.GetAliasedSymbol(symbol)
	}
	if symbol == nil {
		return false
	}
	if symbol.Flags()&(ast.SymbolFlagsModule|ast.SymbolFlagsClass|ast.SymbolFlagsEnum) != 0 {
		return true
	}
	declaration := symbol.ValueDeclaration()
	return declaration != nil && ast.IsVariableDeclaration(declaration) && ast.GetSourceFileOfNode(declaration).IsDeclarationFile
}

// nodeText returns the source text of node, without leading trivia.
func nodeText(file *ast.SourceFile, node *ast.Node) string {
	return file.Text()[scanner.SkipTrivia(file.Text(), node.Pos()):node.End()]
}
