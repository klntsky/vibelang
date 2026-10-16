# Expected Errors

Expected errors describe situations callers can handle: invalid input, a denied operation, or an unavailable dependency. EffectScript infers them in the `E` channel and narrows them inside `catch`.

[Upstream page](https://effect.website/docs/v4/error-management/expected-errors)

## Constructing typed failures

```ts
class InvalidCode extends TaggedError("InvalidCode")<{
  readonly input: string
}> {}
class ItemMissing extends TaggedError("ItemMissing")<{
  readonly code: string
}> {}

function normalizeCode(input: string) {
  const code = input.trim()
  if (code.length === 0) throw new InvalidCode({ input })
  return code.toUpperCase()
}
// Effect<string, InvalidCode, never>
```

The global `TaggedError` is Effect's `Data.TaggedError`. Its constructor stores the fields and adds a readonly `_tag`; `throw` adds the class to `E`. `Effect.fail(error)` is also available, and `Effect.failSync(() => error)` defers construction until execution, though a function body already gives you that deferral.

Trusted built-ins can add expected failures too: `JSON.parse` adds `SyntaxError`. A plain TypeScript function needs `Effect.try` or a trusted declaration, `/** @effectscript trusted */`; each `@throws {T}` names an exported exception class that joins `E`, and unlisted exceptions are defects. Effect's own packages are trusted already, so calls such as `Option.isNone` need no boundary.

## Tracking several error cases

```ts
function findItem(input: string) {
  const code = normalizeCode(input)
  if (code !== "PEN") throw new ItemMissing({ code })
  return { code, label: "Fountain pen" }
}
// Effect<{ code: string; label: string }, InvalidCode | ItemMissing, never>
```

A function collects the failures it throws and those its calls can produce. Sequential statements stop at the first failure, so no lookup happens after normalization fails. An explicit full contract checks this inferred union rather than replacing it.

## Turning failures into values

```ts
import { Option, Result } from "effect"

function inspectItem(input: string) {
  const result = Effect.result(() => findItem(input))
  return Result.match(result, {
    onFailure: (error) => `unavailable: ${error._tag}`,
    onSuccess: (item) => item.label
  })
}

function optionalItem(input: string) {
  const item = Effect.option(() => findItem(input))
  return Option.isNone(item) ? "No item" : item.value.label
}
```

`Effect.result` returns `Result<A, E>` in the success channel and removes the typed failure channel. `Effect.option` returns `Some` on success and discards a typed error as `None`. Use the latter only when every domain failure means absence. Causes without any expected failure still propagate; these Effect combinators can consume a mixed cause when it also contains an expected failure. Use ordinary catch when defects and interruption must always propagate intact.

## Recovering all failures

```ts
function itemLabel(input: string) {
  try {
    return findItem(input).label
  } catch (error) {
    return `Unavailable (${error._tag})`
  }
}
// Effect<string, never, never>
```

The catch receives `InvalidCode | ItemMissing` and handles both here. It handles only causes with no defect and no interruption. If several expected failures occur together, it receives the first and handles their whole cause.

`Effect.catch(() => findItem(input), handler)` also recovers expected failures, but keeps Effect's own policy: it can recover a mixed cause containing an expected failure and drop the other reasons. `Effect.catchEager` uses that same policy and can apply recovery immediately when given an already computed `Exit`. Neither combinator has source catch's mixed-cause guard.

## Recovering selected failures

```ts
function labelOrMissing(input: string) {
  try {
    return findItem(input).label
  } catch (error) {
    if (error instanceof ItemMissing) return "Out of catalog"
    throw error
  }
}
// Effect<string, InvalidCode, never>

function labelByTag(input: string) {
  return Effect.catchTag(
    () => findItem(input),
    "ItemMissing",
    (error) => ({ code: error.code, label: "Out of catalog" })
  )
}
```

Narrowing and rethrowing preserves the unhandled error type. `Effect.catchTag` removes the selected tagged case; it also accepts a nonempty array of tags sharing one handler. `Effect.catchTags` supplies a table of separate handlers.

```ts
import { Filter } from "effect"

function withTagTable(input: string) {
  return Effect.catchTags(() => findItem(input), {
    InvalidCode: () => ({ code: "", label: "Enter a code" }),
    ItemMissing: (error) => ({ code: error.code, label: "Out of catalog" })
  })
}

function withFilter(input: string) {
  return Effect.catchFilter(
    () => findItem(input),
    Filter.tagged("ItemMissing"),
    (error) => ({ code: error.code, label: "Out of catalog" })
  )
}

function withPredicate(input: string) {
  return Effect.catchIf(
    () => findItem(input),
    (error) => error._tag === "ItemMissing",
    () => ({ code: "", label: "Out of catalog" })
  )
}
```

`catchIf` selects errors using a predicate or type guard. `catchFilter` accepts reusable selection logic from `Filter`; a narrowing filter removes the handled subtype. A boolean condition that handles only some values cannot promise that the whole subtype is gone.

## Filtering nested reasons

```ts
class Busy extends TaggedError("Busy")<{ readonly seconds: number }> {}
class Rejected extends TaggedError("Rejected")<{}> {}
class CatalogError extends TaggedError("CatalogError")<{
  readonly reason: Busy | Rejected
}> {}

function remoteLabel(): Effect<string, CatalogError, never> {
  throw new CatalogError({ reason: new Busy({ seconds: 4 }) })
}

function explainBusy() {
  return Effect.catchReason(
    () => remoteLabel(), "CatalogError", "Busy",
    (reason) => `Try again in ${reason.seconds} seconds`
  )
}

function explainReasons() {
  return Effect.catchReasons(() => remoteLabel(), "CatalogError", {
    Busy: (reason) => `Wait ${reason.seconds} seconds`,
    Rejected: () => "Request rejected"
  })
}

function exposeReasons() {
  return Effect.unwrapReason(() => remoteLabel(), "CatalogError")
}
// Effect<string, Busy | Rejected, never>
```

These operators inspect the tagged error's readonly `reason` field. `catchReason` keeps the parent wrapper for unhandled reasons, while `catchReasons` selects several reasons at once. `unwrapReason` replaces the wrapper with its nested reason in `E` without recovering.

## Coming from Effect

Functions replace `Effect.gen`; calls run the callee. Pass a program such as `() => findItem(input)` when a recovery operator needs work it can control.
