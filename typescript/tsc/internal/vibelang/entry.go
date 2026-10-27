package vibelang

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// Entry modules (D135).
//
//	export function main() {
//	  Console.log("Hello")
//	}
//
// A module that exports a function declaration named main with no parameters
// runs it when the host starts the module: the emitted JavaScript ends with
//
//	runtime.$entry(import.meta.main, import.meta.url, main, () => NodeRuntime.runMain(main()));
//
// with NodeRuntime imported from @effect/platform-node when that package
// resolves from the module, and runtime.$runMain, the runtime's copy of the
// same runner, otherwise. $entry calls the arrow when the host started the
// module. import.meta.main is true only in that module, so importing the
// module, from a test or another module, runs nothing. In an ES module bundle
// made by esbuild, Rollup, Rolldown, or Vite, every module shares the bundle's
// import.meta, so $entry runs only the main the bundle exports. A file
// emitted as CommonJS has no import.meta and passes void 0 for both, as every
// module of a CommonJS bundle does, which empties it; $entry then asks the
// host's main module. A module that calls a runner at its top level starts
// its program itself and gets no entry statement.
//
// The file that exports main names its platform once, with a platform import
// (D312):
//
//	import "vibelang/platform/node"
//
// When main needs services that the named platform's bundle provides, such as
// FileSystem and Path, the entry provides the bundle around it:
//
//	runtime.$entry(import.meta.main, import.meta.url, main, () => NodeRuntime.runMain(Effect.provide(main(), NodeServices.layer)));
//
// node names @effect/platform-node's NodeServices, bun @effect/platform-bun's
// BunServices, and browser @effect/platform-browser's BrowserServices. When
// main needs a service that only the platform's experimental bundle provides
// (D313), the entry provides that bundle too, inside Effect's, so its layer
// may need Effect's services. A bundle provides the services its layer's
// declared type names. A provide inside main wins for the code inside it, as
// with Effect.provide. The import loads nothing: the lowering removes it. A
// main that needs none of the bundles' services, and a file without a
// platform import, get the statement above without the provide, and EK3205
// names the services main needs that nothing provides (wire.go).
//
// namedPlatform is the trigger; entryBundles and entryStatement provide the
// layer for whatever platform the trigger names.

// platformNodeModule is the package whose NodeRuntime runs an entry module.
const platformNodeModule = "@effect/platform-node"

// platformImportPrefix starts the specifier of a platform import.
const platformImportPrefix = "vibelang/platform/"

// A platformBundle is the export of a module whose layer member provides a
// platform's services, as @effect/platform-node's NodeServices.layer does.
type platformBundle struct {
	module    string // the module specifier
	namespace string // the export whose layer the entry provides
}

// An entryPlatform is a platform that a platform import names, with its
// bundles: Effect's, and VibeLang's experimental one, which provides the
// services Effect lacks (D313).
type entryPlatform struct {
	name                 string // the name after platformImportPrefix
	effect, experimental platformBundle
}

// entryPlatforms are the platforms a platform import can name.
var entryPlatforms = []entryPlatform{
	{"node", platformBundle{platformNodeModule, "NodeServices"}, platformBundle{"vibelang-runtime/experimental/platform-node", "NodeServices"}},
	{"bun", platformBundle{"@effect/platform-bun", "BunServices"}, platformBundle{"vibelang-runtime/experimental/platform-bun", "BunServices"}},
	{"browser", platformBundle{"@effect/platform-browser", "BrowserServices"}, platformBundle{"vibelang-runtime/experimental/platform-browser", "BrowserServices"}},
}

// namedPlatform returns the platform that file names with a platform import,
// or nil. It reads only syntax, so the implicit imports can use it.
func namedPlatform(file *ast.SourceFile) *entryPlatform {
	for _, statement := range file.Statements.Nodes {
		if platform := platformImport(statement); platform != nil {
			return platform
		}
	}
	return nil
}

