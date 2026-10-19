# VibeLang for Effect users

Keep Effect's runtime, libraries, layers, and error channels. In `.vibe` files, write the sequential work as ordinary functions and statements. For every change side by side with Effect code, see the [cheat sheet](115-cheat-sheet.md). For the rules behind each change, read the [specification](../../../design/src/000-intro.md).

```vibe
// run: Console.log(main())
function greet(name: string) {
  sleep("10 millis")
  const message = `Hello, ${name}`
  Console.log(message)
  return message
}

function main() {
  return retry(greet("Ada"), Schedule.recurs(2))
}
// Output: Hello, Ada
// Output: Hello, Ada
```

Where it stands, `greet("Ada")` runs and returns a string. Passed to `Effect.retry`, whose parameter expects a program, the same call is passed by name: retry runs it again on each attempt. Use `async` and plain `await` only inside a boundary hatch. The concurrent `await.all`, `await.allSettled`, `await.any`, and `await.race` operations are VibeLang forms.

## Translate the idioms

| In Effect TypeScript | In VibeLang |
| --- | --- |
| `Effect.gen(function* () { ... })` | An ordinary function with statements |
| `class NotFound extends Data.TaggedError("NotFound")<{ readonly id: string }> {}` | `error NotFound { id: string }` |
| `const User = Schema.Struct({ id: Schema.String })` and `type User = typeof User.Type` | `schema User { id: string }` |
| `interface Cart { readonly id: string; items: Array<Item> }` | `interface Cart { id: string; mutable items: Array<Item> }` |
| `const x = yield* work()` | `const x = work()` |
| `pipe(x, Effect.map(f), Effect.flatMap(g))` | Bind the result, transform it, then call the next function |
| `Option.getOrElse(Option.map(maybe, f), () => 0)` | `maybe.map(f).getOrElse(() => 0)` |
| `pipe(stream, Stream.filter(p), Stream.take(3))` | `stream.filter(p).take(3)` |
| `work().pipe(Effect.retry(policy), Effect.timeout("1 second"))` | `work() \|> retry(%, policy) \|> timeout(%, "1 second")` |
| `Context.Service` | `service Users { find(id: string): User!NotFound }`; call `Users.find(id)` |
| `Layer.effect(Users, Effect.gen(function* () { ... }))` | `Layer.effect(Users) { setup; return { find(id) { ... } } }` |
| `Effect.provide(program, AppLive)` | `provide AppLive { ... }` |
| `Effect.provideService(program, Random.Random, seeded)` | `provide Random = seeded { ... }` |
| `Effect.catchTag(program, "Missing", recover)` | `try { ... } catch (e) { return match (e) { when Missing: ...; default: throw e; } }` |
| `Effect.catch(program, (e) => recover(e))` | `program catch (e) => recover(e)` |
| `Effect.orElseSucceed(program, () => value)` | `program catch value` |
| `Effect.ensuring(program, cleanup)` | `try` / `finally` |
| `Effect.scoped` with `Effect.acquireRelease` | Acquire with `using` in the block that owns the resource |
| `Effect.forkScoped(program)` | `fork work()` for a child owned by the enclosing block; `fork { ... }` forks a block |
| `Fiber.join(fiber)` | `join fiber`; `join [a, b]` joins several and interrupts the rest on a failure |
| `Effect.all([a, b], { concurrency: "unbounded" })` | `await.all [a(), b()]`, like `Promise.all`; `await.all { x: a(), y: b() }` keeps the keys |
| `Effect.all(effects, { concurrency: "unbounded", mode: "result" })` | `await.allSettled [a(), b()]`, with a `Result` for each success or expected failure |
| `Effect.raceFirst(a, b)` | `await.race [a(), b()]`, like `Promise.race`: the first to finish wins |
| `Effect.raceAll([a, b])` | `await.any [a(), b()]`, like `Promise.any`: the first success wins |
| `Effect.forEach(items, f, { concurrency: 8, discard: true })` | `for concurrent(8) (const item of items) f(item)` |
| `Effect.sync(() => crypto.randomUUID())` | `sync(crypto.randomUUID())` |
| `Effect.try({ try: () => parse(text), catch: (e) => new BadInput({ e }) })` | `try((text) => parse(text), (e) => new BadInput({ e }), text)` |
| `Effect.tryPromise((signal) => fetchJson(url, signal))` | `try(async (url, signal) => fetchJson(url, signal), url)` |
| `Effect.promise(() => load())` | `sync(async () => load())` |
| `Effect.callback((resume) => …resume(Effect.succeed(x))…)` | `callback((resume) => …resume(x)…)` |
| `Effect.runPromise(program)` in TypeScript | `runPromise(main())` in a `.ts` file or a hatch's body |
| `Stream.fromIterable` or a generator-based source | A producer: `function*` with `yield value` |

`await.allSettled` never fails with a typed error: it captures typed failures as `Result.fail`. Defects and interruption propagate exactly as in `Effect.all` with `mode: "result"` (D258).

Use Effect's operators for retries, timeouts, concurrency, and Stream transformations. Statements replace the operators that only sequence ordinary work.

## Calls run; programs stay cold

```vibe
// run: Console.log(JSON.stringify([sequential(), concurrent()]))
function sequential() {
  const first = greet("Ada")
  const second = greet("Grace")
  return [first, second]
}

function concurrent() {
  return all([greet("Ada"), greet("Grace")], { concurrency: 2 })
}
// Output: Hello, Ada
// Output: Hello, Grace
// Output: Hello, Ada
// Output: Hello, Grace
// Output: [["Hello, Ada","Hello, Grace"],["Hello, Ada","Hello, Grace"]]
```

