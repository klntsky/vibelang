### C. Every Effect feature in EffectScript

```ts es
const Port = Config.withDefault(Config.Port("PORT"), 3000)      // unchanged by design: Effect's API, as is

function loadUser(id: string) {
  return Effect.retry(() => Users.find(id), { times: 3 })        // changed: the program is a function
}
```

Every section of Effect 4's documentation, the Effect 3 pages that version 4 dropped, and the `effect@4.0.5` modules the site omits (HTTP, SQL, CLI, AI, cluster, workflow, RPC), mapped to EffectScript. **Changed** means EffectScript has its own form. **Unchanged by design** means you use Effect's API as you do today, on purpose: the call rule (Chapter 13) makes Effect's API work directly, and a second way to do the same thing would be ceremony. The map comes from the feature-coverage audit (`docs/research/effect-feature-coverage.md`, 309 features), updated to the decisions in Chapter 29. The one open item is partial annotation (O7).

#### C1. Getting started: the Effect type, creating, running, generators, pipelines

| Feature | In EffectScript | Form |
| --- | --- | --- |
| The `Effect<A, E, R>` type | Changed | Inferred for every function; written as a return annotation to have it checked (Chapter 2) |
| Type helpers (`Effect.Success`, `Error`, `Services`) | Unchanged by design | Apply them to the TypeScript type of an EffectScript function |
| Annotate one channel, infer the rest | Owner's call (O7) | Until decided, a contract has all three channels or none |
| Installation | Changed | `effect@4.0.5`, `@effectscript/runtime`, and `@effectscript/compiler` (Chapter 28) |
| Importing Effect | Changed | The prelude needs no import (`Effect`, `Schema`, `Layer`, `Scope`, `TaggedError`, `Control`, and the built-ins); other modules come from `effect` |
| `Effect.succeed` | Changed | `return 42` |
| `Effect.fail`, `failSync` | Changed | `throw new HttpError()` |
| `Effect.sync` around a side effect | Changed | Call the code; platform side effects go through Effect modules (Chapter 11) |
| `Effect.try`, `tryPromise`, `promise` | Unchanged by design | The same calls, which run and give the value (Chapter 10) |
| `Effect.suspend` | Changed | `() => …` wherever a program is expected |
| `Effect.callback` | Unchanged by design | The same call; `resume(Exit.succeed(x))` (D43) |
| `Effect.void`, `never`, `yieldNow`, `interrupt` | Unchanged by design | Call the value: `Effect.yieldNow()` |
| Runners (`runPromise`, `runSync`, `runFork`, `runCallback`) | Changed | Take a program, `Effect.runPromise(() => main())`, only where no fiber runs the code (ES1005, D38) |
| `Effect.gen` and `yield*` | Changed | An ordinary function body, and calls (Chapter 2); `Effect.gen` in `.es` is ES1002 |
| `Effect.gen({ self: this }, …)` | Changed | An ordinary method; `this` keeps its JavaScript meaning |
| `Effect.fn`, `fnUntraced` | Changed | `function f() { … }`, untraced unless `tracing` asks for spans (Chapter 28) |
| `map`, `flatMap`, `andThen`, `tap`, `as` | Changed | Statements |
| `pipe` and `.pipe` over programs | Changed | `pipe(() => f(), Effect.retry(…), Effect.timeout(…))` (D8) |
| `pipe` over data | Unchanged by design | `pipe(items, Array.filter(isActive))` |
| `Effect.all`, `zip`, `zipWith` | Changed | Programs as functions: `Effect.all([() => a(), () => b()])` |
| `Effect.fromOption`, `fromResult`, `fromNullishOr` | Unchanged by design | `const n = Effect.fromOption(parse(x))` |
| Control flow inside `Effect.gen` | Changed | `if`, `for`, `while`, and early `return` in an ordinary function |
| Devtools | Changed | The fork's language server for `.es`; `@effect/tsgo` stays on `.ts`; Effect's `DevTools` for fibers and spans (Chapter 28) |

#### C2. Error management

On a cause that mixes an expected failure and a defect, Effect's operators below keep their Effect behavior, which differs from EffectScript's `catch`; Chapter 17 lists how.

