# Welcome to Effeck

Effeck gives every function a contract for its result, expected errors, and required services. Use it to write application logic with TypeScript syntax and Effect's execution, resource, and concurrency tools.

## Why typed programs help

```ts
class EmptyName extends TaggedError("EmptyName")<{}> {}

function greet(name: string) {
  if (name.trim() === "") throw new EmptyName()
  return `Welcome, ${name.trim()}`
}
// Effect<string, EmptyName, never>
```

The contract tells a caller that `greet` returns a string, can fail with `EmptyName`, and needs no services. The compiler infers all three channels from the body. Expected errors remain visible as they pass through callers.

## Programs as values

```ts
function announce(name: string) {
  Effect.log(greet(name))
}

const announcement = () => announce("Mira")

function twice() {
  announcement()
  announcement()
}
```

A function value describes work. Calling it runs that work in the current program; calling it again starts a fresh run. Pass `() => announce("Mira")` to an operator when that operator must control when the work starts.

## Errors, concurrency, resources, and telemetry

```ts
function prepare() {
  return Effect.all([
    () => {
      Effect.sleep("20 millis")
      return "catalog"
    },
    () => {
      Effect.log("checking inventory")
      return "inventory"
    }
  ], { concurrency: 2 })
}

function main() {
  return Effect.withSpan(() => prepare(), "startup")
}
```

`Effect.all` starts these two programs concurrently and preserves their result order. An unhandled failure interrupts the other branch and waits for cleanup. Use `try`/`catch` for expected failures, `using` for acquired resources, and `finally` for cleanup on every exit, including interruption. Effect also supplies schedules, streams, schema validation, configuration, logs, metrics, and tracing.

## Working with coding assistants

Give an assistant the Effeck rules and the version of Effect your project uses. Ask it to write ordinary functions, pass programs as zero-argument functions, and check inferred errors and services. Examples copied from TypeScript Effect need adaptation: `.effeck` files use calls instead of `yield*`, and platform access goes through Effect modules.

Keep feedback concrete: check the contract, test a failure, and test cleanup when a request is interrupted. An assistant needs the same boundaries as a human author, especially around plain JavaScript libraries.

## Your learning path

1. Read [The Effect Type](getting-started/003-the-effect-type.md) to learn the three contract channels.
2. Follow [Installation](getting-started/004-installation.md) for dependencies and compiler setup.
3. Write functions with [Creating Effects](getting-started/005-creating-effects.md), then choose a boundary in [Running Effects](getting-started/009-running-effects.md).
4. Learn [expected failures and defects](error-management/006-two-error-types.md).
5. Add [concurrency](concurrency/007-basic-concurrency.md) and [resource management](resource-management/029-introduction.md) as your application needs them.

Use the [guide index](013-getting-started.md) to return to a topic. The Effect API reference explains library operators; these pages explain how to call them from Effeck.

## Coming from Effect

An ordinary function replaces `Effect.gen` sequencing. Functions, typed errors, services, and resources still run on Effect 4; use Effeck's call rules when reading the examples.

[Upstream Effect page](https://effect.website/docs/v4/onboarding)
