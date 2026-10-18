# SynchronizedRef

A `SynchronizedRef` serializes updates that need to run Effects. Use it when the next value depends on both the current value and work that can pause, such as refreshing a credential or loading a revision.

## Serializing effectful updates

```vibe
// run: Console.log(JSON.stringify(recordRevisions().toSorted((a, b) => a - b)))
function recordRevisions() {
  const revisions = SynchronizedRef.make<ReadonlyArray<number>>([])
  forEach([1, 2, 3], (id) => {
    SynchronizedRef.updateEffect(revisions, (current) => {
      sleep("10 millis")
      return [...current, id]
    })
  }, { concurrency: 3 })
  return SynchronizedRef.get(revisions)
}
// Output: [1,2,3]
```

The callbacks acquire the reference's update lock one at a time, even though three fibers call them concurrently. Each callback sees the value committed by the previous update, so all three revisions remain in the final array. Return a replacement immutable value rather than changing the stored array in place.

## Reading synchronized references

```vibe
// run: Console.log(JSON.stringify(inspect()))
function inspect() {
  const state = SynchronizedRef.make({ version: 0, ready: false })
  const before = SynchronizedRef.get(state)
  SynchronizedRef.set(state, { version: 1, ready: true })
  const after = SynchronizedRef.get(state)
  return { before, after }
}
// Output: {"before":{"version":0,"ready":false},"after":{"version":1,"ready":true}}
```

`get` reads the latest committed value; it does not wait for an in-progress update to compute a replacement. `set`, `update`, and `modify` are also available for synchronous changes. A separate read and later write do not become one atomic operation merely because the reference is synchronized.

## Keeping dependent asynchronous updates atomic

```vibe
// run: provide Versions = { loadAfter: (version) => { sleep("10 millis"); return version + 1 } } { Console.log(JSON.stringify(refreshTwice())) }; const state = SynchronizedRef.make(0); try { SynchronizedRef.updateEffect(state, () => { throw new RefreshFailed() }) } catch (error) { Console.log(error._tag) }; Console.log(SynchronizedRef.updateAndGetEffect(state, (n) => n + 1))
error RefreshFailed

service Versions {
  loadAfter(version: number): number!RefreshFailed
}

function refreshTwice() {
  const version = SynchronizedRef.make(0)
  function refresh() {
    return SynchronizedRef.updateAndGetEffect(version, (current) =>
      Versions.loadAfter(current))
  }
  return all([refresh(), refresh()], { concurrency: 2 })
}
// [number, number]!RefreshFailed!Versions
// Output: [1,2]
// Output: RefreshFailed
// Output: 1
```

The second update reads the first update's committed version before calling `loadAfter`. If a callback fails, it does not commit its replacement, and the lock is released. This protects the reference's update sequence; it does not roll back remote side effects, coordinate several references, or make recursive updates to the same reference safe.

## Coming from Effect

Call `updateEffect(ref, (current) => nextValue(current))` with an ordinary callback. Unlike the callback to `Ref.update`, this callback may pause and use services.