| Feature | In EffectScript | Form |
| --- | --- | --- |
| Expected errors and defects | Changed | `throw` is expected; an exception no contract lists, `Effect.die`, and `Effect.orDie` are defects (Chapter 3) |
| A JavaScript `throw` | Changed | Always a typed failure; `Effect.die(new Error(…))` for a bug |
| A JavaScript `catch` around a throwing call | Changed | `catch` sees expected failures only; a dead `catch` is ES2004 (D37) |
| `Data.TaggedError`, `Data.Error`, `Schema.TaggedError` | Changed | `TaggedError` is in the prelude; `Schema.TaggedError` is unchanged |
| Error tracking in E | Changed | Inferred: what a function throws, plus its callees' E, minus what it catches |
| `Effect.catch`, `catchTag`, `catchTags`, `catchIf` | Changed | `try`/`catch` with narrowing; the combinators still work with a program |
| `catchReason`, `catchReasons`, `unwrapReason` | Changed | Narrow `e._tag` and `e.reason._tag` inside `catch` |
| `Effect.die`, `orDie`, `catchDefect`, `catchCause`, `tapDefect` | Changed | The same functions with a program: `Effect.orDie(() => f())` |
| `Effect.result`, `option`, `exit` | Changed | `Effect.result(() => task())` |
| `orElseSucceed`, `firstSuccessOf` | Changed | `try`/`catch`, or `Effect.firstSuccessOf([() => a(), () => b()])` |
| `Effect.match`, `matchEffect`, `matchCause` | Changed | `try` with statements; `Effect.exit(() => f())`, then `Exit.match` |
| `Effect.ignore`, `ignoreCause` | Changed | `Effect.ignore(() => task())` |
| `Effect.retry`, `retryOrElse` | Changed | `Effect.retry(() => request(), { times: 5 })`, with fresh locals per attempt |
| `Effect.timeout`, `timeoutOption`, `timeoutOrElse` | Changed | `Effect.timeout(() => task(), "1 second")`; E gains `Cause.TimeoutError` |
| `Effect.sandbox` | Changed | `Effect.sandbox(() => task())` inside `try`; `catch (cause)` gets the cause |
| `Effect.validate`, `partition` | Unchanged by design | The same calls, with EffectScript callbacks |
| `Effect.all` with `mode: "result"` | Changed | Programs as functions, the same options |
| `mapError`, `mapBoth`, `tapError`, `flip`, `filterOrFail` | Changed | `catch (e) { throw new Other({ e }) }`; `if (…) throw new E()` |
| Parallel and sequential errors (`Cause.reasons`) | Unchanged by design | `catch` gets the first expected failure (D28); `Effect.catchCause` sees all |
| Yieldable errors | Changed | `throw new E()`; `new` never runs anything (D24) |
| A contract with E = `never` | Changed | Checked by the compiler (ES2001) |

#### C3. Requirements management: services, layers, memoization

| Feature | In EffectScript | Form |
| --- | --- | --- |
| `Context.Service` class | Changed | `abstract class Users { abstract find(id: string): Effect<User, UserNotFound, never> }` (D1) |
| Service keys | Changed | Derived from package, path, and class name; duplicates are ES3001 |
| `yield* Users`, then a method | Changed | `Users.find(id)`; `Users()` gives the implementation |
| `Users.use`, `useSync` | Changed | Call the member: `Users.find(id)` |
| A member that returns a plain value (`Path.join`) | Changed | `Path.join(a, b)`; the service joins R (D41) |
| A member that is an `Effect` value | Unchanged by design | Call it: `Terminal.readLine()` |
| The `make` option and `static layer` | Changed | `static make()` and `static readonly layer` on the service class (D40) |
| `Context.Reference` with a default | Unchanged by design | `Context.Reference(key, { defaultValue })` as is, or `static readonly default` on a service class (D40) |
| `Effect.serviceOption` | Unchanged by design | `Effect.serviceOption(Mailer)` |
| Default services (`Clock`, `Console`, `Random`, `Tracer`, `ConfigProvider`) | Unchanged by design | `Clock.currentTimeMillis()`; R stays empty |
| `Effect.provideService` | Changed | `Effect.provideService(() => program(), Mailer, quiet)` |
| `Context.make`, `Context.add` | Unchanged by design | Values, provided with `Effect.provide(() => program(), context)` |
| `Layer.succeed`, `effect`, `sync` | Changed | `Layer.effect(Users, () => { const db = Database(); return { … } })` |
| Layers that hold resources | Changed | Acquire without `using`; the layer's Scope owns the resource |
| Requirements that leak into methods | Changed | ES3101 |
| `Layer.merge`, `mergeAll`, `provide`, `provideMerge` | Unchanged by design | The same functions; `wire(…)` orders them and reports graph errors (D27) |
| Missing, duplicate, and cyclic providers | Changed | ES3201, ES3202, ES3203 |
| `Effect.provide` | Changed | `Effect.provide(() => program(), AppLive)`; each `provide` builds its layers once (D21) |
| Layer memoization, `Layer.fresh`, MemoMaps | Unchanged by design | As in Effect; provide at `main`, or run entry points through one `ManagedRuntime` (D21) |
| `Layer.launch`, `build`, `unwrap`, `mock`, `tap`, `catch` | Unchanged by design | Values and calls as in Effect |
| Fake services | Changed | `Layer.succeed(Users, { find: (id) => ada, rename: () => {} })` |

