package vibelang

import (
	"slices"
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
	"github.com/microsoft/TypeScript/tsc/internal/ls"
	"github.com/microsoft/TypeScript/tsc/internal/printer"
	"github.com/microsoft/TypeScript/tsc/internal/scanner"
)

// Method syntax on Effect data types (design D152). A value whose type
// belongs to one of effect's data-type modules takes that module's
// data-first functions as methods:
//
//	readings().filter((v) => v >= 25).map((v) => `${v} C`).take(2)
//	// Stream.take(Stream.map(Stream.filter(readings(), …), …), 2)
//
// x.name(args) resolves to Module.name(x, args) when x's type, after its own
// members, has no property name, and x's type is declared by the module:
// Stream<A, E, R> by effect's Stream module, Option<A> by Option. The method
// is every overload of Module.name whose first parameter is named self,
// effect's convention for the data-first form, with that parameter as the
// call's this, so the call checks x against it and infers type parameters
// from x. A type's own members win, so Effect's pipe stays a method, and
// plain JavaScript arrays and objects keep their own methods.

// methodModules are effect's modules whose main types take method syntax:
// the modules of Effect's data types, whose functions take the value first.
var methodModules = map[string]bool{
	"BigDecimal": true, "Cause": true, "Chunk": true, "DateTime": true, "Deferred": true, "Duration": true,
	"Effect": true, "Exit": true, "Fiber": true, "HashMap": true, "HashSet": true, "Option": true,
	"Queue": true, "Ref": true, "Result": true, "Schedule": true, "Sink": true, "Stream": true,
}

func init() {
	checker.RegisterMissingPropertyHook(moduleMethod)
	fileChecks = append(fileChecks, checkModuleMethods)
	ls.RegisterQuickInfoHook(moduleMethodQuickInfo)
	ls.RegisterMemberCompletionHook(moduleMethodCompletions)
}

// methodState is what one checker remembers about method syntax.
type methodState struct {
	properties map[string]*ast.Symbol // "Stream.filter" -> the method property, or nil
	accesses   map[*ast.Node]string   // property access -> the module it calls into
	prelude    map[*ast.Node]bool     // called accesses where the module's name means the module through the prelude
	members    map[string]*checker.Type
}

type methodStateKey struct{}

func methodStateOf(c *checker.Checker) *methodState {
	return c.ExtensionState(methodStateKey{}, func() any {
		return &methodState{properties: map[string]*ast.Symbol{}, accesses: map[*ast.Node]string{}, prelude: map[*ast.Node]bool{}, members: map[string]*checker.Type{}}
	}).(*methodState)
}

// homeModule returns the method module that declares t, or "". Every member
// of a union must come from the same module, as None and Some do from Option.
func homeModule(t *checker.Type) string {
	if t == nil {
		return ""
	}
	if t.Flags()&checker.TypeFlagsUnion != 0 {
		home := ""
		for _, member := range t.Types() {
			module := homeModule(member)
			if module == "" || home != "" && module != home {
				return ""
			}
			home = module
		}
		return home
	}
	symbol := t.Symbol()
	if symbol == nil {
		return ""
	}
	for _, declaration := range symbol.Declarations() {
		if module := effectModuleOfFile(ast.GetSourceFileOfNode(declaration)); methodModules[module] {
			return module
		}
	}
	return ""
}

// effectModuleOfFile returns the name of effect's module that file declares,
// such as Stream for effect/dist/Stream.d.ts, or "".
func effectModuleOfFile(file *ast.SourceFile) string {
	if file == nil {
		return ""
	}
	name := string(file.FileName())
	const marker = "/effect/dist/"
	i := strings.LastIndex(name, marker)
	if i < 0 || !strings.HasSuffix(name, ".d.ts") {
		return ""
	}
	module := strings.TrimSuffix(name[i+len(marker):], ".d.ts")
	if strings.Contains(module, "/") {
		return ""
	}
	return module
}

// moduleMethod supplies name as a method of receiver when receiver belongs to
// a method module that exports a data-first function of that name. An
// optional chain gets none: its lowering would have to stop the whole chain
// at a nullish receiver, not only the call.
func moduleMethod(c *checker.Checker, node *ast.Node, receiver *checker.Type, name string) *ast.Symbol {
	if !isVibeLangFile(ast.GetSourceFileOfNode(node)) || ast.IsOptionalChain(node) {
		return nil
	}
	home := homeModule(receiver)
	if home == "" {
		return nil
	}
	property := methodProperty(c, ast.GetSourceFileOfNode(node), home, name)
	if property != nil {
		methodStateOf(c).accesses[node] = home
	}
	return property
}

