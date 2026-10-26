package vibelang

import (
	"slices"
	"strconv"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/printer"
)

// A callback parameter whose type has no contract accepts any contract
// (design D12). It means "inferred from each call's argument", never "cannot
// fail, needs nothing" (Chapter 16):
//
//	function mapAll<A, B>(items: ReadonlyArray<A>, f: (a: A) => B) { … }
//	// <A, B, E = never, R = never>(items: ReadonlyArray<A>, f: (a: A) => Effect<B, E, R>) => Effect<B[], E, R>
//
// Each such parameter's return type B becomes Effect<B, E, R>, where E and R
// are type parameters the function gains. A call infers them from its
// argument, and a call to the callback runs it and passes them on. E and R
// default to never: a call that omits an optional callback, `f?.(x)` in the
// body, adds no failure and needs nothing. A callback whose return type is an
// Effect already has its contract.

// callbackContracts lists the callback parameters of one function that have
// no contract, and the E and R type parameters each of them gains.
type callbackContracts struct {
	parameters     []*ast.Node
	typeParameters []*checker.Type // E and R for each parameter, in order
}

// callbackContractsOf returns fn's callback parameters without a contract,
// or nil when it has none.
func (a *analyzer) callbackContractsOf(fn *ast.Node) *callbackContracts {
	if s, ok := a.callbacks[fn]; ok {
		return s
	}
	a.callbacks[fn] = nil // while fn's parameters are read
	if !acceptsAnyContract(fn) || a.hasPlainContract(fn) {
		return nil
	}
	var parameters []*ast.Node
	for _, parameter := range fn.Parameters() {
		if a.isUncontractedCallback(parameter) {
			parameters = append(parameters, parameter)
		}
	}
	effect := a.programEffectInterface()
	if len(parameters) == 0 || effect == nil {
		return nil
	}
	s := &callbackContracts{parameters: parameters}
	taken := typeParameterNamesAround(fn)
	for i := range parameters {
		suffix := ""
		if i > 0 {
			suffix = strconv.Itoa(i + 1)
		}
		never := a.checker.GetNeverType()
		s.typeParameters = append(s.typeParameters,
			a.checker.NewTypeParameterWithDefault(freeName("E"+suffix, taken), never),
			a.checker.NewTypeParameterWithDefault(freeName("R"+suffix, taken), never))
	}
	a.callbacks[fn] = s
	return s
}

// uncontractedCallbackOf returns the callback parameter of fn that gained t,
// one of the E and R type parameters D12 adds, or nil.
func (a *analyzer) uncontractedCallbackOf(fn *ast.Node, t *checker.Type) *ast.Node {
	s := a.callbackContractsOf(fn)
	if s == nil {
		return nil
	}
	for i, parameter := range s.parameters {
		if s.typeParameters[2*i] == t || s.typeParameters[2*i+1] == t {
			return parameter
		}
	}
	return nil
}

// callbackContractText writes parameter, an uncontracted callback, with the
// contract that says it cannot fail and needs nothing: make: () => Effect<Boom>,
// or make?: () => Effect<Boom> for an optional one.
func callbackContractText(file *ast.SourceFile, parameter *ast.Node) string {
	name := declarationName(parameter)
	if parameter.QuestionToken() != nil {
		name += "?"
	}
	callback := parameter.Type()
	if callback == nil || callback.Kind != ast.KindFunctionType || callback.Type() == nil {
		return name
	}
	written := nodeText(file, callback)
	returned := nodeText(file, callback.Type())
	prefix, ok := strings.CutSuffix(written, returned)
	if !ok {
		return name
	}
	return name + ": " + prefix + effectText(returned)
}

// acceptsAnyContract reports whether fn's callback parameters accept any
// contract: a .vibe function declaration, method, or function expression,
// but not a lambda passed as an argument, whose parameters the call types.
func acceptsAnyContract(fn *ast.Node) bool {
	if !isLowerable(fn) {
		return false
	}
	if isLambda(fn) {
		call, _ := ownerCall(fn)
		return call == nil
	}
	return true
}

