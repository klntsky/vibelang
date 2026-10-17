### 9. Handlers: defining your own effects

#### What a service cannot do

A service call always comes back to the caller exactly once, with a value or an error. That covers most dependencies. It does not cover cases where whoever handles the request should decide *whether and when the caller continues*:

- pause a checkout until a person approves it, and let the approver's code decide what happens next;
- stop a search as soon as one result is found, cleaning up everything the search had opened;
- let a test script the answers to a sequence of requests, one at a time;
- run a producer one value at a time (Chapter 8).

A **control operation** is a request whose handler receives the rest of the caller's computation and decides what to do with it.

#### Declaring and performing an operation

A control operation is declared like a service, with `control`, which is short for an abstract class that extends the global `Control` (D128):

```ts es
control Approval {
  approve(request: string): boolean!ReviewBusy
}

function checkout(cartId: string) {
  using hold = Inventory.reserve(cartId)
  const ok = Approval.approve(`charge cart ${cartId}`)
  if (!ok) throw new Declined({ cartId })
  return Payments.charge(cartId)
}
// Receipt!ReviewBusy | Declined | PaymentError!Approval | Inventory | Payments
```

To the code that performs it, `Approval.approve` looks like any call: it returns a `boolean` or fails with `ReviewBusy`. Like a service, `Approval` appears in R until something handles it. The service rules apply: its members are abstract with full contracts, its key is derived from its module, and it cannot be extended or instantiated.

#### Handling it

`handle` runs a program and answers its operations. Each answer receives `k`, the rest of the program from the point of the call. `resume(k, value)` continues it with that value.

```ts es
const receipt = handle(() => checkout("c-42"), Approval, {
  approve(request, k) {
    const decision = Reviewers.decide(request)    // may take minutes: the fiber just waits
    return resume(k, decision)                    // checkout continues from its call
  }
})
// receipt: Receipt
```

While the reviewer decides, the cart's hold is still reserved and `checkout` is paused at its call, exactly as it would be during any slow service call. `resume` returns what the rest of `checkout` returned, here the `Receipt`. A clause that ends by resuming, like this one, costs no stack: the compiler ends the clause before it continues the program (Chapter 26), so a handler that resumes a million times runs in constant space.

#### Not continuing

A handler that returns without calling `resume` ends the handled program with the handler's result. Everything the program had open is cleaned up first.

```ts es
control Found {
  found(cell: Cell): never!!
}

function locate(grid: Grid, target: number) {
  return handle(() => {
    for (const row of grid.rows) {
      using cursor = grid.open(row)
      for (const cell of cursor.cells) if (cell.value === target) Found.found(cell)
    }
    return undefined
  }, Found, {
    found(cell, _k) { return cell }      // stop: the open cursor is released before handle returns
  })
}
// Cell | undefined!!
```

`found`'s result type is `never`, so its handler cannot continue the search, and the type says so.

#### Continuing with an error

`resumeWith` continues the program by running code at the call, where the program may catch what it throws:

```ts es
const outcome = handle(() => checkout("c-42"), Approval, {
  approve(request, k) {
    if (Reviewers.busy()) return resumeWith(k, () => { throw new ReviewBusy({ request }) })
    return resume(k, Reviewers.decide(request))
  }
})
```

The code passed to `resumeWith` runs inside the handled program, at the call, so two rules keep its contract honest. It may throw only the operation's declared errors (`ReviewBusy` here), which every caller of `Approval.approve` already sees in E. And it may need no service: the providers around the clause are not in scope at the call, so its R must be `never`.

```text
error EK2005: resumeWith runs this code at the perform site, inside the handled program,
  where the clause's providers are not in scope. It needs Secret.
  approvals.effeck:4:34   return resumeWith(k, () => Secret.read())
  Read it in the clause first:   const secret = Secret.read()   …   resumeWith(k, () => secret)
```

#### Testing with a handler

