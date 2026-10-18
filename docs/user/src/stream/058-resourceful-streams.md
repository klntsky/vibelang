# Resourceful Streams

A stream can keep a connection, file, or subscription open while its elements are consumed. Acquire during stream execution and attach release to the stream's Scope, so normal completion, early stopping, failure, and interruption all close the resource.

## Acquiring for the stream lifetime

```vibe
// run: Console.log(JSON.stringify(readLines()))
function openReader() {
  Console.log("reader opened")
  return { lines: ["header", "entry-a", "entry-b"] }
}

const lines = ((Stream.fromEffect(Effect.acquireRelease(
      () => openReader(),
      () => Console.log("reader closed")
    ))).flatMap((reader) => Stream.fromArray(reader.lines))).scoped()

function readLines() {
  return lines.runCollect()
}
// Output: reader opened
// Output: reader closed
// Output: ["header","entry-a","entry-b"]
```

`Effect.acquireRelease` registers its release action in Scope. `Stream.fromEffect` defers acquisition until consumption, and `Stream.scoped` manages that Scope. Wrap the complete resource-using pipeline so the resource is available to all the operations that use it. Defining `lines` opens nothing; each consumption acquires again.

## Release after stopping or interruption

```vibe
// run: Console.log(JSON.stringify(readHeader())); try { readWithDeadline() } catch (error) { Console.log(error._tag) }
function readHeader() {
  return lines.take(1).runCollect()
}
// Returns ["header"] and closes the reader.

function readWithDeadline() {
  return timeout("100 millis") {
    lines.runForEach((line) => {
      sleep("1 second")
      Console.log(line)
    })
  }
}
// Output: reader opened
// Output: reader closed
// Output: ["header"]
// Output: reader opened
// Output: reader closed
// Output: TimeoutError
```

The early consumer closes the reader even though it does not use every line. The timeout interrupts consumption and waits for release before it reports failure. A `using` acquisition inside a function that merely returns a Stream would close when that function returns; acquire inside the stream instead. Acquisition without `using` belongs to the supplied Scope, as required for this lifetime.

## Registering finalizers

```vibe
// run: Console.log(JSON.stringify(readAudited()))
const audited = lines.ensuring {
  Console.log("reader resources released")
  Console.log("consumption finished")
}

function readAudited() {
  return audited.runCollect()
}
// Output: reader opened
// Output: reader closed
// Output: reader resources released
// Output: consumption finished
// Output: ["header","entry-a","entry-b"]
```

`ensuring` runs its program after the wrapped stream's own finalizers, on every exit. Pass a program so logging runs at finalization, rather than during construction. The finalizer's expected error channel must be `never`; handle expected cleanup errors inside it. Defects during cleanup remain part of the reported cause.

## Coming from Effect

- Keep `Stream.scoped`, `Stream.fromEffect`, and `Stream.ensuring` as stream lifetime operators.
- Use `using` for resources owned by the consuming block. Leave acquisitions inside a stream's resource program without `using` when the stream owns them.
