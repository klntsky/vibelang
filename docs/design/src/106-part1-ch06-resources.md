### 6. Resources

#### `using`

`using` acquires a resource and releases it when the enclosing block exits: on success, on failure, and on interruption. It is standard JavaScript syntax (Explicit Resource Management reached Stage 4 on 2026-05-19, for ES2027) with Effect's guarantees added.

```ts effect
const readReport = Effect.fn("readReport")(
  function* (path: string) {
    const files = yield* Files
    const file = yield* files.open(path)
    const text = yield* file.readAll()
    return yield* parseReport(text)
  },
  Effect.scoped
)
```

```ts es
function readReport(path: string) {
  using file = Files.open(path)
  return parseReport(file.readAll())
}
// Effect<Report, IOError | ParseError, Files>
```

`Files.open` returns a scoped resource: `Effect<FileHandle, IOError, Scope>`. The block that declares `using` supplies the `Scope`, which is why `Scope` does not appear in `readReport`'s requirements. Several `using` declarations release in reverse order.

A call that runs an Effect is an acquisition even when its result has no `Symbol.dispose`; it receives the block Scope. Other initializers must be Disposable, `null`, or `undefined`. A Disposable result also runs `[Symbol.dispose]()` at block exit. If its Effect release already disposes it, this disposes it twice (D94).

In `for (using item of values)`, each iteration owns a Scope region, including `using` and `fork` inside its body (D95). C-style `for` heads still use TypeScript disposal and do not yet release on Effect failure or interruption; an Effect acquisition there reports TS2850.

#### Who owns a resource

Only `using` acquisitions belong to the block. A resource acquired without `using` belongs to the caller, even when the same block also has a `using`:

```ts es
function openAndLog(url: string) {
  const connection = Database.connect(url)      // no `using`: the caller owns it
  using log = Logs.open("connect.log")          // `using`: this block owns it
  log.write(`connected to ${url}`)
  return connection                             // still open when it reaches the caller
}
// Effect<Connection, ConnectError | IOError, Database | Logs | Scope>
```

`log` closes when `openAndLog` returns. `connection` stays open, and `Scope` in the contract says the caller supplies the scope that will close it:

```ts es
function openLog(day: string) {
  return Files.open(`/var/log/app-${day}.log`)    // no `using`: the caller owns the file
}
// Effect<FileHandle, IOError, Files | Scope>

function compareLogs(a: string, b: string) {
  using left = openLog(a)
  using right = openLog(b)
  return diffLines(left.readAll(), right.readAll())
}
// Effect<Diff, IOError, Files>; both files close when compareLogs returns
```

#### Defining a resource

Write the acquire and release steps with Effect's `acquireRelease`:

```ts es
function lease(name: string) {
  return Effect.acquireRelease(
    () => Locks.acquire(name),        // acquire
    (lock) => Locks.release(lock)     // release: always runs, exactly once
  )
}
// Effect<Lock, LockError, Locks | Scope>
```

The release is registered before anything else can happen, so a cancelled request never leaks a lock. By default the acquire step is also masked: an interruption that arrives while it runs waits for it to finish, and then the release runs. That suits a quick step such as taking a lock or opening a file. An acquisition that waits on the outside world, such as a connection that may never complete, passes `{ interruptible: true }` so that cancellation reaches the wait, as the adapter below does.

`using` adds no policy of its own: it supplies the block's Scope, and the resource's `acquireRelease` decides whether acquisition can be interrupted.

#### Resources and handlers

A resource held by code that a handler stops (Chapter 9) is released when it stops. If a handler decides not to continue your function, every `using` and `finally` between the handler and your code runs before the handler returns.

#### Adapting a callback API

Wrapping a callback-based API is boundary code. It belongs in TypeScript, using the runtime's `suspend`, which, unlike `Effect.callback`, tells the producer when a value arrived too late so the producer can release it:

```ts ts
// net-adapter.ts
import { Effect } from "effect"
import { suspend } from "@effeck/runtime"
import * as net from "node:net"

export const connect = (host: string, port: number) =>
  Effect.acquireRelease(
    suspend<net.Socket>((waiter) => {
      const socket = net.connect(port, host)
      socket.once("connect", () => {
        if (!waiter.resume(socket)) socket.destroy()   // the waiter was cancelled first: still ours
      })
      return () => socket.destroy()                    // cancelled while connecting
    }, { release: (socket) => socket.destroy() }),     // accepted, but cancelled before delivery
    (socket) => Effect.sync(() => socket.destroy()),
    { interruptible: true }                            // cancellation reaches the wait
  )
```

```ts es
import { connect } from "./net-adapter.ts"

function session(host: string) {
  using socket = connect(host, 5432)
  return handshake(socket)
}

function sessionWithin(host: string) {
  return Effect.timeout(() => session(host), "2 seconds")    // cancels a connect that hangs
}
```

The two acquisition policies behave differently when cancellation arrives during the wait:

| Acquisition | Cancellation during the wait | Who releases a value that arrives late |
| --- | --- | --- |
| Masked (the default) | Waits for the value, registers its release, then releases it | Effect, through the registered release |
| `{ interruptible: true }` | Wins at once: the registration's cleanup runs and the fiber moves on | The producer, when `resume` returns `false`; the runtime, through `release`, for a value accepted but not yet delivered |

Without `{ interruptible: true }`, `sessionWithin` would wait for a connection that never completes, and the race branches in the adapter would never run (Chapter 21).

A callback API that hands over a plain value, not a resource, can be adapted in `.effeck` with Effect's own `Effect.callback`. Its register function is boundary code, so it may call plain TypeScript. Its `resume` takes an Effect, and `.effeck` code passes an `Exit`: a call returns an `Exit` as data (Chapter 13), while `Effect.succeed(value)` would run where it is written:

```ts es
import { onceReady } from "./sdk.ts"     // (callback: (sdk: Sdk) => void) => void

function ready() {
  return Effect.callback<Sdk>((resume) => {
    onceReady((sdk) => resume(Exit.succeed(sdk)))
  })
}
// Effect<Sdk, never, never>
```

Only `suspend` reports a lost race, so an adapter for a value that needs releasing stays in TypeScript, as above.
