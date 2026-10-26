# Default Services

Effect gives the clock, random generation, console output, configuration sources, and tracing live defaults. VibeLang uses them, so using these adds no application service requirement.

[Upstream page](https://effect.website/docs/v4/requirements-management/default-services)

## Using the clock, random, and console

```vibe
// run: provide Random = { nextDoubleUnsafe: () => 0.25, nextIntUnsafe: () => 12 }, Clock = { ...Clock.Clock(), currentTimeMillis: () => 1000, currentTimeMillisUnsafe: () => 1000 } { startupMessage() }
function startupMessage() {
  const started = Clock.currentTimeMillis()
  const sample = Random.next()
  Console.log(`Started at ${started}; sample=${sample}`)
}
// Effect<void>
// Output: Started at 1000; sample=0.25
```

`Clock.currentTimeMillis` and `Random.next` are Effect values, so calling one runs it; `Console.log` returns an Effect, so its call runs too. Use these modules, not `Date.now()`, `Math.random()`, or the browser console, so tests and local overrides control the behavior.

Default references include `Clock.Clock`, `ConfigProvider.ConfigProvider`, `Console.Console`, `Random.Random`, `Tracer.Tracer`, and `Scheduler.Scheduler`; their modules need no import in `.vibe` files. These defaults are why the example's `R` is `never` though it reads time and random state.

```vibe
// run: provide ConfigProvider.layer(ConfigProvider.fromUnknown({})) { Console.log(configuredPort()) }
const Port = Config.withDefault(Config.Port("PORT"), 3000)

function configuredPort() {
  return Port()
}
// Effect<number, Config.ConfigError>
// Output: 3000
```

`Config.Port` and `Config.withDefault` build configuration data without reading the environment; `Port()` reads it through the default configuration provider. A call returning a `Config`, an `Exit`, or a Context key leaves it as data; calling the held value runs it.

## Replacing default implementations

```vibe
// run: Console.log(fixedRandomSample())
function fixedRandomSample() {
  return Effect.provideService(Random.next(), Random.Random, {
    nextDoubleUnsafe: () => 0.25,
    nextIntUnsafe: () => 12
  })
}
// Output: 0.25
```

`Effect.provideService` overrides a default reference for its program. The implementation has Effect 4's synchronous `Random` interface; both callbacks only return constants, so they qualify as plain JavaScript functions. Override any other default reference the same way, with a complete implementation.

An override holds across pauses, is inherited by new child fibers, and leaves unrelated programs unchanged. Prefer a module helper when it supplies the behavior without implementing a whole service.

```vibe
// run: Console.log(defaultLabel(17)); provide DisplaySettings = { prefix: "Order" } { Console.log(defaultLabel(17)) }
service DisplaySettings {
  prefix: string
  static readonly default: DisplaySettings = { prefix: "Ticket" }
}

function defaultLabel(id: number) {
  return `${DisplaySettings().prefix} ${id}`
}
// Effect<string>
// Output: Ticket 17
// Output: Order 17
```

A service with `static readonly default` compiles to a `Context.Reference`, so using it adds nothing to `R`; override it with `Effect.provideService` as above. Effect's own `Context.Reference(key, { defaultValue })` works unchanged and returns a key as data.

## Reproducible random computations

```vibe
// run: const pairs = reproduciblePairs(); Console.log(JSON.stringify(pairs.first) === JSON.stringify(pairs.second), pairs.first[0] !== pairs.first[1])
function pair() {
  return [Random.next(), Random.next()]
}

function reproduciblePairs() {
  const first = Random.withSeed(() => pair(), "documentation-seed")
  const second = Random.withSeed(() => pair(), "documentation-seed")
  return { first, second }
}
// Output: true true
```

Each `withSeed` call installs a new deterministic generator, so both pairs hold the same sequence. Reads in one seeded region advance the generator; they don't repeat one value. When a test depends on the sequence, keep consumption order stable: concurrent consumers can change which task gets which number.

## Coming from Effect

Default services behave as in Effect. Effect values such as `Random.next` run with parentheses; provision and seed helpers take a program such as `() => pair()`.
