# Introduction to Effect Platform

VibeLang uses Effect's platform services for files, paths, terminals, and application launch. Write the application against those interfaces; the entry point supplies Node.js, Deno, Bun, or browser implementations.

## Choosing portable platform services

```vibe
// run: provide FileSystem.layerNoop({ readFileString: () => "Hello, VibeLang!" }) { Console.log(readGreeting()) }
export function readGreeting() {
  const fs = Effect.service(FileSystem.FileSystem)
  return fs.readFileString("./greeting.txt")
}
// Effect<string, PlatformError.PlatformError, FileSystem.FileSystem>
// Output: Hello, VibeLang!
```

`FileSystem`, `Path`, and `Terminal` are modules in `effect`; runtime adapters live in platform packages. Use them instead of calling `node:*` APIs or browser globals from `.vibe` files. A program ports only to targets that implement the services it needs; a browser gains no arbitrary disk access.

| Capability | API |
| --- | --- |
| Files and directories | `FileSystem` |
| Path manipulation | `Path` |
| File logging | `Logger.toFile` (see the PlatformLogger page) |
| Application launch | Platform `runMain` |
| Interactive input and output | `Terminal` |

## Installing runtime-specific packages

```sh
npm install effect@4.0.5 @effect/platform-node
npm install vibelang-runtime
npm install --save-dev vibelang
```

The `effect` package holds the interfaces. Install `@effect/platform-node` for Node.js or Deno, `@effect/platform-bun` for Bun, or `@effect/platform-browser` for browsers, in a release compatible with the project's Effect version.

```sh
# Deno
deno add npm:effect@4.0.5 npm:@effect/platform-node

# Bun
bun add effect@4.0.5 @effect/platform-bun
```

`vibelang` ships the `vibelang` binary. `npx vibelang` checks and compiles the project's `.vibe` and `.ts` sources, then you launch the generated JavaScript; `npx vibelang --noEmit` only checks. Bun and Deno projects also need `vibelang-runtime` and the compiler. Never pass `.vibe` files to a TypeScript runner.

## Writing platform-independent programs

```vibe
// greeting.vibe
// run: provide FileSystem.layerNoop({ readFileString: () => "  Hello, VibeLang!\n" }) { main() }
export function main() {
  const fs = Effect.service(FileSystem.FileSystem)
  const greeting = fs.readFileString("./greeting.txt")
  Console.log(greeting.trim())
}
// Output: Hello, VibeLang!
```

`Effect.service` runs and returns the implementation for an existing Effect service key. Its Effect-returning methods run on call and add their failures to E. Keep the runtime package import in the entry file, so the application function also runs with test implementations.

## Supplying Node or Deno implementations

```vibe
// main-node.vibe
import { NodeRuntime, NodeServices } from "@effect/platform-node"
import { main } from "./greeting.vibe"

NodeRuntime.runMain(() => Effect.provide(() => { const fs = Effect.service(FileSystem.FileSystem); fs.writeFileString("./greeting.txt", "Hello, VibeLang!"); main() }, NodeServices.layer))
// Output: Hello, VibeLang!
```

`NodeServices.layer` supplies the Node platform service bundle. The same package works in Deno's Node compatibility environment, subject to Deno permissions. Run the generated entry module with Node or Deno, granting only the permissions the application needs.

## Supplying Bun implementations

```vibe
// main-bun.vibe
import { BunRuntime, BunServices } from "@effect/platform-bun"
import { main } from "./greeting.vibe"

BunRuntime.runMain(() => Effect.provide(() => { const fs = Effect.service(FileSystem.FileSystem); fs.writeFileString("./greeting.txt", "Hello, VibeLang!"); main() }, BunServices.layer))
// Output: Hello, VibeLang!
```

Only the entry point changes; the same `main` uses Bun's implementations. A target-specific package can supply a narrower layer than the whole bundle. The FileSystem, Path, PlatformLogger, Runtime, and Terminal pages cover each integration.

## Coming from Effect

`main()` given to a runner, or to any parameter that expects an Effect program, is passed by name: nothing runs until the program executes. Functions run on call, platform methods return success values directly, and no generator wraps the application.
