package effeck

import (
	"slices"
	"strconv"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/core"
	"github.com/microsoft/TypeScript/tsc/internal/ls"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// The rules of Effect's language service (@effect/language-service 0.87.5,
// D240). Effeck's syntax and call semantics make most of them impossible to
// break: there is no Effect.gen to misuse, a call already runs, and catch
// already catches. The rules that still apply are the EK48xx diagnostics
// below, each named after the Effect rules it stands for. Every one but
// EK4813 is adjustable (D241), and those marked off by default report
// nothing until compilerOptions.effeck or an @effeck-diagnostics comment
// turns them on. The editor's quick fixes for them are in lsrules_fixes.go,
// and the rules that also apply to .ts files in lsrules_ts.go.

var (
	// EK4801 (floatingEffect): a statement that discards an Effect value. {0} is the expression.
	EK4801 = message(4801, errorCategory, "{0} is an Effect value, and this statement discards it without running it.")
	// EK4802 (returnEffectInGen, effectInVoidSuccess): an Effect function that returns an Effect value. {0} is the value, {1} the function.
	EK4802 = message(4802, warningCategory, "{0} is an Effect value, but {1} already runs Effects, so its caller gets this program instead of its result.")
	// EK4803 (duplicatePackage): two versions of an Effect package. {0} is the package, {1} the number of versions.
	EK4803 = message(4803, warningCategory, "This program loads {1} versions of {0}, so services, fibers, and types can differ between its modules.")
	// EK4804 (globalErrorInEffectFailure, globalErrorInEffectCatch): an expected failure that is a global Error. {0} is its type.
	EK4804 = message(4804, warningCategory, "This failure is a global {0}, which has no _tag for a catch or a match to tell it apart by.")
	// EK4805 (extendsNativeError): a class that extends Error. {0} is the class, {1} what it extends.
	EK4805 = message(4805, warningCategory, "{0} extends {1}, but an Effeck failure is an error declaration.")
	// EK4806 (unknownInEffectCatch): a try hatch whose mapping returns unknown or any. {0} is the type.
	EK4806 = message(4806, warningCategory, "This try maps its failure to {0}, which tells a catch nothing about what failed.")
	// EK4807 (layerMergeAllWithDependencies): Layer.mergeAll of a layer that needs another. {0} is the dependent layer, {1} the services, {2} the provider.
	EK4807 = message(4807, warningCategory, "{0} needs {1}, which {2} provides, but Layer.mergeAll builds its layers side by side.")
	// EK4808 (multipleEffectProvide): a provide block that is all of another one.
	EK4808 = message(4808, warningCategory, "This provide block is all of another one, so each builds its layers on its own.")
	// EK4809 (leakingRequirements): a service whose every member needs one service. {0} is the service, {1} what each member needs.
	EK4809 = message(4809, warningCategory, "Every member of {0} needs {1}, so each caller must provide it.")
	// EK4810 (unsafeEffectTypeAssertion): an assertion that drops part of a channel. {0} is what it drops, {1} the channel.
	EK4810 = message(4810, warningCategory, "This assertion drops {0} from the Effect's {1} channel, so no contract checks it.")
	// EK4811 (anyUnknownInErrorContext; off by default): any or unknown in a channel. {0} is the source, {1} the type, {2} the channel.
	EK4811 = messageOff(4811, "{0} adds {1} to the {2} channel, where no catch or provide can narrow it.")
	// EK4812 (newPromise): new Promise in Effeck code.
	EK4812 = message(4812, warningCategory, "new Promise makes a promise, which Effeck code cannot wait for.")
	// EK4813 (outdatedApi): an Effect 3 name. {0} is the module, {1} the name, {2} the Effect 4 name.
	EK4813 = message(4813, errorCategory, "{0}.{1} is Effect 3 API; Effect 4 calls it {0}.{2}.")
	// EK4814 (overriddenSchemaConstructor): a constructor in a Schema class. {0} is the class.
	EK4814 = message(4814, errorCategory, "{0} overrides the constructor of a Schema class, which decoding does not call.")
	// EK4815 (effectInFailure): a throw of an Effect value. {0} is the value.
	EK4815 = message(4815, warningCategory, "{0} is an Effect value, so this throw fails with the program itself.")
	// EK4816 (strictEffectProvide; off by default): a layer provided outside main. {0} is the layer.
	EK4816 = messageOff(4816, "{0} is built here, but layers are provided once, in main.")
	// EK4817 (instanceOfSchema; off by default): instanceof a schema class. {0} is the class.
	EK4817 = messageOff(4817, "instanceof {0} tests the class; Schema.is({0}) tests the schema.")
	// EK4818 (preferSchemaOverJson; off by default): JSON.parse or JSON.stringify. {0} is the function.
	EK4818 = messageOff(4818, "JSON.{0} handles JSON without a schema to check it.")
)

// outdatedAPI is EK4813's text for a name that Effect 4 removed.
var outdatedAPI = EK4813.With("{0}.{1} is Effect 3 API, which Effect 4 removed.")

// messageOff returns a warning that is off by default.
func messageOff(code int32, text string) *Message {
	return &Message{Code: code, Category: warningCategory, Text: text, Off: true}
}

func init() {
	for _, m := range []*Message{EK4801, EK4802, EK4803, EK4804, EK4805, EK4806, EK4807, EK4808, EK4809, EK4810,
		EK4811, EK4812, EK4813, EK4814, EK4815, EK4816, EK4817, EK4818} {
		registry[m.Code] = m
	}
	fileChecks = append(fileChecks, checkEffectRules)
	ls.RegisterCodeFixProvider(inferredFormFix)
	ls.RegisterCodeFixProvider(trustedValueFix)
	ls.RegisterCodeFixProvider(callEffectFix)
	ls.RegisterCodeFixProvider(mergeAllFix)
	ls.RegisterCodeFixProvider(nestedProvideFix)
	ls.RegisterCodeFixProvider(assertionFix)
	ls.RegisterCodeFixProvider(outdatedAPIFix)
	ls.RegisterCodeFixProvider(outdatedAPITypeScriptFix)
	ls.RegisterCodeFixProvider(schemaIsFix)
}

// ruleCheck checks one .effeck file against the rules.
type ruleCheck struct {
	c    *checker.Checker
	a    *analyzer
	file *ast.SourceFile
}

// checkEffectRules reports the EK48xx diagnostics in file.
func checkEffectRules(c *checker.Checker, file *ast.SourceFile) {
	r := &ruleCheck{c: c, a: analyzerFor(c), file: file}
	r.checkDuplicatePackages()
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		switch {
		case isProvideBlock(node):
			r.checkProvide(node)
		case ast.IsExpressionStatement(node):
			r.checkFloating(node)
		case ast.IsReturnStatement(node):
			if fn := ast.GetContainingFunction(node); fn != nil && node.Expression() != nil {
				r.checkReturnedEffect(fn, node.Expression())
			}
		case ast.IsArrowFunction(node) && !ast.IsBlock(node.Body()):
			r.checkReturnedEffect(node, node.Body())
		case ast.IsThrowStatement(node):
			r.checkThrow(node)
		case ast.IsClassLike(node):
			r.checkClass(node)
		case ast.IsAsExpression(node) || ast.IsTypeAssertion(node):
			r.checkAssertion(node)
		case ast.IsNewExpression(node):
			r.checkNew(node)
		case ast.IsCallExpression(node):
			r.checkCall(node)
		case ast.IsPropertyAccessExpression(node):
			r.checkOutdated(node)
		case ast.IsBinaryExpression(node) && node.AsBinaryExpression().OperatorToken.Kind == ast.KindInstanceOfKeyword:
			r.checkInstanceOf(node)
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// checkFloating reports EK4801 at statement, an expression statement, when it
// discards an Effect value. A call at a module's top level is EK1001's.
func (r *ruleCheck) checkFloating(statement *ast.Node) {
	expression := statement.Expression()
	if isProvideLayer(statement) || isDiscardingForm(expression) || isHelperCall(expression) {
		return
	}
	body := inLowerableBody(statement)
	if !body && ast.IsCallExpression(ast.SkipParentheses(expression)) && ast.GetContainingFunction(statement) == nil {
		return
	}
	if !isEffectType(r.c, r.c.GetTypeAtLocation(expression)) {
		return
	}
	d := report(r.c, expression, EK4801, messageText(expression, "This expression"))
	switch {
	case body:
		note(d, "Call it to run it: {0}.", callText(expression))
	case inHatch(statement):
		note(d, "A hatch body is TypeScript, where an Effect runs only through a runner. Run it outside the hatch.")
	default:
		note(d, "Call it inside an Effeck function to run it.")
	}
}

// isHelperCall reports whether expression is a call that the parser wrote
// for an Effeck form, such as $await(x) for `await x` or $join(x) for
// `join x`, whose own diagnostics say what is wrong with it.
func isHelperCall(expression *ast.Node) bool {
	expression = ast.SkipParentheses(expression)
	return ast.IsCallExpression(expression) && ast.IsIdentifier(expression.Expression()) && strings.HasPrefix(expression.Expression().Text(), "$")
}

// isDiscardingForm reports whether expression is a form whose value a
// statement means to discard, or that runs on its own: an assignment, void,
// delete, or a producer's yield.
func isDiscardingForm(expression *ast.Node) bool {
	expression = ast.SkipParentheses(expression)
	switch expression.Kind {
	case ast.KindVoidExpression, ast.KindDeleteExpression, ast.KindYieldExpression:
		return true
	}
	return ast.IsAssignmentExpression(expression, false)
}

// checkReturnedEffect reports EK4802 when fn, an Effect function without a
// written contract, returns value, an Effect value: its caller runs fn and
// gets the program back.
func (r *ruleCheck) checkReturnedEffect(fn *ast.Node, value *ast.Node) {
	if !isLowerable(fn) || isProducer(fn) || fn.Type() != nil || !r.a.isEffect(fn) {
		return
	}
	if ast.IsCallExpression(ast.SkipParentheses(value)) && r.a.runs(ast.SkipParentheses(value)) {
		return // the call runs here; its success value is the result
	}
	if !isEffectType(r.c, r.c.GetTypeAtLocation(value)) {
		return
	}
	d := report(r.c, value, EK4802, messageText(value, "This value"), functionDisplayName(fn))
	note(d, "Call it to return its result: {0}. To return the program itself, write the contract.", callText(value))
}

// checkThrow reports what an Effeck function's throw statement fails with:
// an Effect value (EK4815), any or unknown (EK4811), or a global Error
// (EK4804). A hatch's throw is JavaScript's.
func (r *ruleCheck) checkThrow(statement *ast.Node) {
	fn := ast.GetContainingFunction(statement)
	value := statement.Expression()
	if fn == nil || !isLowerable(fn) || value == nil {
		return
	}
	t := r.c.GetTypeAtLocation(value)
	switch {
	case isEffectType(r.c, t):
		note(report(r.c, value, EK4815, messageText(value, "This value")), "Run it instead: {0}.", callText(value))
	case t.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown) != 0:
		note(report(r.c, value, EK4811, "This throw", r.c.TypeToString(t), "E"), "Throw an error declaration's instance.")
	default:
		if name := globalErrorName(r.c, t.Symbol()); name != "" {
			noteErrorDeclaration(report(r.c, value, EK4804, name))
		}
	}
}

// noteErrorDeclaration adds EK4804's repair to d.
func noteErrorDeclaration(d *ast.Diagnostic) {
	note(d, "Declare the failure and throw it: error Failed { message: string }, then throw new Failed({ message }).")
}

// globalErrorNames are the error classes of JavaScript's standard library.
var globalErrorNames = []string{"Error", "TypeError", "RangeError", "SyntaxError", "ReferenceError", "EvalError", "URIError", "AggregateError"}

// globalErrorName returns the name of the standard library's error class
// that symbol declares, its instance type or its constructor, or "".
func globalErrorName(c *checker.Checker, symbol *ast.Symbol) string {
	if symbol == nil || len(symbol.Declarations()) == 0 {
		return ""
	}
	name := strings.TrimSuffix(symbol.Name(), "Constructor")
	if !slices.Contains(globalErrorNames, name) {
		return ""
	}
	if !isGlobal(c, symbol) {
		return ""
	}
	return name
}

// checkClass reports EK4805 for a class that extends a standard library
// error, EK4814 for a constructor in a Schema class, and EK4809 for a
// service whose members all need one service.
func (r *ruleCheck) checkClass(class *ast.Node) {
	if classKindOf(class) == serviceClass {
		r.checkLeakingRequirements(class)
		return
	}
	extends := ast.GetClassExtendsHeritageElement(class)
	if extends == nil {
		return
	}
	base := extends.Expression()
	name := "This class"
	if class.Name() != nil {
		name = class.Name().Text()
	}
	if ast.IsIdentifier(base) {
		if errorName := globalErrorName(r.c, r.c.GetSymbolAtLocation(base)); errorName != "" {
			node := class
			if class.Name() != nil {
				node = class.Name()
			}
			note(report(r.c, node, EK4805, name, errorName), "Declare it with error instead, as in error {0} { message: string }, so that catch, match, and is can tell it apart by its _tag.", core.IfElse(class.Name() != nil, name, "Failed"))
		}
		return
	}
	if !isSchemaValue(r.c, r.c.GetTypeAtLocation(base)) {
		return
	}
	for _, member := range class.Members() {
		if ast.IsConstructorDeclaration(member) && member.Body() != nil {
			d := reportAt(r.c, r.file, constructorKeywordRange(r.file, member), EK4814, name)
			note(d, "Decoding builds instances without calling it. Add a static function that makes an instance instead.")
		}
	}
}

// constructorKeywordRange returns the span of member's constructor keyword.
func constructorKeywordRange(file *ast.SourceFile, member *ast.Node) core.TextRange {
	start := scanner.SkipTrivia(file.Text(), member.Pos())
	if keyword := strings.Index(file.Text()[start:member.End()], "constructor"); keyword >= 0 {
		start += keyword
	}
	return core.NewTextRange(start, start+len("constructor"))
}

// isSchemaValue reports whether t is a schema, such as Schema.String or a
// Schema class.
func isSchemaValue(c *checker.Checker, t *checker.Type) bool {
	return t != nil && c.GetPropertyOfType(t, schemaTypeID) != nil
}

// checkLeakingRequirements reports EK4809 when every Effect member of
// service, at least two, needs the same service: each member's caller must
// then provide what the service's own layer could capture once. Scope is
// exempt, and so is a service whose declaration says @effeck leakable.
func (r *ruleCheck) checkLeakingRequirements(service *ast.Node) {
	symbol := r.c.GetSymbolAtLocation(service.Name())
	if symbol == nil {
		return
	}
	var shared []*checker.Type
	members := 0
	for _, member := range r.c.GetPropertiesOfType(r.c.GetDeclaredTypeOfSymbol(symbol)) {
		if member.Name() == serviceBrandProperty {
			continue
		}
		requirements := memberRequirements(r.c, r.c.GetTypeOfSymbol(member))
		if requirements == nil {
			continue
		}
		var needs []*checker.Type
		for _, t := range requirements.Distributed() {
			if t.Flags()&checker.TypeFlagsNever == 0 && !isScopeType(t) && !isLeakable(t) {
				needs = append(needs, t)
			}
		}
		if members == 0 {
			shared = needs
		} else {
			shared = slices.DeleteFunc(shared, func(t *checker.Type) bool { return !slices.Contains(needs, t) })
		}
		members++
		if len(shared) == 0 {
			return
		}
	}
	if members < 2 {
		return
	}
	names := make([]string, len(shared))
	for i, t := range shared {
		names[i] = r.c.TypeToString(t)
	}
	d := report(r.c, service.Name(), EK4809, service.Name().Text(), strings.Join(names, " and "))
	note(d, "Read {0} where the layer is built, and drop it from the members' contracts. A service meant to pass along, such as a transaction, says @effeck leakable in its documentation comment.", names[0])
}

// memberRequirements returns the R of a service member's type: an Effect
// property's, or a method's that returns an Effect. It returns nil for any
// other member.
func memberRequirements(c *checker.Checker, t *checker.Type) *checker.Type {
	if isEffectType(c, t) {
		return channelType(c, t, "_R")
	}
	if signatures := c.GetSignaturesOfType(t, checker.SignatureKindCall); len(signatures) == 1 {
		if result := c.GetReturnTypeOfSignature(signatures[0]); isEffectType(c, result) {
			return channelType(c, result, "_R")
		}
	}
	return nil
}

// isLeakable reports whether the declaration of t, a service, says
// @effeck leakable in its documentation comment.
func isLeakable(t *checker.Type) bool {
	symbol := t.Symbol()
	if symbol == nil {
		return false
	}
	for _, declaration := range symbol.Declarations() {
		file := ast.GetSourceFileOfNode(declaration)
		for _, doc := range declaration.JSDoc(file) {
			if strings.Contains(scanner.GetTextOfNode(doc), "@effeck leakable") {
				return true
			}
		}
	}
	return false
}

// checkAssertion reports EK4810 when assertion, an as expression or type
// assertion from one Effect type to another, drops failures or
// requirements.
func (r *ruleCheck) checkAssertion(assertion *ast.Node) {
	source := r.c.GetTypeAtLocation(assertion.Expression())
	target := r.c.GetTypeFromTypeNode(assertion.Type())
	if !isEffectType(r.c, source) || !isEffectType(r.c, target) {
		return
	}
	for _, channel := range []struct{ name, property string }{{"E", "_E"}, {"R", "_R"}} {
		from, to := channelType(r.c, source, channel.property), channelType(r.c, target, channel.property)
		if from == nil || to == nil {
			continue
		}
		var dropped []string
		for _, t := range from.Distributed() {
			if !r.c.IsTypeAssignableTo(t, to) {
				dropped = append(dropped, r.c.TypeToString(t))
			}
		}
		if len(dropped) > 0 {
			d := report(r.c, assertion, EK4810, strings.Join(dropped, " | "), channel.name)
			note(d, "Remove the assertion, and handle or provide what it drops.")
			return
		}
	}
}

// checkNew reports EK4812 at new Promise outside a hatch, and outside a
// lambda that compiles to a plain callback, such as the one
// Effect.promise(() => new Promise(…)) calls.
func (r *ruleCheck) checkNew(node *ast.Node) {
	callee := node.Expression()
	if !ast.IsIdentifier(callee) || callee.Text() != "Promise" || inHatch(node) || !isGlobal(r.c, r.c.GetSymbolAtLocation(callee)) {
		return
	}
	if fn := ast.GetContainingFunction(node); fn != nil && isLambda(fn) && isLowerable(fn) && !r.a.isEffect(fn) {
		return
	}
	d := report(r.c, callee, EK4812)
	note(d, "Wrap a callback API with callback((resume) => …), or a promise you already have with sync(promise).")
}

// isGlobal reports whether the standard library declares symbol. Other
// declarations may add to it, as effect adds to the global Error.
func isGlobal(c *checker.Checker, symbol *ast.Symbol) bool {
	return symbol != nil && slices.ContainsFunc(symbol.Declarations(), func(d *ast.Node) bool {
		return c.Program().IsSourceFileDefaultLibrary(ast.GetSourceFileOfNode(d).PathKey())
	})
}

// checkCall checks a call: a try hatch's mapping (EK4806, EK4804),
// Layer.mergeAll (EK4807), JSON (EK4818), Effect.provide (EK4816), and
// a running call that adds any or unknown to a channel (EK4811).
func (r *ruleCheck) checkCall(call *ast.Node) {
	if hatchOf(call) == "try" {
		r.checkTryMapping(call)
		return
	}
	if inHatch(call) {
		return
	}
	switch module, name := effectFunctionOf(r.c, call.Expression()); {
	case module == "Layer" && name == "mergeAll":
		r.checkMergeAll(call)
	case module == "Effect" && name == "provide":
		for _, argument := range call.Arguments() {
			if isLayerType(r.c, r.c.GetTypeAtLocation(argument)) {
				r.checkProvidedLayer(call, argument)
			}
		}
	}
	if callee := call.Expression(); ast.IsPropertyAccessExpression(callee) && ast.IsIdentifier(callee.Expression()) && callee.Expression().Text() == "JSON" {
		if name := callee.Name().Text(); (name == "parse" || name == "stringify") && isGlobal(r.c, r.c.GetSymbolAtLocation(callee.Expression())) {
			d := report(r.c, callee, EK4818, name)
			note(d, "Decode with a schema instead: Schema.decodeUnknownEffect(Schema.fromJsonString(User))(text).")
		}
	}
	if inLowerableBody(call) && r.a.runs(call) {
		r.checkRunChannels(call)
	}
}

// checkTryMapping reports EK4806 or EK4804 at a try hatch's error mapping,
// its second argument (D231), that returns unknown, any, or a global Error.
func (r *ruleCheck) checkTryMapping(call *ast.Node) {
	arguments := call.Arguments()
	if len(arguments) < 2 {
		return
	}
	signatures := r.c.GetSignaturesOfType(r.c.GetTypeAtLocation(arguments[1]), checker.SignatureKindCall)
	if len(signatures) != 1 {
		return
	}
	result := r.c.GetReturnTypeOfSignature(signatures[0])
	if result.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown) != 0 {
		d := report(r.c, arguments[1], EK4806, r.c.TypeToString(result))
		note(d, "Return an error declaration's instance: try(code, (cause) => new Failed({ cause })).")
		return
	}
	if name := globalErrorName(r.c, result.Symbol()); name != "" {
		noteErrorDeclaration(report(r.c, arguments[1], EK4804, name))
	}
}

// checkRunChannels reports EK4811 when call, which runs, adds any or unknown
// to its function's E or R.
func (r *ruleCheck) checkRunChannels(call *ast.Node) {
	signature := r.c.GetResolvedSignature(call)
	if signature == nil {
		return
	}
	result := r.c.GetReturnTypeOfSignature(signature)
	if !isEffectType(r.c, result) {
		return
	}
	for _, channel := range []struct{ name, property string }{{"E", "_E"}, {"R", "_R"}} {
		if t := channelType(r.c, result, channel.property); t != nil && t.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown) != 0 {
			d := report(r.c, call, EK4811, messageText(call, "This call"), r.c.TypeToString(t), channel.name)
			note(d, "Give the callee a contract that names its {0}.", core.IfElse(channel.name == "E", "failures", "services"))
			return
		}
	}
}

