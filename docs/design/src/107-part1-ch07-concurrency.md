### 7. Concurrency

Ordinary calls run one after another. Running things at the same time is always written with an explicit combinator, and the concurrent parts are programs, so they start only when the combinator starts them.

#### Running several things at once

```ts effect
const dashboard = Effect.fn("dashboard")(function* (id: string) {
  const [profile, orders] = yield* Effect.all([loadProfile(id), recentOrders(id)], {
    concurrency: 2
  })
  return { profile, orders }
})
```

```ts es
function dashboard(id: string) {
  const [profile, orders] = all([loadProfile(id), recentOrders(id)], {
    concurrency: 2
  })
  return { profile, orders }
}
```

```ts es
const thumbnails = forEach(images, (image) => resize(image), { concurrency: 8 })
const fastest = race(fromCache(id), fromNetwork(id))
const first = Effect.raceFirst(fromCache(id), fromNetwork(id))
const quick = timeout(slowQuery(id), "500 millis")
```

Each combinator has its own rule for a failing branch, and Effeck adds nothing to Effect's semantics here:

| Combinator | Succeeds with | When a branch fails |
| --- | --- | --- |
| `Effect.all`, `Effect.forEach` | Every result, in order | Interrupts the other branches, waits for their cleanup, and fails with that failure |
| `Effect.race` | The first success | Keeps waiting for the other branch; fails only if both fail, with both failures |
| `Effect.raceFirst` | Whichever branch finishes first, success or failure | Fails if the first branch to finish failed; the other is interrupted |
| `Effect.timeout` | The program's result | At the deadline, interrupts the program and fails with `Cause.TimeoutError` |
| `await [a, b]` | Every fiber's value, in the same shape | Interrupts the other fibers and fails with that failure |
| `select` | The first case to finish; its statements run | Fails if the first case to finish failed; the other cases are interrupted |
| `for concurrent` | Nothing: the loop runs its body for each item | Interrupts the other items and fails with that failure |

A `catch` around any of these sees the failure as usual. When `Effect.race` fails because both branches failed, `catch` receives the first failure (Chapter 3).

#### Background work with a lifetime

`fork` starts a program on a new fiber owned by the enclosing block. When the block exits, by any path, the fiber is interrupted, and the block waits for its cleanup before releasing anything acquired earlier in the block. A function body is a block, so `fork` needs no `using` nearby.

```ts effect
const serve = Effect.fn("serve")(
  function* (port: number) {
    const http = yield* Http
    const server = yield* http.listen(port)
    yield* Effect.forkScoped(heartbeat())
    return yield* server.awaitShutdown()
  },
  Effect.scoped
)
```

```ts es
function serve(port: number) {
  using server = Http.listen(port)
  fork heartbeat()          // interrupted when serve exits, before the server closes
  return server.awaitShutdown()
}
```

`fork` is a keyword (D166): `fork buildReport(id)` forks one call, and `fork { ... }` forks a block. It returns a `Fiber`, and `await` waits for it (D167):

```ts es
function withProgress(id: string) {
  const pending = fork buildReport(id)
  sendProgressEmail(id)
  return await pending          // waits for it; fails if it failed
}
// Report!ReportError | MailError!Reports | Mailer
```

`fork` adds the child's requirements to R and nothing else: the fiber cannot outlive `withProgress`, so no `Scope` appears in the contract, and its failures reach you only through `await`. The call form `fork(buildReport(id))` means the same. For a longer lifetime, say so: `forkIn(scope, program)` ties the fiber to a scope you hold, `detach(program)` ties it to nothing (D170), `Effect.forkScoped` ties it to the caller's scope and adds `Scope` to R, and `Effect.forkChild` ties it to the current fiber.

`await` waits for a `Fiber`, a `Deferred`, or an array or object of them, and keeps the shape. When one fiber fails, the others still running are interrupted and the failure propagates, as `Effect.all` fails fast:

```ts effect
const summary = Effect.fn("summary")(function* (id: string) {
  const profile = yield* Effect.forkChild(loadProfile(id))
  const orders = yield* Effect.forkChild(recentOrders(id))
  const [p, o] = yield* Fiber.joinAll([profile, orders])
  return { profile: p, orders: o }
})
```

```ts es
function summary(id: string) {
  const [profile, orders] = await [fork loadProfile(id), fork recentOrders(id)]
  return { profile, orders }
}
```

`await` never waits for a call: a call already runs and waits where it stands, so `await loadProfile(id)` is EK4601. A promise is EK4602; wait for one in a hatch, `try(async () => …)`, where `await` keeps its JavaScript meaning. Anything else is EK4603, and an `await` in a hatch's function that is not `async` is EK4604.

#### Selecting the first event

`select` starts each case in its own fiber and runs the statements of the first to finish; the other cases are interrupted (D168):

```ts effect
const next = (jobs: Queue.Queue<Job>, shutdown: Deferred.Deferred<void>) =>
  Effect.raceAllFirst([
    Effect.map(Queue.take(jobs), (job) => ({ kind: "job" as const, job })),
    Effect.as(Deferred.await(shutdown), { kind: "shutdown" as const }),
    Effect.as(Effect.sleep("30 seconds"), { kind: "idle" as const })
  ]).pipe(Effect.flatMap((event) => /* switch on event.kind */ handle(event)))
```

```ts es
function next(jobs: Queue.Queue<Job>, shutdown: Deferred.Deferred<void>) {
  select {
    case const job = jobs.take(): return run(job)
    case shutdown.await(): return "stopped"
    case sleep("30 seconds"): return heartbeat()
  }
}
```

- A case's expression is a program: it is passed, not run, like a by-name argument. `case const x = program:` binds the winner's value.
- A failing case that finishes first fails the select. Of cases ready at once, the first written wins: a case that finishes while the cases are starting stops the rest from starting.
- A `default:` clause runs when no case finishes while the cases start, which makes `select` a poll.
- `break` leaves the select, as it leaves a `switch`; `continue` and `return` keep their meaning in the enclosing loop and function. A select whose clauses all return ends the function.

#### Concurrent loops

`for concurrent` runs its body for each item in its own fiber and joins them all before the loop ends (D169). `for concurrent(n)` runs at most `n` at once:

```ts effect
const saveAll = (ids: ReadonlyArray<string>) =>
  Effect.forEach(ids, (id) => Effect.flatMap(fetchPage(id), save), { concurrency: 8, discard: true })
```

```ts es
function saveAll(ids: ReadonlyArray<string>) {
  for concurrent(8) (const id of ids) save(fetchPage(id))
}
```

A failure interrupts the items still running and fails the loop. `continue` ends one item. A `break` that would leave the loop is EK4605 and a `return` is EK4606, because each item runs in its own fiber; throw to stop every item. Use `forEach` when you need the results.

An expression-bodied arrow has no block to own its child: `() => fork(p)` uses `Effect.forkScoped` and keeps `Scope` in R, unlike `() => { return fork(p) }` (D96). Returning a block-owned child currently returns a fiber already interrupted when that block exited; the compiler does not warn.

#### What a new fiber sees

A forked fiber starts with the services of the code that forked it, as in Effect. Later changes in either fiber do not flow to the other. Handlers, introduced in Chapter 9, are the one thing that does not cross to a new fiber; Chapter 9 shows the compiler's check.
