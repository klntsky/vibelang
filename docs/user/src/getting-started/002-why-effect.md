# Why VibeLang?

VibeLang uses Effect to make a program's expected failures and dependencies visible to the compiler. Choose it when application logic needs coordinated retries, cancellation, resource cleanup, and service substitution.

## Describe work before running it

```vibe
// run: Console.log(quote())
error InvalidQuantity {
  quantity: number
}

function unitPrice(total: number, quantity: number) {
  if (quantity <= 0) throw new InvalidQuantity({ quantity })
  return total / quantity
}
// Effect<number, InvalidQuantity>

const quote = () => unitPrice(72, 6)
// Output: 12
```

`quote` holds a reusable program; calling `quote()` inside another function runs it and returns the number. The error channel records the validation failure, so callers can handle it without inspecting this implementation.

## Share an application foundation

```vibe
// run: Console.log(preview())
service Prices {
  total(sku: string): Effect<number, InvalidQuantity>
}

function receipt(sku: string) {
  const amount = Prices.total(sku)
  return `Total: ${amount}`
}
// Effect<string, InvalidQuantity, Prices>

const PricesTest = Layer.succeed(Prices, {
  total(_sku) { return 18 }
})

function preview() {
  provide PricesTest { return receipt("pencil-box") }
}
// Output: Total: 18
```

Services carry dependencies in the same contract as errors and successes. Layers supply implementations and manage their resources: tests provide an in-memory implementation, the application a database or HTTP one.

## Apply composable production tools

```vibe
// run: Console.log(Effect.provide(resilientReceipt("pencil-box"), PricesTest))
function resilientReceipt(sku: string) {
  return receipt(sku)
    |> retry(%, { times: 2 })
    |> timeout(%, "800 millis")
    |> withSpan(%, "receipt")
}
// Output: Total: 18
```

These operators share Effect's execution and cleanup rules. Retry repeats the program from the beginning, timeout interrupts work past its deadline, and the span describes the whole operation. Retry only failures worth repeating, and make repeated writes safe first.

The same foundation serves streams, caches, validated input, configuration, and observability; introduce those modules as concrete problems arise.

## Learn with practical programs

Start with one operation that reads data, validates it, and returns a result. Test its success and failure cases, then add a service boundary and a deadline. Continue with [The Effect Type](003-the-effect-type.md) and [Creating Effects](005-creating-effects.md).

## Coming from Effect

VibeLang keeps Effect's library and runtime. Statements replace most sequencing operators; retry, timeout, concurrency, and provisioning keep their library APIs, with program arguments.

[Upstream Effect page](https://effect.website/docs/v4/getting-started/why-effect)
