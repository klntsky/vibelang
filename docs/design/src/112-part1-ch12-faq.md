### 12. Questions Effect users ask

Short answers, each with the code. Appendix C maps every section of Effect's documentation to Effeck, including the many features that carry over unchanged.

#### Where did `async` and `await` go?

```ts es
function loadProfile(id: string) {
  const user = Users.find(id)                       // waits for the database: no await
  const avatar = Effect.tryPromise({                // a Promise from a TypeScript library
    try: (signal) => avatarSdk.fetch(user.avatarId, { signal }),
    catch: (cause) => new AvatarError({ cause })
  })
  return { user, avatar }
}
// Effect<{ user: User; avatar: Avatar }, UserNotFound | AvatarError, Users>
```

A call waits for its callee, whether the callee finishes at once or after a network round trip. While it waits, the fiber is parked and other fibers run. There is no `async` function and no `await`, so there are no "colored" functions: every function can wait. A Promise from a TypeScript library crosses with `Effect.tryPromise`, or with `Effect.promise` when a rejection is a bug. Platform APIs that return Promises, such as `fetch`, are replaced by Effect modules (Chapter 11).

#### How do I run things concurrently?

```ts es
function homePage(userId: string) {
  const [feed, unread] = Effect.all(
    [() => Feed.load(userId), () => Notifications.unread(userId)],
    { concurrency: "unbounded" }
  )
  const previews = Effect.forEach(feed.items, (item) => preview(item), { concurrency: 8 })
  return { previews, unread }
}
```

Calls run in order. Concurrency is always a combinator over programs: `Effect.all`, `Effect.forEach`, `Effect.race`, `Effect.raceFirst`, or `fork` (Chapter 7). `Semaphore`, `Latch`, `Queue`, `Deferred`, and `PubSub` work unchanged.

#### Where did `Effect.gen` and `yield*` go?

```ts es
function displayName(id: string) {
  return Users.find(id).name
}
```

Every function body plays the part of the generator, and every call that returns an Effect plays the part of `yield*`. `function*` now declares a producer (Chapter 8), so APIs that take a generator, such as `Effect.gen` and `Option.gen`, stay in `.ts` files (EK1002). The editor converts an `Effect.gen` function for you (Chapter 28).

#### What does `throw` mean now?

```ts es
function parseId(raw: string) {
  if (!/^\d+$/.test(raw)) throw new InvalidId({ raw })     // a typed failure: InvalidId joins E
  if (raw.length > 64) Effect.die(new Error("caller bug"))  // a defect, on purpose
  return Number(raw)
}
// Effect<number, InvalidId, never>
```

`throw` is `Effect.fail`. Every thrown value joins E with its type and can be caught, including `throw new Error("unreachable")`, which inside `Effect.gen` would be a defect. To signal a bug, die explicitly with `Effect.die`. Defects also come from exceptions no contract lists, and from `Effect.orDie(() => …)` (Chapter 3).

#### What happens when code that is not Effect throws?

```ts es
function readPort(text: string) {
  try {
    return JSON.parse(text).port as number    // JSON.parse fails with SyntaxError
  } catch (e) {                               // e: SyntaxError
    return 8080
  }
}
// Effect<number, never, never>
```

JavaScript's built-ins and Effect's own functions ship with contracts that say which exceptions are expected failures. Other TypeScript code needs an explicit boundary or a trusted contract (EK4401), and platform APIs are replaced by Effect modules (EK4402). An exception that no contract lists is a defect. Unlike a JavaScript `catch` inside `Effect.gen`, an Effeck `catch` does not see it, and a `catch` that could only have caught such exceptions is reported, EK2004 (Chapter 3).

#### How do I use `Option` and `Result`?

```ts es
function displayName(user: User) {
  return Option.match(Option.fromNullishOr(user.nickname), {
    onNone: () => user.name,
    onSome: (nickname) => nickname
  })
}

function tryCharge(cartId: string) {
  const outcome = Effect.result(() => Payments.charge(cartId))    // Result<Receipt, PaymentError>
  return Result.isSuccess(outcome) ? outcome.success : undefined
}
// Effect<Receipt | undefined, never, Payments>
```

