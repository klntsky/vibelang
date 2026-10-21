# Scope

A scope owns registered finalizers and runs them when its lifetime ends. Use `using` for lexical ownership, or an explicit scope when several operations must share a lifetime independent of one function call.

[Upstream page](https://effect.website/docs/v4/resource-management/scope)

## Registering finalizers

```vibe
// run: Console.log(runRegisteredWork())
function registeredWork() {
  Effect.addFinalizer((exit) => {
    Console.log(`Work scope closed with ${exit._tag}`)
  })
  return "Prepared"
}
// string!!Scope

function runRegisteredWork() {
  return Effect.scoped(() => registeredWork())
}
// string!!
// Output: Work scope closed with Success
// Output: Prepared
```

`Effect.addFinalizer` attaches cleanup to the current scope, not the function's return. `Scope` is in the prelude: it names the module, and in a contract it means `Scope.Scope`, so no import is needed. `Effect.scoped` creates a scope, runs the program, and closes the scope with its outcome: the finalizer receives a success exit on success, and a failure exit on typed failure, a defect, or interruption.

Finalizers run in reverse registration order by default, with interruption deferred during release. Their typed error channel must be `never`, so handle expected cleanup failures inside the callback. A defect during release stays in the resulting cause.

## Opening and closing scopes explicitly

```vibe
// run: Console.log(manuallyManagedWork())
function manuallyManagedWork() {
  return Effect.acquireUseRelease(
    () => Scope.make(),
    (scope) => {
      Scope.addFinalizer(scope, () => Console.log("First finalizer"))
      Scope.addFinalizer(scope, () => Console.log("Second finalizer"))
      return Scope.provide(() => registeredWork(), scope)
    },
    (scope, exit) => Scope.close(scope, exit)
  )
}
// Output: Work scope closed with Success
// Output: Second finalizer
// Output: First finalizer
// Output: Prepared
```

`Scope.make` creates a closeable scope; `Scope.close` supplies the exit its finalizers observe. The bracket closes the scope even if work fails or is cancelled, passing the actual outcome to release. The finalizer from `registeredWork` runs first, then the explicit ones in reverse.

`Scope.provide(program, scope)` supplies a scope without closing it when the program returns. Closing a scope doesn't cancel computations that merely use it; use scoped fiber ownership for cancellation. A finalizer registered in a closed scope runs immediately, so a closed scope can't extend a resource's lifetime.

## Acquiring resources with automatic release

```vibe
// run: Console.log(copyLabels())
function openBuffer(label: string) {
  return Effect.acquireRelease(
    () => {
      Console.log(`Opened ${label}`)
      return { label }
    },
    (buffer) => Console.log(`Closed ${buffer.label}`)
  )
}
// { label: string }!!Scope

function copyLabels() {
  using source = openBuffer("source")
  using destination = openBuffer("destination")
  return `${source.label} -> ${destination.label}`
}
// string!!
// Output: Opened source
// Output: Opened destination
// Output: Closed destination
// Output: Closed source
// Output: source -> destination
```

`acquireRelease` makes acquisition uninterruptible by default and registers release before exposing the resource. Its third argument `{ interruptible: true }` permits cancellation during acquisition, and `using` keeps that choice. A successful acquisition gets exactly one release when its scope closes; a failed one registers none.

`using` gives each declared acquisition a lifetime ending with the enclosing block. Don't return a resource to use after the block closes; return derived data, or let the caller own the acquisition. `using` works the same with Effect's existing scoped APIs.

## Arranging nested resource lifetimes

```vibe
// run: nestedLifetimes(); observeOwnership()
function nestedLifetimes() {
  using outer = openBuffer("session")
  {
    using inner = openBuffer("request")
    Console.log(`${outer.label}/${inner.label}`)
  }
  Console.log("Request buffer closed; session still open")
}

function mixedOwnership() {
  const callerOwned = openBuffer("caller")
  using local = openBuffer("local")
  Console.log(`${callerOwned.label}/${local.label}`)
}
// void!!Scope

function observeOwnership() {
  Effect.scoped {
    mixedOwnership()
    Console.log("Local closed; caller buffer still open")
  }
}
// Output: Opened session
// Output: Opened request
// Output: session/request
// Output: Closed request
// Output: Request buffer closed; session still open
// Output: Closed session
// Output: Opened caller
// Output: Opened local
// Output: caller/local
// Output: Closed local
// Output: Local closed; caller buffer still open
// Output: Closed caller
```

The inner `using` resource closes before the outer block continues. In `mixedOwnership`, the acquisition without `using` belongs to the caller's scope though the function also owns a local resource, so the caller's buffer closes only when `Effect.scoped` exits.

Operations that only register finalizers in one scope share its lifetime. Use separate `Effect.scoped` calls when one operation's resources must close before the next begins, and one outer scope when later operations depend on earlier resources.

A block that calls `fork` also owns a scope region, even without `using`: its child is interrupted when the block exits, and the block waits for cleanup. `Effect.forkScoped` uses the caller's scope instead and adds `Scope` to `R`; `Effect.forkDetach` gives a detached lifetime.

## Sequencing scoped operations with rollback

```vibe
// run: provide ProjectSteps = { create: (stage) => { Console.log(`create ${stage}`); return stage }, remove: (id) => Console.log(`remove ${id}`), finish: () => "ready" } { Console.log(setupProject()) }; provide ProjectSteps = { create: (stage) => { Console.log(`create ${stage}`); if (stage === "index") throw new SetupFailed({ stage }); return stage }, remove: (id) => Console.log(`remove ${id}`), finish: () => "ready" } { try { setupProject() } catch (error) { Console.log(error.stage) } }
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
      if (exit.isFailure()) steps.remove(id)
    }
  )
}
// string!SetupFailed!ProjectSteps | Scope

function setupProject() {
  return Effect.scoped {
    const storage = reserveStep("storage")
    const index = reserveStep("index")
    return ProjectSteps.finish(storage, index)
  }
}
// string!SetupFailed!ProjectSteps
// Output: create storage
// Output: create index
// Output: ready
// Output: create storage
// Output: create index
// Output: remove storage
// Output: index
```

Neither reservation uses lexical `using`, so both register in the same scope. A successful exit keeps the created artifacts. If index creation fails, storage is removed; if `finish` fails, index is removed before storage.

The finalizers see the whole setup's exit, so defects and interruption trigger the same rollback. Nested `using` blocks would end those lifetimes earlier and withhold the overall outcome from the finalizers. This compensates for successful steps; it is not an atomic transaction with an external system.

```vibe
// run: const result = exerciseRollback(); Console.log(JSON.stringify({ outcome: result.outcome._tag, removed: result.removed }))
function exerciseRollback() {
  const removed: string[] = []
  const testLayer = Layer.succeed(ProjectSteps, {
    create(stage) { return stage },
    remove(id) { removed.push(id) },
    finish(_storage, _index) { throw new SetupFailed({ stage: "finish" }) }
  })
  const outcome = Effect.result(Effect.provide(() => setupProject(), testLayer))
  return { outcome, removed } // removed: ["index", "storage"]
}
// Output: {"outcome":"Failure","removed":["index","storage"]}
```

Test failure at each stage to confirm which steps were acquired and which compensations ran. If compensation can fail, handle its expected failure in the release callback and keep unrecoverable cleanup failures as defects; never discard them to make rollback look successful.

## Coming from Effect

Use `using` for a resource that belongs to its block, and `Effect.scoped`, `Effect.addFinalizer`, and the `Scope` module for shared or manually controlled lifetimes; calls run them directly, without generator syntax.