// checkMergeAll reports EK4807 at each argument of call, a Layer.mergeAll
// call, that needs a service another argument provides.
func (r *ruleCheck) checkMergeAll(call *ast.Node) {
	arguments := call.Arguments()
	provided := make([]*checker.Type, len(arguments))
	needs := make([]*checker.Type, len(arguments))
	for i, argument := range arguments {
		provided[i], _, needs[i] = layerChannels(r.c, r.c.GetTypeAtLocation(argument))
	}
	for i, argument := range arguments {
		for j, provider := range arguments {
			if i == j || needs[i] == nil || provided[j] == nil {
				continue
			}
			var services []string
			for _, t := range needs[i].Distributed() {
				if slices.Contains(provided[j].Distributed(), t) {
					services = append(services, r.c.TypeToString(t))
				}
			}
			if len(services) > 0 {
				d := report(r.c, argument, EK4807, messageText(argument, "This layer"), strings.Join(services, " and "), messageText(provider, "another layer"))
				note(d, "Compose them with wire, which provides each layer's services to the layers that need them.")
				break
			}
		}
	}
}

// checkProvide reports EK4808 when block, a provide block, holds nothing but
// another provide block, and EK4816 for each layer it provides outside main.
func (r *ruleCheck) checkProvide(block *ast.Node) {
	statements := block.Statements()
	if len(statements) == 2 && isProvideBlock(statements[1]) {
		inner := statements[1]
		d := reportAt(r.c, r.file, core.NewTextRange(scanner.SkipTrivia(r.file.Text(), inner.Pos()), provideLayer(inner).End()), EK4808)
		note(d, "Provide every layer in one block: provide A, B { … }, or provide wire(A, B) { … } when one needs the other.")
	}
	for _, provider := range providerList(provideLayer(block)) {
		if !isServiceProvider(provider) && isLayerType(r.c, r.c.GetTypeAtLocation(provider)) {
			r.checkProvidedLayer(block, provider)
		}
	}
}

