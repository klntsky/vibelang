# Sensor pipeline

A lazy producer supplies sensor readings to `Stream.filter`, `Stream.map`,
and `Stream.take`. A `for...of` loop consumes the resulting Stream.
The producer closes after the second matching reading, before reading 30 C.
Its finalizer logs closure, including when downstream stops early.

From this directory, with `esc` on `PATH`:
```sh
bun install
bun run build
bun run start
```

Features: `function*`, `yield`, producer-to-Stream conversion, and stream iteration.
The consuming loop in Effect TypeScript is:
```ts
const consume = Stream.runForEach(alerts, (alert) =>
  Console.log(`temperature alert: ${alert}`)
)
```

EffectScript converts the producer when `Stream.filter` expects a Stream.
`Stream.take` closes upstream before delivering its final buffered element,
so the closure message precedes the final alert in the output fixture.
