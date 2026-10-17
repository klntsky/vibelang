# Introduction

Resources need cleanup on success, failure, and cancellation. Effeck uses `finally` for unconditional cleanup, `using` for lexical resource ownership, and Effect's outcome-aware combinators when release depends on how work ended.

[Upstream page](https://effect.website/docs/v4/resource-management/introduction)

## Guaranteeing cleanup

```ts
error ExportRejected

function exportRows(accepted: boolean) {
  try {
    log("Export started")
    if (!accepted) throw new ExportRejected()
    return "Export complete"
  } finally {
    log("Export finished")
  }
}
// string!ExportRejected!
```

`finally` runs after success, a typed failure, a defect, or interruption. It also runs after early return and when a handler abandons the protected computation. Effeck gives it Effect cleanup semantics, including deferred interruption while cleanup runs.

`Effect.ensuring(() => work(), () => cleanup())` remains available for a reusable combinator. A `finally` block is usually clearer for local cleanup. If cleanup itself fails, its failure is retained with the original outcome rather than silently replacing and dropping an earlier failure.

`ensuring` requires cleanup whose typed error channel is `never`; source `finally` permits typed cleanup failures and uses `Effect.onExit` semantics. Cleanup failure is retained even when a handler abandons the computation while interruption is pending.

## Finalizing based on the outcome

```ts
function observedExport(accepted: boolean) {
  return Effect.onExit(() => exportRows(accepted), (exit) => {
    if (Exit.isSuccess(exit)) {
      log("Export succeeded")
    } else {
      log("Export stopped", exit.cause.reasons.map((reason) => reason._tag))
    }
  })
}
```

`onExit` receives an `Exit` containing the success value or complete failure cause. Unlike a boolean success flag, this preserves defects and cancellation as well as domain errors. Its cleanup runs uninterruptibly, and successful cleanup preserves the original result.

## Cleaning up failed work

```ts
function reportFailedExport(accepted: boolean) {
  return Effect.onError(() => exportRows(accepted), (cause) => {
    if (Cause.hasInterrupts(cause)) {
      log("Export cancelled; cleanup complete")
    } else {
      log("Export failed; cleanup complete")
    }
  })
}
```

`onError` runs only on an unsuccessful outcome, including a defect or interruption. The callback receives a full cause and runs as cleanup; it does not recover the original computation. This differs from ordinary catch, which handles expected failures and skips causes containing defects or interruption.

## Bracketing acquisition, use, and release

```ts
interface Lease {
  readonly id: string
}

service LeaseStore {
  acquire(): Lease!!
  release(lease: Lease): void!!
}

function withLease() {
  const store = LeaseStore()
  return Effect.acquireUseRelease(
    () => store.acquire(),
    (lease) => {
      log(`Working with lease ${lease.id}`)
      return lease.id
    },
    (lease, exit) => {
      log(`Releasing after ${exit._tag}`)
      store.release(lease)
    }
  )
}
// string!!LeaseStore
```

`acquireUseRelease` protects acquisition and release while allowing normal interruption during use. Release runs exactly once after successful acquisition, even when use fails or is cancelled; it is not called if acquisition fails. A failing acquisition must clean up any partial work it created before failing.

The release callback receives both the resource and the use phase's `Exit`. The combined contract includes failures and requirements from acquisition, use, and release. Default protected acquisition can delay cancellation, so acquisition should finish in bounded time.

## Giving the caller resource ownership

```ts
function acquireLease() {
  const store = LeaseStore()
  return Effect.acquireRelease(
    () => store.acquire(),
    (lease) => store.release(lease)
  )
}
// Lease!!LeaseStore | Scope

function leaseId() {
  using lease = acquireLease()
  return lease.id
}
// string!!LeaseStore

layer LeaseStoreTest: LeaseStore {
  acquire() { return ({ id: "test-lease" }) }
  release(_lease) {}
}

function example() {
  with LeaseStoreTest { return leaseId() }
}
```

`acquireRelease` registers a release in a scope and returns the acquired value. `using` supplies a lexical scope for that acquisition and releases it at block exit, removing `Scope` from the caller's requirements. Its release callback must have no typed failure; handle an expected release error explicitly or deliberately promote it with `Effect.orDie`.

Only `using` acquisitions belong to the lexical block. An acquisition without `using` keeps `Scope` in `R` and belongs to its caller, including when the same function also uses a local resource.

`Scope` is in the prelude as both the module and the type alias for `Scope.Scope`, so these contracts need no import. `using` adds no masking policy: `acquireRelease` masks acquisition by default, while its third argument `{ interruptible: true }` lets cancellation reach an acquisition that waits on the outside world. An adapter that receives a resource after cancellation must release it; the spec's resource adapter uses the runtime's TypeScript `suspend` helper to detect that race.

## Coming from Effect

`try`/`finally` replaces local `ensuring` chains, and `using resource = acquire()` replaces a scoped acquisition for a lexical block. Effect's `onExit`, `onError`, and bracket combinators still take programs and Effeck callbacks.
