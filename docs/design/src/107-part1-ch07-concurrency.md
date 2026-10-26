### 7. Concurrency

Ordinary calls run in sequence. Concurrency is always an explicit combinator, and its concurrent parts are programs, so they start only when the combinator starts them.

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
const first = await.race [fromCache(id), fromNetwork(id)]
const quick = timeout(slowQuery(id), "500 millis")
```

Each combinator has its own rule for a failing branch; VibeLang adds nothing to Effect's semantics:

| Combinator | Succeeds with | When a branch fails |
| --- | --- | --- |
| `Effect.all`, `Effect.forEach` | Every result, in order | Interrupts the other branches, waits for their cleanup, fails with that failure |
| `Effect.race` | The first success | Keeps waiting; fails only if both fail, with both failures |
| `Effect.raceFirst` | The first to finish, success or failure | Fails if the first to finish failed; interrupts the other |
| `Effect.timeout` | The program's result | At the deadline, interrupts the program and fails with `Cause.TimeoutError` |
| `join [a, b]` | Every fiber's value, same shape | Interrupts the other fibers, fails with that failure |
| `await.all [a(), b()]` | Every result, same shape | Interrupts the other elements, fails with that failure |
| `await.allSettled [a(), b()]` | Every success or typed failure as a `Result`, same shape | Never fails with a typed error; defects and interruption propagate exactly as in `Effect.all` with `mode: "result"` (D258) |
| `await.race [a(), b()]` | The first to finish, success or failure | Fails if the first to finish failed; interrupts the others |
| `await.any [a(), b()]` | The first success | Keeps waiting; fails only if every element fails, with every failure |
| `for concurrent` | Nothing: the loop runs its body per item | Interrupts the other items, fails with that failure |

A `catch` around any of these sees the failure. When `Effect.race` fails because both branches failed, `catch` receives the first failure (Chapter 3).

#### Background work with a lifetime

`fork` starts a program on a new fiber owned by the enclosing block. When the block exits by any path, it interrupts the fiber and waits for its cleanup before releasing anything acquired earlier in the block. A function body is a block, so `fork` needs no `using` nearby.

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

`fork` is a keyword (D166): `fork buildReport(id)` forks one call; `fork { ... }` forks a block. It returns a `Fiber`; `join` waits for it (D225):

```ts es
function withProgress(id: string) {
  const pending = fork buildReport(id)
  sendProgressEmail(id)
  return join pending           // waits for it; fails if it failed
}
// Effect<Report, ReportError | MailError, Reports | Mailer>
```

`fork` adds the child's requirements to R and nothing else: the fiber cannot outlive `withProgress`, so no `Scope` appears in the contract, and its failures reach you only through `join`. The call form `fork(buildReport(id))` means the same. For a longer lifetime, say so: `forkIn(scope, program)` ties the fiber to a scope you hold, `detach(program)` to nothing (D170), `Effect.forkScoped` to the caller's scope (adding `Scope` to R), and `Effect.forkChild` to the current fiber.

`join` waits for a `Fiber`, or an array or object of fibers, keeping the shape (`Fiber.join`). When one fiber fails, `join` interrupts the others still running and propagates the failure, failing fast like `Effect.all`. A `Deferred` waits with its method, `d.await()` (D227):

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
  const profile = fork loadProfile(id)
  const orders = fork recentOrders(id)
  return { profile: join profile, orders: join orders }
}
```

To start several programs and wait for them in one step, use an await operation, `await.all [loadProfile(id), recentOrders(id)]` (below), instead of forking each.

`await` waits for promises in JavaScript, so in VibeLang code a plain `await` is an error that names its replacement (D226): `await fiber` is EK4609 (quick fix: `join fiber`); `await d` on a Deferred is EK4610 (quick fix: `d.await()`); `await loadProfile(id)` is EK4601, because a call already runs and waits where it stands; a promise is EK4602 and waits in a hatch, `try(async () => …)`, where `await` keeps its JavaScript meaning; anything else is EK4603. An `await` in a hatch's function that is not `async` is EK4604. `join` on anything but fibers is EK4611, and inside a hatch's function, which is plain TypeScript, EK4612.

#### Await operations

`await.all`, `await.allSettled`, `await.race`, and `await.any` (D196) come from the TC39 [await operations](https://github.com/tc39/proposal-await.ops) proposal (stage 1) and, for an object operand, the [Await Dictionary](https://github.com/tc39/proposal-await-dictionary) proposal (stage 3). Each behaves as the same-named `Promise` method. Each element is a program passed by name (D147), started in its own fiber; an element that is a fiber or a `Deferred` already running is waited for, not restarted.

```ts effect
const next = (jobs: Queue.Queue<Job>, shutdown: Deferred.Deferred<void>) =>
  Effect.raceAllFirst([
    Effect.map(Queue.take(jobs), (job) => run(job)),
    Effect.as(Deferred.await(shutdown), "stopped"),
    Effect.flatMap(Effect.sleep("30 seconds"), () => heartbeat())
  ])
```

```ts es
function next(jobs: Queue.Queue<Job>, shutdown: Deferred.Deferred<void>) {
  return await.race [
    run(jobs.take()),
    do {
      shutdown.await()
      "stopped"
    },
    do {
      sleep("30 seconds")
      heartbeat()
    }
  ]
}
```

- `await.all [a(), b()]` and `await.all { x: a(), y: b() }` are `Effect.all` with unbounded concurrency; an object's keys keep the order written.
- `await.allSettled` is `Effect.all` with `mode: "result"`.
- `await.race` is `Effect.raceFirst` over any number of elements. Of elements ready at once, the first written wins. An empty race never finishes, as `Promise.race([])` does not.
- `await.any` is `Effect.raceAll`.
- A `do { ... }` expression (D187) gives an element statements of its own. A race over an object is EK4608.
- **Controversial** (D196): `await.x` behaves like `Promise.x`, while a named Effect function keeps Effect's meaning, so `await.race [a(), b()]` takes the first to finish and `race(a(), b())` the first success.
- `select` (D168) was replaced by `await.race` (D197) and is EK4607. To poll without waiting, test `Queue.poll`'s result with an `if`.

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

`continue` ends one item. Because each item runs in its own fiber, a `break` that would leave the loop is EK4605 and a `return` is EK4606; throw to stop every item. Use `forEach` for the results.

An expression-bodied arrow has no block to own its child: `() => fork(p)` uses `Effect.forkScoped` and keeps `Scope` in R, unlike `() => { return fork(p) }` (D96). Returning a block-owned child currently returns a fiber already interrupted when that block exited; the compiler does not warn.

#### What a new fiber sees

A forked fiber starts with the services of the code that forked it, as in Effect; later changes in either fiber do not reach the other. Handlers are the one thing that does not cross to a new fiber; Chapter 9 introduces them and shows the compiler's check.
