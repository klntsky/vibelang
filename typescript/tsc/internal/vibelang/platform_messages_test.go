package vibelang

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// platformTypes stands in for @types/node: the modules and globals that the
// tests run, declared in a platform package.
const platformTypes = `declare module "node:fs/promises" {
  export function readFile(path: string, encoding: string): Promise<string>
  export function mkdtemp(prefix: string): Promise<string>
  export function lstat(path: string): Promise<unknown>
}
declare module "node:path" { export function join(...paths: string[]): string }
declare module "node:child_process" { export function execFileSync(command: string, args: string[]): string }
declare module "node:http" { export function createServer(): unknown }
declare module "node:os" { export function tmpdir(): string; export function hostname(): string }
declare var process: { cwd(): string; env: Record<string, string | undefined>; pid: number; argv: string[] }
declare var Buffer: { from(text: string): Uint8Array }`

const (
	provideNodeServices = `Provide it in main with NodeServices.layer from "@effect/platform-node", or BunServices.layer from "@effect/platform-bun".`
	importFileSystem    = `Import it with   import { FileSystem } from "effect/FileSystem"`
	importPath          = `Import it with   import { Path } from "effect/Path"`
	importCrypto        = `Import it with   import { Crypto } from "effect/Crypto"`
	provideCrypto       = `Provide it in main with NodeServices.layer from "@effect/platform-node", BunServices.layer from "@effect/platform-bun", or BrowserCrypto.layer from "@effect/platform-browser".`
	noServiceYet        = "Effect, VibeLang's standard library, has no service for it yet."
	allowEK4403         = `To allow it, set "EK4403": "warning" or "off" in "vibelang": { "diagnostics": { ... } } at the top level of tsconfig.json.`
)

