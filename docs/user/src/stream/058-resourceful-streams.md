# Resourceful Streams

A stream can hold a connection, file, or subscription open while its elements are consumed. Acquire during stream execution and attach release to the stream's Scope, so normal completion, early stopping, failure, and interruption all close the resource.

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

`Effect.acquireRelease` registers its release action in Scope, `Stream.fromEffect` defers acquisition until consumption, and `Stream.scoped` manages that Scope. Wrap the whole pipeline that uses the resource so every stage can reach it. Defining `lines` opens nothing; each consumption acquires again.

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

The timeout interrupts consumption and waits for release before reporting failure. A `using` acquisition in a function that only returns a Stream closes when that function returns, so acquire inside the stream, without `using`: that acquisition belongs to the supplied Scope, as this lifetime requires.

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

`ensuring` runs its program after the wrapped stream's own finalizers, on every exit. Pass a program so the logging runs at finalization, not construction. The finalizer's expected error channel must be `never`, so handle expected cleanup errors inside it; cleanup defects stay in the reported cause.

## Coming from Effect

- Keep `Stream.scoped`, `Stream.fromEffect`, and `Stream.ensuring` as stream lifetime operators.
- Use `using` for resources the consuming block owns. Acquire resources the stream owns in its resource program, without `using`.
