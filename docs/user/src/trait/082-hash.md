# Hash

`Hash` works in VibeLang exactly as in Effect. It gives equality-aware collections a numeric fingerprint for a value, allowing them to locate likely matches before performing a full comparison.

## Connecting hashing to equality

```vibe
// run: Console.log(JSON.stringify([Equal.equals(first, same), Hash.hash(first) === Hash.hash(same)]))
const first = { section: "billing", page: 2 }
const same = { section: "billing", page: 2 }
Equal.equals(first, same) // true
Hash.hash(first) === Hash.hash(same) // true
// Output: [true,true]
```

Equal values must have equal hashes. A hash collision does not prove equality: different values can share a hash, so collections still compare candidate values. Hashes are implementation details for lookup, not cryptographic digests or stable identifiers for storage.

## Implementing the hashing protocol

```vibe
// run: Console.log(JSON.stringify([Hash.hash(new GridCell(3, 7)) === Hash.hash(new GridCell(3, 7)), Equal.equals(new GridCell(3, 7), new GridCell(3, 8))]))
class GridCell implements Equal.Equal {
  constructor(readonly row: number, readonly column: number) {}

  [Equal.symbol](other: Equal.Equal): boolean {
    return other instanceof GridCell &&
      this.row === other.row && this.column === other.column
  }

  [Hash.symbol]() {
    return Hash.hash([this.row, this.column])
  }
}

Hash.hash(new GridCell(3, 7)) === Hash.hash(new GridCell(3, 7)) // true
Equal.equals(new GridCell(3, 7), new GridCell(3, 8)) // false
// Output: [true,false]
```

A `[Hash.symbol]` method returns a number and must be synchronous. `Hash.hash` handles primitive and structural inputs, so hashing a tuple of identity fields is a convenient implementation. `Equal.Equal` requires hashing as part of its interface.

The method compiles to a plain JavaScript method, because `[Hash.symbol]` is typed as a plain function. Its body must perform no Effect operation and cannot fail with an expected error; otherwise it is error EK4301.

## Keeping equal values hash-consistent

```vibe
// run: Console.log(JSON.stringify([cells.size(), Hash.hash(first) === Hash.hash(equivalent)]))
const first = new GridCell(3, 7)
const equivalent = new GridCell(3, 7)
const cells = HashSet.make(first, equivalent)
cells.size() // 1
Hash.hash(first) === Hash.hash(equivalent) // true
// Output: [1,true]
```

Hash the same identity fields that equality compares. Including a field that equality ignores can send equal values to different buckets and break collection lookup. Using fewer fields is correct but increases collisions; keep hash-relevant fields immutable after insertion because hashes can be cached.