// providerList returns the providers of a provide block's layer expression:
// the elements of the array that a comma list becomes (D173), or the one
// provider.
func providerList(providers *ast.Node) []*ast.Node {
	if ast.IsArrayLiteralExpression(providers) {
		return providers.AsArrayLiteralExpression().Elements.Nodes
	}
	return []*ast.Node{providers}
}

// isServiceProvider reports whether provider is a provide block's
// Key = value, which the parser writes as a call to $provideService.
func isServiceProvider(provider *ast.Node) bool {
	return ast.IsCallExpression(provider) && ast.IsIdentifier(provider.Expression()) && provider.Expression().Text() == "$provideService"
}

// isLayerType reports whether t is a Layer.
func isLayerType(c *checker.Checker, t *checker.Type) bool {
	provided, _, _ := layerChannels(c, t)
	return provided != nil
}

// checkProvidedLayer reports EK4816 at layer, which site provides, unless
// site is inside a function named main.
func (r *ruleCheck) checkProvidedLayer(site *ast.Node, layer *ast.Node) {
	for fn := ast.GetContainingFunction(site); fn != nil; fn = ast.GetContainingFunction(fn) {
		if name := fn.Name(); name != nil && ast.IsIdentifier(name) && name.Text() == "main" {
			return
		}
	}
	note(report(r.c, layer, EK4816, messageText(layer, "This layer")), "Provide the layers in the entry module's main, and let other functions name what they need.")
}

