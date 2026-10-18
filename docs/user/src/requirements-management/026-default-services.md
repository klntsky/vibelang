# Default Services

Clock, random generation, console output, configuration sources, and tracing have live defaults in Effect. VibeLang uses those same defaults, so normal access does not add an application service requirement.

[Upstream page](https://effect.website/docs/v4/requirements-management/default-services)

## Using the clock, random, and console

```vibe
// run: provide Random = { nextDoubleUnsafe: () => 0.25, nextIntUnsafe: () => 12 }, Clock = { ...Clock.Clock(), currentTimeMillis: () => 1000, currentTimeMillisUnsafe: () => 1000 } { startupMessage() }
function startupMessage() {
  const started = Clock.currentTimeMillis()
  const sample = Random.next()
  Console.log(`Started at ${started}; sample=${sample}`)
}
// void!!
// Output: Started at 1000; sample=0.25
```

`Clock.currentTimeMillis` and `Random.next` are Effect values; calling them runs them. `Console.log` returns an Effect, so its call runs too. Use these modules instead of `Date.now()`, `Math.random()`, or the browser console so tests and local overrides can control the behavior.

Default references include `Clock.Clock`, `ConfigProvider.ConfigProvider`, `Console.Console`, `Random.Random`, `Tracer.Tracer`, and `Scheduler.Scheduler`. Their module names are available without imports in `.vibe` files. Their defaults explain why the example's `R` is `never` even though it reads time and random state.

```vibe
// run: provide ConfigProvider.layer(ConfigProvider.fromUnknown({})) { Console.log(configuredPort()) }
const Port = Config.withDefault(Config.Port("PORT"), 3000)

function configuredPort() {
  return Port()
}
// number!Config.ConfigError!
// Output: 3000
```

`Config.Port` and `Config.withDefault` build configuration data; they do not read the environment. Calling `Port()` reads it through the default configuration provider. Calls returning a `Config`, an `Exit`, or a Context key leave it as data; calling the held value runs it.

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

`Effect.provideService` overrides a default reference for the supplied program. This implementation has Effect 4's synchronous `Random` interface; both callbacks only return constants and qualify as plain JavaScript functions. Use the same provision mechanism with a complete implementation of any other default reference.

An override remains in effect across pauses and is inherited by new child fibers. It does not change unrelated programs. Prefer module helpers when they supply the behavior you need without implementing a whole service.

```vibe
// run: Console.log(defaultLabel(17)); provide DisplaySettings = { prefix: "Order" } { Console.log(defaultLabel(17)) }
service DisplaySettings {
  prefix: string
  static readonly default: DisplaySettings = { prefix: "Ticket" }
}

function defaultLabel(id: number) {
  return `${DisplaySettings().prefix} ${id}`
}
// string!!
// Output: Ticket 17
// Output: Order 17
```

A service with `static readonly default` compiles to a `Context.Reference`, so using it adds nothing to `R`. Override it with `Effect.provideService` as above. Effect's own `Context.Reference(key, { defaultValue })` also works unchanged and returns a key as data.

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

Each `withSeed` call installs a new deterministic generator, so both pairs contain the same sequence. Reads within one seeded region advance that generator rather than returning one fixed value. Keep random consumption order stable when a test depends on the sequence; concurrent consumers can change which task receives which number.

## Coming from Effect

Default service behavior is unchanged. Effect values such as `Random.next` run with parentheses, while provision and seed helpers receive a program such as `() => pair()`.
