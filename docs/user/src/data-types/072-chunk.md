# Chunk

`Chunk` works in Effeck exactly as in Effect. It is an immutable sequence optimized for repeated concatenation; use an ordinary array when concatenation is not the main workload.

## Choosing sequence storage

```effeck
function collectBatches(batches: ReadonlyArray<ReadonlyArray<string>>) {
  let result = Chunk.empty<string>()
  for (const batch of batches) {
    result = result.appendAll(Chunk.fromIterable(batch))
  }
  return result
}
```

A chunk preserves sequence order and shares internal structure across updates. That makes repeated concatenation useful without repeatedly copying the whole prefix. Its bookkeeping adds overhead, so it is not a general replacement for arrays.

## Building chunks

```effeck
const empty = Chunk.empty<string>()
const pair = Chunk.make("north", "south")
const fromArray = Chunk.fromIterable(["east", "west"])
const fromSet = Chunk.fromIterable(new Set(["center"]))
```

`make` returns a nonempty chunk, while `empty` represents a sequence with no elements. `fromIterable` accepts arrays, sets, and other iterables; it reuses input arrays and existing chunks. Copy an array before passing it when other code can still mutate it. Immutability applies to the sequence, not to mutable objects stored as its elements.

## Reusing an array unsafely

```effeck
function ownedBatch() {
  const values = ["queued", "running"]
  return Chunk.fromArrayUnsafe(values)
  // Ownership is handed to the chunk. Never mutate values afterward.
}
```

`fromArrayUnsafe` avoids copying its input. Any later mutation of that array breaks the chunk's immutability guarantees, including through aliases held elsewhere. Use `Chunk.fromIterable([...values])` when you need a copy; `fromIterable(values)` alone also reuses an array.

## Concatenating and dropping elements

```effeck
const original = Chunk.make("draft", "review")
const extended = original.appendAll(Chunk.make("published"))
const remaining = extended.drop(1)
original.toReadonlyArray() // ["draft", "review"]
remaining.toReadonlyArray() // ["review", "published"]
```

`appendAll` preserves the order of both chunks. `drop` removes a prefix and returns the remaining sequence. Neither operation changes its inputs.

## Checking structural equality

```effeck
const first = Chunk.make({ code: "A" }, { code: "B" })
const same = Chunk.make({ code: "A" }, { code: "B" })
const reversed = Chunk.make({ code: "B" }, { code: "A" })
Equal.equals(first, same) // true
Equal.equals(first, reversed) // false
```

Equality compares lengths and corresponding elements using Effect's equality rules. Distinct chunk objects can therefore be equal, including when their plain object elements were created separately. Element order matters.

## Converting to arrays

```effeck
const nonempty = Chunk.make(7, 8).toReadonlyArray()
const none = Chunk.toReadonlyArray(Chunk.empty<number>())
const mutableCopy = [...nonempty]
mutableCopy.push(9)
```

`toReadonlyArray` exposes a readonly view, preserving nonempty information when the input is known to be nonempty. Do not cast the result to a mutable array. Copy it with spread when a JavaScript consumer needs mutable storage.