#### C4. Resource management

| Feature | In EffectScript | Form |
| --- | --- | --- |
| `Effect.scoped` | Changed | A block that declares `using` |
| `acquireRelease`, `acquireUseRelease` | Changed | `using lock = Effect.acquireRelease(() => acquire(), (l) => release(l))` |
| Interruptible acquisition | Unchanged by design | `acquireRelease(…, { interruptible: true })`; `using` keeps the choice (D25) |
| A resource the caller owns | Changed | Acquire without `using`; `Scope` stays in R (D14) |
| `ensuring`, `onExit`, `onError`, `addFinalizer` | Changed | `finally`; `Effect.onExit(() => f(), (exit) => …)` when the exit matters |
| `Scope.make`, `close`, `provide` | Unchanged by design | The same calls |
| `Disposable` resources | Changed | `using x = disposable` registers `[Symbol.dispose]()` |
| `RcRef`, `RcMap`, `Pool`, `ScopedRef`, `ScopedCache`, `LayerMap` | Changed | `using pool = Pool.make({ acquire: () => connect(), size: 10 })` |
| Cleanup when a handler abandons a program | Changed | No Effect equivalent; `using` and `finally` run before `handle` returns |

#### C5. Observability

| Feature | In EffectScript | Form |
| --- | --- | --- |
| `Effect.log` and its levels | Unchanged by design | `Effect.logWarning("disk low")` |
| A minimum log level for a region | Changed | `Effect.provideService(() => program(), References.MinimumLogLevel, "Debug")` |
| `annotateLogs`, `withLogSpan` | Changed | With a program: `Effect.withLogSpan(() => handle(req), "handler")` |
| `annotateLogsScoped` | Unchanged by design | The same call; a `using` block or the caller supplies the Scope |
| `Logger.make`, `Logger.layer`, formats, `Logger.toFile` | Unchanged by design | Values, provided as layers |
| `Console` | Unchanged by design | `Console.log("hi")`; `console.log` is ES4402 |
| Metrics (`counter`, `gauge`, `histogram`, `summary`, `frequency`, `timer`) | Unchanged by design | Defined at the top level; `Metric.update(requests, 1)` |
| `trackSuccesses`, `trackDuration` | Changed | `Effect.trackDuration(() => task(), latency)` |
| `Metric.CurrentMetricAttributes` | Changed | Provided around a program with `Effect.provideService` |
| Spans per function (`Effect.fn("name")`) | Changed | `"tracing"` in `tsconfig.json`, or `Effect.withSpan` where you want one (Chapter 28) |
| `Effect.withSpan` | Changed | `Effect.withSpan(() => loadUser(id), "load-user")` |
| `annotateCurrentSpan`, `currentSpan`, `linkSpans` | Unchanged by design | The same calls |
| Logs as span events | Unchanged by design | Spans stay correct across handler capture (G6a) |
| Exporting traces, logs, and metrics (`OtlpTracer`, `OtlpLogger`, `OtlpMetrics` from `effect/observability`) | Unchanged by design | Layers, provided as in Effect: `OtlpTracer.layer({ url, resource: { serviceName } })` |
| `FiberSet`, `FiberMap`, `FiberHandle` | Changed | `using set = FiberSet.make()`, then `FiberSet.run(set, () => task())` |
| Supervisors (Effect 3) | Unchanged by design | Effect 4 has none; `FiberSet` and `fork` |

