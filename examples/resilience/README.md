# Resilience

A transient report failure retries on a spaced `Schedule`, with two retries.
A cache races the network, a timeout recovers to its tag, and a block-owned
`fork` runs the report while the caller works before `Fiber.join`.

From this directory, with `vibelang` on `PATH`:
```sh
bun install
bun run build
bun run start
bun run test
```

Vitest runs `src/main.test.vibe` through `vibelang-vite` without a prior
build. Assertion stacks point to `.vibe` lines. The tests use `it.effect` and
`TestClock.adjust`; they never wait for live time.
A Deferred checks that retries remain pending before their scheduled deadline.
The deadline function in Effect TypeScript is:
```ts
const deadline = Effect.fnUntraced(function* () {
  return yield* slowReport().pipe(
    Effect.timeout("20 millis"),
    Effect.catch((error) => Effect.succeed(error._tag))
  )
})
```

The fixture checks the cache winner, timeout tag, and three total attempts.
