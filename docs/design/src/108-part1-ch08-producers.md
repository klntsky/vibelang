### 8. Producers: streams of values

A `function*` in VibeLang is a **producer**: a function that yields values one at a time and can do anything a VibeLang function does between yields, including calling services and waiting. `for...of` consumes it. The comparison below builds the same reusable, lazy line source over the same `Files` service on both sides.

```ts effect
const lines = (path: string) =>
  Stream.unwrap(Effect.gen(function* () {
    const files = yield* Files
    const file = yield* files.open(path)                // closes when the stream ends
    return Stream.fromEffectRepeat(file.readLine()).pipe(
      Stream.takeWhile((line): line is string => line !== undefined)
    )
  }))

const firstErrors = Effect.fn("firstErrors")(function* (path: string) {
  return yield* lines(path).pipe(
    Stream.filter((line) => line.startsWith("ERROR")),
    Stream.take(3),
    Stream.runCollect
  )
})
```

```ts es
function* lines(path: string) {
  using file = Files.open(path)
  while (true) {
    const line = file.readLine()        // a service call: may wait on I/O
    if (line === undefined) return
    yield line
  }
}

function firstErrors(path: string) {
  const found: string[] = []
  for (const line of lines(path)) {
    if (line.startsWith("ERROR")) found.push(line)
    if (found.length === 3) break       // the file closes here
  }
  return found
}
// string[]!IOError!Files
```

How a producer behaves:

- **Lazy.** Calling `lines(path)` runs none of its body. The file opens when the loop asks for the first line.
- **One value at a time.** Each `yield` hands one value to the loop body. When the body finishes, the producer continues to the next `yield`.
- **Cleanup on early exit.** `break`, `return`, or an error in the loop body stops the producer, and its `using` and `finally` run before the loop exits. Here the file is closed after the third error line.
- **One pass per run.** Iterating the same producer again starts it again from the beginning.
- **Constant stack.** A loop over a million elements uses the same stack depth as a loop over three (Chapter 26).

A JavaScript generator could not do this: its body runs synchronously inside `next()`, so it cannot wait for a service between values. A VibeLang producer can, because each `yield` is a control operation that the loop handles (Chapter 9 shows the machinery, and Part III how it runs). For the same reason, `Effect.gen`, `Option.gen`, and other generator-based APIs cannot be written in `.vibe` files: there, `function*` declares a producer (Chapter 10).

#### What `for...of` does depends on what it iterates

| Iterated value | What `for...of` does |
| --- | --- |
| A producer | Runs it under a `Yield` handler: the loop body runs once per `yield` |
| An Effect `Stream` | Pulls the stream chunk by chunk; leaving the loop closes the stream |
| Any other iterable: an array, a `Map`, a `Set`, a string | JavaScript's loop, unchanged |

The choice is made from the static type, like every other call rule (Chapter 13).

#### Producers and Effect streams

To TypeScript, a producer is a `Stream`: the declaration file types `lines` as `(path: string) => Stream.Stream<string, IOError, Files>`. Inside VibeLang, a producer converts to a `Stream` wherever a `Stream` is expected, so Stream's operators apply directly:

```ts es
function errorCount(path: string) {
  return lines(path).filter((line) => line.startsWith("ERROR")).runCount()
}
// number!IOError!Files
```

The conversion runs the producer on a fiber of its own and hands over each element only when the stream pulls it, through a handoff with no buffer, so a pull from any fiber works and the producer never runs ahead of its consumer. Closing the stream interrupts that fiber, which runs the producer's cleanup.

The other direction needs no adapter. Stream's own functions take callbacks that return Effects, so VibeLang callbacks work there, and `for...of` iterates a `Stream` directly:

```ts es
function auditAll(events: Stream.Stream<Event, EventError>) {
  for (const event of events) Audit.record(event)
}
// void!EventError!Audit
```
