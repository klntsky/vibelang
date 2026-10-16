package effectscript

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

// Services and control operations (design D1, D2, D41, D45).
//
// An abstract class in an .es file whose instance members are all abstract
// declares a service, and one that extends Control declares a control
// operation. The checker sees such a class as this TypeScript would declare
// it:
//
//	interface Users {
//	  find(id: string): Effect<User, UserNotFound, never>
//	  readonly "~esc/service"?: "app/users#Users"
//	}
//	class Users extends Context.Service<Users, Users> {}
//
// As a type, Users is the service's shape, branded with its key so that two
// services with the same members stay distinct requirements. As a value,
// Users is a Context key, so Layer.succeed(Users, …), Effect.provideService,
// and yield* Users work as they do with a Context.Service class. In an .es
// function body, Users.find(id) calls a member through the key and Users()
// returns the implementation.

// serviceBrandProperty is the optional property that brands a service's
// shape with its key.
const serviceBrandProperty = "~esc/service"

// contextServiceTypeID names the property that every Effect 4 Context key
// has: `readonly [ServiceTypeId]: ServiceTypeId`.
const contextServiceTypeID = "~effect/Context/Service"

type classKind uint8

const (
	ordinaryClass classKind = iota
	serviceClass            // an abstract class that declares a service (D1)
	controlClass            // an abstract class that extends Control (D2)
)

// classKindOf reports what class declares. A class declaration in an .es
// file is a service when it is abstract, has no type parameters, no
// implements clause, and no extends clause, and has at least one instance
// member, every one of them an abstract method or property. With
// `extends Control` instead of no extends clause, it is a control operation.
// Static members are allowed. Every other class is ordinary.
func classKindOf(class *ast.Node) classKind {
	if class == nil || !ast.IsClassDeclaration(class) || class.Name() == nil ||
		!ast.HasSyntacticModifier(class, ast.ModifierFlagsAbstract) || class.TypeParameters() != nil ||
		len(ast.GetImplementsHeritageClauseElements(class)) != 0 ||
		!isEffectScriptFile(ast.GetSourceFileOfNode(class)) {
		return ordinaryClass
	}
	kind := serviceClass
	if extends := ast.GetClassExtendsHeritageElement(class); extends != nil {
		if !isControlReference(extends) {
			return ordinaryClass
		}
		kind = controlClass
	}
	instanceMembers := 0
	for _, member := range class.Members() {
		if ast.IsSemicolonClassElement(member) || ast.IsStatic(member) {
			continue
		}
		if !ast.IsMethodDeclaration(member) && !ast.IsPropertyDeclaration(member) || !ast.HasSyntacticModifier(member, ast.ModifierFlagsAbstract) {
			return ordinaryClass
		}
		instanceMembers++
	}
	if instanceMembers == 0 {
		return ordinaryClass
	}
	return kind
}

// isControlReference reports whether heritage, an extends clause element,
// names the prelude's Control.
func isControlReference(heritage *ast.Node) bool {
	expression := heritage.Expression()
	return ast.IsIdentifier(expression) && expression.Text() == "Control" && heritage.TypeArguments() == nil
}

// hasStaticDefault reports whether class declares `static readonly default`,
// which makes its key a Context.Reference that needs no provider (D40).
func hasStaticDefault(class *ast.Node) bool {
	for _, member := range class.Members() {
		if ast.IsPropertyDeclaration(member) && ast.IsStatic(member) && ast.HasSyntacticModifier(member, ast.ModifierFlagsReadonly) {
			if name := member.Name(); name != nil && ast.IsIdentifier(name) && name.Text() == "default" {
				return true
			}
		}
	}
	return false
}

// services remembers one checker's answers about service classes. A checker
// is used by one goroutine at a time, so it needs no lock of its own.
type services struct {
	checker         *checker.Checker
	contextModule   *ast.Symbol // effect's Context module, nil if the program lacks it
	contextSearched bool
	keys            map[*ast.Node]string
}

type servicesKey struct{}

// servicesFor returns the services that remember c's answers. They live on c,
// so they are freed with c.
func servicesFor(c *checker.Checker) *services {
	return c.ExtensionState(servicesKey{}, func() any {
		return &services{checker: c, keys: map[*ast.Node]string{}}
	}).(*services)
}

