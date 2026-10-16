# HashSet

`HashSet` and `MutableHashSet` work in EffectScript exactly as in Effect. They store unique, unordered values using Effect's equality and hashing rules; choose them for frequent membership checks and set operations.

## Choosing immutable or mutable sets

```ts
import { HashSet, MutableHashSet } from "effect"

const snapshot = HashSet.make("green", "blue")
const revised = HashSet.add(snapshot, "red")
HashSet.has(snapshot, "red") // false
HashSet.has(revised, "red") // true

function buildLocal() {
  const working = MutableHashSet.empty<string>()
  MutableHashSet.add(working, "green")
  MutableHashSet.add(working, "blue")
  return HashSet.fromIterable(working)
}
```

Immutable updates preserve earlier versions, making snapshots suitable for sharing between fibers. Mutable sets update in place and suit local accumulation with one owner. Use an array for indexed access or required insertion order, and a hash map for key-value associations.

## Equality and uniqueness

```ts
import { Equal, HashSet } from "effect"

const a = { region: "west", active: true }
const b = { region: "west", active: true }
Equal.equals(a, b) // true
HashSet.size(HashSet.make(a, b)) // 1
new Set([a, b]).size // 2
```

Plain objects compare structurally in Effect 4.0.5, so separately allocated records can count as one member. All compared fields matter: adding different request identifiers makes two records distinct. Custom equality uses `Equal.Equal` and a consistent `Hash.Hash` implementation; keep members immutable after insertion so their hashes remain valid.

## Constructing and changing immutable sets

```ts
import { HashSet } from "effect"

const empty = HashSet.empty<number>()
const initial = HashSet.fromIterable([2, 4, 4, 6])
const changed = pipe(initial, HashSet.add(8), HashSet.remove(2))
const other = HashSet.make(6, 10)
const together = HashSet.union(changed, other)
const common = HashSet.intersection(changed, other)
const exclusive = HashSet.difference(changed, other)
const large = HashSet.filter(together, (n) => n > 5)
const labels = HashSet.map(large, (n) => `item-${n}`)
const sum = HashSet.reduce(initial, 0, (total, n) => total + n)
```

`empty`, `make`, and `fromIterable` all construct sets. `add` and `remove` return updated sets; union, intersection, and difference return the corresponding set operation. A data pipeline chains immutable updates without changing the original.

## Querying membership and size

```ts
import { HashSet } from "effect"

const allowed = HashSet.make("read", "write", "audit")
HashSet.has(allowed, "write") // true
HashSet.size(allowed) // 3
HashSet.some(allowed, (name) => name.startsWith("a")) // true
HashSet.every(allowed, (name) => name.length > 0) // true
HashSet.isSubset(HashSet.make("read"), allowed) // true
```

`has` tests one member and `size` counts unique members. `some` and `every` run predicates over members; `isSubset` tests containment of one set in another. Traversal order is not part of the contract.

## Updating mutable sets

```ts
import { MutableHashSet } from "effect"

function uniqueRegions(regions: ReadonlyArray<string>) {
  const seen = MutableHashSet.fromIterable(regions)
  MutableHashSet.add(seen, "unknown")
  MutableHashSet.remove(seen, "")
  const count = MutableHashSet.size(seen)
  const containsUnknown = MutableHashSet.has(seen, "unknown")
  const values = Array.from(seen)
  MutableHashSet.clear(seen)
  return { count, containsUnknown, values }
}
```

`make`, `empty`, and `fromIterable` also exist on `MutableHashSet`. Its `add`, `remove`, and `clear` change the original object. Keep mutation within an owned operation; a mutable collection alone does not coordinate concurrent workflows.

## Balancing performance and persistence

```ts
import { HashSet } from "effect"

function snapshots(values: ReadonlyArray<number>) {
  let current = HashSet.empty<number>()
  const history = [current]
  for (const value of values) {
    current = HashSet.add(current, value)
    history.push(current)
  }
  return history
}
```

Persistent sets share storage between versions; mutable sets avoid persistent-update allocation when previous versions are unnecessary. With well-distributed hashes, lookup, insertion, and removal have near-constant average cost; traversals and set-wide operations scale with the members visited. Poor custom hashes increase collision work, so measure the actual workload rather than assuming every operation is constant time.

## Converting to JavaScript collections

```ts
import { HashSet } from "effect"

const regions = HashSet.make("west", "east")
const array = Array.from(regions)
const copy = [...regions]
const native = new Set(regions)

function report() {
  for (const region of regions) Effect.log(region)
}
```

Both variants implement `Iterable`, supporting spread, `Array.from`, and `for...of`. Native sets use JavaScript equality for future operations, so a conversion does not transfer Effect's equality behavior. Avoid repeated array conversions in hot loops.
