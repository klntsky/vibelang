# Filters

Filters add validation to a schema without changing its decoded type. They work as in Effect and run after the underlying schema accepts the value. Use synchronous predicates for local checks and effectful getters for checks that need services. Plain builders follow the [trusted call rules](085-introduction.md#configuring-compiler-prerequisites).

## Defining predicate-based checks

```vibe
const ShelfCode = Schema.String.check(
  Schema.makeFilter((code) => code.startsWith("S-") || "Use an S- shelf code")
)

function readShelf(input: unknown) {
  return Schema.decodeUnknownEffect(ShelfCode)(input)
}
```

A predicate returning `true` or `undefined` accepts the value; `false` rejects it with the default message. A string rejects it with that message. A predicate can also return a `SchemaIssue.Issue`, a path-and-issue object, or an array of issues. It receives the value, the schema AST, and the active parse options.

Filters keep the original TypeScript type. Add a brand when you need validated values to have a distinct static identity. A plain filter callback must complete synchronously with no typed failures or service calls; use `SchemaGetter.checkEffect` for effectful checks.

## Annotating validation checks

```vibe
const FourDigits = Schema.String.check(
  Schema.makeFilter((text) => /^\d{4}$/.test(text), {
    expected: "four decimal digits",
    description: "The depot's four-digit routing code",
    toJsonSchema: () => ({ pattern: "^[0-9]{4}$" })
  })
)
```

`expected` supplies the default formatter's description, while `message` replaces its message. `toJsonSchema` explains the constraint to the JSON Schema interpreter. A custom predicate alone cannot teach a generator or document exporter its meaning; add `arbitraryConstraint` when the generator can construct matching values.

## Assigning issue paths and reporting several issues

```vibe
const DispatchWindow = Schema.Struct({
  start: Schema.Int,
  end: Schema.Int,
  destination: Schema.String
}).check(
  Schema.makeFilter((window) => {
    const issues: Array<Schema.FilterIssue> = []
    if (window.end <= window.start) {
      issues.push({ path: ["end"], issue: "End must follow start" })
    }
    if (window.destination.length === 0) {
      issues.push({ path: ["destination"], issue: "Choose a destination" })
    }
    return issues
  })
)

function validateWindow(input: unknown) {
  return Schema.decodeUnknownEffect(DispatchWindow)(input, { errors: "all" })
}
```

Paths are relative to the checked value and can contain keys or array indices. Returning several issues attaches each one to its own field; an empty list succeeds. `errors: "all"` also collects independent failures elsewhere in the schema. The cross-field check runs only when the struct has supplied a valid value to it.

## Applying built-in constraints

```vibe
const Label = Schema.String.check(
  Schema.isMinLength(2), Schema.isMaxLength(40), Schema.isTrimmed()
)
const Count = Schema.Finite.check(
  Schema.isInt(), Schema.isGreaterThanOrEqualTo(0), Schema.isMultipleOf(2)
)
const Ratio = Schema.Finite.check(Schema.isBetween({ minimum: 0, maximum: 1 }))
const Batch = Schema.Array(Schema.String).check(Schema.isBetweenLength(1, 8))
const Serial = Schema.BigInt.check(Schema.isGreaterThanBigInt(0n))
```

String checks include `isNonEmpty`, `isPattern`, `isStartingWith`, `isEndingWith`, `isIncluding`, `isTrimmed`, `isLowercased`, `isUppercased`, `isCapitalized`, and `isUncapitalized`. Length checks work on strings and arrays; `isBetweenLength(n, n)` expresses an exact length. `Schema.NonEmptyString` is a ready-made nonempty string schema.

Numbers support strict and inclusive lower and upper bounds, `isBetween`, `isInt`, and `isMultipleOf`. Bigints have the corresponding `BigInt` bounds, including `isBetweenBigInt`. Checks validate their input; they do not round numbers, change case, or trim strings.

```vibe
const ShipDate = Schema.Date.check(
  Schema.isBetweenDate({
    minimum: new Date("2026-01-01T00:00:00.000Z"),
    maximum: new Date("2026-12-31T23:59:59.999Z")
  })
)

function boundedPriceSchema() {
  const zero = try(BigDecimal.fromNumberUnsafe(0))
  return Schema.BigDecimal.check(Schema.isGreaterThanOrEqualToBigDecimal(zero))
}

const HoldDuration = Schema.Duration.check(
  Schema.makeFilter((duration) => duration.between({
    minimum: Duration.seconds(1), maximum: Duration.seconds(30)
  }))
)
```

`Schema.Date` rejects invalid dates before date bounds run. Date and BigDecimal checks provide strict, inclusive, and range variants named with `Date` and `BigDecimal` suffixes. Duration checks can use a custom filter with `Duration.isGreaterThan`, `isLessThan`, their inclusive variants, or `between`.

Fixed date parsing is deterministic. For a boundary relative to the current time, read time through Effect's clock APIs inside a function and build the schema there.

[Upstream page](https://effect.website/docs/v4/schema/filters)
