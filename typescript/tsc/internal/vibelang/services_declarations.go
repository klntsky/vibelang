package vibelang

import (
	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/compiler"
	"github.com/microsoft/TypeScript/tsc/internal/printer"
)

// The declaration file of a service or control class (design D45). The
// declaration transform writes the class as it is in source:
//
//	export declare abstract class Users {
//	    abstract find(id: string): Effect.Effect<User, UserNotFound, never>;
//	    static readonly layer: Layer.Layer<Users, never, never>;
//	}
//
// TypeScript code that imports it must see what the checker sees in .vibe
// files: an interface for the shape, branded with the key, merged with a
// class whose static side is a Context key, the runtime's ServiceKey:
//
//	export interface Users {
//	    find(id: string): Effect.Effect<User, UserNotFound, never>;
//	    readonly "~vibelang/service"?: "app/users#Users";
//	}
//	declare const Users_base: import("vibelang-runtime").ServiceKey<Users>;
//	export declare abstract class Users extends Users_base {
//	    static readonly layer: Layer.Layer<Users, never, never>;
//	}
//
// A service with a static default extends ReferenceKey instead.

func init() { compiler.RegisterDeclarationTransformerHook(serviceDeclarations) }

func serviceDeclarations(resolver printer.EmitResolver, file *ast.SourceFile) func(*ast.SourceFile) *ast.SourceFile {
	r, ok := resolver.(*checker.EmitResolver)
	if !ok || !isVibeLangFile(file) {
		return nil
	}
	keys := map[*ast.Node]string{}
	r.WithChecker(func(c *checker.Checker) {
		s := servicesFor(c)
		for _, statement := range file.Statements.Nodes {
			if classKindOf(statement) != ordinaryClass {
				keys[statement] = s.key(statement)
			}
		}
	})
	if len(keys) == 0 {
		return nil
	}
	ec := resolver.EmitContext()
	return func(declarations *ast.SourceFile) *ast.SourceFile {
		var statements []*ast.Node
		changed := false
		for _, statement := range declarations.Statements.Nodes {
			if class := ec.MostOriginal(statement); ast.IsClassDeclaration(statement) && keys[class] != "" {
				statements = append(statements, serviceDeclaration(ec, statement, class, keys[class])...)
				changed = true
				continue
			}
			statements = append(statements, statement)
		}
		if !changed {
			return declarations
		}
		list := ec.Factory.NewNodeList(statements)
		list.Loc = declarations.Statements.Loc
		return ec.Factory.UpdateSourceFile(declarations, list, declarations.EndOfFileToken).AsSourceFile()
	}
}

// serviceDeclaration returns the declarations of a service: declared, the
// class the declaration transform wrote for class, becomes an interface for
// its instance members and the brand, a `<Name>_base` constant typed with the
// runtime's key type, and a class that extends it with the static members.
func serviceDeclaration(ec *printer.EmitContext, declared, class *ast.Node, key string) []*ast.Node {
	f := ec.Factory
	name := declared.Name().Text()
	isDefault := ast.HasSyntacticModifier(declared, ast.ModifierFlagsDefault)
	exported := ast.HasSyntacticModifier(declared, ast.ModifierFlagsExport) && !isDefault

	var shape, statics []*ast.Node
	for _, member := range declared.Members() {
		switch {
		case ast.IsStatic(member):
			statics = append(statics, member)
		case ast.IsMethodDeclaration(member):
			shape = append(shape, f.NewMethodSignatureDeclaration(nil /*modifiers*/, member.Name(), member.PostfixToken(), member.TypeParameterList(), member.ParameterList(), member.Type()))
		case ast.IsPropertyDeclaration(member):
			var modifiers *ast.ModifierList
			if ast.HasSyntacticModifier(member, ast.ModifierFlagsReadonly) {
				modifiers = f.NewModifierList([]*ast.Node{f.NewModifier(ast.KindReadonlyKeyword)})
			}
			shape = append(shape, f.NewPropertySignatureDeclaration(modifiers, member.Name(), member.PostfixToken(), member.Type(), nil /*initializer*/))
		}
	}
	shape = append(shape, f.NewPropertySignatureDeclaration(
		f.NewModifierList([]*ast.Node{f.NewModifier(ast.KindReadonlyKeyword)}),
		f.NewStringLiteral(serviceBrandProperty, ast.TokenFlagsNone),
		f.NewToken(ast.KindQuestionToken),
		f.NewLiteralTypeNode(f.NewStringLiteral(key, ast.TokenFlagsNone)),
		nil, /*initializer*/
	))
	var interfaceModifiers *ast.ModifierList
	if exported {
		interfaceModifiers = f.NewModifierList([]*ast.Node{f.NewModifier(ast.KindExportKeyword)})
	}
	shapeDeclaration := f.NewInterfaceDeclaration(interfaceModifiers, f.NewIdentifier(name), nil /*typeParameters*/, nil /*heritageClauses*/, f.NewNodeList(shape))

	keyType := "ServiceKey"
	if hasStaticDefault(class) {
		keyType = "ReferenceKey"
	}
	base := name + "_base"
	baseType := f.NewImportTypeNode(false /*isTypeOf*/, f.NewLiteralTypeNode(f.NewStringLiteral("vibelang-runtime", ast.TokenFlagsNone)), nil /*attributes*/, f.NewIdentifier(keyType), f.NewNodeList([]*ast.Node{f.NewTypeReferenceNode(f.NewIdentifier(name), nil /*typeArguments*/)}))
	baseDeclaration := f.NewVariableStatement(
		f.NewModifierList([]*ast.Node{f.NewModifier(ast.KindDeclareKeyword)}),
		f.NewVariableDeclarationList(f.NewNodeList([]*ast.Node{f.NewVariableDeclaration(f.NewIdentifier(base), nil /*exclamationToken*/, baseType, nil /*initializer*/)}), ast.NodeFlagsConst),
	)

	classModifiers := []*ast.Node{}
	if exported {
		classModifiers = append(classModifiers, f.NewModifier(ast.KindExportKeyword))
	}
	classModifiers = append(classModifiers, f.NewModifier(ast.KindDeclareKeyword), f.NewModifier(ast.KindAbstractKeyword))
	extends := f.NewHeritageClause(ast.KindExtendsKeyword, f.NewNodeList([]*ast.Node{f.NewExpressionWithTypeArguments(f.NewIdentifier(base), nil /*typeArguments*/)}))
	keyDeclaration := f.UpdateClassDeclaration(declared.AsClassDeclaration(), f.NewModifierList(classModifiers), declared.Name(), nil /*typeParameters*/, f.NewNodeList([]*ast.Node{extends}), f.NewNodeList(statics))

	result := []*ast.Node{shapeDeclaration, baseDeclaration, keyDeclaration}
	if isDefault {
		// The interface and the class merge only with the same export status,
		// so both stay local and the module exports the merged name.
		result = append(result, f.NewExportAssignment(nil /*modifiers*/, false /*isExportEquals*/, nil /*type*/, f.NewIdentifier(name)))
	}
	return result
}
