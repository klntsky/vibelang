# Effeck for Effect users

Keep Effect's runtime, libraries, layers, and error channels. In `.effeck` files, write the sequential work as ordinary functions and statements.

```ts
function greet(name: string) {
  sleep("10 millis")
  const message = `Hello, ${name}`
  Console.log(message)
  return message
}

function main() {
  return retry(greet("Ada"), Schedule.recurs(2))
}
```

Where it stands, `greet("Ada")` runs and returns a string. Passed to `Effect.retry`, whose parameter expects a program, the same call is passed by name: retry runs it again on each attempt. Write neither `async` nor `await` outside a boundary hatch.

## Translate the idioms

| In Effect TypeScript | In Effeck |
| --- | --- |
| `Effect.gen(function* () { ... })` | An ordinary function with statements |
| `class NotFound extends Data.TaggedError("NotFound")<{ readonly id: string }> {}` | `error NotFound { id: string }` |
| `const x = yield* work()` | `const x = work()` |
| `pipe(x, Effect.map(f), Effect.flatMap(g))` | Bind the result, transform it, then call the next function |
| `Option.getOrElse(Option.map(maybe, f), () => 0)` | `maybe.map(f).getOrElse(() => 0)` |
| `pipe(stream, Stream.filter(p), Stream.take(3))` | `stream.filter(p).take(3)` |
| `Context.Service` | `service Users { find(id: string): User!NotFound }`; call `Users.find(id)` |
| `Layer.effect(Users, Effect.gen(function* () { ... }))` | `layer UsersLive: Users { setup; find(id) { ... } }` |
| `Effect.provide(program, AppLive)` | `with AppLive { ... }` |
| `Effect.catchTag(program, "Missing", recover)` | `try { ... } catch (e: Missing) { ... }` |
| `Effect.ensuring(program, cleanup)` | `try` / `finally` |
| `Effect.scoped` with `Effect.acquireRelease` | Acquire with `using` in the block that owns the resource |
| `Effect.forkScoped(program)` | `fork(work())` for a child owned by the enclosing block |
| `Effect.sync(() => crypto.randomUUID())` | `sync(crypto.randomUUID())` |
| `Effect.try({ try: () => parse(text), catch: (e) => new BadInput({ e }) })` | `try(parse(text), (e) => new BadInput({ e }))` |
| `Effect.tryPromise((signal) => fetchJson(url, signal))` | `try(async (signal) => fetchJson(url, signal))` |
| `Effect.promise(() => load())` | `sync(async () => load())` |
| `Effect.callback((resume) => …resume(Effect.succeed(x))…)` | `callback((resume) => …resume(x)…)` |
| `Effect.runPromise(program)` in TypeScript | `runPromise(main())` in a `.ts` file or a hatch's body |
| `Stream.fromIterable` or a generator-based source | A producer: `function*` with `yield value` |

Use Effect's operators for retries, timeouts, concurrency, and Stream transformations. Statements replace the operators that only sequence ordinary work.

## Calls run; programs stay cold

```ts
function sequential() {
  const first = greet("Ada")
  const second = greet("Grace")
  return [first, second]
}

function concurrent() {
  return all([greet("Ada"), greet("Grace")], { concurrency: 2 })
}
```

A call whose result type is Effect runs at that point, except where a parameter expects a program: there the call is passed by name, and runs when the program runs. This applies to `Effect.retry`, `Effect.timeout`, `fork`, and entries in `Effect.all`. A program of several statements is a function with no arguments, `() => { ... }`.

A held Effect value runs when called with no arguments. Data that Effect can also execute, such as `Config` and `Exit`, stays data until explicitly called. A producer call constructs a lazy Stream; see [Producers](112-producers.md).

## Declare at the module top level

```ts
// run.effeck
import { NodeRuntime } from "@effect/platform-node"
function start() {
  Console.log("started")
}

NodeRuntime.runMain(() => start())
```

Declare functions, services, schemas, and layers at module scope. Do not run a program there: even `Effect.succeed(1)` is EK1001 at the top level. A runner is the execution boundary; call it where no fiber is already running.

`Effect`, `Schema`, `Layer`, `Scope`, `TaggedError`, `Control`, `pipe`, `wire`, `fork`, `handle`, `resume`, `resumeWith`, and `abandon` need no imports. Import other APIs from their Effect packages.

## Keep platform work in Effect

```ts
function reportTime() {
  const now = Clock.currentTimeMillis()
  Console.log(now)
  return now
}
```

`console.log`, `fetch`, timers, and `Date.now()` are errors in `.effeck`. Use `Console`, `Clock`, `Effect.sleep`, and Effect's HTTP modules. Importing a Node function or passing it as a value is allowed; directly running it is a platform API error. [Building an HTTP API](113-building-an-http-api.md) shows `createServer` passed to a server layer.

A plain function from your `.ts` code or a foreign package needs a boundary hatch, `try(parse(text))` or `sync(crypto.randomUUID())`, or a trusted declaration. A hatch's argument is plain TypeScript: `async`, `await`, `throw`, and platform APIs keep their JavaScript meaning there, and `runPromise` or `runFork` leads back in. Effect's own packages are trusted, including `assert` and `expect` re-exported by `@effect/vitest`.

## Catch expected failures only

```ts
error Missing { id: string }

function lookup(id: string) {
  if (id !== "ada") throw new Missing({ id })
  return { id, name: "Ada" }
}

function displayName(id: string) {
  try {
    return lookup(id).name
  } catch (error) {
    return `Unknown user: ${error.id}`
  }
}
```

`throw` adds an expected failure to the function's error channel, and `catch` receives those failures. Defects and interruption bypass `catch`; `finally` still runs. Use `Effect.orDie(() => work())` to promote an expected failure to a defect, and Effect's cause APIs at a boundary when you need to inspect defects.

Contracts remain `Effect<A, E, R>`. Most functions infer them. A return contract with a body must spell all three channels; `infer E` or `infer R` asks the compiler to infer that channel.

For the feature with no direct Effect equivalent, see [Control operations and handlers](111-control-operations-and-handlers.md).