#### C6. Configuration

| Feature | In EffectScript | Form |
| --- | --- | --- |
| `Config.String`, `Port`, `Int`, `Boolean`, `Duration`, `URL`, `Redacted` | Unchanged by design | Data until called: `const Port = Config.Port("PORT")`, then `Port()` (D24) |
| `Config.all`, `withDefault`, `nested`, `option`, `orElse`, `schema` | Unchanged by design | Composed as data, read once by calling |
| `ConfigProvider` (`fromEnv`, `fromUnknown`, `layer`) | Unchanged by design | Provided as a layer |
| `ConfigProvider.fromDotEnv()`, `fromDir()` | Unchanged by design | Calls that run, inside a function |
| `config.parse(provider)` in tests | Unchanged by design | `AppConfig.parse(provider)` returns an Effect, so the call runs it |
| A Layer chosen by config (`Layer.unwrap`) | Changed | `Layer.unwrap(() => …)`, with a program |
| `Redacted` | Unchanged by design | `Redacted.value(DatabaseUrl())` |

#### C7. Runtime

| Feature | In EffectScript | Form |
| --- | --- | --- |
| `NodeRuntime.runMain`, `BunRuntime.runMain` | Changed | `NodeRuntime.runMain(() => main())` at a module's top level |
| `runMain` options | Changed | The same options, with a program |
| One-shot runners and `run*With(context)` | Changed | Programs as functions, only where no fiber runs the code (D38) |
| `ManagedRuntime` | Unchanged by design | `ManagedRuntime.make(AppLive)`; `runtime.runPromise(() => handle(req))` in a framework callback, or from TypeScript |
| Top-level `await` | Changed | No `await`; the top level builds values and calls runners (ES1001) |
| Scheduling and fairness | Unchanged by design | 2048 operations per slice; a plain JavaScript region is atomic |
| `ErrorReporter`, `Effect.withErrorReporting` | Changed | `Effect.withErrorReporting(() => program())`: the thunk is a program (D39) |

#### C8. Scheduling

| Feature | In EffectScript | Form |
| --- | --- | --- |
| Schedule constructors (`spaced`, `fixed`, `exponential`, `recurs`, `cron`) | Unchanged by design | `Schedule.exponential("250 millis")` |
| Schedule combinators | Unchanged by design | `.pipe` over a schedule value |
| Effectful schedule callbacks (`tap`, `modifyDelay`) | Unchanged by design | EffectScript callbacks |
| `Effect.repeat` | Changed | `Effect.repeat(() => poll(), { schedule, until: (s) => s === "done" })` |
| `Effect.retry` with a schedule | Changed | `Effect.retry(() => f(), Schedule.exponential("100 millis"))` |
| `Effect.schedule`, `repeatOrElse` | Changed | Programs as functions |
| `Cron.parse`, `Schedule.cron` | Unchanged by design | `Effect.fromResult(Cron.parse("0 3 * * *", "UTC"))` |
| A cron job's lifetime | Changed | `fork(() => Effect.schedule(() => report(), Schedule.cron(cron)))` inside `main` |

#### C9. State management

| Feature | In EffectScript | Form |
| --- | --- | --- |
| `Ref` | Unchanged by design | `const ref = Ref.make(0); Ref.update(ref, (n) => n + 1)` |
| Programs stored as object fields | Changed | `inc = () => Ref.update(this.value, (n) => n + 1)` |
| `SynchronizedRef.updateEffect` | Unchanged by design | `SynchronizedRef.updateEffect(ref, (xs) => [...xs, load()])` |
| `SubscriptionRef.changes` | Unchanged by design | A `Stream`: iterate it with `for...of` |
| Transactions (`TxRef`, `Effect.tx`) | Changed | `Effect.tx(() => TxRef.update(ref, (n) => n + 1))` |

