# HashSet

`HashSet` and `MutableHashSet` work in VibeLang exactly as in Effect. They store unique, unordered values by Effect's equality and hashing rules; use them for frequent membership checks and set operations.

## Choosing immutable or mutable sets

```vibe
// run: Console.log(snapshot.has("red"), revised.has("red")); Console.log(JSON.stringify(Array.from(buildLocal()).sort()))
const snapshot = HashSet.make("green", "blue")
const revised = snapshot.add("red")
snapshot.has("red") // false
revised.has("red") // true

function buildLocal() {
  const working = MutableHashSet.empty<string>()
  MutableHashSet.add(working, "green")
  MutableHashSet.add(working, "blue")
  return HashSet.fromIterable(working)
}
```

```output
false true
["blue","green"]
```

Immutable updates keep earlier versions, so snapshots suit sharing between fibers. Mutable sets update in place and suit local accumulation with one owner. For indexed access or required insertion order, use an array; for key-value pairs, a hash map.

## Equality and uniqueness

```vibe
// run: Console.log(Equal.equals(a, b), HashSet.make(a, b).size(), new Set([a, b]).size)
const a = { region: "west", active: true }
const b = { region: "west", active: true }
Equal.equals(a, b) // true
HashSet.make(a, b).size() // 1
new Set([a, b]).size // 2
```

```output
true 1 2
```

Plain objects compare structurally in Effect 4.0.6, so separately allocated records can count as one member. Every compared field matters: different request IDs make two records distinct. Custom equality uses `Equal.Equal` with a consistent `Hash.Hash`; keep members immutable after insertion so their hashes stay valid.

## Constructing and changing immutable sets

```vibe
// run: Console.log(JSON.stringify([Array.from(initial).sort(Order.Number), Array.from(changed).sort(Order.Number), Array.from(together).sort(Order.Number), Array.from(common).sort(Order.Number), Array.from(exclusive).sort(Order.Number), Array.from(large).sort(Order.Number), Array.from(labels).sort(), sum, empty.size()]))
const empty = HashSet.empty<number>()
const initial = HashSet.fromIterable([2, 4, 4, 6])
const changed = initial.add(8).remove(2)
const other = HashSet.make(6, 10)
const together = changed.union(other)
const common = changed.intersection(other)
const exclusive = changed.difference(other)
const large = together.filter((n) => n > 5)
const labels = large.map((n) => `item-${n}`)
const sum = initial.reduce(0, (total, n) => total + n)
```

```output
[[2,4,6],[4,6,8],[4,6,8,10],[6],[4,8],[6,8,10],["item-10","item-6","item-8"],12,0]
```

`empty`, `make`, and `fromIterable` construct sets. `add`, `remove`, union, intersection, and difference return new sets, so method chains never change the original.

## Querying membership and size

```vibe
// run: Console.log(allowed.has("write"), allowed.size(), allowed.some((name) => name.startsWith("a")), allowed.every((name) => name.length > 0), HashSet.make("read").isSubset(allowed))
const allowed = HashSet.make("read", "write", "audit")
allowed.has("write") // true
allowed.size() // 3
allowed.some((name) => name.startsWith("a")) // true
allowed.every((name) => name.length > 0) // true
HashSet.make("read").isSubset(allowed) // true
```

```output
true 3 true true true
```

`has` tests one member; `size` counts unique members. `some` and `every` run predicates over members; `isSubset` tests whether one set is contained in another. Traversal order is unspecified.

## Updating mutable sets

```vibe
// run: const result = uniqueRegions(["west", "west", ""]); Console.log(result.count, result.containsUnknown, JSON.stringify(result.values.sort()))
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

```output
2 true ["unknown","west"]
```

`MutableHashSet` also has `make`, `empty`, and `fromIterable`; its `add`, `remove`, and `clear` change the set in place. Keep mutation inside one owned operation; a mutable collection alone does not coordinate concurrent workflows.

## Balancing performance and persistence

```vibe
// run: Console.log(JSON.stringify(snapshots([2, 2, 4]).map((set) => set.size())))
function snapshots(values: ReadonlyArray<number>) {
  let current = HashSet.empty<number>()
  const history = [current]
  for (const value of values) {
    current = current.add(value)
    history.push(current)
  }
  return history
}
```

```output
[0,1,1,2]
```

Persistent sets share storage between versions; mutable sets skip that allocation when you need no earlier versions. With well-distributed hashes, lookup, insertion, and removal cost near-constant time on average; traversals and set-wide operations scale with the members visited. Poor custom hashes add collision work, so measure the real workload instead of assuming constant time.

## Converting to JavaScript collections

```vibe
// run: Console.log(JSON.stringify([array.sort(), copy.sort(), Array.from(native).sort()])); report()
const regions = HashSet.make("west", "east")
const array = Array.from(regions)
const copy = [...regions]
const native = new Set(regions)

function report() {
  for (const region of Array.from(regions).sort()) Console.log(region)
}
```

```output
[["east","west"],["east","west"],["east","west"]]
east
west
```

Both variants implement `Iterable`, so spread, `Array.from`, and `for...of` work. A native set uses JavaScript equality from then on: conversion does not carry Effect's equality over. Avoid repeated array conversions in hot loops.