// platformImport returns the platform that statement names when it is a
// platform import, `import "vibelang/platform/node"`, or nil.
func platformImport(statement *ast.Node) *entryPlatform {
	if !ast.IsImportDeclaration(statement) || statement.AsImportDeclaration().ImportClause != nil {
		return nil
	}
	specifier := statement.AsImportDeclaration().ModuleSpecifier
	if !ast.IsStringLiteral(specifier) {
		return nil
	}
	name, ok := strings.CutPrefix(specifier.Text(), platformImportPrefix)
	if !ok {
		return nil
	}
	for i := range entryPlatforms {
		if entryPlatforms[i].name == name {
			return &entryPlatforms[i]
		}
	}
	return nil
}

// entryModules are the modules that file, an entry module, resolves as
// implicit imports (implicitRuntimeImport), so the plan can ask which are
// installed: NodeRuntime's package, and the bundles of the platform it names.
func entryModules(file *ast.SourceFile) []string {
	modules := []string{platformNodeModule}
	if platform := namedPlatform(file); platform != nil {
		modules = append(modules, platform.effect.module, platform.experimental.module)
	}
	return modules
}

// runnerNames are the functions that start a program from a module's top level.
var runnerNames = map[string]bool{"runMain": true, "runPromise": true, "runFork": true, "runSync": true, "runCallback": true}

// entryMain returns the declaration of file's exported main function with no
// parameters, or nil when file has none or calls a runner at its top level.
func entryMain(file *ast.SourceFile) *ast.Node {
	if !isVibeLangFile(file) {
		return nil
	}
	var main *ast.Node
	for _, statement := range file.Statements.Nodes {
		switch {
		case statement.Kind == ast.KindFunctionDeclaration && statement.Name() != nil && statement.Name().Text() == "main" &&
			ast.HasSyntacticModifier(statement, ast.ModifierFlagsExport) && !ast.HasSyntacticModifier(statement, ast.ModifierFlagsDefault) &&
			len(statement.Parameters()) == 0 && statement.Body() != nil:
			main = statement
		case containsTopLevelRunner(statement):
			return nil
		}
	}
	return main
}

// containsTopLevelRunner finds runner calls in expressions and initializers
// that execute during module evaluation, without descending into delayed
// function bodies or instance field initializers.
func containsTopLevelRunner(node *ast.Node) bool {
	if ast.IsFunctionLike(node) {
		return false
	}
	if node.Kind == ast.KindPropertyDeclaration && !ast.HasSyntacticModifier(node, ast.ModifierFlagsStatic) {
		return false
	}
	return callsRunner(node) || node.ForEachChild(containsTopLevelRunner)
}

// callsRunner reports whether expression calls a runner, such as
// NodeRuntime.runMain(…) or runPromise(…).
func callsRunner(expression *ast.Node) bool {
	expression = ast.SkipParentheses(expression)
	if !ast.IsCallExpression(expression) {
		return false
	}
	switch callee := expression.Expression(); {
	case ast.IsIdentifier(callee):
		return runnerNames[callee.Text()]
	case ast.IsPropertyAccessExpression(callee):
		return runnerNames[callee.Name().Text()]
	}
	return false
}

// An entryPlan records how the lowering runs an entry module's main.
type entryPlan struct {
	main        *ast.Node        // the main function's declaration
	nodeRuntime bool             // @effect/platform-node resolves from the module
	bundles     []platformBundle // the bundles the entry provides to main, outermost first
}

// planEntry returns the entry plan of file, or nil when file is not an entry
// module. The program resolves @effect/platform-node from an entry module as
// an implicit import (implicitRuntimeImport), so the plan can ask whether it
// resolved.
func planEntry(c *checker.Checker, file *ast.SourceFile) *entryPlan {
	main := entryMain(file)
	if main == nil {
		return nil
	}
	entry := &entryPlan{main: main, nodeRuntime: resolvedModuleFile(c, file, platformNodeModule) != nil}
	entry.bundles, _ = entryBundles(c, file, main)
	return entry
}

