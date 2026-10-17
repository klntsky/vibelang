### 5. Programs as values: retry, timeout, and pipe

#### The one rule

A call runs where it stands, except where a parameter expects a program: there it is passed by name, and runs when the program runs. A function value does not run.

```ts es
syncInventory(store)                    // runs now
fork(syncInventory(store))              // passed by name: the forked fiber runs it

const job = () => syncInventory(store)  // a program: nothing has run
job()                                   // runs it
job()                                   // runs it again, from scratch
```

That is the whole story of programs as values. An `Effect` value you get from TypeScript works the same way: it is a program, and calling it runs it (`Clock.currentTimeMillis()`). A value whose type extends `Effect` with structure of its own, such as a `Config` or an `Exit`, is data until you call it (Chapter 13).

Where a parameter's type says it expects an `Effect`, the call itself is the program (D147): the parameter already says "not yet", so the source needs no marker. Elsewhere a program is written as a function with no arguments, because that needs no new syntax: `() =>` says "not yet". A program of several statements is always `() => { … }`. A quotation operator would be new syntax with the same meaning, and Effect's constructors (`Effect.suspend(() => …)`) are the boilerplate the language exists to remove.

Effect's combinators take programs, so you pass the call itself wherever Effect expects an Effect, as in Effect's own code. The compiler passes it by name.

```ts effect
const user = yield* fetchUser(id).pipe(Effect.retry(Schedule.exponential("100 millis")))
const fresh = yield* fetchUser(id).pipe(Effect.timeout("1 second"))
const fastest = yield* Effect.race(fromCache(id), fromNetwork(id))
```

```ts es
const user = retry(fetchUser(id), Schedule.exponential("100 millis"))
const fresh = timeout(fetchUser(id), "1 second")
const fastest = race(fromCache(id), fromNetwork(id))   // the first to succeed
```

Each line is a call, so it runs: `user` is a `User`, not an `Effect`. The inner calls are passed by name: `Effect.retry` runs `fetchUser(id)` again on each attempt, with fresh local variables each time.

#### Pipe

Stacking several combinators reads best as a pipeline. Effeck's built-in `pipe` takes the program first:

```ts effect
const user = yield* fetchUser(id).pipe(
  Effect.retry({ times: 3 }),
  Effect.timeout("2 seconds"),
  Effect.withSpan("load-user")
)
```

```ts es
const user = pipe(
  () => fetchUser(id),
  retry({ times: 3 }),
  timeout("2 seconds"),
  withSpan("load-user")
)
// user: User;  E gains Cause.TimeoutError from Effect.timeout
```

When `pipe`'s first argument is a function with no parameters and the first stage expects an `Effect`, that function is the program. Applied to plain data, `pipe` is Effect's ordinary `pipe`. The types flow exactly as in Effect: `Effect.timeout` adds `Cause.TimeoutError` to E, and `Effect.retry` keeps E and R. To keep the pipeline as a program instead of running it, wrap it: `const loadUser = () => pipe(() => fetchUser(id), …)`.

Most pipes you write today disappear:

| You pipe today | In Effeck |
| --- | --- |
| `Effect.map`, `Effect.flatMap`, `Effect.andThen`, `Effect.tap` | statements |
| `Effect.catchTag`, `Effect.catch`, `Effect.mapError` | `try`/`catch` |
| `Effect.ensuring` | `finally` |
| `Effect.provide`, `Effect.provideService` | the same functions, with a program argument |
| `Effect.retry`, `Effect.timeout`, `Effect.withSpan`, `Effect.cached`, `Effect.uninterruptible` | the same functions, called or piped as above |

On a cause that also holds a defect, Effect's recovery operators and `try`/`catch` differ; Chapter 17 lists how.

Measured on two large Effect codebases, once statements replace sequencing, error handling, and cleanup, only 1.1% (Smithers) and 2.6% (opencode) of the Effect pipes outside tests keep two or more stages (`docs/research/pipe-census.md`). Pipelines over plain data stay plain: `pipe(items, Array.filter(isActive), Array.map(toRow))` is an ordinary function call.

#### Methods on Effect's data types

A value of one of Effect's data types takes its module's functions as methods (D152). `x.name(args)` calls `Module.name(x, args)`:

```ts effect
const alerts = pipe(readings, Stream.filter((c) => c > 25), Stream.map((c) => `${c} C`), Stream.take(3))
const label = Option.getOrElse(Option.map(user.nickname, (n) => n.toUpperCase()), () => "anonymous")
```

```ts es
const alerts = readings.filter((c) => c > 25).map((c) => `${c} C`).take(3)
const label = user.nickname.map((n) => n.toUpperCase()).getOrElse(() => "anonymous")
```

The module is the one that declares the value's type: a `Stream` takes `Stream`'s functions, an `Option` takes `Option`'s. Eighteen modules qualify: `BigDecimal`, `Cause`, `Chunk`, `DateTime`, `Deferred`, `Duration`, `Effect`, `Exit`, `Fiber`, `HashMap`, `HashSet`, `Option`, `Queue`, `Ref`, `Result`, `Schedule`, `Sink`, and `Stream`. A type's own members come first, so an `Effect`'s `pipe` is its own, and arrays, strings, and objects keep JavaScript's methods: `[1, 2].map(f)` is `Array.prototype.map`. A method that returns an Effect is a call like any other, so it runs where it stands: `fiber.join()` waits for the fiber, and `counter.update((n) => n + 1)` updates the `Ref`. Chapter 13 states the rule.

The compiled JavaScript calls the module's function, `Stream.take(Stream.map(Stream.filter(readings, …), …), 3)`, so a bundler drops the functions a program never calls, as it does for Effect code.

#### Why there is no `|>`

JavaScript's proposed pipeline operator is not part of Effeck. It conflicts with the rule that calls run: in `fetchUser(id) |> Effect.retry(%, policy)`, the call `fetchUser(id)` runs before the pipe sees it, and so would every stage that calls a combinator. The proposal is also not settled: it has been at Stage 2 since 2021-08, its topic token is undecided (the `%` in its README is disqualified on the proposal's own wiki), the committee's January 2026 review listed it as "somewhat stuck", and TypeScript waits for Stage 3 before implementing it (`docs/research/tc39-syntax.md` §1). Effeck revisits the operator if it reaches Stage 3.

#### Storing, returning, and composing programs

Programs are values, so you can put them in variables, return them, and pass them around. Nothing runs until something calls them.

```ts es
function makeSync(store: Store) {
  return () => syncInventory(store)            // returns a program
}
// (() => SyncReport!SyncError!Inventory)!!

function syncEverything(stores: ReadonlyArray<Store>) {
  const syncAll = stores.map(makeSync)                    // an array of programs; nothing ran
  return all(syncAll, { concurrency: 4 })          // now they run
}
```

`stores.map(makeSync)` passes `makeSync` to JavaScript's `Array.prototype.map`, which expects a plain function. `makeSync` performs no Effect operation, so it compiles to plain JavaScript (Chapter 15), and `map` receives that form: `syncAll` holds programs. A function that can pause cannot be passed to `map` (EK4301, Chapter 10). Where `Effect.all` expects Effects, an array or record of programs converts element by element.

```ts es
function startServer() {
  const cachedConfig = Effect.cached(() => loadConfig())  // Effect.cached returns a program
  const config = cachedConfig()                           // loads once
  const again = cachedConfig()                            // returns the cached value
  return Http.listen(config.port)
}
```
