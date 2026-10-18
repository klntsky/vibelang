# Managing Services

A service gives a program a named capability without passing an implementation through every function. VibeLang declares a service with `service`, an abstract class whose members state their contracts, and infers its use in the `R` channel.

[Upstream page](https://effect.website/docs/v4/requirements-management/services)

## Declaring typed service keys

```vibe
// run: provide Sequence = { next: () => 17 }, Reporter = { write: (message) => Console.log(message) } { Reporter.write(`ticket-${Sequence.next()}`) }
service Sequence {
  next(): number!!
}

service Reporter {
  write(message: string): void!!
}
// Output: ticket-17
```

A service class has no superclass or type parameters, and all its instance members are abstract. Methods state full contracts because they have no implementation from which to infer one; `abstract readonly` value members are also allowed. Static fields can hold layers or other helpers.

The compiler gives each service nominal identity and derives its key from package, module path, and class name. Two services with identical members remain different dependencies. Implement a service with an object literal or a class that `implements` it; do not construct, extend, or use `instanceof` on the service class.

## Requesting and supplying implementations

```vibe
// run: Console.log(fixedLabel()); provide Sequence = { next: () => 23 } { Console.log(nextLabel(), readImplementation()) }
function nextLabel() {
  return `ticket-${Sequence.next()}`
}
// string!!Sequence

function fixedLabel() {
  return Effect.provideService(nextLabel(), Sequence, {
    next: () => 17
  })
}
// string!!

function readImplementation() {
  const sequence = Sequence()
  return sequence.next()
}
// number!!Sequence
// Output: ticket-17
// Output: ticket-23 23
```

`Sequence.next()` looks up the service and runs its method. `Sequence()` returns the implementation itself, which is useful for capturing it or reading value members. A runner needs a program whose remaining requirements have been supplied.

```vibe
// run: provide Settings = { baseUrl: "https://example.test" } { Console.log(endpoint("tickets")) }
service Settings {
  baseUrl: string
}

function endpoint(path: string) {
  return `${Settings().baseUrl}/${path}`
}
// string!!Settings
// Output: https://example.test/tickets
```

Value members are read from the implementation, as `Settings().baseUrl`. Calling a member through a service adds that service to `R` whatever the member returns: even `Path.join(a, b)`, from Effect's `Path` service, returns a plain string and requires `Path`. Effect's package members and packages matching `compilerOptions.vibelang.trustedPackages` are trusted. Other plain TypeScript members need a boundary or `trusted(impl, [Errors])`, or the call is EK4401. A member that is itself an Effect value runs when called, as in `Terminal.readLine()`.

A declared service member wins over the key's own members, including `pipe`, `use`, `key`, and `call`. A static member with the same name as a service member is error EK3006.

`provideService` removes this dependency only within its supplied program. Nested overrides are local, and a captured implementation remains the same object across later overrides or pauses. Forked fibers inherit the context present when they start.

## Extracting service shapes

```vibe
type SequenceImplementation = Context.Service.Shape<typeof Sequence>
```

`Context.Service.Shape` is Effect 4's utility for extracting the implementation associated with a key. A service compiles to an interface for its implementation and a class whose static side is a `Context.Key`. The utility therefore works on `typeof Sequence`; using `Sequence` as a type also names the implementation, so `implements Sequence` checks its methods. The generated key keeps the requirement nominal.

Normal providers already infer and check their implementation from the key argument, so this utility is unnecessary for most layers. When crossing a TypeScript boundary, it describes the emitted implementation's Effect-returning methods.

## Combining multiple dependencies

```vibe
// run: Console.log(JSON.stringify(testAnnouncement()))
function announceTicket() {
  const label = nextLabel()
  Reporter.write(`Created ${label}`)
  return label
}
// string!!Sequence | Reporter

const SequenceTest = Layer.succeed(Sequence, {
  next() { return 17 }
})
const ReporterTest = Layer.succeed(Reporter, {
  write(_message) {}
})

function testAnnouncement() {
  provide wire(SequenceTest, ReporterTest) { return announceTicket() }
}
// Output: "ticket-17"
```

A union in `R` means both capabilities are needed. Layers let you supply the group together; `wire` composes their graph and rejects duplicate providers and cycles. Inputs no argument provides remain requirements of the resulting layer; a missing provider is reported where a runner or contract requires those requirements to be satisfied. Direct service provision can also be nested when a small local override is enough.

```vibe
// run: Console.log(testWithContext())
const testContext = Context.add(
  Context.make(Sequence, { next: () => 17 }),
  Reporter,
  { write: (_message) => {} }
)

function testWithContext() {
  provide testContext { return announceTicket() }
}
// Output: ticket-17
```

A `Context` holds implementations that already exist, while a `Layer` describes how to build them. A `provide` block, like `Effect.provide`, accepts either. Keep resource-owning implementations within the lifetime that created them.

## Requesting optional services

```vibe
// run: Console.log(optionalAnnouncement("hello")); provide Reporter = { write: (message) => Console.log(message) } { Console.log(optionalAnnouncement("hello")) }
function optionalAnnouncement(message: string) {
  const reporter = Effect.serviceOption(Reporter)
  if (reporter.isNone()) return false
  reporter.value.write(message)
  return true
}
// boolean!!
// Output: false
// Output: hello
// Output: true
```

`serviceOption` returns `None` when an application service is absent and `Some(implementation)` when it is present. The lookup does not add that service to `R`. Calling a found implementation still carries any method errors or call-time requirements declared by the service.

## Handling dependencies between services

```vibe
// run: provide ReportSettings = { prefix: "audit" } { provide ReporterLive { Reporter.write("ready") } }
service ReportSettings {
  prefix: string
}

const ReporterLive = Layer.effect(Reporter) {
  const settings = ReportSettings()
  return {
    write(message) {
      Console.log(`${settings.prefix}: ${message}`)
    }
  }
}
// Layer<Reporter, never, ReportSettings>
// Output: audit: ready
```

Capture construction dependencies in the layer factory. Calling `Reporter.write` then requires only `Reporter`, because the implementation retains `settings`. Looking up `ReportSettings()` inside `write` instead would create a call-time requirement that its declared `R = never` rejects.

## Coming from Effect

Abstract classes replace hand-written `Context.Service` declarations in `.vibe` files. Call service methods directly, and use `ServiceName()` where a TypeScript generator would yield the key to obtain its implementation.
