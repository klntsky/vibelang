# Explain repeated Interrupt reasons introduced by tracing

**Status:** drafted, not filed. **Target:** Effect runtime and diagnostics docs.
**Version:** `effect@4.0.5`. **Kind:** diagnostic representation proposal.

## Behavior and reproduction

```ts
import { Cause, Effect, Exit, Fiber } from "effect"

const exit = await Effect.runPromise(Effect.gen(function* () {
  const child = yield* Effect.forkChild(
    Effect.fn("draft/wait")(function* () { yield* Effect.never })()
  )
  yield* Effect.sleep("1 millis")
  yield* Fiber.interrupt(child)
  return yield* Fiber.await(child)
}))
if (Exit.isFailure(exit)) {
  console.log(exit.cause.reasons.filter(r => r._tag === "Interrupt").length)
  console.log(Cause.interruptors(exit.cause).size)
}
// Tracing can produce several reasons for the same single interruptor.
```

Run the [existing comparison](../../experiments/scratch-agent-a/scripts/q3-fn-interrupt-reasons.ts):
`cd experiments && bun scratch-agent-a/scripts/q3-fn-interrupt-reasons.ts`.
The research observed one Interrupt reason for `Effect.gen`, three for one
`Effect.fn`, six for nested `Effect.fn` calls, and two for `withSpan`, all with
the same interruptor id. Cause annotations differ by reference as stack
information is attached during unwinding.

## Expected behavior

Diagnostics should distinguish interruptor identities from annotated stack
observations. Adding tracing should not make a
single interruption appear to be several unrelated cancellation events.
Retain useful stack annotations even when grouping reasons by interruptor.

## Why it matters to EffectScript

Tracing is opt-in, and enabling it changes the number of reasons developers
see. Abandonment must retain every real pending interruption and cleanup
failure while removing only its own sentinel. Miscounting reasons makes these
guarantees harder to verify and can inflate cancellation metrics.

## Suggested fix

Document `Cause.interruptors` as the way to count distinct interruptor ids.
Consider grouping repeated annotated Interrupt reasons in pretty printing, or
merging their annotations while preserving all sites. Add traced and untraced
regression cases. Do not simply discard annotations or deduplicate all reasons
by fiber id, since distinct sites carry useful diagnostic information.
