# BigDecimal

`BigDecimal` works in VibeLang exactly as in Effect. Use it for decimal arithmetic where binary floating point loses information, such as prices imported as decimal strings.

## Representing decimal values

```vibe
// run: Console.log(BigDecimal.format(unitPrice)); Console.log(BigDecimal.format(bulkPrice))
const unitPrice = BigDecimal.make(875n, 2)
const bulkPrice = BigDecimal.make(875n, -1)
BigDecimal.format(unitPrice) // "8.75"
BigDecimal.format(bulkPrice) // "8750"
```

```output
8.75
8750
```

A value is an integer coefficient and a scale, meaning coefficient × 10^-scale: a positive scale places digits after the decimal point, a negative one adds integer zeros. Precision is arbitrary up to implementation and memory limits, not JavaScript's number precision.

## Constructing decimals

```vibe
// run: Console.log(parsed.isSome(), invalid.isNone(), BigDecimal.format(whole), fromNumber.isSome())
const whole = BigDecimal.fromBigInt(42n)
const parsed = BigDecimal.fromString("18.625")
const invalid = BigDecimal.fromString("eighteen")
const fromNumber = BigDecimal.fromNumber(2.5)

parsed.isSome() // true
invalid.isNone() // true
BigDecimal.format(whole) // "42"
```

```output
true true 42 true
```

`make` takes a coefficient and scale; `fromBigInt` uses scale zero. `fromString` and `fromNumber` return an `Option`, so invalid input stays data. Prefer strings for external decimals: converting a number cannot recover digits floating point already lost.

## Safe and unsafe constructors

```vibe
// run: Console.log(BigDecimal.format(parsePrice("18.625")), BigDecimal.format(fixedRate)); try { parsePrice("invalid") } catch (error) { Console.log(error._tag, error.input) }
error InvalidPrice {
  input: string
}

function parsePrice(input: string) {
  return match (BigDecimal.fromString(input)) {
    when Option.Some(let price): price;
    when Option.None: throw new InvalidPrice({ input });
  }
}
// BigDecimal.BigDecimal!InvalidPrice!

const fixedRate = BigDecimal.fromNumberUnsafe(1.25)
```

```output
18.625 1.25
InvalidPrice invalid
```

`fromStringUnsafe` throws for invalid text, `fromNumberUnsafe` for non-finite numbers. Use an unsafe constructor for a known valid constant; validate input with the safe constructor and fail explicitly.

## Performing arithmetic

```vibe
// run: Console.log(JSON.stringify([BigDecimal.format(total), BigDecimal.format(total.sum(discount)), total.divide(quantity).map(BigDecimal.format).getOrNull(), total.remainder(quantity).map(BigDecimal.format).getOrNull(), BigDecimal.sign(BigDecimal.negate(total)), BigDecimal.format(BigDecimal.abs(BigDecimal.negate(total)))]))
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

```output
["32.5","35","8.125","0.5",-1,"32.5"]
```

`sum`, `subtract`, and `multiply` return new decimals. For a zero divisor, `divide` and `remainder` return `None`, and `divideUnsafe` and `remainderUnsafe` throw. A nonterminating quotient follows the library's precision policy: arbitrary precision does not mean an infinitely long quotient.

## Comparing values

```vibe
// run: Console.log(JSON.stringify([amount.isLessThan(limit), amount.isLessThanOrEqualTo(amount), limit.isGreaterThan(amount), limit.isGreaterThanOrEqualTo(limit), BigDecimal.format(amount.min(limit)), BigDecimal.format(amount.max(limit)), amount.between({ minimum: BigDecimal.fromBigInt(0n), maximum: limit }), BigDecimal.isZero(amount), BigDecimal.isPositive(amount), BigDecimal.isNegative(amount), BigDecimal.isInteger(amount)]))
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

```output
[true,true,true,true,"32.5","40",true,false,true,false,false]
```

Compare decimals directly, not as numbers. `between` bounds are inclusive. The sign and integer predicates test the value, whatever its stored scale.

## Normalization and equality

```vibe
// run: Console.log(padded.equals(BigDecimal.make(875n, 2)), BigDecimal.format(compact), BigDecimal.toExponential(compact))
const padded = BigDecimal.make(87500n, 4)
const compact = padded.normalize()
padded.equals(BigDecimal.make(875n, 2)) // true
BigDecimal.format(compact) // "8.75"
BigDecimal.toExponential(compact) // "8.75e+0"
```

```output
true 8.75 8.75e+0
```

Normalization strips trailing coefficient zeros and adjusts the scale. Numeric equality already ignores that difference: normalize for a compact representation, not before comparing.
