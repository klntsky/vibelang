package vibelang

import (
	"slices"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// A handler belongs to the fiber that installed it, so a control operation
// performed on another fiber cannot reach it (chapter 9, "Handlers and
// concurrency"; D20). Two checks keep operations on their handler's fiber:
//
//	error EK4201: Approval.approve is a control operation, and forEach with concurrency 4 runs it
//	  on other fibers, where the Approval handler on this fiber cannot be reached.
//	  Handle Approval inside the callback, or run it with concurrency 1.
//
// EK4201 reads a table of the Effect combinators that run an argument on
// another fiber (forks, races, timeouts, and any call with a concurrency
// option other than 1) and each argument's requirements. EK4202 rejects a call
// to a TypeScript function whose declared requirements hold a control
// operation, because its body can fork where no table sees it.

func init() {
	fileChecks = append(fileChecks, checkReach)
}

// forkingEffectFunctions are the functions of effect's Effect module that
// always run an argument on another fiber.
var forkingEffectFunctions = []string{
	"forkChild", "forkDetach", "forkIn", "forkScoped",
	"race", "raceAll", "raceAllFirst", "raceFirst",
	"timeout", "timeoutOption", "timeoutOrElse",
}

func checkReach(c *checker.Checker, file *ast.SourceFile) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if ast.IsCallExpression(node) {
			if what, fix := forkingCall(c, file, node); what != "" {
				checkForkedArguments(c, file, node, what, fix)
			}
			checkProducerArguments(c, file, node)
			checkTypeScriptCallee(c, file, node)
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// forkingCall describes how call runs an argument on another fiber, such as
// "forEach with concurrency 4", with the fix that keeps it on this fiber, or
// returns "" when call runs its arguments on the calling fiber.
func forkingCall(c *checker.Checker, file *ast.SourceFile, call *ast.Node) (what string, fix string) {
	if name := runtimeExport(c, call.Expression()); name == "fork" || name == "$fork" {
		return "fork", "run it on this fiber"
	}
	module, name := effectModuleExport(c, call.Expression())
	if module == "" {
		return "", ""
	}
	if concurrency := concurrencyOption(file, call); concurrency != "" {
		return name + " with concurrency " + concurrency, "run it with concurrency 1"
	}
	if module == "Effect" && slices.Contains(forkingEffectFunctions, name) {
		return module + "." + name, "run it on this fiber"
	}
	if module == "Effect" && (name == "zip" || name == "zipWith") && hasOption(file, call, "concurrent", "true") {
		return name + " with concurrent: true", "run it without concurrent: true"
	}
	return "", ""
}

// effectModuleExport returns the module of effect, such as "Effect" or
// "Stream", and the export that callee refers to, or "" and "".
func effectModuleExport(c *checker.Checker, callee *ast.Node) (module string, name string) {
	if !ast.IsIdentifier(callee) && !ast.IsPropertyAccessExpression(callee) {
		return "", ""
	}
	symbol := c.GetSymbolAtLocation(callee)
	if symbol != nil && symbol.Flags()&ast.SymbolFlagsAlias != 0 {
		symbol = c.GetAliasedSymbol(symbol)
	}
	if symbol == nil || len(symbol.Declarations()) == 0 {
		return "", ""
	}
	file := ast.GetSourceFileOfNode(symbol.Declarations()[0])
	for _, m := range []string{"Effect", "Stream"} {
		if isEffectModuleFile(file, m) {
			return m, symbol.Name()
		}
	}
	return "", ""
}

// concurrencyOption returns the text of the concurrency option an object
// literal argument of call passes, unless it is 1, or "".
func concurrencyOption(file *ast.SourceFile, call *ast.Node) string {
	for _, argument := range call.Arguments() {
		if value := optionValue(argument, "concurrency"); value != nil {
			if text := nodeText(file, value); text != "1" {
				return text
			}
		}
	}
	return ""
}

// hasOption reports whether an object literal argument of call sets option
// to the expression text value.
func hasOption(file *ast.SourceFile, call *ast.Node, option string, value string) bool {
	for _, argument := range call.Arguments() {
		if v := optionValue(argument, option); v != nil && nodeText(file, v) == value {
			return true
		}
	}
	return false
}

// optionValue returns the value of property name in argument, an object
// literal, or nil.
func optionValue(argument *ast.Node, name string) *ast.Node {
	if !ast.IsObjectLiteralExpression(argument) {
		return nil
	}
	for _, property := range argument.Properties() {
		if ast.IsPropertyAssignment(property) && ast.IsIdentifier(property.Name()) && property.Name().Text() == name {
			return property.Initializer()
		}
	}
	return nil
}

// checkForkedArguments reports EK4201 for each control operation that an
// argument of call, which runs it on another fiber, still requires.
func checkForkedArguments(c *checker.Checker, file *ast.SourceFile, call *ast.Node, what string, fix string) {
	for _, argument := range call.Arguments() {
		for _, control := range controlsIn(c, requirementsOf(c, c.GetTypeAtLocation(argument), 0)) {
			at, operation := performSite(c, file, argument, control)
			d := report(c, at, EK4201, operation, what, control.Symbol().Name())
			note(d, "Handle {0} inside the {1}, or {2}.", control.Symbol().Name(), argumentKind(argument), fix)
		}
	}
}

// argumentKind names what an argument is for a fix: a callback or a program.
func argumentKind(argument *ast.Node) string {
	if ast.IsArrowFunction(argument) || ast.IsFunctionExpression(argument) {
		if len(argument.Parameters()) > 0 {
			return "callback"
		}
	}
	return "program"
}

// requirementsOf returns the requirements of the Effects that t holds: t
// itself, what its call signatures return, and the elements or properties of
// an array or record of them.
func requirementsOf(c *checker.Checker, t *checker.Type, depth int) []*checker.Type {
	if t == nil || depth > 3 {
		return nil
	}
	if t.Flags()&checker.TypeFlagsUnion != 0 {
		var out []*checker.Type
		for _, member := range t.Types() {
			out = append(out, requirementsOf(c, member, depth+1)...)
		}
		return out
	}
	if effect := effectOfValue(c, t); effect != nil {
		if r := channelType(c, effect, "_R"); r != nil {
			return []*checker.Type{r}
		}
		return nil
	}
	var out []*checker.Type
	for _, signature := range c.GetSignaturesOfType(t, checker.SignatureKindCall) {
		out = append(out, requirementsOf(c, c.GetReturnTypeOfSignature(signature), depth+1)...)
	}
	if len(out) > 0 {
		return out
	}
	if t.Flags()&checker.TypeFlagsObject != 0 {
		for _, info := range c.GetIndexInfosOfType(t) {
			out = append(out, requirementsOf(c, info.ValueType(), depth+1)...)
		}
		if c.IsArrayType(t) || checker.IsTupleType(t) {
			for _, element := range c.GetTypeArguments(t) {
				out = append(out, requirementsOf(c, element, depth+1)...)
			}
		}
	}
	return out
}

// controlsIn returns the control operations among the members of types.
func controlsIn(c *checker.Checker, types []*checker.Type) []*checker.Type {
	var out []*checker.Type
	for _, t := range types {
		for _, member := range t.Distributed() {
			if isControlType(member) && !slices.Contains(out, member) {
				out = append(out, member)
			}
		}
	}
	return out
}

// isControlType reports whether t is the type of a control class.
func isControlType(t *checker.Type) bool {
	symbol := t.Symbol()
	if symbol == nil {
		return false
	}
	return slices.ContainsFunc(symbol.Declarations(), func(d *ast.Node) bool { return classKindOf(d) == controlClass })
}

// performSite finds where argument performs control: the first call in it
// that looks control up, named as written, such as Approval.approve, or the
// first call whose requirements hold control. Without one it returns
// argument and the control's name.
func performSite(c *checker.Checker, file *ast.SourceFile, argument *ast.Node, control *checker.Type) (*ast.Node, string) {
	var found *ast.Node
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		if node.Kind == ast.KindClassDeclaration || node.Kind == ast.KindClassExpression {
			return false
		}
		if ast.IsCallExpression(node) {
			if identifier := serviceLookup(c, node); identifier == control {
				found = node
				return true
			}
		}
		return node.ForEachChild(visit)
	}
	visit(argument)
	if found != nil {
		return found, nodeText(file, found.Expression())
	}
	return argument, control.Symbol().Name()
}

// checkProducerArguments reports EK4201 for a producer passed to call, a
// Stream operator or any other function, whose requirements hold a control
// operation: run as a Stream, a producer runs on a fiber of its own (D26).
// for...of and yield* run it on this fiber instead.
func checkProducerArguments(c *checker.Checker, file *ast.SourceFile, call *ast.Node) {
	for _, argument := range call.Arguments() {
		producer := producerOfCall(c, ast.SkipParentheses(argument))
		if producer == nil {
			continue
		}
		requirements := streamChannel(c, c.GetTypeAtLocation(argument), "_R")
		if requirements == nil {
			continue
		}
		for _, control := range controlsIn(c, []*checker.Type{requirements}) {
			operation := control.Symbol().Name()
			if site, _ := performSite(c, ast.GetSourceFileOfNode(producer), producer.Body(), control); site != producer.Body() {
				operation = nodeText(ast.GetSourceFileOfNode(producer), site.Expression())
			}
			d := report(c, argument, EK4201, operation, "the producer "+nodeText(file, argument)+" passed to "+nodeText(file, call.Expression()), control.Symbol().Name())
			note(d, "A producer used as a Stream runs on a fiber of its own. Loop over it with for...of, or handle {0} inside the producer.", control.Symbol().Name())
		}
	}
}

// producerOfCall returns the producer that call, a call expression, calls,
// or nil.
func producerOfCall(c *checker.Checker, call *ast.Node) *ast.Node {
	if !ast.IsCallExpression(call) {
		return nil
	}
	if signature := c.GetResolvedSignature(call); signature != nil {
		if fn := implementationOf(signature.Declaration()); fn != nil && isProducer(fn) {
			return fn
		}
	}
	return nil
}

// checkTypeScriptCallee reports EK4202 when call calls a function declared
// outside VibeLang whose declared requirements hold a control
// operation. A generic combinator such as Effect.forEach declares its
// requirements as a type parameter, so only its arguments' requirements hold
// the operation, and the forking table above judges it.
func checkTypeScriptCallee(c *checker.Checker, file *ast.SourceFile, call *ast.Node) {
	signature := c.GetResolvedSignature(call)
	if signature == nil || signature.Declaration() == nil {
		return
	}
	declaration := signature.Declaration()
	if isVibeLangFile(ast.GetSourceFileOfNode(declaration)) || !ast.IsFunctionLike(declaration) {
		return
	}
	declared := c.GetReturnTypeOfSignature(c.GetSignatureFromDeclaration(declaration))
	controls := controlsIn(c, requirementsOf(c, declared, 0))
	if len(controls) == 0 {
		return
	}
	callee := nodeText(file, call.Expression())
	for _, control := range controls {
		d := report(c, call.Expression(), EK4202, callee, control.Symbol().Name())
		note(d, "TypeScript code can perform {0} on another fiber, where this handler cannot reach it.", control.Symbol().Name())
		note(d, "Write {0} in VibeLang, where the compiler sees every fork, or handle {1} inside it.", callee, control.Symbol().Name())
	}
}
