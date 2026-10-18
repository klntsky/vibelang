# Sensor pipeline

A lazy producer supplies sensor readings to `Stream.filter`, `Stream.map`,
and `Stream.take`. A `for...of` loop consumes the resulting Stream.
The producer closes after the second matching reading, before reading 30 C.
Its finalizer logs closure, including when downstream stops early.

From this directory, with `vibelang` on `PATH`:
```sh
bun install
bun run build
bun run start
```

Features: `function*`, `yield`, a producer as a Stream, and `for...of` over a Stream.
The consuming loop in Effect TypeScript is:
```ts
const consume = Stream.runForEach(alerts, (alert) =>
  Console.log(`temperature alert: ${alert}`)
)
```

A producer is a Stream, so `Stream.filter` takes it directly. The producer
hands over each reading only when the stream pulls it, so it never reads
ahead. After the second alert, the loop's next pull ends `Stream.take`, which
closes the stream and runs the producer's finalizer, so the closure message
follows the final alert in the output fixture.
