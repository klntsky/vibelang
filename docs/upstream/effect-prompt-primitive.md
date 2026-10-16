# Support one-shot delimited continuations on an Effect fiber

**Status:** drafted, not filed. **Target:** Effect runtime.
**Version examined:** `effect@4.0.5`. **Kind:** API proposal (G1).

## Current behavior and minimal evidence

`Effect.callback` captures a wait at the fiber root. Public APIs do not capture
a segment of the current continuation between a perform site and an interior
delimiter. Starting a child fiber changes Context, finalization, and fiber
identity, so it does not implement same-fiber delimited control.

```ts
import { Effect } from "effect"

Effect.runSync(Effect.withFiber(fiber => Effect.sync(() => {
  const internal = fiber as unknown as {
    _stack: unknown[]
    interruptible: boolean
    _interruptedCause: unknown
  }
  console.log(Array.isArray(internal._stack), typeof internal.interruptible)
  // true, "boolean": accessible structurally, not a supported capture API
})))
```

The [minimal shift/reset prototype](../../experiments/scratch-agent-a/scripts/q9-shift-reset-proto.ts)
is an executable example of the missing operation. Run
`cd experiments && bun scratch-agent-a/scripts/q9-shift-reset-proto.ts`.
Its basic reset example produces 21: the body multiplies the resumed value 10
by 2, then the clause adds 1. It also demonstrates abort, one-shot rejection,
and Context restoration. That early prototype is not the final ownership
model; the [kernel](../../packages/runtime/src/kernel/kernel.ts) and its law suite
supply abandonment, interruption, and transferred ownership.

## Expected capability

A supported primitive should delimit, capture, resume, and abandon a segment
on the same fiber through ordinary TypeScript-authored Effect frames. Resuming
is one-shot, remains on the capturing fiber, restores captured Context and
interruptibility, and returns to the clause through a reinstalled delimiter.
Abandonment must run finalizers and preserve real pending interruptions and
cleanup failures. A capturing fiber must release its unconsumed transfers
before publishing its exit.

## Why it matters to EffectScript

Control handlers and producers use this operation without another scheduler
or DI container. The kernel currently reads `_stack`, `interruptible`,
`_interruptedCause`, `_deferredInterrupt`, and private frame protocol keys.
Every Effect upgrade requires revalidating these undocumented seams.

## Suggested fix

Offer `Effect.prompt`-style operations, or an opaque same-fiber stack-segment
API with access to the required Context and interruption state. Opaque segments
are preferable to exposing mutable arrays. Define ownership, rejection of
double consumption, finalizer unwinding, and fiber affinity as part of the API.
Do not infer these guarantees from a raw stack splice alone.

Use T1-T17, K11-K14, and the conformance tests as a concrete discussion suite;
run `cd experiments && bun test`. K11 requires tail resumption after clause
frames have completed, and K12-K13 cover interruption and ownership edges.
For root suspension, consider a resumer that reports a lost race, as T5/K10
do, so adapters retain ownership of undelivered resources. This is a capability
proposal, not an assertion that Effect's current public APIs are broken.
