# Equivalence

`Equivalence` works in EffectScript exactly as in Effect. It packages a reusable rule for deciding whether two values count as equivalent in a particular operation, without changing their global equality behavior.

## Representing a reusable equality relation

```ts
import { Equivalence } from "effect"

const sameLabel: Equivalence.Equivalence<string> = (a, b) =>
  a.trim().toLowerCase() === b.trim().toLowerCase()

sameLabel(" Inbox ", "inbox") // true
sameLabel("inbox", "archive") // false
```

An `Equivalence<A>` is a synchronous function from two A values to a boolean. A sound relation is reflexive, symmetric, and transitive. Keep it free of service calls and expected failures so Effect can invoke it as an ordinary JavaScript callback.

## Choosing built-in relations

```ts
import { Equivalence } from "effect"

Equivalence.String("cyan", "cyan") // true
Equivalence.Number(12, 13) // false
Equivalence.Boolean(true, true) // true
Equivalence.BigInt(5n, 5n) // true
Equivalence.Date(new Date("2026-04-12"), new Date("2026-04-12")) // true

const sameSymbol = Equivalence.strictEqual<symbol>()
const key = Symbol("entry")
sameSymbol(key, key) // true
sameSymbol(key, Symbol("entry")) // false
```

The string, boolean, bigint, and `strictEqual` relations use strict equality. `Equivalence.Number` also equates `NaN` with itself. The date relation compares timestamps using that numeric relation instead of comparing object references.

## Deriving comparisons through input projections

```ts
import { Equivalence } from "effect"

interface Product {
  readonly sku: string
  readonly displayName: string
}

const bySku = Equivalence.mapInput(
  Equivalence.String,
  (product: Product) => product.sku
)

bySku(
  { sku: "part-4", displayName: "Bolt" },
  { sku: "part-4", displayName: "Steel bolt" }
) // true
```

`mapInput` projects each operand before applying the underlying relation. Here the SKU determines equivalence and the display name is ignored. Pass this rule to an API that accepts an equivalence; it does not alter `Equal.equals`, JavaScript `===`, or hash-collection key behavior.
