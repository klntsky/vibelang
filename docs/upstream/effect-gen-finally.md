# Draft issue for Effect-TS/effect (not filed)

**Title:** `Effect.gen` / `Effect.fn` never run JavaScript `finally`, `catch`, or `using` cleanup on failure or interruption (4.0.5); the language service calls `try/finally` valid cleanup

**Version:** effect 4.0.5.

**What happens.** The generator frame created by `Effect.gen`, `Effect.fn`, `Effect.fnUntraced`, and `Effect.fnUntracedEager` (`fromIteratorUnsafe`, `internal/effect.ts:1413-1445`) defines only a success continuation. When a yielded effect fails, or the fiber is interrupted while suspended inside the generator, the frame is popped and the generator object is dropped. Nothing calls `iterator.throw()` or `iterator.return()`. JavaScript `catch`, `finally`, and `using` disposal inside the body therefore never run on those paths.

```ts
import { Effect, Fiber } from "effect"

const log: Array<string> = []
await Effect.runPromiseExit(Effect.gen(function*() {
  try { yield* Effect.fail("boom") } finally { log.push("finally") }
}))
console.log(log)   // []

await Effect.runPromise(Effect.gen(function*() {
  const fiber = yield* Effect.forkChild(Effect.gen(function*() {
    try { yield* Effect.never } finally { log.push("finally on interrupt") }
  }))
  yield* Effect.sleep("1 millis")
  yield* Fiber.interrupt(fiber)
}))
console.log(log)   // []
```

**Why it matters.** The Effect language service warns on `try/catch` inside `Effect.gen` (language-service#271) but exempts `try/finally` as a valid cleanup pattern (language-service#442). At runtime that cleanup does not run on failure or interruption, so users following the exemption leak resources.

**Options.** (a) Call `iterator.return()` when unwinding past a generator frame, and drive any effects yielded from `finally` uninterruptibly. (b) Document that generator `finally` is not a finalizer, and change the language-service rule to warn on `try/finally` and `using` inside generators, pointing to `Effect.ensuring` / `acquireRelease`.

**Evidence:** `experiments/tests/effect-facts.test.ts` F1-F3.
