# Resourceful Streams

A stream can keep a connection, file, or subscription open while its elements are consumed. Acquire during stream execution and attach release to the stream's Scope, so normal completion, early stopping, failure, and interruption all close the resource.

## Acquiring for the stream lifetime

```ts
function openReader() {
  Effect.log("reader opened")
  return { lines: ["header", "entry-a", "entry-b"] }
}

const lines = Stream.scoped(
  Stream.flatMap(
    Stream.fromEffect(() => Effect.acquireRelease(
      () => openReader(),
      () => Effect.log("reader closed")
    )),
    (reader) => Stream.fromArray(reader.lines)
  )
)

function readLines() {
  return Stream.runCollect(lines)
}
```

`Effect.acquireRelease` registers its release action in Scope. `Stream.fromEffect` defers acquisition until consumption, and `Stream.scoped` manages that Scope. Wrap the complete resource-using pipeline so the resource is available to all the operations that use it. Defining `lines` opens nothing; each consumption acquires again.

## Release after stopping or interruption

```ts
function readHeader() {
  return Stream.runCollect(Stream.take(lines, 1))
}
// Returns ["header"] and closes the reader.

function readWithDeadline() {
  return Effect.timeout(() => {
    Stream.runForEach(lines, (line) => {
      Effect.sleep("1 second")
      Effect.log(line)
    })
  }, "100 millis")
}
```

The early consumer closes the reader even though it does not use every line. The timeout interrupts consumption and waits for release before it reports failure. A `using` acquisition inside a function that merely returns a Stream would close when that function returns; acquire inside the stream instead. Acquisition without `using` belongs to the supplied Scope, as required for this lifetime.

## Registering finalizers

```ts
const audited = Stream.ensuring(lines, () => {
  Effect.log("reader resources released")
  Effect.log("consumption finished")
})

function readAudited() {
  return Stream.runCollect(audited)
}
```

`ensuring` runs its program after the wrapped stream's own finalizers, on every exit. Pass a program so logging runs at finalization, rather than during construction. The finalizer's expected error channel must be `never`; handle expected cleanup errors inside it. Defects during cleanup remain part of the reported cause.

## Coming from Effect

- Keep `Stream.scoped`, `Stream.fromEffect`, and `Stream.ensuring` as stream lifetime operators.
- Use `using` for resources owned by the consuming block. Leave acquisitions inside a stream's resource program without `using` when the stream owns them.
