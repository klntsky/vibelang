# Managing Layers

A layer describes how to construct services, including their dependencies, initialization errors, and resources. Build the application graph once around an entry point so its services share one managed lifetime.

[Upstream page](https://effect.website/docs/v4/requirements-management/layers)

## Describing a dependency graph

```ts
service Settings {
  prefix: string
}
service Printer {
  print(text: string): void!!
}
service Reports {
  render(title: string): string!!
}

function report(title: string) {
  return Reports.render(title)
}
// string!!Reports
```

The implementation graph on this page has `Printer` depend on `Settings`, and `Reports` depend on both. `Layer.Layer<Out, E, In>` describes the services produced, construction failures, and construction requirements. The global `Layer` namespace provides the constructors and that type.

## Preventing dependency leakage

```ts
layer PrinterLive: Printer {
  const settings = Settings()
  print(text) {
    log(`${settings.prefix}: ${text}`)
  }
}
// Layer<Printer, never, Settings>
```

`layer PrinterLive: Printer { ... }` builds a `Printer`. The statements before the members run once, when the layer builds, and the members form the implementation; a value member is written `readonly name = value`. The declaration is short for `Layer.effect(Printer, () => { ...; return { print(text) { ... } } })`, and a layer without statements is `Layer.succeed`. `Settings()` runs once when the layer builds, and the method captures that implementation, so callers need `Printer` without also needing `Settings`. Looking up a service inside a method instead makes it a call-time requirement, which must appear in the member's full contract.

Keep method requirements at `never` when dependencies are implementation details. This lets a test provide a simple fake without also recreating the production graph.

## Constructing service layers

```ts
layer SettingsLive: Settings { readonly prefix = "reports" }
// Layer<Settings, never, never>

layer ReportsLive: Reports {
  const settings = Settings()
  const printer = Printer()
  render(title) {
    printer.print("Rendering report")
    return `${settings.prefix}/${title}`
  }
}
// Layer<Reports, never, Settings | Printer>
```

Use `Layer.succeed` for an existing implementation and `Layer.effect` for a factory that performs Effect operations. `Layer.sync` accepts an eligible synchronous constructor when no Effect work is required. Constructing a layer value does not run its initialization program.

A factory can also register `Effect.addFinalizer` or acquire with `Effect.acquireRelease`. `Layer.effect` supplies its construction scope and removes `Scope` from the layer's input requirements. Acquire a returned service's resources without `using`: a lexical `using` would release them when the factory returns.

## Merging independent layers

```ts
const Merged = Layer.merge(SettingsLive, PrinterLive)
// Layer<Settings | Printer, never, Settings>

const Many = Layer.mergeAll(SettingsLive, PrinterLive, ReportsLive)
// Layer<Settings | Printer | Reports, never, Settings | Printer>
```

Merging combines outputs and inputs, and can initialize independent branches concurrently. It does not automatically feed one sibling's output into another's input. `Merged` therefore still needs `Settings` to build `PrinterLive`, even though another branch produces `Settings`.

## Wiring dependent layers

```ts
const PrinterReady = Layer.provide(PrinterLive, SettingsLive)
// Layer<Printer, never, never>

const PrinterAndSettings = Layer.provideMerge(PrinterLive, SettingsLive)
// Layer<Printer | Settings, never, never>

const ReportsReady = Layer.provide(ReportsLive, PrinterAndSettings)
// Layer<Reports, never, never>

const AppLive = wire(ReportsLive, PrinterLive, SettingsLive)
// Layer<Reports | Printer | Settings, never, never>
```

`Layer.provide(consumer, dependencies)` feeds dependency outputs into the consumer and retains the consumer's outputs. `provideMerge` also retains the supplied dependency outputs. `wire` derives this ordering from the layer contracts and rejects duplicate providers (EK3202) and cycles (EK3203).

Inputs no argument provides stay in the resulting layer's `In`, so a partial graph is valid. Missing providers are reported with their dependency paths where a runner or contract requires them to be supplied (EK3201). An argument whose type does not name its outputs, such as `Layer.Layer<any>`, is EK3204; compose it with `Layer.provide` instead.

Reuse the same layer value when several paths need a shared instance. Reconstructing equivalent layer values does not give them the same memoization identity.

## Providing and building layers

```ts
function main() {
  provide AppLive {
    const first = report("Inventory")
    const second = report("Forecast")
    return [first, second]
  }
}
// string[]!!

function withBuiltContext() {
  return Effect.scoped(() => {
    const context = Layer.build(AppLive)
    provide context { return report("Inventory") }
  })
}
```

`provide AppLive { ... }` provides the layer to its statements: it builds the layer once, every call inside shares the implementations, and the layer's resources are released when the block exits. The statements are code of the enclosing function, so the `return` inside returns from `main`. The block is short for `Effect.provide(() => { ... }, AppLive)`, and like it accepts a layer, an array of layers, or a `Context`. Separate provision boundaries acquire separately; use `ManagedRuntime` when several entry points need one application lifetime.

`Layer.build` produces a `Context` and requires a `Scope`, supplied here by `Effect.scoped`. Consume resource-owning services inside that scope. For an application represented entirely by a layer, `Layer.launch(AppLive)` builds it and waits until interruption, keeping its resources alive.

## Observing initialization and recovering failures

```ts
error InitializationFailed

const ReportsUnavailable = Layer.effect(Reports, () => {
  throw new InitializationFailed()
})

layer ReportsFallback: Reports {
  render(title) { return `Offline report: ${title}` }
}

const ObservedReports = ReportsUnavailable
  |> Layer.tap(%, () => log("Reports initialized"))
  |> Layer.tapError(%, (error) => log(`Reports initialization failed: ${error._tag}`))

const ResilientReports = Layer.catch(ObservedReports, () => ReportsFallback)
```

`Layer.tap` observes successful acquisition; `Layer.tapError` observes typed construction failures. Observers can add their own errors or requirements, so use infallible logging when you want to preserve the original signature. `Layer.catch` recovers construction errors with another layer; it does not handle errors from later service calls.

If a larger graph fails to initialize, acquired resources still release. Keep initialization diagnostics focused on service names and safe failure tags rather than configuration secrets.

## Declaring service factories and dependencies

```ts
service Banner {
  text(title: string): string!!

  static make(): Banner!!Settings {
    const settings = Settings()
    return { text: (title) => `${settings.prefix}: ${title}` }
  }

  static readonly layerWithoutDependencies = Layer.effect(Banner, Banner.make())

  static readonly layer = Layer.provide(Banner.layerWithoutDependencies, SettingsLive)
}
```

Static members stay on the service class. `static make()` builds the implementation, taking the place of Effect 4's service `make` option. `layerWithoutDependencies` exposes a factory requiring `Settings`; `static readonly layer` holds the chosen default layer. The compiler does not generate a layer automatically.

A constant factory can use `Layer.succeed`; synchronous factories can use `Layer.sync`; effectful and scoped factories use `Layer.effect`. A service may expose several layer values for different configurations or runtimes.

## Replacing services in tests

```ts
layer SettingsTest: Settings { readonly prefix = "test" }
const BannerWithTestSettings = Layer.provide(Banner.layerWithoutDependencies, SettingsTest)
layer BannerTest: Banner { text(title) { return `fixture/${title}` } }

function testBannerDependency() {
  provide BannerWithTestSettings { return Banner.text("Orders") }
}

function testBannerDirectly() {
  provide BannerTest { return Banner.text("Orders") }
}
```

Replace a dependency to exercise the real factory, or replace the service itself to isolate its consumers. A layer that already includes production dependencies must be changed at its exposed factory boundary if the test needs different construction inputs. Small local overrides can use `Effect.provideService` directly.

## Coming from Effect

Abstract services and static layer fields replace hand-written key classes and generator-based factories. The underlying Layer graph, memoization, and resource semantics remain Effect 4's.