`Option` and `Result` (Effect 4's name for `Either`) are plain data, imported from `effect` and used as in TypeScript. `Effect.result` and `Effect.option` turn a failure into a value; `throw` turns a value back into a failure. `Option.gen` and `Result.gen` take generators, so they stay in `.ts` files.

#### How do I validate data with `Schema`?

```ts es
class Signup extends Schema.Class<Signup>("Signup")({
  email: Schema.String,
  plan: Schema.Literals(["free", "pro"])
}) {}

function register(body: unknown) {
  const signup = Schema.decodeUnknownEffect(Signup)(body)    // fails with SchemaError
  return Accounts.create(signup.email, signup.plan)
}
// Effect<Account, SchemaError | AccountError, Accounts>
```

`Schema` is in the prelude and unchanged. A decoder that returns an `Effect` runs when called and fails with a typed `SchemaError`. The other forms (`decodeUnknownSync`, `is`) are trusted functions whose exceptions are defects, so `.effeck` code prefers the `Effect` forms. Errors that must cross a process boundary are `Schema.TaggedError` classes.

#### How do I work with streams?

```ts es
function digestActive(users: Stream.Stream<User, DbError>) {
  for (const user of users) {
    if (user.active) Mailer.digest(user)
  }
}
// Effect<void, DbError | MailError, Mailer>

function firstErrorLines(path: string) {
  return Stream.runCollect(Stream.take(Stream.filter(lines(path), isError), 10))
}
```

Effect's `Stream`, `Sink`, and `Channel` modules work unchanged: their operators take plain functions and Effect-returning callbacks, and the `Stream.run*` functions return Effects, so calling them runs the stream. `for...of` iterates a `Stream` directly, and a `function*` producer such as `lines` is a `Stream` wherever a `Stream` is expected (Chapter 8).

#### How do I test?

```ts es
import { assert, it } from "@effect/vitest"
import { TestClock } from "effect/testing"

it.effect("a reminder fires after one day", () => {
  const reminder = fork(() => remindLater("ada", "1 day"))
  TestClock.adjust("1 day")                          // virtual time: nothing waits
  assert.strictEqual(Fiber.join(reminder), "sent")
})
```

`@effect/vitest` works unchanged: `it.effect` runs an Effeck function as a test, `assert` comes from the same package, and `TestClock` controls time. Fake services are Layers (Chapter 4), and a control operation is scripted with a handler (Chapter 9). An `Exit` is data until called, so `assert.deepStrictEqual(Effect.exit(() => f()), Exit.succeed(1))` compares two `Exit` values. A test written as `async () => { await Effect.runPromise(p) }` stays in a `.ts` file: `.effeck` has no `await`, and a runner inside a test body is EK1005 (Chapter 13).

#### How do I log, trace, and record metrics?

```ts es
const checkouts = Metric.counter("checkouts")

function checkout(cartId: string) {
  Effect.log(`checkout ${cartId}`)
  const receipt = Effect.withSpan(() => Payments.charge(cartId), "charge")
  Metric.update(checkouts, 1)
  return receipt
}
```

Unchanged: `Effect.log` and its levels, `Effect.annotateLogs`, `Effect.withLogSpan`, `Effect.withSpan`, `Logger`, and `Metric`, each taking a program argument where it takes an Effect today. Functions are untraced unless you add a span or turn on `tracing` for the project (Chapter 28). Exporters come from `effect/observability`.

#### How do I read configuration?

```ts es
const Port = Config.withDefault(Config.Port("PORT"), 3000)

function main() {
  const port = Port()                         // reads PORT; ConfigError if it is invalid
  Effect.log(`listening on ${port}`)
}
```

A `Config` is data until you call it (Chapter 13), so `Config.all`, `Config.nested`, and `Config.withDefault` compose it first and a call reads it. A `ConfigProvider` replaces the environment in tests (Chapter 11).

#### How do I run a program?

```ts es
// main.effeck
import { NodeRuntime } from "@effect/platform-node"

function main() {
  Effect.provide(() => serve(8080), AppLive)
}

NodeRuntime.runMain(() => main())
```

A program starts only at a runner: `NodeRuntime.runMain` or `BunRuntime.runMain` for a process, `it.effect` for a test, and `Effect.runPromise` or a `ManagedRuntime` where TypeScript code needs a result. A web framework that calls your handlers builds one `ManagedRuntime` from the application layer and runs each handler through it, so the layer is built once (Chapter 4). A module's top level cannot run a program (EK1001).

#### Do I still use `pipe`?

```ts es
const user = pipe(() => fetchUser(id), Effect.retry({ times: 3 }), Effect.timeout("2 seconds"))
```

Rarely. Statements replace sequencing, error handling, and cleanup, so most pipelines disappear. The built-in `pipe` stacks policies over a program and still works on data. There is no `|>` (Chapter 5).

#### How do I pattern match?

```ts es
function area(shape: Circle | Square) {
  switch (shape._tag) {
    case "Circle": return Math.PI * shape.radius ** 2
    case "Square": return shape.side ** 2
  }
}

const describe = Match.type<Circle | Square>().pipe(
  Match.tag("Circle", (c) => `circle of radius ${c.radius}`),
  Match.tag("Square", (s) => `square of side ${s.side}`),
  Match.exhaustive
)
```

TypeScript's narrowing does most of the work: `switch` on `_tag`, `instanceof`, and `catch` narrowing on errors (Chapter 3). Effect's `Match` module works unchanged; its handlers are plain functions, so they cannot pause (EK4301). TC39's pattern matching proposal is at Stage 1 and is not used.

#### How do fibers, interruption, and cancellation work?

```ts es
function search(query: string) {
  const scan = fork(() => slowIndexScan(query))
  const cached = SearchCache.lookup(query)
  if (cached !== undefined) {
    Fiber.interrupt(scan)               // cancels the scan; its finally blocks run first
    return cached
  }
  return Fiber.join(scan)
}
```

They are Effect's. A fiber is Effect's fiber, `fork` starts one owned by the enclosing block (Chapter 7), and `Fiber.join` and `Fiber.interrupt` work unchanged. Interruption runs every `finally` block and `using` release on the way out, and `catch` never catches it (Chapter 3). `Effect.uninterruptible(() => …)` protects a region, `Effect.onInterrupt` reacts to cancellation, and `Effect.timeout` and `Effect.race` interrupt the branches they discard. A Promise is cancelled through the `AbortSignal` that `Effect.tryPromise` passes.

#### How do I wrap a callback API?

```ts es
function ready() {
  return Effect.callback<Sdk>((resume) => {
    onceReady((sdk) => resume(Exit.succeed(sdk)))
  })
}
```

`Effect.callback` works in `.effeck`: its register function is a boundary, so it may call plain TypeScript, and its `resume` takes an Effect, to which `.effeck` code passes an `Exit`, data until run (Chapter 13). A callback API that delivers a resource, such as a socket, is adapted in TypeScript with the runtime's `suspend` and `Effect.acquireRelease(…, { interruptible: true })`, so a value that arrives after cancellation is released (Chapter 6). Node.js and browser callback APIs already have Effect modules (Chapter 11).

#### Can I mix `.ts` and `.effeck` files?

```ts es
import { findUser } from "./users-repo.ts"     // Effect.fn("findUser")(function* …)

export function title(id: string) {
  return findUser(id).name.toUpperCase()
}
```

Yes, in one program: `effeck` checks both, and the editor serves both (Chapter 28). A TypeScript function that returns an `Effect` is called like an Effeck function, and TypeScript sees every Effeck function as one that returns an `Effect` (Chapter 10).

#### Is there anything from Effect I cannot use?

```ts es
const legacy = Effect.tryPromise({
  try: () => Effect3.runPromise(oldProgram),
  catch: (cause) => new LegacyError({ cause })
})
```

Five things, each with a replacement. Generator-based APIs (`Effect.gen`, `Option.gen`) cannot be written in `.effeck` files; call them from TypeScript. Effectful handlers in `Option.match` or `Match` are rejected (EK4301); use `if` or `switch`. Runners inside code a fiber runs are rejected (EK1005); call or `fork` the program. A TypeScript function whose requirements include a control operation is rejected (EK4202). Effect 3 code, including `Micro`, runs only as foreign code behind a boundary, as above. Everything else in Effect is available, and Appendix C lists it feature by feature.

#### How do I keep state that several fibers share?

```ts es
function countVisits(paths: ReadonlyArray<string>) {
  const visits = Ref.make(0)
  Effect.forEach(paths, (path) => {
    Visits.record(path)
    Ref.update(visits, (n) => n + 1)
  }, { concurrency: 4 })
  return Ref.get(visits)
}
// Effect<number, VisitError, Visits>
```

Local variables work inside one function, as in any JavaScript. State shared between fibers uses Effect's `Ref`, `SynchronizedRef`, and `SubscriptionRef`, unchanged: `Ref.make(0)` returns an `Effect`, so the call runs it and gives you the `Ref`.