A call whose result type is Effect runs at that point, except where a parameter expects a program: there the call is passed by name, and runs when the program runs. This applies to `Effect.retry`, `Effect.timeout`, `fork`, and entries in `Effect.all`. A program of several statements is a function with no arguments, `() => { ... }`.

A held Effect value runs when called with no arguments. Data that Effect can also execute, such as `Config` and `Exit`, stays data until explicitly called. A producer call constructs a lazy Stream; see [Producers](112-producers.md).

## Preserve a factory's call-time work

When TypeScript captures configuration before returning an Effect, keep that phase in a function with a declared Effect return and return a parameterless program (D269):

```vibe
export const capture = (input: { mutable value: number }): Effect.Effect<number, never, never> => {
  const snapshot = input.value
  return () => snapshot
}
```

A TypeScript caller can build `capture(input)`, mutate `input`, then run the program: it still gets the snapshot. The outer body runs synchronously at the call and can validate or copy plain data. Every path returns a program. **Where TypeScript returned Effect.fail(e), write return () => { throw e }**; an outer throw is a synchronous JavaScript throw. Put Effects and service lookups inside the returned program.

A forwarding member keeps its delegate's call phase with `const program: Effect.Effect<number, never, never> = base.read(path); return program`. A const with a declared Effect type holds its initializer in any scope: `const built: Made = make(config)` calls the factory now and runs none of the result. For a Registry layer, build that local before `Layer.effect(Registry)(built)`; snapshot only what TypeScript snapshots, keeping later reads such as `config.snapshots` in the returned program.

A synchronous visitor that must return a program keeps its callback type unchanged. Write `withRoute(plan, apiKey, (route) => () => Route.toModel(route))`. After checking the visitor's Result, call `routed.success()` to run it. The extra `() =>` returns the program rather than running `Route.toModel` inside the visitor. A discarded `items.map(item => save(item))` still needs `Effect.forEach` or a loop.

VibeLang callers keep their usual rules: a call runs its program, and a program argument is passed by name. `retry(make(config), policy)` calls the factory again each attempt; `const p: Made = make(config); retry(p, policy)` calls it once and retries that program. `Effect.suspend(() => () => 1)` still yields a function value.

## Declare at the module top level

```vibe
// start.vibe
export function main() {
  Console.log("started")
}
// node start.js runs main
// Output: started
```

Declare functions, services, schemas, and layers at module scope. Do not run a program there: `const answer = Effect.succeed(1)` is EK1001. An explicitly Effect-typed const, such as `const answer: Effect.Effect<number, never, never> = Effect.succeed(1)`, stores a cold program and runs nothing at import (D262). Export a `main` without parameters instead: the module runs it with `NodeRuntime.runMain` when node starts the module, and importing the module runs nothing. To provide layers or runner options, call the runner at the top level yourself, `NodeRuntime.runMain(() => Effect.provide(() => main(), AppLive))`; the module then runs only that.

Every export of Effect's root is in scope without an import, along with VibeLang's runtime helpers. This includes `Console`, `Clock`, `Option`, `Schedule`, and `Stream`. JavaScript globals such as `Array` keep their meaning; import Effect's conflicting modules under another name. Subpaths such as `effect/http` still need imports.

## Keep platform work in Effect

```vibe
// run: provide Clock.Clock = { ...Clock.Clock(), currentTimeMillis: () => 123, currentTimeMillisUnsafe: () => 123 } { Console.log(reportTime()) }
function reportTime() {
  const now = Clock.currentTimeMillis()
  Console.log(now)
  return now
}
// Output: 123
// Output: 123
```

`console.log`, `fetch`, timers, and `Date.now()` are errors in `.vibe`. Use `Console`, `Clock`, `Effect.sleep`, and Effect's HTTP modules. Importing a Node function or passing it as a value is allowed; directly running it is a platform API error. [Building an HTTP API](113-building-an-http-api.md) shows `createServer` passed to a server layer.

Inside an Effect function, a plain function from your `.ts` code or a foreign package needs a boundary hatch, `try((text) => parse(text), text)` or `sync(crypto.randomUUID())`, or a trusted declaration. A hatch's first argument is plain, self-contained TypeScript: `async`, `await`, `throw`, and platform APIs keep their JavaScript meaning there, and `runPromise` or `runFork` leads back in. It reads a value of the `.vibe` file, such as `text`, only as an input: an argument after the function, bound to its first parameter. Projects can also trust workspace or other TypeScript packages with `compilerOptions.vibelang.trustedPackages: ["@smthrs/*"]` (D261). Matching packages have synchronous plain calls whose exceptions are defects; their Effects run, and EK4401 does not fire. Use `trusted(impl, [Errors])` for a function's expected failures. Effect's own packages are trusted, including `assert` and `expect` re-exported by `@effect/vitest`.

## Catch expected failures only

```vibe
// run: Console.log(JSON.stringify([displayName("ada"), displayName("missing")]))
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
// Output: ["Ada","Unknown user: missing"]
```

Inside an Effect function, `throw` adds an expected failure to the function's error channel, and `catch` receives those failures. Defects and interruption bypass `catch`; `finally` still runs. Use `Effect.orDie(() => work())` to promote an expected failure to a defect, and Effect's cause APIs at a boundary when you need to inspect defects.

Contracts remain `Effect<A, E, R>` and can be written `A!E!R`. Most functions infer them. In the shorthand, `A!E` checks the success and error channels and infers dependencies from the body; an empty slot is `never`. With `Effect<A, E, R>`, write all three channels and use `infer E` or `infer R` to infer a channel.

For the feature with no direct Effect equivalent, see [Control operations and handlers](111-control-operations-and-handlers.md).