// contextExport returns the symbol that effect's Context module exports as
// name, or nil when the program does not include effect's declarations.
func (s *services) contextExport(name string) *ast.Symbol {
	if !s.contextSearched {
		s.contextSearched = true
		for _, file := range s.checker.Program().SourceFiles() {
			if isEffectModuleFile(file, "Context") {
				s.contextModule = file.Symbol
				break
			}
		}
	}
	if s.contextModule == nil {
		return nil
	}
	for _, export := range s.checker.GetExportsOfModule(s.contextModule) {
		if export.Name() == name {
			return export
		}
	}
	return nil
}

// isEffectModuleFile reports whether file declares the module effect/<name>
// of the effect package.
func isEffectModuleFile(file *ast.SourceFile, name string) bool {
	fileName := file.FileName().AsString()
	return strings.HasSuffix(fileName, "/effect/dist/"+name+".d.ts") || strings.HasSuffix(fileName, "/effect/src/"+name+".ts")
}

// key returns the key of the service that class declares: its package name,
// its module path within the package, and its name, as in
// "@acme/app/src/users#Users". A package without a name uses its directory's
// name, and a file outside every package uses its path from the program's
// common source directory.
func (s *services) key(class *ast.Node) string {
	if key, ok := s.keys[class]; ok {
		return key
	}
	file := ast.GetSourceFileOfNode(class)
	fileName := file.FileName()
	program := s.checker.Program()
	module := string(fileName.RemoveExtension(tspath.ExtensionEs))
	prefix := ""
	if directory := program.GetNearestAncestorDirectoryWithPackageJson(fileName.Directory()); directory != "" {
		prefix = directory.BaseName()
		if info := program.GetPackageJsonInfo(directory.ResolveFile("package.json")); info.Exists() {
			if name, ok := info.Contents.Name.GetValue(); ok && name != "" {
				prefix = name
			}
		}
		module = strings.TrimPrefix(module, directory.AsString()+"/")
	} else if common := program.CommonSourceDirectory(); common != "" {
		module = strings.TrimPrefix(module, common.AsString()+"/")
	}
	key := module + "#" + class.Name().Text()
	if prefix != "" {
		key = prefix + "/" + key
	}
	s.keys[class] = key
	return key
}

// classBases is the checker's class base hook. A service or control class
// inherits the static side of Context.Service<Self, Self>, or of
// Context.Service<never, Self> when a static default makes its key a
// reference, and its instances inherit the brand
// { readonly "~esc/service"?: key }. A control class's own extends clause
// is ignored.
func classBases(c *checker.Checker, class *checker.Type) (*checker.Type, *checker.Type) {
	symbol := class.Symbol()
	if symbol == nil {
		return nil, nil
	}
	declaration := symbol.ValueDeclaration()
	if classKindOf(declaration) == ordinaryClass {
		return nil, nil
	}
	s := servicesFor(c)
	service := s.contextExport("Service")
	if service == nil {
		return nil, nil
	}
	identifier := class
	if hasStaticDefault(declaration) {
		identifier = c.GetNeverType()
	}
	key := c.NewTypeReference(c.GetDeclaredTypeOfSymbol(service), []*checker.Type{identifier, class})
	brand := c.NewOptionalReadonlyPropertyType(serviceBrandProperty, c.GetStringLiteralType(s.key(declaration)))
	return key, brand
}

// serviceOfKey returns the identifier and the shape of the service that t,
// a Context key, looks up, or nil and nil when t is not a Context key.
func serviceOfKey(c *checker.Checker, t *checker.Type) (identifier *checker.Type, shape *checker.Type) {
	if t == nil || t.Flags()&checker.TypeFlagsObject == 0 || c.GetPropertyOfType(t, contextServiceTypeID) == nil {
		return nil, nil
	}
	return c.GetTypeOfPropertyOfType(t, "Identifier"), c.GetTypeOfPropertyOfType(t, "Service")
}

// isMemberCallSite reports whether node is the callee of a call
// `S.member(…)` in the body of an .es function.
func isMemberCallSite(node *ast.Node) bool {
	if !ast.IsPropertyAccessExpression(node) || ast.IsPrivateIdentifier(node.Name()) {
		return false
	}
	call := node.Parent
	return call != nil && ast.IsCallExpression(call) && call.Expression() == node && call.QuestionDotToken() == nil && inLowerableBody(call)
}