// methodProperty returns the method property for module's function name, or
// nil when module exports no data-first function of that name.
func methodProperty(c *checker.Checker, file *ast.SourceFile, module string, name string) *ast.Symbol {
	state := methodStateOf(c)
	key := module + "." + name
	if property, ok := state.properties[key]; ok {
		return property
	}
	var property *ast.Symbol
	if namespace := effectModuleNamespace(c, file, module); namespace != nil {
		if function := c.TryGetMemberInModuleExports(name, namespace); function != nil {
			var methods []*checker.Signature
			for _, signature := range c.GetSignaturesOfType(c.GetTypeOfSymbol(function), checker.SignatureKindCall) {
				if takesSelfFirst(signature) {
					if method := c.SignatureWithThisFromFirstParameter(signature); method != nil {
						methods = append(methods, method)
					}
				}
			}
			if len(methods) > 0 {
				property = c.NewPropertySymbol(name, c.NewFunctionType(nil, methods), function.Declarations())
			}
		}
	}
	state.properties[key] = property
	return property
}

// takesSelfFirst reports whether signature takes the value first: its first
// parameter is named self, effect's name for the data-first form, or, as in
// Stream.flatten's <Arg extends Stream<…> | Options>(streamsOrOptions?: Arg),
// ends in OrOptions, effect's name for a dual whose one signature decides by
// its first argument's type.
func takesSelfFirst(signature *checker.Signature) bool {
	declaration := signature.Declaration()
	if declaration == nil {
		return false
	}
	parameters := declaration.Parameters()
	if len(parameters) == 0 || !ast.IsIdentifier(parameters[0].Name()) {
		return false
	}
	name := parameters[0].Name().Text()
	return name == "self" || strings.HasSuffix(name, "OrOptions")
}

// effectModuleNamespace returns the symbol of effect's module named module,
// reached through effect's root from file, or through the root that another
// file of the program imports.
func effectModuleNamespace(c *checker.Checker, file *ast.SourceFile, module string) *ast.Symbol {
	root := orProgramModule(c, effectRootModuleSymbol(c, preludeStateOf(c), file), effectRootModule)
	if root == nil {
		return nil
	}
	namespace := c.TryGetMemberInModuleExports(module, root)
	if namespace != nil && namespace.Flags()&ast.SymbolFlagsAlias != 0 {
		namespace = c.GetAliasedSymbol(namespace)
	}
	return namespace
}

// methodCall is a call through method syntax.
type methodCall struct {
	module  string // the module the call reaches, such as Stream
	prelude bool   // the module's name at the call means the module through the prelude
}

// methodCallOf returns the method call whose callee is access, a property
// access the checker resolved through method syntax, or false.
func methodCallOf(c *checker.Checker, access *ast.Node) (methodCall, bool) {
	state := methodStateOf(c)
	module := state.accesses[access]
	return methodCall{module: module, prelude: state.prelude[access]}, module != ""
}

// methodModuleOf returns the module that access, a property access the
// checker resolved through method syntax, calls into, or "".
func methodModuleOf(c *checker.Checker, access *ast.Node) string {
	return methodStateOf(c).accesses[access]
}

// checkModuleMethods reports EK4501 where method syntax names a module's
// function without calling it: the method exists only as a call. For a call,
// it records whether the module's name there means the module through the
// prelude, so the lowered call uses the prelude's import of it instead of a
// second import.
func checkModuleMethods(c *checker.Checker, file *ast.SourceFile) {
	state := methodStateOf(c)
	var nodes []*ast.Node
	for access, module := range state.accesses {
		if ast.GetSourceFileOfNode(access) != file {
			continue
		}
		if parent := access.Parent; parent != nil && ast.IsCallExpression(parent) && parent.Expression() == access {
			if namesPreludeModule(c, access, module) {
				state.prelude[access] = true
				recordPreludeUse(preludeStateOf(c), file, module)
			}
			continue
		}
		nodes = append(nodes, access)
	}
	slices.SortFunc(nodes, func(a, b *ast.Node) int { return a.Pos() - b.Pos() })
	for _, access := range nodes {
		module := state.accesses[access]
		name := access.Name().Text()
		d := report(c, access.Name(), EK4501, name, module)
		note(d, "Call it, or pass {0}.{1} itself.", module, name)
	}
}