// isUncontractedCallback reports whether parameter is annotated with a
// function type whose return type is not an Effect.
func (a *analyzer) isUncontractedCallback(parameter *ast.Node) bool {
	t := parameter.Type()
	if t == nil || t.Kind != ast.KindFunctionType || t.Type() == nil {
		return false
	}
	return !a.isEffectAnnotation(t.Type())
}

// typeParameterNamesAround returns the names of the type parameters that fn
// and the declarations around it declare, so E and R do not shadow them in
// what the compiler prints.
func typeParameterNamesAround(fn *ast.Node) []string {
	var names []string
	for n := fn; n != nil; n = n.Parent {
		if ast.IsFunctionLike(n) || ast.IsClassLike(n) || ast.IsInterfaceDeclaration(n) || ast.IsTypeAliasDeclaration(n) {
			for _, tp := range n.TypeParameters() {
				names = append(names, tp.Name().Text())
			}
		}
	}
	return names
}

// freeName returns name, or name followed by a number, whichever taken does
// not hold.
func freeName(name string, taken []string) string {
	candidate := name
	for i := 2; slices.Contains(taken, candidate); i++ {
		candidate = name + "_" + strconv.Itoa(i)
	}
	return candidate
}

// extraTypeParameters gives a function with uncontracted callback parameters
// its E and R type parameters.
func extraTypeParameters(c *checker.Checker, declaration *ast.Node) []*checker.Type {
	if !isLowerable(declaration) {
		return nil
	}
	if s := analyzerFor(c).callbackContractsOf(declaration); s != nil {
		return s.typeParameters
	}
	return nil
}

// callbackParameterType gives an uncontracted callback parameter the type
// (a: A) => Effect<B, E, R>, given t, the (a: A) => B its annotation declares.
func callbackParameterType(c *checker.Checker, parameter *ast.Node, t *checker.Type) *checker.Type {
	fn := parameter.Parent
	if fn == nil || !isLowerable(fn) {
		return t
	}
	a := analyzerFor(c)
	s := a.callbackContractsOf(fn)
	if s == nil {
		return t
	}
	i := slices.Index(s.parameters, parameter)
	signatures := c.GetSignaturesOfType(t, checker.SignatureKindCall)
	if i < 0 || len(signatures) != 1 {
		return t
	}
	returned := c.GetReturnTypeOfSignature(signatures[0])
	effect := c.NewTypeReference(a.programEffectInterface(), []*checker.Type{returned, s.typeParameters[2*i], s.typeParameters[2*i+1]})
	return c.NewFunctionTypeWithReturnType(t.Symbol(), signatures[0], effect)
}

// declareCallbackContracts returns the transform that writes the contracts
// D12 gives callback parameters into the declaration file of a .vibe file:
//
//	export declare function mapAll<A, B>(items: ReadonlyArray<A>, f: (a: A) => B): Effect<B[], E, R>
//
// becomes
//
//	export declare function mapAll<A, B, E = never, R = never>(items: ReadonlyArray<A>, f: (a: A) => Effect<B, E, R>): Effect<B[], E, R>
//
// It runs before the prelude transform, which imports the Effect it writes.
func declareCallbackContracts(resolver printer.EmitResolver, sourceFile *ast.SourceFile) func(*ast.SourceFile) *ast.SourceFile {
	r, ok := resolver.(*checker.EmitResolver)
	if !ok || !isVibeLangFile(sourceFile) {
		return nil
	}
	return func(declarations *ast.SourceFile) *ast.SourceFile {
		ec := resolver.EmitContext()
		f := ec.Factory
		changed := false
		statements := make([]*ast.Node, 0, len(declarations.Statements.Nodes))
		r.WithChecker(func(c *checker.Checker) {
			a := analyzerFor(c)
			effectName := preludeEffectName(c, sourceFile)
			for _, statement := range declarations.Statements.Nodes {
				source := ec.MostOriginal(statement)
				s := (*callbackContracts)(nil)
				if statement.Kind == ast.KindFunctionDeclaration && source != nil && source.Kind == ast.KindFunctionDeclaration {
					s = a.callbackContractsOf(source)
				}
				if s == nil {
					statements = append(statements, statement)
					continue
				}
				changed = true
				statements = append(statements, a.declareContracts(f, statement, source, s, effectName))
			}
		})
		if !changed {
			return declarations
		}
		// The list keeps the source range, from which the printer emits the
		// file's leading comments, such as its header.
		list := f.NewNodeList(statements)
		list.Loc = declarations.Statements.Loc
		return f.UpdateSourceFile(declarations, list, declarations.EndOfFileToken).AsSourceFile()
	}
}

