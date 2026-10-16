import { expect, test } from "bun:test"
import { Effect, Exit, Fiber, Scope } from "effect"
import { handle, operation, resume, transfer, type Continuation } from "../../kernel/kernel.ts"
import { defect, note, run, sync, value } from "./helpers.ts"

test("O01 §1.4/1.7 Owner inside an abandoned segment abandons its own held continuation", () => {
  const Outer = operation<void, void>("O01Outer")
  const Inner = operation<void, void>("O01Inner")
  const seen: string[] = []
  let ki!: Continuation
  const inner = handle("inner", [Inner], Inner().pipe(Effect.ensuring(note(seen, "inner-body"))), {
    O01Inner: (_, k) => {
      ki = k
      return Outer().pipe(Effect.ensuring(note(seen, "inner-clause")))
    }
  })
  const exit = sync(handle("outer", [Outer], inner.pipe(Effect.ensuring(note(seen, "outer-body"))), {
    O01Outer: () => Effect.succeed("stop")
  }).pipe(Effect.ensuring(note(seen, "outside"))))
  expect(value(exit)).toBe("stop")
  expect(ki.state).toBe("abandoned")
  expect(seen).toEqual(["inner-clause", "inner-body", "outer-body", "outside"])
})

test("O02 §1.4 nested owner finalizer defects propagate through both abandonments", () => {
  const A = operation<void, void>("O02A")
  const B = operation<void, void>("O02B")
  const body = handle("B", [B], B().pipe(Effect.ensuring(Effect.die("release-defect"))), {
    O02B: () => A()
  })
  const exit = sync(handle("A", [A], body, { O02A: () => Effect.succeed("stop") }))
  expect(Exit.isFailure(exit) && exit.cause.reasons.map(r => r._tag === "Die" ? r.defect : r._tag)).toEqual(["release-defect"])
})

test("O03 §1.4 transfer then same-fiber Scope.close consumes k; later resume fails", () => {
  const Ask = operation<void, void>("O03")
  const seen: string[] = []
  let k!: Continuation
  const exit = sync(Effect.gen(function*() {
    const scope = yield* Scope.make()
    yield* handle("h", [Ask], Ask().pipe(Effect.ensuring(note(seen, "release"))), {
      O03: (_, captured) => { k = captured; return Effect.provideService(transfer(k), Scope.Scope, scope) }
    })
    expect(k.state).toBe("suspended")
    yield* Scope.close(scope, Exit.void)
    return yield* Effect.exit(resume(k, undefined))
  }))
  expect(defect(value(exit))).toBe("ContinuationAlreadyConsumed")
  expect(k.state).toBe("abandoned")
  expect(seen).toEqual(["release"])
})

test("O04 §1.4 clause-forked fiber cannot resume k and leaves it usable by its owner", async () => {
  const Ask = operation<void, number>("O04")
  let foreign: any
  let state: string | undefined
  const exit = await run(handle("h", [Ask], Ask(), {
    O04: (_, k) => Effect.gen(function*() {
      const child = yield* Effect.forkChild(resume(k, 99), { startImmediately: true })
      foreign = yield* Fiber.await(child)
      state = k.state
      return yield* resume(k, 42)
    })
  }))
  expect(defect(foreign)).toBe("ContinuationWrongFiber")
  expect(state).toBe("suspended")
  expect(value(exit)).toBe(42)
})

test("O05 §1.4 transfer to an already closed Scope abandons immediately", () => {
  const Ask = operation<void, number>("O05")
  const seen: string[] = []
  let k!: Continuation
  const exit = sync(Effect.gen(function*() {
    const scope = yield* Scope.make()
    yield* Scope.close(scope, Exit.void)
    yield* handle("h", [Ask], Ask().pipe(Effect.ensuring(note(seen, "release"))), {
      O05: (_, captured) => { k = captured; return Effect.provideService(transfer(k), Scope.Scope, scope) }
    })
    return yield* Effect.exit(resume(k, 1))
  }))
  expect(defect(value(exit))).toBe("ContinuationAlreadyConsumed")
  expect(k.state).toBe("abandoned")
  expect(seen).toEqual(["release"])
})

test("O06 §1.4 transfer from a foreign fiber is rejected; the clause invocation keeps ownership", async () => {
  const Ask = operation<void, number>("O06")
  let foreign: any
  let after: string | undefined
  const seen: string[] = []
  const exit = await run(Effect.gen(function*() {
    const scope = yield* Scope.make()
    yield* handle("h", [Ask], Ask().pipe(Effect.ensuring(note(seen, "release"))), {
      O06: (_, k) => Effect.gen(function*() {
        const child = yield* Effect.forkChild(Effect.provideService(transfer(k), Scope.Scope, scope), { startImmediately: true })
        foreign = yield* Fiber.await(child)
        after = k.owner
        return "stop"
      })
    })
    const atBoundary = [...seen]
    yield* Scope.close(scope, Exit.void)
    return atBoundary
  }))
  // Fixed: transfer is fiber-affine, so the foreign transfer is a defect and
  // the clause's return abandons k at the invocation boundary.
  expect(defect(foreign)).toBe("ContinuationWrongFiber")
  expect(after).toBe("invocation")
  expect(value(exit)).toEqual(["release"])
  expect(seen).toEqual(["release"])
})

test("O07 §1.4 transfer into the Scope parked inside k is rejected; the invocation abandons k", () => {
  const Ask = operation<Scope.Scope, void>("O07")
  const seen: string[] = []
  let k!: Continuation
  const body = Effect.scoped(Effect.gen(function*() {
    const ownScope = yield* Effect.scope
    yield* Effect.addFinalizer(() => note(seen, "release"))
    return yield* Ask(ownScope)
  }))
  const exit = sync(handle("h", [Ask], body, {
    O07: (scope, captured) => { k = captured; return Effect.as(Effect.provideService(transfer(k), Scope.Scope, scope), "stop") }
  }))
  // Fixed: the direct cycle is detected; the clause fails and its owner
  // abandons k, so the parked region's finalizer runs.
  expect(defect(exit)).toBe("TransferIntoOwnRegion")
  expect(k.state).toBe("abandoned")
  expect(seen).toEqual(["release"])
})

test("O08 §1.4 moving ownership a second time must revoke the first Scope's ownership", () => {
  const Ask = operation<void, number>("O08")
  const seen: string[] = []
  let stateAfterOldClose: string | undefined
  const exit = sync(Effect.gen(function*() {
    const first = yield* Scope.make()
    const second = yield* Scope.make()
    return yield* handle("h", [Ask], Ask().pipe(Effect.ensuring(note(seen, "release"))), {
      O08: (_, k) => Effect.gen(function*() {
        yield* Effect.provideService(transfer(k), Scope.Scope, first)
        yield* Effect.provideService(transfer(k), Scope.Scope, second)
        yield* Scope.close(first, Exit.void)
        stateAfterOldClose = k.state
        const result = yield* resume(k, 42)
        yield* Scope.close(second, Exit.void)
        return result
      })
    })
  }))
  expect(stateAfterOldClose).toBe("suspended")
  expect(value(exit)).toBe(42)
  expect(seen).toEqual(["release"])
})
