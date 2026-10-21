# Managing Layers

A layer describes how to build services: their dependencies, initialization errors, and resources. Build the application graph once around an entry point so its services share one managed lifetime.

[Upstream page](https://effect.website/docs/v4/requirements-management/layers)

## Describing a dependency graph

```vibe
// run: provide Reports = { render: (title) => `report/${title}` } { Console.log(report("Inventory")) }
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
// Output: report/Inventory
```

On this page, `Printer` depends on `Settings`, and `Reports` on both. `Layer.Layer<Out, E, In>` gives the services produced, construction failures, and construction requirements; the global `Layer` namespace holds the constructors and that type.

## Preventing dependency leakage

```vibe
// run: provide Settings = { prefix: "test" } { provide PrinterLive { Printer.print("ready") } }
const PrinterLive = Layer.effect(Printer) {
  const settings = Settings()
  return {
    print(text) {
      Console.log(`${settings.prefix}: ${text}`)
    }
  }
}
// Layer<Printer, never, Settings>
// Output: test: ready
```

`Layer.effect(Printer) { ... }` builds a `Printer`: the block is the layer's effect, passed to Effect's `Layer.effect` through block params. Its statements, `Settings()` included, run once when the layer builds. The returned object is the implementation; the service types its members, so `print(text)` needs no annotation. The method captures the `Settings` implementation, so callers need `Printer` but not `Settings`. A layer without setup is `Layer.succeed(Printer, { ... })`. Looking up a service inside a method instead makes it a call-time requirement, which the member's full contract must list.

Keep method requirements at `never` when dependencies are implementation details, so a test can provide a simple fake without recreating the production graph.

## Constructing service layers

```vibe
// run: provide SettingsLive { provide PrinterLive { provide ReportsLive { Console.log(report("Inventory")) } } }
const SettingsLive = Layer.succeed(Settings, {
  prefix: "reports"
})
// Layer<Settings, never, never>

const ReportsLive = Layer.effect(Reports) {
  const settings = Settings()
  const printer = Printer()
  return {
    render(title) {
      printer.print("Rendering report")
      return `${settings.prefix}/${title}`
    }
  }
}
// Layer<Reports, never, Settings | Printer>
// Output: reports: Rendering report
// Output: reports/Inventory
```

Use `Layer.succeed` for an existing implementation or constant factory, `Layer.effect` for a factory that performs Effect operations, and `Layer.sync` for an eligible synchronous constructor that needs no Effect work. Constructing a layer value doesn't run its initialization program.

A factory can register `Effect.addFinalizer` or acquire with `Effect.acquireRelease`; `Layer.effect` supplies the construction scope and removes `Scope` from the layer's input requirements. Acquire a returned service's resources without `using`: a lexical `using` releases them when the factory returns.

## Merging independent layers

```vibe
// run: provide SettingsLive { provide PrinterLive { provide Merged { Printer.print("merged") }; provide Many { Console.log(report("many")) } } }
const Merged = Layer.merge(SettingsLive, PrinterLive)
// Layer<Settings | Printer, never, Settings>

const Many = Layer.mergeAll(SettingsLive, PrinterLive, ReportsLive)
// Layer<Settings | Printer | Reports, never, Settings | Printer>
// Output: reports: merged
// Output: reports: Rendering report
// Output: reports/many
```

Merging combines outputs and inputs and can initialize independent branches concurrently. It doesn't feed one sibling's output into another's input, so `Merged` still needs `Settings` to build `PrinterLive`, though a sibling produces it.

## Wiring dependent layers

```vibe
// run: provide PrinterReady { Printer.print("ready") }; provide PrinterAndSettings { Console.log(Settings().prefix) }; provide ReportsReady { Console.log(report("wired")) }; provide AppLive { Console.log(report("app")) }
const PrinterReady = Layer.provide(PrinterLive, SettingsLive)
// Layer<Printer, never, never>

const PrinterAndSettings = Layer.provideMerge(PrinterLive, SettingsLive)
// Layer<Printer | Settings, never, never>

const ReportsReady = Layer.provide(ReportsLive, PrinterAndSettings)
// Layer<Reports, never, never>

const AppLive = wire(ReportsLive, PrinterLive, SettingsLive)
// Layer<Reports | Printer | Settings, never, never>
// Output: reports: ready
// Output: reports
// Output: reports: Rendering report
// Output: reports/wired
// Output: reports: Rendering report
// Output: reports/app
```

`Layer.provide(consumer, dependencies)` feeds dependency outputs into the consumer and keeps the consumer's outputs; `provideMerge` also keeps the dependency outputs. `wire` derives this ordering from the layer contracts and rejects duplicate providers (EK3202) and cycles (EK3203).

Inputs no argument provides stay in the resulting layer's `In`, so a partial graph is valid. At a runner, missing providers report EK3201 with the unsatisfied inputs. A written contract that excludes a remaining requirement reports EK2002. An argument whose type doesn't name its outputs, such as `Layer.Layer<any>`, is EK3204; compose it with `Layer.provide` instead.

Reuse one layer value when several paths need a shared instance: equivalent reconstructed layer values have different memoization identities.

## Providing and building layers

```vibe
// run: Console.log(JSON.stringify(main())); Console.log(withBuiltContext())
function main() {
  provide AppLive {
    const first = report("Inventory")
    const second = report("Forecast")
    return [first, second]
  }
}
// string[]!!

function withBuiltContext() {
  return Effect.scoped {
    const context = Layer.build(AppLive)
    provide context { return report("Inventory") }
  }
}
// Output: reports: Rendering report
// Output: reports: Rendering report
// Output: ["reports/Inventory","reports/Forecast"]
// Output: reports: Rendering report
// Output: reports/Inventory
```

`provide AppLive { ... }` builds the layer once, shares its implementations with every call inside, and releases its resources when the block exits. The statements belong to the enclosing function, so the `return` inside returns from `main`. The block is short for `Effect.provide(() => { ... }, AppLive)` and, like it, accepts a layer, an array of layers, or a `Context`. Separate provision boundaries acquire separately; use `ManagedRuntime` when several entry points need one application lifetime.

`Layer.build` produces a `Context` and requires a `Scope`, here from `Effect.scoped`; consume resource-owning services inside that scope. For an application that is entirely a layer, `Layer.launch(AppLive)` builds it and keeps its resources alive until interruption.

## Observing initialization and recovering failures

```vibe
// run: provide ResilientReports { Console.log(report("Inventory")) }
error InitializationFailed

const ReportsUnavailable = Layer.effect(Reports) {
  throw new InitializationFailed()
}

const ReportsFallback = Layer.succeed(Reports, {
  render(title) { return `Offline report: ${title}` }
})

const ObservedReports = ReportsUnavailable
  |> Layer.tap(%, () => Console.log("Reports initialized"))
  |> Layer.tapError(%, (error) => Console.log(`Reports initialization failed: ${error._tag}`))

const ResilientReports = Layer.catch(ObservedReports, () => ReportsFallback)
// Output: Reports initialization failed: InitializationFailed
// Output: Offline report: Inventory
```

`Layer.tap` observes successful acquisition; `Layer.tapError` observes typed construction failures. Observers can add their own errors or requirements, so log infallibly to keep the original signature. `Layer.catch` recovers construction errors with another layer, not errors from later service calls.

When a larger graph fails to initialize, acquired resources still release. Initialization diagnostics should name services and safe failure tags, not configuration secrets.

## Declaring service factories and dependencies

```vibe
// run: provide Banner.layer { Console.log(Banner.text("Orders")) }
service Banner {
  text(title: string): string!!

  static make(): Banner!!Settings {
    const settings = Settings()
    return { text: (title) => `${settings.prefix}: ${title}` }
  }

  static readonly layerWithoutDependencies = Layer.effect(Banner, Banner.make())

  static readonly layer = Layer.provide(Banner.layerWithoutDependencies, SettingsLive)
}
// Output: reports: Orders
```

Static members stay on the service class. `static make()` builds the implementation, replacing Effect 4's service `make` option. `layerWithoutDependencies` exposes a factory that requires `Settings`; `static readonly layer` holds the chosen default layer. The compiler generates no layer, and a service can expose several layer values for different configurations or runtimes.

## Replacing services in tests

```vibe
// run: Console.log(JSON.stringify([testBannerDependency(), testBannerDirectly(), testBannerWithInlineSettings()]))
const SettingsTest = Layer.succeed(Settings, {
  prefix: "test"
})
const BannerWithTestSettings = Layer.provide(Banner.layerWithoutDependencies, SettingsTest)
const BannerTest = Layer.succeed(Banner, {
  text(title) { return `fixture/${title}` }
})

function testBannerDependency() {
  provide BannerWithTestSettings { return Banner.text("Orders") }
}

function testBannerDirectly() {
  provide BannerTest { return Banner.text("Orders") }
}

function testBannerWithInlineSettings() {
  provide Settings = { prefix: "inline" } {
    return Effect.provide(() => Banner.text("Orders"), Banner.layerWithoutDependencies)
  }
}
// Output: ["test: Orders","fixture/Orders","inline: Orders"]
```

Replace a dependency to exercise the real factory, or the service itself to isolate its consumers. If a test needs different construction inputs, change a layer that already includes production dependencies at its exposed factory boundary. For a small local override, `provide Key = value` provides one implementation without a layer. Put construction dependencies in an outer block so they exist while the inner layer builds: providers in one list supply the block together and don't feed one another's construction dependencies.

## Coming from Effect

Abstract services and static layer fields replace hand-written key classes and generator-based factories. The Layer graph, memoization, and resource semantics stay Effect 4's. `return Effect.provide(program, AppLive)` draws EK4836, whose quick fix writes `provide AppLive { return program }`.
