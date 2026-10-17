package effeck

import (
	"fmt"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// Checks of service and control classes (design D1, D2): a member without a
// full contract (EK2101), a key that two services share (EK3001), and a
// service used as a base class (EK3002, EK3003, EK3004), declared generic
// (EK3005), or with a static member that shadows a service member (EK3006).

func init() { fileChecks = append(fileChecks, checkServices) }

func checkServices(c *checker.Checker, file *ast.SourceFile) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		switch {
		case ast.IsClassDeclaration(node) || ast.IsClassExpression(node):
			checkServiceDeclaration(c, node)
			checkExtendsService(c, node)
		case ast.IsNewExpression(node):
			if service := serviceNamedBy(c, node.Expression()); service != nil {
				report(c, node, EK3003, service.Name().Text())
				cover(c, node) // TS2511: an abstract class cannot be instantiated
			}
		case ast.IsBinaryExpression(node) && node.AsBinaryExpression().OperatorToken.Kind == ast.KindInstanceOfKeyword:
			if service := serviceNamedBy(c, node.AsBinaryExpression().Right); service != nil {
				report(c, node, EK3004, service.Name().Text())
			}
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// checkServiceDeclaration reports EK2101, EK3001, EK3005, and EK3006 for
// class, if it declares a service or a control operation.
func checkServiceDeclaration(c *checker.Checker, class *ast.Node) {
	kind := shapeKindOf(class)
	if kind == ordinaryClass {
		return
	}
	name := class.Name().Text()
	if class.TypeParameters() != nil {
		note(report(c, class.Name(), EK3005), "Use an interface for the generic type.")
		return
	}
	s := servicesFor(c)
	if others := s.sharingKey(class); len(others) > 0 {
		d := report(c, class.Name(), EK3001, s.key(class))
		for _, other := range others {
			note(d, "{0} at {1} has the same key. Rename one.", other.Name().Text(), position(other.Name()))
		}
	}
	instance := map[string]bool{}
	for _, member := range class.Members() {
		if !ast.IsStatic(member) && member.Name() != nil && ast.IsIdentifier(member.Name()) {
			instance[member.Name().Text()] = true
		}
	}
	for _, member := range class.Members() {
		memberName := member.Name()
		if memberName == nil || !ast.IsIdentifier(memberName) {
			continue
		}
		if ast.IsStatic(member) {
			if instance[memberName.Text()] {
				note(report(c, memberName, EK3006, memberName.Text()), "Rename one: {0}.{1} would call the service member.", name, memberName.Text())
			}
			continue
		}
		if ast.IsMethodDeclaration(member) && !isFullContract(c, member.Type()) && !isUnresolved(c, member.Type()) {
			at := member.Type()
			if at == nil {
				at = memberName
				cover(c, memberName) // TS7010: an implicit any return type
			}
			d := report(c, at, EK2101, name+"."+memberName.Text())
			note(d, "Write Effect<{0}, E, R>; use Effect<{0}, never, never> if it cannot fail and needs nothing.", writtenSuccess(member.Type()))
		}
	}
}

// isFullContract reports whether typeNode, a return type annotation, states
// a full contract: an Effect whose three channels are written out. A
// reference written Effect<A> or Effect<A, E> leaves channels to the
// interface's defaults, so it is not one; an alias of a full Effect is.
func isFullContract(c *checker.Checker, typeNode *ast.Node) bool {
	if typeNode == nil || !isEffectType(c, c.GetTypeFromTypeNode(typeNode)) {
		return false
	}
	if ast.IsTypeReferenceNode(typeNode) && isEffectName(typeNode.AsTypeReferenceNode().TypeName) {
		return len(typeNode.TypeArguments()) == 3
	}
	return true
}

// isUnresolved reports whether typeNode names a type the checker could not
// resolve, which TypeScript already reports.
func isUnresolved(c *checker.Checker, typeNode *ast.Node) bool {
	return typeNode != nil && c.GetTypeFromTypeNode(typeNode) == c.GetErrorType()
}

// writtenSuccess returns the text of the success type that typeNode, a
// return type annotation that is not a full contract, states: the first type
// argument of a partial Effect<A>, the annotation itself otherwise, or A when
// there is none.
func writtenSuccess(typeNode *ast.Node) string {
	switch {
	case typeNode == nil:
		return "A"
	case ast.IsTypeReferenceNode(typeNode) && isEffectName(typeNode.AsTypeReferenceNode().TypeName):
		if arguments := typeNode.TypeArguments(); len(arguments) > 0 {
			return scanner.GetTextOfNode(arguments[0])
		}
		return "A"
	}
	return scanner.GetTextOfNode(typeNode)
}

// isEffectName reports whether name, a type reference's name, is Effect or
// a qualified name ending in Effect, such as Effect.Effect.
func isEffectName(name *ast.Node) bool {
	if ast.IsQualifiedName(name) {
		name = name.AsQualifiedName().Right
	}
	return name.Text() == "Effect"
}

// checkExtendsService reports EK3002 for a class that extends a service or a
// control operation: a service is a key, not a base class.
func checkExtendsService(c *checker.Checker, class *ast.Node) {
	extends := ast.GetClassExtendsHeritageElement(class)
	if extends == nil {
		return
	}
	service := serviceNamedBy(c, extends.Expression())
	if service == nil {
		return
	}
	d := report(c, extends.Expression(), EK3002)
	name := "Implementation"
	if class.Name() != nil {
		name = class.Name().Text()
	}
	note(d, "Implement it instead: class {0} implements {1} { … }", name, service.Name().Text())
	// TypeScript's complaints about inherited abstract members restate this.
	cover(c, extends.Parent)
	if class.Name() != nil {
		cover(c, class.Name())
	}
}

// serviceNamedBy returns the declaration of the service or control class
// that expr names, following imports, or nil.
func serviceNamedBy(c *checker.Checker, expr *ast.Node) *ast.Node {
	if !ast.IsIdentifier(expr) && !ast.IsPropertyAccessExpression(expr) {
		return nil
	}
	symbol := c.GetSymbolAtLocation(expr)
	if symbol != nil && symbol.Flags()&ast.SymbolFlagsAlias != 0 {
		symbol = c.GetAliasedSymbol(symbol)
	}
	if symbol == nil || classKindOf(symbol.ValueDeclaration()) == ordinaryClass {
		return nil
	}
	return symbol.ValueDeclaration()
}

// sharingKey returns the other service and control classes in the program
// whose key is class's key.
func (s *services) sharingKey(class *ast.Node) []*ast.Node {
	if s.byKey == nil {
		s.byKey = map[string][]*ast.Node{}
		for _, file := range s.checker.Program().SourceFiles() {
			if !isEffeckFile(file) {
				continue
			}
			var visit func(node *ast.Node) bool
			visit = func(node *ast.Node) bool {
				if classKindOf(node) != ordinaryClass {
					key := s.key(node)
					s.byKey[key] = append(s.byKey[key], node)
				}
				return node.ForEachChild(visit)
			}
			file.AsNode().ForEachChild(visit)
		}
	}
	var others []*ast.Node
	for _, other := range s.byKey[s.key(class)] {
		if other != class {
			others = append(others, other)
		}
	}
	return others
}

// position returns node's file name and line as file.effeck:12.
func position(node *ast.Node) string {
	file := ast.GetSourceFileOfNode(node)
	line := scanner.GetECMALineOfPosition(file, scanner.SkipTrivia(file.Text(), node.Pos()))
	return fmt.Sprintf("%s:%d", file.FileName().BaseName(), line+1)
}