// entryBundles returns the bundles whose layers the entry provides to main,
// the main of file, outermost first, and the services they provide; or none
// when file names no platform or main needs none of its bundles' services.
// They are the platform's Effect bundle, and its experimental bundle when
// main needs a service that only that one provides.
func entryBundles(c *checker.Checker, file *ast.SourceFile, main *ast.Node) ([]platformBundle, *checker.Type) {
	platform := namedPlatform(file)
	if platform == nil {
		return nil, nil
	}
	needed := neededServices(c, main)
	effect := bundleServices(c, file, platform.effect)
	experimental := bundleServices(c, file, platform.experimental)
	var usesEffect, usesExperimental bool
	for _, service := range needed {
		switch {
		case provides(c, effect, service):
			usesEffect = true
		case provides(c, experimental, service):
			usesExperimental = true
		}
	}
	var bundles []platformBundle
	var services []*checker.Type
	if effect != nil && (usesEffect || usesExperimental) {
		bundles, services = append(bundles, platform.effect), append(services, effect)
	}
	if usesExperimental {
		bundles, services = append(bundles, platform.experimental), append(services, experimental)
	}
	if len(bundles) == 0 {
		return nil, nil
	}
	return bundles, c.GetUnionType(services)
}

// neededServices returns the services that main, an entry module's main,
// needs: the members of its program's R, or none when R is any.
func neededServices(c *checker.Checker, main *ast.Node) []*checker.Type {
	needed := runnerRequirements(c, c.GetTypeAtLocation(main.Name()))
	if needed == nil || needed.Flags()&(checker.TypeFlagsAny|checker.TypeFlagsUnknown) != 0 {
		return nil
	}
	return unionMembers(needed)
}

// provides reports whether service is one of services, which may be nil.
func provides(c *checker.Checker, services *checker.Type, service *checker.Type) bool {
	return services != nil && c.IsTypeAssignableTo(service, services)
}

// platformNote returns what EK3205 adds at file's main, which needs the
// services needed: the platform import that would provide some of them when
// file names no platform and @effect/platform-node provides them, or that
// the named platform's package is not installed.
func platformNote(c *checker.Checker, file *ast.SourceFile, needed []*checker.Type) string {
	platform := namedPlatform(file)
	if platform != nil {
		if resolvedModuleFile(c, file, platform.effect.module) == nil {
			return "import \"" + platformImportPrefix + platform.name + "\" names " + platform.effect.module + ", which is not installed."
		}
		return ""
	}
	node := &entryPlatforms[0]
	services := bundleServices(c, file, node.effect)
	var names []string
	for _, service := range needed {
		if provides(c, services, service) {
			names = append(names, c.TypeToString(service))
		}
	}
	if len(names) == 0 {
		return ""
	}
	return "Name the platform at the top of this file, and the entry provides " + strings.Join(names, ", ") + ": import \"" + platformImportPrefix + node.name + "\""
}

// resolvedModuleFile returns the file of the module named name that file
// resolves, or nil.
func resolvedModuleFile(c *checker.Checker, file *ast.SourceFile, name string) *ast.SourceFile {
	program := c.Program()
	for key, resolved := range program.GetResolvedModules()[file.PathKey()] {
		if key.Name == name && resolved.IsResolved() {
			return program.GetSourceFileForResolvedModule(resolved)
		}
	}
	return nil
}