// EK4402 and EK4403 name the Effect API that replaces a platform API, the
// import that brings it in, and the layer that provides it; for an API Effect
// lacks, they show it in a hatch, never in a wrapper module.
func TestPlatformReplacementMessages(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []string // EK4402's or EK4403's lines; none for code that runs no platform API
	}{
		{"fs", `import { readFile } from "node:fs/promises"
export const run = (path: string) => readFile(path, "utf8")`, []string{
			"readFile from node:fs/promises is a platform API, which VibeLang code does not call directly.",
			"Use the FileSystem service:   FileSystem.readFileString(path)", importFileSystem, provideNodeServices,
		}},
		{"path", `import * as path from "node:path"
export const run = (name: string) => path.join("dist", name)`, []string{
			"path.join from node:path is a platform API, which VibeLang code does not call directly.",
			"Use the Path service:   Path.join(directory, name)", importPath, provideNodeServices,
		}},
		{"child process", `import { execFileSync } from "node:child_process"
export const run = () => execFileSync("git", ["status"])`, []string{
			"execFileSync from node:child_process is a platform API, which VibeLang code does not call directly.",
			"Use the ChildProcessSpawner service:   ChildProcessSpawner.string(ChildProcess.make(command, args))",
			`Import it with   import { ChildProcessSpawner } from "effect/process/ChildProcessSpawner"   and   import * as ChildProcess from "effect/process/ChildProcess"`,
			provideNodeServices,
		}},
		{"http client", `export const run = (url: string) => fetch(url)`, []string{
			"fetch is a platform API, which VibeLang code does not call directly.",
			"Use HttpClient:   HttpClient.get(url)",
			`Import it with   import { HttpClient } from "effect/http"`,
			`Provide it in main with FetchHttpClient.layer from "effect/http", or NodeHttpClient.layerUndici from "@effect/platform-node".`,
		}},
		{"http server", `import { createServer } from "node:http"
export const run = () => createServer()`, []string{
			"createServer from node:http is a platform API, which VibeLang code does not call directly.",
			"Use HttpRouter:   HttpRouter.serve(app)",
			`Import it with   import { HttpRouter } from "effect/http"`,
			`Provide it in main with NodeHttpServer.layer(createServer, { port }) from "@effect/platform-node", which runs createServer itself.`,
		}},
		{"process.cwd", `export const run = () => process.cwd()`, []string{
			"process.cwd is a platform API, which VibeLang code does not call directly.",
			"Use the Path service:   Path.resolve()", importPath, provideNodeServices,
		}},
		{"process.env", `export const run = () => process.env.PORT`, []string{
			"process.env is a platform API, which VibeLang code does not call directly.",
			`Use Config, from the prelude:   Config.String("NAME")()`,
		}},
		{"process.argv", `export const run = () => process.argv`, []string{
			"process.argv is a platform API, which VibeLang code does not call directly.",
			"Use the Stdio service:   Stdio.args()",
			`Import it with   import { Stdio } from "effect/Stdio"`, provideNodeServices,
			"Stdio.args() leaves out argv[0] and argv[1], the runtime and the script.",
		}},
		{"tmpdir", `import { tmpdir } from "node:os"
export const run = () => tmpdir()`, []string{
			"tmpdir from node:os is a platform API, which VibeLang code does not call directly.",
			"Use the FileSystem service:   FileSystem.makeTempDirectory()", importFileSystem, provideNodeServices,
			"FileSystem.makeTempDirectory creates a directory in it; nothing returns its path alone.",
		}},
		{"mkdtemp", `import { mkdtemp } from "node:fs/promises"
export const run = () => mkdtemp("build-")`, []string{
			"mkdtemp from node:fs/promises is a platform API, which VibeLang code does not call directly.",
			"Use the FileSystem service:   FileSystem.makeTempDirectory()", importFileSystem, provideNodeServices,
		}},
		{"crypto digest", `export const run = (bytes: Uint8Array) => crypto.subtle.digest("SHA-256", bytes)`, []string{
			"crypto.subtle is a platform API, which VibeLang code does not call directly.",
			`Use the Crypto service:   Crypto.digest("SHA-256", bytes)`, importCrypto, provideCrypto,
		}},
		{"timers", `export const run = (f: () => void) => setTimeout(f, 10)`, []string{
			"setTimeout is a platform API, which VibeLang code does not call directly.",
			`Use sleep and Schedule, from the prelude:   sleep("1 second")`,
		}},
		{"bytes", `export const run = (text: string) => Buffer.from(text)`, []string{
			"Buffer.from is a platform API, which VibeLang code does not call directly.",
			"Use Uint8Array and TextEncoder or TextDecoder, which need no layer, or Base64 and Hex:   new TextEncoder().encode(text)",
			`Import it with   import { Base64, Hex } from "effect/encoding"`,
		}},
		{"gap", `import { hostname } from "node:os"
export const run = () => hostname()`, []string{
			"hostname from node:os is a platform API, which VibeLang code does not call directly.",
			noServiceYet, "Run it in a hatch:   sync(() => hostname()), or with try when it can throw.",
		}},
		{"gap with a near miss", `import { lstat } from "node:fs/promises"
export const run = (path: string) => lstat(path)`, []string{
			"lstat from node:fs/promises is a platform API, which VibeLang code does not call directly.",
			noServiceYet, "FileSystem.stat follows symbolic links, so it does not replace lstat.",
			"Run it in a hatch:   sync(() => lstat(...)), or with try when it can throw.",
		}},
		{"gap of a property", `export const run = () => process.pid`, []string{
			"process.pid is a platform API, which VibeLang code does not call directly.",
			noServiceYet, "Run it in a hatch:   sync(() => process.pid), or with try when it can throw.",
		}},
		{"clock", `export const run = () => Date.now()`, []string{
			"Date.now() reads the clock or randomness, which a test cannot control.",
			"Use Clock.currentTimeMillis(), the same number of milliseconds.", "In a test, TestClock sets it.", allowEK4403,
		}},
		{"monotonic clock", `export const run = () => performance.now()`, []string{
			"performance.now() reads the clock or randomness, which a test cannot control.",
			"Use Number(Clock.monotonicTimeNanos()) / 1e6: Clock.monotonicTimeNanos() is a bigint of nanoseconds, and performance.now() a number of milliseconds.",
			"In a test, TestClock sets it.", allowEK4403,
		}},
		{"date", `export const run = () => new Date()`, []string{
			"new Date() reads the clock or randomness, which a test cannot control.",
			"Use DateTime.nowAsDate() for a Date, or DateTime.now() for a DateTime.Utc.", "In a test, TestClock sets it.", allowEK4403,
		}},
		{"random", `export const run = () => Math.random()`, []string{
			"Math.random() reads the clock or randomness, which a test cannot control.",
			"Use Random.next(), a number from 0 up to 1.", "In a test, Random.withSeed(() => program(), seed) fixes it.", allowEK4403,
		}},
		{"crypto", `export const run = () => crypto.randomUUID()`, []string{
			"crypto.randomUUID() reads the clock or randomness, which a test cannot control.",
			"Use Crypto.randomUUIDv4().", importCrypto, provideCrypto,
			`In a test, provide TestCrypto.layer(seed) from "effect/testing"; a seeded Random does not reach Crypto.`, allowEK4403,
		}},
		{"pure web helpers", `export const run = (text: string, url: string) => [
  new TextEncoder().encode(text), new TextDecoder().decode(new Uint8Array()), new URL(url).host, new URLSearchParams(text).size,
  structuredClone({ text }), atob(btoa(text)), new Headers().has(text), new AbortController().signal.aborted,
]`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			files := computedKeyFiles(false)
			files["/Users/x/src/node_modules/@types/node/package.json"] = `{"name":"@types/node","types":"index.d.ts"}`
			files["/Users/x/src/node_modules/@types/node/index.d.ts"] = platformTypes
			files["/Users/x/src/main.vibe"] = test.source
			var got []string
			for _, d := range computedKeyProgram(t, files).GetSemanticDiagnostics(t.Context(), nil) {
				if d.Code() == EK4402.Code || d.Code() == EK4403.Code {
					got = append(got, diagnosticWithNotes(d))
				}
			}
			var want []string
			if test.want != nil {
				want = []string{strings.Join(test.want, "\n")}
			}
			if !slices.Equal(got, want) {
				t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n--\n"), strings.Join(want, "\n--\n"))
			}
		})
	}
}

