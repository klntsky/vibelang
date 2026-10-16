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
  const [profile, orders] = Effect.all([() => loadProfile(id), () => recentOrders(id)], {
    concurrency: 2
  })
  return { profile, orders }
}
```

```ts es
const thumbnails = Effect.forEach(images, (image) => resize(image), { concurrency: 8 })
const fastest = Effect.race(() => fromCache(id), () => fromNetwork(id))
const first = Effect.raceFirst(() => fromCache(id), () => fromNetwork(id))
const quick = Effect.timeout(() => slowQuery(id), "500 millis")
```

Each combinator has its own rule for a failing branch, and EffectScript adds nothing to Effect's semantics here:

| Combinator | Succeeds with | When a branch fails |
| --- | --- | --- |
| `Effect.all`, `Effect.forEach` | Every result, in order | Interrupts the other branches, waits for their cleanup, and fails with that failure |
| `Effect.race` | The first success | Keeps waiting for the other branch; fails only if both fail, with both failures |
| `Effect.raceFirst` | Whichever branch finishes first, success or failure | Fails if the first branch to finish failed; the other is interrupted |
| `Effect.timeout` | The program's result | At the deadline, interrupts the program and fails with `Cause.TimeoutError` |

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
  fork(() => heartbeat())          // interrupted when serve exits, before the server closes
  return server.awaitShutdown()
}
```

`fork` returns a `Fiber`, which you can join or interrupt with Effect's `Fiber` module:

```ts es
function withProgress(id: string) {
  const pending = fork(() => buildReport(id))
  sendProgressEmail(id)
  return Fiber.join(pending)          // waits for it; fails if it failed
}
// Effect<Report, ReportError | MailError, Reports | Mailer>
```

`fork` adds the child's requirements to R and nothing else: the fiber cannot outlive `withProgress`, so no `Scope` appears in the contract, and its failures reach you only through `Fiber.join`. For a longer lifetime, say so with Effect's combinators: `Effect.forkScoped` ties the fiber to the caller's scope and adds `Scope` to R, `Effect.forkChild` ties it to the current fiber, and `Effect.forkDetach` ties it to nothing.

An expression-bodied arrow has no block to own its child: `() => fork(p)` uses `Effect.forkScoped` and keeps `Scope` in R, unlike `() => { return fork(p) }` (D96). Returning a block-owned child currently returns a fiber already interrupted when that block exited; the compiler does not warn.

#### What a new fiber sees

A forked fiber starts with the services of the code that forked it, as in Effect. Later changes in either fiber do not flow to the other. Handlers, introduced in Chapter 9, are the one thing that does not cross to a new fiber; Chapter 9 shows the compiler's check.
