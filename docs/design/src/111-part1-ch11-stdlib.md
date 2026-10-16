### 11. The standard library

```ts es
import { readFile } from "node:fs/promises"

function loadSettings(path: string) {
  return JSON.parse(readFile(path, "utf8"))
}
```

```text
error ES4402: node:fs/promises is a platform API, which EffectScript code does not call directly.
  settings.es:1:26   import { readFile } from "node:fs/promises"
  Use FileSystem from "effect/FileSystem":   FileSystem.readFileString(path)
  Provide it with NodeServices.layer from @effect/platform-node, or the Bun or browser equivalent.
```

EffectScript's standard library is Effect's. Node.js and browser APIs are not callable from `.es` files, and each error names the Effect module that replaces the API. EffectScript adds no Node.js clones: the replacement is the module Effect already ships, implemented by the platform package your program runs with.

There are three reasons. Platform APIs throw and reject in ways their types do not state, which is exactly what Chapter 10's boundary rules exist to contain. An Effect module states its errors in its types and is a service, so a test replaces it with a Layer. And Effect already ships these modules, with implementations for Node.js, Bun, and browsers, so cloning Node's API surface would duplicate them.

#### What replaces what

| You would use | In EffectScript | Import from |
| --- | --- | --- |
| `fs`, `fs/promises` | `FileSystem` | `effect/FileSystem` |
| `path` | `Path` | `effect/Path` |
| `fetch`, `http.request`, an HTTP client library | `HttpClient`, `HttpClientRequest`, `HttpClientResponse` | `effect/http` |
| `http.createServer`, an HTTP framework | `HttpRouter`, `HttpServer`; schema-first APIs with `HttpApi` | `effect/http`, `effect/http-api` |
| `child_process` | `ChildProcess`, `ChildProcessSpawner` | `effect/process` |
| `setTimeout`, `setInterval` | `Effect.sleep`; `Effect.repeat` with a `Schedule` | `effect` |
| `process.env` | `Config` | `effect` |
| `console.log` | `Effect.log`, `Console` | `effect` |
| `Date.now()`, `new Date()` | `Clock`, `DateTime` | `effect` |
| `Math.random()` | `Random` | `effect` |
| `crypto.randomUUID()`, `crypto.subtle.digest` | `Crypto` | `effect` |
| `net`, `dgram`, `WebSocket` | `Socket`, `SocketServer`, `DatagramSocket` | `effect/socket` |
| `dns` | `Dns` | `effect/net` |
| `worker_threads`, Web Workers | `Worker`, `WorkerRunner` | `effect/workers` |
| `process.argv`, `readline`, `process.stdin` | `Command`, `Flag`, `Argument`; `Terminal`, `Stdio` | `effect/cli`, `effect` |
| `localStorage`, `sessionStorage` | `KeyValueStore` | `effect/persistence` |
| a database driver | `SqlClient`, with a driver package such as `@effect/sql-sqlite-node` | `effect/sql` |
| `EventEmitter`, DOM events | `PubSub`, `Stream.fromEventListener` | `effect` |
| `stream.Readable` | `Stream`; `NodeStream.fromReadable` at the edge | `effect`, `@effect/platform-node` |

The runtime packages supply the implementations and the runner: `@effect/platform-node` (`NodeRuntime`, `NodeServices.layer`), `@effect/platform-bun`, and `@effect/platform-browser`.

A module whose service is a `Context.Service` value, such as `FileSystem`, is imported from its module path, `effect/FileSystem`, so its methods are called directly: `FileSystem.readFileString(path)`. Importing the namespace from `effect` works too, as `FileSystem.FileSystem.readFileString(path)`.

#### Files

```ts effect
const countLines = Effect.fn("countLines")(function* (path: string) {
  const fs = yield* FileSystem.FileSystem
  const text = yield* fs.readFileString(path)
  return text.split("\n").length
})
```

```ts es
import { FileSystem } from "effect/FileSystem"

function countLines(path: string) {
  return FileSystem.readFileString(path).split("\n").length
}
// Effect<number, PlatformError, FileSystem>
```

