# BigDecimal

`BigDecimal` works in Effeck exactly as in Effect. Use it for decimal arithmetic when binary floating point would lose information, such as prices imported as decimal strings.

## Representing decimal values

```ts
const unitPrice = BigDecimal.make(875n, 2)
const bulkPrice = BigDecimal.make(875n, -1)
BigDecimal.format(unitPrice) // "8.75"
BigDecimal.format(bulkPrice) // "8750"
```

A value stores an integer coefficient and a scale: coefficient × 10 to the power of negative scale. Positive scales place digits after the decimal point; negative scales add integer zeros. Precision is arbitrary within implementation and memory limits, rather than limited to JavaScript's number precision.

## Constructing decimals

```ts
const whole = BigDecimal.fromBigInt(42n)
const parsed = BigDecimal.fromString("18.625")
const invalid = BigDecimal.fromString("eighteen")
const fromNumber = BigDecimal.fromNumber(2.5)

parsed.isSome() // true
invalid.isNone() // true
BigDecimal.format(whole) // "42"
```

`make` accepts a coefficient and scale, while `fromBigInt` uses scale zero. `fromString` and `fromNumber` return an `Option`, so invalid input stays data. Prefer strings for externally supplied decimals: converting a number cannot recover digits already lost in floating point calculations.

## Safe and unsafe constructors

```ts
error InvalidPrice {
  input: string
}

function parsePrice(input: string) {
  return try((input) => BigDecimal.fromStringUnsafe(input), () => new InvalidPrice({ input }), input)
}
// BigDecimal.BigDecimal!InvalidPrice!

const fixedRate = BigDecimal.fromNumberUnsafe(1.25)
```

`fromStringUnsafe` throws for invalid text; `fromNumberUnsafe` throws for non-finite numbers. Use an unsafe constructor for a known valid constant, or put it inside an explicit boundary that declares the expected error. The safe constructors are usually simpler for validation.

## Performing arithmetic

```ts
const price = BigDecimal.make(875n, 2)
const quantity = BigDecimal.fromBigInt(4n)
const subtotal = price.multiply(quantity)
const discount = BigDecimal.make(250n, 2)
const total = subtotal.subtract(discount)

BigDecimal.format(total) // "32.5"
BigDecimal.format(total.sum(discount)) // "35"
total.divide(quantity) // Some containing 8.125
total.remainder(quantity) // Some containing 0.5
BigDecimal.sign(BigDecimal.negate(total)) // -1
BigDecimal.format(BigDecimal.abs(BigDecimal.negate(total))) // "32.5"
```

`sum`, `subtract`, and `multiply` produce new decimals. `divide` and `remainder` return `None` for a zero divisor; `divideUnsafe` and `remainderUnsafe` throw instead. Division with a nonterminating decimal expansion uses the library's precision policy, so arbitrary precision does not mean an infinitely long quotient.

## Comparing values

```ts
const amount = BigDecimal.make(325n, 1)
const limit = BigDecimal.fromBigInt(40n)
amount.isLessThan(limit) // true
amount.isLessThanOrEqualTo(amount) // true
limit.isGreaterThan(amount) // true
limit.isGreaterThanOrEqualTo(limit) // true
amount.min(limit) // amount
amount.max(limit) // limit
amount.between({
  minimum: BigDecimal.fromBigInt(0n), maximum: limit
}) // true
BigDecimal.isZero(amount) // false
BigDecimal.isPositive(amount) // true
BigDecimal.isNegative(amount) // false
BigDecimal.isInteger(amount) // false
```

Use decimal comparisons rather than converting to numbers. Bounds passed to `between` are inclusive. The sign and integer predicates inspect the decimal value, independent of its stored scale.

## Normalization and equality

```ts
const padded = BigDecimal.make(87500n, 4)
const compact = padded.normalize()
padded.equals(BigDecimal.make(875n, 2)) // true
BigDecimal.format(compact) // "8.75"
BigDecimal.toExponential(compact) // "8.75e+0"
```

Normalization removes unnecessary trailing coefficient zeros and adjusts the scale. Numeric equality already ignores those representation differences; normalize when you need a compact representation, not as a prerequisite for comparison.
