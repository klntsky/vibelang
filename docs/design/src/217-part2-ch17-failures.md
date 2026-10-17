### 17. Failures, defects, interruption, and cleanup

```ts es
function exportReport(id: string) {
  using tmp = TempFiles.create()
  try {
    writeRows(tmp, Reports.rows(id))
    return Storage.upload(tmp)
  } catch (e) {                    // e: UploadError
    return Storage.queueRetry(id)
  } finally {
    Metrics.increment("exports")
  }
}

function exportWithDeadline(id: string) {
  return timeout(exportReport(id), "30 seconds")
}
```

If the 30 seconds run out during the upload, the upload is interrupted, the `catch` block does not run, the `finally` block runs, the temporary file is removed, and the `Effect.timeout` call fails with `Cause.TimeoutError`. Each kind of exit has one rule:

| Exit | Source | In E | Runs `catch` | Runs `finally` and `using` |
| --- | --- | --- | --- | --- |
| Expected failure | `throw` in Effeck; a callee's E; an exception a trusted contract lists | Yes | Yes, narrowed by type | Yes |
| Defect | Any other JavaScript exception, such as `(undefined as any).x`; `Effect.die` | No | No; observe it with `Effect.catchDefect` at a boundary | Yes |
| Interruption | Another fiber, a timeout, or an enclosing scope closing | No | No | Yes |
| Abandonment | A handler that does not continue its program (Chapter 9) | No | No | Yes |
| Mixed: expected failures plus a defect or an interruption | A `finally` that crashed while the block was failing | No: E describes causes made only of expected failures, so this cause can carry failures outside E (Chapter 14) | No: the whole exit propagates, every reason intact | Yes |
| Several expected failures | `Effect.race` with both branches failed | Yes | Yes; `e` is the first, and the `catch` handles the exit as a whole | Yes |

Rules:

- **E comes from `throw` and from contracts.** A `throw` written in Effeck, a callee's E, and the exceptions a trusted contract lists are expected failures. Any other JavaScript exception is a defect, as in Effect today.
- **`catch` handles a cause only when it is made entirely of expected failures.** A cause that also holds a defect or an interruption passes through `catch` untouched, including its expected failures. Effect's own `Effect.catch` recovers such a cause and drops the defect (Codex review R1, `internal/effect.ts:158-165, 2665-2685`); source `catch` therefore compiles to the runtime's `catchExpected` (K14a-K14c, Chapter 26).
- **Cleanup runs in reverse order of acquisition**, once per exit, with interruption held off until it finishes.
- **A failure during cleanup is never dropped.** It is added to the cause being reported, on every path: a block's exit, a clause that abandons, an explicit `abandon` while an interruption is pending, a scope that closes, and a fiber that exits (K12a-K12e, K13d).
- **Control cannot leave a `finally` block.** A `return`, `break`, or `continue` there would replace the block's exit, and in JavaScript it discards a failure in flight, so it is EK2006.
- **Abandonment unwinds like interruption.** Cleanup sees an interrupted exit, `catch` does not run, and the handler that abandoned the program continues only after cleanup finishes. A cleanup failure during abandonment fails the `handle` or the `abandon` call that abandoned.
- **Cleanup that uses a control operation handles it inside the cleanup** (Chapter 9). A handler outside could stop the cleanup partway.

A cause with expected failures and a defect keeps both after it passes `catch`, so a cause-level tool such as `Effect.catchCause` or a TypeScript caller's `Effect.catch` can still see the expected failure. No Effeck `catch` will.

#### Effect's operators on a mixed cause

Effect's own operators keep their Effect behavior when Effeck calls them. On a cause that holds both an expected failure and a defect, most of them act on one reason and drop the other, unlike `catch`:

| Operators | Given a `Fail` plus a `Die` |
| --- | --- |
| `Effect.catch`, `catchTag`, `result`, `option`, `match`, `ignore`, `orElseSucceed`, `firstSuccessOf`, `flip`, `partition` | Act on the expected failure and drop the defect |
| `Effect.validate`, `mapError` | Fail with the expected failure, mapped for `mapError`, and drop the defect |
| `Effect.catchDefect` | Acts on the defect and drops the expected failure |
| `Effect.retry`, `Effect.exit`, `Effect.catchCause` | Keep every reason |

These were checked against `effect@4.0.5` (review probe R1 for `Effect.catch`). Use such an operator where dropping the other reason is what the program means; an Effeck `catch` never drops one.

Interruption can be held off for a block of work with Effect's own combinators:

```ts es
function moveFunds(from: Account, to: Account, amount: number) {
  Effect.uninterruptible(() => {
    Ledger.debit(from, amount)
    Ledger.credit(to, amount)     // an interruption waits until both have run
  })
}
```

`try`, `catch`, `finally`, and `using` compile to Effect's own failure and cleanup frames, never to JavaScript `try`, in any function that compiles to frames. Effect does not run JavaScript `finally` blocks inside generators when a fiber fails or is interrupted (F1-F3), so a JavaScript `try` would skip cleanup. Chapter 26 shows the generated code. Functions that compile to plain JavaScript are the exception, because nothing in them can pause (Chapter 15).
