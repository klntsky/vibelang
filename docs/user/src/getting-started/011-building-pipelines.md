# Building Pipelines

Use statements for dependent steps and `pipe` for a series of operators that control a program. Effeck's global `pipe` supports ordinary data transformations and Effect operators whose first input is a program.

## Compose reusable functions

```ts
function addMargin(n: number) { return n + 3 }
function double(n: number) { return n * 2 }

function priceLabel() {
  return pipe(9, addMargin, double, (n) => `Price: ${n}`)
}
// Effect<string, never, never>; "Price: 24"
```

Each stage takes the previous stage's result as its argument. Standalone functions remain reusable without adding methods to a value's prototype. Effect exports also support bundlers removing unused code when the bundler can analyze the module graph.

For program operators, start with the work to run:

```ts
function preparedPrice() {
  return pipe(
    () => priceLabel(),
    Effect.timeout("1 second"),
    Effect.withSpan("price-label")
  )
}
```

The program function delays `priceLabel` until the pipeline runs it. The `pipe` call returns the final success value. Wrap the whole pipeline in another function to keep it as reusable work.

## Map or replace a success

```ts
function adjustedPrice() {
  const price = 24
  return price + 5
}

function recordPrice() {
  Effect.log(`price=${adjustedPrice()}`)
  return "recorded"
}
```

Using the returned value replaces `Effect.map` for local transformations. A subsequent constant return replaces a successful result after earlier work finishes, as `Effect.as` does. Earlier failures still propagate, so replacing the success does not hide an error.

## Sequence dependent computations

```ts
class InvalidRate extends TaggedError("InvalidRate")<{}> {}

function discounted(price: number, percent: number) {
  if (percent < 0 || percent > 100) throw new InvalidRate()
  return price * (1 - percent / 100)
}

function total() {
  const price = adjustedPrice()
  const reduced = discounted(price, 12)
  return reduced + 2
}
// Effect<number, InvalidRate, never>
```

A call followed by another call expresses the dependency that `flatMap` or `andThen` handles in TypeScript Effect. Each call runs once, in order. A failure prevents later statements from executing unless you handle it.

## Integrate Option and Result

```ts
import { Option, Result } from "effect"

function positive(value: number) {
  const optional = value > 0 ? Option.some(value) : Option.none()
  return Effect.fromOption(optional)
}
// Effect<number, NoSuchElementError, never>

function validated(value: number) {
  const checked = value >= 0
    ? Result.succeed(value)
    : Result.fail("negative value")
  return Effect.fromResult(checked)
}
// Effect<number, string, never>
```

`Option` and `Result` are data values, so keep them intact when you want to inspect them. Explicit conversion runs them as computations: a missing option adds `NoSuchElementError`, and a failed result adds its error type. Calling a constructor such as `Option.some` builds the container.

## Observe intermediate results

```ts
function quotedTotal() {
  const amount = total()
  Effect.log(`quoted amount=${amount}`)
  return amount
}
```

The local variable remains available after logging, giving the behavior of `Effect.tap`. An observation that fails also fails the enclosing function. Use `try`/`catch` around the observation only when ignoring that failure is intentional.

## Collect multiple computations

```ts
function summary() {
  const details = Effect.all({
    price: () => total(),
    label: () => priceLabel()
  }, { concurrency: 2 })
  return `${details.label}, total=${details.price}`
}
```

`Effect.all` preserves the input's tuple or object shape. It runs sequentially by default; this example explicitly starts up to two computations at once. If a branch fails, the combined computation fails and concurrent siblings are interrupted and cleaned up.

For a dynamic collection, use `Effect.forEach(values, (value) => work(value), { concurrency: 4 })`. You can also build a collection of programs first: `const jobs = values.map((value) => () => work(value))`, then `Effect.all(jobs, { concurrency: 4 })`. Iterables and records of zero-argument programs convert to Effects where the combinator expects them. A synchronous array callback cannot call work that pauses or has expected failures; returning a program delays that work.

## Avoid discarded programs

```ts
function saveNotice() {
  const notice = () => Effect.log("saved")
  notice()
  Effect.log("finished")
}
```

A call in an Effeck statement runs even when its success value is ignored. A function value such as `notice` describes work and needs a call to execute. Merely writing `() => Effect.log("saved")` creates a value that no runner or caller uses.

## Coming from Effect

Statements replace `map`, `flatMap`, `andThen`, and `tap` in most functions. Use the global `pipe` with a program first for retry, timeout, provisioning, and tracing; an ordinary program function has no Effect `.pipe` method.

[Upstream Effect page](https://effect.website/docs/v4/getting-started/building-pipelines)