// checkOutdated reports EK4813 at access, a name that Effect 3's Effect
// module had and Effect 4's does not, in place of TypeScript's TS2339.
func (r *ruleCheck) checkOutdated(access *ast.Node) {
	name := access.Name()
	if !ast.IsIdentifier(name) {
		return
	}
	change, ok := effect3API[name.Text()]
	if !ok || effectModuleOf(r.c, access.Expression()) != "Effect" || r.c.GetPropertyOfType(r.c.GetTypeAtLocation(access.Expression()), name.Text()) != nil {
		return
	}
	module := scanner.GetTextOfNode(access.Expression())
	if change.renamed != "" {
		d := report(r.c, name, EK4813, module, name.Text(), change.renamed)
		if change.note != "" {
			note(d, change.note)
		}
	} else {
		report(r.c, name, outdatedAPI, module, name.Text())
	}
	cover(r.c, name)
}

// checkInstanceOf reports EK4817 at an instanceof whose right side is a
// schema class.
func (r *ruleCheck) checkInstanceOf(node *ast.Node) {
	right := node.AsBinaryExpression().Right
	if isSchemaValue(r.c, r.c.GetTypeAtLocation(right)) {
		report(r.c, node, EK4817, messageText(right, "the class"))
	}
}

// checkDuplicatePackages reports EK4803 at file's first statement for each
// Effect package that the program loads more than one version of.
func (r *ruleCheck) checkDuplicatePackages() {
	statements := r.file.Statements.Nodes
	if len(statements) == 0 {
		return
	}
	for _, duplicate := range duplicatePackagesOf(r.c) {
		d := report(r.c, statements[0], EK4803, duplicate.name, strconv.Itoa(len(duplicate.versions)))
		for _, version := range duplicate.versions {
			note(d, "{0}@{1} is at {2}.", duplicate.name, version.version, version.directory)
		}
		note(d, "Install one version, for example by overriding it in package.json.")
	}
}

