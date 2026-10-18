package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// Checks of implementations against the service they implement (Chapter 4,
// "Writing a provider" and "Requirements that would leak"). A provider's
// object literal, or a class that implements a service, has members whose
// Effects must fit the service members' contracts. TypeScript reports a
// mismatch as TS2322 or TS2345 on the whole provider; these checks name the
// member and the failure or the service:
//
//	error EK2001: DbError can escape UsersLive.find, but Users.find's contract allows only UserNotFound.
//	error EK3101: UsersAudited.find looks up Audit each time it is called, but
//	  Users.find(id: string): User!UserNotFound! allows no call-time requirements.
//
// A requirement that leaks this way is a service the member looks up when it
// is called, in the caller's Context, which the contract does not declare.

func init() { fileChecks = append(fileChecks, checkProviders) }

func checkProviders(c *checker.Checker, file *ast.SourceFile) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		switch {
		case ast.IsObjectLiteralExpression(node):
			checkProviderObject(c, node)
		case ast.IsClassDeclaration(node) || ast.IsClassExpression(node):
			checkProviderClass(c, node)
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// checkProviderObject checks each method and function-valued property of
// object against the type its contextual type gives that member.
func checkProviderObject(c *checker.Checker, object *ast.Node) {
	contextual := c.GetContextualType(object, checker.ContextFlagsNone)
	if contextual == nil {
		return
	}
	owner := serviceNameOf(c, contextual)
	for _, member := range object.AsObjectLiteralExpression().Properties.Nodes {
		name := member.Name()
		if name == nil || !ast.IsIdentifier(name) && !ast.IsStringLiteral(name) {
			continue
		}
		var implementation *ast.Node
		switch {
		case ast.IsMethodDeclaration(member):
			implementation = member
		case ast.IsPropertyAssignment(member) && (ast.IsArrowFunction(member.Initializer()) || ast.IsFunctionExpression(member.Initializer())):
			implementation = member.Initializer()
		default:
			continue
		}
		declared := c.GetContextualTypeForObjectLiteralElement(member, checker.ContextFlagsNone)
		if declared == nil {
			continue
		}
		if checkMember(c, member, implementation, providerName(object), owner, name.Text(), declared, c.GetTypeAtLocation(implementation)) {
			coverMismatch(c, object, member, implementation)
		}
	}
}

// checkProviderClass checks each method of class against the member it
// implements through class's implements clause.
func checkProviderClass(c *checker.Checker, class *ast.Node) {
	heritage := ast.GetImplementsHeritageClauseElements(class)
	if len(heritage) == 0 {
		return
	}
	holder := "this class"
	if class.Name() != nil {
		holder = class.Name().Text()
	}
	for _, member := range class.Members() {
		name := member.Name()
		if !ast.IsMethodDeclaration(member) || ast.IsStatic(member) || name == nil || !ast.IsIdentifier(name) {
			continue
		}
		for _, element := range heritage {
			implemented := c.GetTypeFromTypeNode(element)
			property := c.GetPropertyOfType(implemented, name.Text())
			if property == nil {
				continue
			}
			if checkMember(c, member, member, holder, serviceNameOf(c, implemented), name.Text(), c.GetTypeOfSymbol(property), c.GetTypeOfSymbol(member.Symbol())) {
				coverErrorSpan(c, name) // TS2416: the method does not fit the member it implements
			}
		}
	}
}

// checkMember compares implementation, the type of fn, holder's member name,
// with declared, the type owner declares for it. Both must be functions that
// return Effects; it reports a failure the contract does not allow (EK2001)
// at at, and a requirement it does not declare (EK3101) at the call in fn
// that looks it up, and reports whether it reported anything.
func checkMember(c *checker.Checker, at, fn *ast.Node, holder, owner, name string, declared, implementation *checker.Type) bool {
	declaredSignature := singleCallSignature(c, declared)
	implementationSignature := singleCallSignature(c, implementation)
	if declaredSignature == nil || implementationSignature == nil {
		return false
	}
	contract := c.GetReturnTypeOfSignature(declaredSignature)
	effect := c.GetReturnTypeOfSignature(implementationSignature)
	if !isEffectType(c, contract) || !isEffectType(c, effect) {
		return false
	}
	allowedErrors, allowedRequirements := channelType(c, contract, "_E"), channelType(c, contract, "_R")
	if containsTypeParameter(allowedErrors) || containsTypeParameter(allowedRequirements) {
		return false
	}
	reported := false
	for _, failure := range unionMembers(channelType(c, effect, "_E")) {
		if c.IsTypeAssignableTo(failure, allowedErrors) {
			continue
		}
		reported = true
		allows := "allows only " + c.TypeToString(allowedErrors)
		if allowedErrors.Flags()&checker.TypeFlagsNever != 0 {
			allows = "allows no failures"
		}
		d := report(c, at, EK2001, c.TypeToString(failure), holder+"."+name, owner+"."+name+"'s contract "+allows)
		note(d, "Handle it, add {0} to {1}.{2}'s contract, or make it a defect with Effect.orDie.", c.TypeToString(failure), owner, name)
	}
	for _, service := range unionMembers(channelType(c, effect, "_R")) {
		if c.IsTypeAssignableTo(service, allowedRequirements) {
			continue
		}
		reported = true
		signature := owner + "." + name + c.SignatureToStringEx(declaredSignature, nil, checker.TypeFormatFlagsNone, nil)
		lookup, member := lookupOf(c, fn, service)
		where := at
		if lookup != nil {
			where = lookup
		}
		var d *ast.Diagnostic
		if allowedRequirements.Flags()&checker.TypeFlagsNever != 0 {
			d = report(c, where, EK3101, holder+"."+name, c.TypeToString(service), bangTypes(signature))
		} else {
			d = report(c, where, EK3101.With("{0} looks up {1} each time it is called, but {2} allows only {3}."), holder+"."+name, c.TypeToString(service), bangTypes(signature), c.TypeToString(allowedRequirements))
		}
		serviceName := c.TypeToString(service)
		variable := serviceName
		if variable != "" {
			variable = string(variable[0]|0x20) + variable[1:]
		}
		if member != "" {
			note(d, "Capture it once when the layer is built: const {0} = {1}()   …   {0}.{2}(…)", variable, serviceName, member)
		} else {
			note(d, "Capture it once when the layer is built: const {0} = {1}()", variable, serviceName)
		}
		note(d, "or declare it: {0}(…): {1}", name, bangText(c.TypeToString(channelType(c, contract, "_A")), c.TypeToString(allowedErrors), c.TypeToString(c.GetUnionType([]*checker.Type{allowedRequirements, service}))))
	}
	return reported
}

// lookupOf returns the call in fn's own body that looks service up, and the
// member it calls, such as Audit.record(…) and "record", or nil and "".
func lookupOf(c *checker.Checker, fn *ast.Node, service *checker.Type) (*ast.Node, string) {
	var found *ast.Node
	member := ""
	forEachCallInBody(fn, func(call *ast.Node) bool {
		identifier := serviceLookup(c, call)
		if identifier == nil || !c.IsTypeAssignableTo(identifier, service) {
			return false
		}
		found = call
		member = call.Expression().Name().Text()
		return true
	})
	return found, member
}

// singleCallSignature returns t's call signature when it has exactly one.
func singleCallSignature(c *checker.Checker, t *checker.Type) *checker.Signature {
	if t == nil {
		return nil
	}
	signatures := c.GetSignaturesOfType(t, checker.SignatureKindCall)
	if len(signatures) != 1 {
		return nil
	}
	return signatures[0]
}

func containsTypeParameter(t *checker.Type) bool {
	if t == nil {
		return false
	}
	if t.Flags()&checker.TypeFlagsTypeParameter != 0 {
		return true
	}
	if t.Flags()&checker.TypeFlagsUnion != 0 {
		for _, member := range t.Types() {
			if containsTypeParameter(member) {
				return true
			}
		}
	}
	return false
}

// serviceNameOf returns the name of the service whose shape t is, or "the
// service" when t has no name of its own.
func serviceNameOf(c *checker.Checker, t *checker.Type) string {
	if symbol := t.Symbol(); symbol != nil && symbol.Name() != "" && symbol.Name()[0] != '_' {
		return symbol.Name()
	}
	return "the service"
}

// providerName returns the name of the variable, property, or class that
// holds the provider object belongs to, such as UsersLive in
// const UsersLive = Layer.effect(Users, () => ({ … })).
func providerName(object *ast.Node) string {
	for node := object.Parent; node != nil; node = node.Parent {
		switch {
		case ast.IsVariableDeclaration(node) || ast.IsPropertyDeclaration(node) || ast.IsPropertyAssignment(node):
			if name := node.Name(); name != nil && ast.IsIdentifier(name) {
				return name.Text()
			}
			return "this provider"
		case ast.IsClassLike(node) || ast.IsSourceFile(node):
			return "this provider"
		}
	}
	return "this provider"
}

// coverMismatch hides TypeScript's own report of the mismatch that a member
// check explained, at the error spans where TypeScript reports it: the
// member's name, an arrow member's expression body, and the header of each
// function that encloses object and is itself a call argument, such as the
// `() =>` of Layer.effect(Users, () => { … return { … } }).
func coverMismatch(c *checker.Checker, object, member, implementation *ast.Node) {
	coverErrorSpan(c, member.Name())
	if ast.IsArrowFunction(implementation) && !ast.IsBlock(implementation.Body()) {
		coverErrorSpan(c, implementation.Body())
	}
	for node := object; node != nil && !ast.IsSourceFile(node); node = node.Parent {
		if ast.IsFunctionLike(node) && node.Parent != nil && (ast.IsCallExpression(node.Parent) || ast.IsNewExpression(node.Parent)) {
			coverErrorSpan(c, node)
		}
	}
}

// coverErrorSpan hides TypeScript's diagnostics within node's error span,
// such as the name of a declaration or the `() =>` of an arrow function.
func coverErrorSpan(c *checker.Checker, node *ast.Node) {
	file := ast.GetSourceFileOfNode(node)
	state := diagnosticsStateOf(c)
	state.covered[file] = append(state.covered[file], scanner.GetErrorRangeForNode(file, node))
}
