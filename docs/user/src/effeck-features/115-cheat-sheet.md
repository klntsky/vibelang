# Cheat sheet

Every Effeck change next to the Effect code it replaces. The left column is Effect 4 TypeScript, and the right column is Effeck. The examples share these declarations:

```ts es
error NotFound { id: string }

interface User { id: string; name: string }

function getUser(id: string): User!NotFound {
  sleep("5 millis")
  if (id === "missing") throw new NotFound({ id })
  return { id, name: "Ada" }
}

function getOrders(id: string): number!! {
  return id.length
}

control class Approval {
  approve(request: string): boolean!!
}

function release(version: string) {
  return Approval.approve(`deploy ${version}`) ? `released ${version}` : "declined"
}
```

## Types

**Effect types** (D127). An empty slot is `never`. Dependencies left out are inferred from a function's body, and are `never` anywhere else.

```ts effect
type Find = (id: string) => Effect.Effect<User, NotFound>
type Release = (version: string) => Effect.Effect<string, never, Approval>
type Load = () => Effect.Effect<string>
const hello = (id: string): Effect.Effect<string, NotFound> =>
  Effect.map(getUser(id), (user) => `Hello, ${user.name}`)
```

```ts es
type Find = (id: string) => User!NotFound
type Release = (version: string) => string!!Approval
type Load = () => string!!
function hello(id: string): string!NotFound {
  return `Hello, ${getUser(id).name}`
}
```

## Declarations

**Errors** (D126).

```ts effect
class Missing extends Data.TaggedError("Missing")<{ readonly id: string }> {}
class Timeout extends Data.TaggedError("Timeout") {}
```

```ts es
error Missing { id: string }
error Timeout
```

**Services** (D128, D220). `service` modifies a class declaration, as `declare` does; members have no bodies, and value members are readonly.

```ts effect
class Users extends Context.Service<Users, {
  find(id: string): Effect.Effect<User, NotFound>
  readonly region: string
}>()("app/Users") {}
```

```ts es
service class Users {
  find(id: string): User!NotFound
  region: string
}
```

**Control operations** (D128, D220). Effect has none, so the left column is Effeck's long form.

```ts long
abstract class Review extends Control {
  abstract approve(request: string): Effect<boolean, never, never>
}
```

```ts es
control class Review {
  approve(request: string): boolean!!
}
```

**Layers** (D221). A layer is Effect's own call: the block after `Layer.effect(Users)` is its effect (block params, D188), its statements run once when the layer builds, and the service types the returned members. Without setup, write `Layer.succeed(Users, { ... })`.

```ts effect
const UsersLive = Layer.effect(Users, Effect.gen(function* () {
  const seen = new Set<string>()
  return Users.of({ find: (id) => getUser(id), region: "eu" })
}))
```

```ts es
const UsersLive = Layer.effect(Users) {
  const seen = new Set<string>()
  return {
    find(id) { return getUser(id) },
    region: "eu"
  }
}
```

**Schemas** (D133). One declaration gives the schema and its type.

```ts effect
const Account = Schema.Struct({
  id: Schema.String,
  nickname: Schema.optional(Schema.String),
  role: Schema.Literals(["admin", "member"])
})
interface Account extends Schema.Schema.Type<typeof Account> {}
```

```ts es
schema Account {
  id: string
  nickname?: string
  role: "admin" | "member"
}
```

**Readonly by default** (D134). Write `mutable` for a member you assign.

```ts effect
interface Cart { readonly id: string; items: Array<string> }
```

```ts es
interface Cart { id: string; mutable items: Array<string> }
```

## Programs

**Statements instead of generators.** Calls run; there is no `yield*`.

```ts effect
const greetUser = Effect.fn("greetUser")(function* (id: string) {
  const user = yield* getUser(id)
  yield* Effect.log(`Hello, ${user.name}`)
  return user.name
})
```

```ts es
function greetUser(id: string) {
  const user = getUser(id)
  log(`Hello, ${user.name}`)
  return user.name
}
```

**Programs passed by name** (D147). A call runs where it stands, and is passed as a program where a parameter expects one.