// namesPreludeModule reports whether module's name at access resolves to the
// prelude's binding of effect's module, which no scope around access shadows.
func namesPreludeModule(c *checker.Checker, access *ast.Node, module string) bool {
	symbol := c.ResolveName(module, access, ast.SymbolFlagsValue, false /*excludeGlobals*/)
	return symbol != nil && symbol == preludeSymbol(c, preludeStateOf(c), ast.GetSourceFileOfNode(access), module)
}

// moduleMethodQuickInfo tells a hover on a method's name which function the
// call reaches.
func moduleMethodQuickInfo(c *checker.Checker, node *ast.Node) (string, string) {
	parent := node.Parent
	if parent == nil || !ast.IsPropertyAccessExpression(parent) || parent.Name() != node {
		return "", ""
	}
	module := methodModuleOf(c, parent)
	if module == "" {
		return "", ""
	}
	file := ast.GetSourceFileOfNode(parent)
	receiver := scanner.GetSourceTextOfNodeFromSourceFile(file, parent.Expression(), false /*includeTrivia*/)
	return "", "Method syntax: calls `" + module + "." + node.Text() + "(" + receiver + ", …)`."
}

// moduleMethodCompletions adds the data-first functions of t's module to the
// members listed after the dot.
func moduleMethodCompletions(c *checker.Checker, left *ast.Node, t *checker.Type) *checker.Type {
	file := ast.GetSourceFileOfNode(left)
	if !isVibeLangFile(file) {
		return t
	}
	home := homeModule(c.GetApparentType(t))
	if home == "" {
		return t
	}
	state := methodStateOf(c)
	members, ok := state.members[home]
	if !ok {
		table := ast.SymbolTable{}
		if namespace := effectModuleNamespace(c, file, home); namespace != nil {
			for _, export := range c.GetExportsOfModule(namespace) {
				if property := methodProperty(c, file, home, export.Name()); property != nil {
					table[export.Name()] = property
				}
			}
		}
		members = c.NewFunctionType(table, nil)
		state.members[home] = members
	}
	return c.GetIntersectionType([]*checker.Type{t, members})
}

// moduleNamespace returns the local name of the namespace import of module,
// such as effect/Stream, which the lowering adds on first use under a free
// name: Stream, or Stream_1 when the file binds Stream.
func (tx *lowering) moduleNamespace(module string) *ast.IdentifierNode {
	if tx.modules == nil {
		tx.modules = map[string]*ast.IdentifierNode{}
	}
	if name, ok := tx.modules[module]; ok {
		return name
	}
	base := module[strings.LastIndex(module, "/")+1:]
	name := tx.Factory().NewUniqueNameEx(base, printer.AutoGenerateOptions{Flags: printer.GeneratedIdentifierFlagsOptimistic | printer.GeneratedIdentifierFlagsFileLevel})
	tx.modules[module] = name
	tx.moduleOrder = append(tx.moduleOrder, module)
	return name
}

// visitModuleMethodCall lowers x.name(args), a method call into module, to
// Module.name(x, args), run with yield* where its call runs. Module is the
// prelude's import of the module where its name means that, and otherwise a
// namespace import the lowering adds.
func (tx *lowering) visitModuleMethodCall(node *ast.Node) *ast.Node {
	call := tx.plan.methods[node]
	f := tx.Factory()
	access := node.Expression()
	receiver := tx.Visitor().VisitNode(access.Expression())
	arguments := []*ast.Node{receiver}
	for _, argument := range node.Arguments() {
		arguments = append(arguments, tx.Visitor().VisitNode(argument))
	}
	namespace := f.NewIdentifier(call.module)
	if !call.prelude {
		namespace = tx.moduleNamespace("effect/" + call.module)
	}
	callee := f.NewPropertyAccessExpression(namespace, nil /*questionDotToken*/, f.NewIdentifier(access.Name().Text()), ast.NodeFlagsNone)
	lowered := f.NewCallExpression(callee, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList(arguments), ast.NodeFlagsNone)
	tx.EmitContext().SetOriginal(lowered, node)
	lowered.Loc = node.Loc
	if tx.inEffect && tx.plan.runsCall(node) {
		run := f.NewYieldExpression(f.NewToken(ast.KindAsteriskToken), lowered)
		tx.EmitContext().SetOriginal(run, node)
		return run
	}
	return lowered
}
