# Result

`Result` works in VibeLang exactly as in Effect. A `Result<A, E>` stores a synchronous success or an expected failure as data, so validation can return an outcome without raising it in the caller.

## Results and execution exits

```vibe
// run: Console.log(parseCount("6").isSuccess(), parseCount("-1").isFailure(), parseCount("").isFailure())
function parseCount(text: string) {
  const count = Number(text)
  return text.trim() !== "" && Number.isInteger(count) && count >= 0
    ? Result.succeed(count)
    : Result.fail("Expected a nonnegative integer")
}
// Result.Result<number, string>!!
```

```output
true true true
```

Returning a failed `Result` does not fail `parseCount`: the result object is its success value. `Result` is a choice between A and E and records no defects or interruption; for a computation's complete outcome, use `Exit`.

## Constructing and identifying variants

```vibe
// run: Console.log(Result.isResult(accepted), accepted.isSuccess(), rejected.isFailure()); Console.log(label(accepted)); Console.log(label(rejected))
const accepted = Result.succeed(8)
const rejected = Result.fail("limit exceeded")
Result.isResult(accepted) // true
accepted.isSuccess() // true
rejected.isFailure() // true

function label(result: Result.Result<number, string>) {
  if (result.isSuccess()) return `Count: ${result.success}`
  return `Rejected: ${result.failure}`
}
```

```output
true true true
Count: 8
Rejected: limit exceeded
```

A success holds its payload in `.success`, a failure in `.failure`; guards narrow the union before you read them. Neither variant is an Effect subtype, though JavaScript Effect generators can yield a Result.

## Matching and mapping outcomes

```vibe
// run: Console.log(summarize(Result.succeed(6))); Console.log(summarize(Result.fail("invalid")))
function summarize(result: Result.Result<number, string>) {
  const increased = result.map((count) => count + 2)
  const contextual = increased.mapError((error) => `count: ${error}`)
  const both = contextual.mapBoth({
    onSuccess: (count) => String(count),
    onFailure: (error) => error.toUpperCase()
  })
  return both.match({
    onSuccess: (count) => `Accepted ${count}`,
    onFailure: (error) => `Rejected ${error}`
  })
}
```

```output
Accepted 8
Rejected COUNT: INVALID
```

`map` transforms only success and `mapError` only failure; `mapBoth` applies the transformation for the current variant, and `match` handles both and removes the wrapper. Keep these callbacks synchronous; for service work, narrow the result in an ordinary function first.

## Converting results into effects

```vibe
// run: Console.log(JSON.stringify(validatePair())); try { requireCount("-1") } catch (error) { Console.log(error._tag, error.input) }
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
// number!InvalidCount!

function validatePair() {
  return all([
    requireCount("6"),
    Effect.fromResult(Result.succeed("ready"))
  ])
}
```

```output
[6,"ready"]
InvalidCount -1
```

`Effect.fromResult` runs as a call: success gives A, failure enters E. Convert explicitly when passing a result to an Effect combinator. Conversely, `Effect.result(() => work())` collects expected failures into a result: a cause with none still propagates, and for a cause that mixes one with defects or interruption, Effect 4 returns the first expected failure and discards the other reasons. To keep every reason, use `Effect.exit`.

## Sequencing and combining results

```vibe
// run: const result = combine(Result.succeed(" Weekly "), Result.succeed(6)); Console.log(JSON.stringify([result.chained.getOrThrow(), result.tuple.getOrThrow(), result.record.getOrThrow()])); const failed = combine(Result.fail("missing"), Result.fail("invalid")); Console.log(failed.chained.getOrElse((error) => error), failed.tuple.getOrElse((error) => error), failed.record.getOrElse((error) => error))
function combine(
  name: Result.Result<string, string>,
  count: Result.Result<number, string>
) {
  const chained = name.flatMap((name) =>
    count.map((count) => ({ name: name.trim(), count })))
  const tuple = Result.all([name, count])
  const record = Result.all({ name, count })
  return { chained, tuple, record }
}
```

```output
[{"name":"Weekly","count":6},[" Weekly ",6],{"name":" Weekly ","count":6}]
missing missing missing
```

`flatMap` chains a callback that returns a result, stopping at the first failure. `all` combines a tuple, record, or iterable of results into one, returning the first failure. It combines values already supplied; it is not a concurrency operator.

```ts
// TypeScript (.ts)
// combine-results.ts
// run: console.log(JSON.stringify(Result.getOrThrow(combineResults(Result.succeed(" Weekly "), Result.succeed(6))))); console.log(Result.getOrElse(combineResults(Result.fail("missing"), Result.succeed(6)), (error) => error))
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

```output
{"title":"Weekly","count":6}
missing
```

```vibe
// run: Console.log(JSON.stringify(combinedData().getOrThrow()))
import { combineResults } from "./combine-results.ts"

function combinedData() {
  return try(() => combineResults(
    Result.succeed(" Weekly "), Result.succeed(6)
  ))
}
```

```output
{"title":"Weekly","count":6}
```

`Result.gen` sequences with JavaScript generators: the first yielded failure skips the rest. VibeLang's `function*` declares a producer, so `Result.gen` in `.vibe` is EK1002 and generator-based APIs stay in `.ts` files. In VibeLang, use `flatMap` and `all`, or a TypeScript helper: an unannotated one, like this, is called through an explicit `try` hatch; one declared with `/** @vibelang trusted */` can be called directly.
