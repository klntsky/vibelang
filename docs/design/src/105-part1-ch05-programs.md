### 5. Programs as values: retry, timeout, and pipe

#### The one rule

A call runs where it stands, except where a parameter expects a program: there it is passed by name, and runs when the program runs. A function value does not run.

```ts es
syncInventory(store)                    // runs now
fork syncInventory(store)              // passed by name: the forked fiber runs it

const job = () => syncInventory(store)  // a program: nothing has run
job()                                   // runs it
job()                                   // runs it again, from scratch
```

An `Effect` value from TypeScript works the same way: it is a program, and calling it runs it (`Clock.currentTimeMillis()`). A value whose type extends `Effect` with structure of its own, such as a `Config` or an `Exit`, is data until you call it (Chapter 13).

Where a parameter's type expects an `Effect`, the call itself is the program (D147): the parameter already says "not yet", so the source needs no marker. Elsewhere a program is a function with no arguments, which needs no new syntax: `() =>` says "not yet". A program of several statements is always `() => { … }`. A quotation operator would be new syntax with the same meaning, and Effect's constructors (`Effect.suspend(() => …)`) are the boilerplate the language exists to remove.

Effect's combinators take programs, so pass the call itself, as in Effect's own code; the compiler passes it by name.

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

#### The pipe operator

Several stacked combinators read best as a pipeline. VibeLang has TC39's proposed Hack-style pipe operator (D154): in `x |> f(%, a)`, the topic `%` stands for `x`.

```ts effect
const user = yield* fetchUser(id).pipe(
  Effect.retry({ times: 3 }),
  Effect.timeout("2 seconds"),
  Effect.withSpan("load-user")
)
```

```ts es
const user = fetchUser(id)
  |> retry(%, { times: 3 })
  |> timeout(%, "2 seconds")
  |> withSpan(%, "load-user")
// user: User;  E gains Cause.TimeoutError from Effect.timeout
```

The topic follows the one rule (D155). Each `%` above stands where a parameter expects a program, so the expression before its `|>` is passed by name: `retry` receives `fetchUser(id)` as a program and runs it again on each attempt, `timeout` receives the retried program, and `withSpan` the timed one. Where `%` stands for a value, the expression before `|>` runs once, before the body, and `%` is its result: `9 |> addMargin(%) |> double(%)` is `double(addMargin(9))`. A pipe does one or the other (EK1008), and its body must use `%` (EK1006). The types flow exactly as in Effect: `Effect.timeout` adds `Cause.TimeoutError` to E, and `Effect.retry` keeps E and R. The pipe is a call, so it runs; to keep it as a program, wrap it: `const loadUser = () => fetchUser(id) |> retry(%, { times: 3 })`.

The built-in `pipe` function still works: on plain data it is Effect's `pipe`; given a function with no parameters first, that function is the program. It suits data-last functions with no data-first form, such as `Match.when`.

Most pipes you write today disappear:

| You pipe today | In VibeLang |
| --- | --- |
| `Effect.map`, `Effect.flatMap`, `Effect.andThen`, `Effect.tap` | statements |
| `Effect.catchTag`, `Effect.catch`, `Effect.mapError` | `try`/`catch`, testing the failure with `match` or `is` (D216), or `x catch (e) => …` (D236) |
| `Effect.orElseSucceed`, `Effect.orElse` | `x catch fallback` (D236) |
| `Effect.ensuring` | `finally` |
| `Effect.provide`, `Effect.provideService` | the same functions, with a program argument |
| `Effect.retry`, `Effect.timeout`, `Effect.withSpan`, `Effect.cached` | the same functions, called or piped with `\|>` as above |
| `Effect.catchCause`, `Effect.failCause`, `Effect.onExit` | `match (exit(…))`, `throw cause`, `finally (exit)` (D217, D177, D178) |
| `Effect.uninterruptibleMask`, `Effect.interruptible` | `uninterruptible { … restore { … } }`, `interruptible { … }` (D180) |

On a cause that also holds a defect, Effect's recovery operators and `try`/`catch` differ; Chapter 17 lists how. A catch expression recovers as `try`/`catch` does.

In two large Effect codebases, once statements replace sequencing, error handling, and cleanup, only 1.1% (Smithers) and 2.6% (opencode) of Effect pipes outside tests keep two or more stages (`docs/research/pipe-census.md`). Pipelines over plain data stay plain: `items |> Arr.filter(%, isActive) |> Arr.map(%, toRow)` is `Arr.map(Arr.filter(items, isActive), toRow)`.

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

#### Why `|>` passes by name

D8 kept the operator out of v0 because calls run: in `fetchUser(id) |> Effect.retry(%, policy)`, a topic that always held a value would run `fetchUser(id)` once, before `Effect.retry` saw it, and retry would have nothing to repeat. The by-name rule (D147) removes the conflict: where `%` stands for a program, the expression before `|>` is passed as one, exactly as `retry(fetchUser(id), policy)` passes it. The proposal is still at Stage 2 with an undecided topic token (`docs/research/tc39-syntax.md` §1); VibeLang uses `%`, the token in the proposal's README, and will follow TC39's choice.

#### Storing, returning, and composing programs

Programs are values: store them in variables, return them, pass them around. Nothing runs until something calls them.

```ts es
function makeSync(store: Store) {
  return () => syncInventory(store)            // returns a program
}
// Effect<() => Effect<SyncReport, SyncError, Inventory>>

function syncEverything(stores: ReadonlyArray<Store>) {
  const syncAll = stores.map(makeSync)                    // an array of programs; nothing ran
  return all(syncAll, { concurrency: 4 })          // now they run
}
```

`stores.map(makeSync)` passes `makeSync` to JavaScript's `Array.prototype.map`, which expects a plain function. `makeSync` performs no Effect operation, so it compiles to plain JavaScript (Chapter 15) and `map` receives that form: `syncAll` holds programs. A function that can pause cannot be passed to `map` (EK4301, Chapter 10). Where `Effect.all` expects Effects, an array or record of programs converts element by element.

To capture inputs at call time for a TypeScript caller, declare an Effect return and return a program (D269):

```ts es
export const capture = (input: { mutable value: number }): Effect.Effect<number, never, never> => {
  const snapshot = input.value
  return () => snapshot
}
```

The factory body runs as plain JavaScript at the call, so changing `input.value` after `capture(input)` does not change the snapshot its program returns. The returned program can run repeatedly. Every path must return a program; Effects and service lookups belong inside it (EK2115, EK2116). Where TypeScript returned `Effect.fail(e)`, write `return () => { throw e }`; an outer `throw e` would throw synchronously.

A declared Effect const holds an initializer at any scope: `const p: Effect.Effect<number> = capture(input)` calls the factory once and stores its result. `retry(p, policy)` reuses that snapshot; `retry(capture(input), policy)` passes the call by name and takes a fresh snapshot per attempt. Without a declared Effect return, the earlier `makeSync` example keeps its meaning.

```ts es
function startServer() {
  const cachedConfig = Effect.cached(() => loadConfig())  // Effect.cached returns a program
  const config = cachedConfig()                           // loads once
  const again = cachedConfig()                            // returns the cached value
  return Http.listen(config.port)
}
```
