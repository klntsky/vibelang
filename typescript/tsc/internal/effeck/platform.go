package effeck

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// Effeck's standard library is Effect's (design D6, Chapter 11). An .effeck
// file may not run a Node.js or browser API, and the error names the Effect
// module that replaces it (EK4402):
//
//	error EK4402: readFile from node:fs/promises is a platform API, which Effeck code does not call directly.
//	  Use FileSystem from "effect/FileSystem":   FileSystem.readFileString(path)
//	  Provide it with NodeServices.layer from @effect/platform-node, or the Bun or browser equivalent.
//
// D108 puts the error where the code runs the API: a call, a new, or a
// tagged template of it, or a read of one of its properties, which can run a
// getter, as process.env does. The API is a binding imported from a Node.js
// or Bun built-in module, or a platform global such as fetch, setTimeout,
// process, or console. Importing one, or passing one as a value, runs
// nothing, so Effect's own setup works:
//
//	import { createServer } from "node:http"
//	const Server = NodeHttpServer.layer(createServer, { port: 3000 })   // allowed
//
// A clock or randomness read, such as Date.now(), is EK4403 instead, an error
// by default that a project may lower (D34); a read it allows compiles as a
// trusted sync call. Both apply everywhere in an .effeck file, a boundary's
// callback and the module's top level included.

// replacement is the Effect API that replaces a platform API.
type replacement struct {
	use     string // what to use, such as "FileSystem"
	from    string // where it comes from, such as "effect/FileSystem"
	example string // a call that shows it, or ""
	layer   bool   // a platform layer such as NodeServices.layer provides it
}

// replacements are the Effect APIs that replace platform APIs (Chapter 11),
// by the name the tables below use.
var replacements = map[string]*replacement{
	"FileSystem":     {"FileSystem", "effect/FileSystem", "FileSystem.readFileString(path)", true},
	"Path":           {"Path", "effect/Path", "Path.join(directory, name)", true},
	"HttpClient":     {"HttpClient", "effect/http", "HttpClient.get(url)", true},
	"ChildProcess":   {"ChildProcessSpawner", "effect/process", "ChildProcessSpawner.string(ChildProcess.make(command, args))", true},
	"Socket":         {"Socket and SocketServer", "effect/socket", "", true},
	"DatagramSocket": {"DatagramSocket", "effect/socket", "", true},
	"Dns":            {"Dns", "effect/net", "", true},
	"Worker":         {"Worker and WorkerRunner", "effect/workers", "", true},
	"Terminal":       {"Terminal and Stdio", "effect", "", true},
	"PubSub":         {"PubSub, or Stream.fromEventListener", "effect", "", false},
	"Stream":         {"Stream", "effect", "", false},
	"Schedule":       {"Effect.sleep and Schedule", "effect", `Effect.sleep("1 second")`, false},
	"Crypto":         {"Crypto", "effect/Crypto", "Crypto.randomBytes(16)", true},
	"Config":         {"Config", "effect", `Config.String("NAME")()`, false},
	"Console":        {"Console or Effect.log", "effect", "Console.log(message)", false},
	"Clock":          {"Clock", "effect", "Clock.currentTimeMillis()", false},
	"KeyValueStore":  {"KeyValueStore", "effect/persistence", "", true},
	"SqlClient":      {"SqlClient, with a driver package such as @effect/sql-sqlite-node", "effect/sql", "", true},
	"UrlParams":      {"UrlParams", "effect/http", "", false},
	"Command":        {"Command, Flag, and Argument", "effect/cli", "", true},
	"@effect/vitest": {"it.effect and assert", "@effect/vitest", "", false},
	"import":         {"an import declaration", "", `import { name } from "./module.ts"`, false},
}

