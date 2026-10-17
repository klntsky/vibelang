# Scope

A scope owns registered finalizers and runs them when its lifetime ends. Use `using` for lexical ownership, or explicit scopes when several operations must share a lifetime independent of one function call.

[Upstream page](https://effect.website/docs/v4/resource-management/scope)

## Registering finalizers

```ts
function registeredWork() {
  Effect.addFinalizer((exit) => {
    Effect.log(`Work scope closed with ${exit._tag}`)
  })
  return "Prepared"
}
// string!!Scope

function runRegisteredWork() {
  return Effect.scoped(() => registeredWork())
}
// string!!
```

`Effect.addFinalizer` attaches cleanup to the current scope, not to the function's return. `Scope` is in the prelude: it names the module, and in a contract it means `Scope.Scope`, so no import is needed. `Effect.scoped` creates a scope, runs the program, and closes that scope with its outcome. On success the finalizer receives a success exit; typed failure, defects, and interruption produce failure exits.

Finalizers run in reverse registration order by default, with interruption deferred during release. Their typed error channel must be `never`; deal with expected cleanup failures inside the callback. A defect during release remains part of the resulting cause.

## Opening and closing scopes explicitly

```ts
function manuallyManagedWork() {
  return Effect.acquireUseRelease(
    () => Scope.make(),
    (scope) => {
      Scope.addFinalizer(scope, () => Effect.log("First finalizer"))
      Scope.addFinalizer(scope, () => Effect.log("Second finalizer"))
      return Scope.provide(() => registeredWork(), scope)
    },
    (scope, exit) => Scope.close(scope, exit)
  )
}
```

`Scope.make` creates a closeable scope; `Scope.close` supplies the exit that its finalizers observe. The bracket guarantees closure even if work fails or is cancelled, and passes that actual outcome to release. The finalizer from `registeredWork` runs first, then the second explicit finalizer, then the first.

`Scope.provide(program, scope)` supplies a scope without closing it when that program returns. Closing a scope does not itself cancel arbitrary computations merely using it; use scoped fiber ownership when cancellation is required. Registering a finalizer in an already closed scope runs it immediately, so a closed scope cannot provide a useful extended resource lifetime.

## Acquiring resources with automatic release

```ts
function openBuffer(label: string) {
  return Effect.acquireRelease(
    () => {
      Effect.log(`Opened ${label}`)
      return { label }
    },
    (buffer) => Effect.log(`Closed ${buffer.label}`)
  )
}
// { label: string }!!Scope

function copyLabels() {
  using source = openBuffer("source")
  using destination = openBuffer("destination")
  return `${source.label} -> ${destination.label}`
}
// string!!
```

`acquireRelease` makes acquisition uninterruptible by default and registers release before exposing the resource. Its third argument `{ interruptible: true }` permits cancellation during acquisition; `using` keeps that choice. Successful acquisition gets exactly one release when its scope closes; failed acquisition does not register one. Release order here is destination, then source.

`using` gives each declared acquisition a lifetime ending with the enclosing block. Do not return a resource expecting to use it after that block closes; return derived data or let the caller own the acquisition instead. Effect's existing scoped APIs can be acquired with `using` in the same way.

## Arranging nested resource lifetimes

```ts
function nestedLifetimes() {
  using outer = openBuffer("session")
  {
    using inner = openBuffer("request")
    Effect.log(`${outer.label}/${inner.label}`)
  }
  Effect.log("Request buffer closed; session still open")
}

function mixedOwnership() {
  const callerOwned = openBuffer("caller")
  using local = openBuffer("local")
  Effect.log(`${callerOwned.label}/${local.label}`)
}
// void!!Scope

function observeOwnership() {
  Effect.scoped(() => {
    mixedOwnership()
    Effect.log("Local closed; caller buffer still open")
  })
}
```

The inner `using` resource closes before the outer block continues. In `mixedOwnership`, the acquisition without `using` belongs to the caller's scope even though the function also owns a local resource. The caller's buffer closes only when `Effect.scoped` exits.

Operations that merely register finalizers in the same scope share its lifetime. Use separate `Effect.scoped` calls when one operation's resources must close before the next begins. Use one outer scope when later operations depend on resources acquired earlier.

A block that calls `fork` also owns a scope region, even without `using`. Its child is interrupted when the block exits, and the block waits for cleanup. `Effect.forkScoped` instead uses the caller's scope and adds `Scope` to `R`; `Effect.forkDetach` gives a detached lifetime.

## Sequencing scoped operations with rollback

```ts
error SetupFailed {
  stage: string
}

service ProjectSteps {
  create(stage: string): string!SetupFailed
  remove(id: string): void!!
  finish(storage: string, index: string): string!SetupFailed
}

function reserveStep(stage: string) {
  const steps = ProjectSteps()
  return Effect.acquireRelease(
    () => steps.create(stage),
    (id, exit) => {
      if (Exit.isFailure(exit)) steps.remove(id)
    }
  )
}
// string!SetupFailed!ProjectSteps | Scope

function setupProject() {
  return Effect.scoped(() => {
    const storage = reserveStep("storage")
    const index = reserveStep("index")
    return ProjectSteps.finish(storage, index)
  })
}
// string!SetupFailed!ProjectSteps
```

Both reservations register in the same scope because neither uses lexical `using`. A successful scope exit keeps the created artifacts. If index creation fails, storage is removed; if `finish` fails, index is removed before storage.

The finalizers see the whole setup operation's exit, so defects and interruption trigger the same rollback. Separate nested `using` blocks would end those lifetimes earlier and would not give every finalizer the overall setup outcome. This is compensation for successfully completed steps, not an atomic transaction with an external system.

```ts
function exerciseRollback() {
  const removed: string[] = []
  const testLayer = Layer.succeed(ProjectSteps, {
    create: (stage) => stage,
    remove: (id) => { removed.push(id) },
    finish: (_storage, _index) => { throw new SetupFailed({ stage: "finish" }) }
  })
  const outcome = Effect.result(Effect.provide(() => setupProject(), testLayer))
  return { outcome, removed } // removed: ["index", "storage"]
}
```

Test failure at each stage to confirm which steps were acquired and which compensations ran. If compensation can fail, handle its expected failure in the release callback and preserve unrecoverable cleanup failures as defects. Do not discard cleanup failures to make rollback appear successful.

## Coming from Effect

Use `using` for a resource that belongs to its block. Keep `Effect.scoped`, `Effect.addFinalizer`, and the `Scope` module for shared or manually controlled lifetimes; calls run them directly without generator syntax.
