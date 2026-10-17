# Control operations and handlers

A control operation lets a handler decide how the caller continues. Use one when a dependency must pause a program, answer a request, or end the program early.

## Declare and perform an operation

```ts
// approval.effeck
export error ReviewBusy { request: string }

export control Approval {
  approve(request: string): boolean!ReviewBusy
}

export function release(version: string) {
  if (!Approval.approve(`deploy ${version} to staging`)) return "staging declined"
  if (!Approval.approve(`deploy ${version} to production`)) return "production declined"
  return `released ${version}`
}
// string!ReviewBusy!Approval
```

`Approval.approve` returns a boolean at the call. `Approval` stays in the program's requirements until a handler answers it. Each abstract operation states all three channels of its contract.

`Control`, `handle`, `resume`, `resumeWith`, and `abandon` are globals in `.effeck` files.

## Run one program with two policies

```ts
// policies.effeck
import { Approval, release } from "./approval.effeck"

export function autoApprove(version: string) {
  return handle(() => release(version), Approval, {
    approve(_request, k) {
      return resume(k, true)
    }
  })
}

export function scriptedReview(version: string, decisions: ReadonlyArray<boolean>) {
  let next = 0
  return handle(() => release(version), Approval, {
    approve(_request, k) {
      const approved = decisions[next] ?? false
      next++
      return resume(k, approved)
    }
  })
}

export function comparePolicies() {
  return [autoApprove("v1"), scriptedReview("v1", [true, false])]
  // ["released v1", "production declined"]
}
```

The business program stays the same. `handle` takes a program, the control class, and a clause for every operation. A clause receives the operation's arguments followed by `k`, the continuation: the remaining work after the call.

`resume(k, true)` continues the program with `true` as the operation's result. The handler is deep: the next approval request reaches the same clauses. A final `return resume(k, value)` outside cleanup regions runs in constant stack space.

A continuation can be consumed once. Keep it inside its clause; never save it or return it. The runtime rejects a second resume, and the compiler does not yet check continuation escape.

## Stop and clean up

```ts
// stopping.effeck
import { Approval } from "./approval.effeck"

function deployment() {
  try {
    Approval.approve("deploy to production")
    return "deployed"
  } finally {
    Console.log("deployment hold released")
  }
}

export function stopDeployment() {
  return handle(() => deployment(), Approval, {
    approve(_request, k) {
      abandon(k)
      Console.log("cleanup finished")
      return "cancelled"
    }
  })
}
```

`abandon(k)` runs the suspended program's cleanup before the clause continues. It skips the program's `catch` blocks and runs its `finally` blocks and resource releases. Cleanup failures still fail the operation.

Returning from a clause without resuming also stops the program and cleans it up before `handle` returns. Use explicit `abandon` when the clause needs cleanup to finish before its next statement.

## Resume with an expected failure

```ts
// busy.effeck
import { Approval, ReviewBusy } from "./approval.effeck"

function requestApproval() {
  try {
    return Approval.approve("deploy now") ? "approved" : "declined"
  } catch (error) {
    return `review unavailable: ${error.request}`
  }
}

export function busyReviewer() {
  return handle(() => requestApproval(), Approval, {
    approve(request, k) {
      return resumeWith(k, () => { throw new ReviewBusy({ request }) })
    }
  })
}
```

`resumeWith` runs its program at the original operation call. The caller can catch `ReviewBusy` there. This program can raise only errors declared by the operation, and it must need no services. Read services in the clause first, then capture their values in the program passed to `resumeWith`.

`resume` returns the resumed computation's result. The optional fourth argument of `handle` transforms normal returns. In this preview, `resume` is typed with the program's original result, or `never` when that fourth argument is supplied; avoid inspecting the resumed result when clauses return different types.

## Keep handlers on the executing fiber

```ts
// concurrent.effeck
import { Approval } from "./approval.effeck"

export function approveAll(requests: ReadonlyArray<string>) {
  return Effect.forEach(requests, (request) =>
    handle(() => Approval.approve(request), Approval, {
      approve(_request, k) { return resume(k, true) }
    }),
    { concurrency: 4 })
}
```

Install the handler inside each concurrent branch. A handler on the parent fiber cannot answer a child fiber's control operations.

| Diagnostic | Meaning | Fix |
| --- | --- | --- |
| EK2005 | A `resumeWith` program needs a service or raises an undeclared error | Read the service in the clause; raise only the operation's declared errors |
| EK4201 | Work sent to another fiber still requires a control operation | Handle it inside the branch, or use sequential execution where the combinator supports it |
| EK4202 | A TypeScript function's declared requirements include a control operation | Write it in Effeck or handle the operation inside the TypeScript implementation |

Stream operators run a producer on its own fiber too. Handle any control requirements inside the producer before passing it to a Stream operator. See [Producers](112-producers.md).