#### HTTP

```ts effect
const getTodo = Effect.fn("getTodo")(function* (id: number) {
  const response = yield* HttpClient.get(`https://jsonplaceholder.typicode.com/todos/${id}`)
  return yield* HttpClientResponse.schemaBodyJson(Todo)(response)
})
```

```ts es
import { HttpClient, HttpClientResponse } from "effect/http"

function getTodo(id: number) {
  const response = HttpClient.get(`https://jsonplaceholder.typicode.com/todos/${id}`)
  return HttpClientResponse.schemaBodyJson(Todo)(response)
}
// Effect<Todo, HttpClientError | SchemaError, HttpClient>
```

`HttpClient.get` returns an `Effect`, so the call runs it. `HttpClientResponse.schemaBodyJson(Todo)` returns a plain function, which `effect` exports with a trusted contract, and calling that function with the response returns an `Effect`, which runs.

#### Timers

```ts effect
const greetLater = Effect.fn("greetLater")(function* (name: string) {
  yield* Effect.sleep("1 second")
  return `Hello, ${name}`
})

const heartbeat = Effect.log("alive").pipe(Effect.repeat(Schedule.spaced("30 seconds")))
```

```ts es
function greetLater(name: string) {
  Effect.sleep("1 second")
  return `Hello, ${name}`
}

function heartbeat() {
  Effect.repeat(() => Effect.log("alive"), Schedule.spaced("30 seconds"))
}
```

Time comes from the `Clock` service, so a test moves it forward with `TestClock.adjust` instead of waiting (Chapter 12).

#### Environment

```ts effect
const serverConfig = Effect.gen(function* () {
  const port = yield* Config.Port("PORT").pipe(Config.withDefault(3000))
  const databaseUrl = yield* Config.Redacted("DATABASE_URL")
  return { port, databaseUrl }
})
```

```ts es
const Port = Config.withDefault(Config.Port("PORT"), 3000)
const DatabaseUrl = Config.Redacted("DATABASE_URL")

function serverConfig() {
  return { port: Port(), databaseUrl: DatabaseUrl() }
}
// Effect<{ port: number; databaseUrl: Redacted<string> }, ConfigError, never>
```

A `Config` is data until you call it (Chapter 13). The module's top level builds `Port` and `DatabaseUrl`; `Port()` reads the variable and fails with `ConfigError` if it is set but invalid. A test supplies values with a `ConfigProvider` instead of touching the process environment.

#### Child processes

```ts effect
const currentBranch = Effect.fn("currentBranch")(function* () {
  const spawner = yield* ChildProcessSpawner.ChildProcessSpawner
  const output = yield* spawner.string(ChildProcess.make("git", ["branch", "--show-current"]))
  return output.trim()
})
```

```ts es
import { ChildProcess } from "effect/process"
import { ChildProcessSpawner } from "effect/process/ChildProcessSpawner"

function currentBranch() {
  return ChildProcessSpawner.string(ChildProcess.make("git", ["branch", "--show-current"])).trim()
}
// Effect<string, PlatformError, ChildProcessSpawner>
```

`ChildProcessSpawner.spawn` gives a handle whose output is a `Stream` and whose process belongs to a scope (its type requires `Scope`), so `using child = ChildProcessSpawner.spawn(command)` ties a long-running process to a block.

#### Providing the platform

```ts es
// main.es
import { NodeRuntime, NodeServices } from "@effect/platform-node"
import { FetchHttpClient } from "effect/http"

function main() {
  const branch = Effect.provide(() => currentBranch(), wire(NodeServices.layer, FetchHttpClient.layer))
  Effect.log(`on ${branch}`)
}

NodeRuntime.runMain(() => main())
```

Tests provide fakes instead: a `Layer.succeed(FileSystem, { … })` with the methods a test needs, or an in-memory implementation where Effect ships one, such as `KeyValueStore.layerMemory`.
