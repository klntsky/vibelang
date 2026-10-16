package effectscript

import (
	"strings"

	"github.com/microsoft/TypeScript/tsc/internal/ast"
	"github.com/microsoft/TypeScript/tsc/internal/checker"
)

// EffectScript's standard library is Effect's (design D6, Chapter 11). A
// Node.js or browser API is not usable from an .es file, and the error names
// the Effect module that replaces it (ES4402):
//
//	error ES4402: node:fs/promises is a platform API, which EffectScript code does not call directly.
//	  Use FileSystem from "effect/FileSystem":   FileSystem.readFileString(path)
//	  Provide it with NodeServices.layer from @effect/platform-node, or the Bun or browser equivalent.
//
// An import of a Node.js built-in module is reported at its module specifier,
// and a reference to a platform global such as fetch, setTimeout, process, or
// console where it appears. A clock or randomness read, such as Date.now(),
// is ES4403 instead, an error by default that a project may lower (D34); a
// read it allows compiles as a trusted sync call. Both apply everywhere in an
// .es file, a boundary's callback and the module's top level included.

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

// platformGlobals maps the platform globals that .es code may not use to the
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

// clockReads lists the clock and randomness reads (ES4403), by the
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

// platformModule is ES4402's text for an import, which names the module.
var platformModule = ES4402.With("{0} is a platform API, which EffectScript code does not call directly.")

// checkPlatform reports ES4402 for platform modules and globals, and ES4403
// for clock and randomness reads, in file.
func checkPlatform(c *checker.Checker, file *ast.SourceFile) {
	var visit func(node *ast.Node) bool
	visit = func(node *ast.Node) bool {
		switch {
		case ast.IsImportDeclaration(node) || ast.IsExportDeclaration(node):
			checkPlatformImport(c, file, node)
			return false
		case ast.IsImportCall(node):
			if arguments := node.Arguments(); len(arguments) > 0 && ast.IsStringLiteral(arguments[0]) {
				if r, ok := platformModuleOf(arguments[0].Text()); ok {
					noteReplacement(report(c, arguments[0], platformModule, arguments[0].Text()), replacements[r])
				}
			}
		case ast.IsCallExpression(node) || ast.IsNewExpression(node):
			checkClockRead(c, file, node)
		case ast.IsIdentifier(node):
			checkPlatformGlobal(c, file, node)
		}
		return node.ForEachChild(visit)
	}
	file.AsNode().ForEachChild(visit)
}

// checkPlatformImport reports an import or re-export of a platform module.
// An import of types only uses nothing at run time and stays allowed.
func checkPlatformImport(c *checker.Checker, file *ast.SourceFile, node *ast.Node) {
	specifier := node.ModuleSpecifier()
	if specifier == nil || !ast.IsStringLiteral(specifier) || importsOnlyTypes(node) {
		return
	}
	r, ok := platformModuleOf(specifier.Text())
	if !ok {
		return
	}
	d := report(c, specifier, platformModule, specifier.Text())
	noteReplacement(d, replacements[r])
	cover(c, specifier) // TypeScript cannot find the module without @types/node
}

// importsOnlyTypes reports whether an import or export declaration brings in
// types only: `import type`, `export type`, or named imports that are all
// marked type, as in import { type Server } from "node:http".
func importsOnlyTypes(node *ast.Node) bool {
	if ast.IsExportDeclaration(node) {
		return node.IsTypeOnly()
	}
	clause := node.AsImportDeclaration().ImportClause
	if clause == nil {
		return false // import "node:fs" runs the module
	}
	if clause.IsTypeOnly() {
		return true
	}
	if clause.Name() != nil {
		return false
	}
	bindings := clause.AsImportClause().NamedBindings
	if bindings == nil || bindings.Kind != ast.KindNamedImports {
		return false
	}
	elements := bindings.AsNamedImports().Elements.Nodes
	for _, element := range elements {
		if !element.AsImportSpecifier().IsTypeOnly {
			return false
		}
	}
	return len(elements) > 0
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

// checkPlatformGlobal reports a reference to a platform global.
func checkPlatformGlobal(c *checker.Checker, file *ast.SourceFile, id *ast.Node) {
	r, listed := platformGlobals[id.Text()]
	if !listed || !isValueUse(id) {
		return
	}
	// A name that resolves must be the platform's global, not a local, an
	// import, or a global the program declares. A platform name that does not
	// resolve, such as process without @types/node, is the platform's too, and
	// ES4402 explains TypeScript's "cannot find name".
	symbol := c.GetSymbolAtLocation(id)
	if symbol != nil && !isGlobalOf(symbol, func(file *ast.SourceFile) bool { return isLibFile(c, file) || isPlatformPackage(packageOf(c, file)) }) {
		return
	}
	name := id.Text()
	parent := id.Parent
	if ast.IsPropertyAccessExpression(parent) && parent.Expression() == id {
		member := parent.Name().Text()
		if call := parent.Parent; call != nil && (ast.IsCallExpression(call) || ast.IsNewExpression(call)) && call.Expression() == parent && isClockRead(c, call) {
			return // ES4403 reports it
		}
		name += "." + member
		if id.Text() == "process" {
			if m, ok := processMembers[member]; ok {
				r = m
			}
		}
	}
	d := report(c, id, ES4402, name)
	noteReplacement(d, replacements[r])
	if symbol == nil {
		cover(c, id)
	}
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

// checkClockRead reports ES4403 for a clock or randomness read.
func checkClockRead(c *checker.Checker, file *ast.SourceFile, call *ast.Node) {
	key := clockReadOf(c, call)
	if key == "" {
		return
	}
	read := nodeText(file, call.Expression()) + "()"
	if ast.IsNewExpression(call) {
		read = "new " + read
	}
	d := report(c, call, ES4403, read)
	note(d, "Use {0}, which a test controls with TestClock or a seeded Random.", clockReads[key])
	note(d, "To allow it, set \"ES4403\": \"warning\" or \"off\" in compilerOptions.effectscript.diagnostics.")
}