// duplicatePackage is an Effect package that a program loads more than one
// version of.
type duplicatePackage struct {
	name     string
	versions []packageVersion
}

// packageVersion is one version of a package, and the first directory the
// program loads it from.
type packageVersion struct {
	version   string
	directory string
}

type duplicatePackagesKey struct{}

// duplicatePackagesOf returns the Effect packages, effect, @effect/*, and
// effeck-runtime, that c's program loads more than one version of, as
// Effect's language service counts them. A file belongs to the package of
// the nearest package.json above it. Two copies of one version are not
// reported: a project may install them for tools, and the program's types
// agree.
func duplicatePackagesOf(c *checker.Checker) []duplicatePackage {
	return c.ExtensionState(duplicatePackagesKey{}, func() any {
		program := c.Program()
		versions := map[string][]packageVersion{}
		var names []string
		seen := map[string]bool{}
		for _, file := range program.SourceFiles() {
			directory := program.GetNearestAncestorDirectoryWithPackageJson(file.FileName().Directory())
			if directory == "" || seen[directory.AsString()] {
				continue
			}
			seen[directory.AsString()] = true
			info := program.GetPackageJsonInfo(directory.ResolveFile("package.json"))
			if !info.Exists() {
				continue
			}
			name, _ := info.Contents.Name.GetValue()
			if name != "effect" && name != "effeck-runtime" && !strings.HasPrefix(name, "@effect/") {
				continue
			}
			version, _ := info.Contents.Version.GetValue()
			if versions[name] == nil {
				names = append(names, name)
			}
			if !slices.ContainsFunc(versions[name], func(v packageVersion) bool { return v.version == version }) {
				versions[name] = append(versions[name], packageVersion{version, directory.AsString()})
			}
		}
		var duplicates []duplicatePackage
		for _, name := range names {
			if len(versions[name]) > 1 {
				duplicates = append(duplicates, duplicatePackage{name, versions[name]})
			}
		}
		return duplicates
	}).([]duplicatePackage)
}

