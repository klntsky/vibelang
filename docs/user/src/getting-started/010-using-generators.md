# Sequencing Effects

VibeLang sequences computations with ordinary functions and calls, covering the control-flow tasks `Effect.gen` handles in TypeScript; `function*` has a separate role.

## Sequence computations

```vibe
// run: Console.log(Effect.provide(invoice(3), ConfigProvider.layer(ConfigProvider.fromUnknown({ UNIT_PRICE: 4 }))))
function invoice(quantity: number) {
  const unitPrice = Config.Finite("UNIT_PRICE")()
  const subtotal = quantity * unitPrice
  Console.log(`subtotal=${subtotal}`)
  return subtotal
}
// Output: subtotal=12
// Output: 12
```

Each call completes before the next statement runs. `Config.Finite("UNIT_PRICE")` builds a descriptor; the following `()` loads the setting and returns a number, and a loading failure stops the function. `.vibe` files have no `Effect.gen` or `yield*`.

## Relate calls to async functions

```vibe
// run: Console.log(greeting())
function delayedLabel(name: string) {
  sleep("30 millis")
  return name.toUpperCase()
}

function greeting() {
  const label = delayedLabel("sora")
  return `Hello ${label}`
}
// Output: Hello SORA
```

`greeting` waits for the callee without `async` or `await`; the computation can pause and keeps typed errors, service requirements, and interruption support. At a promise-based framework boundary, run the exported function through a runner from `.ts`.

## Branches and loops

```vibe
// run: Console.log(reportEven([1, 2, 3, 4, -1, 6]))
function reportEven(values: ReadonlyArray<number>) {
  let count = 0
  for (const value of values) {
    if (value < 0) break
    if (value % 2 !== 0) continue
    Console.log(`even=${value}`)
    count += 1
  }
  return count
}
// Output: even=2
// Output: even=4
// Output: 2
```

Conditions, local variables, `break`, `continue`, and early returns keep JavaScript's evaluation order. This loop runs sequentially; for overlapping iterations, use `Effect.forEach` with an explicit concurrency limit.

## Raise failures and stop early

```vibe
// run: Console.log(optionalTitle(undefined))
error MissingTitle

function titleLength(title: string | undefined) {
  if (title === undefined) throw new MissingTitle()
  return title.length
}
// number!MissingTitle!

function optionalTitle(title: string | undefined) {
  try {
    return titleLength(title)
  } catch (error) {
    return 0
  }
}
// Output: 0
```

A throw stops the current sequence and adds its type to `E`. Control-flow analysis knows the missing-title branch cannot continue, so `title` is narrowed afterward. `try`/`catch` handles expected failures; defects and interruption propagate, and `finally` still runs. A cause containing a defect or interruption bypasses `catch` as a whole, even if it also contains an expected failure.

When a cause contains several expected failures and no defect or interruption, `catch` receives the first and handles the cause as a whole; use `Effect.catchCause` to inspect every failure. To keep processing after a failure without catching it immediately, use `Effect.result(() => titleLength(title))` and inspect the resulting `Result`.

## Retain the receiver

```vibe
// run: Console.log(label())
class Prefixer {
  constructor(readonly prefix: string) {}

  format(value: string) {
    Console.log("formatting label")
    return `${this.prefix}: ${value}`
  }
}

function label() {
  const prefixer = new Prefixer("item")
  return prefixer.format("marker")
}
// Output: formatting label
// Output: item: marker
```

Method calls preserve `this` and evaluate the receiver once; no nested generator needs a special receiver binding. To pass the method as a program, capture the receiver with `() => prefixer.format("marker")`.

## Generators produce values and streams

```vibe
// run: Console.log(JSON.stringify(firstTwo()))
function* ticks() {
  for (let n = 0; n < 3; n += 1) {
    sleep("10 millis")
    yield n
  }
}

function firstTwo() {
  const values: number[] = []
  for (const n of ticks()) {
    values.push(n)
    if (values.length === 2) break
  }
  return values
}
// Output: [0,1]
```

A VibeLang generator produces values for `for...of`; `yield` delivers a value instead of running an Effect. Leaving the loop early runs the producer's cleanup. Existing `Effect.gen` code stays in `.ts` files, where JavaScript generators keep their usual Effect meaning.

A producer also converts to a `Stream` wherever one is expected, so `Stream.take(ticks(), 2)` uses Effect's operators directly. The adapter hands over each value only when pulled, never running ahead; closing it interrupts the producer's fiber and runs its cleanup. `for...of` also consumes an Effect `Stream` directly and closes it when the loop exits.

## Coming from Effect

To convert a sequence to an ordinary VibeLang function, remove `Effect.gen`, `function*`, and `yield*`. Keep `function*` only for a function that produces a sequence of values.

[Upstream Effect page](https://effect.website/docs/v4/getting-started/using-generators)
