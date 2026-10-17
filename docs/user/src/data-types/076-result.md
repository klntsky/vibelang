# Result

`Result` works in Effeck exactly as in Effect. A `Result<A, E>` stores either a synchronous success or an expected failure as data, useful when validation should return an outcome without raising it in the caller.

## Results and execution exits

```ts
function parseCount(text: string) {
  const count = Number(text)
  return text.trim() !== "" && Number.isInteger(count) && count >= 0
    ? Result.succeed(count)
    : Result.fail("Expected a nonnegative integer")
}
// Effect<Result.Result<number, string>, never, never>
```

Returning a failed `Result` does not fail `parseCount`: its success value is the result object. `Result` describes a choice between A and E; it does not record defects or interruption. Use `Exit` when you need the complete outcome of running a computation.

## Constructing and identifying variants

```ts
const accepted = Result.succeed(8)
const rejected = Result.fail("limit exceeded")
Result.isResult(accepted) // true
Result.isSuccess(accepted) // true
Result.isFailure(rejected) // true

function label(result: Result.Result<number, string>) {
  if (Result.isSuccess(result)) return `Count: ${result.success}`
  return `Rejected: ${result.failure}`
}
```

Success stores its payload in `.success`; failure uses `.failure`. Guards narrow the union before you access those fields. Neither variant is an Effect subtype, even though Result supports yielding from JavaScript Effect generators.

## Matching and mapping outcomes

```ts
function summarize(result: Result.Result<number, string>) {
  const increased = Result.map(result, (count) => count + 2)
  const contextual = Result.mapError(increased, (error) => `count: ${error}`)
  const both = Result.mapBoth(contextual, {
    onSuccess: (count) => String(count),
    onFailure: (error) => error.toUpperCase()
  })
  return Result.match(both, {
    onSuccess: (count) => `Accepted ${count}`,
    onFailure: (error) => `Rejected ${error}`
  })
}
```

`map` transforms only success, and `mapError` transforms only failure. `mapBoth` selects the transformation for the current variant; `match` removes the wrapper by handling both possibilities. Keep these callbacks as synchronous computations; narrow the result in an ordinary function before doing service work.

## Converting results into effects

```ts
error InvalidCount {
  input: string
}

function requireCount(input: string) {
  const count = Number(input)
  const result = input.trim() !== "" && Number.isInteger(count) && count >= 0
    ? Result.succeed(count)
    : Result.fail(new InvalidCount({ input }))
  return Effect.fromResult(result)
}
// Effect<number, InvalidCount, never>

function validatePair() {
  return Effect.all([
    requireCount("6"),
    Effect.fromResult(Result.succeed("ready"))
  ])
}
```

`Effect.fromResult` runs as a call: success produces A, while failure enters E. Convert explicitly when passing a result to an Effect combinator. Conversely, `Effect.result(() => work())` collects expected failures into a result; defects and interruption still propagate.

## Sequencing and combining results

```ts
function combine(
  name: Result.Result<string, string>,
  count: Result.Result<number, string>
) {
  const chained = Result.flatMap(name, (name) =>
    Result.map(count, (count) => ({ name: name.trim(), count }))
  )
  const tuple = Result.all([name, count])
  const record = Result.all({ name, count })
  return { chained, tuple, record }
}
```

`flatMap` chains a callback that itself returns a result, stopping at the first failure. `all` combines tuples, records, or iterables into one result and returns the first encountered failure. It combines values already supplied; it is not a concurrency operator.

```ts
// TypeScript (.ts)
// combine-results.ts
import { Result } from "effect"

export function combineResults(
  title: Result.Result<string, string>,
  count: Result.Result<number, string>
) {
  return Result.gen(function* () {
    const normalized = (yield* title).trim()
    const total = yield* count
    return { title: normalized, count: total }
  })
}
```

```ts
import { combineResults } from "./combine-results.ts"

function combinedData() {
  return Effect.try(() => combineResults(
    Result.succeed(" Weekly "), Result.succeed(6)
  ))
}
```

`Result.gen` is the JavaScript generator sequencing API: the first yielded failure skips the rest. Effeck's `function*` declares a producer, so generator-based APIs stay in `.ts` files; using `Result.gen` in `.effeck` is EK1002. Use `flatMap` and `all` locally, or keep generator syntax in a TypeScript helper. This unannotated helper uses an explicit `Effect.try` boundary; a helper declared with `/** @effeck trusted */` can be called directly.