```ts effect
const user = yield* getUser("1")
const first = yield* Effect.race(getUser("1"), getUser("2"))
```

```ts es
function fastest() {
  const user = getUser("1")
  return race(getUser("1"), getUser("2"))
}
```

**Providing layers** (D130). `return` inside the block returns from the function.

```ts effect
const program = Effect.gen(function* () {
  const users = yield* Users
  return yield* users.find("1")
}).pipe(Effect.provide(UsersLive))
```

```ts es
function program() {
  provide UsersLive {
    return Users.find("1")
  }
}
```

**Providing one service** (D173, D174). `provide Key = value` provides one implementation without a layer; layers and services mix in one list, and an effect module such as `Random` stands for its key, `Random.Random`.

```ts effect
const seeded = { nextIntUnsafe: () => 4, nextDoubleUnsafe: () => 0.5 }
const roll = Random.nextInt.pipe(
  Effect.provideService(Random.Random, seeded),
  Effect.provide(Logger.layer([]))
)
```

```ts es
const seeded = { nextIntUnsafe: () => 4, nextDoubleUnsafe: () => 0.5 }

function roll(): number!! {
  provide Logger.layer([]), Random = seeded {
    return Random.nextInt()
  }
}
```

**Catching by tag** (D131). A failure no clause names keeps propagating.

```ts effect
const describe = (id: string) =>
  Effect.map(getUser(id), (user) => user.name).pipe(
    Effect.catchTag("NotFound", (e) => Effect.succeed(`no user ${e.id}`))
  )
```

```ts es
function describe(id: string) {
  try {
    return getUser(id).name
  } catch (e: NotFound) {
    return `no user ${e.id}`
  }
}
```

**Handling control operations** (D132). Effect has none, so the left column is Effeck's long form.

```ts long
function auto(version: string) {
  return handle(() => release(version), Approval, {
    approve(_request, continuation) {
      return resume(continuation, true)
    }
  })
}
```

```ts es
function auto(version: string) {
  try {
    return release(version)
  } handle Approval.approve(request) {
    resume(true)
  }
}
```

**Entry point** (D135). An exported `main` runs when Node starts its module, and not when a test imports it.

```ts effect
// main.ts
export const main = Effect.log("Hello")
// run.ts
NodeRuntime.runMain(main)
```

```ts es
export function main() {
  log("Hello")
}
```

## Causes, exits, and interruption

**Catch every cause** (D176). A clause typed `Cause` also sees defects and interrupts; clauses before it still see expected failures only.

```ts effect
const safe = (id: string) =>
  Effect.map(getUser(id), (user) => user.name).pipe(
    Effect.catchTag("NotFound", (e) => Effect.succeed(`no user ${e.id}`)),
    Effect.catchCause((cause) => Effect.succeed(`failed: ${Cause.pretty(cause)}`))
  )
```

```ts es
function safe(id: string) {
  try {
    return getUser(id).name
  } catch (e: NotFound) {
    return `no user ${e.id}`
  } catch (cause: Cause) {
    return `failed: ${Cause.pretty(cause)}`
  }
}
```

**Rethrow a cause** (D177). `throw` with a `Cause` fails with every reason it holds.

```ts effect
const logged = getUser("1").pipe(
  Effect.catchCause((cause) => Effect.andThen(Effect.logError(cause), Effect.failCause(cause)))
)
```

```ts es
function logged() {
  try {
    return getUser("1")
  } catch (cause: Cause) {
    logError(cause)
    throw cause
  }
}
```

**Run on every exit** (D178). The block runs without interruption; a success carries no value.

```ts effect
const tracked = getUser("1").pipe(
  Effect.onExit((exit) => Effect.log(`lookup ${exit._tag}`))
)
```

```ts es
function tracked() {
  try {
    return getUser("1")
  } finally (exit) {
    log(`lookup ${exit._tag}`)
  }
}
```

**Exit, yield, and interrupt** (D179). `exit` takes its program by name.

