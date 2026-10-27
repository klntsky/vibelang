# Introduction to Effect Platform

VibeLang reaches files, paths, processes, HTTP, randomness, and terminals through Effect's platform services. Write the application against those services; the file that exports `main` names its platform once, and the entry provides that platform's implementations to `main`. A `.vibe` file never runs a `node:*` API or a browser global itself (EK4402), and it needs no wrapper of its own around one.

## Using platform services

```vibe
// main.vibe
import "vibelang/platform/node"
import { FileSystem } from "effect/FileSystem"
import { Path } from "effect/Path"
import * as ChildProcess from "effect/process/ChildProcess"
import { ChildProcessSpawner } from "effect/process/ChildProcessSpawner"

export function main() {
  using directory = FileSystem.makeTempDirectoryScoped()
  const file = Path.join(directory, "greeting.txt")
  FileSystem.writeFileString(file, "Hello, VibeLang!")
  Console.log(FileSystem.readFileString(file))
  Console.log(ChildProcessSpawner.string(ChildProcess.make("echo", ["from a child process"])).trim())
}
// Output: Hello, VibeLang!
// Output: from a child process
```

Import a service by name from its module, as in `import { FileSystem } from "effect/FileSystem"`, and call its methods: `FileSystem.readFileString(file)` runs and returns the text, and its `PlatformError` joins E. The prelude's `FileSystem`, without an import, is the module, whose service is `FileSystem.FileSystem`. `import "vibelang/platform/node"` names the platform, so `main` writes no `provide`: when Node starts the module, it runs `main` with `NodeServices.layer` from `@effect/platform-node`, which supplies the services `main` needs (D312). The import loads nothing; the compiler removes it. The functions that call the services only require them, so a test provides others.

## Replacing Node.js and web calls

Each Node.js or web call has an Effect replacement. EK4402 and EK4403 name it, its import, and its layer:

| Call | Effect | Import from | Layer |
| --- | --- | --- | --- |
| `fs.readFile` | `FileSystem.readFileString(path)` | `effect/FileSystem` | `NodeServices.layer` |
| `os.tmpdir` | `FileSystem.makeTempDirectory()` | `effect/FileSystem` | `NodeServices.layer` |
| `path.join` | `Path.join(directory, name)` | `effect/Path` | `NodeServices.layer` |
| `process.cwd` | `Path.resolve()` | `effect/Path` | `NodeServices.layer` |
| `child_process.execFileSync` | `ChildProcessSpawner.string(ChildProcess.make(command, args))` | `effect/process/ChildProcessSpawner` | `NodeServices.layer` |
| `fetch` | `HttpClient.get(url)` | `effect/http` | `FetchHttpClient.layer` |
| `http.createServer` | `HttpRouter.serve(app)` | `effect/http` | `NodeHttpServer.layer(createServer, { port })` |
| `crypto.randomUUID` | `Crypto.randomUUIDv4()` | `effect/Crypto` | `NodeServices.layer` |
| `crypto.subtle.digest` | `Crypto.digest("SHA-256", bytes)` | `effect/Crypto` | `NodeServices.layer` |
| `process.argv` | `Stdio.args()` | `effect/Stdio` | `NodeServices.layer` |
| `process.env` | `Config.String("NAME")()` | the prelude | none |
| `console.log` | `Console.log(message)` | the prelude | none |
| `setTimeout` | `sleep("1 second")` | the prelude | none |
| `Date.now` | `Clock.currentTimeMillis()` | the prelude | none |
| `performance.now` | `Number(Clock.monotonicTimeNanos()) / 1e6` | the prelude | none |
| `Math.random` | `Random.next()` | the prelude | none |

Import a service from the module the table names, by its own name: `import { Crypto } from "effect/Crypto"`. `ChildProcess.make` needs `import * as ChildProcess from "effect/process/ChildProcess"` too, and `HttpClient`, `HttpRouter`, and `FetchHttpClient` are named imports of `effect/http`. `NodeServices.layer` provides `FileSystem`, `Path`, `Crypto`, `Stdio`, `Terminal`, and `ChildProcessSpawner`; `BunServices.layer` from `@effect/platform-bun` is Bun's. The platform import provides the platform's bundle to `main`; provide every other layer inside `main`. Effect's runtime provides the prelude's services itself.