#### C10. Caching

| Feature | In EffectScript | Form |
| --- | --- | --- |
| `Effect.cached` | Changed | `const get = Effect.cached(() => load())`, then `get()` |
| `cachedWithTTL`, `cachedInvalidateWithTTL` | Changed | Programs as functions; `get()` and `invalidate()` |
| `Cache` | Unchanged by design | `Cache.make({ capacity: 100, lookup: (id) => fetchUser(id) })` |

#### C11. Batching

| Feature | In EffectScript | Form |
| --- | --- | --- |
| `Request.Class`, `TaggedClass` | Unchanged by design | The same classes |
| `RequestResolver.make`, `fromEffect`, `makeGrouped`, `batchN` | Unchanged by design | Callbacks are EffectScript functions |
| `Effect.request` | Unchanged by design | `Effect.request(new GetUser({ id }), resolver)` |
| Batches form under concurrency | Unchanged by design | Sequential calls never batch, as in Effect |
| `RequestResolver.withCache` | Unchanged by design | Inside a function or a Layer |
| A resolver that needs a service | Changed | `Layer.effect(Todos, () => { const http = Http(); … })` |

#### C12. Concurrency

| Feature | In EffectScript | Form |
| --- | --- | --- |
| The `concurrency` option | Changed | Programs as functions, the same options |
| Struct and record forms of `Effect.all` | Changed | Record fields convert to programs |
| A computed array of programs | Changed | `Effect.all(stores.map(makeSync))` (D9) |
| `Effect.forEach` with concurrency | Unchanged by design | The same call |
| `Effect.zip` with `concurrent: true` | Changed | Programs as functions |
| `race`, `raceFirst`, `raceAll` | Changed | Programs as functions; each has its own exit rule (Chapter 7) |
| `forkChild`, `forkDetach`, `forkScoped`, `forkIn` | Changed | Programs as functions |
| Background work owned by a block | Changed | `fork(() => heartbeat())` (D19) |
| `Fiber.join`, `await`, `interrupt`, `joinAll`, `interruptAll` | Unchanged by design | The same calls |
| Interruption is not an error | Changed | `catch` never sees it; `finally` and `using` always run |
| `uninterruptible`, `uninterruptibleMask`, `interruptibleMask` | Changed | Programs as functions |
| `onInterrupt` | Changed | `Effect.onInterrupt(() => task(), () => Console.log("stopped"))` |
| `Effect.forever` | Changed | `while (true) tick()` |
| `Deferred` | Unchanged by design | `Deferred.make<number>()`, `Deferred.await(d)` |
| `Deferred.complete` with a program | Changed | `Deferred.complete(d, () => task())` |
| `Queue` | Unchanged by design | `Queue.offer(queue, 1)` |
| `PubSub.subscribe` | Changed | `using subscription = PubSub.subscribe(pubsub)` |
| `PubSub.publish` | Unchanged by design | The same call |
| `Semaphore` | Unchanged by design | `Semaphore.make(1)`; `mutex.withPermits(1)(() => task())` |
| `Latch` | Unchanged by design | `latch.open()`: an `Effect` member runs when called |
| What a forked fiber sees | Unchanged by design | A snapshot of the parent's Context |
| Control operations across fibers | Changed | ES4201 (Chapter 9) |

#### C13. Async interop

| Feature | In EffectScript | Form |
| --- | --- | --- |
| `async` and `await` | Changed | None in `.es`: a call waits |
| `Effect.promise`, `Effect.tryPromise` | Unchanged by design | The same calls; the `AbortSignal` cancels on interruption |
| `Effect.callback` | Unchanged by design | A boundary; `resume(Exit.succeed(x))`; `suspend` for resources (D43) |
| `Effect.runPromise` from Promise code | Unchanged by design | From TypeScript, or a top-level callback (D38) |
| Async iterables and event emitters | Unchanged by design | `Stream.fromAsyncIterable`, `Stream.fromEventListener` |
| Synchronous code that throws | Changed | `Effect.try`; a dead `catch` is ES2004 (D37) |
| Effect 3 libraries | Changed | Foreign code: `Effect.tryPromise({ try: () => Effect3.runPromise(p), catch })` |

