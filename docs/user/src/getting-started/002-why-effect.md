# Why Effeck?

Effeck uses Effect to make a program's expected failures and dependencies visible to the compiler. Choose it when application logic needs coordinated retries, cancellation, resource cleanup, and service substitution.

## Describe work before running it

```ts
error InvalidQuantity {
  quantity: number
}

function unitPrice(total: number, quantity: number) {
  if (quantity <= 0) throw new InvalidQuantity({ quantity })
  return total / quantity
}
// number!InvalidQuantity!

const quote = () => unitPrice(72, 6)
```

`quote` holds a reusable program. Calling `quote()` inside another function runs it and returns the number. The error channel records the validation failure, so callers can handle it without inspecting this implementation.

## Share an application foundation

```ts
service Prices {
  total(sku: string): number!InvalidQuantity
}

function receipt(sku: string) {
  const amount = Prices.total(sku)
  return `Total: ${amount}`
}
// string!InvalidQuantity!Prices

const PricesTest = Layer.succeed(Prices, { total: (_sku) => 18 })

function preview() {
  return Effect.provide(() => receipt("pencil-box"), PricesTest)
}
```

Services carry dependencies through the same contract as errors and successes. Layers supply implementations and manage their resources. Tests can provide an in-memory implementation while the application provides a database or HTTP implementation.

## Apply composable production tools

```ts
function resilientReceipt(sku: string) {
  return pipe(
    () => receipt(sku),
    retry({ times: 2 }),
    timeout("800 millis"),
    withSpan("receipt")
  )
}
```

These operators share Effect's execution and cleanup rules. Retry repeats the program from the beginning; timeout interrupts work that exceeds its deadline; the span describes the whole operation. Choose retry only for failures worth repeating, and make repeated writes safe before retrying them.

Use the same foundation for streams, caches, validated input, configuration, and observability. You can introduce those modules as concrete problems arise.

## Learn with practical programs

Start with one operation that reads data, validates it, and returns a result. Test its success and failure cases, then add a service boundary and a deadline. Continue with [The Effect Type](003-the-effect-type.md) and [Creating Effects](005-creating-effects.md).

## Coming from Effect

Effeck keeps Effect's library and runtime. Ordinary statements replace most sequencing operators, while retry, timeout, concurrency, and provisioning keep their library APIs with program arguments.

[Upstream Effect page](https://effect.website/docs/v4/getting-started/why-effect)