// bundleServices returns the services that bundle's layer provides, the Out
// of its declared Layer type, or nil when bundle does not resolve from file
// or declares no such layer.
func bundleServices(c *checker.Checker, file *ast.SourceFile, bundle platformBundle) *checker.Type {
	moduleFile := resolvedModuleFile(c, file, bundle.module)
	if moduleFile == nil || moduleFile.Symbol == nil {
		return nil
	}
	namespace := c.TryGetMemberInModuleExports(bundle.namespace, moduleFile.Symbol)
	if namespace == nil {
		return nil
	}
	if namespace.Flags()&ast.SymbolFlagsAlias != 0 {
		namespace = c.GetAliasedSymbol(namespace)
	}
	layer := c.GetTypeOfPropertyOfType(c.GetTypeOfSymbol(namespace), "layer")
	if layer == nil {
		return nil
	}
	outputs, _, opaque, ok := layerParts(c, layer)
	if !ok || opaque || len(outputs) == 0 {
		return nil
	}
	return c.GetUnionType(outputs)
}

// entryStatement makes `runtime.$entry(import.meta.main, import.meta.url,
// main, () => NodeRuntime.runMain(main()))`, or the same with runtime.$runMain.
// A main that compiles to plain JavaScript runs through Effect.sync, and one
// that needs a platform's services runs as Effect.provide(main(),
// NodeServices.layer) (D312), inside a provide of each further bundle. In a
// file emitted as CommonJS, both import.meta arguments are void 0.
func (tx *lowering) entryStatement() *ast.Node {
	f := tx.Factory()
	entry := tx.plan.entry
	program := f.NewCallExpression(f.NewIdentifier("main"), nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList(nil), ast.NodeFlagsNone)
	if !tx.plan.isEffect(entry.main) {
		sync := f.NewPropertyAccessExpression(tx.effectName(), nil /*questionDotToken*/, f.NewIdentifier("sync"), ast.NodeFlagsNone)
		program = f.NewCallExpression(sync, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{f.NewIdentifier("main")}), ast.NodeFlagsNone)
	}
	layers := make([]*ast.Node, len(entry.bundles))
	for i, bundle := range entry.bundles {
		layers[i] = f.NewPropertyAccessExpression(tx.classReference(classRef{module: bundle.module, name: bundle.namespace}), nil /*questionDotToken*/, f.NewIdentifier("layer"), ast.NodeFlagsNone)
	}
	for i := len(layers) - 1; i >= 0; i-- {
		provide := f.NewPropertyAccessExpression(tx.effectName(), nil /*questionDotToken*/, f.NewIdentifier("provide"), ast.NodeFlagsNone)
		program = f.NewCallExpression(provide, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{program, layers[i]}), ast.NodeFlagsNone)
	}
	var runner *ast.Node
	if entry.nodeRuntime {
		nodeRuntime := tx.classReference(classRef{module: platformNodeModule, name: "NodeRuntime"})
		runner = f.NewPropertyAccessExpression(nodeRuntime, nil /*questionDotToken*/, f.NewIdentifier("runMain"), ast.NodeFlagsNone)
	} else {
		runner = tx.runtimeMember("$runMain")
	}
	run := tx.generated(f.NewArrowFunction(nil /*modifiers*/, nil /*typeParameters*/, f.NewNodeList(nil), nil /*returnType*/, nil /*fullSignature*/, f.NewToken(ast.KindEqualsGreaterThanToken),
		tx.generated(f.NewCallExpression(runner, nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList([]*ast.Node{program}), ast.NodeFlagsNone))))
	started, url := f.NewVoidZeroExpression(), f.NewVoidZeroExpression()
	if !tx.commonJS {
		meta := func(name string) *ast.Node {
			return f.NewPropertyAccessExpression(f.NewMetaProperty(ast.KindImportKeyword, f.NewIdentifier("meta")), nil /*questionDotToken*/, f.NewIdentifier(name), ast.NodeFlagsNone)
		}
		started, url = meta("main"), meta("url")
	}
	arguments := []*ast.Node{started, url, f.NewIdentifier("main"), run}
	return f.NewExpressionStatement(f.NewCallExpression(tx.runtimeMember("$entry"), nil /*questionDotToken*/, nil /*typeArguments*/, f.NewNodeList(arguments), ast.NodeFlagsNone))
}