Where Effect has no service yet, as for `os.hostname()`, `fs.lstat`, or `process.pid`, EK4402 says so: run the call in a hatch where you need it, `sync(hostname())`, or `try` when it can throw ([Creating Effects](../getting-started/005-creating-effects.md)).

## Installing runtime-specific packages

```sh
npm install effect@4.0.6 @effect/platform-node
npm install vibelang-runtime
npm install --save-dev vibelang
```

The `effect` package holds the interfaces. Install `@effect/platform-node` for Node.js or Deno, `@effect/platform-bun` for Bun, or `@effect/platform-browser` for browsers, in a release compatible with the project's Effect version.

```sh
# Deno
deno add npm:effect@4.0.6 npm:@effect/platform-node

# Bun
bun add effect@4.0.6 @effect/platform-bun
```

`vibelang` ships the `vibelang` binary. `npx vibelang` checks and compiles the project's `.vibe` and `.ts` sources, then you launch the generated JavaScript; `npx vibelang --noEmit` only checks. Bun and Deno projects also need `vibelang-runtime` and the compiler. Never pass `.vibe` files to a TypeScript runner.

## Writing platform-independent programs

```vibe
// greeting.vibe
// run: provide layerNoop({ readFileString: () => "  Hello, VibeLang!\n" }) { greet() }
import { FileSystem, layerNoop } from "effect/FileSystem"

export function greet() {
  const greeting = FileSystem.readFileString("./greeting.txt")
  Console.log(greeting.trim())
}
// Output: Hello, VibeLang!
```

`greet` requires `FileSystem` and nothing else, so it runs on any platform that provides it, and a test provides a layer of its own, here `layerNoop` from the same module. Shared modules name no platform; the entry file does.

## Supplying Node or Deno implementations

```vibe
// main-node.vibe
import "vibelang/platform/node"
import { FileSystem } from "effect/FileSystem"
import { greet } from "./greeting.vibe"

export function main() {
  FileSystem.writeFileString("./greeting.txt", "Hello, VibeLang!")
  greet()
}
// Output: Hello, VibeLang!
```

`import "vibelang/platform/node"` names Node and `main` needs `FileSystem`, so the compiled module runs `main` as Effect's canonical entry does, `NodeRuntime.runMain(Effect.provide(main(), NodeServices.layer))`, with the Node platform service bundle. `"vibelang/platform/bun"` names Bun's bundle and `"vibelang/platform/browser"` the browser's; the services each bundle supplies come from its installed declarations. The same package works in Deno's Node compatibility environment, subject to Deno permissions. Run the generated entry module with Node or Deno, granting only the permissions the application needs.

A `provide` inside `main` wins for the code inside it, so `provide NodeFileSystem.layer { ... }` replaces one service. Tests never run the entry: they import `main` and provide their own layers. EK3205 names any other service `main` needs; provide it inside `main`. Without a platform import the entry provides no platform, and EK3205 names the import that would.

## Supplying Bun implementations

```vibe
// main-bun.vibe
import "vibelang/platform/bun"
import { FileSystem } from "effect/FileSystem"
import { greet } from "./greeting.vibe"

export function main() {
  FileSystem.writeFileString("./greeting.txt", "Hello, VibeLang!")
  greet()
}
// Output: Hello, VibeLang!
```

The entry provides `BunServices.layer` from `@effect/platform-bun`. Only the platform import changes; the same `greet` uses Bun's implementations. A target-specific package can supply a narrower layer than the whole bundle, such as `NodeFileSystem.layer`. The FileSystem, Path, PlatformLogger, Runtime, and Terminal pages cover each integration.

## Coming from Effect

A service imported by name is its key: `FileSystem.readFileString(path)` is `yield* FileSystem.FileSystem` followed by the method call. An exported `main` in a file that imports `"vibelang/platform/node"` compiles to `NodeRuntime.runMain(main.pipe(Effect.provide(NodeServices.layer)))`, and `provide layer { ... }` is `Effect.provide(program, layer)`. Functions run on call, platform methods return success values directly, and no generator wraps the application.
