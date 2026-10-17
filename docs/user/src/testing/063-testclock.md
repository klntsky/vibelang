# TestClock

`TestClock` replaces live time with time controlled by your test. Use it to check sleeps, timeouts, recurring work, and clock reads without waiting for wall-clock time. Import it from `effect/testing`, and use `Clock` rather than JavaScript time reads in application code.

## Configure the test runner

The [starter](../getting-started/004-installation.md) includes Vitest 5.0.5,
`@effect/vitest` 4.0.5, and the local `@effeck/vite` plugin. Run
`bun run test` to compile and execute `src/**/*.test.effeck` directly. Tests need no
prior `dist/` build. Keep configuration in TypeScript:

```ts
// TypeScript (.ts)
// vitest.config.ts
import { defineConfig } from "vitest/config"
import effeck from "@effeck/vite"

export default defineConfig({
  plugins: [effeck()],
  test: { include: ["src/**/*.test.effeck"] }
})
```

Register `it.effect` at the top level or inside `describe`. The callback is a
program, so Effeck calls inside it run. `assert` and `expect` from
`@effect/vitest` are exports of an Effect package, so call them directly, as in
`assert.strictEqual(actual, expected)` or `expect(actual).toBe(expected)`. A
failed assertion is a defect: it fails the test, and its stack points to the
original `.effeck` line through the plugin's maps. Import them from
`@effect/vitest`; the same functions imported from `vitest` are plain
TypeScript, which is EK4401 inside a test. The examples below use a local
`check` helper, which works the same way.

## Controlling virtual time

```ts
import { Fiber, Option } from "effect"
import { TestClock } from "effect/testing"
import { it } from "@effect/vitest"

function check(condition: boolean, message: string) {
  if (!condition) throw new Error(message)
}

it.effect("a deadline returns the fallback", () => {
  const pending = fork(() => Effect.timeoutOrElse(() => {
    Effect.sleep("10 minutes")
    return Option.some("finished")
  }, {
    duration: "30 seconds",
    orElse: () => Option.none<string>()
  }))

  TestClock.adjust("30 seconds")
  const result = Fiber.join(pending)
  check(Option.isNone(result), "expected the timeout fallback")
})
```

`it.effect` provides the test services, including virtual time. `adjust` advances by a duration and runs sleepers whose deadlines fall within that interval; `setTime` sets an absolute timestamp in milliseconds. Fork the timed work first so the test's own fiber remains free to adjust the clock. The block owns `pending` and interrupts it if the test exits early.

```ts
import { Clock } from "effect"
import { TestClock } from "effect/testing"

function standaloneClockExample() {
  return Effect.provide(() => {
    TestClock.setTime(5_000)
    TestClock.adjust("2 seconds")
    return Clock.currentTimeMillis() // 7000
  }, TestClock.layer())
}
```

Outside the test harness, explicitly provide `TestClock.layer()`. The virtual clock does not advance merely because real time passes. Sleeping on the test's own fiber before adjusting time leaves the test waiting for an adjustment it cannot perform.

## Advancing recurring computations

```ts
import { Option, Queue } from "effect"
import { TestClock } from "effect/testing"
import { it } from "@effect/vitest"

it.effect("emits once per interval", () => {
  const queue = Queue.unbounded<string>()
  fork(() => {
    while (true) {
      Effect.sleep("15 minutes")
      Queue.offer(queue, "refresh")
    }
  })

  TestClock.adjust("14 minutes")
  check(Option.isNone(Queue.poll(queue)), "refresh arrived early")

  TestClock.adjust("1 minute")
  check(Queue.take(queue) === "refresh", "missing first refresh")
  check(Option.isNone(Queue.poll(queue)), "duplicate refresh")

  TestClock.adjust("15 minutes")
  check(Queue.take(queue) === "refresh", "missing second refresh")
  check(Option.isNone(Queue.poll(queue)), "duplicate second refresh")
})
```

Use the `check` helper from the first example. Advancing to each boundary lets the loop schedule its next sleep before the next adjustment. The queue makes each recurrence observable without relying on real-time delays, and the background loop stops when the test block exits.

## Checking clock-dependent behavior

```ts
import { Clock } from "effect"
import { TestClock } from "effect/testing"
import { it } from "@effect/vitest"

function isExpired(createdAt: number, lifetimeMillis: number) {
  return Clock.currentTimeMillis() - createdAt >= lifetimeMillis
}

it.effect("expires at the boundary", () => {
  const createdAt = Clock.currentTimeMillis()
  TestClock.adjust("9 seconds")
  check(!isExpired(createdAt, 10_000), "expired too soon")
  TestClock.adjust("1 second")
  check(isExpired(createdAt, 10_000), "should expire at ten seconds")
})
```

Calls to `Clock.currentTimeMillis()` read the supplied clock and remain deterministic under the test layer. `Date.now()`, zero-argument `new Date()`, and platform timers are not application time sources in Effeck. Testing just before and at a boundary catches comparison mistakes that checking only a later timestamp misses.

## Timed waits and deferred results

```ts
import { Deferred, Fiber } from "effect"
import { TestClock } from "effect/testing"
import { it } from "@effect/vitest"

it.effect("completes a deferred after a delay", () => {
  const ready = Deferred.make<number>()
  const worker = fork(() => {
    Effect.sleep("6 seconds")
    Deferred.succeed(ready, 42)
  })

  TestClock.adjust("6 seconds")
  const value = Deferred.await(ready)
  Fiber.join(worker)
  check(value === 42, "unexpected deferred value")
})
```

The sleep precedes completion in the worker, so the test checks delayed completion. After advancing time, await the Deferred or join the worker to synchronize with the observable outcome. A fork alone says work has started, not that its result is ready.