```ts effect
const result = yield* Effect.exit(getUser("1"))
yield* Effect.yieldNow
if (result._tag === "Failure") yield* Effect.interrupt
```

```ts es
function outcome() {
  const result = exit(getUser("1"))
  yieldNow()
  if (result is Exit.Failure) interrupt()
  return result
}
```

**Interruption** (D180). The blocks keep `return` and `break` meaning what they mean in the function; `uninterruptible(program)` also works.

```ts effect
const guarded = Effect.uninterruptibleMask((restore) =>
  Effect.gen(function* () {
    const user = yield* getUser("1")
    const orders = yield* restore(getOrders(user.id))
    return { user, orders }
  })
)
const cancellable = Effect.interruptible(getOrders("1"))
```

```ts es
function guarded() {
  uninterruptible {
    const user = getUser("1")
    restore {
      return { user, orders: getOrders(user.id) }
    }
  }
}

function cancellable() {
  interruptible {
    return getOrders("1")
  }
}
```

## Calls

**No imports from `effect`** (D146).

```ts effect
import { Console } from "effect"

const ready = Console.log("ready")
```

```ts es
function ready() {
  Console.log("ready")
}
```

**Common functions without the `Effect.` prefix** (D151): `sleep`, `log`, `retry`, `repeat`, `timeout`, `race`, `all`, `forEach`, `die`, `withSpan`, and the `log` levels.

```ts effect
const patient = Effect.gen(function* () {
  yield* Effect.sleep("10 millis")
  return yield* Effect.timeout(Effect.retry(getUser("1"), Schedule.recurs(2)), "1 second")
})
```

```ts es
function patient() {
  sleep("10 millis")
  return timeout(retry(getUser("1"), Schedule.recurs(2)), "1 second")
}
```

**Methods on Effect's data types** (D152).

```ts effect
const hot = Stream.take(
  Stream.map(Stream.filter(Stream.make(18, 25, 30), (c) => c >= 25), (c) => `${c} C`),
  2
)
```

```ts es
function hot() {
  return Stream.make(18, 25, 30).filter((c) => c >= 25).map((c) => `${c} C`).take(2)
}
```

**The pipe operator** (D154). `%` is the value on the left; where it lands in a program parameter, the left side is passed as a program.

```ts effect
const piped = getUser("1").pipe(
  Effect.retry(Schedule.recurs(2)),
  Effect.timeout("1 second")
)
```

```ts es
function piped() {
  return getUser("1") |> retry(%, Schedule.recurs(2)) |> timeout(%, "1 second")
}
```

## TC39 forms

