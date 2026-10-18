# Introduction to Effect Platform

Effeck uses Effect's platform services for files, paths, terminals, and application launch. Write the application against those interfaces, then supply implementations for Node.js, Deno, Bun, or the browser at the entry point.

## Choosing portable platform services

```effeck
export function readGreeting() {
  const fs = Effect.service(FileSystem.FileSystem)
  return fs.readFileString("./greeting.txt")
}
// string!PlatformError.PlatformError!FileSystem.FileSystem
```

`FileSystem`, `Path`, and `Terminal` are modules in `effect`; runtime adapters live in platform packages. Use these interfaces instead of calling `node:*` APIs or browser globals from `.effeck` files. Portability depends on the target actually implementing the services your program needs; a browser does not automatically gain arbitrary disk access.

| Capability | API |
| --- | --- |
| Files and directories | `FileSystem` |
| Path manipulation | `Path` |
| File logging | `Logger.toFile`, documented on the PlatformLogger page |
| Application launch | Platform `runMain` |
| Interactive input and output | `Terminal` |

## Installing runtime-specific packages

```sh
npm install effect@4.0.5 @effect/platform-node
npm install effeck-runtime
npm install --save-dev effeck
```

The `effect` package contains the interfaces. Install `@effect/platform-node` for Node.js or Deno, `@effect/platform-bun` for Bun, or `@effect/platform-browser` for browser integrations. Choose adapter releases compatible with the project's Effect version.

```sh
# Deno
deno add npm:effect@4.0.5 npm:@effect/platform-node

# Bun
bun add effect@4.0.5 @effect/platform-bun
```

`effeck` ships the `effeck` binary. Run `npx effeck` to check and compile the project's `.effeck` and `.ts` sources before launching the generated JavaScript; `npx effeck --noEmit` checks without emitting. The Bun and Deno projects also need `effeck-runtime` and the compiler. Do not pass `.effeck` files directly to a TypeScript runner.

## Writing platform-independent programs

```effeck
// greeting.effeck
export function main() {
  const fs = Effect.service(FileSystem.FileSystem)
  const greeting = fs.readFileString("./greeting.txt")
  log(greeting.trim())
}
```

`Effect.service` runs and returns the implementation associated with an existing Effect service key. Calls to its Effect-returning methods run normally and contribute their failures to E. Keep the runtime package import in the entry file so the application function can also run with test implementations.

## Supplying Node or Deno implementations

```effeck
// main-node.effeck
import { NodeRuntime, NodeServices } from "@effect/platform-node"
import { main } from "./greeting.effeck"

NodeRuntime.runMain(() => Effect.provide(() => main(), NodeServices.layer))
```

`NodeServices.layer` supplies the Node platform service bundle. The same package supports Deno's Node compatibility environment, subject to Deno permissions. Run the generated entry module with Node or Deno and grant only the permissions needed by the application.

## Supplying Bun implementations

```effeck
// main-bun.effeck
import { BunRuntime, BunServices } from "@effect/platform-bun"
import { main } from "./greeting.effeck"

BunRuntime.runMain(() => Effect.provide(() => main(), BunServices.layer))
```

Only the entry point changes: the same `main` uses Bun's implementations. A target-specific package can supply a narrower layer when the whole service bundle is unnecessary. See the FileSystem, Path, PlatformLogger, Runtime, and Terminal pages for individual integrations.

## Coming from Effect

Pass `main()` wherever an Effect program is expected: it is passed by name, so nothing runs until the runner runs it. Effeck functions run on calls, and platform methods return success values directly when called; there is no generator wrapper around the application.
