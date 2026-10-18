# Semaphore

A semaphore limits access to a shared resource with a pool of permits. A one-permit semaphore provides mutual exclusion; a larger pool allows several operations to enter at once. Callers wait without blocking the JavaScript thread when too few permits remain.

## Creating a permit pool

```effeck
function makeUploadLimit() {
  return Semaphore.make(3)
}
// Semaphore.Semaphore!!
```

Create the pool once for the callers that must share a limit. Allocating a separate pool inside every upload would give each upload its own permits and impose no combined limit. A permit represents access, not a resource value such as a connection.

## Limiting concurrent access

```effeck
function uploadAll() {
  const uploads = Semaphore.make(2)
  return forEach(["a.csv", "b.csv", "c.csv", "d.csv"], (file) => {
    return Semaphore.withPermits(uploads, 1) {
      logInfo("upload started", file)
      sleep("30 millis")
      return file
    }
  }, { concurrency: "unbounded" })
}
```

The traversal can start all four callers, but only two can enter the permit-protected region at once. This is useful when several unrelated parts of the application must share a resource limit. A traversal's own `concurrency` option instead limits only that traversal.

## Acquiring several permits around work

```effeck
function weightedJobs() {
  const memoryBudget = Semaphore.make(4)
  return forEach([1, 2, 3], (weight) => {
    return Semaphore.withPermits(memoryBudget, weight) {
      sleep("20 millis")
      return `job using ${weight} units`
    }
  }, { concurrency: 3 })
}
```

An operation waits until its requested permits can be acquired together. This models jobs with different resource costs, such as memory reservations. Requesting more permits than the pool can supply leaves the operation waiting, so ensure job weights fit the configured pool.

## Releasing permits safely

```effeck
error Rejected

function releaseOnFailure() {
  const mutex = Semaphore.make(1)
  try {
    Semaphore.withPermits(mutex, 1) { throw new Rejected() }
  } catch (error) {
    logWarning("first job rejected", error._tag)
  }
  return pipe(() => "next job can enter", Semaphore.withPermits(mutex, 1))
}
```

`withPermits` returns acquired permits on success, failure, and interruption. Prefer it to manually pairing `Semaphore.take` and `release`, where cancellation between acquisition and cleanup registration can leak permits. `Semaphore.withPermit` is the single-permit convenience form with the same release guarantee.