// Every service the tables name exists, and each example calls only what
// its service's import brings in, the prelude, or a value the program has.
func TestPlatformServicesTable(t *testing.T) {
	named := func(where, service string) {
		if service != "" && platformServices[service] == nil {
			t.Errorf("%s names %q, which is not in platformServices", where, service)
		}
	}
	for module, service := range platformModules {
		named("platformModules["+module+"]", service)
	}
	for module, service := range prefixOnlyModules {
		named("prefixOnlyModules["+module+"]", service)
	}
	for global, service := range platformGlobals {
		named("platformGlobals["+global+"]", service)
	}
	for api, entry := range platformAPIs {
		named("platformAPIs["+api+"]", entry.service)
	}
	for read, entry := range clockReads {
		named("clockReads["+read+"]", entry.service)
	}
	imported := regexp.MustCompile(`import \{ ([^}]*) \}|import \* as (\w+)`)
	receiver := regexp.MustCompile(`(?:^|[^.\w/])([A-Za-z_]\w*)\.`) // not a path's or a member's
	prelude := []string{"Clock", "Config", "Console", "DateTime", "Inspectable", "Number", "Random", "Schedule", "Stream", "String", "fiber", "import"}
	check := func(where, example string, s *platformService) {
		names := slices.Clone(prelude)
		for _, m := range imported.FindAllStringSubmatch(s.imports, -1) {
			for _, name := range strings.Split(m[1]+m[2], ",") {
				names = append(names, strings.TrimSpace(name))
			}
		}
		for _, m := range receiver.FindAllStringSubmatch(example, -1) {
			if !slices.Contains(names, m[1]) {
				t.Errorf("%s: %s uses %s, which %q does not import", where, example, m[1], s.imports)
			}
		}
	}
	for name, s := range platformServices {
		check("platformServices["+name+"]", s.example, s)
	}
	for api, entry := range platformAPIs {
		if s := platformServices[entry.service]; s != nil {
			check("platformAPIs["+api+"]", entry.example, s)
		}
	}
	for read, entry := range clockReads {
		if s := platformServices[entry.service]; s != nil {
			check("clockReads["+read+"]", entry.use, s)
		}
	}
}