// effectModuleOf returns the name of effect's module that expression names,
// such as Effect, or "".
func effectModuleOf(c *checker.Checker, expression *ast.Node) string {
	symbol := c.GetSymbolAtLocation(expression)
	if symbol != nil && symbol.Flags()&ast.SymbolFlagsAlias != 0 {
		symbol = c.GetAliasedSymbol(symbol)
	}
	if symbol == nil {
		return ""
	}
	for _, declaration := range symbol.Declarations() {
		if ast.IsSourceFile(declaration) {
			return effectModuleOfFile(declaration.AsSourceFile())
		}
	}
	return ""
}

// effectFunctionOf returns the module and name of the effect export that
// callee names, such as Layer and mergeAll for Layer.mergeAll, or "".
func effectFunctionOf(c *checker.Checker, callee *ast.Node) (module string, name string) {
	if ast.IsPropertyAccessExpression(callee) {
		callee = callee.Name()
	}
	if !ast.IsIdentifier(callee) {
		return "", ""
	}
	symbol := c.GetSymbolAtLocation(callee)
	if symbol != nil && symbol.Flags()&ast.SymbolFlagsAlias != 0 {
		symbol = c.GetAliasedSymbol(symbol)
	}
	if symbol == nil || len(symbol.Declarations()) == 0 {
		return "", ""
	}
	return effectModuleOfFile(ast.GetSourceFileOfNode(symbol.Declarations()[0])), symbol.Name()
}

// nodeText returns node's source text for a message, or otherwise when the
// text is long or spans lines.
func messageText(node *ast.Node, otherwise string) string {
	text := scanner.GetTextOfNode(node)
	if len(text) > 40 || strings.ContainsAny(text, "\r\n") {
		return otherwise
	}
	return text
}

// callText returns the text that calls expression: e() for e, and (e)()
// for an expression that needs parentheses first.
func callText(expression *ast.Node) string {
	text := messageText(expression, "the expression")
	if needsParentheses(expression) {
		return "(" + text + ")()"
	}
	return text + "()"
}

// needsParentheses reports whether expression needs parentheses before a
// call's argument list.
func needsParentheses(expression *ast.Node) bool {
	switch expression.Kind {
	case ast.KindIdentifier, ast.KindPropertyAccessExpression, ast.KindElementAccessExpression, ast.KindCallExpression, ast.KindParenthesizedExpression, ast.KindThisKeyword:
		return false
	}
	return true
}