// declareContracts adds s's type parameters to declaration, a function
// declaration in a declaration file, and makes each of source's uncontracted
// callback parameters return Effect<B, E, R>.
func (a *analyzer) declareContracts(f *printer.NodeFactory, declaration *ast.Node, source *ast.Node, s *callbackContracts, effectName func() *ast.Node) *ast.Node {
	fn := declaration.AsFunctionDeclaration()
	var typeParameters []*ast.Node
	if fn.TypeParameters != nil {
		typeParameters = append(typeParameters, fn.TypeParameters.Nodes...)
	}
	for _, tp := range s.typeParameters {
		typeParameters = append(typeParameters, f.NewTypeParameterDeclaration(nil /*modifiers*/, f.NewIdentifier(tp.Symbol().Name()), nil /*constraint*/, nil /*expression*/, f.NewKeywordTypeNode(ast.KindNeverKeyword)))
	}
	parameters := slices.Clone(fn.Parameters.Nodes)
	for i, parameter := range s.parameters {
		index := slices.Index(source.Parameters(), parameter)
		if index < 0 || index >= len(parameters) {
			continue
		}
		p := parameters[index].AsParameterDeclaration()
		callback := p.Type
		if callback == nil || callback.Kind != ast.KindFunctionType {
			continue
		}
		ft := callback.AsFunctionTypeNode()
		effect := f.NewTypeReferenceNode(effectName(), f.NewNodeList([]*ast.Node{
			ft.Type,
			f.NewTypeReferenceNode(f.NewIdentifier(s.typeParameters[2*i].Symbol().Name()), nil /*typeArguments*/),
			f.NewTypeReferenceNode(f.NewIdentifier(s.typeParameters[2*i+1].Symbol().Name()), nil /*typeArguments*/),
		}))
		callback = f.UpdateFunctionTypeNode(ft, ft.TypeParameters, ft.Parameters, effect)
		parameters[index] = f.UpdateParameterDeclaration(p, p.Modifiers(), p.DotDotDotToken, p.Name(), p.QuestionToken, callback, p.Initializer)
	}
	return f.UpdateFunctionDeclaration(fn, fn.Modifiers(), fn.AsteriskToken, fn.Name(), f.NewNodeList(typeParameters), f.NewNodeList(parameters), fn.Type, fn.FullSignature, fn.Body)
}

// preludeEffectName returns a function that writes the name of the Effect
// type in sourceFile's declaration file: Effect from the prelude, which the
// prelude transform then imports, or Effect.Effect when the source imports
// effect's Effect module under that name.
func preludeEffectName(c *checker.Checker, sourceFile *ast.SourceFile) func() *ast.Node {
	return func() *ast.Node {
		f := ast.NewNodeFactory(ast.NodeFactoryHooks{})
		if bindsName(sourceFile, "Effect") {
			return f.NewQualifiedName(f.NewIdentifier("Effect"), f.NewIdentifier("Effect"))
		}
		state := preludeStateOf(c)
		if state.typeUses[sourceFile] == nil {
			state.typeUses[sourceFile] = map[string]bool{}
		}
		state.typeUses[sourceFile]["Effect"] = true
		return f.NewIdentifier("Effect")
	}
}

// bindsName reports whether file's own imports bind name.
func bindsName(file *ast.SourceFile, name string) bool {
	for _, statement := range file.Statements.Nodes {
		if statement.Kind != ast.KindImportDeclaration {
			continue
		}
		clause := statement.AsImportDeclaration().ImportClause
		if clause == nil {
			continue
		}
		if clause.Name() != nil && clause.Name().Text() == name {
			return true
		}
		bindings := clause.AsImportClause().NamedBindings
		switch {
		case bindings == nil:
		case ast.IsNamespaceImport(bindings):
			if bindings.Name().Text() == name {
				return true
			}
		default:
			for _, element := range bindings.AsNamedImports().Elements.Nodes {
				if element.Name().Text() == name {
					return true
				}
			}
		}
	}
	return false
}