// platformModules maps Node.js built-in modules to the replacements' names;
// "" means Effect has no module for it. Node.js's other modules are platform
// APIs too when imported with the node: prefix.
var platformModules = map[string]string{
	"assert": "@effect/vitest", "assert/strict": "@effect/vitest", "async_hooks": "", "buffer": "",
	"child_process": "ChildProcess", "cluster": "Worker", "console": "Console", "constants": "",
	"crypto": "Crypto", "dgram": "DatagramSocket", "diagnostics_channel": "", "dns": "Dns",
	"dns/promises": "Dns", "domain": "", "events": "PubSub", "fs": "FileSystem",
	"fs/promises": "FileSystem", "http": "HttpClient", "http2": "HttpClient", "https": "HttpClient",
	"inspector": "", "inspector/promises": "", "module": "", "net": "Socket", "os": "",
	"path": "Path", "path/posix": "Path", "path/win32": "Path", "perf_hooks": "Clock",
	"process": "Config", "punycode": "", "querystring": "UrlParams", "readline": "Terminal",
	"readline/promises": "Terminal", "repl": "", "stream": "Stream", "stream/consumers": "Stream",
	"stream/promises": "Stream", "stream/web": "Stream", "string_decoder": "", "sys": "",
	"timers": "Schedule", "timers/promises": "Schedule", "tls": "Socket", "trace_events": "", "tty": "Terminal",
	"url": "Path", "util": "", "util/types": "", "v8": "", "vm": "", "wasi": "",
	"worker_threads": "Worker", "zlib": "",
}

// prefixOnlyModules are Node.js built-in modules that exist only with the
// node: prefix.
var prefixOnlyModules = map[string]string{"sea": "", "sqlite": "SqlClient", "test": "@effect/vitest", "test/reporters": "@effect/vitest"}

// platformGlobals maps the platform globals that .effeck code may not use to the
// replacements' names. Platform-neutral web standards that touch nothing
// outside the program, such as URL, TextEncoder, AbortController, and
// structuredClone, are not in the table.
var platformGlobals = map[string]string{
	"fetch": "HttpClient", "XMLHttpRequest": "HttpClient", "EventSource": "HttpClient", "WebSocket": "Socket",
	"setTimeout": "Schedule", "setInterval": "Schedule", "setImmediate": "Schedule", "clearTimeout": "Schedule",
	"clearInterval": "Schedule", "clearImmediate": "Schedule", "queueMicrotask": "Schedule",
	"requestAnimationFrame": "Schedule", "cancelAnimationFrame": "Schedule", "requestIdleCallback": "Schedule",
	"cancelIdleCallback": "Schedule",
	"process":            "Config", "console": "Console", "crypto": "Crypto", "performance": "Clock",
	"localStorage": "KeyValueStore", "sessionStorage": "KeyValueStore", "indexedDB": "KeyValueStore",
	"Worker": "Worker", "SharedWorker": "Worker", "importScripts": "Worker",
	"document": "", "window": "", "self": "", "navigator": "", "location": "", "history": "",
	"alert": "", "confirm": "", "prompt": "",
	"addEventListener": "PubSub", "removeEventListener": "PubSub", "dispatchEvent": "PubSub",
	"require": "import", "module": "", "exports": "", "__dirname": "Path", "__filename": "Path",
	"Buffer": "", "Bun": "", "Deno": "",
}

// processMembers gives the replacement for one member of process.
var processMembers = map[string]string{
	"env": "Config", "argv": "Command", "stdin": "Terminal", "stdout": "Terminal", "stderr": "Terminal",
	"hrtime": "Clock", "uptime": "Clock",
}

// clockReads lists the clock and randomness reads (EK4403), by the
// declaration that a call resolves to, with what replaces each one.
var clockReads = map[string]string{
	"DateConstructor.now":         "Clock.currentTimeMillis()",
	"DateConstructor":             "DateTime.now()",
	"DateConstructor.constructor": "DateTime.now()", // new Date()
	"Math.random":                 "Random.next()",
	"Crypto.randomUUID":           `Crypto.randomUUIDv4(), from "effect/Crypto"`,
	"Crypto.getRandomValues":      `Crypto.randomBytes(size), from "effect/Crypto"`,
	"Performance.now":             "Clock.monotonicTimeNanos()",
	"Now.instant":                 "DateTime.now()",
	"Now.zonedDateTimeISO":        "DateTime.nowInCurrentZone()",
	"Now.plainDateTimeISO":        "DateTime.now()",
	"Now.plainDateISO":            "DateTime.now()",
	"Now.plainTimeISO":            "DateTime.now()",
	"Now.timeZoneId":              "DateTime.CurrentTimeZone()",
}

