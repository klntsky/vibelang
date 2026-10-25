# TestClock

`TestClock` replaces live time with time your test controls, so you can check sleeps, timeouts, recurring work, and clock reads without waiting for wall-clock time. Import it from `effect/testing`.

## Configure the test runner

The [starter](../getting-started/004-installation.md) includes Vitest 5.0.5, `@effect/vitest` 4.0.6, and the local `vibelang-vite` plugin. `bun run test` compiles and runs `src/**/*.test.vibe` directly, with no prior `dist/` build. Keep configuration in TypeScript:

```ts
// TypeScript (.ts)
// vitest.config.ts
import { defineConfig } from "vitest/config"
import vibelang from "vibelang-vite"

export default defineConfig({
  plugins: [vibelang()],
  test: { include: ["src/**/*.test.vibe"] }
})
```

Register `it.effect` at the top level or inside `describe`; its callback is a program, so VibeLang calls inside it run. `assert` and `expect` from `@effect/vitest` are Effect package exports: call them directly, as in `assert.strictEqual(actual, expected)` or `expect(actual).toBe(expected)`. A failed assertion is a defect that fails the test, and the plugin's maps point its stack at the original `.vibe` line. Import them from `@effect/vitest`: from `vitest` they are plain TypeScript, which is EK4401 inside a test. The examples below use a local `check` helper that works the same way.

## Controlling virtual time

```vibe
import { TestClock } from "effect/testing"
import { assert, it } from "@effect/vitest"

function check(condition: boolean, message: string) {
  assert.isTrue(condition, message)
}

// Output:
it.effect("a deadline returns the fallback") {
  const pending = fork(Effect.timeoutOrElse(() => {
    sleep("10 minutes")
    return Option.some("finished")
  }, {
    duration: "30 seconds",
    orElse: () => Option.none<string>()
  }))

  TestClock.adjust("30 seconds")
  const result = join pending
  check(result.isNone(), "expected the timeout fallback")
}
```

`it.effect` provides the test services, virtual time included. `adjust` advances by a duration and runs sleepers whose deadlines fall within it; `setTime` sets an absolute timestamp in milliseconds. Fork the timed work first so the test's own fiber stays free to adjust the clock. The block owns `pending` and interrupts it if the test exits early.

```vibe
// run: Console.log(standaloneClockExample())
import { TestClock } from "effect/testing"

function standaloneClockExample() {
  provide TestClock.layer() {
    TestClock.setTime(5_000)
    TestClock.adjust("2 seconds")
    return Clock.currentTimeMillis() // 7000
  }
}
```

```output
7000
```

Outside the test harness, provide `TestClock.layer()` yourself. Real time passing does not advance the virtual clock. Sleeping on the test's own fiber before adjusting time leaves the test waiting for an adjustment it cannot make.

## Advancing recurring computations

```vibe
import { TestClock } from "effect/testing"
import { it } from "@effect/vitest"

// Output:
it.effect("emits once per interval") {
  const queue = Queue.unbounded<string>()
  fork {
    while (true) {
      sleep("15 minutes")
      queue.offer("refresh")
    }
  }

  TestClock.adjust("14 minutes")
  check(queue.poll().isNone(), "refresh arrived early")

  TestClock.adjust("1 minute")
  check(queue.take() === "refresh", "missing first refresh")
  check(queue.poll().isNone(), "duplicate refresh")

  TestClock.adjust("15 minutes")
  check(queue.take() === "refresh", "missing second refresh")
  check(queue.poll().isNone(), "duplicate second refresh")
}
```

Use the `check` helper from the first example. Advancing to each boundary lets the loop schedule its next sleep before the next adjustment. The queue makes each recurrence observable without real-time delays, and the background loop stops when the test block exits.

## Checking clock-dependent behavior

```vibe
import { TestClock } from "effect/testing"
import { it } from "@effect/vitest"

function isExpired(createdAt: number, lifetimeMillis: number) {
  return Clock.currentTimeMillis() - createdAt >= lifetimeMillis
}

// Output:
it.effect("expires at the boundary") {
  const createdAt = Clock.currentTimeMillis()
  TestClock.adjust("9 seconds")
  check(!isExpired(createdAt, 10_000), "expired too soon")
  TestClock.adjust("1 second")
  check(isExpired(createdAt, 10_000), "should expire at ten seconds")
}
```

Application code reads time with `Clock`: `Clock.currentTimeMillis()` reads the supplied clock, so it stays deterministic under the test layer. `Date.now()`, zero-argument `new Date()`, and platform timers are not application time sources in VibeLang. Testing just before and at a boundary catches comparison mistakes that a later timestamp alone misses.

## Timed waits and deferred results

```vibe
import { TestClock } from "effect/testing"
import { it } from "@effect/vitest"

// Output:
it.effect("completes a deferred after a delay") {
  const ready = Deferred.make<number>()
  const worker = fork {
    sleep("6 seconds")
    ready.succeed(42)
  }

  TestClock.adjust("6 seconds")
  const value = ready.await()
  join worker
  check(value === 42, "unexpected deferred value")
}
```

The worker sleeps before completing, so the test checks delayed completion. After advancing time, wait for the Deferred with `ready.await()` or join the worker to synchronize with the observable outcome. A fork alone says work started, not that its result is ready.
