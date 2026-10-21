### 11. The standard library

```ts es
import { readFile } from "node:fs/promises"

function loadSettings(path: string) {
  return JSON.parse(readFile(path, "utf8"))
}
```

```text
error EK4402: node:fs/promises is a platform API, which VibeLang code does not call directly.
  settings.vibe:4:21   return JSON.parse(readFile(path, "utf8"))
  Use FileSystem from "effect/FileSystem":   FileSystem.readFileString(path)
  Provide it with NodeServices.layer from @effect/platform-node, or the Bun or browser equivalent.
```

VibeLang's standard library is Effect's. Node.js and browser APIs are not callable from `.vibe` files; each error names the Effect module that replaces the API. VibeLang adds no Node.js clones: the replacement is the module Effect already ships, implemented by the platform package your program runs with.

Imports, re-exports, and passing platform values to adapters are allowed (D108): `NodeHttpServer.layer(createServer, { port })` passes a host function without calling it. EK4402 reports execution: calls, construction, tagged templates, and reads of platform properties, including destructuring and spreading. Namespace members used as values, such as `os.EOL`, are allowed. The current check does not follow platform functions through local aliases, parameters, or objects returned by platform calls.

Three reasons:

- Platform APIs throw and reject in ways their types do not state, which Chapter 10's boundary rules exist to contain.
- An Effect module states its errors in its types and is a service, so a test replaces it with a Layer.
- Effect already ships these modules, with implementations for Node.js, Bun, and browsers, so cloning Node's API surface would duplicate them.

#### What replaces what

| You would use | In VibeLang | Import from |
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

Import a module whose service is a `Context.Service` value, such as `FileSystem`, from its module path, `effect/FileSystem`, to call its methods directly: `FileSystem.readFileString(path)`. The namespace from `effect` works too: `FileSystem.FileSystem.readFileString(path)`.

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
// number!PlatformError!FileSystem
```

#### Schemas

```ts effect
const Todo = Schema.Struct({
  userId: Schema.Number,
  id: Schema.Number,
  title: Schema.String,
  completed: Schema.Boolean
})
interface Todo extends Schema.Schema.Type<typeof Todo> {}
```

```ts es
schema Todo {
  userId: number
  id: number
  title: string
  completed: boolean
}
```

A `schema` declaration declares both: the constant `Todo`, a `Schema.Struct`, and the interface `Todo`, its decoded type (D133). Each field has a type, and the compiler writes its schema: a primitive, a literal or a union, an array, a tuple, a record, an object type, or the name of another schema, standing for that schema's decoded type. `name?: T` is `Schema.optional`, which accepts an absent key or `undefined`. A field that needs another schema, such as `Schema.DateTimeUtc` or a checked string, names a schema constant: `const Created = Schema.DateTimeUtc`, then `created: Created`. A field type with no schema form is EK2109. The interface comes from the struct, so the two cannot drift apart.

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
// Todo!HttpClientError | SchemaError!HttpClient
```

`HttpClient.get` returns an `Effect`, so the call runs it. `HttpClientResponse.schemaBodyJson(Todo)` returns a plain function, which `effect` exports with a trusted contract; calling it with the response returns an `Effect`, which runs.

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
  sleep("1 second")
  return `Hello, ${name}`
}

function heartbeat() {
  repeat(log("alive"), Schedule.spaced("30 seconds"))
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
// { port: number; databaseUrl: Redacted<string> }!ConfigError!
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
// string!PlatformError!ChildProcessSpawner
```

`ChildProcessSpawner.spawn` gives a handle whose output is a `Stream` and whose process belongs to a scope (its type requires `Scope`), so `using child = ChildProcessSpawner.spawn(command)` ties a long-running process to a block.

#### Providing the platform

```ts es
// main.vibe
import { NodeRuntime, NodeServices } from "@effect/platform-node"
import { FetchHttpClient } from "effect/http"

function main() {
  const branch = Effect.provide(() => currentBranch(), wire(NodeServices.layer, FetchHttpClient.layer))
  log(`on ${branch}`)
}

NodeRuntime.runMain(() => main())
```

Tests provide fakes instead: a `Layer.succeed(FileSystem, { … })` with the methods a test needs, or an in-memory implementation where Effect ships one, such as `KeyValueStore.layerMemory`.