**Throw in an expression** (D186, [throw expressions](https://github.com/tc39/proposal-throw-expressions)).

```ts effect
const findUser = (users: Map<string, User>, id: string) =>
  Effect.fromNullishOr(users.get(id)).pipe(Effect.mapError(() => new NotFound({ id })))
```

```ts es
function findUser(users: Map<string, User>, id: string) {
  return users.get(id) ?? throw new NotFound({ id })
}
```

**Blocks as values** (D187, [do expressions](https://github.com/tc39/proposal-do-expressions)). The value is the last statement's.

```ts effect
const label = (id: string) =>
  Effect.map(getUser(id), (user) => (user.name.length > 3 ? user.name : "short"))
```

```ts es
function label(id: string) {
  const name = do {
    const user = getUser(id)
    if (user.name.length > 3) user.name
    else "short"
  }
  return name
}
```

**Blocks after calls** (D188, [block params](https://github.com/samuelgoto/proposal-block-params)). The block is the call's program; it needs `return` for a value.

```ts effect
const loaded = Effect.withSpan(
  Effect.gen(function* () {
    const user = yield* getUser("1")
    return user.name
  }),
  "load"
)

it.effect("finds a user", () =>
  Effect.gen(function* () {
    const user = yield* getUser("1")
    assert.strictEqual(user.name, "Ada")
  }))
```

```ts es
import { assert, it } from "@effect/vitest"

function loaded() {
  return withSpan("load") {
    const user = getUser("1")
    return user.name
  }
}

it.effect("finds a user") {
  const user = getUser("1")
  assert.strictEqual(user.name, "Ada")
}
```

## Pattern matching

From the TC39 [pattern matching](https://github.com/tc39/proposal-pattern-matching) proposal (stage 1). See [Pattern matching with match and is](116-match-and-is.md).

**Match a value** (D206, D209). Without `default`, a missing case of a finite union is EK4701, as with `Match.exhaustive`.

```ts effect
const phaseLabel = (phase: "draft" | "published") =>
  Match.value(phase).pipe(
    Match.when("draft", () => "Draft"),
    Match.when("published", () => "Published"),
    Match.exhaustive
  )
```

```ts es
function phaseLabel(phase: "draft" | "published") {
  return match (phase) {
    when "draft": "Draft";
    when "published": "Published";
  }
}
```

**Match errors by class** (D215). An `error` class is a pattern; `and { let id }` binds a property.

```ts effect
const describeFailure = (failure: NotFound | Timeout) =>
  Match.value(failure).pipe(
    Match.tag("NotFound", (e) => `no user ${e.id}`),
    Match.tag("Timeout", () => "retry"),
    Match.exhaustive
  )
```

```ts es
function describeFailure(failure: NotFound | Timeout) {
  return match (failure) {
    when NotFound and { let id }: `no user ${id}`;
    when Timeout: "retry";
  }
}
```

**Option, Exit, and Result** (D208). The arms can run Effect functions.

```ts effect
const greet = (name: Option.Option<string>) =>
  Option.match(name, { onNone: () => "Hello", onSome: (n) => `Hello, ${n}` })
const failed = (outcome: Exit.Exit<User, NotFound>) => Exit.isFailure(outcome)
```

```ts es
function greet(name: Option.Option<string>) {
  return match (name) {
    when Option.Some(let n): `Hello, ${n}`;
    when Option.None: "Hello";
  }
}

function failed(outcome: Exit.Exit<User, NotFound>) {
  return outcome is Exit.Failure
}
```

**Test and bind** (D211). Bindings in an `is` work in an `if` or `while` condition.

```ts effect
const firstFailure = (outcome: Exit.Exit<User, NotFound>) =>
  Exit.isFailure(outcome) && Cause.hasFails(outcome.cause) ? Cause.pretty(outcome.cause) : "ok"
```

```ts es
function firstFailure(outcome: Exit.Exit<User, NotFound>) {
  if (outcome is Exit.Failure(let cause) && Cause.hasFails(cause)) return Cause.pretty(cause)
  return "ok"
}
```

## Escape hatches

**Plain TypeScript inside `sync`, `try`, and `callback`** (D148). A throw in `sync` is a defect; in `try` it is an expected failure. Async bodies are `Effect.promise` and `Effect.tryPromise`. `runPromise` leads back in from a hatch's body or a `.ts` file.

```ts effect
const id = Effect.sync(() => crypto.randomUUID())
const parsed = Effect.try({ try: () => JSON.parse(text), catch: (error) => new BadJson({ error }) })
const loaded = Effect.promise(() => Promise.resolve(42))
const fetched = Effect.tryPromise((signal) => fetch(url, { signal }))
const tick = Effect.callback<number>((resume) => {
  const timer = setTimeout(() => resume(Effect.succeed(7)), 10)
  return Effect.sync(() => clearTimeout(timer))
})
const name = await Effect.runPromise(greetUser("1"))
```

```ts es
error BadJson { error: unknown }

function hatches(text: string, url: string) {
  const id = sync(crypto.randomUUID())
  const parsed = try(() => JSON.parse(text), (error) => new BadJson({ error }))
  const loaded = sync(async () => Promise.resolve(42))
  const fetched = try(async (signal) => fetch(url, { signal }))
  const tick = callback<number>((resume) => {
    const timer = setTimeout(() => resume(7), 10)
    return () => clearTimeout(timer)
  })
  const name = sync(async () => runPromise(greetUser("1")))
  return { id, parsed, loaded, status: fetched.status, tick, name }
}
```

## Fibers

**Fork and join** (D166, D225). A fiber belongs to the block that forks it. `join [a, b]` joins several and interrupts the rest on a failure.

```ts effect
const user = yield* Effect.forkChild(getUser("1"))
const orders = yield* Effect.forkChild(getOrders("1"))
return [yield* Fiber.join(user), yield* Fiber.join(orders)]
```

```ts es
function both() {
  const user = fork getUser("1")
  const orders = fork getOrders("1")
  return [join user, join orders]
}
```

**Wait for several** (D196). `await.all` behaves like `Promise.all`: it starts each element in its own fiber, keeps the shape, and a failure interrupts the rest.

```ts effect
const [user, orders] = yield* Effect.all([getUser("1"), getOrders("1")], { concurrency: "unbounded" })
const pair = yield* Effect.all({ user: getUser("1"), orders: getOrders("1") }, { concurrency: "unbounded" })
```

```ts es
function many() {
  const [user, orders] = await.all [getUser("1"), getOrders("1")]
  const pair = await.all { user: getUser("1"), orders: getOrders("1") }
  return { user, orders, pair }
}
```

**First success, every outcome** (D196). `await.any` behaves like `Promise.any` and `await.allSettled` like `Promise.allSettled`.

```ts effect
const fastest = yield* Effect.raceAll([getUser("1"), getUser("2")])
const outcomes = yield* Effect.all([getUser("1"), getOrders("1")], { concurrency: "unbounded", mode: "result" })
```

```ts es
function outcomes() {
  const fastest = await.any [getUser("1"), getUser("2")]
  const results = await.allSettled [getUser("1"), getOrders("1")]
  return { fastest, results }
}
```

**Fork a block** (D166). The loop stops when the enclosing block exits.

```ts effect
const withHeartbeat = Effect.scoped(Effect.gen(function* () {
  yield* Effect.forkScoped(Effect.forever(Effect.andThen(Effect.log("tick"), Effect.sleep("1 second"))))
  return yield* getUser("1")
}))
```

```ts es
function withHeartbeat() {
  fork {
    while (true) {
      log("tick")
      sleep("1 second")
    }
  }
  return getUser("1")
}
```

**Detached and scoped fibers** (D170).

```ts effect
yield* Effect.forkDetach(getUser("1"))
const scope = yield* Scope.make()
const fiber = yield* Effect.forkIn(getUser("2"), scope)
```

```ts es
function background() {
  detach(getUser("1"))
  const scope = Scope.make()
  const fiber = forkIn(scope, getUser("2"))
  Scope.close(scope, Exit.void)
  return fiber
}
```

**Wait for a Deferred** (D227). `await` waits for promises in JavaScript, so a Deferred uses its method.

```ts effect
const ready = yield* Deferred.make<string>()
yield* Deferred.succeed(ready, "go")
const value = yield* Deferred.await(ready)
```

```ts es
function signal() {
  const ready = Deferred.make<string>()
  Deferred.succeed(ready, "go")
  return ready.await()
}
```

**Race** (D196, D197). `await.race` behaves like `Promise.race`: the first to finish wins, success or failure, and the rest are interrupted. It replaced `select` (EK4607); poll with `Queue.poll` and an `if`.

```ts effect
const next = yield* Effect.raceFirst(
  Effect.map(Queue.take(jobs), (job) => `ran ${job}`),
  Effect.as(Effect.sleep("1 second"), "timeout")
)
```

```ts es
function next() {
  const jobs = Queue.unbounded<string>()
  Queue.offer(jobs, "build")
  return await.race [
    `ran ${Queue.take(jobs)}`,
    do {
      sleep("1 second")
      "timeout"
    }
  ]
}
```

**A concurrent loop** (D169). At most `n` items run at once, and all finish before the loop ends.

```ts effect
yield* Effect.forEach(ids, (id) => Effect.flatMap(getUser(id), (user) => Effect.log(user.name)), {
  concurrency: 8,
  discard: true
})
```

```ts es
function logAll(ids: string[]) {
  for concurrent(8) (const id of ids) log(getUser(id).name)
}
```
