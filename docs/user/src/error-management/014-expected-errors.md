# Expected Errors

Expected errors describe situations callers can handle: invalid input, a denied operation, or an unavailable dependency. Effeck infers them in the `E` channel and narrows them inside `catch`.

[Upstream page](https://effect.website/docs/v4/error-management/expected-errors)

## Constructing typed failures

```ts
error InvalidCode {
  input: string
}
error ItemMissing {
  code: string
}

function normalizeCode(input: string) {
  const code = input.trim()
  if (code.length === 0) throw new InvalidCode({ input })
  return code.toUpperCase()
}
// string!InvalidCode!
```

`error InvalidCode { input: string }` declares an expected failure. The declaration is a class: its constructor takes the fields, every field is readonly, and each instance has a readonly `_tag` equal to the class name. `throw` adds the class to `E`. `error Timeout` declares an error without fields. The declaration is short for `class InvalidCode extends TaggedError("InvalidCode")<{ readonly input: string }> {}`, which also compiles; the global `TaggedError` is Effect's `Data.TaggedError`. `Effect.fail(error)` is also available, and `Effect.failSync(() => error)` defers construction until execution, though a function body already gives you that deferral.

Trusted built-ins can add expected failures too: `JSON.parse` adds `SyntaxError`. A plain TypeScript function needs the `try` hatch or a trusted declaration, `/** @effeck trusted */`; each `@throws {T}` names an exported exception class that joins `E`, and unlisted exceptions are defects. Effect's own packages are trusted already, so calls such as `Option.isNone` need no boundary.

## Failing in an expression

```ts
error MissingPort

function port(env: Map<string, string>) {
  return env.get("PORT") ?? throw new MissingPort()
}
// string!MissingPort!
```

`throw` also works in expression position, as the TC39 [throw expressions](https://github.com/tc39/proposal-throw-expressions) proposal writes it, and fails the same way the statement does. The right side of `??`, `||`, `&&`, and `?:` runs only when it is chosen.

## Tracking several error cases

```ts
function findItem(input: string) {
  const code = normalizeCode(input)
  if (code !== "PEN") throw new ItemMissing({ code })
  return { code, label: "Fountain pen" }
}
// { code: string; label: string }!InvalidCode | ItemMissing!
```

A function collects the failures it throws and those its calls can produce. Sequential statements stop at the first failure, so no lookup happens after normalization fails. An explicit full contract checks this inferred union rather than replacing it.

## Turning failures into values

```ts
function inspectItem(input: string) {
  const result = Effect.result(findItem(input))
  return result.match({
    onFailure: (error) => `unavailable: ${error._tag}`,
    onSuccess: (item) => item.label
  })
}

function optionalItem(input: string) {
  const item = Effect.option(findItem(input))
  return item.isNone() ? "No item" : item.value.label
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
// string!!
```

The catch receives `InvalidCode | ItemMissing` and handles both here. It handles only causes with no defect and no interruption. If several expected failures occur together, it receives the first and handles their whole cause.

`Effect.catch(() => findItem(input), handler)` also recovers expected failures, but keeps Effect's own policy: it can recover a mixed cause containing an expected failure and drop the other reasons. `Effect.catchEager` uses that same policy and can apply recovery immediately when given an already computed `Exit`. Neither combinator has source catch's mixed-cause guard.

## Recovering selected failures

```ts
function labelOrMissing(input: string) {
  try {
    return findItem(input).label
  } catch (error: ItemMissing) {
    return "Out of catalog"
  }
}
// string!InvalidCode!

function labelByTag(input: string) {
  return Effect.catchTag(
    findItem(input),
    "ItemMissing",
    (error) => ({ code: error.code, label: "Out of catalog" })
  )
}
```

A typed catch clause, `catch (error: ItemMissing)`, handles only failures with that type's `_tag`; the others propagate with their types, as `string!InvalidCode!` shows. Several clauses can follow one `try`, `catch (error: A | B)` names several types, and a final untyped clause catches the rest. `Effect.catchTag` removes the selected tagged case; it also accepts a nonempty array of tags sharing one handler. `Effect.catchTags` supplies a table of separate handlers.

```ts
function withTagTable(input: string) {
  return Effect.catchTags(findItem(input), {
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
    (error) => error is ItemMissing,
    () => ({ code: "", label: "Out of catalog" })
  )
}
```

`catchIf` selects errors using a predicate or type guard. `catchFilter` accepts reusable selection logic from `Filter`; a narrowing filter removes the handled subtype. A boolean condition that handles only some values cannot promise that the whole subtype is gone.

## Filtering nested reasons

```ts
error Busy { seconds: number }
error Rejected
error CatalogError {
  reason: Busy | Rejected
}

function remoteLabel(): string!CatalogError! {
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
// string!Busy | Rejected!
```

These operators inspect the tagged error's readonly `reason` field. `catchReason` keeps the parent wrapper for unhandled reasons, while `catchReasons` selects several reasons at once. `unwrapReason` replaces the wrapper with its nested reason in `E` without recovering.

## Coming from Effect

Functions replace `Effect.gen`; calls run the callee. Pass a program such as `() => findItem(input)` when a recovery operator needs work it can control.