// serviceMember returns the identifier and the shape of the service whose
// member node, at a member call site, calls: receiver is the apparent type
// of S in `S.member(…)`, and it must be a Context key whose service declares
// member. It returns nil and nil otherwise.
func serviceMember(c *checker.Checker, node *ast.Node, receiver *checker.Type) (identifier *checker.Type, shape *checker.Type) {
	identifier, shape = serviceOfKey(c, receiver)
	if shape == nil || c.GetPropertyOfType(shape, node.Name().Text()) == nil {
		return nil, nil
	}
	return identifier, shape
}

// memberReceiver is the checker's member receiver hook. In a call
// `S.member(…)` in an .es function body, where S is a Context key whose
// service declares member, member is read from the service's shape (design
// D41). A member the service declares therefore wins over the key's own
// members, such as pipe, of, and use.
func memberReceiver(c *checker.Checker, node *ast.Node, receiver *checker.Type) *checker.Type {
	if !isMemberCallSite(node) {
		return receiver
	}
	if _, shape := serviceMember(c, node, receiver); shape != nil {
		return shape
	}
	return receiver
}

// serviceLookup returns the identifier of the service that call looks up,
// or nil when call is not a service member call. A service member call looks
// the service up in the current Context, so the service joins R whatever the
// member returns, and then calls the member.
func serviceLookup(c *checker.Checker, call *ast.Node) *checker.Type {
	callee := call.Expression()
	if !isMemberCallSite(callee) {
		return nil
	}
	identifier, _ := serviceMember(c, callee, c.GetApparentType(c.GetTypeAtLocation(callee.Expression())))
	return identifier
}

// isServiceLookup reports whether call is a service member call.
func isServiceLookup(c *checker.Checker, call *ast.Node) bool {
	return serviceLookup(c, call) != nil
}

// isEffectValueCallSite reports whether call has the form `p()` in the body
// of an .es function.
func isEffectValueCallSite(call *ast.Node) bool {
	return len(call.Arguments()) == 0 && call.TypeArguments() == nil && call.QuestionDotToken() == nil && inLowerableBody(call)
}

// effectValueCall is the checker's call signatures hook. In an .es function
// body, a value whose type is an Effect, called with no arguments, runs:
// Users() gives the service's implementation, and Port() reads a Config
// (design D24). The call gets the signature () => Effect<A, E, R>.
func effectValueCall(c *checker.Checker, call *ast.Node, callee *checker.Type) []*checker.Signature {
	if !isEffectValueCallSite(call) {
		return nil
	}
	if effect := effectOfValue(c, callee); effect != nil {
		return []*checker.Signature{c.NewCallSignature(effect)}
	}
	return nil
}

// isEffectValueCall reports whether call runs its callee, an Effect value.
func isEffectValueCall(c *checker.Checker, call *ast.Node) bool {
	if !isEffectValueCallSite(call) {
		return false
	}
	callee := c.GetApparentType(c.GetTypeAtLocation(call.Expression()))
	return len(c.GetSignaturesOfType(callee, checker.SignatureKindCall)) == 0 && effectOfValue(c, callee) != nil
}

// effectOfValue returns Effect<A, E, R> for t, a type whose values are
// Effects, such as a Context key or a Config, or nil when t is not one.
func effectOfValue(c *checker.Checker, t *checker.Type) *checker.Type {
	if t == nil || t.Flags()&checker.TypeFlagsObject == 0 {
		return nil
	}
	effect := effectInterface(c, t)
	if effect == nil {
		return nil
	}
	a, e, r := varianceChannel(c, t, "_A"), varianceChannel(c, t, "_E"), varianceChannel(c, t, "_R")
	if a == nil || e == nil || r == nil {
		return nil
	}
	return c.NewTypeReference(effect, []*checker.Type{a, e, r})
}

// varianceChannel reads one channel of t's `~effect/Effect` variance
// property, whether or not t is the Effect interface itself.
func varianceChannel(c *checker.Checker, t *checker.Type, name string) *checker.Type {
	variance := c.GetTypeOfPropertyOfType(t, effectTypeID)
	if variance == nil {
		return nil
	}
	covariant := c.GetTypeOfPropertyOfType(variance, name)
	if covariant == nil {
		return nil
	}
	signatures := c.GetSignaturesOfType(covariant, checker.SignatureKindCall)
	if len(signatures) != 1 {
		return nil
	}
	return c.GetReturnTypeOfSignature(signatures[0])
}
