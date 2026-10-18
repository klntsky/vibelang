# Sink Operations

Transform a Sink when its existing consumption rule fits but its input or result type does not. Pure mappings build a new sink; effectful mappings defer their work until consumption. Filter the feeding Stream when only selected elements should reach a sink.

## Adapting consumer input

```effeck
const quantities = Sink.sum.mapInput((entry: { quantity: number }) => entry.quantity)

function countItems() {
  return Stream.make({ quantity: 7 }, { quantity: 11 }).run(quantities)
}
// Returns 18.
```

`mapInput` converts each incoming value before the original sink sees it. Its callback is synchronous and cannot perform Effect work. Use `mapInputEffect` for validation, service calls, or other work with failures and requirements.

```effeck
error InvalidQuantity {
  raw: string
}

const parsedTotal = Sink.sum.mapInputEffect((raw: string) => {
  const value = Number(raw)
  if (!Number.isFinite(value)) throw new InvalidQuantity({ raw })
  return value
})
```

The new sink accepts strings and can fail with `InvalidQuantity`. A failed input transformation fails consumption; it does not silently discard the input.

## Mapping consumer output

```effeck
const report = Sink.sum.map((total) => ({ total, unit: "items" }))

function totalReport() {
  return Stream.make(7, 11).run(report)
}
// Returns { total: 18, unit: "items" }.
```

`map` runs once on the sink's successful final result. It leaves the input and leftover types unchanged. `mapEffect` allows Effect work when transforming that result, such as saving a summary.

## Composing input and output transformations

```effeck
const report = Sink.sum
  .mapInput((entry: { bytes: number }) => entry.bytes)
  .map((bytes) => `${bytes} bytes`)

function byteReport() {
  return Stream.make({ bytes: 128 }, { bytes: 256 }).run(report)
}
// Returns "384 bytes".
```

The input stage adapts each element, the original sink accumulates, and the output stage formats once. The method chain builds a Sink value. `Stream.run` is the operation that executes it.

## Filtering before consumption

```effeck
function positiveBatches() {
  const positive = Stream.make(-3, 4, 0, 8, -1, 12, 16).filter((n) => n > 0)
  const batches = positive.transduce(Sink.take<number>(3))
  return batches.runCollect() // [[4, 8, 12], [16]]
}
```

Filtering upstream means rejected elements do not count toward the sink's limit. `Stream.transduce` repeatedly runs the sink and passes batch leftovers into the next run. If input ends exactly at a completed sink boundary, a final empty sink result can appear; remove empty arrays downstream when the application needs only non-empty groups.