func init() {
	fileChecks = append(fileChecks, checkPlatform)
}

// platformModule is EK4402's text for a dynamic import of a platform module.
var platformModule = EK4402.With("{0} is a platform API, which Effeck code does not call directly.")

// checkPlatform reports EK4402 where file's code runs a platform API (D108),
// and EK4403 for clock and randomness reads.
func checkPlatform(c *checker.Checker, file *ast.SourceFile) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		switch {
		case ast.IsImportDeclaration(node) || ast.IsExportDeclaration(node):
			return false // importing or re-exporting a platform API runs nothing
		case ast.IsImportCall(node):
			if arguments := node.Arguments(); len(arguments) > 0 && ast.IsStringLiteral(arguments[0]) {
				if r, ok := platformModuleOf(arguments[0].Text()); ok {
					noteReplacement(report(c, arguments[0], platformModule, arguments[0].Text()), replacements[r])
				}
			}
		case ast.IsCallExpression(node) || ast.IsNewExpression(node):
			checkClockRead(c, file, node)
		case ast.IsIdentifier(node):
			checkPlatformUse(c, file, node)
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// platformModuleOf returns the name of module's replacement, and whether
// module is a platform module: a Node.js built-in, with or without the node:
// prefix, or a Bun built-in.
func platformModuleOf(module string) (string, bool) {
	if strings.HasPrefix(module, "bun:") {
		return "", true
	}
	name, prefixed := strings.CutPrefix(module, "node:")
	if r, ok := platformModules[name]; ok {
		return r, true
	}
	if r, ok := prefixOnlyModules[name]; ok && prefixed {
		return r, true
	}
	return "", prefixed
}

// platformRef is what a reference to a platform API refers to.
type platformRef struct {
	replacement string // the name of what replaces it in replacements, or ""
	module      string // the platform module it is imported from, or "" for a global
	namespace   bool   // a namespace import, whose members are values until code runs them
	unresolved  bool   // a platform global's name that does not resolve
}

// platformReference reports whether id refers to a platform API: a binding
// imported from a platform module, or a global in platformGlobals that a lib
// file or a platform package declares, or that does not resolve at all, as
// process does without @types/node.
func platformReference(c *checker.Checker, id *ast.Node) (platformRef, bool) {
	if !ast.IsIdentifier(id) || !isValueUse(id) {
		return platformRef{}, false
	}
	symbol := c.GetSymbolAtLocation(id)
	if symbol != nil && symbol.Flags()&ast.SymbolFlagsAlias != 0 {
		for _, decl := range symbol.Declarations() {
			if module, namespace := importedFrom(decl); module != "" {
				if r, ok := platformModuleOf(module); ok {
					return platformRef{replacement: r, module: module, namespace: namespace}, true
				}
			}
		}
		return platformRef{}, false
	}
	r, listed := platformGlobals[id.Text()]
	if !listed {
		return platformRef{}, false
	}
	if symbol != nil && !isGlobalOf(symbol, func(file *ast.SourceFile) bool { return isLibFile(c, file) || isPlatformPackage(packageOf(c, file)) }) {
		return platformRef{}, false // a local, or a global the program declares
	}
	return platformRef{replacement: r, unresolved: symbol == nil}, true
}

// importedFrom returns the module that decl, an import binding's
// declaration, imports from, and whether it imports the module's namespace.
func importedFrom(decl *ast.Node) (module string, namespace bool) {
	namespace = decl.Kind == ast.KindNamespaceImport || decl.Kind == ast.KindImportEqualsDeclaration
	for n := decl; n != nil; n = n.Parent {
		switch {
		case ast.IsImportDeclaration(n):
			if specifier := n.ModuleSpecifier(); specifier != nil && ast.IsStringLiteral(specifier) {
				return specifier.Text(), namespace
			}
			return "", false
		case ast.IsImportEqualsDeclaration(n):
			if reference := n.AsImportEqualsDeclaration().ModuleReference; reference != nil && ast.IsExternalModuleReference(reference) {
				if expression := reference.AsExternalModuleReference().Expression; expression != nil && ast.IsStringLiteral(expression) {
					return expression.Text(), namespace
				}
			}
			return "", false
		}
	}
	return "", false
}

// isPlatformCallee reports whether the callee of call is rooted in a
// reference to a platform API, whose run EK4402 reports.
func isPlatformCallee(c *checker.Checker, call *ast.Node) bool {
	root := ast.SkipParentheses(call.Expression())
	for ast.IsPropertyAccessExpression(root) || ast.IsElementAccessExpression(root) {
		root = ast.SkipParentheses(root.Expression())
	}
	_, ok := platformReference(c, root)
	return ok
}

// checkPlatformUse reports id, a reference to a platform API, where the code
// runs it (D108): calls it, constructs it, tags a template with it, or reads
// a property of it, which can run a getter, as process.env does, directly or
// by destructuring. Importing it, or passing it as a value, as to
// NodeHttpServer.layer(createServer, …), runs nothing. A namespace import's
// member is a value too, until the code runs it: os.EOL is allowed, and
// os.cpus() is not.
func checkPlatformUse(c *checker.Checker, file *ast.SourceFile, id *ast.Node) {
	ref, ok := platformReference(c, id)
	if !ok {
		return
	}
	value := id // the expression that denotes the platform API
	if ref.namespace {
		parent := id.Parent
		if !(ast.IsPropertyAccessExpression(parent) || ast.IsElementAccessExpression(parent)) || parent.Expression() != id {
			return // the namespace object itself, as a value
		}
		value = parent
	}
	use, runs := platformRun(c, value)
	if !runs {
		return
	}
	replacement := ref.replacement
	if id.Text() == "process" && ref.module == "" && ast.IsPropertyAccessExpression(use) {
		if r, ok := processMembers[use.Name().Text()]; ok {
			replacement = r
		}
	}
	name := nodeText(file, use)
	if ref.module != "" {
		name += " from " + ref.module
	}
	d := report(c, use, EK4402, name)
	noteReplacement(d, replacements[replacement])
	if ref.unresolved {
		cover(c, id) // EK4402 explains TypeScript's "cannot find name"
	}
}

// platformRun reports whether the code runs value, an expression that
// denotes a platform API, where it appears, and returns the expression that
// names what runs: value itself for a call, a new, or a tagged template, and
// the property access for a read of one of its properties.
func platformRun(c *checker.Checker, value *ast.Node) (use *ast.Node, runs bool) {
	parent := value.Parent
	for parent != nil && ast.IsParenthesizedExpression(parent) {
		value, parent = parent, parent.Parent
	}
	switch {
	case parent == nil:
		return nil, false
	case (ast.IsCallExpression(parent) || ast.IsNewExpression(parent)) && parent.Expression() == value:
		return value, true
	case ast.IsTaggedTemplateExpression(parent) && parent.AsTaggedTemplateExpression().Tag == value:
		return value, true
	case (ast.IsPropertyAccessExpression(parent) || ast.IsElementAccessExpression(parent)) && parent.Expression() == value:
		if call := parent.Parent; call != nil && (ast.IsCallExpression(call) || ast.IsNewExpression(call)) && call.Expression() == parent && isClockRead(c, call) {
			return nil, false // EK4403 reports it
		}
		return parent, true
	case ast.IsVariableDeclaration(parent) && parent.Initializer() == value && ast.IsBindingPattern(parent.Name()):
		return value, true // destructuring reads its properties
	case ast.IsBinaryExpression(parent) && parent.AsBinaryExpression().OperatorToken.Kind == ast.KindEqualsToken &&
		parent.AsBinaryExpression().Right == value && (ast.IsObjectLiteralExpression(parent.AsBinaryExpression().Left) || ast.IsArrayLiteralExpression(parent.AsBinaryExpression().Left)):
		return value, true
	case ast.IsSpreadElement(parent) || ast.IsSpreadAssignment(parent):
		return value, true // spreading reads its elements or properties
	}
	return nil, false
}

// isValueUse reports whether id, an identifier, refers to a value where it
// appears: not a declaration's name, a property name, or part of a type.
func isValueUse(id *ast.Node) bool {
	parent := id.Parent
	if parent == nil || ast.IsDeclarationName(id) || ast.IsRightSideOfQualifiedNameOrPropertyAccess(id) || ast.IsPartOfTypeNode(id) || ast.IsJSDocNode(parent) {
		return false
	}
	switch parent.Kind {
	case ast.KindPropertyAssignment, ast.KindPropertyDeclaration, ast.KindPropertySignature, ast.KindMethodDeclaration,
		ast.KindMethodSignature, ast.KindGetAccessor, ast.KindSetAccessor, ast.KindEnumMember:
		return parent.Name() != id
	case ast.KindImportSpecifier, ast.KindExportSpecifier, ast.KindLabeledStatement, ast.KindBreakStatement, ast.KindContinueStatement, ast.KindQualifiedName:
		return false
	}
	return true
}

// noteReplacement adds the lines that say what replaces a platform API.
func noteReplacement(d *ast.Diagnostic, r *replacement) {
	if r == nil {
		note(d, "Effect has no module for it: keep the code in a TypeScript module, and call that module through Effect.try or a trusted contract.")
		return
	}
	switch {
	case r.from == "":
		note(d, "Use {0}:   {1}", r.use, r.example)
	case r.example != "":
		note(d, "Use {0} from \"{1}\":   {2}", r.use, r.from, r.example)
	default:
		note(d, "Use {0} from \"{1}\".", r.use, r.from)
	}
	if r.layer {
		note(d, "Provide it with NodeServices.layer from @effect/platform-node, or the Bun or browser equivalent.")
	}
}

// clockReadOf returns the key in clockReads of the clock or randomness read
// that call, a call or new expression, makes, or "".
func clockReadOf(c *checker.Checker, call *ast.Node) string {
	decl := signatureDeclaration(c, call)
	if decl == nil {
		return ""
	}
	file := ast.GetSourceFileOfNode(decl)
	if !isLibFile(c, file) && !isPlatformPackage(packageOf(c, file)) {
		return ""
	}
	owner := ownerOf(decl)
	if owner == nil {
		return ""
	}
	key := declarationName(owner)
	if container := containerName(owner); container != "" {
		key = container + "." + key
	}
	switch {
	case ast.IsNewExpression(call):
		// new Date() reads the clock; new Date(value) parses value.
		if key == "DateConstructor" && decl.Kind == ast.KindConstructSignature && len(call.Arguments()) == 0 {
			return "DateConstructor.constructor"
		}
		return ""
	case key == "DateConstructor" && decl.Kind != ast.KindCallSignature:
		return ""
	}
	if _, ok := clockReads[key]; ok {
		return key
	}
	return ""
}

func isClockRead(c *checker.Checker, call *ast.Node) bool {
	return clockReadOf(c, call) != ""
}

// checkClockRead reports EK4403 for a clock or randomness read.
func checkClockRead(c *checker.Checker, file *ast.SourceFile, call *ast.Node) {
	key := clockReadOf(c, call)
	if key == "" {
		return
	}
	read := nodeText(file, call.Expression()) + "()"
	if ast.IsNewExpression(call) {
		read = "new " + read
	}
	d := report(c, call, EK4403, read)
	note(d, "Use {0}, which a test controls with TestClock or a seeded Random.", clockReads[key])
	note(d, "To allow it, set \"EK4403\": \"warning\" or \"off\" in compilerOptions.effeck.diagnostics.")
}
