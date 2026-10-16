package effectscript

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/tspath"
)

// Services (design D1, D45).
//
// An abstract class in an .es file whose instance members are all abstract
// declares a service. The checker sees such a class as this TypeScript would
// declare it:
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
// and yield* Users work as they do with a Context.Service class.

// serviceBrandProperty is the optional property that brands a service's
// shape with its key.
const serviceBrandProperty = "~esc/service"

type classKind uint8

const (
	ordinaryClass classKind = iota
	serviceClass            // an abstract class that declares a service (D1)
)

// classKindOf reports what class declares. A class declaration in an .es
// file is a service when it is abstract, has no type parameters, no
// implements clause, and no extends clause, and has at least one instance
// member, every one of them an abstract method or property. Static members
// are allowed. Every other class is ordinary.
func classKindOf(class *ast.Node) classKind {
	if class == nil || !ast.IsClassDeclaration(class) || class.Name() == nil ||
		!ast.HasSyntacticModifier(class, ast.ModifierFlagsAbstract) || class.TypeParameters() != nil ||
		len(ast.GetImplementsHeritageClauseElements(class)) != 0 ||
		!isEffectScriptFile(ast.GetSourceFileOfNode(class)) {
		return ordinaryClass
	}
	if ast.GetClassExtendsHeritageElement(class) != nil {
		return ordinaryClass
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
	return serviceClass
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

// classBases is the checker's class base hook. A service class inherits the
// static side of Context.Service<Self, Self>, and its instances inherit the
// brand { readonly "~esc/service"?: key }.
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
	key := c.NewTypeReference(c.GetDeclaredTypeOfSymbol(service), []*checker.Type{class, class})
	brand := c.NewOptionalReadonlyPropertyType(serviceBrandProperty, c.GetStringLiteralType(s.key(declaration)))
	return key, brand
}
