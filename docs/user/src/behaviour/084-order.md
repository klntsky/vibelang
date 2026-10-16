# Order

`Order` works in EffectScript exactly as in Effect. It defines reusable comparison rules returning `-1`, `0`, or `1`, so sorting and range checks can share the same definition of before, tied, and after.

## Using built-in orderings

```ts
import { Order } from "effect"

Order.String("amber", "violet") // -1
Order.Number(7, 7) // 0
Order.BigInt(11n, 3n) // 1
Order.Date(new Date("2026-01-01"), new Date("2026-02-01")) // -1
```

An `Order<A>` compares two values of A synchronously. Built-in orders cover strings, numbers, booleans, bigints, and dates. Use valid dates and numbers appropriate to a total order; `NaN` does not have ordinary numeric ordering semantics.

## Sorting values

```ts
import { Array as Arr, Order } from "effect"

const input = [9, 2, 6]
const sorted = Arr.sort(input, Order.Number)
// sorted: [2, 6, 9]; input: [9, 2, 6]

const mutableCopy = [...input]
mutableCopy.sort(Order.Number)
// mutableCopy: [2, 6, 9]
```

Effect's array `sort` returns a sorted copy. JavaScript's array method sorts its receiver in place. Both accept an order as a comparator; custom callbacks must remain synchronous and must not fail through E.

## Deriving comparison rules

```ts
import { Order } from "effect"

interface Task {
  readonly priority: number
  readonly title: string
}

const byPriority = Order.mapInput(Order.Number, (task: Task) => task.priority)
const byTitle = Order.mapInput(Order.String, (task: Task) => task.title)

byPriority(
  { priority: 1, title: "Publish" },
  { priority: 3, title: "Review" }
) // -1
```

`mapInput` extracts a comparable value from each operand, then delegates to the underlying order. The projection defines which field matters. It does not mutate or transform the objects being sorted.

## Combining comparison rules

```ts
import { Array as Arr, Order } from "effect"

interface Task {
  readonly priority: number
  readonly title: string
}

const byPriority = Order.mapInput(Order.Number, (task: Task) => task.priority)
const byTitle = Order.mapInput(Order.String, (task: Task) => task.title)
const taskOrder = Order.combine(byPriority, byTitle)
const tasks = Arr.sort([
  { priority: 2, title: "Review" },
  { priority: 1, title: "Publish" },
  { priority: 2, title: "Archive" }
], taskOrder)
// Publish, Archive, Review
```

`combine` uses the first rule and consults the second only for a tie. Combine more rules with `combineAll` when several fields break ties. Rule order therefore determines sorting priority.

## Reversing an ordering

```ts
import { Array as Arr, Order } from "effect"

const descending = Order.flip(Order.Number)
Arr.sort([5, 12, 3], descending) // [12, 5, 3]
```

`flip` reverses an order. Flip one component before combining to reverse only that criterion, or flip the combined order to reverse all criteria.

## Checking comparisons

```ts
import { Order } from "effect"

const less = Order.isLessThan(Order.Number)
const greater = Order.isGreaterThan(Order.Number)
const atMost = Order.isLessThanOrEqualTo(Order.Number)
const atLeast = Order.isGreaterThanOrEqualTo(Order.Number)
less(3, 7) // true
greater(7, 3) // true
atMost(7, 7) // true
atLeast(7, 7) // true
```

These helpers turn an order into boolean comparisons. They work with derived object orders as well as primitive orders. Equality here means a comparison tie, which can intentionally ignore fields.

## Selecting extrema

```ts
import { Order } from "effect"

Order.min(Order.Number)(13, 8) // 8
Order.max(Order.Number)(13, 8) // 13
Order.min(Order.flip(Order.Number))(13, 8) // 13
```

`min` and `max` select according to the supplied order. With a reversed order, the numeric meaning also reverses. An object order returns one of the original objects rather than just its projected field.

## Clamping values and testing ranges

```ts
import { Order } from "effect"

const bounds = { minimum: 10, maximum: 50 }
const clamp = Order.clamp(Order.Number)(bounds)
const inside = Order.isBetween(Order.Number)(bounds)
clamp(4) // 10
clamp(30) // 30
clamp(70) // 50
inside(10) // true
inside(51) // false
```

`clamp` replaces an out-of-range value with the nearest bound. `isBetween` tests the same inclusive interval without changing the value. Choose bounds ordered according to the supplied order.
