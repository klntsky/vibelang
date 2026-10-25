# Cheat sheet

Every VibeLang change beside the Effect code it replaces: VibeLang and Effect 4 TypeScript in labeled tabs. The [specification](../../../design/src/000-intro.md) states the rules behind each. In a `.vibe` file, the Effect tab's error classes, services, schemas, Effect types, `Effect.succeed` and `Effect.fail`, catch combinators, and `withSpan` and `provide` calls draw warnings whose quick fixes, and `vibelang --fix`, write the VibeLang tab where it keeps the program's meaning ([TypeScript-style Effect code](../getting-started/012-devtools.md#typescript-style-effect-code)). The examples share these declarations:

```vibe es
// run: Console.log(JSON.stringify([getUser("1"), getOrders("1"), handle(() => release("v1"), Approval, { approve: (_request, k) => resume(k, true) })]))
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

control Approval {
  approve(request: string): boolean!!
}

function release(version: string) {
  return Approval.approve(`deploy ${version}`) ? `released ${version}` : "declined"
}
// Output: [{"id":"1","name":"Ada"},1,"released v1"]
```

## Types

**Effect types** (D127). An empty slot is `never`. Omitted dependencies are inferred from a function's body, and are `never` elsewhere.

```ts effect
type Find = (id: string) => Effect.Effect<User, NotFound>
type Release = (version: string) => Effect.Effect<string, never, Approval>
type Load = () => Effect.Effect<string>
const hello = (id: string): Effect.Effect<string, NotFound> =>
  Effect.map(getUser(id), (user) => `Hello, ${user.name}`)
```

```vibe es
// run: Console.log(hello("1"))
type Find = (id: string) => User!NotFound
type Release = (version: string) => string!!Approval
type Load = () => string!!
function hello(id: string): string!NotFound {
  return `Hello, ${getUser(id).name}`
}
// Output: Hello, Ada
```

## Declarations

**Errors** (D126).

```ts effect
class Missing extends Data.TaggedError("Missing")<{ readonly id: string }> {}
class Timeout extends Data.TaggedError("Timeout") {}
```

```vibe es
error Missing { id: string }
error Timeout
```

**Errors are classes** (D278, D304). Constructors, methods, accessors, statics, `abstract`, `implements` and `extends` keep their TypeScript meaning, and each subclass is tagged with its own name. A family's base declares `_tag: string` so its subclasses' tags fit. Effect has no subclass with its own tag, so the Effect tab writes the tag by hand.

```ts effect
class Invalid extends Data.TaggedError("Invalid")<{ readonly code: string }> {
  constructor(input: string) {
    super({ code: input.trim() })
  }
  get label(): string {
    return `invalid ${this.code}`
  }
}
class Failure extends Data.TaggedError("Failure" as string)<{ readonly code: string }> {}
class Refused extends Failure {
  override readonly _tag = "Refused"
}
```

```vibe es
// run: Console.log(JSON.stringify([new Invalid(" x ").label, new Refused({ code: "r" })._tag]))
error Invalid {
  code: string
  constructor(input: string) {
    super({ code: input.trim() })
  }
  get label(): string {
    return `invalid ${this.code}`
  }
}
error Failure { _tag: string; code: string }
error Refused extends Failure
// Output: ["invalid x","Refused"]
```

**Errors that encode** (D279). Fields map to schemas as a `schema` declaration's do.

```ts effect
class Stored extends Schema.TaggedError<Stored>()("Stored", { key: Schema.String, attempts: Schema.optional(Schema.Number) }) {}
```

```vibe es
schema error Stored { key: string; attempts?: number }
```

**Services** (D128, D223). Members have no bodies; value members are readonly.

```ts effect
class Users extends Context.Service<Users, {
  find(id: string): Effect.Effect<User, NotFound>
  readonly region: string
}>()("app/Users") {}
```

```vibe es
service Users {
  find(id: string): User!NotFound
  region: string
}
```

**Services of any shape** (D281). The shape is any type: an interface, a primitive, a function type. A body is optional and holds only static members.

```ts effect
interface SearchApi {
  find(query: string): Effect.Effect<string>
}
class Search extends Context.Service<Search, SearchApi>()("app/Search") {}
class Prefix extends Context.Service<Prefix, string>()("app/Prefix") {}
```

```vibe es
interface SearchApi {
  find(query: string): string!!
}
service Search: SearchApi
service Prefix: string
```

**Control operations** (D128, D223). Effect has none; the VibeLang long form tab shows the expanded spelling.

```vibe long
abstract class Review extends Control {
  abstract approve(request: string): Effect<boolean, never, never>
}
```

```vibe es
control Review {
  approve(request: string): boolean!!
}
```

**Layers** (D221). A layer is Effect's own call: the block after `Layer.effect(Users)` is its effect (block params, D188), runs once when the layer builds, and returns members typed by the service. Without setup, write `Layer.succeed(Users, { ... })`.

```ts effect
const UsersLive = Layer.effect(Users, Effect.gen(function* () {
  const seen = new Set<string>()
  return Users.of({ find: (id) => getUser(id), region: "eu" })
}))
```

```vibe es
// run: provide UsersLive { Console.log(JSON.stringify(Users.find("1"))); Console.log(Users().region) }
const UsersLive = Layer.effect(Users) {
  const seen = new Set<string>()
  return {
    find(id) { return getUser(id) },
    region: "eu"
  }
}
// Output: {"id":"1","name":"Ada"}
// Output: eu
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

```vibe es
// run: Console.log(JSON.stringify(Schema.decodeUnknownEffect(Account)({ id: "1", role: "admin" })))
schema Account {
  id: string
  nickname?: string
  role: "admin" | "member"
}
// Output: {"id":"1","role":"admin"}
```

**Schema aliases** (D280). A schema of any type the struct form maps, and the type, under one name.

```ts effect
const Role = Schema.Literals(["admin", "member"])
type Role = typeof Role.Type
```

```vibe es
// run: Console.log(Schema.is(Role)("admin")); Console.log(Schema.is(Role)("owner"))
schema Role = "admin" | "member"
// Output: true
// Output: false
```

**Readonly by default** (D134). Write `mutable` for a member you assign.

```ts effect
interface Cart { readonly id: string; items: Array<string> }
```

```vibe es
interface Cart { id: string; mutable items: Array<string> }
```

## Programs

**Statements instead of generators.** Calls run; there is no `yield*`.

```ts effect
const greetUser = Effect.fn("greetUser")(function* (id: string) {
  const user = yield* getUser(id)
  yield* Effect.Console.log(`Hello, ${user.name}`)
  return user.name
})
```

```vibe es
// run: Console.log(Effect.provide(() => greetUser("1"), Logger.layer([sync(() => Logger.make(({ message }) => console.log(...(message as unknown[]))))])))
function greetUser(id: string) {
  return withSpan("greetUser") {
    const user = getUser(id)
    Console.log(`Hello, ${user.name}`)
    return user.name
  }
}
// Output: Hello, Ada
// Output: Ada
```

`Effect.fn("name")(body)` adds a named tracing span; a bare function declaration uses `Effect.fnUntraced` by default and loses it. Keep the span with `function f(...) { return withSpan("name") { ... } }`, or `const f = (...) => withSpan("name") { ... }` for an object field or arrow. Put the function's work inside the `withSpan` block; its return value, failures, and requirements flow through.

**Programs passed by name** (D147). A call runs where it stands, and is passed as a program where a parameter expects one.

```ts effect
const user = yield* getUser("1")
const first = yield* Effect.race(getUser("1"), getUser("2"))
```

```vibe es
// run: Console.log(fastest().name)
function fastest() {
  const user = getUser("1")
  return race(getUser("1"), getUser("2"))
}
// Output: Ada
```

**Providing layers** (D130). `return` inside the block returns from the function.

```ts effect
const program = Effect.gen(function* () {
  const users = yield* Users
  return yield* users.find("1")
}).pipe(Effect.provide(UsersLive))
```

```vibe es
// run: Console.log(JSON.stringify(program()))
function program() {
  provide UsersLive {
    return Users.find("1")
  }
}
// Output: {"id":"1","name":"Ada"}
```

**Providing one service** (D173, D174). `provide Key = value` provides one implementation without a layer; layers and services mix in one list, and an effect module such as `Random` stands for its key, `Random.Random`.

```ts effect
const seeded = { nextIntUnsafe: () => 4, nextDoubleUnsafe: () => 0.5 }
const roll = Random.nextInt.pipe(
  Effect.provideService(Random.Random, seeded),
  Effect.provide(Logger.layer([]))
)
```

```vibe es
// run: Console.log(roll())
const seeded = { nextIntUnsafe: () => 4, nextDoubleUnsafe: () => 0.5 }

function roll(): number!! {
  provide Logger.layer([]), Random = seeded {
    return Random.nextInt()
  }
}
// Output: 4
```

**Catching by tag** (D216). Catch the failure and `match` on it. A match naming every failure needs no rethrow; `default: throw e` passes on the rest.

```ts effect
const describe = (id: string) =>
  Effect.map(getUser(id), (user) => user.name).pipe(
    Effect.catchTag("NotFound", (e) => Effect.succeed(`no user ${e.id}`))
  )
```

```vibe es
// run: Console.log(JSON.stringify([describe("1"), describe("missing")]))
function describe(id: string) {
  try {
    return getUser(id).name
  } catch (e) {
    return match (e) {
      when NotFound: `no user ${e.id}`;
    }
  }
}
// Output: ["Ada","no user missing"]
```

**Falling back in an expression** (D236). `x catch fallback` runs the fallback only after an expected failure; defects and interrupts pass through. Both sides take the expression's contextual type, as a conditional's branches do (D293).

```ts effect
const nameOrGuest = (id: string) =>
  Effect.map(getUser(id), (user) => user.name).pipe(
    Effect.orElseSucceed(() => "guest")
  )
```

```vibe es
// run: Console.log(JSON.stringify([nameOrGuest("1"), nameOrGuest("missing")]))
function nameOrGuest(id: string) {
  return getUser(id).name catch "guest"
}
// Output: ["Ada","guest"]
```

**Catching in an expression** (D236). `x catch (e) => …` passes the failure to a handler, which can `match` on it; a match naming every failure removes them all.

```ts effect
const nameOrReason = (id: string) =>
  Effect.map(getUser(id), (user) => user.name).pipe(
    Effect.catch((e) => Effect.succeed(`no user ${e.id}`))
  )
```

```vibe es
// run: Console.log(JSON.stringify([nameOrReason("1"), nameOrReason("missing")]))
function nameOrReason(id: string) {
  return getUser(id).name catch (e) => match (e) {
    when NotFound: `no user ${e.id}`;
  }
}
// Output: ["Ada","no user missing"]
```

**Handling control operations** (D132). Effect has none; the VibeLang long form tab shows the expanded spelling.

```vibe long
// run: Console.log(auto("v1"))
function auto(version: string) {
  return handle(() => release(version), Approval, {
    approve(_request, continuation) {
      return resume(continuation, true)
    }
  })
}
// Output: released v1
```

```vibe es
// run: Console.log(auto("v1"))
function auto(version: string) {
  try {
    return release(version)
  } handle Approval.approve(request) {
    resume(true)
  }
}
// Output: released v1
```

**Entry point** (D135). An exported `main` runs when Node starts its module, not when a test imports it.

```ts effect
// main.ts
export const main = Effect.log("Hello")
// run.ts
NodeRuntime.runMain(main)
```

```vibe es
// run: Effect.provide(main(), Logger.layer([sync(() => Logger.make(({ message }) => console.log(...(message as unknown[]))))]))
export function main() {
  log("Hello")
}
// Output: Hello
```

## Causes, exits, and interruption

**Catch every cause** (D217). `match` on `exit(…)`: a `Failure` exit's cause also holds defects and interrupts.

```ts effect
const safe = (id: string) =>
  Effect.map(getUser(id), (user) => user.name).pipe(
    Effect.catchCause((cause) => Effect.succeed(`failed: ${Cause.pretty(cause)}`))
  )
```

```vibe es
// run: Console.log(safe("1")); Console.log(safe("missing").includes("NotFound"))
function safe(id: string) {
  return match (exit(getUser(id))) {
    when Exit.Success(let user): user.name;
    when Exit.Failure(let cause): `failed: ${Cause.pretty(cause)}`;
  }
}
// Output: Ada
// Output: true
```

**Rethrow a cause** (D177). `throw` with a `Cause` fails with every reason it holds.

```ts effect
const logged = getUser("1").pipe(
  Effect.catchCause((cause) => Effect.andThen(Effect.logError(cause), Effect.failCause(cause)))
)
```

```vibe es
// run: Console.log(JSON.stringify(logged()))
function logged() {
  const outcome = exit(getUser("1"))
  if (outcome is Exit.Failure(let cause)) {
    logError(cause)
    throw cause
  }
  return outcome.value
}
// Output: {"id":"1","name":"Ada"}
```

**Run on every exit** (D178). The block runs without interruption; a success carries no value.

```ts effect
const tracked = getUser("1").pipe(
  Effect.onExit((exit) => Effect.log(`lookup ${exit._tag}`))
)
```

```vibe es
// run: Console.log(Effect.provide(() => tracked().name, Logger.layer([sync(() => Logger.make(({ message }) => console.log(...(message as unknown[]))))])))
function tracked() {
  try {
    return getUser("1")
  } finally (exit) {
    log(`lookup ${exit._tag}`)
  }
}
// Output: lookup Success
// Output: Ada
```

**Exit, yield, and interrupt** (D179). `exit` takes its program by name.

```ts effect
const result = yield* Effect.exit(getUser("1"))
yield* Effect.yieldNow
if (result._tag === "Failure") yield* Effect.interrupt
```

```vibe es
// run: Console.log(outcome()._tag)
function outcome() {
  const result = exit(getUser("1"))
  yieldNow()
  if (result is Exit.Failure) interrupt()
  return result
}
// Output: Success
```

**Interruption** (D180). In the blocks, `return` and `break` keep their meaning in the function; `uninterruptible(program)` also works.

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

```vibe es
// run: Console.log(JSON.stringify([guarded(), cancellable()]))
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
// Output: [{"user":{"id":"1","name":"Ada"},"orders":1},1]
```

## Calls

A tagged template is a call of its tag (D267). If `sql` returns an Effect, the statement ``sql`CREATE TABLE example (id TEXT)` `` runs it, and ``const rows = sql`SELECT * FROM example` `` gets its success value. Plain tags keep JavaScript behavior. Discarding a structured Effect subtype, such as SQL Statement, is EK4301: call the result explicitly, or annotate the tag as returning `Effect<A, E, R>`. A program parameter takes an Effectful template by name, as it takes a call (D147).

**No imports from `effect`** (D146).

```ts effect
import { Console } from "effect"

const ready = Console.log("ready")
```

```vibe es
// run: ready()
function ready() {
  Console.log("ready")
}
// Output: ready
```

**Common functions without the `Effect.` prefix** (D151): `sleep`, `log`, `retry`, `repeat`, `timeout`, `race`, `all`, `forEach`, `die`, `withSpan`, and the `log` levels.

```ts effect
const patient = Effect.gen(function* () {
  yield* Effect.sleep("10 millis")
  return yield* Effect.timeout(Effect.retry(getUser("1"), Schedule.recurs(2)), "1 second")
})
```

```vibe es
// run: Console.log(JSON.stringify(patient()))
function patient() {
  sleep("10 millis")
  return timeout(retry(getUser("1"), Schedule.recurs(2)), "1 second")
}
// Output: {"id":"1","name":"Ada"}
```

**Methods on Effect's data types** (D152).

```ts effect
const hot = Stream.take(
  Stream.map(Stream.filter(Stream.make(18, 25, 30), (c) => c >= 25), (c) => `${c} C`),
  2
)
```

```vibe es
// run: Console.log(JSON.stringify(Stream.runCollect(hot())))
function hot() {
  return Stream.make(18, 25, 30).filter((c) => c >= 25).map((c) => `${c} C`).take(2)
}
// Output: ["25 C","30 C"]
```

A method is a data-first overload: its first parameter is named `self`, ends in `OrOptions`, or is another supported receiver parameter, such as the one in `Cause.findErrorOption(input)`.

**The pipe operator** (D154). `%` is the value on the left; where it lands in a program parameter, the left side is passed as a program.

```ts effect
const piped = getUser("1").pipe(
  Effect.retry(Schedule.recurs(2)),
  Effect.timeout("1 second")
)
```

```vibe es
// run: Console.log(JSON.stringify(piped()))
function piped() {
  return getUser("1") |> retry(%, Schedule.recurs(2)) |> timeout(%, "1 second")
}
// Output: {"id":"1","name":"Ada"}
```

## TC39 forms

**Throw in an expression** (D186, [throw expressions](https://github.com/tc39/proposal-throw-expressions)).

```ts effect
const findUser = (users: Map<string, User>, id: string) =>
  Effect.fromNullishOr(users.get(id)).pipe(Effect.mapError(() => new NotFound({ id })))
```

```vibe es
// run: Console.log(findUser(new Map([["1", { id: "1", name: "Ada" }]]), "1").name); Console.log(Effect.result(findUser(new Map<string, User>(), "missing"))._tag)
function findUser(users: Map<string, User>, id: string) {
  return users.get(id) ?? throw new NotFound({ id })
}
// Output: Ada
// Output: Failure
```

In expression position, `throw` takes a unary operand (D186), so it binds more tightly than `===`, `??`, and `?:`: `throw failure.code === "unknown" ? a : b` would throw only `failure.code`. EK1010 refuses an unparenthesized throw expression on the left of a binary operator (except comma), as the condition of `?:`, or before `as` or `satisfies`. To throw the selected value, write `throw (failure.code === "unknown" ? a : b)`, or in a catch handler `load() catch (failure) => throw (cond ? a : b)`. Right-hand forms such as `value ?? throw e` and `ok ? value : throw e` stay legal. A **throw statement** still takes the whole expression: `throw cond ? a : b;` at the start of a statement throws the selected value, as in TypeScript.

**Blocks as values** (D187, [do expressions](https://github.com/tc39/proposal-do-expressions)). The value is the last statement's.

```ts effect
const label = (id: string) =>
  Effect.map(getUser(id), (user) => (user.name.length > 3 ? user.name : "short"))
```

```vibe es
// run: Console.log(label("1"))
function label(id: string) {
  const name = do {
    const user = getUser(id)
    if (user.name.length > 3) user.name
    else "short"
  }
  return name
}
// Output: short
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

it.live("finds a user", () =>
  Effect.gen(function* () {
    const user = yield* getUser("1")
    assert.strictEqual(user.name, "Ada")
  }))
```

```vibe es
// run: Console.log(loaded())
function loaded() {
  return withSpan("load") {
    const user = getUser("1")
    return user.name
  }
}

// Output: Ada
```

```vibe
// block-params.test.vibe
import { assert, it } from "@effect/vitest"

it.live("finds a user") {
  const user = getUser("1")
  assert.strictEqual(user.name, "Ada")
}
// Output:
```

`it.live` uses the live clock, because this page's `getUser` sleeps. Under `it.effect`'s test clock, a sleeping test must advance it with `TestClock.adjust`.

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

```vibe es
// run: Console.log(JSON.stringify([phaseLabel("draft"), phaseLabel("published")]))
function phaseLabel(phase: "draft" | "published") {
  return match (phase) {
    when "draft": "Draft";
    when "published": "Published";
  }
}
// Output: ["Draft","Published"]
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

```vibe es
// run: Console.log(JSON.stringify([describeFailure(new NotFound({ id: "7" })), describeFailure(new Timeout())]))
function describeFailure(failure: NotFound | Timeout) {
  return match (failure) {
    when NotFound and { let id }: `no user ${id}`;
    when Timeout: "retry";
  }
}
// Output: ["no user 7","retry"]
```

**Option, Exit, and Result** (D208). Arms can run Effect functions.

```ts effect
const greet = (name: Option.Option<string>) =>
  Option.match(name, { onNone: () => "Hello", onSome: (n) => `Hello, ${n}` })
const failed = (outcome: Exit.Exit<User, NotFound>) => Exit.isFailure(outcome)
```

```vibe es
// run: Console.log(greet(Option.some("Ada"))); Console.log(greet(Option.none())); Console.log(failed(Exit.fail(new NotFound({ id: "7" })))); Console.log(failed(Exit.succeed({ id: "1", name: "Ada" })))
function greet(name: Option.Option<string>) {
  return match (name) {
    when Option.Some(let n): `Hello, ${n}`;
    when Option.None: "Hello";
  }
}

function failed(outcome: Exit.Exit<User, NotFound>) {
  return outcome is Exit.Failure
}
// Output: Hello, Ada
// Output: Hello
// Output: true
// Output: false
```

**Test and bind** (D211). Bindings in an `is` work in an `if` or `while` condition.

```ts effect
const firstFailure = (outcome: Exit.Exit<User, NotFound>) =>
  Exit.isFailure(outcome) && Cause.hasFails(outcome.cause) ? Cause.pretty(outcome.cause) : "ok"
```

```vibe es
// run: Console.log(firstFailure(Exit.succeed({ id: "1", name: "Ada" }))); Console.log(firstFailure(Exit.fail(new NotFound({ id: "7" }))).includes("NotFound"))
function firstFailure(outcome: Exit.Exit<User, NotFound>) {
  if (outcome is Exit.Failure(let cause) && Cause.hasFails(cause)) return Cause.pretty(cause)
  return "ok"
}
// Output: ok
// Output: true
```

## Escape hatches

**Plain, self-contained TypeScript inside `sync`, `try`, and `callback`** (D148, D229-D232). A throw in `sync` is a defect; in `try` it is an expected failure. Async bodies are `Effect.promise` and `Effect.tryPromise`. A hatch's function reads `.vibe` values only as inputs, the arguments after it, which fill its first parameters; `try`'s error mapper stays second (EK4405). `runPromise` leads back in from a hatch's body or a `.ts` file.

**Built-in exceptions (D101).** In an Effect function, `JSON.parse(text)` has a shipped contract: its `SyntaxError` is an expected failure that joins E and reaches `catch`. `decodeURI` and `decodeURIComponent` list `URIError`. Other built-ins' exceptions are defects. In a plain function `JSON.parse` is a JavaScript call, unless a `try` of that function whose `catch` handles the `SyntaxError` encloses it (D282); an exception that escapes into an Effect caller becomes a defect. Inside TypeScript's `Effect.gen`, by contrast, a plain `JSON.parse` throw is a defect.

Use ordinary `catch` to map a built-in’s expected failure; reserve hatches for foreign or platform operations.

```ts effect
const id = Effect.sync(() => crypto.randomUUID())
const parsed = Effect.try({ try: () => JSON.parse(text), catch: (error) => new BadJson({ error }) })
const loaded = Effect.promise(() => crypto.subtle.digest("SHA-256", new Uint8Array()))
const fetched = Effect.tryPromise((signal) => fetch(url, { signal }))
const tick = Effect.callback<number>((resume) => {
  const timer = setTimeout(() => resume(Effect.succeed(7)), 10)
  return Effect.sync(() => clearTimeout(timer))
})
const name = await Effect.runPromise(greetUser("1"))
```

```vibe es
// run: provide Logger.layer([sync(() => Logger.make(({ message }) => console.log(...(message as unknown[]))))]) { const value = hatches('{"ok":true}', "data:text/plain,hello"); Console.log(JSON.stringify({ ...value, id: /^[0-9a-f-]{36}$/.test(value.id) })) }
error BadJson { error: unknown }

function hatches(text: string, url: string) {
  const id = sync(crypto.randomUUID())
  const parsed = JSON.parse(text) catch (error) => throw new BadJson({ error })
  const loaded = sync(async () => {
    const digest = await crypto.subtle.digest("SHA-256", new Uint8Array())
    return digest.byteLength
  })
  const fetched = try(async (url, signal) => await fetch(url, { signal }), url)
  const tick = callback<number>((resume) => {
    const timer = setTimeout(() => resume(7), 10)
    return () => clearTimeout(timer)
  })
  const name = sync(async (greet) => runPromise(greet("1")), greetUser)
  return { id, parsed, loaded, status: fetched.status, tick, name }
}
// Output: Hello, Ada
// Output: {"id":true,"parsed":{"ok":true},"loaded":32,"status":200,"tick":7,"name":"Ada"}
```

**TypeScript modules inline** (D283, [module declarations](https://github.com/tc39/proposal-module-declarations)). A module's body is TypeScript, compiled as a `.ts` file would be, so JavaScript's `throw` and `catch` and `async` keep their meaning there. It is self-contained: it imports what it needs, and the file reaches it only through imports.

```ts effect
// platform.ts
export function randomId(): string {
  return crypto.randomUUID()
}
// main.ts
import { randomId } from "./platform.ts"
const requestId = Effect.sync(() => randomId())
```

```vibe es
// run: Console.log(/^[-0-9a-f]{36}$/.test(requestId()))
module platform {
  export function randomId(): string {
    return crypto.randomUUID()
  }
}

import { randomId } from platform

function requestId() {
  return sync(() => randomId())
}
// Output: true
```

## Fibers

**Fork and join** (D166, D225). A fiber belongs to the block that forks it. `join [a, b]` joins several and interrupts the rest on a failure.

```ts effect
const user = yield* Effect.forkChild(getUser("1"))
const orders = yield* Effect.forkChild(getOrders("1"))
return [yield* Fiber.join(user), yield* Fiber.join(orders)]
```

```vibe es
// run: Console.log(JSON.stringify(both()))
function both() {
  const user = fork getUser("1")
  const orders = fork getOrders("1")
  return [join user, join orders]
}
// Output: [{"id":"1","name":"Ada"},1]
```

**Wait for several** (D196). `await.all` behaves like `Promise.all`: it starts each element in its own fiber, keeps the shape, and a failure interrupts the rest.

```ts effect
const [user, orders] = yield* Effect.all([getUser("1"), getOrders("1")], { concurrency: "unbounded" })
const pair = yield* Effect.all({ user: getUser("1"), orders: getOrders("1") }, { concurrency: "unbounded" })
```

```vibe es
// run: Console.log(JSON.stringify(many()))
function many() {
  const [user, orders] = await.all [getUser("1"), getOrders("1")]
  const pair = await.all { user: getUser("1"), orders: getOrders("1") }
  return { user, orders, pair }
}
// Output: {"user":{"id":"1","name":"Ada"},"orders":1,"pair":{"user":{"id":"1","name":"Ada"},"orders":1}}
```

**First success, expected outcomes** (D196, D258). `await.any` waits for the first success. `await.allSettled` returns successes and typed failures as `Result` values, not JavaScript's settlement objects, and never fails with a typed error. Defects and interruption propagate exactly as in `Effect.all` with `mode: "result"`; a cause with no typed failure still propagates. To make every cause data, wrap each element in `exit(...)`.

```ts effect
const fastest = yield* Effect.raceAll([getUser("1"), getUser("2")])
const outcomes = yield* Effect.all([getUser("1"), getOrders("1")], { concurrency: "unbounded", mode: "result" })
```

```vibe es
// run: const value = outcomes(); Console.log(value.fastest.name); Console.log(JSON.stringify(value.results.map((result) => result._tag))); Console.log(JSON.stringify((await.allSettled [getUser("missing"), getOrders("1")]).map((result) => result._tag))); Console.log(Exit.isFailure(exit(await.allSettled [die("defect"), getOrders("1")])))
function outcomes() {
  const fastest = await.any [getUser("1"), getUser("2")]
  const results = await.allSettled [getUser("1"), getOrders("1")]
  return { fastest, results }
}
// Output: Ada
// Output: ["Success","Success"]
// Output: ["Failure","Success"]
// Output: true
```

**Fork a block** (D166). The loop stops when the enclosing block exits.

```ts effect
const withHeartbeat = Effect.scoped(Effect.gen(function* () {
  yield* Effect.forkScoped(Effect.forever(Effect.andThen(Effect.log("tick"), Effect.sleep("1 second"))))
  return yield* getUser("1")
}))
```

```vibe es
// run: provide Logger.layer([]) { Console.log(withHeartbeat().name) }
function withHeartbeat() {
  fork {
    while (true) {
      log("tick")
      sleep("1 second")
    }
  }
  return getUser("1")
}
// Output: Ada
```

**Detached and scoped fibers** (D170).

```ts effect
yield* Effect.forkDetach(getUser("1"))
const scope = yield* Scope.make()
const fiber = yield* Effect.forkIn(getUser("2"), scope)
```

```vibe es
// run: Console.log(Exit.isFailure(Fiber.await(background())))
function background() {
  detach(getUser("1"))
  const scope = Scope.make()
  const fiber = forkIn(scope, getUser("2"))
  Scope.close(scope, Exit.void)
  return fiber
}
// Output: true
```

**Wait for a Deferred** (D227). `await` waits for promises in JavaScript, so a Deferred uses its method.

```ts effect
const ready = yield* Deferred.make<string>()
yield* Deferred.succeed(ready, "go")
const value = yield* Deferred.await(ready)
```

```vibe es
// run: Console.log(signal())
function signal() {
  const ready = Deferred.make<string>()
  Deferred.succeed(ready, "go")
  return ready.await()
}
// Output: go
```

**Race** (D196, D197). `await.race` behaves like `Promise.race`: the first to finish wins, success or failure, and the rest are interrupted. It replaced `select` (EK4607); poll with `Queue.poll` and an `if`.

```ts effect
const next = yield* Effect.raceFirst(
  Effect.map(Queue.take(jobs), (job) => `ran ${job}`),
  Effect.as(Effect.sleep("1 second"), "timeout")
)
```

```vibe es
// run: Console.log(next())
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
// Output: ran build
```

**A concurrent loop** (D169). At most `n` items run at once, and all finish before the loop ends.

```ts effect
yield* Effect.forEach(ids, (id) => Effect.flatMap(getUser(id), (user) => Effect.log(user.name)), {
  concurrency: 8,
  discard: true
})
```

```vibe es
// run: Effect.provide(logAll(["1", "2"]), Logger.layer([sync(() => Logger.make(({ message }) => console.log(...(message as unknown[]))))]))
function logAll(ids: string[]) {
  for concurrent(8) (const id of ids) log(getUser(id).name)
}
// Output: Ada
// Output: Ada
```

