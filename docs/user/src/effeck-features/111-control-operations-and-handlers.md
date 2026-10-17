# Control operations and handlers

A control operation lets a handler decide how the caller continues. Use one when a dependency must pause a program, answer a request, or end the program early.

## Declare and perform an operation

```ts
// approval.effeck
export error ReviewBusy { request: string }

export control class Approval {
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
  try {
    return release(version)
  } handle Approval.approve(_request) {
    resume(true)
  }
}

export function scriptedReview(version: string, decisions: ReadonlyArray<boolean>) {
  let next = 0
  try {
    return release(version)
  } handle Approval.approve(_request) {
    const approved = decisions[next] ?? false
    next++
    resume(approved)
  }
}

export function comparePolicies() {
  return [autoApprove("v1"), scriptedReview("v1", [true, false])]
  // ["released v1", "production declined"]
}
```

The business program stays the same. A `handle` clause after a try block names an operation, `Approval.approve`, and parameters that take the operation's parameter types. Each time the try block performs the operation, the clause runs.

`resume(true)` continues the try block with `true` as the operation's result. The handler is deep: the next approval request reaches the same clause. A clause that ends with `resume` runs in constant stack space, however many times the block performs the operation.

A `return` in the try block or in a clause returns from the enclosing function, and `break` and `continue` leave its loops. A statement answers every operation of each control it names: give each operation a clause (EK2108). One statement can handle several controls.

A clause can work after `resume`. The try block runs first, until it finishes. Then the clause goes on, and when the clause ends, the statement ends the way the try block did: a `return` in the try block takes effect after the clause, as a pending `return` waits for a `finally` block.

A continuation can be consumed once. Call `resume`, `resumeWith`, and `abandon` in the clause's own code, not in a function nested inside it. The runtime rejects a second resume.

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
  try {
    return deployment()
  } handle Approval.approve(_request) {
    abandon()
    Console.log("cleanup finished")
    return "cancelled"
  }
}
```

`abandon()` runs the try block's cleanup before the clause continues. It skips the block's `catch` blocks and runs its `finally` blocks and resource releases. Cleanup failures still fail the operation.

A clause that ends without resuming also stops the try block and cleans it up first. The statement then completes, and the function continues after it. Use `abandon()` when the clause needs cleanup to finish before its next statement.

## Resume with an expected failure

```ts
// busy.effeck
import { Approval, ReviewBusy } from "./approval.effeck"

export function busyReviewer() {
  try {
    return Approval.approve("deploy now") ? "approved" : "declined"
  } handle Approval.approve(request) {
    resumeWith(() => { throw new ReviewBusy({ request }) })
  } catch (error: ReviewBusy) {
    return `review unavailable: ${error.request}`
  }
}
```

`resumeWith` runs its program at the original operation call, inside the try block. This program can raise only errors declared by the operation, and it must need no services. Read services in the clause first, then capture their values in the program passed to `resumeWith`.

Catch clauses and a `finally` block may follow the handle clauses. They cover the try block and the clauses, so here the catch clause sees the `ReviewBusy` that the operation call raised.

## Use handle as an expression

The statement is short for the `handle` function, which takes a program and returns its result:

```ts
// expression.effeck
import { Approval, release } from "./approval.effeck"

export function countReleased(versions: ReadonlyArray<string>) {
  let released = 0
  for (const version of versions) {
    const outcome = handle(() => release(version), Approval, {
      approve(_request, k) {
        return resume(k, true)
      }
    })
    if (outcome.startsWith("released")) released++
  }
  return released
}
```

`handle(program, Control, clauses, onReturn?)` takes a program, the control class, and a clause for every operation. A clause receives the operation's arguments followed by `k`, the continuation, which `resume(k, value)`, `resumeWith(k, program)`, and `abandon(k)` take. `resume` returns the resumed program's result. The optional fourth argument of `handle` transforms normal returns. In this preview, `resume` is typed with the program's original result, or `never` when that fourth argument is supplied; avoid inspecting the resumed result when clauses return different types. Keep `k` inside its clause; never save it or return it.

## Keep handlers on the executing fiber

```ts
// concurrent.effeck
import { Approval } from "./approval.effeck"

export function approveAll(requests: ReadonlyArray<string>) {
  return forEach(requests, (request) => {
    try {
      return Approval.approve(request)
    } handle Approval.approve(_request) {
      resume(true)
    }
  }, { concurrency: 4 })
}
```

Handle the operation inside each concurrent branch. A handler on the parent fiber cannot answer a child fiber's control operations.

| Diagnostic | Meaning | Fix |
| --- | --- | --- |
| EK2005 | A `resumeWith` program needs a service or raises an undeclared error | Read the service in the clause; raise only the operation's declared errors |
| EK2107 | A handle clause names an operation of something that is not a control, such as a service | Provide a service with a layer or a `provide` block |
| EK2108 | A handle statement leaves an operation of its control without a clause, or gives one two clauses | Add the missing clause, or remove the second |
| EK4201 | Work sent to another fiber still requires a control operation | Handle it inside the branch, or use sequential execution where the combinator supports it |
| EK4202 | A TypeScript function's declared requirements include a control operation | Write it in Effeck or handle the operation inside the TypeScript implementation |

Stream operators run a producer on its own fiber too. Handle any control requirements inside the producer before passing it to a Stream operator. See [Producers](112-producers.md).