#### C14. Streams

| Feature | In EffectScript | Form |
| --- | --- | --- |
| A sequence produced by code that waits between values | Changed | A `function*` producer and `for...of` (Chapter 8) |
| Streams from data (`make`, `range`, `fromIterable`, `tick`) | Unchanged by design | `Stream.range(1, 10)` |
| `fromEffect`, `fromEffectRepeat` | Changed | Programs as functions |
| `unfold`, `paginate` | Unchanged by design | EffectScript callbacks |
| `Stream.callback`, `fromQueue`, `fromPubSub`, `fromEventListener` | Unchanged by design | The same calls; inner JavaScript callbacks cannot pause (ES4301) |
| Operators (`map`, `filter`, `mapEffect`, `merge`, `buffer`, `debounce`, `groupedWithin`) | Unchanged by design | The same calls; a producer converts to a `Stream` (D26) |
| `runCollect`, `runForEach`, `runFold`, `runDrain`, `run` | Unchanged by design | Calls that run the stream |
| `Stream.toPull` | Changed | `for...of` over a `Stream` (D26) |
| `Stream.catch`, `retry`, `timeout` | Unchanged by design | The same calls; `try`/`catch` applies to the call that runs a stream |
| `Stream.scoped` | Unchanged by design | `Stream.scoped(stream)`: it takes a Stream |
| `Stream.ensuring` | Changed | `Stream.ensuring(stream, () => cleanup())`: the finalizer is a program |
| `Stream.partition`, `broadcastN` | Changed | `using`, because they need a Scope |
| Producers and streams together | Changed | A producer is a `Stream` wherever one is expected (D26) |
| `Stream.toAsyncIterable` | Unchanged by design | From TypeScript, with `for await` |

#### C15. Sinks

| Feature | In EffectScript | Form |
| --- | --- | --- |
| Sinks (`sum`, `count`, `collect`, `head`, `drain`) | Unchanged by design | `Stream.run(stream, Sink.sum)` |
| `Sink.fold`, `foldUntil`, `forEach` | Unchanged by design | EffectScript callbacks; pure ones pass as plain functions (D9) |
| A producer into a sink | Changed | The producer converts to a `Stream` (D26) |

#### C16. Testing

| Feature | In EffectScript | Form |
| --- | --- | --- |
| `it.effect` | Changed | `it.effect("name", () => { … })` |
| `it.live`, `.each`, `.prop`, `it.flakyTest` | Unchanged by design | The same calls |
| `layer(AppLive)("suite", (it) => …)` | Unchanged by design | The same call |
| Fake services | Changed | Object literals in `Layer.succeed` |
| An override for one region | Changed | `Effect.provideService(() => program(), Mailer, quiet)` |
| Scripted answers to requests | Changed | `handle` (Chapter 9) |
| `TestClock` | Unchanged by design | `TestClock.adjust("1 minute")` |
| `TestConsole`, `TestCrypto` | Unchanged by design | `TestConsole.logLines()` |
| Asserting on an `Exit` | Unchanged by design | `Exit.succeed(1)` is data (D24) |
| Promise-style tests | Changed | Stay in `.ts`, or move to `it.effect` (D38) |
| Property tests | Unchanged by design | `it.effect.prop`, `Arbitrary.checkEffect` |

#### C17. Code style

| Feature | In EffectScript | Form |
| --- | --- | --- |
| Run once, at the edge | Changed | `NodeRuntime.runMain(() => main())` |
| Avoid tacit calls | Unchanged by design | The same advice |
| Dual APIs | Changed | Data-first with a program; data-last inside `pipe` |
| Branded types | Unchanged by design | A failed check throws a defect; `Int.result(n)` for a typed error |
| Pattern matching with pure handlers (`Match`) | Unchanged by design | The same module |
| Pattern matching with effectful branches | Changed | `switch` or `if`; effectful `Match` handlers are ES4301 (D9) |
| Do notation (`Effect.Do`, `bind`, `let`) | Changed | Statements |
| Control flow operators (`Effect.when`, `whileLoop`) | Changed | `if` and `while` |

#### C18. Data types

