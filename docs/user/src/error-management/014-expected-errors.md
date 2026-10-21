# Expected Errors

Expected errors are situations callers can handle: invalid input, a denied operation, an unavailable dependency. VibeLang infers them in `E` and narrows them inside `catch`.

[Upstream page](https://effect.website/docs/v4/error-management/expected-errors)

## Constructing typed failures

```vibe
// run: Console.log(normalizeCode(" pen ")); Console.log(Effect.flip(() => normalizeCode(" "))._tag)
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

```output
PEN
InvalidCode
```

`error InvalidCode { input: string }` declares an expected failure class, short for `class InvalidCode extends TaggedError("InvalidCode")<{ readonly input: string }> {}`, which also compiles; the global `TaggedError` is Effect's `Data.TaggedError`. The constructor takes the fields, all readonly, and each instance has a readonly `_tag` equal to the class name. `error Timeout` has no fields. In an Effect function, `throw` adds the class to `E`. `Effect.fail(error)` also works; `Effect.failSync(() => error)` defers construction to execution, as a function body already does.

In Effect functions, trusted built-ins can add expected failures too: `JSON.parse` adds `SyntaxError`. A plain TypeScript function needs the `try` hatch or a `.ts` wrapper made with `trusted(implementation, [ErrorClass])`, importing `trusted` from `vibelang-runtime`: instances of listed classes join `E`; unlisted exceptions are defects. The older `/** @vibelang trusted */` and `@throws` declaration tags still work; calls that rely on tagged expected failures get migration warning EK4406. Effect's own packages are trusted, so calls such as `Option.isNone` need no boundary.

`throw` is a typed failure everywhere outside a hatch, and `catch` receives typed failures only (D282). A function with a plain return annotation is plain JavaScript because it never fails and needs nothing. It may throw what its own `catch` handles, and it still returns a value:

```vibe
// run: Console.log(quantityOr("3", 0)); Console.log(quantityOr("x", 0))
error InvalidQuantity { text: string }

function quantityOr(text: string, fallback: number): number {
  try {
    const quantity = Number(text)
    if (!Number.isInteger(quantity)) throw new InvalidQuantity({ text })
    return quantity
  } catch {
    return fallback
  }
}
// Output: 3
// Output: 0
```

A failure that can escape a plain function is EK2003, and the quick fix writes the Effect contract:

```vibe
// expect-error: EK2003
function quantity(text: string): number {
  const value = Number(text)
  if (!Number.isInteger(value)) throw new InvalidQuantity({ text })
  return value
}
```

```text
error EK2003: number is a plain return annotation, but quantity can fail with InvalidQuantity, and a plain function never fails.
  Write number!InvalidQuantity! to make it an Effect function that fails with InvalidQuantity.
  Or throw inside a hatch, whose TypeScript keeps JavaScript's throw: sync makes the exception a defect (number!!), and try an UnknownError.
```

The same holds for a throw expression and for a `match` arm or `do` block that throws. A plain function calls TypeScript code directly. A call whose contract lists a failure, such as `JSON.parse`, is a typed failure only inside a `try` whose `catch` belongs to the function, which receives it with its type; elsewhere its exception propagates unchanged. Other TypeScript code's exceptions pass through the `catch` unchanged and become defects where they reach an effectful caller; to catch one, use the `try` hatch in an Effect function. Running an Effect inside a plain-annotated function is EK2003 too.

## Failing in an expression

```vibe
// run: Console.log(port(new Map([["PORT", "3000"]]))); Console.log(Effect.flip(() => port(new Map()))._tag)
error MissingPort

function port(env: Map<string, string>) {
  return env.get("PORT") ?? throw new MissingPort()
}
// string!MissingPort!
```

```output
3000
MissingPort
```

`throw` also works as an expression, per the TC39 [throw expressions](https://github.com/tc39/proposal-throw-expressions) proposal, and fails as the statement does. The right side of `??`, `||`, `&&`, and `?:` runs only when chosen. A throw expression takes only a unary operand: EK1010 refuses an unparenthesized throw on an operator's left, as a conditional's condition, or before `as` or `satisfies`, so write `throw (cond ? a : b)` to throw a whole conditional. The statement `throw cond ? a : b;` still throws the selected value.

## Tracking several error cases

```vibe
// run: Console.log(findItem("pen").label); Console.log(Effect.flip(() => findItem("book"))._tag); Console.log(Effect.flip(() => findItem(""))._tag)
function findItem(input: string) {
  const code = normalizeCode(input)
  if (code !== "PEN") throw new ItemMissing({ code })
  return { code, label: "Fountain pen" }
}
// { code: string; label: string }!InvalidCode | ItemMissing!
```

```output
Fountain pen
ItemMissing
InvalidCode
```

A function collects the failures it throws and those its calls can produce. Sequential statements stop at the first failure: no lookup follows a failed normalization. An explicit full contract checks this inferred union; it doesn't replace it.

## Turning failures into values

```vibe
// run: Console.log(inspectItem("pen")); Console.log(inspectItem("")); Console.log(inspectItem("book")); Console.log(optionalItem("pen")); Console.log(optionalItem("book"))
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

```output
Fountain pen
unavailable: InvalidCode
unavailable: ItemMissing
Fountain pen
No item
```

`Effect.result` moves `E` into the success channel as `Result<A, E>`. `Effect.option` gives `Some` on success and discards a typed error as `None`; use it only when every domain failure means absence. Causes with no expected failure still propagate, but both can consume a mixed cause that contains one; use ordinary catch when defects and interruption must always propagate intact.

## Recovering all failures

```vibe
// run: Console.log(itemLabel("pen")); Console.log(itemLabel("")); Console.log(itemLabel("book"))
function itemLabel(input: string) {
  try {
    return findItem(input).label
  } catch (error) {
    return `Unavailable (${error._tag})`
  }
}
// string!!
```

```output
Fountain pen
Unavailable (InvalidCode)
Unavailable (ItemMissing)
```

The catch receives `InvalidCode | ItemMissing` and handles both, but only in causes with no defect or interruption; given several expected failures together, it receives the first and handles their whole cause.

```vibe
// run: Console.log(itemLabelInline("pen")); Console.log(itemLabelInline("")); Console.log(itemLabelInline("book"))
function itemLabelInline(input: string) {
  return findItem(input).label catch (error) => `Unavailable (${error._tag})`
}
// string!!
```

```output
Fountain pen
Unavailable (InvalidCode)
Unavailable (ItemMissing)
```

After an expression, `x catch (error) => handler` runs `x` and passes an expected failure to the handler, as `Effect.catch` does, with the statement's guard: defects and interruption pass through. `x catch fallback` evaluates the fallback instead, as `Effect.orElseSucceed` does; see [Fallback](016-fallback.md).

`Effect.catch(() => findItem(input), handler)` also recovers expected failures, under Effect's own policy and without source catch's mixed-cause guard: it can recover a mixed cause containing an expected failure and drop the other reasons. `Effect.catchEager` has the same policy and can recover immediately when given an already computed `Exit`.

## Recovering selected failures

```vibe
// run: Console.log(labelOrMissing("book")); Console.log(Effect.flip(() => labelOrMissing(""))._tag); Console.log(labelByTag("book").label); Console.log(Effect.flip(() => labelByTag(""))._tag)
function labelOrMissing(input: string) {
  try {
    return findItem(input).label
  } catch (error) {
    return match (error) {
      when ItemMissing: "Out of catalog";
      default: throw error;
    }
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

```output
Out of catalog
InvalidCode
Out of catalog
InvalidCode
```

Catch, then `match` on the failure: an arm such as `when ItemMissing:` handles that error class, and `default: throw error` passes the others on with their types (`string!InvalidCode!`). The expression form takes the same `match`: `findItem(input).label catch (error) => match (error) { when ItemMissing: "Out of catalog"; default: throw error; }`. A `match` without `default` must handle every failure the `try` block can raise (compile error EK4701), so a complete match needs no rethrow. `error is ItemMissing` tests one class in an `if`. `Effect.catchTag` removes the selected tagged case, and accepts a nonempty array of tags for one handler; `Effect.catchTags` takes a table of separate handlers.

```vibe
// run: Console.log(withTagTable("").label); Console.log(withFilter("book").label); Console.log(withPredicate("book").label); Console.log(Effect.flip(() => withFilter(""))._tag); Console.log(Effect.flip(() => withPredicate(""))._tag)
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

```output
Enter a code
Out of catalog
Out of catalog
InvalidCode
InvalidCode
```

`catchIf` selects errors with a predicate or type guard; `catchFilter`, with reusable selection logic from `Filter`. A narrowing filter removes the handled subtype; a boolean condition that handles only some values can't promise the whole subtype is gone.

## Filtering nested reasons

```vibe
// run: Console.log(explainBusy()); Console.log(explainReasons()); Console.log(Effect.flip(() => exposeReasons())._tag)
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

```output
Try again in 4 seconds
Wait 4 seconds
Busy
```

These operators inspect the tagged error's readonly `reason` field. `catchReason` keeps the parent wrapper for unhandled reasons; `catchReasons` selects several at once; `unwrapReason` replaces the wrapper in `E` with its nested reason, without recovering.

## Coming from Effect

Functions replace `Effect.gen`; calls run the callee. Where it runs, `Effect.fail(error)` draws EK4834, whose quick fix writes `throw error`, and `Effect.catchTag(program, "Tag", handler)` draws EK4835, whose quick fix writes `program catch (e) => match (e) { … }` ([TypeScript-style Effect code](../getting-started/012-devtools.md#typescript-style-effect-code)). Pass a program such as `() => findItem(input)` when a recovery operator needs work it can control.
