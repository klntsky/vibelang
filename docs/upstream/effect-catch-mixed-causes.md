# Preserve cleanup defects during typed failure recovery

**Status:** drafted, not filed. **Target:** Effect runtime.
**Version:** `effect@4.0.5`. **Kind:** recovery-policy proposal.

## Behavior and reproduction

```ts
import { Effect, Exit } from "effect"

const body = Effect.ensuring(
  Effect.fail("expected"),
  Effect.die("release-defect")
)
const before = Effect.runSyncExit(body)
const after = Effect.runSyncExit(
  Effect.catch(body, () => Effect.succeed("recovered"))
)
console.log(Exit.isFailure(before) && before.cause.reasons.map(r => r._tag))
// ["Fail", "Die"]
console.log(after) // Success("recovered"); the cleanup defect is gone
```

Existing reproduction:
[R1](../../experiments/scratch-review-codex/probes.test.ts), run with
`cd experiments && bun test scratch-review-codex/probes.test.ts -t 'R1 '`.
[K14](../../experiments/tests/review-fixes.test.ts) checks the replacement
policy. `internal/effect.ts`'s `findError` selects the first Fail reason, and
`Effect.catch` replaces the entire cause with the handler's result.

## Expected behavior

Provide a typed recovery operation that recovers only causes made entirely of
expected failures. A cause containing a Die or Interrupt should pass through
with every reason intact. For several expected failures, explicitly document
whether the handler receives the first failure or all of them.

This is a request for an explicit policy, not a claim that the present
`Effect.catch` implementation contradicts its public contract.

## Why it matters to Effeck

Source `catch` handles expected failures only. If it recovers a body failure and
silently drops a release defect, the program appears successful after broken
cleanup. The kernel's `catchExpected` re-fails mixed causes unchanged (D13,
D46), so generated code cannot lower this construct to ordinary `Effect.catch`.

## Suggested fix

Add a documented recovery combinator or option implemented with
`Effect.catchCause`: call the handler only when every reason is a Fail;
otherwise return `Effect.failCause(cause)`. Retain existing `Effect.catch`
behavior if compatibility requires it, and document that it selects a failure
from a mixed cause. Test Fail+Die, Fail+Interrupt, Die alone, Interrupt alone,
and several Fail reasons. Kernel abandonment defects R2b and R5 were separate
local bugs and are already fixed; they are not part of this request.