| Feature | In EffectScript | Form |
| --- | --- | --- |
| `Option` | Unchanged by design | Plain data; `Option.gen` stays in `.ts` |
| An `Option` to a typed failure | Changed | `Effect.fromOption(found)`, or `throw` |
| `Result` (Effect 3's `Either`) | Unchanged by design | Plain data |
| `Effect.result`, `Effect.exit` | Changed | Programs as functions |
| `Effect.fromResult` | Unchanged by design | The same call |
| `Exit` constructors and readers | Unchanged by design | Data (D24); `Exit.match` |
| `Cause` | Unchanged by design | `Effect.catchCause(() => f(), (cause) => …)` |
| `Data.Class`, `TaggedClass`, `taggedEnum` | Unchanged by design | The same functions |
| Error classes | Changed | `TaggedError` in the prelude; `throw` |
| `new` of a class whose instances are Effects | Changed | `new` constructs and never runs (D24) |
| `Chunk`, `HashSet`, `HashMap`, `Duration`, `BigDecimal`, `Redacted` | Unchanged by design | The same modules |
| `DateTime` | Unchanged by design | `DateTime.now()`; `new Date()` is ES4403 |
| A time zone for a region | Changed | Programs as functions |

#### C19. Traits and behaviours

| Feature | In EffectScript | Form |
| --- | --- | --- |
| `Equal`, `Hash` | Unchanged by design | The same functions; `===` keeps its JavaScript meaning. A class implements `[Equal.symbol]` and `[Hash.symbol]` with plain methods (D55, Chapter 15) |
| `Equivalence`, `Order` | Unchanged by design | The same modules |

#### C20. Schema

| Feature | In EffectScript | Form |
| --- | --- | --- |
| Defining schemas | Unchanged by design | Values, built at the top level |
| `decodeUnknownEffect`, `decodeEffect`, `encodeEffect` | Unchanged by design | Calls that run; `SchemaError` joins E |
| `decodeUnknownSync`, `is`, `asserts` | Unchanged by design | Trusted; their exceptions are defects (D37) |
| `decodeUnknownExit` | Unchanged by design | An `Exit` value (D24) |
| Filters and transformations | Unchanged by design | The same functions |
| `transformEffect`, `checkEffect` | Unchanged by design | EffectScript callbacks |
| Defaults given as Effects (`withConstructorDefault`) | Changed | A program: `Schema.withConstructorDefault(() => Clock.currentTimeMillis())` |
| `Schema.Class`, `TaggedClass` | Unchanged by design | The same classes |
| `Schema.TaggedError` | Changed | `throw new HttpError({ status: 500 })` |
| `make`, `makeEffect` | Unchanged by design | `make` throws defects; `makeEffect` runs |
| Annotations, formatters, JSON Schema, Arbitrary, compilers | Unchanged by design | The same functions |
| `Config.schema` | Unchanged by design | Data (D24) |

#### C21. Platform

| Feature | In EffectScript | Form |
| --- | --- | --- |
| Node.js and browser APIs | Changed | Not callable from `.es` (ES4402); Chapter 11 names each replacement |
| Platform layers (`NodeServices.layer`) | Changed | `Effect.provide(() => main(), NodeServices.layer)` |
| `FileSystem` | Unchanged by design | `FileSystem.readFileString(path)`, from `effect/FileSystem` |
| `FileSystem.open` (a scoped `File`) | Changed | `using file = FileSystem.open(path)` |
| `FileSystem.layerNoop` | Changed | Its methods return values, not Effects |
| `Path` | Unchanged by design | `Path.join(a, b)`; `Path` joins R (D41) |
| `Terminal`, `Stdio` | Unchanged by design | `Terminal.readLine()` |
| Child processes | Unchanged by design | `ChildProcessSpawner.string(ChildProcess.make("git", ["status"]))`; the command is data (D24) |
| `KeyValueStore` | Unchanged by design | From `effect/persistence` |
| Sockets | Changed | `using` for a scoped reader |
| Workers | Unchanged by design | EffectScript handlers |

#### C22. HTTP

| Feature | In EffectScript | Form |
| --- | --- | --- |
| `HttpClient` | Unchanged by design | `HttpClient.get(url)` runs |
| Response bodies | Unchanged by design | `response.json()` |
| Client middleware | Unchanged by design | `.pipe` over a client value |
| `HttpRouter.add` handlers | Unchanged by design | A handler function, or a program (D39) |
| Serving | Changed | `NodeRuntime.runMain(() => Layer.launch(HttpRouter.serve(routes)))` |
| `HttpApi` definitions, middleware, security | Unchanged by design | Values |
| `HttpApi` handlers | Unchanged by design | EffectScript functions |
| `HttpApiClient` | Unchanged by design | Calls that run |

#### C23. SQL

| Feature | In EffectScript | Form |
| --- | --- | --- |
| `SqlClient` | Unchanged by design | `const sql = SqlClient.SqlClient()` |
| `` sql`…` `` statements | Unchanged by design | Data until called (D24): `` sql`SELECT * FROM users`() `` runs it, and `.stream` stays reachable |
| Fragments (`sql.in`, `sql.insert`, `sql.and`) | Unchanged by design | The same calls |
| `sql.withTransaction` | Changed | `sql.withTransaction(() => insertAll())` |
| `SqlSchema`, `SqlResolver`, `SqlModel`, migrations | Unchanged by design | The same modules |
| Driver layers read from config | Unchanged by design | `Config` arguments are data (D24) |

#### C24. CLI

| Feature | In EffectScript | Form |
| --- | --- | --- |
| `Command.make`, `withSubcommands`, `withHandler` | Unchanged by design | Data (D24), declared at the top level |
| Flags and arguments | Unchanged by design | Values |
| Prompts | Unchanged by design | Data until called |
| Running a command | Changed | Inside `main`, handed to a runner |

#### C25. AI

| Feature | In EffectScript | Form |
| --- | --- | --- |
| `LanguageModel.generateText`, `generateObject` | Unchanged by design | Calls that run |
| `LanguageModel.streamText` | Unchanged by design | A `Stream` |
| `Tool.make`, `Toolkit.make` | Unchanged by design | Data (D24) |
| Tool handlers | Changed | `toolkit.toLayer(() => ({ search: ({ q }) => runSearch(q) }))` |
| `Chat` | Unchanged by design | `Chat.empty()` |
| `Effect.withExecutionPlan` | Changed | `Effect.withExecutionPlan(() => program(), plan)` |
| `AiError` reasons | Changed | Narrow `e.reason._tag` in `catch` |
| Provider layers read from config | Unchanged by design | `Config` arguments are data (D24) |

#### C26. Cluster, workflow, and RPC

| Feature | In EffectScript | Form |
| --- | --- | --- |
| `Rpc.make`, `RpcGroup.make` | Unchanged by design | Values |
| `group.toLayer(handlers)` | Unchanged by design | An object of EffectScript handlers (D39) |
| RPC clients and servers | Changed | `using client = RpcClient.make(UserRpcs)` |
| Entities | Unchanged by design | `Counter.client()` |
| Workflows | Unchanged by design | Bodies are EffectScript functions (D42) |
| Activities | Unchanged by design | Data until called (D24) |
| `DurableClock.sleep`, `DurableDeferred.await` | Unchanged by design | Calls that run |
| Replay | Unchanged by design | Each execution is a fresh program run; held continuations end with it (D42) |

#### C27. Effect 3 features that Effect 4 removed

| Feature | In EffectScript | Form |
| --- | --- | --- |
| `Micro` | Unchanged by design | Effect 4 has none; Effect 3 code is foreign code (Chapter 10) |
| `Supervisor` | Unchanged by design | `FiberSet` and `fork` |
| STM | Unchanged by design | `Effect.tx` with `TxRef` |
| `FiberRef`, `Effect.locally` | Changed | `Effect.provideService(() => program(), References.CurrentLogLevel, "Debug")` |
| `Either` | Unchanged by design | `Result` |
| `Mailbox` | Unchanged by design | `Queue` |
| `Runtime<R>` | Changed | `ManagedRuntime`, or `run*With` under D38 |
| `Effect.Service`, `Effect.Tag` accessors | Changed | Abstract service classes, called as `Users.find(id)` |
| `withConcurrency`, `withClock`, `withRandom` | Unchanged by design | Per-call options and `References` |