```ts es
it.effect("checkout charges when approved", () => {
  const receipt = handle(() => checkout("c-42"), Approval, {
    approve: (_request, k) => resume(k, true)
  })
  assert.isTrue(receipt.paid)
})
```

#### The full API

`handle(program, Operations, clauses, onReturn?)` runs `program` with a handler for `Operations`. The optional `onReturn` transforms the program's own result before `handle` returns it. Inside a clause, `k` has the type `Continuation<V, E, H>`. `V` is what the call receives, `E` is the operation's declared errors, which only `resumeWith` may raise, and `H` is the result type of the whole `handle`. `resume(k, v)` returns an `H`: the handler is deep, so the resumed program runs until it reaches the handler again, where `onReturn`, or a later clause that does not resume, produces the value. The compiler computes `H` from `onReturn`'s result (the program's own result without one) and every clause's return type. Three operations take `k`:

| Call | Meaning |
| --- | --- |
| `resume(k, value)` | Continue the program with `value` as the operation's result. Returns an `H`: what the `handle` produces when the program next reaches it |
| `resumeWith(k, () => …)` | Continue the program by running the given code at the call. It may throw only the operation's declared errors, and its R is `never` |
| `abandon(k)` | Stop the program now: its cleanup runs, its `catch` blocks do not. Fails if cleanup fails |

The standard library uses a fourth operation, `transfer`, which hands a continuation to a scope so it can be resumed after the clause returns. Producers' external iteration needs it. v0 keeps it inside the standard library (Chapter 21).

#### The rules

- **Continue at most once.** Calling `resume(k, …)` a second time is an error at the second call; nothing in the program runs twice.
- **Not continuing cleans up.** If the handler returns, fails, or is interrupted without resuming, the program's `using` and `finally` blocks run before `handle` returns.
- **`k` stays inside the clause.** You cannot store `k`, return it, or capture it in a closure that outlives the clause.
- **Handlers belong to a fiber.** A program forked onto another fiber cannot reach a handler on this one; the next section shows the check.
- **Cleanup code handles its own operations.** A `finally` block or `using` release that performs a control operation must handle it inside itself, so a handler elsewhere cannot cut cleanup short.

#### Handlers and concurrency

A handler belongs to the fiber it was installed on, so a program that runs on other fibers must handle its own control operations. The compiler checks every Effect combinator that runs its argument elsewhere (Chapter 16):

```ts es
function approveAll(ids: ReadonlyArray<string>) {
  return forEach(ids, (id) => Approval.approve(id), { concurrency: 4 })
}
```

```text
error EK4201: Approval.approve is a control operation, and forEach with concurrency 4 runs
  its callback on other fibers, where the Approval handler on this fiber cannot be reached.
  Handle Approval inside the callback, or run the loop with concurrency 1.
```

The fix puts a handler inside each concurrent branch:

```ts es
function approveAll(ids: ReadonlyArray<string>) {
  return forEach(ids, (id) =>
    handle(() => Approval.approve(id), Approval, {
      approve: (request, k) => resume(k, Reviewers.decide(request))
    }),
    { concurrency: 4 })
}
```

TypeScript code is checked the same way, from its signature. A TypeScript function can fork internally without taking a program argument, where no table can see it, so a TypeScript function whose R contains a control operation cannot be called from Effeck in v0:

```text
error EK4202: retryApproval is a TypeScript function whose requirements include the control
  operation Approval. TypeScript code can perform Approval.approve on another fiber, where
  this handler cannot reach it.
  Write retryApproval in Effeck, where the compiler sees every fork, or handle Approval inside it.
```

Effeck functions in other packages are not affected: their declarations carry the compiler's metadata, and the compiler checked their bodies when it built them.

#### In Effect today

There is no equivalent. A service method can wait and return, but it cannot end its caller with a value, and it cannot hand the caller's remaining work to someone else to continue later. Code that needs those things today restructures itself around queues, `Deferred`s, and extra fibers. Part III explains how Effeck provides this on Effect's runtime, and why it is the foundation everything else in this part is built on.
